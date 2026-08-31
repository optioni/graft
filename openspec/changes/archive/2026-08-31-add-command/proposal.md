## Why

Every consumer today starts by hand-writing `graft.toml`: guessing the source name, looking
up a tag, and typing selectors it cannot check without running `sync` and reading the error.
`graft add` is the command SPEC.md's table promises for that, and it is what `self-hosting`
needs in order for this repo to convert its hand-copied `openspec/schemas/tdd/` and
`.claude/agents/` into a declared source.

## What Changes

- `graft add <source>[@rev] [selector...]` declares the source in `graft.toml` and syncs.
  A source already declared is **amended** — new selectors are unioned into its `install`
  list — rather than duplicated.
- The source name is derived from the git value's last path segment (`optioni/schemas` →
  `schemas`). A derived name that is not a TOML bare key, or that is already declared with a
  different `git`, is an error rather than a guess.
- Without `@rev`, `add` resolves a default pin: the source's highest semver tag, falling
  back to its default branch's name when it publishes none. `@rev` accepts anything `rev`
  accepts, a semver range included.
- `graft add <source> --list` prints the source's catalog — every item id with the
  destination it would be written to — and exits, writing nothing.
- `--no-sync` writes `graft.toml` and stops.
- `graft.toml` is amended by **surgical text edit**, never by re-serializing a parsed
  manifest: a new source's table is appended, and an existing source's `install` list is
  amended in place. Any shape that cannot be rewritten exactly is refused.
- `add` is the only command permitted to create `graft.toml`. Every other command still
  fails on its absence.
- `add` given a rev that differs from the source's current pin moves the pin, and that
  source alone is re-resolved. This is the second command allowed to move one, decided
  when `semver-ranges` was proposed.

Nothing about `graft.toml`, `graft.lock`, or `catalog.yaml`'s format changes, and no
existing command's output moves. There is no **BREAKING** change here.

## Non-Goals

- **The interactive picker.** `add` without selectors is an error naming the selectors it
  needed, on a TTY and off one alike. `add-picker` narrows that to the no-TTY case.
- **The `kind:*` collapse offer.** It is a property of the picker's selection, not of `add`.
- **Naming a source something other than its repo's last segment.** No `--as` flag; a
  collision is refused, and the consumer edits `graft.toml` by hand.
- **Removing a source.** There is no `graft remove`; deleting the block and syncing is it.
- Re-resolving a pin `add` did not move, and re-resolving on `sync`. Both unchanged.

## Capabilities

### New Capabilities
- `add-execution`: the `graft add` sequence — name derivation, `@rev`, the default pin,
  the manifest amendment, `--list`, `--no-sync`, and the argument surface.

### Modified Capabilities
- `manifest-format`: appending a source table and amending an `install` list in place,
  beside the existing in-place pin move, with the shapes each refuses.
- `rev-resolution`: a source's default rev, when the invocation names none.
- `file-application`: a manifest-only apply, for `--no-sync`.

## Impact

- New: `internal/add` (the sequence), `internal/cli/add.go` (wiring).
- Changed: `internal/manifest` (append and amend), `internal/source` (default rev, catalog
  listing for `--list`), `internal/apply` (manifest-only write), `internal/sync`
  (`Options` accepts pre-edited manifest bytes), `internal/cli` (register the command).
- No new dependency. No format change, so no lock or manifest migration.
