# graft — Engineering

How this repo is built, tested, and shipped. Behavior lives in [SPEC.md](SPEC.md);
motivation in [PRD.md](PRD.md).

## Toolchain

Go **1.27.0**, pinned by the `toolchain` directive in `go.mod`. Nothing else is required to
build.

Local development also wants `task`, `golangci-lint`, `gofumpt`, and `goreleaser`. None are
installed on a fresh machine:

```sh
brew install go go-task golangci-lint gofumpt goreleaser
```

## Layout

```
cmd/graft/          main() and nothing else — flag wiring, exit codes
internal/manifest/  graft.toml parsing and validation
internal/lock/      graft.lock read, write, deterministic serialization
internal/catalog/   catalog.yaml parsing, selector expansion
internal/source/    git resolution and the content-addressed fetch cache
internal/plan/      pure: manifest + lock + catalog -> file operations
internal/apply/     the only package that touches the working tree
```

`plan` is pure and `apply` is the sole writer. That split is what makes the SPEC's
"nothing touches the tree until every check passes" invariant testable rather than
aspirational.

## Tasks

`Taskfile.yml` is the single definition of every command. CI runs `task ci` and nothing
else — no step is ever spelled out twice.

| Task | Does |
|---|---|
| `task fmt` | `gofumpt -w .` |
| `task lint` | `golangci-lint run`, plus `gofumpt -l` failing on any output |
| `task test` | `go test -race ./...` |
| `task cover` | test with coverage, print the report, fail under the threshold |
| `task build` | local binary with version ldflags |
| `task ci` | `lint` + `cover` + `build` |
| `task release:snapshot` | `goreleaser release --snapshot --clean` |

## Testing

**Coverage floor: 80%**, enforced by `task cover` and measured over `./internal/...`.
`cmd/graft` is excluded deliberately — the way to hit a coverage number honestly is to keep
`main()` trivial and put every decision in a package, not to write tests for flag wiring.

- **Unit**: selector expansion, destination computation (`{name}`, `flatten`, list-valued
  `to`), prune-set derivation, lock serialization determinism.
- **Integration**: fixture git repos built in `t.TempDir()`. First sync, idempotent
  re-sync, version bump that adds and removes items, dropping an item from `install`, and
  — every time — a foreign file in a shared destination surviving untouched.
- **Invariants get their own tests.** Path escape, cross-source collision, and no-partial-
  writes-on-failure are the safety story; they are not covered incidentally.
- **Error text is asserted.** For a CLI this small the failure-mode table *is* the product,
  so tests pin the messages and they cannot rot silently.

## CI

GitHub Actions. On pull requests and pushes to `main`: `task ci` on `ubuntu-latest` and
`macos-latest`, with the module cache warmed. That is the whole pipeline.

One extra job dogfoods: the repo consumes `openspec-schemas` through graft, so CI runs a
sync and asserts the tree is unchanged. It is the most realistic integration test available
and it costs one job.

Note this is **producer** CI only. Repos that consume graft need no CI integration —
syncing is a deliberate human action on committed files, and a repo sitting on an older pin
is a legitimate state, not a failure.

## Releases

Tag `vX.Y.Z` on `main` → GoReleaser builds and publishes.

- `CGO_ENABLED=0`, `darwin` and `linux`, `amd64` and `arm64`.
- Archives plus a checksums file, attached to a GitHub Release with build provenance.
- Changelog grouped by Conventional Commit type, generated from the commits.
- Version, commit, and build date injected with `-ldflags -X`; `graft --version` prints all
  three.
- The Homebrew formula is published to `optioni/homebrew-tap` by the same run.

Releases are cut by hand. Automated version bumping stays out until tagging actually hurts.

## Installation

```sh
brew install optioni/tap/graft          # primary
go install github.com/optioni/graft@latest
```

Or download a binary from the releases page. There is no install script and no
auto-updater.

## Compatibility

- `catalog.yaml` and `graft.lock` both carry `version = 1`. A graft that meets a format
  version it does not know **fails and says to upgrade** — it never guesses, and never
  half-reads a newer file.
- Minimum Go is whatever `go.mod` pins; it moves only in a minor release.
- **`darwin` and `linux` only.** Windows is out of scope for v1 rather than nominally
  supported: path separators and symlink semantics leak straight into the copy layer, and
  untested support is worse than none.

## Security

**graft executes nothing from a source.** It reads `catalog.yaml` and copies files. There
is no build step, no hook, no script a source repo can cause to run. That property is the
reason no audit or sandboxing subsystem is needed, and it is worth defending in review:
any feature that would run source-provided content changes the tool's threat model
entirely.

Dependabot covers Go modules and Actions. Releases carry provenance attestations.

There is no telemetry of any kind.

## Workflow

Public repo, MIT licensed. Conventional Commits. Commits land on `main` directly; a branch
only when the work genuinely needs isolation.

This repo uses **OpenSpec with the `tdd` schema**. Non-trivial work starts as a change
proposal (`/opsx:propose`), not as ad-hoc planning; bug fixes and trivial changes go
direct. Commit after each task group, and archive a change once it is implemented and
verified.

**Bootstrap:** graft cannot yet install its own schema, so `openspec/schemas/tdd/` and
`.claude/agents/` are copied in by hand from `openspec-schemas`. They convert to a normal
`graft.toml` entry as soon as `sync` works — the dogfooding job above depends on that
switch happening.
