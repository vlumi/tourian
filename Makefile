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
deps:  ## Install the npm packages of the console and the tooling
	npm ci

##@ Contract

.PHONY: generate
generate:  ## Regenerate the Go and TypeScript code from proto/ into gen/
	cd proto && buf generate

.PHONY: generate-check
generate-check: generate  ## Fail if the committed generated code is out of date
	@git diff --exit-code --stat -- gen/ || { echo "gen/ is stale: run make generate and commit"; exit 1; }

##@ Build & test

.PHONY: build
build:  ## Build the Go binaries and the console
	go build ./...
	npm run build --workspace console

.PHONY: test
test:  ## Run the Go and console unit tests
	go test ./...
	npm test --workspace console

.PHONY: run
run:  ## Run the platform
	go run ./cmd/tourian

.PHONY: dev
dev:  ## Run the console with live reload
	npm run dev --workspace console

##@ Quality

.PHONY: fmt
fmt:  ## Format Go, Protobuf, TypeScript and the configuration files
	golangci-lint fmt
	cd proto && buf format -w
	npx prettier --write .

.PHONY: lint
lint:  ## Lint and check formatting across the repository
	golangci-lint run
	golangci-lint fmt --diff
	cd proto && buf lint && buf format --diff --exit-code
	npx prettier --check .
	npm run lint --workspace console
	npm run typecheck --workspace console
	npx markdownlint-cli2

.PHONY: ci
ci: lint generate-check test build  ## Everything hosted CI runs

##@ Docs

.PHONY: screenshot
screenshot:  ## Build the console and capture it to OUT (default docs/screenshots/latest.png)
	@scripts/screenshot.sh "$(or $(OUT),docs/screenshots/latest.png)"
