#!/usr/bin/env sh
# Builds the console, serves it on a spare port, captures it to the given file and stops the server.
set -eu

out="$1"
port="${PORT:-3100}"

npm run build --workspace console >/dev/null
npm run start --workspace console -- --port "$port" >/dev/null 2>&1 &
server=$!
trap 'kill $server 2>/dev/null' EXIT

for _ in $(seq 1 50); do
  curl -sf "http://localhost:$port/" >/dev/null 2>&1 && break
  sleep 0.2
done

node scripts/screenshot.mjs "$out" "http://localhost:$port/"
