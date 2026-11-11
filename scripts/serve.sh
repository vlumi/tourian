#!/usr/bin/env sh
# Builds the platform and the console and serves both until the caller's trap kills them: sourced by the scripts that
# capture or measure the console. The processes are the real ones, not `go run` or `npm` wrappers, so a kill reaches them.
#
#   . scripts/serve.sh            # sets $platform, $console (pids), $port, $platform_port
set -eu

port="${PORT:-3100}"
platform_port="${PLATFORM_PORT:-8090}"
build="$(mktemp -d)"

NEXT_PUBLIC_TOURIAN_URL="ws://localhost:$platform_port" npm run build --workspace console >/dev/null
go build -o "$build/tourian" ./cmd/tourian
"$build/tourian" -sim -addr ":$platform_port" -data "$build/data" >/dev/null 2>&1 &
platform=$!
(cd console && exec ../node_modules/.bin/next start --port "$port" >/dev/null 2>&1) &
console=$!

up=0
for _ in $(seq 1 100); do
  if curl -sf "http://localhost:$port/" >/dev/null 2>&1 && curl -sf "http://localhost:$platform_port/healthz" >/dev/null 2>&1; then
    up=1
    break
  fi
  sleep 0.2
done
if [ "$up" != 1 ]; then
  echo "the console on :$port or the platform on :$platform_port did not come up" >&2
  kill "$platform" "$console" 2>/dev/null || true
  exit 1
fi
