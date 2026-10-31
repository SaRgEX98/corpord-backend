#!/usr/bin/env python3
"""Validate the actual pinned nginx config, with synthetic bytes; no OSM downloads."""
import pathlib
import subprocess
import tempfile
import time
import urllib.request
import urllib.error
import uuid

HERE = pathlib.Path(__file__).resolve().parent
IMAGE = "nginx:1.28.0-alpine@sha256:30f1c0d78e0ad60901648be663a710bdadf19e4c10ac6782c235200619158284"
name = "corpord-tiles-smoke-" + uuid.uuid4().hex[:10]
with tempfile.TemporaryDirectory(prefix="corpord-tiles-") as folder:
    data = pathlib.Path(folder)
    data.chmod(0o755)
    (data / "fixture.pmtiles").write_bytes(b"0123456789abcdef")
    subprocess.run(["docker", "run", "-d", "--name", name, "--memory", "128m", "--cpus", "1", "--user", "101:101", "--read-only", "--tmpfs", "/tmp", "--tmpfs", "/var/cache/nginx:mode=1777", "--tmpfs", "/var/run", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "-p", "127.0.0.1::8080", "-v", f"{HERE / 'nginx.conf'}:/etc/nginx/nginx.conf:ro", "-v", f"{data}:/srv/maps:ro", IMAGE], check=True, stdout=subprocess.DEVNULL)
    try:
        port = subprocess.check_output(["docker", "port", name, "8080/tcp"], text=True).strip().rsplit(":", 1)[1]
        base = f"http://127.0.0.1:{port}"
        for _ in range(50):
            try:
                with urllib.request.urlopen(base + "/healthz", timeout=1) as r:
                    assert r.status == 200
                break
            except (OSError, urllib.error.URLError):
                time.sleep(0.1)
        headers = {"Origin": "http://localhost:3000", "Range": "bytes=2-5"}
        with urllib.request.urlopen(urllib.request.Request(base + "/fixture.pmtiles", headers=headers)) as r:
            assert r.status == 206 and r.read() == b"2345"
            assert r.headers["Content-Range"] == "bytes 2-5/16"
            assert r.headers["Access-Control-Allow-Origin"] == headers["Origin"]
            assert "ETag" in r.headers["Access-Control-Expose-Headers"]
            etag = r.headers["ETag"]
        with urllib.request.urlopen(urllib.request.Request(base + "/fixture.pmtiles", method="HEAD")) as r:
            assert r.headers["Content-Length"] == "16" and r.headers["Accept-Ranges"] == "bytes"
            assert "max-age=3600" in r.headers["Cache-Control"]
        with urllib.request.urlopen(urllib.request.Request(base + "/fixture.pmtiles", headers=headers, method="OPTIONS")) as r:
            assert r.status == 204 and "Range" in r.headers["Access-Control-Allow-Headers"]
        with urllib.request.urlopen(urllib.request.Request(base + "/fixture.pmtiles", headers={"Origin": "https://untrusted.invalid"})) as r:
            assert not r.headers.get("Access-Control-Allow-Origin")
        for extra, status in [({"Range": "bytes=30-40"}, 416), ({"If-None-Match": etag}, 304), ({"If-Match": '"wrong"'}, 412)]:
            try:
                urllib.request.urlopen(urllib.request.Request(base + "/fixture.pmtiles", headers=extra))
                raise AssertionError(f"expected {status}")
            except urllib.error.HTTPError as error:
                assert error.code == status
        print("PASS: Range 206/416, HEAD, CORS/preflight, ETag 304/412, cache headers")
    finally:
        subprocess.run(["docker", "rm", "-f", name], check=True, stdout=subprocess.DEVNULL)
