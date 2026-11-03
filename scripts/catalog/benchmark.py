#!/usr/bin/env python3
"""Repeatable local measurements; saves counts, latency and EXPLAIN evidence.

Run against the explicitly supplied disposable database/API. Numbers are machine
and cache dependent, and are not a production SLA.
"""
import argparse
import collections
import concurrent.futures
import datetime as dt
import json
import os
import statistics
import time
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path
import psycopg


def run(api, output, repeats=12, concurrency=4):
    with psycopg.connect(os.environ['CATALOG_DATABASE_URL']) as db:
        row=db.execute('''SELECT a.stop_id,b.stop_id,s.city_id,s2.city_id,a.departure_time,c.timezone
          FROM trips t JOIN trip_stops a ON a.trip_id=t.id AND a.stop_order=1
          JOIN trip_stops b ON b.trip_id=t.id AND b.stop_order=(SELECT max(stop_order) FROM trip_stops WHERE trip_id=t.id)
          JOIN stops s ON s.id=a.stop_id JOIN stops s2 ON s2.id=b.stop_id JOIN route_cities c ON c.id=s.city_id
          WHERE t.is_demo AND a.departure_time>now() ORDER BY t.id LIMIT 1''').fetchone()
        if not row:
            raise ValueError('future demo timetable required')
        a,b,city,city2,instant,zone=row
        from zoneinfo import ZoneInfo
        day=instant.replace(tzinfo=dt.timezone.utc).astimezone(ZoneInfo(zone)).date().isoformat()
        month=day[:7]
        counts={table:db.execute('SELECT count(*) FROM '+table).fetchone()[0] for table in ['route_cities','stops','catalog_search','trips','trip_stops','bus','drivers']}
        sizes={r[0]:r[1] for r in db.execute("SELECT relname,pg_total_relation_size(oid) FROM pg_class WHERE relkind='r' AND relnamespace='public'::regnamespace")}
        total=db.execute('SELECT pg_database_size(current_database())').fetchone()[0]
    def url(path, **params):
        return path+'?'+urllib.parse.urlencode(params)
    cases={
        'city_full':url('/locations/suggest',q='Екатеринбург',limit=8),
        'city_prefix':url('/locations/suggest',q='Ека',limit=8),
        'city_one_letter':url('/locations/suggest',q='Е',limit=8),
        'city_typo':url('/locations/suggest',q='Екатеринбруг',limit=8),
        'region':url('/catalog/regions',q='Свердловская',limit=20),
        'stop':url('/locations/suggest',q='Автовокзал',limit=8),
        'destination':url('/locations/suggest',q='Челябинск',origin_id=f'city:{city}',limit=8),
        'stop_page':url('/catalog/stops',limit=30),
        'stop_city_page':url('/catalog/stops',city_id=city,limit=30),
        'search_stops':url('/trips/search',from_id=f'stop:{a}',to_id=f'stop:{b}',date=day,limit=20),
        'search_cities':url('/trips/search',from_id=f'city:{city}',to_id=f'city:{city2}',date=day,limit=20),
        'search_price':url('/trips/search',from_id=f'city:{city}',to_id=f'city:{city2}',date=day,limit=20,sort='price'),
        'calendar':url('/trips/calendar',from_id=f'city:{city}',to_id=f'city:{city2}',month=month),
    }
    results=collections.defaultdict(list)
    failures=[]
    def measure(item):
        key,path=item
        start=time.perf_counter()
        try:
            with urllib.request.urlopen(api+path,timeout=10) as response:
                value=json.load(response)
                if not isinstance(value,dict) or not isinstance(value.get('items',value.get('days')),list):
                    raise ValueError('invalid response')
                return key,(time.perf_counter()-start)*1000,None
        except Exception as error:
            return key,(time.perf_counter()-start)*1000,type(error).__name__
    # One warm-up per case is separate from concurrent measurements.
    warm=[measure(item) for item in cases.items()]
    with concurrent.futures.ThreadPoolExecutor(max_workers=concurrency) as pool:
        for key,elapsed,error in pool.map(measure,list(cases.items())*repeats):
            results[key].append(elapsed)
            if error:failures.append({'case':key,'error':error})
    summary={key:{'samples':len(values),'p50_ms':round(statistics.median(values),2),'p95_ms':round(sorted(values)[max(0,int(len(values)*.95)-1)],2),'max_ms':round(max(values),2)} for key,values in results.items()}
    plans={}
    with psycopg.connect(os.environ['CATALOG_DATABASE_URL']) as db:
        queries={
            'prefix':("SELECT entity_id FROM catalog_search WHERE kind='city' AND name_norm LIKE 'ека%' ORDER BY name_norm,entity_id LIMIT 64",()),
            'fuzzy':("SELECT entity_id FROM catalog_search ORDER BY name_norm <-> 'екатеринбруг' LIMIT 64",()),
            'departure':("SELECT trip_id FROM trip_stops WHERE stop_id=%s AND departure_time>=%s AND departure_time<%s ORDER BY departure_time LIMIT 21",(a,instant.replace(hour=0,minute=0,second=0),instant.replace(hour=0,minute=0,second=0)+dt.timedelta(days=1)))
        }
        for key,(sql,args) in queries.items():
            plans[key]=db.execute('EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) '+sql,args or None).fetchone()[0]
        sql_stats=db.execute("SELECT calls,rows,round(mean_exec_time::numeric,3),left(query,150) FROM pg_stat_statements WHERE dbid=(SELECT oid FROM pg_database WHERE datname=current_database()) AND query LIKE 'WITH origin_legs AS MATERIALIZED%' ORDER BY calls DESC LIMIT 5").fetchall()
    report={'counts':counts,'relation_bytes':sizes,'database_bytes':total,'concurrency':concurrency,'warmup':warm,'latency':summary,'failures':failures,'plans':plans,'suggest_statements':sql_stats,'limits':'local warm-cache measurements, not production SLA'}
    output.write_text(json.dumps(report,ensure_ascii=False,indent=2,default=str)+'\n')
    print(json.dumps({'counts':counts,'database_bytes':total,'latency':summary,'failures':failures},ensure_ascii=False))
    if failures or any(error for _,_,error in warm):
        raise SystemExit(1)


if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--api',default='http://127.0.0.1:8083/api/v1');p.add_argument('--output',required=True,type=Path);p.add_argument('--repeats',default=12,type=int);p.add_argument('--concurrency',default=4,type=int);a=p.parse_args()
    run(a.api,a.output,a.repeats,a.concurrency)
