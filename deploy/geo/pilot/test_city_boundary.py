import importlib.util
import hashlib
import json
from pathlib import Path
import tempfile
import unittest

from shapely.geometry import LineString, MultiPolygon, Polygon

spec = importlib.util.spec_from_file_location('city_boundary',Path(__file__).with_name('city-boundary.py'))
boundary = importlib.util.module_from_spec(spec)
spec.loader.exec_module(boundary)

class CityBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.outer = [(60,56),(61,56),(61,57),(60,57),(60,56)]
        self.hole = [(60.4,56.4),(60.6,56.4),(60.6,56.6),(60.4,56.6),(60.4,56.4)]
        self.geometry = boundary.assemble([LineString(self.outer)],[LineString(self.hole)])

    def place(self,lon=60.2,lat=56.2,city=None,way=False):
        return {'centroid':[lon,lat],'object_type':'W' if way else 'N',
            'address':{'city':city} if city else {},
            'extra':{'coordinate_method':'way_bbox_center' if way else 'osm_node'}}

    def test_hole_and_boundaries_remain_excluded(self):
        self.assertEqual(len(self.geometry.geoms[0].interiors),1)
        for point in [(60.5,56.5),(60.4,56.5),(60,56.2),(62,56.2)]:
            self.assertEqual(boundary.eligibility(self.place(*point),self.geometry,'Екатеринбург'),'outside_or_boundary')

    def test_margin_and_valid_node(self):
        self.assertEqual(boundary.eligibility(self.place(60.0001),self.geometry,'Екатеринбург'),'boundary_margin')
        self.assertEqual(boundary.eligibility(self.place(),self.geometry,'Екатеринбург'),'enrich')

    def test_explicit_cities_and_uncertain_ways(self):
        for city,reason in [('Берёзовский','conflicting_city'),('Екатеринбург','explicit_city')]:
            self.assertEqual(boundary.eligibility(self.place(city=city),self.geometry,'Екатеринбург'),reason)
        self.assertEqual(boundary.eligibility(self.place(way=True),self.geometry,'Екатеринбург'),'uncertain_way_or_coordinate')

    def test_unclosed_and_uncontained_hole_fail_closed(self):
        with self.assertRaises(ValueError):boundary.assemble([LineString(self.outer[:-1])],[])
        with self.assertRaises(ValueError):boundary.assemble([LineString(self.outer)],[LineString([(62,56),(63,56),(63,57),(62,57),(62,56)])])

    def test_enrichment_copy_preserves_explicit_and_provenance(self):
        from shapely.geometry import mapping
        with tempfile.TemporaryDirectory() as temp:
            source=Path(temp)/'source.jsonl';output=Path(temp)/'new.jsonl'
            places=[self.place(),self.place(city='Берёзовский'),self.place(60.5,56.5),self.place(way=True)]
            original=json.dumps({'type':'Place','content':places},ensure_ascii=False)+'\n'
            source.write_text(original)
            feature={'geometry':mapping(self.geometry),'properties':{'tags':{'name':'Екатеринбург'},'source_sha256':'abc'}}
            checksum=hashlib.sha256(boundary.feature_bytes(feature)).hexdigest()
            counts=boundary.enrich(source,output,feature,checksum)
            self.assertEqual(source.read_text(),original)
            rows=json.loads(output.read_text())['content']
            self.assertEqual(rows[0]['address']['city'],'Екатеринбург')
            self.assertEqual(rows[0]['extra']['city_provenance']['relation_id'],6564910)
            self.assertEqual(rows[1]['address']['city'],'Берёзовский')
            self.assertNotIn('city',rows[2]['address'])
            self.assertNotIn('city',rows[3]['address'])
            self.assertEqual(counts['enrich'],1)
            with self.assertRaises(FileExistsError):boundary.enrich(source,output,feature,checksum)
            with self.assertRaises(ValueError):boundary.enrich(source,Path(temp)/'tampered.jsonl',feature,'wrong')
            self.assertFalse((Path(temp)/'tampered.jsonl').exists())

if __name__=='__main__':unittest.main()
