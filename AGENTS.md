# Development rules

The same rules as [nebulosa](https://github.com/vlumi/nebulosa) and [fiftysixty](https://github.com/vlumi/fiftysixty), adapted to a Go and TypeScript repository with a Protobuf contract. Read [SCOPE.md](SCOPE.md) for what is being built and in which order, and [ARCHITECTURE.md](ARCHITECTURE.md) for how.

## Rules of the repository

- Never mention where this project came from, any company or product it might be compared to as its origin, or any hiring process, in code, documentation, commit messages or anywhere else in the repository. Technical references to well-known systems are fine where they explain a choice.
- The Protobuf contract in `proto/` is the source of truth. Every robot adapter, transport, controller and console speaks it; nothing crosses a seam in a shape the contract does not define. Generated code is committed, so a checkout builds without the generators.
- Every seam (robot, transport, video, dataset sink, controller) is an interface with at least two implementations before it is called an abstraction. One implementation behind an interface is a premature interface: write the concrete thing and extract the seam when the second one arrives.
- Safety lives in the robot adapter, never in the console: command timeouts, the dead-man switch and the emergency stop must work with the console gone. A robot that has not heard a command within its timeout stops.
- Recorded episodes and media are data, never committed; `data/` is gitignored.
- No secrets in the repository, and no settings files: the platform's flags are the configuration, and the console finds the platform by the page it was served from.

## Commits and pull requests

- Incremental, honest history: each commit is one coherent step that builds and makes sense on its own, one concern per commit.
- Pull requests are sized by concern too: not padded with unrelated changes, not split into fragments that only make sense together. Squash locally *before* pushing.
- Commit messages describe the step, not the process.
- Every concrete chunk of work is a branch and a pull request against `main`, rebased on `main` before pushing. Rewriting and force-pushing an open PR branch to keep its history clean is fine; `main` is never rewritten. The bootstrap is committed directly; everything since goes through a PR.
- Gate a push on `make ci`, which is what hosted CI runs: formatting, linters, generated code up to date, the unit tests for Go, the console and the sidecars, then the contract tests against the Go robot adapters.

## Comments

- Minimal. A comment only where the code cannot be made to say it; prefer renaming and refactoring over explaining.
- No narration ("increment counter"), no changelog comments, no restating types.

## Code

- Clean and well-structured, but built iteratively: working first, then dedicated cleanup and review rounds. Don't gold-plate mid-feature; do flag debt worth a cleanup pass.
- en-US throughout: code, comments, docs, interface.
- Go is organized by feature, not by layer (no `controller/`, `service/`, `repository/` packages); `cmd/` holds the binaries, everything else is a package named for what it does.
- The console is Next.js with TypeScript; it reads the generated contract types and the store, never raw messages.
- Routine operations are make targets with a `##` description under a `##@` group, so plain `make` lists them; scripts do the work, the Makefile names it.
- Toolchains: latest stable of Go, Node and Python, `buf` for the contract. `make tools` installs buf, protoc-gen-go and golangci-lint; `make deps` the npm packages, with protoc-gen-es, and the Python environment, with ruff.

## Documentation

- Markdown paragraphs are one line each; the editor wraps. `.markdownlint.jsonc` encodes this.
- When a milestone lands, mark it done in SCOPE.md with the date, describe it in the current state of ARCHITECTURE.md and add it to the history at its end, and add a capture to `docs/screenshots/`.
