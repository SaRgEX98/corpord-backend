#!/usr/bin/env python3
"""Local bounded multi-repository navigation; Python standard library only."""
import argparse
import ast
import contextlib
import hashlib
import json
import os
from pathlib import Path
import re
import sqlite3
import subprocess
import sys

VERSION=1
MAX_BYTES=1024*1024
DENY_DIRS={'.git','node_modules','.nuxt','.output','.nitro','.cache','.venv','venv',
           '__pycache__','dist','build','vendor','backups','test-results',
           'playwright-report','logs','corpord-catalog-data','corpord-geo-pilot-data'}
DENY_EXT={'.dump','.backup','.bak','.log','.sqlite','.sqlite3','.db','.pbf','.pmtiles',
          '.p12','.pfx','.pem','.key','.crt','.pyc'}
TEXT_EXT={'.go','.py','.ts','.tsx','.js','.jsx','.mjs','.cjs','.vue','.json','.yaml',
          '.yml','.toml','.md','.sql','.sh','.css','.scss','.html','.txt','.xml',
          '.proto','.mod','.sum','.lock','.conf','.ini'}
TEXT_NAMES={'Dockerfile','Makefile','.gitignore','.dockerignore','LICENSE','AGENTS.md'}


def digest(raw):return hashlib.sha256(raw).hexdigest()


def redact(text):
    text=re.sub(r'-----BEGIN [^-]*PRIVATE KEY[^-]*-----.*?(?:-----END [^-]+-----|\Z)',
                lambda m:'[REDACTED PRIVATE KEY]'+'\n'*m[0].count('\n'),text,flags=re.S)
    text=re.sub(r'\b(?:gh[pousr]_[A-Za-z0-9_]{15,}|github_pat_[A-Za-z0-9_]{15,}|sk-[A-Za-z0-9_-]{16,})\b','[REDACTED TOKEN]',text)
    text=re.sub(r'\beyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\b','[REDACTED JWT]',text)
    text=re.sub(r'(\b[a-z][a-z0-9+.-]*://[^\s/:@]+:)[^\s@]+(@)',r'\1[REDACTED]\2',text,flags=re.I)
    pattern=r'''(?<![\w-])((?:[\w-]{0,80}(?:password|passwd|secret|api[_-]?key|access[_-]?token|refresh[_-]?token)[\w-]{0,80})["']?\s*[:=]\s*)(["'])([^\n]*?)\2'''
    def literal(m):
        return m[0] if m[3].startswith('${') or len(m[3])<4 else m[1]+m[2]+'[REDACTED]'+m[2]
    text=re.sub(pattern,literal,text,flags=re.I)
    # Plain YAML and shell assignments may carry credentials without quotes.
    text=re.sub(r'(?im)^(\s*(?:export\s+)?[\w-]*(?:password|passwd|secret|api[_-]?key|access[_-]?token|refresh[_-]?token)[\w-]*\s*[:=]\s*)([^\n]+)',
                lambda m:m[1]+('[ENV REFERENCE]' if m[2].lstrip().startswith('${') else '[REDACTED]'),text)
    return text


def permitted(path):
    parts=Path(path).parts
    if not parts or any(p in DENY_DIRS or p in {'.','..'} for p in parts):return False
    if '.agents' in parts and 'cache' in parts:return False
    name=parts[-1].lower()
    if name=='.env' or name.startswith(('.env.','.env-')):return False
    if name in {'credentials','credentials.json','id_rsa','id_ed25519','.netrc','.npmrc','.pypirc'}:return False
    if any(word in name for word in ('cookie','credential','token-cache','secret-key')):return False
    return Path(name).suffix not in DENY_EXT


def git(root,*args):
    return subprocess.run(['git','-C',str(root),*args],capture_output=True,check=True).stdout.decode('utf-8','replace').rstrip('\n')


def symbols(text,path):
    found=[]
    if path.endswith('.py'):
        with contextlib.suppress(SyntaxError):
            for node in ast.walk(ast.parse(text)):
                if isinstance(node,(ast.ClassDef,ast.FunctionDef,ast.AsyncFunctionDef)):
                    found.append({'line':node.lineno,'name':node.name,'kind':type(node).__name__})
    patterns=[('go',r'^func\s+(?:\(([^)]*)\)\s*)?([A-Za-z_]\w*)'),
      ('js',r'^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function|class|interface|type|enum)\s+([\w$]+)'),
      ('binding',r'^\s*(?:export\s+)?(?:const|let)\s+([\w$]+)\s*='),
      ('sql',r'^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:UNIQUE\s+)?(?:TABLE|INDEX|FUNCTION|TRIGGER)\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w.]+)'),
      ('heading',r'^#{1,4}\s+(.+)')]
    ext=Path(path).suffix
    for number,line in enumerate(text.splitlines(),1):
        for kind,pattern in patterns:
            if kind=='go' and ext!='.go':continue
            if kind=='heading' and ext!='.md':continue
            if kind=='sql' and ext!='.sql':continue
            if kind in {'js','binding'} and ext not in {'.ts','.tsx','.js','.jsx','.mjs','.vue'}:continue
            m=re.match(pattern,line,re.I if kind=='sql' else 0)
            if m:
                name=m[2] if kind=='go' else m[1]
                if kind=='go' and m[1]:name=m[1].split()[-1].lstrip('*')+'.'+name
                found.append({'line':number,'name':name[:180],'kind':kind})
    return sorted(found,key=lambda x:(x['line'],x['name']))[:400]


def imports(text,path):
    values=[];ext=Path(path).suffix
    if ext=='.go':
        for block in re.findall(r'(?ms)^import\s+(\(.*?^\)|"[^"]+")',text):values.extend(re.findall(r'"([^"\n]+)"',block))
    elif ext in {'.ts','.tsx','.js','.mjs','.vue','.jsx','.cjs'}:
        values.extend(re.findall(r'''(?:from\s*|import\s*\(\s*|import\s*|require\(\s*)["']([^"'\n]+)["']''',text))
    elif ext=='.py':
        with contextlib.suppress(SyntaxError):
            for node in ast.walk(ast.parse(text)):
                if isinstance(node,ast.Import):values.extend(n.name for n in node.names)
                if isinstance(node,ast.ImportFrom) and node.module:values.append('.'*node.level+node.module)
    return sorted(set(values))[:200]


class Index:
    def __init__(self,root):
        self.root=Path(root).resolve();source=self.root/'.agents/project-index.json'
        config=json.loads(source.read_text())
        if config.get('version')!=VERSION:raise ValueError('unsupported configuration')
        self.config_digest=digest(source.read_bytes());self.repos=[];names=set()
        self.processor_digest=digest(Path(__file__).read_bytes())
        for item in config['repos']:
            name=item['name'];path=(self.root/item['path']).resolve()
            if not re.fullmatch(r'[a-zA-Z0-9_-]+',name) or name in names:raise ValueError('invalid/duplicate repository')
            if not path.is_relative_to(self.root):raise ValueError('repository escapes workspace')
            names.add(name);self.repos.append(dict(item,root=path))
        self.cache=(self.root/config.get('cache','.agents/cache/project-index.sqlite3')).resolve()
        if not self.cache.is_relative_to(self.root):raise ValueError('cache escapes workspace')

    @contextlib.contextmanager
    def connect(self):
        self.cache.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
        fd=os.open(self.cache,os.O_CREAT|os.O_RDWR,0o600);os.close(fd);self.cache.chmod(0o600)
        db=sqlite3.connect(self.cache,timeout=30);db.row_factory=sqlite3.Row
        try:
            db.execute('PRAGMA secure_delete=ON')
            db.executescript('''
              CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY,value TEXT);
              CREATE TABLE IF NOT EXISTS repos(name TEXT PRIMARY KEY,path TEXT,head TEXT,branch TEXT,fingerprint TEXT);
              CREATE TABLE IF NOT EXISTS files(repo TEXT,path TEXT,sha TEXT,bytes INT,lines INT,kind TEXT,symbols TEXT,imports TEXT,PRIMARY KEY(repo,path));
              CREATE VIRTUAL TABLE IF NOT EXISTS search USING fts5(repo UNINDEXED,path,content,tokenize='unicode61 remove_diacritics 2');
            ''')
            row=db.execute("SELECT value FROM meta WHERE key='version'").fetchone()
            if row and row[0]!=str(VERSION):raise ValueError('cache version mismatch')
            with db:
                yield db
        finally:
            db.close()

    def inventory(self):
        repos=[];files={}
        for spec in self.repos:
            name=spec['name'];root=spec['root']
            if not root.is_dir():raise ValueError('missing root '+name)
            if spec.get('git'):
                paths=git(root,'ls-files','-z','--cached','--others','--exclude-standard').split('\0')
                head=git(root,'rev-parse','HEAD');branch=git(root,'branch','--show-current')
            else:
                paths=[p.relative_to(root).as_posix() for pattern in spec.get('include',[]) for p in root.glob(pattern)]
                head='unversioned';branch=''
            manifest=[]
            for relative in sorted(set(paths)):
                if not relative or not permitted(relative):continue
                path=root/relative
                try:
                    if path.is_symlink() or not path.resolve().is_relative_to(root) or not path.is_file():continue
                    stamp=path.stat();size=stamp.st_size;content='';kind='metadata';lines=0
                    if size<=MAX_BYTES and (Path(relative).suffix in TEXT_EXT or path.name in TEXT_NAMES):
                        raw=path.read_bytes();after=path.stat()
                        if (stamp.st_size,stamp.st_mtime_ns)!=(after.st_size,after.st_mtime_ns):raise ValueError('source changed during scan')
                        sha=digest(raw)
                        if b'\0' not in raw:
                            try:content=redact(raw.decode('utf-8'));kind='text';lines=len(content.splitlines())
                            except UnicodeDecodeError:pass
                    else:sha=digest(f'metadata:{size}:{stamp.st_mtime_ns}'.encode())
                    files[(name,relative)]={'sha':sha,'bytes':size,'lines':lines,'kind':kind,'content':content}
                    manifest.append((relative,sha))
                except (FileNotFoundError,PermissionError):continue
            repos.append({'name':name,'path':str(root),'head':head,'branch':branch,'fingerprint':digest(json.dumps([head,branch,manifest]).encode())})
        return repos,files

    def refresh(self):
        repos,files=self.inventory()
        with self.connect() as db:
            previous={(r['repo'],r['path']):r['sha'] for r in db.execute('SELECT repo,path,sha FROM files')};changed=0
            processor=db.execute("SELECT value FROM meta WHERE key='processor'").fetchone()
            if processor is None or processor[0]!=self.processor_digest:
                previous={key:'' for key in previous}
            for key,item in files.items():
                if previous.get(key)==item['sha']:continue
                repo,path=key;content=item['content']
                db.execute('DELETE FROM search WHERE repo=? AND path=?',key)
                db.execute('INSERT OR REPLACE INTO files VALUES(?,?,?,?,?,?,?,?)',(*key,item['sha'],item['bytes'],item['lines'],item['kind'],json.dumps(symbols(content,path),ensure_ascii=False),json.dumps(imports(content,path))))
                db.execute('INSERT INTO search(repo,path,content) VALUES(?,?,?)',(repo,path,content));changed+=1
            removed=set(previous)-set(files)
            for key in removed:
                db.execute('DELETE FROM files WHERE repo=? AND path=?',key);db.execute('DELETE FROM search WHERE repo=? AND path=?',key)
            for row in db.execute('SELECT name FROM repos').fetchall():
                if row[0] not in {r['name'] for r in repos}:db.execute('DELETE FROM repos WHERE name=?',(row[0],))
            for repo in repos:db.execute('INSERT OR REPLACE INTO repos VALUES(:name,:path,:head,:branch,:fingerprint)',repo)
            db.executemany('INSERT OR REPLACE INTO meta VALUES(?,?)',[('version',str(VERSION)),('config',self.config_digest),('processor',self.processor_digest)])
        return {'files':len(files),'updated':changed,'removed':len(removed),'repositories':[{k:r[k] for k in ('name','head','branch')} for r in repos]}

    def status(self):
        repos,files=self.inventory()
        if not self.cache.exists():return {'fresh':False,'reason':'missing index','files':len(files)}
        with self.connect() as db:
            prior={r['name']:r['fingerprint'] for r in db.execute('SELECT * FROM repos')}
            config=db.execute("SELECT value FROM meta WHERE key='config'").fetchone()
            processor=db.execute("SELECT value FROM meta WHERE key='processor'").fetchone()
        stale=[r['name'] for r in repos if prior.get(r['name'])!=r['fingerprint']]
        return {'fresh':not stale and config is not None and config[0]==self.config_digest and processor is not None and processor[0]==self.processor_digest and set(prior)=={r['name'] for r in repos},'stale_repositories':stale,'files':len(files)}

    def search(self,query,repo=None,limit=8):
        terms=re.findall(r'\w+',query,re.UNICODE)[:20]
        if not terms:return []
        match=' OR '.join('"'+word+'"*' for word in terms)
        with self.connect() as db:
            rows=db.execute('''SELECT repo,path,snippet(search,2,'[',']',' … ',24) excerpt,
              bm25(search,0,5,1) rank FROM search WHERE search MATCH ?
              AND (? IS NULL OR repo=?) ORDER BY rank,path LIMIT ?''',(match,repo,repo,limit)).fetchall()
        return [dict(r,excerpt=r['excerpt'][:1200]) for r in rows]

    def symbol(self,name,repo=None,limit=15):
        found=[]
        with self.connect() as db:
            for row in db.execute('SELECT repo,path,symbols FROM files WHERE (? IS NULL OR repo=?) ORDER BY repo,path',(repo,repo)):
                for entry in json.loads(row['symbols']):
                    if name.casefold() in entry['name'].casefold():found.append({'repo':row['repo'],'path':row['path'],**entry})
        return sorted(found,key=lambda x:(x['name'].casefold()!=name.casefold(),x['path'],x['line']))[:limit]

    def excerpt(self,repo,path,line=1,count=45,symbol=None,max_chars=6000):
        spec=next((s for s in self.repos if s['name']==repo),None)
        if not spec or not permitted(path):raise ValueError('unknown/excluded source')
        target=spec['root']/path
        if target.is_symlink() or not target.resolve().is_relative_to(spec['root']):raise ValueError('source escapes root')
        with self.connect() as db:row=db.execute('SELECT * FROM files WHERE repo=? AND path=?',(repo,path)).fetchone()
        if row is None or row['kind']!='text':raise ValueError('source not indexed text')
        raw=target.read_bytes()
        if len(raw)>MAX_BYTES or digest(raw)!=row['sha']:raise ValueError('source changed or too large; refresh')
        if symbol:
            matches=[s for s in json.loads(row['symbols']) if s['name']==symbol or s['name'].endswith('.'+symbol)]
            if not matches:raise ValueError('symbol not found')
            line=max(1,matches[0]['line']-2)
        selected=redact(raw.decode('utf-8')).splitlines()[max(0,line-1):max(0,line-1)+min(count,120)]
        body='\n'.join(f'{n}: {s}' for n,s in enumerate(selected,max(1,line)))
        return {'repo':repo,'path':path,'sha256':row['sha'],'start_line':line,'excerpt':body[:max_chars],'truncated':len(body)>max_chars}

    def related(self,repo,path,limit=12):
        with self.connect() as db:rows=db.execute('SELECT path,imports FROM files WHERE repo=?',(repo,)).fetchall()
        target=next((r for r in rows if r['path']==path),None)
        if target is None:raise ValueError('source not indexed')
        incoming=[];stem=Path(path).stem
        for row in rows:
            if row['path']==path:continue
            for imp in json.loads(row['imports']):
                cleaned=imp.split('?')[0]
                if cleaned.endswith(('/'+stem,'/'+str(Path(path).parent))) or Path(cleaned).name==stem:
                    incoming.append(row['path']);break
        return {'repo':repo,'path':path,'imports':json.loads(target['imports'])[:limit],'potential_dependents':incoming[:limit],'note':'Lexical hints; verify with rg, not a complete call graph.'}


def main():
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--root',type=Path,default=Path.cwd())
    subs=parser.add_subparsers(dest='command',required=True)
    subs.add_parser('refresh');subs.add_parser('status')
    for name in ['search','symbol','context']:
        p=subs.add_parser(name);p.add_argument('query');p.add_argument('--repo');p.add_argument('--limit',type=int,default=8)
        if name=='context':p.add_argument('--max-chars',type=int,default=6000)
    p=subs.add_parser('excerpt');p.add_argument('repo');p.add_argument('path');p.add_argument('--line',type=int,default=1);p.add_argument('--lines',type=int,default=45);p.add_argument('--symbol');p.add_argument('--max-chars',type=int,default=6000)
    p=subs.add_parser('related');p.add_argument('repo');p.add_argument('path');p.add_argument('--limit',type=int,default=12)
    args=parser.parse_args()
    if not 1<=getattr(args,'limit',1)<=30:parser.error('limit must be 1..30')
    if not 1000<=getattr(args,'max_chars',6000)<=20000:parser.error('max-chars must be 1000..20000')
    if getattr(args,'line',1)<1 or not 1<=getattr(args,'lines',1)<=120:parser.error('line>=1; lines 1..120')
    index=Index(args.root)
    if args.command=='status':result=index.status()
    elif args.command=='refresh':result=index.refresh()
    else:
        index.refresh()
        if args.command=='search':result=index.search(args.query,args.repo,args.limit)
        elif args.command=='symbol':result=index.symbol(args.query,args.repo,args.limit)
        elif args.command=='excerpt':result=index.excerpt(args.repo,args.path,args.line,args.lines,args.symbol,args.max_chars)
        elif args.command=='related':result=index.related(args.repo,args.path,args.limit)
        else:
            with index.connect() as db:revisions=[dict(r) for r in db.execute('SELECT name,head,branch FROM repos')]
            budget=args.max_chars-800;out=[]
            for hit in index.search(args.query,args.repo,args.limit):
                fragment={'repo':hit['repo'],'path':hit['path'],'excerpt':hit['excerpt'][:min(450,max(0,budget))]}
                size=len(json.dumps(fragment,ensure_ascii=False))
                if size>budget:break
                out.append(fragment);budget-=size
            result={'revisions':revisions,'task_query':args.query[:200],'files':out,'instruction':'Navigation hints only; verify source and task contract.'}
    rendered=json.dumps(result,ensure_ascii=False,indent=2)
    if args.command=='context':
        while len(rendered)+1>args.max_chars:
            if result['files']:result['files'].pop()
            elif result['task_query']:result['task_query']=result['task_query'][:len(result['task_query'])//2]
            elif result['revisions']:result['revisions'].pop();result['omitted_revisions']=result.get('omitted_revisions',0)+1
            else:break
            rendered=json.dumps(result,ensure_ascii=False,indent=2)
    print(rendered)


if __name__=='__main__':
    try:main()
    except (ValueError,OSError,sqlite3.Error,subprocess.SubprocessError):
        print(json.dumps({'error':'IndexError','message':'Check workspace config, source stability, SQLite FTS5 and Git availability.'}),file=sys.stderr)
        raise SystemExit(1)
