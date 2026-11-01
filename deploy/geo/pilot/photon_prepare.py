#!/usr/bin/env python3
"""Prepare a bounded, tag-only Photon interchange dump from a real OSM PBF.

Requires osmium==4.1.1. No public search API and no inferred city hierarchy.
"""
import argparse
import collections
import hashlib
import json
import pathlib
import time
from datetime import datetime, timezone

import osmium

BBOX = [60.45, 56.72, 60.75, 56.92]
SOURCE = 'https://download.geofabrik.de/russia/ural-fed-district-261031.osm.pbf'


def sha256(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for part in iter(lambda: f.read(1024 * 1024), b''):
            h.update(part)
    return h.hexdigest()


def write_line(f, kind, content):
    f.write(json.dumps({'type': kind, 'content': content}, ensure_ascii=False) + '\n')


def classification(tags):
    if 'place' in tags:
        value = tags['place']
        address_type = {'city': 'city', 'town': 'city', 'village': 'city',
                        'suburb': 'district', 'neighbourhood': 'district'}.get(value, 'locality')
        return 'place', value, address_type
    for key in ('amenity', 'railway', 'public_transport', 'tourism', 'shop', 'leisure'):
        if key in tags and tags.get('name'):
            return key, tags[key], 'other'
    if 'highway' in tags and tags.get('name'):
        if tags['highway'] == 'bus_stop':
            return 'highway', 'bus_stop', 'other'
        return 'highway', tags['highway'], 'street'
    if tags.get('addr:housenumber') and tags.get('addr:street'):
        return 'place', 'house', 'house'
    return None


class Export(osmium.SimpleHandler):
    def __init__(self, output, objects):
        super().__init__()
        self.output, self.objects = output, objects
        self.counts = collections.Counter()

    def emit(self, obj, coords, approx):
        if not (BBOX[0] <= coords[0] <= BBOX[2] and BBOX[1] <= coords[1] <= BBOX[3]):
            return
        tags = dict(obj.tags)
        kind = classification(tags)
        if not kind:
            return
        key, value, address_type = kind
        osm_type = 'N' if isinstance(obj, osmium.osm.Node) else 'W'
        place = {'place_id': f'{osm_type}{obj.id}', 'object_type': osm_type,
                 'object_id': obj.id, 'osm_key': key, 'osm_value': value,
                 'address_type': address_type, 'country_code': 'ru',
                 'centroid': coords, 'address': {},
                 'extra': {'coordinate_method': 'way_bbox_center' if approx else 'osm_node',
                           'source': 'OpenStreetMap/Geofabrik',
                           'hierarchy': 'source_addr_tags_only'}}
        # Country code is the declared regional pilot scope; no fabricated city/state.
        for level in ('country', 'state', 'county', 'city', 'district', 'locality', 'street'):
            if tags.get(f'addr:{level}'):
                place['address'][level] = tags[f'addr:{level}']
        names = {k: v for k, v in tags.items() if k == 'name' or k.startswith('name:')
                 or k in ('alt_name', 'old_name', 'short_name', 'loc_name', 'official_name')}
        if names:
            place['name'] = names
        for source, target in (('addr:housenumber', 'housenumber'), ('addr:postcode', 'postcode')):
            if source in tags:
                place[target] = tags[source]
        write_line(self.output, 'Place', [place])
        self.objects.write(json.dumps({'osm_type': osm_type, 'osm_id': obj.id,
                                       'tags': tags, 'centroid': coords,
                                       'coordinate_method': place['extra']['coordinate_method']},
                                      ensure_ascii=False) + '\n')
        self.counts[address_type] += 1
        self.counts['total'] += 1
        if not place['address'].get('city'):
            self.counts['without_addr_city'] += 1
        if approx:
            self.counts['way_bbox_centers'] += 1

    def node(self, node):
        if node.location.valid() and len(node.tags):
            self.emit(node, [node.location.lon, node.location.lat], False)

    def way(self, way):
        if not classification(dict(way.tags)):
            return
        if not way.nodes or any(not n.location.valid() for n in way.nodes):
            self.counts['invalid_way_locations'] += 1
            return
        xs, ys = zip(*[(n.location.lon, n.location.lat) for n in way.nodes])
        self.emit(way, [(min(xs) + max(xs)) / 2, (min(ys) + max(ys)) / 2], True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('source', type=pathlib.Path)
    parser.add_argument('output_dir', type=pathlib.Path)
    parser.add_argument('--source-url', default=SOURCE)
    args = parser.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=True)
    output = args.output_dir / 'photon.jsonl'
    objects = args.output_dir / 'source-objects.jsonl'
    manifest_path = args.output_dir / 'manifest.json'
    if any(p.exists() for p in (output, objects, manifest_path)):
        parser.error('output exists; use a new versioned directory to preserve rollback')
    reader = osmium.io.Reader(str(args.source))
    timestamp = reader.header().get('osmosis_replication_timestamp')
    reader.close()
    start = time.monotonic()
    with output.open('x') as f, objects.open('x') as o:
        write_line(f, 'NominatimDumpFile', {'version': '0.1.0', 'generator': 'corpord_osm_pbf_pilot',
                   'data_timestamp': timestamp, 'features': {'sorted_by_country': True, 'has_addresslines': False}})
        # Country display names deliberately omitted: scope is metadata, not fabricated OSM names.
        handler = Export(f, o)
        handler.apply_file(str(args.source), locations=True,
                           idx='sparse_file_array,' + str(args.output_dir / 'locations.idx'))
    (args.output_dir / 'locations.idx').unlink(missing_ok=True)
    manifest = {'generator': 'corpord_osm_pbf_pilot_v1', 'prepared_at': datetime.now(timezone.utc).isoformat(),
                'osm_timestamp': timestamp, 'bbox': BBOX,
                'source': {'url': args.source_url, 'file': str(args.source.resolve()),
                           'bytes': args.source.stat().st_size, 'sha256': sha256(args.source)},
                'download': json.loads((args.source.parent / 'source-download.json').read_text())
                    if (args.source.parent / 'source-download.json').exists() else None,
                'license': 'ODbL-1.0', 'attribution': '© OpenStreetMap contributors',
                'license_url': 'https://www.openstreetmap.org/copyright',
                'counts': dict(handler.counts), 'preparation_seconds': time.monotonic() - start,
                'artifacts': {p.name: {'bytes': p.stat().st_size, 'sha256': sha256(p)} for p in (output, objects)},
                'limitations': ['Selected named nodes/ways and explicitly tagged street addresses only',
                 'No relations, interpolation or associatedStreet resolution',
                 'Way coordinates are bbox centers, not entrances or point-on-surface',
                 'Only source addr:* hierarchy; absent city/state stays absent',
                 'BBox center filtering; not entire city or national coverage',
                 'Rebuild/switch required; no incremental updates']}
    manifest_path.write_text(json.dumps(manifest, indent=2, ensure_ascii=False) + '\n')
    print(json.dumps(manifest['counts'], ensure_ascii=False))


if __name__ == '__main__':
    main()
