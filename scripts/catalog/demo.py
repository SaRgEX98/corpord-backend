#!/usr/bin/env python3
"""Generate an explicitly fictional, repeatable 30-day timetable in bulk.

Every pair/slot has its own bus, an outbound and a return driver. A round trip
finishes before that bus's next day departure. No account or payment is created.
"""
import argparse
import datetime as dt
import hashlib
import json
import math
import os
from decimal import Decimal
from zoneinfo import ZoneInfo
import psycopg
from psycopg.types.json import Jsonb


def distance(a, b):
    lat1, lat2 = math.radians(a[4]), math.radians(b[4])
    dlat, dlon = lat2 - lat1, math.radians(b[5] - a[5])
    value = math.sin(dlat / 2) ** 2 + math.cos(lat1) * math.cos(lat2) * math.sin(dlon / 2) ** 2
    return 6371 * 2 * math.asin(min(1, math.sqrt(value)))


def run(start, days, max_directions, departures, dataset, apply=False):
    if not 1 <= days <= 31 or not 2 <= max_directions <= 10000 or max_directions % 2 or departures not in (1, 2):
        raise ValueError('days 1..31, even directions 2..10000, departures 1 or 2')
    if not dataset or len(dataset) > 60 or not all(c.isalnum() or c in '-_' for c in dataset):
        raise ValueError('invalid dataset identifier')
    with psycopg.connect(os.environ['CATALOG_DATABASE_URL']) as conn:
        conn.execute("SET LOCAL lock_timeout='5s'")
        conn.execute('SELECT pg_advisory_xact_lock(1129271890)')
        conn.execute('''CREATE TABLE IF NOT EXISTS catalog_demo_datasets(
          dataset text PRIMARY KEY,config jsonb NOT NULL);
          CREATE TABLE IF NOT EXISTS catalog_demo_pairs(
          dataset text NOT NULL REFERENCES catalog_demo_datasets(dataset),pair_key text NOT NULL,slot int NOT NULL,
          stops int[] NOT NULL,minutes int[] NOT NULL,fares int[] NOT NULL,
          bus_id int NOT NULL REFERENCES bus(id),out_driver int NOT NULL REFERENCES drivers(id),back_driver int NOT NULL REFERENCES drivers(id),
          timezone text NOT NULL,PRIMARY KEY(dataset,pair_key,slot));
          CREATE TABLE IF NOT EXISTS catalog_demo_days(
          dataset text NOT NULL REFERENCES catalog_demo_datasets(dataset),day date NOT NULL,
          PRIMARY KEY(dataset,day));
          CREATE TABLE IF NOT EXISTS catalog_demo_trips(
          dataset text NOT NULL REFERENCES catalog_demo_datasets(dataset),trip_id int PRIMARY KEY REFERENCES trips(id) ON DELETE CASCADE);''')
        config = {'version': 1, 'max_directions': max_directions, 'departures': departures, 'max_distance_km': 300}
        saved = conn.execute('SELECT config FROM catalog_demo_datasets WHERE dataset=%s', (dataset,)).fetchone()
        if saved and saved[0] != config:
            raise ValueError('dataset configuration is immutable; choose a new identifier')
        if not saved:
            conn.execute('INSERT INTO catalog_demo_datasets VALUES(%s,%s)', (dataset, Jsonb(config)))
            cities = conn.execute('''SELECT c.id,c.name,c.region,c.timezone,c.latitude,c.longitude,s.id,c.popularity
              FROM route_cities c JOIN LATERAL(SELECT id FROM stops WHERE city_id=c.id AND latitude IS NOT NULL
              AND longitude IS NOT NULL ORDER BY (stop_kind='bus_station') DESC,id LIMIT 1) s ON true
              WHERE c.latitude IS NOT NULL AND c.longitude IS NOT NULL AND c.timezone IS NOT NULL
              ORDER BY c.popularity DESC,c.id''').fetchall()
            pairs = {}
            # Round-robin nearest neighbours covers small cities before adding
            # further links for hubs. No national complete graph is materialized.
            neighbours = {a[0]: sorted([(distance(a,b),b) for b in cities if a[0]!=b[0]], key=lambda v:(v[0],v[1][0]))[:12] for a in cities}
            for rank in range(12):
                for a in cities:
                    if len(pairs) >= max_directions // 2:
                        break
                    if len(neighbours[a[0]]) <= rank:
                        continue
                    km, b = neighbours[a[0]][rank]
                    if not 15 <= km <= 300:
                        continue
                    key = tuple(sorted((a[0], b[0])))
                    if key in pairs:
                        continue
                    # Optional intermediate towns must lie close to the corridor;
                    # these are demo stops, not a claim about a real bus itinerary.
                    middle = []
                    for c in {v[1][0]:v[1] for v in neighbours[a[0]]+neighbours[b[0]]}.values():
                        if c[0] in key:
                            continue
                        ac, cb = distance(a,c), distance(c,b)
                        if ac > 10 and cb > 10 and ac + cb < km * 1.08:
                            middle.append((ac,c))
                    middle = [v[1] for v in sorted(middle,key=lambda v:v[0])[:2]]
                    points = [a] + middle + [b]
                    minutes = [max(20,math.ceil(distance(x,y)*1.35/50*60)) for x,y in zip(points,points[1:])]
                    if 2 * (sum(minutes) + 10 * len(middle)) + 60 >= 22 * 60:
                        continue
                    fares = [max(15000,round(distance(x,y)*1.35*350)) for x,y in zip(points,points[1:])]
                    pairs[key] = (points,minutes,fares)
            if not pairs:
                raise ValueError('no eligible mapped city pairs')
            # Reserved names are adopted only as named status vocabulary, never
            # existing buses or drivers. Prefix collisions roll back all work.
            def named(table, value):
                row = conn.execute(f'SELECT id FROM {table} WHERE name=%s ORDER BY id LIMIT 1',(value,)).fetchone()
                return row[0] if row else conn.execute(f'INSERT INTO {table}(name) VALUES(%s) RETURNING id',(value,)).fetchone()[0]
            category = named('bus_categories', 'Демонстрационный автобус')
            bus_status = named('bus_statuses', 'Демонстрационный')
            driver_status = named('driver_status', 'Демонстрационный')
            conn.execute("""CREATE TEMP TABLE demo_resources(
              dataset text,pair_key text,slot int,stops int[],minutes int[],fares int[],timezone text,tag text,
              bus_id int DEFAULT nextval('bus_id_seq'),out_driver int DEFAULT nextval('drivers_id_seq'),
              back_driver int DEFAULT nextval('drivers_id_seq')) ON COMMIT DROP""")
            with conn.cursor().copy('COPY demo_resources(dataset,pair_key,slot,stops,minutes,fares,timezone,tag) FROM STDIN') as copy:
                for key,(points,minutes,fares) in sorted(pairs.items()):
                    pair = '-'.join(map(str,key))
                    for slot in range(departures):
                        tag = hashlib.sha256(f'{dataset}:{pair}:{slot}'.encode()).hexdigest()[:18]
                        copy.write_row((dataset,pair,slot,[p[6] for p in points],minutes,fares,points[0][3],tag))
            conn.execute("""INSERT INTO bus(id,license_plate,brand,capacity,category_id,status_id)
              SELECT bus_id,'DEMO-'||tag,'Тестовый автобус',45,%s,%s FROM demo_resources""",(category,bus_status))
            conn.execute("""INSERT INTO drivers(id,first_name,last_name,middle_name,phone_number,status)
              SELECT out_driver,'Тестовый','Водитель','туда','DEMO-NO-PHONE',%s FROM demo_resources UNION ALL
              SELECT back_driver,'Тестовый','Водитель','обратно','DEMO-NO-PHONE',%s FROM demo_resources""",(driver_status,driver_status))
            conn.execute("""INSERT INTO catalog_demo_pairs(dataset,pair_key,slot,stops,minutes,fares,bus_id,out_driver,back_driver,timezone)
              SELECT dataset,pair_key,slot,stops,minutes,fares,bus_id,out_driver,back_driver,timezone FROM demo_resources""")
        pairs = conn.execute('SELECT pair_key,slot,stops,minutes,fares,bus_id,out_driver,back_driver,timezone FROM catalog_demo_pairs WHERE dataset=%s ORDER BY pair_key,slot',(dataset,)).fetchall()
        expected=conn.execute('SELECT count(*) FROM catalog_demo_days WHERE dataset=%s',(dataset,)).fetchone()[0]*len(pairs)*2
        actual=conn.execute('SELECT count(*) FROM catalog_demo_trips WHERE dataset=%s',(dataset,)).fetchone()[0]
        if expected!=actual:
            raise ValueError('a tracked demo trip was deleted; restore it or use a new dataset')
        completed = {r[0] for r in conn.execute('SELECT day FROM catalog_demo_days WHERE dataset=%s',(dataset,))}
        missing = [start+dt.timedelta(days=d) for d in range(days) if start+dt.timedelta(days=d) not in completed]
        conn.execute('''CREATE TEMP TABLE demo_trips(id int,bus_id int,driver_id int,start_time timestamp,end_time timestamp,base_price numeric) ON COMMIT DROP;
          CREATE TEMP TABLE demo_stops(trip_id int,stop_id int,arrival_time timestamp,departure_time timestamp,stop_order int,price_to_next numeric) ON COMMIT DROP''')
        count = len(missing) * len(pairs) * 2
        ids = iter(r[0] for r in conn.execute("SELECT nextval('trips_id_seq') FROM generate_series(1,%s)",(count,)).fetchall())
        trips, stops = [], []
        for day in missing:
            for pair,slot,points,minutes,fares,bus,out_driver,back_driver,zone in pairs:
                departure = dt.datetime.combine(day,dt.time(8+slot*2),ZoneInfo(zone)).astimezone(dt.timezone.utc).replace(tzinfo=None)
                for reverse in (False,True):
                    trip_id = next(ids)
                    ps, ms, fs = (list(reversed(points)),list(reversed(minutes)),list(reversed(fares))) if reverse else (points,minutes,fares)
                    instant = departure
                    for order,stop in enumerate(ps):
                        arrival = instant
                        leave = instant + (dt.timedelta(minutes=10) if 0<order<len(ps)-1 else dt.timedelta())
                        stops.append((trip_id,stop,arrival,leave,order+1,Decimal(fs[order])/100 if order<len(fs) else 0))
                        if order<len(ms):
                            instant = leave+dt.timedelta(minutes=ms[order])
                    trips.append((trip_id,bus,back_driver if reverse else out_driver,departure,instant,Decimal(sum(fs))/100))
                    departure = instant+dt.timedelta(minutes=60)
                # Bound process memory independently of timetable length.
                if len(trips)>=2000:
                    flush(conn,trips,stops)
        flush(conn,trips,stops)
        for resource in ('bus_id','driver_id'):
            conflict=conn.execute(f"SELECT 1 FROM demo_trips d JOIN trips t ON t.{resource}=d.{resource} AND t.start_time<d.end_time AND d.start_time<t.end_time LIMIT 1").fetchone()
            if conflict:
                raise ValueError('existing bus/driver schedule conflicts with demo dates')
        conn.execute('''INSERT INTO trips(id,bus_id,driver_id,start_time,end_time,status,base_price,is_demo)
          SELECT id,bus_id,driver_id,start_time,end_time,'scheduled',base_price,true FROM demo_trips''')
        conn.execute('''INSERT INTO trip_stops(trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next)
          SELECT trip_id,stop_id,arrival_time,departure_time,stop_order,price_to_next FROM demo_stops''')
        conn.execute('INSERT INTO catalog_demo_trips SELECT %s,id FROM demo_trips',(dataset,))
        conn.execute("INSERT INTO trip_status_history(trip_id,status) SELECT id,'scheduled' FROM demo_trips")
        for day in missing:
            conn.execute('INSERT INTO catalog_demo_days VALUES(%s,%s)',(dataset,day))
        stop_count = conn.execute('SELECT count(*) FROM demo_stops').fetchone()[0]
        if apply:
            conn.commit()
            conn.execute('ANALYZE trips; ANALYZE trip_stops')
        else:
            conn.rollback()
        return {'applied':apply,'directed_routes':len(pairs)//departures*2,'new_days':len(missing),'created_trips':count,'created_trip_stops':stop_count}


def flush(conn,trips,stops):
    if not trips:
        return
    with conn.cursor().copy('COPY demo_trips FROM STDIN') as copy:
        for row in trips:
            copy.write_row(row)
    with conn.cursor().copy('COPY demo_stops FROM STDIN') as copy:
        for row in stops:
            copy.write_row(row)
    trips.clear()
    stops.clear()


if __name__ == '__main__':
    p=argparse.ArgumentParser()
    p.add_argument('--start-date',type=dt.date.fromisoformat,default=dt.datetime.now(dt.timezone.utc).date()+dt.timedelta(days=1))
    p.add_argument('--days',type=int,default=30)
    p.add_argument('--directions',type=int,default=10000)
    p.add_argument('--departures',type=int,default=2)
    p.add_argument('--dataset',required=True)
    p.add_argument('--apply',action='store_true')
    a=p.parse_args()
    try:
        print(json.dumps(run(a.start_date,a.days,a.directions,a.departures,a.dataset,a.apply)))
    except Exception as error:
        print(json.dumps({'error':type(error).__name__,'message':'Demo generation rolled back; check schema, source mappings and dataset configuration.'}))
        raise SystemExit(1)
