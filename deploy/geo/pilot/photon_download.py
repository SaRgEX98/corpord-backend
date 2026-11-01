#!/usr/bin/env python3
"""Fetch exactly one dated Geofabrik source, bounded to 400 MB; resume retained bytes."""
import argparse
import hashlib
import json
from datetime import datetime, timezone
import pathlib
import time
import urllib.request

URL = 'https://download.geofabrik.de/russia/ural-fed-district-261031.osm.pbf'
MAX_BYTES = 400_000_000
MAX_SECONDS = 900
EXPECTED_BYTES = 396_673_710
EXPECTED_SHA256 = '4b75d5d4d4850bb80999d32844032626545a2068da8b4538cadb154818dc3ec0'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=pathlib.Path)
    directory = parser.parse_args().directory
    directory.mkdir(parents=True, exist_ok=True)
    output = directory / URL.rsplit('/', 1)[1]
    offset = output.stat().st_size if output.exists() else 0
    if offset > EXPECTED_BYTES:
        raise SystemExit('Unexpected existing source size')
    headers = {'User-Agent': 'CorpordGeoPilot/1.0 (https://github.com/SaRgEX98/corpord-backend)'}
    if offset < EXPECTED_BYTES:
        if offset:
            headers['Range'] = f'bytes={offset}-'
        start = time.monotonic()
        request = urllib.request.Request(URL, headers=headers)
        with urllib.request.urlopen(request, timeout=60) as response:
            if offset and (response.status != 206 or not response.headers.get('Content-Range', '').startswith(f'bytes {offset}-')):
                raise SystemExit('Server did not honor resume; retained source untouched')
            length = int(response.headers.get('Content-Length', '0'))
            if length <= 0 or offset + length != EXPECTED_BYTES or offset + length > MAX_BYTES:
                raise SystemExit('Source size exceeds cap or differs from dated manifest')
            with output.open('ab') as target:
                while chunk := response.read(1024 * 1024):
                    if time.monotonic() - start > MAX_SECONDS:
                        raise SystemExit('Download time cap exceeded; resume explicitly later')
                    if target.tell() + len(chunk) > MAX_BYTES:
                        raise SystemExit('Download cap exceeded')
                    target.write(chunk)
    checksum_path = directory / 'source.md5'
    if not checksum_path.exists():
        with urllib.request.urlopen(URL + '.md5', timeout=30) as response:
            checksum_path.write_bytes(response.read(1024))
    expected_md5 = checksum_path.read_text().split()[0]
    checksum = hashlib.md5()
    sha256 = hashlib.sha256()
    with output.open('rb') as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b''):
            checksum.update(chunk)
            sha256.update(chunk)
    actual_md5 = checksum.hexdigest()
    if output.stat().st_size != EXPECTED_BYTES or actual_md5 != expected_md5 or sha256.hexdigest() != EXPECTED_SHA256:
        raise SystemExit('Source checksum mismatch; do not import')
    (directory / 'source-download.json').write_text(json.dumps({
        'url': URL, 'verified_at': datetime.now(timezone.utc).isoformat(),
        'bytes': EXPECTED_BYTES, 'md5': actual_md5, 'sha256': sha256.hexdigest(),
        'checksum_url': URL + '.md5', 'download_cap_bytes': MAX_BYTES,
        'resumable': True, 'attempt_time_cap_seconds': MAX_SECONDS, 'socket_timeout_seconds': 60}, indent=2) + '\n')
    print(f'{output}: {EXPECTED_BYTES} bytes; Geofabrik MD5 verified')


if __name__ == '__main__':
    main()
