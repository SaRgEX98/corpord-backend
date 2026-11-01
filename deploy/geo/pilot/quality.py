#!/usr/bin/env python3
"""Bounded stdlib-only acceptance probe; use real source files, never public search APIs."""
import argparse
import datetime
import hashlib
import json
import math
import pathlib
import resource
import struct
import time
import urllib.error
import urllib.parse
import urllib.request

resource.setrlimit(resource.RLIMIT_AS, (512 * 1024**2, 512 * 1024**2))
BBOX = [60.45, 56.72, 60.75, 56.92]


def local(url):
    if urllib.parse.urlparse(url).hostname not in ('localhost', '127.0.0.1', '::1'):
        raise ValueError('Only loopback pilot services are allowed: ' + url)
    return url


def fetch(url, method='GET', headers=None, limit=4 * 1024**2):
    request = urllib.request.Request(local(url), method=method, headers=headers or {})
    # Refuse redirects: an apparently local source must not fetch a public service.
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            return None
    with urllib.request.build_opener(NoRedirect).open(request, timeout=10) as response:
        body = response.read(limit + 1)
        if len(body) > limit:
            raise ValueError('Response exceeds bounded read')
        return response.status, dict(response.headers), body


def digest(path):
    result = hashlib.sha256()
    with open(path, 'rb') as stream:
        for block in iter(lambda: stream.read(65536), b''):
            result.update(block)
    return result.hexdigest()


def inside(xy, bbox):
    return len(xy) == 2 and all(isinstance(n, (int, float)) and math.isfinite(n) for n in xy) and bbox[0] <= xy[0] <= bbox[2] and bbox[1] <= xy[1] <= bbox[3]


def identity(feature):
    p = feature['properties']
    return str(p.get('osm_type', '')).upper(), str(p.get('osm_id', ''))


def fixtures(args):
    records = []
    with open(args.source) as stream:
        for line in stream:
            row = json.loads(line)
            if row.get('type') == 'Place':
                records.extend(row['content'])
    cases = []
    def add(kind, query, record=None, required=False):
        if not query or any(c['kind'] == kind and c['query'] == query for c in cases):
            return
        cases.append({'kind': kind, 'query': query, 'expected_osm': [record['object_type'], str(record['object_id'])] if record else None, 'required_match': required})
    names = []
    addresses = []
    for r in records:
        name = r.get('name', {}).get('name:ru') or r.get('name', {}).get('name', '')
        if name and any('\u0400' <= c <= '\u04ff' for c in name):
            names.append((r, name))
        street = r.get('address', {}).get('street')
        if street and r.get('housenumber'):
            addresses.append((r, street + ' ' + str(r['housenumber'])))
    # Prefer transport/settlement objects before other named POI, reproducibly by ID.
    names.sort(key=lambda pair: (pair[0].get('osm_value') not in ('city', 'town', 'bus_station', 'station'), str(pair[0]['object_id'])))
    for r, name in names[:12]:
        kind = 'city' if r.get('osm_value') in ('city', 'town', 'village') else 'station' if r.get('osm_value') in ('station', 'bus_station') else 'poi'
        add(kind, name, r, True)
    for r, text in addresses[:8]:
        add('address', text, r, True)
    for r, name in names[:8]:
        add('partial', name[:max(3, len(name) * 2 // 3)], r)
    for r, name in names[:6]:
        position = max(1, len(name) // 2)
        add('typo', name[:position] + name[position + 1:], r)
    seen = {}
    for r, name in names:
        if name in seen:
            add('duplicate_name', name, r)
        seen[name] = r
        if sum(c['kind'] == 'duplicate_name' for c in cases) >= 3:
            break
    for query in ['Москва Красная площадь', 'Тюмень автовокзал', 'Владивосток вокзал', 'НесуществующийОбъектЫЪ999999']:
        add('out_of_coverage', query)
    if not 30 <= len(cases) <= 50:
        raise ValueError(f'Need 30–50 grounded queries, got {len(cases)}; source lacks named/address coverage')
    output = {'schema': 1, 'bbox': BBOX, 'source_sha256': digest(args.source), 'truth_scope': 'import parity only; source OSM data has not been independently surveyed', 'cases': cases}
    pathlib.Path(args.output).write_text(json.dumps(output, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({'fixture_count': len(cases), 'output': args.output}))


def percentile(values, fraction):
    return sorted(values)[max(0, math.ceil(len(values) * fraction) - 1)] if values else None


def font_stacks(value):
    if not value:
        return set()
    if value[0] == 'literal':
        return {','.join(value[1])}
    if value[0] == 'case':
        return set().union(*(font_stacks(branch) for branch in [*value[2:-1:2], value[-1]]))
    if all(isinstance(item, str) for item in value):
        return {','.join(value)}
    raise ValueError('Unsupported dynamic text-font expression; review manually')


def run(args):
    fixture = json.loads(pathlib.Path(args.fixtures).read_text())
    if digest(args.source) != fixture['source_sha256']:
        raise ValueError('Source changed; regenerate query fixtures')
    imported = {}
    with open(args.source) as stream:
        for line in stream:
            value = json.loads(line)
            if value.get('type') == 'Place':
                for place in value['content']:
                    imported[(place['object_type'], str(place['object_id']))] = place['centroid']
    report = {'started_utc': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'fixture_sha256': digest(args.fixtures), 'source_sha256': digest(args.source), 'bbox': fixture['bbox'], 'truth_scope': fixture['truth_scope'], 'repeats': args.repeats, 'queries': [], 'failures': []}
    bbox = fixture['bbox']
    latencies = []
    for case in fixture['cases']:
        row = dict(case, runs=[])
        for _ in range(args.repeats):
            url = args.photon.rstrip('/') + '/api?' + urllib.parse.urlencode({'q': case['query'], 'lang': 'ru', 'limit': 10, 'bbox': ','.join(map(str, bbox))})
            began = time.perf_counter()
            try:
                status, _, body = fetch(url)
                ms = (time.perf_counter() - began) * 1000
                latencies.append(ms)
                result = json.loads(body)
                assert result.get('type') == 'FeatureCollection', 'not FeatureCollection'
                features = result['features']
                assert isinstance(features, list) and len(features) <= 10, 'invalid result list'
                ids = []
                for f in features:
                    assert f.get('type') == 'Feature' and f['geometry']['type'] == 'Point', 'invalid result geometry'
                    assert inside(f['geometry']['coordinates'], bbox), 'coordinates outside pilot bbox'
                    key = identity(f)
                    assert key[0] in ('N', 'W', 'R') and key[1].isdigit() and int(key[1]) > 0, 'invalid OSM identity'
                    assert key in imported, 'OSM identity absent from imported source'
                    assert max(abs(a - b) for a, b in zip(imported[key], f['geometry']['coordinates'])) < .00001, 'centroid differs from imported source'
                    ids.append(key)
                assert len(set(ids)) == len(ids), 'duplicate OSM identity'
                matched = not case['expected_osm'] or tuple(case['expected_osm']) in ids
                row['runs'].append({'status': status, 'ms': round(ms, 3), 'ids': ids, 'expected_match': matched, 'features': features})
            except (AssertionError, OSError, ValueError, KeyError, TypeError) as error:
                row['runs'].append({'error': str(error)})
                report['failures'].append({'query': case['query'], 'error': str(error)})
        report['queries'].append(row)
    # Probe malformed API input, separately from network/downstream failure tests.
    for suffix in ['/api?q=x&bbox=broken', '/api?q=x&limit=-1']:
        try:
            status, _, _ = fetch(args.photon.rstrip('/') + suffix)
        except urllib.error.HTTPError as error:
            status = error.code
        except OSError as error:
            status = str(error)
        report.setdefault('invalid_requests', []).append({'path': suffix, 'status': status})
    matches = [r for q in report['queries'] if q['expected_osm'] for r in q['runs']]
    report['summary'] = {'requests': len(fixture['cases']) * args.repeats, 'successful_requests': len(latencies), 'p50_ms': percentile(latencies, .5), 'p95_ms': percentile(latencies, .95), 'structural_failures': len(report['failures']), 'source_identity_matches': sum(r.get('expected_match', False) for r in matches), 'source_identity_attempts': len(matches), 'quality_gate': 'manual review required: exact/partial/typo recall is measured, not silently passed'}
    report['summary']['by_kind'] = {}
    report['summary']['required_identity_misses'] = [{'query': q['query'], 'expected_osm': q['expected_osm']} for q in report['queries'] if q['required_match'] and any(not r.get('expected_match') for r in q['runs'])]
    for kind in sorted({q['kind'] for q in report['queries']}):
        rows = [r for q in report['queries'] if q['kind'] == kind for r in q['runs']]
        report['summary']['by_kind'][kind] = {'attempts': len(rows), 'nonempty': sum(bool(r.get('features')) for r in rows), 'identity_matches': sum(bool(r.get('expected_match')) for r in rows), 'errors': sum('error' in r for r in rows)}
    pathlib.Path(args.output).write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(report['summary'], ensure_ascii=False, indent=2))
    return 1 if report['failures'] else 0


def maps(args):
    checks = []
    for style_url in args.style:
        _, _, body = fetch(style_url)
        style = json.loads(body)
        assert style['version'] == 8 and style['layers'], 'invalid/empty map style'
        glyphs = style.get('glyphs')
        assert glyphs, 'no glyphs URL'
        fonts = set().union(*(font_stacks(layer.get('layout', {}).get('text-font', [])) for layer in style['layers']))
        assert fonts, 'no label font stack'
        for font in fonts:
            glyph_url = urllib.parse.urljoin(style_url, glyphs.replace('{fontstack}', urllib.parse.quote(font)).replace('{range}', '1024-1279'))
            status, _, data = fetch(glyph_url)
            assert status == 200 and data, 'missing Cyrillic glyph block'
        for name, source in style['sources'].items():
            assert source.get('attribution') and 'OpenStreetMap' in source['attribution'], 'missing OSM attribution'
            source_url = source.get('url', '')
            assert source_url.startswith('pmtiles://'), 'pilot expects actual PMTiles source'
            url = urllib.parse.urljoin(style_url, source_url.removeprefix('pmtiles://'))
            status, head, _ = fetch(url, method='HEAD')
            assert status == 200 and int(head['Content-Length']) > 127 and head.get('Accept-Ranges') == 'bytes', 'HEAD/Range capability'
            status, headers, data = fetch(url, headers={'Range': 'bytes=0-126', 'Origin': 'http://localhost:3000'})
            assert status == 206 and len(data) == 127 and data[:7] == b'PMTiles' and data[7] == 3, 'not PMTiles v3 ranged header'
            assert headers.get('Content-Range', '').startswith('bytes 0-126/'), 'invalid Content-Range'
            assert headers.get('Access-Control-Allow-Origin') == 'http://localhost:3000', 'missing CORS'
            assert head.get('ETag') and head.get('Cache-Control'), 'missing cache/ETag'
            bounds = [v / 1e7 for v in struct.unpack_from('<iiii', data, 102)]
            assert bounds[0] <= BBOX[0] and bounds[1] <= BBOX[1] and bounds[2] >= BBOX[2] and bounds[3] >= BBOX[3], 'archive bounds do not cover pilot'
            for extra, expected in [({'Range': 'bytes=' + head['Content-Length'] + '-'}, 416), ({'If-None-Match': head['ETag']}, 304)]:
                try:
                    actual, _, _ = fetch(url, headers=extra)
                except urllib.error.HTTPError as error:
                    actual = error.code
                assert actual == expected, f'expected HTTP {expected}, got {actual}'
            _, untrusted, _ = fetch(url, method='HEAD', headers={'Origin': 'https://untrusted.invalid'})
            assert not untrusted.get('Access-Control-Allow-Origin'), 'untrusted origin allowed'
            checks.append({'style': style_url, 'source': name, 'archive_bytes': int(head['Content-Length']), 'archive_bbox': bounds, 'zoom_range': list(data[100:102]), 'glyph_fonts': sorted(fonts), 'range': 'PASS', 'cyrillic_block': 'PASS', 'note': 'glyph availability is not visual label readability'})
        if style.get('sprite'):
            for ext in ('.json', '.png'):
                fetch(urllib.parse.urljoin(style_url, style['sprite']) + ext)
    pathlib.Path(args.output).write_text(json.dumps(checks, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps(checks, ensure_ascii=False, indent=2))
    return 0


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    fixture = commands.add_parser('fixtures')
    fixture.add_argument('--source', required=True)
    fixture.add_argument('--output', required=True)
    probe = commands.add_parser('run')
    probe.add_argument('--source', required=True)
    probe.add_argument('--fixtures', required=True)
    probe.add_argument('--photon', default='http://127.0.0.1:2333')
    probe.add_argument('--repeats', type=int, default=3)
    probe.add_argument('--output', required=True)
    tile = commands.add_parser('maps')
    tile.add_argument('--style', action='append', required=True)
    tile.add_argument('--output', required=True)
    args = parser.parse_args()
    if args.command == 'run' and not 1 <= args.repeats <= 10:
        parser.error('repeats must be 1–10')
    return {'fixtures': fixtures, 'run': run, 'maps': maps}[args.command](args)


if __name__ == '__main__':
    raise SystemExit(main())
