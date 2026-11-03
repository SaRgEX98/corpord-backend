#!/usr/bin/env python3
"""Build a bounded-memory, auditable city/stop extract from a pinned OSM PBF.

Only explicit settlement names or settlement polygons establish stop membership.
Nearest-city guesses and missing timezones are never silently filled in.
"""
import argparse
import collections
import hashlib
import json
import math
import importlib.metadata
from pathlib import Path
import osmium
from shapely.geometry import Point, shape
from shapely.strtree import STRtree
from timezonefinder import TimezoneFinder
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError


def norm(value):
    return ' '.join(value.casefold().replace('ё', 'е').split())


def sha256(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for block in iter(lambda: f.read(8 * 1024 * 1024), b''):
            h.update(block)
    return h.hexdigest()


def tags(obj):
    return dict(obj.tags)


def name(t):
    return (t.get('name:ru') or t.get('name') or '').strip()


def is_stop(t):
    return (t.get('amenity') == 'bus_station' or t.get('highway') == 'bus_stop'
            or (t.get('public_transport') == 'platform' and t.get('bus') == 'yes'))


def included_region(t):
    # The pinned Geofabrik Russia extract explicitly includes these two areas.
    # Preserve OSM subdivision identifiers; this profile is dataset coverage,
    # not a reassignment of ISO country or sovereignty.
    code=t.get('ISO3166-2','')
    return code.startswith('RU-') or code in ('UA-43','UA-40')


def prepare(source, output, source_url):
    output.mkdir(parents=True, exist_ok=True)
    if (output / 'catalog.jsonl').exists():
        raise ValueError('output already exists; use a new versioned output directory')
    stats = collections.Counter()
    selected = {}
    relations = {}
    needed_ways = set()
    needed_nodes = set()
    city_nodes = []
    stop_nodes = []
    # C++ filters avoid Python callbacks for hundreds of millions of unrelated nodes.
    filt = osmium.filter.TagFilter(('place', 'city'), ('place', 'town'),
        ('amenity', 'bus_station'), ('highway', 'bus_stop'),
        ('public_transport', 'platform'), ('boundary', 'administrative'))
    for obj in osmium.FileProcessor(str(source)).with_filter(filt):
        t = tags(obj)
        settlement = t.get('place') in ('city', 'town')
        region = t.get('boundary') == 'administrative' and t.get('admin_level') == '4' and included_region(t)
        stop = is_stop(t)
        if not (settlement or region or stop):
            continue
        if not name(t):
            stats['unnamed_objects_skipped'] += 1
            continue
        typ = obj.type_str()
        key = f'osm:{typ}:{obj.id}'
        if typ == 'n':
            if not obj.location.valid():
                stats['invalid_locations'] += 1
                continue
            record = (key, t, obj.location.lon, obj.location.lat)
            if settlement:
                city_nodes.append(record)
            elif stop:
                stop_nodes.append(record)
        elif typ == 'w':
            refs = [n.ref for n in obj.nodes]
            selected[obj.id] = (key, t, refs)
            needed_nodes.update(refs)
        elif typ == 'r' and (region or settlement or t.get('amenity') == 'bus_station'):
            members = [(m.type, m.ref, m.role) for m in obj.members]
            relations[obj.id] = (key, t, members)
            needed_ways.update(ref for typ, ref, role in members if typ == 'w')
            if any(typ == 'r' for typ, ref, role in members):
                stats['relations_with_nested_members'] += 1
    print(json.dumps({'phase': 'selected', 'cities': len(city_nodes), 'stops': len(stop_nodes), 'relations': len(relations)}), flush=True)
    # Only geometry referenced by selected ways/relations enters the location map.
    ways = {}
    if needed_ways:
        for obj in osmium.FileProcessor(str(source), osmium.osm.WAY).with_filter(osmium.filter.IdFilter(list(needed_ways))):
            refs = [n.ref for n in obj.nodes]
            ways[obj.id] = refs
            needed_nodes.update(refs)
    coords = {}
    if needed_nodes:
        for obj in osmium.FileProcessor(str(source), osmium.osm.NODE).with_filter(osmium.filter.IdFilter(list(needed_nodes))):
            if obj.location.valid():
                coords[obj.id] = (obj.location.lon, obj.location.lat)
    # Let libosmium assemble rings, roles and holes from a small reference-complete file.
    filtered = output / 'geometry.osm.pbf'
    with osmium.SimpleWriter(str(filtered)) as writer:
        for ident, (lon, lat) in sorted(coords.items()):
            writer.add_node(osmium.osm.mutable.Node(id=ident, location=(lon, lat)))
        for ident, refs in sorted({**ways, **{k: v[2] for k, v in selected.items()}}.items()):
            writer.add_way(osmium.osm.mutable.Way(id=ident, nodes=refs, tags=selected.get(ident, ('', {}, []))[1]))
        for ident, (_, t, members) in sorted(relations.items()):
            writer.add_relation(osmium.osm.mutable.Relation(id=ident, tags=t, members=members))
    regions = []
    city_areas = []
    station_areas = []
    factory = osmium.geom.GeoJSONFactory()

    class Areas(osmium.SimpleHandler):
        def area(self, area):
            try:
                t = tags(area)
                geom = shape(json.loads(factory.create_multipolygon(area)))
                if not geom.is_valid or geom.is_empty:
                    stats['invalid_areas'] += 1
                    return
                key = f"osm:{'w' if area.from_way() else 'r'}:{area.orig_id()}"
                if t.get('boundary') == 'administrative' and t.get('admin_level') == '4' and included_region(t):
                    regions.append((key, t, geom))
                elif t.get('place') in ('city', 'town'):
                    city_areas.append((key, t, geom))
                elif t.get('amenity') == 'bus_station':
                    station_areas.append((key, t, geom))
            except (RuntimeError, ValueError):
                stats['invalid_areas'] += 1

    Areas().apply_file(str(filtered), locations=True)
    region_tree = STRtree([r[2] for r in regions])
    tf = TimezoneFinder(in_memory=False)
    if tf.timezone_at(lng=60.6, lat=56.8) != 'Asia/Yekaterinburg':
        raise ValueError('full timezone dataset required; reduced same-since-now data is unsupported')
    city_records = []
    city_geometries = []
    aliases = collections.defaultdict(list)
    review = []

    def make_city(key, t, lon, lat, polygon=None):
        p = Point(lon, lat)
        hits = region_tree.query(p, predicate='within')
        if len(hits) != 1:
            stats['cities_without_unique_region'] += 1
            review.append({'source_key': key, 'reason': 'region_ambiguous_or_missing', 'name': name(t)})
            return
        region = regions[int(hits[0])]
        explicit_zone=t.get('timezone')
        if explicit_zone:
            try: ZoneInfo(explicit_zone)
            except (ValueError,ZoneInfoNotFoundError):
                stats['invalid_source_timezones']+=1
                explicit_zone=None
        zone = explicit_zone or tf.timezone_at(lng=lon, lat=lat)
        if zone and zone.startswith('Etc/'):
            zone = None
        try:
            popularity = max(0, int(t.get('population', '0').replace(' ', '')))
        except ValueError:
            popularity = 0
        record = dict(kind='city', source_key=key, name=name(t), region=name(region[1]),
            timezone=zone, latitude=lat, longitude=lon, popularity=min(popularity, 2147483647),
            provenance={'region_source': region[0], 'region_iso3166_2':region[1].get('ISO3166-2'), 'timezone_source': 'OSM timezone' if explicit_zone else 'timezonefinder-8.3.0', 'place': t.get('place')})
        city_records.append(record)
        aliases[norm(record['name'])].append(record)
        for alias in (t.get('alt_name', '') + ';' + t.get('official_name', '')).split(';'):
            if alias.strip():
                aliases[norm(alias)].append(record)
        if polygon is not None:
            city_geometries.append((record, polygon))

    for key, t, lon, lat in city_nodes:
        matching = [(ak, at, g) for ak, at, g in city_areas if norm(name(at)) == norm(name(t)) and g.contains(Point(lon, lat))]
        make_city(key, t, lon, lat, matching[0][2] if len(matching) == 1 else None)
    # Polygon-only cities remain discoverable; do not duplicate label nodes.
    for key, t, geom in city_areas:
        if any(norm(name(t)) == norm(name(nt)) and geom.contains(Point(lon, lat)) for _, nt, lon, lat in city_nodes):
            continue
        p = geom.representative_point()
        make_city(key, t, p.x, p.y, geom)
    city_tree = STRtree([g for _, g in city_geometries])
    stop_records = []
    area_stop_keys = {key for key, _, _ in station_areas}
    for key, t, refs in selected.values():
        if is_stop(t) and key not in area_stop_keys:
            # Open ways / incomplete geometries cannot provide a verified station point.
            stats['nonarea_stop_ways_skipped'] += 1
    stops = stop_nodes + [(key, t, g.representative_point().x, g.representative_point().y) for key, t, g in station_areas]
    for key, t, lon, lat in stops:
        p = Point(lon, lat)
        city = None
        method = None
        hits = city_tree.query(p, predicate='within')
        if len(hits) == 1:
            city = city_geometries[int(hits[0])][0]
            method = 'settlement_polygon'
        explicit = t.get('addr:city') or t.get('is_in:city')
        if city is None and explicit:
            # An exact name alone is ambiguous across Russia. Require region and
            # a conservative distance guard; distance NEVER creates a mapping.
            rh = region_tree.query(p, predicate='within')
            region_name = name(regions[int(rh[0])][1]) if len(rh) == 1 else None
            matches = [c for c in aliases[norm(explicit)] if c['region'] == region_name
                and abs(c['latitude'] - lat) < 0.6
                and abs((c['longitude'] - lon) * math.cos(math.radians(lat))) < 0.6]
            matches = {c['source_key']: c for c in matches}
            if len(matches) == 1:
                city = next(iter(matches.values()))
                method = 'explicit_city_and_region'
        if city is None:
            stats['stops_unmapped'] += 1
            review.append({'source_key': key, 'name': name(t), 'reason': 'city_mapping_unconfirmed'})
        address = ', '.join(filter(None, [t.get('addr:city'), t.get('addr:street'), t.get('addr:housenumber')]))
        stop_zone=t.get('timezone')
        if stop_zone:
            try: ZoneInfo(stop_zone)
            except (ValueError,ZoneInfoNotFoundError):
                stats['invalid_source_timezones']+=1
                review.append({'source_key':key,'name':name(t),'reason':'invalid_source_timezone'})
                stop_zone=None
        stop_records.append(dict(kind='stop', source_key=key, name=name(t), address=address,
            latitude=lat, longitude=lon, city_source_key=city['source_key'] if city else None,
            timezone=stop_zone, stop_kind='bus_station' if t.get('amenity') == 'bus_station' else 'bus_stop',
            provenance={'mapping_method': method, 'explicit_city': explicit}))
    with (output / 'catalog.jsonl').open('w') as out:
        for record in sorted(city_records, key=lambda x: x['source_key']) + sorted(stop_records, key=lambda x: x['source_key']):
            out.write(json.dumps(record, ensure_ascii=False) + '\n')
    with (output / 'review.jsonl').open('w') as out:
        for record in review:
            out.write(json.dumps(record, ensure_ascii=False) + '\n')
    manifest = dict(format_version=1, coverage_profile='geofabrik-russia-v1 (OSM RU-* + UA-43/UA-40)', timezone_data_version=importlib.metadata.version('timezonefinder-data'), prepare_version='2026-11-03-v1', source_url=source_url, source_sha256=sha256(source),
        source_bytes=source.stat().st_size, license='ODbL-1.0 / © OpenStreetMap contributors',
        cities=len(city_records), stops=len(stop_records), regions=len(regions),
        mapped_stops=sum(bool(s['city_source_key']) for s in stop_records), counts=dict(stats),
        catalog_sha256=sha256(output / 'catalog.jsonl'),
        completeness='OSM city/town objects in this extract; no official completeness guarantee')
    (output / 'manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(manifest, ensure_ascii=False), flush=True)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--pbf', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--source-url', required=True)
    args = parser.parse_args()
    prepare(args.pbf, args.output, args.source_url)
