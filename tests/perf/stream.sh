#!/bin/sh
# Stream playback: a fresh built-in server, a streamer and viewers in
# headless Chromium (stream.mjs), one viewer direct and one behind a slow,
# jittery link (linkproxy), then a summary (stream-analyze.mjs). Manual
# only, not in CI (see README.md).
#
#   CHROME=<headless chromium> tests/perf/stream.sh [label]
#
# Scenarios (SCENARIOS, space-separated): name=<down bits/s>:<one-way
# delay>:<jitter>:<jitter chance> (see linkproxy); each is one run with
# sound and one without (SOUNDS).
set -eu
root=$(cd "$(dirname "$0")/../.." && pwd)
perf="$root/tests/perf"
label=${1:-run}
tmp=${TMPDIR:-/tmp}
work=${WORK:-${tmp%/}/kallisti-perf-stream}
port=${PORT:-18111}
proxy=${PROXY_PORT:-18112}
mumble=${MUMBLE_PORT:-64830}
web=${WEB_DIR:-$root/apps/web/dist}
scenarios=${SCENARIOS:-local=0:0s:0s:1 far=10e6:75ms:400ms:0.15 jitter=8e6:20ms:200ms:1 tight=5e6:75ms:300ms:0.15}
sounds=${SOUNDS:-1 0}
: "${CHROME:?set CHROME to a headless Chromium (see README.md)}"
die() {
  echo "$*" >&2
  exit 1
}
[ -f "$web/index.html" ] || die "no web app in $web: run pnpm --filter @kallisti/web build"

mkdir -p "$work/bin" "$work/results"
rm -rf "$work/kallisti.db" "$work/kallisti.db-wal" "$work/kallisti.db-shm" "$work/uploads"
(cd "$root/server" && go build -o "$work/bin/" ./cmd/server ./cmd/linkproxy)

pids=""
cleanup() {
  for p in $pids; do kill "$p" 2>/dev/null || true; done
  wait 2>/dev/null || true
}
trap cleanup EXIT
trap 'exit 130' INT TERM

curl -s "http://localhost:$port/" >/dev/null && die "port $port is in use"
WEB_DIR="$web" LISTEN_PORT=$port MUMBLE_LISTEN=127.0.0.1:$mumble \
  DB_PATH="$work/kallisti.db" UPLOAD_DIR="$work/uploads" \
  ALLOWED_ORIGINS="http://localhost:$port,http://localhost:$proxy" \
  "$work/bin/server" >"$work/server.log" 2>&1 &
server_pid=$!
pids="$server_pid"
for _ in $(seq 100); do
  kill -0 "$server_pid" 2>/dev/null || die "server exited, see $work/server.log"
  curl -sf "http://localhost:$port/" >/dev/null && break
  sleep 0.1
done

outs=""
n=0
for s in $scenarios; do
  name=${s%%=*}
  # shellcheck disable=SC2086 # word splitting wanted
  set -- $(echo "${s#*=}" | tr : ' ')
  "$work/bin/linkproxy" -listen 127.0.0.1:$proxy -target 127.0.0.1:$port -down "$1" -up 0 -delay "$2" -jitter "$3" -jitter-chance "$4" -jitter-every 300ms >"$work/proxy.log" 2>&1 &
  proxy_pid=$!
  pids="$pids $proxy_pid"
  sleep 0.3
  for sound in $sounds; do
    n=$((n + 1))
    out="$work/results/$label-$n-$name-sound$sound.json"
    SOUND=$sound node "$perf/stream.mjs" "$out" "http://localhost:$port/" "http://localhost:$port/" "http://localhost:$proxy/"
    outs="$outs $out"
  done
  kill "$proxy_pid"
  wait "$proxy_pid" 2>/dev/null || true
done

# shellcheck disable=SC2086 # word splitting wanted
node "$perf/stream-analyze.mjs" $outs | tee "$work/results/$label.txt"
echo "details: $work/results/$label.txt"
