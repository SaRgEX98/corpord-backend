#!/usr/bin/env python3
"""Bounded, pinned city pilot preparation. Linux x86_64, Python3, Node/npm required.

All large/generated data stays in the supplied directory outside Git. No OSM
public tile scraping. Source supports official Protomaps HTTP Range extraction.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import resource
import shutil
import subprocess
import tarfile
import time
import urllib.request
import zipfile

SOURCE = 'https://build.protomaps.com/20261031.pmtiles'
ASSETS = '028c18f713baecad011301ff7a69acc39bcc2ae7'
TOOL_HASH = '3ed7dbf4ec2e6dfe5e25b6f70d1ffc932729f93c86db353bf514dd71010a312f'
ASSET_HASH = 'e942a417d94a12596842a20b53d6b785cbf6d47f2545e458538191ba6d74b305'
EXTRACT_HASH = 'f86daa943da333c07ba8b1270adb64bf74b8804030136bb292c0c4e3c67ec31f'

def digest(path):
    return hashlib.file_digest(open(path, 'rb'), 'sha256').hexdigest()

def download(url, path, maximum, expected=None):
    if not path.exists():
        with urllib.request.urlopen(url, timeout=60) as response, open(path, 'wb') as out:
            size = 0
            while block := response.read(65536):
                size += len(block)
                if size > maximum:
                    path.unlink(missing_ok=True)
                    raise RuntimeError('Download budget exceeded')
                out.write(block)
    if expected and digest(path) != expected:
        raise RuntimeError(f'Checksum mismatch: {path.name}')

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('data_dir', type=Path)
    parser.add_argument('--origin', default='http://127.0.0.1:8092')
    args = parser.parse_args()
    data = args.data_dir.resolve()
    data.mkdir(parents=True, exist_ok=True)
    tools = data/'tools'
    tools.mkdir(exist_ok=True)
    tool_archive = data/'go-pmtiles.tar.gz'
    download('https://github.com/protomaps/go-pmtiles/releases/download/v1.31.2/go-pmtiles_1.31.2_Linux_x86_64.tar.gz', tool_archive, 20000000, TOOL_HASH)
    with tarfile.open(tool_archive) as archive:
        member = next(m for m in archive.getmembers() if m.name == 'pmtiles')
        with archive.extractfile(member) as src, open(tools/'pmtiles','wb') as dst:
            shutil.copyfileobj(src, dst)
    (tools/'pmtiles').chmod(0o755)
    command = [str(tools/'pmtiles'), 'extract', SOURCE, str(data/'yekaterinburg.pmtiles'),
        '--bbox=60.45,56.72,60.75,56.92', '--maxzoom=14', '--download-threads=2']
    env = {**os.environ, 'GOMEMLIMIT': '768MiB', 'GOMAXPROCS': '2'}
    # Dry-run fetches only directory metadata, checks estimated transfer first.
    dry = subprocess.run(command+['--dry-run'], capture_output=True, text=True, check=True, env=env)
    (data/'extract-dry-run.log').write_text(dry.stdout+dry.stderr)
    import re
    size = re.search(r'Extract transferred ([0-9.]+) (kB|MB|GB)', dry.stderr+dry.stdout)
    if not size or float(size[1])*{'kB':1000,'MB':1000000,'GB':1000000000}[size[2]] > 200000000:
        raise RuntimeError('Unexpected extract budget; review before proceeding')
    if not (data/'yekaterinburg.pmtiles').exists():
        start = time.monotonic()
        with open(data/'extract.log','w') as log:
            subprocess.run(command, stdout=log, stderr=subprocess.STDOUT, check=True, env=env)
        (data/'extract-resources.json').write_text(json.dumps({'elapsed_seconds':time.monotonic()-start,
            'max_rss_kib':resource.getrusage(resource.RUSAGE_CHILDREN).ru_maxrss}, indent=2))
    if digest(data/'yekaterinburg.pmtiles') != EXTRACT_HASH:
        raise RuntimeError('Regional archive differs from reviewed snapshot; review new source before publishing')
    subprocess.run([str(tools/'pmtiles'),'verify',str(data/'yekaterinburg.pmtiles')],check=True)
    archive_path = data/'assets.zip'
    download(f'https://codeload.github.com/protomaps/basemaps-assets/zip/{ASSETS}', archive_path, 10000000, ASSET_HASH)
    with zipfile.ZipFile(archive_path) as archive:
        for name in archive.namelist():
            relative = '/'.join(name.split('/')[1:])
            if name.endswith('/') or not relative.startswith(('fonts/', 'sprites/v4/')):
                continue
            destination = data/relative
            destination.parent.mkdir(parents=True, exist_ok=True)
            destination.write_bytes(archive.read(name))
    licenses = data/'licenses'
    licenses.mkdir(exist_ok=True)
    shutil.copy(data/'fonts/OFL.txt', licenses/'fonts-OFL.txt')
    # The basemaps-assets README identifies tangrams/icons as sprite upstream.
    download('https://raw.githubusercontent.com/tangrams/icons/92510779634f4a006c61ea70e50cb8c52c765a81/LICENSE.md', licenses/'sprites-MIT.txt', 20000)
    build = data/'style-build'
    subprocess.run(['npm','install','--prefix',str(build),'--save-exact','--ignore-scripts','--no-audit','--no-fund','@protomaps/basemaps@5.7.2'], check=True)
    lock = json.loads((build/'package-lock.json').read_text())
    if lock['packages']['node_modules/@protomaps/basemaps']['integrity'] != 'sha512-K1Yk6bWdULulYg+R2QRVXx4NzJZan5YQhpejEG0c1/sXruJrfPIPZuakpf3jwAgVmjIRVQwAv+yRafDeN0aaUQ==':
        raise RuntimeError('Basemaps npm integrity differs from reviewed package')
    subprocess.run(['node',str(Path(__file__).with_name('map-style.mjs')),str(data),args.origin], check=True)
    download('https://raw.githubusercontent.com/protomaps/basemaps/3ea8293a28131c3dc63f1bb20827bdb8a76df06f/LICENSE.md', licenses/'basemaps-BSD.txt', 20000)
    download('https://raw.githubusercontent.com/protomaps/basemaps/3ea8293a28131c3dc63f1bb20827bdb8a76df06f/LICENSE_DATA.md', licenses/'basemaps-DATA.md', 20000)
    (licenses/'DATA.md').write_text('OpenStreetMap contributors: ODbL 1.0 https://www.openstreetmap.org/copyright\nNatural Earth: public domain https://www.naturalearthdata.com/about/terms-of-use/\n')
    for path in data.rglob('*'):
        path.chmod(0o755 if path.is_dir() or path.name=='pmtiles' else 0o644)
    manifest = {'source':SOURCE, 'source_date':'2026-10-31', 'osm_replication_time':'2026-10-31T04:00:00Z',
        'bounds':[60.45,56.72,60.75,56.92], 'maxzoom':14,'tile_count':360,
        'pmtiles_cli':'1.31.2','basemaps':'5.7.2','assets_commit':ASSETS,
        'origin':args.origin,'files':{str(p.relative_to(data)):{'sha256':digest(p),'bytes':p.stat().st_size}
            for p in sorted(data.rglob('*')) if p.is_file() and p.parts[len(data.parts)] in ('styles','fonts','sprites','licenses','yekaterinburg.pmtiles')}}
    (data/'map-manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
    total = sum(p.stat().st_size for p in data.rglob('*') if p.is_file())
    if total > 2000000000:
        raise RuntimeError('Stored data budget exceeded')
    print(json.dumps({'stored_bytes':total,'pmtiles_bytes':(data/'yekaterinburg.pmtiles').stat().st_size}))

if __name__ == '__main__':
    main()
