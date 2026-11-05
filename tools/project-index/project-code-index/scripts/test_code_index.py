import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest

SCRIPT=Path(__file__).with_name('code_index.py')
spec=importlib.util.spec_from_file_location('code_index',SCRIPT)
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)


class IndexBehavior(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.root=Path(self.temp.name)
        self.repo=self.root/'source';self.repo.mkdir();(self.root/'.agents').mkdir()
        self.config={'version':1,'repos':[{'name':'code','path':'source','git':True}]}
        (self.root/'.agents/project-index.json').write_text(json.dumps(self.config))
        self.git('init','-q');self.git('config','user.email','index@example.test');self.git('config','user.name','Index test')
        (self.repo/'service.py').write_text('def reserve_place():\n    """Удержание места пассажира."""\n    return 1\n')
        self.git('add','service.py');self.git('commit','-qm','initial fixture')
        self.index=module.Index(self.root)

    def tearDown(self):self.temp.cleanup()
    def git(self,*args):return module.git(self.repo,*args)

    def test_incremental_edit_rename_delete_and_same_stat(self):
        self.assertFalse(self.index.status()['fresh'])
        self.assertEqual(self.index.refresh()['updated'],1)
        self.assertEqual(self.index.refresh()['updated'],0)
        self.assertTrue(self.index.status()['fresh'])
        path=self.repo/'service.py';stamp=path.stat()
        path.write_text(path.read_text().replace('return 1','return 2'))
        os.utime(path,ns=(stamp.st_atime_ns,stamp.st_mtime_ns))
        self.assertFalse(self.index.status()['fresh'])
        self.assertEqual(self.index.refresh()['updated'],1)
        self.assertIn('return 2',self.index.excerpt('code','service.py')['excerpt'])
        path.rename(self.repo/'renamed.py')
        result=self.index.refresh();self.assertEqual((result['updated'],result['removed']),(1,1))
        (self.repo/'renamed.py').unlink();self.assertEqual(self.index.refresh()['removed'],1)

    def test_branch_and_config_changes_invalidate(self):
        self.index.refresh();self.git('checkout','-qb','other')
        self.assertFalse(self.index.status()['fresh'])
        self.index.refresh();self.assertTrue(self.index.status()['fresh'])
        self.config['repos'][0]['name']='renamed'
        (self.root/'.agents/project-index.json').write_text(json.dumps(self.config))
        newer=module.Index(self.root);self.assertFalse(newer.status()['fresh'])
        newer.refresh();self.assertTrue(newer.status()['fresh'])

    def test_exclusions_are_applied_even_to_tracked_files(self):
        outside=self.root/'outside.py';outside.write_text('OUTSIDE_MARKER=1')
        (self.repo/'escape.py').symlink_to(outside)
        for name in ['.env','.env.seed.local','credentials.json','cookie.txt','backup.dump','node_modules/dependency.py']:
            path=self.repo/name;path.parent.mkdir(exist_ok=True,parents=True);path.write_text('PRIVATE_MARKER_123')
        self.git('add','-f','.')
        self.index.refresh()
        self.assertEqual(self.index.search('PRIVATE_MARKER_123'),[])
        self.assertEqual(self.index.search('OUTSIDE_MARKER'),[])
        for name in ['.env','credentials.json','escape.py','../outside.py']:
            with self.assertRaises(ValueError):self.index.excerpt('code',name)
        self.assertEqual(self.index.cache.stat().st_mode&0o777,0o600)

    def test_credentials_redacted_in_storage_search_and_excerpt(self):
        raw='password: UNQUOTED_FAKE_12345\napi_key: ANOTHER_FAKE_12345\nsecret = "QUOTED_FAKE_12345"\nurl = "postgres://user:URL_FAKE_12345@localhost/db"\n'
        (self.repo/'config.yaml').write_text(raw)
        self.index.refresh();out=self.index.excerpt('code','config.yaml')['excerpt']
        for marker in ['UNQUOTED_FAKE_12345','ANOTHER_FAKE_12345','QUOTED_FAKE_12345','URL_FAKE_12345']:
            self.assertNotIn(marker,out);self.assertEqual(self.index.search(marker),[])
        with self.index.connect() as db:
            content=db.execute("SELECT content FROM search WHERE path='config.yaml'").fetchone()[0]
            self.assertNotIn('UNQUOTED_FAKE_12345',content)

    def test_long_token_linear_and_bounded(self):
        raw='LongNeedle='+'A'*50000
        (self.repo/'long.py').write_text(raw)
        started=time.monotonic();self.index.refresh()
        self.assertLess(time.monotonic()-started,4)
        hits=self.index.search('LongNeedle',limit=1)
        self.assertLessEqual(len(hits[0]['excerpt']),1200)

    def test_symbols_excerpts_fts_escaping_and_binary_metadata(self):
        (self.repo/'image.png').write_bytes(b'\0binary fixture')
        self.index.refresh()
        hit=self.index.symbol('reserve_place')[0];self.assertEqual(hit['line'],1)
        self.assertTrue(self.index.search('Удержание'))
        self.assertIsInstance(self.index.search('" OR : ()*'),list)
        self.assertLessEqual(len(self.index.excerpt('code','service.py',symbol='reserve_place',max_chars=40)['excerpt']),40)
        with self.assertRaises(ValueError):self.index.excerpt('code','image.png')

    def test_redaction_preserves_source_line_numbers(self):
        raw='before\n-----BEGIN PRIVATE KEY-----\nFAKE_PRIVATE_DATA\n-----END PRIVATE KEY-----\nafter\n'
        sanitized=module.redact(raw)
        self.assertEqual(raw.count('\n'),sanitized.count('\n'))
        self.assertNotIn('FAKE_PRIVATE_DATA',sanitized)

    def test_cli_context_budget_and_invalid_limits(self):
        self.index.refresh()
        run=subprocess.run([sys.executable,str(SCRIPT),'--root',str(self.root),'context','reserve place','--max-chars','1000'],capture_output=True,text=True,timeout=5)
        self.assertEqual(run.returncode,0,run.stderr);self.assertLessEqual(len(run.stdout),1000)
        self.assertIn('revisions',json.loads(run.stdout))
        bad=subprocess.run([sys.executable,str(SCRIPT),'--root',str(self.root),'search','test','--limit','31'],capture_output=True,text=True,timeout=5)
        self.assertNotEqual(bad.returncode,0)

    def test_changed_processor_invalidates_cached_documents(self):
        self.index.refresh()
        with self.index.connect() as db:db.execute("UPDATE meta SET value='previous-processor' WHERE key='processor'")
        self.assertFalse(self.index.status()['fresh'])
        self.assertEqual(self.index.refresh()['updated'],1)
        self.assertTrue(self.index.status()['fresh'])


if __name__=='__main__':unittest.main()
