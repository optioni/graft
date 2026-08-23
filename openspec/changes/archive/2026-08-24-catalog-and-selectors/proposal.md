## Why

A source repository's `catalog.yaml` is the only place graft learns what a source offers
and where each class of thing belongs — the tool holds no built-in vocabulary of agents,
schemas, or skills. Nothing reads it yet, so a consumer's `install` selectors cannot be
turned into the items they name, and `destination-and-plan` has no input.

## What Changes

- New `internal/catalog`: parse and validate `catalog.yaml` — `version`, `kinds` with a
  string- or list-valued `to` and an optional `flatten`, and `provides` entries carrying
  `kind`, `name`, and `from`. Unknown keys are an error, not a silent typo.
- A catalog whose `version` this binary does not know fails and says to upgrade graft; an
  absent `catalog.yaml` is the "not graftable" error the failure-mode table names.
- Internal consistency is checked at parse: every provided item names a declared kind,
  `kind:name` is unique, and `from` stays inside the source tree.
- Selector expansion: each `graft.toml` selector is matched against `provides`, with `*`
  and `?` globbing in the name position. The result is the deduplicated union, ordered by
  item id.
- A selector matching nothing is an error listing what the catalog does provide — typo
  protection, per SPEC.md's failure-mode table.
- Second module dependency: `github.com/goccy/go-yaml` for decoding. It is maintained,
  has no transitive dependencies, and rejects unknown fields.

Not **BREAKING**: SPEC.md already documents `catalog.yaml` and no released graft reads it,
so its `version` is established at `1` rather than moved. No command output changes,
because no command is wired to this yet.

## Non-Goals

- **No destination computation.** `{name}` interpolation, trailing-slash and `flatten`
  semantics, list-valued `to` fan-out, and consumer overrides are `destination-and-plan`.
- **No prune set, no path-escape check over computed destinations, no collision check.**
- **No writing** of any kind, and no code in `internal/plan` or `internal/apply`.
- **No fetching.** Reading a catalog out of a fetched tree is `git-fetch` plus
  `sync-command`; this change reads bytes and a path handed to it.
- **No CLI surface** — no command, flag, exit code, or stderr formatting.
- Clear of the PRD non-goals: no dependency resolution (`requires` stays an open question
  in SPEC.md), no registry, no merge behavior, no auth, no runtime dependency on graft.

## Capabilities

### New Capabilities
- `catalog-format`: parsing and validation of `catalog.yaml` — version gating, `kinds`,
  `provides`, unknown-key rejection, and the source-tree containment rule for `from`.
- `selector-expansion`: matching `graft.toml` selectors against a catalog's `provides`,
  including globs, deduplication, ordering, and the no-match error.

### Modified Capabilities
- None. `manifest-format` and `lock-format` keep every requirement they have.

## Impact

- New `internal/catalog/`. `internal/manifest`, `internal/lock`, and `cmd/graft` unchanged;
  `internal/itemid` gains a caller, not a change.
- `go.mod` gains its second dependency.
- Downstream: `destination-and-plan` consumes the parsed catalog and the expanded item set.
- No data model, background job, external service, sibling repository, or deployment
  manifest.
