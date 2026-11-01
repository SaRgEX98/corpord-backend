#!/usr/bin/env bash
# Run only the isolated pilot; never touches application containers or sysctl.
set -euo pipefail
mode=${1:?Usage: photon_runtime.sh import|serve|stop /absolute/pilot/data}
pilot_dir=${2:?Absolute pilot data directory required}
[[ "$pilot_dir" = /* && -f "$pilot_dir/photon.jsonl" ]] || { echo 'Expected absolute directory containing photon.jsonl' >&2; exit 1; }
image=corpord-photon:1.3.0
name=corpord-geo-pilot-photon
common=(--memory=3g --memory-swap=3g --cpus=2 --pids-limit=512 --user "$(id -u):$(id -g)" -e JAVA_TOOL_OPTIONS=-Xms256m\ -Xmx1024m -v "$pilot_dir/index:/data" -v "$pilot_dir/photon.jsonl:/import/photon.jsonl:ro")
case "$mode" in
  import)
    [[ ! -d "$pilot_dir/index" ]] || { echo 'Index exists; use a new versioned directory' >&2; exit 1; }
    mkdir "$pilot_dir/index"
    docker run --name "$name-import" --network none "${common[@]}" "$image" import -import-file /import/photon.jsonl -languages ru,en -country-codes ru
    ;;
  serve)
    [[ -d "$pilot_dir/index/photon_data" ]] || { echo 'Index missing' >&2; exit 1; }
    docker run -d --label corpord.geo.pilot=true --name "$name" -p 127.0.0.1:2333:2322 "${common[@]}" "$image" serve -listen-ip 0.0.0.0 -listen-port 2322 -max-results 10 -default-language ru -query-timeout 3
    ;;
  stop)
    docker stop "$name"
    docker rm "$name"
    ;;
  *) echo 'Unknown mode' >&2; exit 1 ;;
esac
