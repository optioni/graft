# AGENTS.md

## Project Overview

`graft` vendors files from a source git repository into the repository it runs in —
agent definitions, OpenSpec schemas, skills — pinned by a lockfile and committed.

**The tool is not built yet.** This repo currently holds design documents and scaffolding.
Read [SPEC.md](SPEC.md) before writing any code; it is the contract, not a sketch.

- [PRD.md](PRD.md) — problem, goals, non-goals, why existing tools do not fit
- [SPEC.md](SPEC.md) — file formats, commands, resolution, invariants, failure modes
- [ENGINEERING.md](ENGINEERING.md) — toolchain, layout, testing, CI, releases

## Setup

Go 1.27, plus tooling:

```sh
brew install go go-task golangci-lint gofumpt goreleaser
```

## Commands

`Taskfile.yml` is the single definition of every command; CI runs `task ci` and nothing
else. Never spell a step out twice.

```sh
task fmt      # gofumpt -w .
task lint     # golangci-lint + gofumpt -l
task test     # go test -race ./...
task cover    # coverage, fails under 80% over ./internal/...
task build    # local binary with version ldflags
task ci       # lint + cover + build
```

## Architecture

```
cmd/graft/          flag wiring and exit codes, nothing else
internal/manifest/  graft.toml
internal/lock/      graft.lock
internal/catalog/   catalog.yaml, selector expansion
internal/source/    git resolution, content-addressed fetch cache
internal/plan/      pure: manifest + lock + catalog -> file operations
internal/apply/     the only package that writes to the working tree
```

`plan` is pure and `apply` is the sole writer. This is not stylistic — it is what makes
"nothing touches the tree until every check passes" a testable invariant. A write outside
`apply`, or a filesystem read inside `plan`, breaks the design rather than bending it.

## Rules that are easy to get wrong

- **Never delete a file absent from `graft.lock`.** `.claude/agents/` holds repo-owned
  agents beside synced ones. The lock's per-item file list exists solely to make removal
  safe; any change near the prune set needs a test proving a foreign file survives.
- **Lock serialization is deterministic** — sources by name, items by id, files by path.
  Assert byte equality across two runs, not semantic equality, or every sync churns the
  diff.
- **`sync` never re-resolves a pin.** It installs what the lock says. Only `update` moves
  a pin. Re-resolving on sync would make `rev = "main"` drift silently, which defeats the
  point of pinning.
- **Error strings are asserted by tests.** For a CLI this small the failure-mode table is
  the product. Changing a message is a deliberate contract change.
- **graft executes nothing from a source, but it does place.** It reads `catalog.yaml`
  and copies files — no build step, no hook, no script a source can cause to run. The
  claim stops there: kinds are arbitrary, so a catalog can name a destination that
  something *else* executes (`.github/workflows/` runs in CI, `.claude/agents/` is
  instructions an agent follows). That is why `add` shows every item's destination before
  writing and why a consumer override beats the catalog. Any feature that would run
  source-provided content changes the threat model and needs arguing for explicitly.
- **Coverage is measured over `./internal/...` only.** Logic in `cmd/graft` is invisible
  to the gate — which is the reason to keep it out of there.
- **Fixture git repos need `user.name` and `user.email` set on the repo**, not the
  machine, or commits fail on a clean CI runner.

## Workflow

This repo uses **OpenSpec with the `tdd` schema**. Non-trivial work starts as a change
proposal (`/opsx:propose`), not ad-hoc planning; bug fixes and trivial changes go direct.
Run `openspec validate --strict` before presenting a proposal as ready. Commit after each
task group; archive a change once implemented and verified.

Conventional Commits. Commits land on `main` directly unless the work needs isolation.
Never push or open a PR without being asked.

## Synced files

`openspec/schemas/tdd/` and `.claude/agents/` are **copies** from
[openspec-schemas](https://github.com/optioni/openspec-schemas). Edit them there, not
here. They are copied by hand today; once `sync` works they convert to a `graft.toml`
entry and this repo becomes its own first consumer.
