"""A manual city mapping must survive a subsequent source refresh."""
import os
import unittest

import psycopg
import test_catalog
from import_catalog import run as import_catalog


@unittest.skipUnless(os.getenv('TEST_DATABASE_URL'), 'disposable TEST_DATABASE_URL required')
class QualityMappingImportTests(unittest.TestCase):
    setUp = test_catalog.ImportTests.setUp
    tearDown = test_catalog.ImportTests.tearDown
    write = test_catalog.ImportTests.write

    def test_manual_mapping_is_protected_and_snapshot_unchanged(self):
        self.rows[2]['city_source_key'] = None
        self.write()
        import_catalog(self.directory, True)
        with psycopg.connect(self.url) as db:
            city = db.execute("SELECT id FROM route_cities WHERE source_key='fixture:city:1'").fetchone()[0]
            before = db.execute("SELECT source_snapshot FROM stops WHERE source_key='fixture:stop:1'").fetchone()[0]
            db.execute("UPDATE stops SET city_id=%s WHERE source_key='fixture:stop:1'", (city,))
        self.rows[2]['address'] = 'Обновлённый адрес источника'
        self.write()
        result = import_catalog(self.directory, True)
        self.assertEqual(result['protected_or_changed_stops'], 1)
        with psycopg.connect(self.url) as db:
            actual = db.execute("SELECT city_id,source_snapshot,address FROM stops WHERE source_key='fixture:stop:1'").fetchone()
        self.assertEqual(actual[0], city)
        self.assertEqual(actual[1], before)
        self.assertEqual(actual[2], 'Тестовый адрес')


if __name__ == '__main__':
    unittest.main()
