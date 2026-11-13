#!/usr/bin/env sh
# Builds and serves the console with the platform and the simulator, captures the console to the given file and stops both.
set -eu

out="$1"
mkdir -p "$(dirname "$out")"
trap 'kill $platform $console 2>/dev/null; rm -rf "$build"' EXIT
. scripts/serve.sh

node scripts/screenshot.mjs "$out" "$base/"
