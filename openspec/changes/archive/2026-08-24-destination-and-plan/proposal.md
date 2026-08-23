## Why

`manifest-and-lock` and `catalog-and-selectors` can say what a consumer asked for and what
a source offers, but nothing yet answers the question the user actually cares about: *which
files will land where, and which will be deleted?* Until that answer exists as a value —
computed without touching the tree — `sync` cannot promise that a validation failure leaves
no partial state, and `--dry-run` has nothing to print.

## What Changes

- New `internal/plan`, pure: manifest sources + their catalogs + a file listing + the
  current lock in, a plan out. It reads no file, runs no command, and writes nothing.
- **Destination computation** — `{name}` interpolation, the trailing-slash "into this
  directory" rule, `flatten`, list-valued `to`, and the per-source `[sources.*.kinds]`
  consumer override that beats the catalog.
- **The prune set** — the files `graft.lock` claims that the new resolution no longer
  produces. Derived from the lock alone, never from scanning a directory, so a file graft
  did not write can never enter it.
- **The next lock** as a value, so `apply` can write it last without recomputing anything.
- **Two invariants, enforced before any plan is returned**: no destination escapes the repo
  root, and no two items resolve to the same path — within a source or across sources.
- Not **BREAKING**: no file format and no command output changes. `graft.lock` keeps
  `version = 1`.

## Non-Goals

- **No writing.** No copy, no delete, no directory creation, no lock write — that is
  `internal/apply`, which this change does not add.
- **No fetching and no listing.** The resolved sha and the source's file listing are
  inputs; producing them is `git-fetch`.
- **No command surface.** No cobra, no `sync`, no `--dry-run`, no `added/updated/removed`
  report. Those are `command-surface` and `sync-command`.
- **No empty-directory removal**, which is a property of a real tree, not of a plan.
- Nothing near the PRD's non-goals: no dependency resolution, no registry, no merge
  behavior, no auth, no runtime dependency on graft, and no path by which a source can
  cause anything to execute.

## Capabilities

### New Capabilities
- `destination-computation`: how a kind's `to`, `{name}`, a trailing slash, `flatten`, a
  list-valued `to`, and a consumer override map one item's source files to repo-relative
  paths — and when that mapping is refused.
- `sync-plan`: assembling those destinations into writes, deriving the prune set from the
  lock, building the next lock, and holding the collision and escape invariants.

### Modified Capabilities
None. `catalog-format`, `manifest-format`, `lock-format`, and `selector-expansion` keep
every requirement they have.

## Impact

- New package `internal/plan`; new specs `destination-computation` and `sync-plan`.
- No change to `internal/{manifest,lock,catalog,itemid}`, to `cmd/graft`, to `go.mod`, or to
  any file format. Nothing outside this repository is affected.
