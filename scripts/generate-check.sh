#!/usr/bin/env sh
# Regenerates the contract code and fails if the result differs from what is in gen/.
set -eu

before="$(mktemp -d)"
trap 'rm -rf "$before"' EXIT
cp -R gen/. "$before"

(cd proto && buf generate)

if ! diff -r "$before" gen >/dev/null; then
  echo "gen/ is stale; the fresh generation is now in place, review and commit it" >&2
  diff -r "$before" gen | head -20 >&2
  exit 1
fi
