# Routine operations, named here and done by scripts/. Run `make` to list them.

.DEFAULT_GOAL := help

BIN := $(CURDIR)/bin
export PATH := $(BIN):$(PATH)

export BUF_VERSION := v1.75.0
export PROTOC_GEN_GO_VERSION := v1.36.12
export GOLANGCI_LINT_VERSION := v2.15.0

.PHONY: help
help:  ## List the available commands
	@echo "tourian — available make targets:"
	@awk -f scripts/help.awk $(MAKEFILE_LIST)

##@ Setup

.PHONY: tools
tools:  ## Install buf, protoc-gen-go and golangci-lint into bin/ at the pinned versions
	@scripts/tools.sh "$(BIN)"

.PHONY: deps
deps:  ## Install the npm packages of the console and the tooling, and the Python environment of the sidecars
	npm ci
	python3 -m venv .venv && .venv/bin/pip install -q -e 'sidecars/arm[dev]'

##@ Contract

.PHONY: generate
generate:  ## Regenerate the Go and TypeScript code from proto/ into gen/
	cd proto && buf generate

.PHONY: generate-check
generate-check:  ## Fail if the generated code in gen/ is out of date
	@scripts/generate-check.sh

##@ Build & test

.PHONY: build
build:  ## Build the Go binaries and the console
	go build ./...
	npm run build --workspace console

.PHONY: test
test:  ## Run the Go, console and sidecar unit tests, Go under the race detector
	go test -race ./...
	npm test --workspace console
	PYTHONDONTWRITEBYTECODE=1 .venv/bin/pytest -q -p no:cacheprovider sidecars/arm

.PHONY: contract
contract:  ## Run the contract tests against every robot adapter
	go test -run TestContract -count=1 ./emmi/...

.PHONY: run
run:  ## Run the platform with the simulated robot in-process
	go run ./cmd/tourian -sim

.PHONY: dev
dev:  ## Run the console with live reload
	npm run dev --workspace console

##@ Quality

.PHONY: fmt
fmt:  ## Format Go, Protobuf, TypeScript and the configuration files
	golangci-lint fmt
	cd proto && buf format -w
	npx prettier --write .
	.venv/bin/ruff format sidecars

.PHONY: lint
lint:  ## Lint and check formatting across the repository
	golangci-lint run
	golangci-lint fmt --diff
	cd proto && buf lint && buf format --diff --exit-code
	npx prettier --check .
	npm run lint --workspace console
	npm run typecheck --workspace console
	npx markdownlint-cli2
	.venv/bin/ruff check sidecars
	.venv/bin/ruff format --check sidecars

.PHONY: ci
ci: lint generate-check test build contract  ## Everything hosted CI runs

##@ Docs

.PHONY: results-video
results-video:  ## Measure video latency on both paths and write docs/results/<date>-video-latency.md
	@mkdir -p docs/results && scripts/video-latency.sh "docs/results/$$(date +%Y-%m-%d)-video-latency.md"

.PHONY: screenshot
screenshot:  ## Build the console and capture it to OUT (default docs/screenshots/latest.png)
	@scripts/screenshot.sh "$(or $(OUT),docs/screenshots/latest.png)"
