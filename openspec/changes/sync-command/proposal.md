## Why

Six changes have landed and none of them touches a file. `graft.toml` parses, `catalog.yaml`
parses, a plan computes every write and every deletion, a fetch cache holds source trees,
and a command surface reports errors — and the tool still installs nothing. Everything built
so far exists to make one operation safe; this is the change that performs it.

It is also the change that creates the only package in the repository permitted to write to
the user's working tree, and the only one that can delete a file someone wrote by hand.
`internal/plan` already refuses every plan that would be unsafe to apply, but refusing is
half of the guarantee: the other half is an applier that deletes nothing the lock does not
claim, writes nothing outside the repository root, and records what it did only after every
operation succeeded. Until that exists, "graft may never remove a file it did not write" is
a sentence in SPEC.md rather than a property of a program.

## What Changes

- **New `internal/apply`** — the only package that writes to the working tree. Given a
  repository root, the fetched tree of each source, and a `plan.Plan`, it writes the planned
  files, deletes the prune set, removes the directories the prune set left empty, and writes
  `graft.lock` last. Every path operation is contained by an `os.Root` at the repository
  root, so no destination can escape it even if a check upstream were to miss one.
- **New `internal/sync`** — the orchestration `graft sync` performs: load `graft.toml` and
  `graft.lock`, check the pins agree, take each source's sha **from the lock** (resolving
  only a source the lock has never seen), fetch it, read its catalog, list each installed
  item's files, build the plan, and apply it. It takes its repository root and cache root as
  values, so every test names its own and none can reach the developer's real cache.
- **New `graft sync` command** — registered on the existing cobra root, wired to
  `internal/sync`, with `--dry-run`. It prints its report to the **error** stream, because a
  summary is not machine-readable output, and writes nothing to standard output at all.
- **The sync report** — SPEC.md's per-item `added` / `updated` / `removed` block, its source
  header showing the rev and sha that moved, and its one-line summary. A sync with nothing
  to do prints `up to date` and nothing else.
- **`--dry-run`** prints the same report and touches nothing — no file written, no file
  deleted, no directory created, and no `graft.lock`. SPEC.md names the flag under
  `Commands`; no later change in `IMPLEMENTATION-ORDER.md` owns it, so it lands here rather
  than becoming a documented flag with no home.
- **`graft help` becomes an unknown command**, closing the transition `command-invocation`
  left open: registering a subcommand makes cobra install its built-in `help` command, and
  SPEC.md's command table does not name one. This is the same trade the same spec already
  makes for `version`, where `--version` is the only spelling. **BREAKING** only in the sense
  that a behavior left deliberately unspecified is now pinned; no released binary has one.
- **`manifest.Filename` and `lock.Filename`** are added as exported constants, so
  `graft.toml` and `graft.lock` are each spelled in exactly one place now that two packages
  name them. Additive; no parsing, validation, or serialization changes.

## Non-Goals

- **No `graft update`, `graft add`, or `graft list`.** `sync` never re-resolves a pin — it
  installs what the lock says — and moving a pin is a later change. There is no `--force` and
  no `--frozen`, per SPEC.md: sync is always frozen.
- **No dependency resolution, registry, merge behavior, or auth layer.** The PRD's non-goals
  are untouched. A source's files are copied verbatim over whatever is there; nothing is
  merged, and nothing is collected back.
- **Nothing a source provides is executed.** No build step, no hook, no script. Files are
  read and written as bytes with a fixed mode; a source cannot make a file executable in a
  consumer's tree.
- **No self-hosting.** This repository's own `openspec/schemas/tdd/` and `.claude/agents/`
  stay hand-copied. Converting them to a `graft.toml` entry is the `self-hosting` change, and
  nothing here runs `graft sync` against this repository.
- **No transactional apply.** A plan that passes validation can still fail partway through on
  a full disk. `internal/plan` already refuses the plans whose partial application would be
  unrecoverable; making the rest atomic would mean a staging tree, which is a cost nothing yet
  justifies.
- **No `graft.lock` format change.** `version` stays `1`.

## Capabilities

### New Capabilities

- `file-application`: the only package that writes to the working tree — how files are
  written, how the prune set is executed, which directories are removed, that `graft.lock` is
  written last, and what is refused rather than written.
- `sync-execution`: `graft sync` end to end — the order of operations, taking each pin from
  the lock rather than re-resolving it, `--dry-run`, and every failure mode reaching the user
  with the tree untouched.
- `sync-report`: what a sync prints — the per-source header, the per-item `added` /
  `updated` / `removed` lines, the summary, `up to date`, and which stream carries them.

### Modified Capabilities

- `command-invocation`: registering `sync` closes the transition the spec left open — `graft
  help` is refused like any other unrecognised argument, and an argument to `sync` is a usage
  error.

## Impact

- **New code**: `internal/apply` (the sole writer), `internal/sync` (orchestration and the
  report value).
- **Changed code**: `internal/cli` gains the `sync` subcommand and its flag; `internal/lock`
  and `internal/manifest` each gain one exported filename constant.
- **Unchanged**: `internal/plan` stays pure and gains nothing. `internal/catalog`,
  `internal/source`, `internal/ui`, `internal/itemid`, and `cmd/graft` are untouched.
- **Data model**: none. `graft.lock` keeps `version = 1` and its documented layout, byte for
  byte.
- **External services**: `git` on `PATH`, already required by `internal/source`. No new
  module dependency.
- **Sibling repositories**: none. The dogfood CI job stays inert until `self-hosting`.
