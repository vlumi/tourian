#!/usr/bin/env sh
# Regenerates the contract's Go, TypeScript and Python code, clearing what was generated before and nothing else: gen/ts
# also holds the package file that makes it the npm package @tourian/contract.
set -eu

rm -rf gen/go/tourian gen/ts/tourian gen/python/tourian
(cd proto && buf generate)
