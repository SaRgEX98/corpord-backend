#!/usr/bin/env python3
"""Real pinned Photon import/serve smoke using invented data, NOT a coverage pilot."""
import json
import os
import pathlib
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request
import uuid

HERE = pathlib.Path(__file__).resolve().parent
IMAGE = "corpord-photon:1.3.0"
name = "corpord-photon-smoke-" + uuid.uuid4().hex[:10]
with tempfile.TemporaryDirectory(prefix="corpord-photon-") as folder:
    data = pathlib.Path(folder)
    user = ["--user", f"{os.getuid()}:{os.getgid()}"]  # Host owns test files; production uses uid 10001.
    mounts = ["-v", f"{data}:/data", "-v", f"{HERE / 'fixtures/synthetic.jsonl'}:/fixture.jsonl:ro"]
    began = time.monotonic()
    try:
        subprocess.run(["docker", "run", "--name", name + "-import", "--rm", "--memory", "2g", "--cpus", "2", "--pids-limit", "256", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", *user, *mounts, IMAGE, "import", "-import-file", "/fixture.jsonl", "-languages", "ru,en"], check=True, timeout=180)
    except BaseException:
        subprocess.run(["docker", "rm", "-f", name + "-import"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        raise
    elapsed = time.monotonic() - began
    size = sum(p.stat().st_size for p in data.rglob("*") if p.is_file())
    subprocess.run(["docker", "run", "-d", "--name", name, "--memory", "2g", "--cpus", "2", "--pids-limit", "256", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", *user, "-p", "127.0.0.1::2322", *mounts, IMAGE], check=True, stdout=subprocess.DEVNULL)
    try:
        port = subprocess.check_output(["docker", "port", name, "2322/tcp"], text=True).strip().rsplit(":", 1)[1]
        base = f"http://127.0.0.1:{port}"
        for attempt in range(120):
            try:
                with urllib.request.urlopen(base + "/status", timeout=2) as r:
                    assert r.status == 200
                break
            except OSError:
                time.sleep(0.5)
        else:
            raise RuntimeError("Photon not ready")
        for query in ["Екатеринбург", "Учебный автовокзал", "Учебная 145"]:
            url = base + "/api?" + urllib.parse.urlencode({"q": query, "lang": "ru", "limit": 5, "countrycode": "RU"})
            with urllib.request.urlopen(url, timeout=5) as response:
                result = json.load(response)
                assert result["features"], query
            print("PASS synthetic query:", query)
        print(f"Synthetic-only import: {elapsed:.2f}s, index files {size} bytes")
        print(subprocess.check_output(["docker", "stats", "--no-stream", "--format", "{{.MemUsage}} / {{.CPUPerc}}", name], text=True).strip())
    finally:
        subprocess.run(["docker", "rm", "-f", name], check=True, stdout=subprocess.DEVNULL)
