import datetime as dt
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest
import uuid
from zoneinfo import ZoneInfo
import psycopg
from psycopg.conninfo import make_conninfo
from prepare import prepare, sha256
from import_catalog import run as import_catalog, validate
from demo import run as demo


class GeometryTests(unittest.TestCase):
    def test_full_russian_timezone_names(self):
        from timezonefinder import TimezoneFinder
        tf=TimezoneFinder()
        for lon,lat,expected in [(37.617,55.755,'Europe/Moscow'),(60.6,56.8,'Asia/Yekaterinburg'),(131.886,43.115,'Asia/Vladivostok'),(158.65,53.02,'Asia/Kamchatka')]:
            self.assertEqual(tf.timezone_at(lng=lon,lat=lat),expected)

    def test_geometry_hole_and_ambiguous_city_mapping(self):
        # A real geometry fixture tests point-in-polygon with a hole. The point
        # in the hole must not become a stop of the enclosing settlement.
        xml='''<osm version="0.6">
        <node id="1" lon="60" lat="56"/><node id="2" lon="62" lat="56"/><node id="3" lon="62" lat="58"/><node id="4" lon="60" lat="58"/>
        <node id="5" lon="60.2" lat="56.2"/><node id="6" lon="61.8" lat="56.2"/><node id="7" lon="61.8" lat="57.8"/><node id="8" lon="60.2" lat="57.8"/>
        <node id="9" lon="60.8" lat="56.8"/><node id="10" lon="61.2" lat="56.8"/><node id="11" lon="61.2" lat="57.2"/><node id="12" lon="60.8" lat="57.2"/>
        <node id="20" lon="60.5" lat="56.5"><tag k="place" v="city"/><tag k="name" v="Тестоград"/></node>
        <node id="21" lon="60.6" lat="56.6"><tag k="highway" v="bus_stop"/><tag k="name" v="Внутри"/></node>
        <node id="22" lon="61" lat="57"><tag k="highway" v="bus_stop"/><tag k="name" v="В отверстии"/></node>
        <way id="100"><nd ref="1"/><nd ref="2"/><nd ref="3"/><nd ref="4"/><nd ref="1"/></way>
        <way id="101"><nd ref="5"/><nd ref="6"/><nd ref="7"/><nd ref="8"/><nd ref="5"/></way>
        <way id="102"><nd ref="9"/><nd ref="10"/><nd ref="11"/><nd ref="12"/><nd ref="9"/></way>
        <relation id="200"><member type="way" ref="100" role="outer"/><tag k="type" v="multipolygon"/><tag k="boundary" v="administrative"/><tag k="admin_level" v="4"/><tag k="ISO3166-2" v="RU-TEST"/><tag k="name" v="Тестовая область"/></relation>
        <relation id="201"><member type="way" ref="101" role="outer"/><member type="way" ref="102" role="inner"/><tag k="type" v="multipolygon"/><tag k="place" v="city"/><tag k="name" v="Тестоград"/></relation></osm>'''
        with tempfile.TemporaryDirectory() as directory:
            source=Path(directory)/'fixture.osm';source.write_text(xml)
            output=Path(directory)/'result';prepare(source,output,'fixture://geometry')
            rows=[json.loads(line) for line in (output/'catalog.jsonl').read_text().splitlines()]
            stops={r['name']:r for r in rows if r['kind']=='stop'}
            self.assertEqual(stops['Внутри']['city_source_key'],'osm:n:20')
            self.assertIsNone(stops['В отверстии']['city_source_key'])
            self.assertEqual(sum(r['kind']=='city' for r in rows),1)

    def test_invalid_coordinates_and_timezone_are_rejected(self):
        base={'kind':'city','source_key':'fixture:city','name':'Город','region':'Область','latitude':56,'longitude':60,'timezone':'Asia/Yekaterinburg'}
        for patch in [{'latitude':float('nan')},{'longitude':181},{'timezone':'No/SuchZone'},{'name':''}]:
            with self.assertRaises(ValueError):validate(base|patch)


@unittest.skipUnless(os.getenv('TEST_DATABASE_URL'),'set TEST_DATABASE_URL for disposable PostgreSQL checks')
class ImportTests(unittest.TestCase):
    def setUp(self):
        self.original=os.environ.get('CATALOG_DATABASE_URL')
        self.schema='catalog_py_'+uuid.uuid4().hex
        self.admin=psycopg.connect(os.environ['TEST_DATABASE_URL'],autocommit=True)
        self.admin.execute('CREATE SCHEMA '+self.schema)
        self.url=make_conninfo(os.environ['TEST_DATABASE_URL'],options=f'-c search_path={self.schema},public')
        os.environ['CATALOG_DATABASE_URL']=self.url
        with psycopg.connect(self.url) as db:
            # Database-scoped extensions always belong to public, never a
            # disposable schema that can disappear under another test process.
            for path in sorted((Path(__file__).resolve().parents[2]/'db/migrations').glob('*.sql')):
                sql=path.read_text().split('-- +goose Down')[0].replace('CREATE EXTENSION "uuid-ossp";','CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;')
                db.execute(sql)
        self.temp=tempfile.TemporaryDirectory();self.directory=Path(self.temp.name)
        self.rows=[{'kind':'city','source_key':f'fixture:city:{n}','name':f'Тестоград{n}','region':'Тестовая область','timezone':'Asia/Yekaterinburg','latitude':56.8,'longitude':60.6+n,'popularity':100} for n in (1,2)]
        self.rows += [{'kind':'stop','source_key':f'fixture:stop:{n}','name':f'Тестовая остановка{n}','address':'Тестовый адрес','timezone':None,'latitude':56.8,'longitude':60.6+n,'city_source_key':f'fixture:city:{n}','stop_kind':'bus_station'} for n in (1,2)]
        self.write()

    def tearDown(self):
        if self.original is None:os.environ.pop('CATALOG_DATABASE_URL',None)
        else:os.environ['CATALOG_DATABASE_URL']=self.original
        self.admin.execute('DROP SCHEMA '+self.schema+' CASCADE');self.admin.close();self.temp.cleanup()

    def write(self):
        data=self.directory/'catalog.jsonl';data.write_text(''.join(json.dumps(r,ensure_ascii=False)+'\n' for r in self.rows))
        (self.directory/'manifest.json').write_text(json.dumps({'format_version':1,'catalog_sha256':sha256(data),'cities':2,'stops':2}))

    def test_import_idempotence_manual_edits_updates_and_dry_run(self):
        self.assertFalse(import_catalog(self.directory)['applied'])
        with psycopg.connect(self.url) as db:self.assertEqual(db.execute('SELECT count(*) FROM stops').fetchone()[0],0)
        first=import_catalog(self.directory,True);self.assertEqual(first['cities_created'],2);self.assertEqual(first['stops_created'],2)
        again=import_catalog(self.directory,True);self.assertEqual(again['cities_created'],0);self.assertEqual(again['stops_updated'],0)
        with psycopg.connect(self.url) as db:db.execute("UPDATE stops SET name='Ручная правка' WHERE source_key='fixture:stop:1'")
        self.rows[2]['name']='Обновление источника';self.rows[3]['address']='Новый адрес';self.write()
        changed=import_catalog(self.directory,True);self.assertEqual(changed['protected_or_changed_stops'],1);self.assertEqual(changed['stops_updated'],1)
        with psycopg.connect(self.url) as db:
            self.assertEqual(db.execute("SELECT name FROM stops WHERE source_key='fixture:stop:1'").fetchone()[0],'Ручная правка')
            self.assertEqual(db.execute("SELECT address FROM stops WHERE source_key='fixture:stop:2'").fetchone()[0],'Новый адрес')
        self.rows.append(self.rows[0]);self.write()
        with self.assertRaises(Exception):import_catalog(self.directory,True)
        with psycopg.connect(self.url) as db:self.assertEqual(db.execute('SELECT count(*) FROM stops').fetchone()[0],2)

    def test_demo_repeats_overlap_dates_money_and_resource_conflicts(self):
        import_catalog(self.directory,True)
        start=dt.date(2026,11,10)
        first=demo(start,3,2,2,'test-demo',True);self.assertEqual(first['created_trips'],12)
        again=demo(start,3,2,2,'test-demo',True);self.assertEqual(again['created_trips'],0)
        overlap=demo(start+dt.timedelta(days=2),3,2,2,'test-demo',True);self.assertEqual(overlap['created_trips'],8)
        with self.assertRaises(ValueError):demo(start,3,4,2,'test-demo',True)
        with psycopg.connect(self.url) as db:
            self.assertEqual(db.execute('SELECT count(*) FROM trips WHERE NOT is_demo').fetchone()[0],0)
            self.assertEqual(db.execute('SELECT count(*) FROM trips a JOIN trips b ON a.id<b.id AND a.bus_id=b.bus_id AND a.start_time<b.end_time AND b.start_time<a.end_time').fetchone()[0],0)
            self.assertEqual(db.execute('SELECT count(*) FROM trips a JOIN trips b ON a.id<b.id AND a.driver_id=b.driver_id AND a.start_time<b.end_time AND b.start_time<a.end_time').fetchone()[0],0)
            self.assertEqual(db.execute('SELECT count(*) FROM trips t WHERE base_price_minor<>(SELECT sum(price_to_next_minor) FROM trip_stops s WHERE s.trip_id=t.id)').fetchone()[0],0)
            self.assertEqual(db.execute('SELECT count(*) FROM users').fetchone()[0],0)


if __name__=='__main__':unittest.main()
