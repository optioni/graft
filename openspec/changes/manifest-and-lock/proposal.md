## Why

`graft` reads two consumer-owned files on every run: `graft.toml`, which a human writes to
declare what the repo wants, and `graft.lock`, which graft writes to record what it
installed. Neither exists in code, so nothing downstream can be built. The lock is also
committed and reviewed as a git diff, so its bytes are part of the product — a serializer
that reorders or reformats between runs turns every sync into a noisy diff.

## What Changes

- New `internal/manifest`: parse and validate `graft.toml` — `[sources.<name>]` with `git`,
  `rev`, `install` selectors, and the optional per-source `kinds` override. Unknown keys are
  an error, not silently ignored.
- New `internal/lock`: parse and validate `graft.lock` — `version`, `[[source]]` with
  `name`/`git`/`rev`/`resolved`, nested `[[source.item]]` with `id` and `files`.
- Deterministic lock serialization to bytes, in the exact layout SPEC.md documents: sources
  by name, items by id, files by path. Serializing twice is byte-identical, and
  parse → serialize of canonical bytes returns those same bytes.
- A missing `graft.lock` loads as an empty lock (the first sync is a legitimate state); a
  missing `graft.toml` is an error.
- A lock whose `version` this binary does not know fails and says to upgrade graft.
- Pin drift across the pair — manifest `rev` differing from the lock's — is an error naming
  both and pointing at `graft update`.
- First module dependency: `github.com/BurntSushi/toml v1.6.0` for decoding. The lock writer
  is hand-written; no encoder emits the documented layout.

Not **BREAKING**: SPEC.md already documents both formats and no released graft reads either,
so `graft.lock`'s `version` is established at `1` rather than moved.

## Non-Goals

- **No writing.** `lock` returns bytes; `internal/apply` writes them, in `sync-command`.
- **No `graft.toml` serialization** — amendment is `add-command`, where preserving a human's
  comments has a real caller to answer it.
- **No selector expansion.** Selector syntax is checked here; matching against a catalog is
  `catalog-and-selectors`.
- **No destinations, prune set, or path-escape check** (`destination-and-plan`); **no git,
  network, cache, or rev resolution** (`git-fetch`); **no CLI surface** (`command-surface`).
- Clear of the PRD non-goals: no dependency resolution, registry, merge behavior, auth
  layer, or runtime dependency on graft.

## Capabilities

### New Capabilities
- `manifest-format`: parsing and validation of `graft.toml` — source blocks, required
  fields, selector syntax, kind overrides, unknown-key rejection.
- `lock-format`: parsing, validation, and byte-stable deterministic serialization of
  `graft.lock`, plus format-version compatibility and manifest/lock pin agreement.

### Modified Capabilities
- None. `openspec/specs/` is empty; this is the first change.

## Impact

- New `internal/manifest/` and `internal/lock/`. `cmd/graft` unchanged.
- `go.mod` gains the module's first dependency.
- Downstream: `destination-and-plan` consumes both; `sync-command` writes the bytes `lock`
  produces; `add-command` later adds manifest serialization.
- No data model, background job, external service, sibling repo, or deployment manifest.
