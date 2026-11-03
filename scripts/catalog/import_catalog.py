#!/usr/bin/env python3
"""COPY to staging, validate, link, update unchanged owned rows, commit atomically.

CATALOG_DATABASE_URL is never printed. Dry run rolls back by default.
"""
import argparse
import collections
import json
import math
import os
from pathlib import Path
from zoneinfo import ZoneInfo, ZoneInfoNotFoundError
import psycopg
from psycopg.types.json import Jsonb
from prepare import sha256

CITY_FIELDS = "jsonb_build_object('name',c.name,'region',c.region,'timezone',c.timezone,'popularity',c.popularity,'latitude',c.latitude,'longitude',c.longitude)"
STOP_FIELDS = "jsonb_build_object('name',c.name,'address',coalesce(c.address,''),'city_id',c.city_id,'timezone',c.timezone,'latitude',c.latitude::float8,'longitude',c.longitude::float8,'stop_kind',c.stop_kind)"


def validate(p):
    if p.get('kind') not in ('city', 'stop') or not isinstance(p.get('source_key'), str) or len(p['source_key']) > 180:
        raise ValueError('invalid kind/source_key')
    if not isinstance(p.get('name'), str) or not p['name'].strip() or len(p['name']) > (160 if p['kind']=='city' else 2048):
        raise ValueError('invalid name')
    for field, bound in [('latitude', 90), ('longitude', 180)]:
        if not isinstance(p.get(field), (int, float)) or not math.isfinite(p[field]) or abs(p[field]) > bound:
            raise ValueError('invalid coordinates')
    if p.get('timezone'):
        try:
            ZoneInfo(p['timezone'])
        except (ValueError, ZoneInfoNotFoundError):
            raise ValueError('invalid IANA timezone') from None
    if p['kind'] == 'city':
        if not isinstance(p.get('region'), str) or not p['region'].strip() or len(p['region']) > 160:
            raise ValueError('invalid region')
        if not isinstance(p.get('popularity', 0), int) or not 0 <= p.get('popularity', 0) <= 2147483647:
            raise ValueError('invalid popularity')
    elif len(p.get('address', '')) > 1000 or p.get('stop_kind') not in ('bus_station', 'bus_stop'):
        raise ValueError('invalid stop')
    return p


def run(directory, apply=False):
    manifest = json.loads((directory / 'manifest.json').read_text())
    data = directory / 'catalog.jsonl'
    if manifest.get('format_version') != 1 or sha256(data) != manifest.get('catalog_sha256'):
        raise ValueError('manifest/catalog digest mismatch')
    stats = collections.Counter()
    with psycopg.connect(os.environ['CATALOG_DATABASE_URL']) as conn:
        conn.execute("SET LOCAL lock_timeout='5s'")
        conn.execute("SELECT pg_advisory_xact_lock(1129271890)")
        conn.execute('CREATE TEMP TABLE stage(kind text,source_key text PRIMARY KEY,p jsonb,entity_id int,city_id int,owned boolean) ON COMMIT DROP')
        with conn.cursor().copy('COPY stage(kind,source_key,p) FROM STDIN') as copy:
            with data.open() as f:
                for line in f:
                    if len(line) > 65536:
                        raise ValueError('oversized source record')
                    p = validate(json.loads(line))
                    copy.write_row((p['kind'], p['source_key'], Jsonb(p)))
                    stats[p['kind'] + '_input'] += 1
        if stats['city_input'] != manifest['cities'] or stats['stop_input'] != manifest['stops']:
            raise ValueError('manifest row counts mismatch')
        conn.execute('CREATE INDEX ON stage(kind); ANALYZE stage')
        if conn.execute("SELECT 1 FROM stage s JOIN catalog_source_links l USING(source_key) WHERE s.kind<>l.kind LIMIT 1").fetchone():
            raise ValueError('source identity changed kind')
        conn.execute('UPDATE stage s SET entity_id=l.entity_id,owned=l.owned FROM catalog_source_links l WHERE l.source_key=s.source_key')
        for kind, table in [('city', 'route_cities'), ('stop', 'stops')]:
            if conn.execute(f"SELECT 1 FROM stage s WHERE s.kind=%s AND entity_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM {table} c WHERE c.id=s.entity_id) LIMIT 1", (kind,)).fetchone():
                raise ValueError('a tracked catalog row was deleted; restore it or use a clean database')
        # Bind an unambiguous legacy city without taking ownership of its fields.
        conn.execute('''WITH matches AS (
          SELECT s.source_key,min(c.id) AS id FROM stage s JOIN route_cities c
          ON c.name=s.p->>'name' AND c.region=s.p->>'region' AND c.timezone IS NOT DISTINCT FROM s.p->>'timezone'
          WHERE s.kind='city' AND s.entity_id IS NULL AND c.source_key IS NULL
          AND 1=(SELECT count(*) FROM stage other WHERE other.kind='city' AND other.p->>'name'=s.p->>'name' AND other.p->>'region'=s.p->>'region')
          GROUP BY s.source_key HAVING count(*)=1)
          UPDATE stage s SET entity_id=m.id,owned=false FROM matches m WHERE s.source_key=m.source_key''')
        # Enrich only missing coordinates of an unambiguous legacy city. Its
        # name, timezone, priority and existing coordinates remain user-owned.
        stats['legacy_city_coordinates_filled'] = conn.execute('''UPDATE route_cities c SET latitude=(s.p->>'latitude')::float8,longitude=(s.p->>'longitude')::float8
          FROM stage s WHERE s.kind='city' AND s.owned=false AND s.entity_id=c.id AND c.latitude IS NULL AND c.longitude IS NULL''').rowcount
        stats['legacy_city_links'] = conn.execute("SELECT count(*) FROM stage WHERE kind='city' AND owned=false").fetchone()[0]
        conn.execute("UPDATE stage SET entity_id=nextval('route_cities_id_seq'),owned=true WHERE kind='city' AND entity_id IS NULL")
        stats['cities_created'] = conn.execute('''INSERT INTO route_cities(id,name,region,timezone,popularity,latitude,longitude,source_key)
          SELECT entity_id,p->>'name',p->>'region',p->>'timezone',coalesce((p->>'popularity')::int,0),(p->>'latitude')::float8,(p->>'longitude')::float8,source_key
          FROM stage s WHERE kind='city' AND owned AND NOT EXISTS(SELECT 1 FROM route_cities c WHERE c.id=s.entity_id)''').rowcount
        conn.execute(f"UPDATE route_cities c SET source_snapshot={CITY_FIELDS} WHERE source_snapshot IS NULL AND source_key IN (SELECT source_key FROM stage WHERE kind='city')")
        stats['cities_updated'] = conn.execute(f'''UPDATE route_cities c SET name=s.p->>'name',region=s.p->>'region',
          popularity=coalesce((s.p->>'popularity')::int,0),latitude=(s.p->>'latitude')::float8,longitude=(s.p->>'longitude')::float8
          FROM stage s WHERE s.kind='city' AND c.source_key=s.source_key AND {CITY_FIELDS}=c.source_snapshot
          AND c.timezone IS NOT DISTINCT FROM s.p->>'timezone'
          AND (c.name,c.region,c.popularity,c.latitude,c.longitude) IS DISTINCT FROM
          (s.p->>'name',s.p->>'region',coalesce((s.p->>'popularity')::int,0),(s.p->>'latitude')::float8,(s.p->>'longitude')::float8)''').rowcount
        # Snapshot only owned rows whose current values still match the incoming values.
        conn.execute(f'''UPDATE route_cities c SET source_snapshot={CITY_FIELDS} FROM stage s
          WHERE s.kind='city' AND c.source_key=s.source_key AND c.source_snapshot IS DISTINCT FROM {CITY_FIELDS} AND (c.name,c.region,c.timezone,c.popularity,c.latitude,c.longitude)
          IS NOT DISTINCT FROM (s.p->>'name',s.p->>'region',s.p->>'timezone',coalesce((s.p->>'popularity')::int,0),(s.p->>'latitude')::float8,(s.p->>'longitude')::float8)''')
        conn.execute("UPDATE stage s SET city_id=c.entity_id FROM stage c WHERE c.kind='city' AND c.source_key=s.p->>'city_source_key' AND s.kind='stop'")
        conn.execute("UPDATE stage SET entity_id=nextval('stops_id_seq'),owned=true WHERE kind='stop' AND entity_id IS NULL")
        stats['stops_created'] = conn.execute('''INSERT INTO stops(id,name,address,latitude,longitude,city_id,timezone,stop_kind,source_key)
          SELECT entity_id,p->>'name',coalesce(p->>'address',''),(p->>'latitude')::numeric,(p->>'longitude')::numeric,city_id,p->>'timezone',p->>'stop_kind',source_key
          FROM stage s WHERE kind='stop' AND NOT EXISTS(SELECT 1 FROM stops c WHERE c.id=s.entity_id)''').rowcount
        conn.execute(f"UPDATE stops c SET source_snapshot={STOP_FIELDS} WHERE source_snapshot IS NULL AND source_key IN (SELECT source_key FROM stage WHERE kind='stop')")
        stats['stops_updated'] = conn.execute(f'''UPDATE stops c SET name=s.p->>'name',address=coalesce(s.p->>'address',''),
          latitude=(s.p->>'latitude')::numeric,longitude=(s.p->>'longitude')::numeric,city_id=s.city_id,timezone=s.p->>'timezone',stop_kind=s.p->>'stop_kind'
          FROM stage s WHERE s.kind='stop' AND c.source_key=s.source_key AND {STOP_FIELDS}=c.source_snapshot
          AND (c.city_id,c.timezone) IS NOT DISTINCT FROM (s.city_id,s.p->>'timezone')
          AND (c.name,coalesce(c.address,''),c.latitude,c.longitude,c.stop_kind) IS DISTINCT FROM
          (s.p->>'name',coalesce(s.p->>'address',''),(s.p->>'latitude')::numeric(10,8),(s.p->>'longitude')::numeric(11,8),s.p->>'stop_kind')''').rowcount
        conn.execute(f'''UPDATE stops c SET source_snapshot={STOP_FIELDS} FROM stage s
          WHERE s.kind='stop' AND c.source_key=s.source_key AND c.source_snapshot IS DISTINCT FROM {STOP_FIELDS} AND (c.name,coalesce(c.address,''),c.latitude,c.longitude,c.city_id,c.timezone,c.stop_kind)
          IS NOT DISTINCT FROM (s.p->>'name',coalesce(s.p->>'address',''),(s.p->>'latitude')::numeric(10,8),(s.p->>'longitude')::numeric(11,8),s.city_id,s.p->>'timezone',s.p->>'stop_kind')''')
        conn.execute('''INSERT INTO catalog_source_links(source_key,kind,entity_id,owned)
          SELECT source_key,kind,entity_id,owned FROM stage ON CONFLICT(source_key) DO NOTHING''')
        stats['stops_without_city'] = conn.execute("SELECT count(*) FROM stage WHERE kind='stop' AND city_id IS NULL").fetchone()[0]
        stats['protected_or_changed_cities'] = conn.execute(f"SELECT count(*) FROM route_cities c JOIN stage s ON s.source_key=c.source_key WHERE s.kind='city' AND ({CITY_FIELDS}<>c.source_snapshot OR c.timezone IS DISTINCT FROM s.p->>'timezone')").fetchone()[0]
        stats['protected_or_changed_stops'] = conn.execute(f"SELECT count(*) FROM stops c JOIN stage s ON s.source_key=c.source_key WHERE s.kind='stop' AND ({STOP_FIELDS}<>c.source_snapshot OR (c.city_id,c.timezone) IS DISTINCT FROM (s.city_id,s.p->>'timezone'))").fetchone()[0]
        if not apply:
            conn.rollback()
        else:
            conn.commit()
            for table in ['route_cities', 'stops', 'catalog_search', 'route_regions']:
                conn.execute('ANALYZE ' + table)
    stats['applied'] = apply
    return dict(stats)


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--directory', required=True, type=Path)
    parser.add_argument('--apply', action='store_true')
    args = parser.parse_args()
    try:
        print(json.dumps(run(args.directory, args.apply), ensure_ascii=False))
    except Exception as error:
        # Driver errors can embed connection details or source records.
        print(json.dumps({'error': type(error).__name__, 'message': 'Import rolled back; check source/schema and database connectivity.'}))
        raise SystemExit(1)
