#!/usr/bin/env python3
"""Exact OSM city polygon extraction; optional conservative NEW Photon dump copy.

Dependencies: osmium==4.1.1, shapely==2.1.2. Never imports into Photon.
"""
import argparse
from collections import Counter
import hashlib
import json
import math
from pathlib import Path
import resource
import time

import osmium
from shapely.geometry import LineString, MultiPolygon, Point, Polygon, mapping, shape
from shapely.ops import polygonize_full
from shapely.validation import explain_validity

RELATION_ID = 6564910
SOURCE_URL = 'https://download.geofabrik.de/russia/ural-fed-district-261031.osm.pbf'
SOURCE_SHA256 = '4b75d5d4d4850bb80999d32844032626545a2068da8b4538cadb154818dc3ec0'
BOUNDARY_SHA256 = 'a310ff34cedb4d08973a64b63d7a17e511b9f78869c6d48e3484f7103da5a646'

def sha256(path):
    with open(path, 'rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

def select(source, entity, ids):
    # Filter runs in C++; Python sees only selected objects. No full node cache.
    return osmium.FileProcessor(str(source), entities=entity).with_filter(osmium.filter.IdFilter(ids))

def rings(lines):
    polygons, cuts, dangles, invalid = polygonize_full(lines)
    if not cuts.is_empty or not dangles.is_empty or not invalid.is_empty:
        raise ValueError('Unclosed/dangling/invalid boundary edges; no automatic repair')
    if polygons.is_empty:
        raise ValueError('No closed boundary rings')
    if any(len(p.interiors) for p in polygons.geoms):
        raise ValueError('Unexpected nested same-role rings; manual review required')
    return list(polygons.geoms)

def assemble(outer_lines, inner_lines):
    outer = rings(outer_lines)
    inner = rings(inner_lines) if inner_lines else []
    assigned = [[] for _ in outer]
    for hole in inner:
        owners = [i for i, shell in enumerate(outer) if shell.contains(hole)]
        if len(owners) != 1:
            raise ValueError('Inner ring must belong strictly to exactly one outer ring')
        assigned[owners[0]].append(hole.exterior.coords)
    geometry = MultiPolygon([Polygon(p.exterior.coords, assigned[i]) for i,p in enumerate(outer)])
    if geometry.is_empty or not geometry.is_valid:
        raise ValueError('Invalid assembled polygon: '+explain_validity(geometry))
    return geometry

def extract_city(source):
    tags = None
    members = []
    for relation in select(source, osmium.osm.RELATION, [RELATION_ID]):
        tags = dict(relation.tags)
        members = [(m.ref,m.role) for m in relation.members if m.type=='w']
        if len(members) != len(relation.members):
            raise ValueError('Unsupported non-way boundary member')
    if not tags or any(tags.get(k)!=v for k,v in {'place':'city','type':'multipolygon','wikidata':'Q887','name':'Екатеринбург'}.items()):
        raise ValueError('Exact city identity mismatch; municipality/bbox substitution forbidden')
    if not members or any(role not in ('outer','inner') for _,role in members) or len(set(i for i,_ in members))!=len(members):
        raise ValueError('Missing, duplicate or ambiguous member roles')
    ways = {}
    for way in select(source, osmium.osm.WAY, [i for i,_ in members]):
        ways[way.id] = [n.ref for n in way.nodes]
    if set(ways)!={i for i,_ in members} or any(len(nodes)<2 for nodes in ways.values()):
        raise ValueError('Incomplete member ways')
    node_ids = {n for nodes in ways.values() for n in nodes}
    nodes = {}
    for node in select(source, osmium.osm.NODE, node_ids):
        if not node.location.valid():
            raise ValueError('Invalid member node coordinates')
        nodes[node.id] = (node.location.lon,node.location.lat)
    if set(nodes)!=node_ids:
        raise ValueError('Incomplete member nodes')
    geometry = assemble([LineString([nodes[n] for n in ways[i]]) for i,r in members if r=='outer'],
                        [LineString([nodes[n] for n in ways[i]]) for i,r in members if r=='inner'])
    return geometry, tags, members, len(nodes)

def eligibility(place, geometry, city, margin_m=100):
    address = place.get('address',{})
    if address.get('city'):
        return 'explicit_city' if address['city']==city else 'conflicting_city'
    if place.get('extra',{}).get('coordinate_method')!='osm_node' or place.get('object_type')!='N':
        return 'uncertain_way_or_coordinate'
    coords = place.get('centroid')
    if not isinstance(coords,list) or len(coords)!=2 or not all(isinstance(c,(int,float)) and math.isfinite(c) for c in coords):
        return 'invalid_coordinate'
    point = Point(coords)
    if not geometry.contains(point):
        return 'outside_or_boundary'
    # Conservative local lower bound: both axes use the smallest metre/degree
    # scale across this city's latitude band, with Earth's minimum curvature
    # radius and a further 1% guard. It rejects more points than a true metric
    # buffer, especially N/S. No optimistic longitude-only distance assumption.
    lat_max = max(abs(geometry.bounds[1]),abs(geometry.bounds[3]))
    lower_m_per_degree = 6335000*math.pi/180*math.cos(math.radians(lat_max))*.99
    if point.distance(geometry.boundary)*lower_m_per_degree <= margin_m:
        return 'boundary_margin'
    return 'enrich'

def feature_bytes(feature):
    return (json.dumps(feature,ensure_ascii=False)+'\n').encode()

def enrich(source_dump, output, feature, expected_boundary_sha256):
    if hashlib.sha256(feature_bytes(feature)).hexdigest()!=expected_boundary_sha256:
        raise ValueError('Boundary checksum mismatch; no enrichment permitted')
    geometry = shape(feature['geometry'])
    if geometry.is_empty or not geometry.is_valid:
        raise ValueError('Invalid enrichment geometry')
    city = feature['properties']['tags'].get('name:ru') or feature['properties']['tags']['name']
    counts = Counter()
    with open(source_dump) as inp, open(output,'x') as out:
        for line in inp:
            record = json.loads(line)
            if record.get('type')=='Place':
                for place in record['content']:
                    decision = eligibility(place, geometry, city)
                    counts[decision] += 1
                    if decision=='enrich':
                        place.setdefault('address',{})['city'] = city
                        place.setdefault('extra',{})['city_provenance'] = {
                            'method':'strict_point_in_exact_city_polygon', 'relation_id':RELATION_ID,
                            'boundary_source_sha256':feature['properties']['source_sha256'],
                            'boundary_sha256':expected_boundary_sha256,
                            'margin_m':100,'margin_method':'conservative_local_lower_bound',
                            'coordinate_method':'osm_node'}
            out.write(json.dumps(record,ensure_ascii=False)+'\n')
    return dict(counts)

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('pbf',type=Path)
    parser.add_argument('new_output_dir',type=Path)
    parser.add_argument('--enrich-copy',type=Path,help='Existing Photon dump; output is a NEW copy, never imported')
    args = parser.parse_args()
    if args.new_output_dir.exists():
        parser.error('Output directory must not exist; preserve baseline and previous versions')
    start = time.monotonic()
    source_hash = sha256(args.pbf)
    if source_hash!=SOURCE_SHA256:
        parser.error('PBF checksum differs from reviewed snapshot; review new source before use')
    geometry,tags,members,node_count = extract_city(args.pbf)
    with osmium.io.Reader(str(args.pbf)) as reader:
        timestamp = reader.header().get('osmosis_replication_timestamp')
    feature = {'type':'Feature','geometry':mapping(geometry),'properties':{
        'relation_id':RELATION_ID,'tags':tags,'members':[{'way_id':i,'role':r} for i,r in members],
        'source_url':SOURCE_URL,'source_sha256':source_hash,'source_timestamp':timestamp,
        'license':'ODbL-1.0','attribution':'© OpenStreetMap contributors',
        'license_url':'https://www.openstreetmap.org/copyright'}}
    if hashlib.sha256(feature_bytes(feature)).hexdigest()!=BOUNDARY_SHA256:
        parser.error('Geometry checksum differs from reviewed snapshot; no output/enrichment permitted')
    args.new_output_dir.mkdir(parents=True)
    polygon_path = args.new_output_dir/'city-boundary.geojson'
    polygon_path.write_bytes(feature_bytes(feature))
    stats = {'relation_id':RELATION_ID,'way_count':len(members),'node_count':node_count,
        'polygon_count':len(geometry.geoms),'hole_count':sum(len(p.interiors) for p in geometry.geoms),
        'bounds':geometry.bounds,'valid':geometry.is_valid,'boundary_sha256':sha256(polygon_path)}
    if args.enrich_copy:
        stats['baseline_sha256'] = sha256(args.enrich_copy)
        stats['enrichment'] = enrich(args.enrich_copy,args.new_output_dir/'photon.jsonl',feature,BOUNDARY_SHA256)
        stats['enriched_sha256'] = sha256(args.new_output_dir/'photon.jsonl')
    stats.update({'elapsed_seconds':time.monotonic()-start,'rss_kib':resource.getrusage(resource.RUSAGE_SELF).ru_maxrss})
    (args.new_output_dir/'boundary-stats.json').write_text(json.dumps(stats,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(stats,ensure_ascii=False))

if __name__=='__main__':
    main()
