## ADDED Requirements

### Requirement: A plan is a value, and building one touches nothing

`internal/plan` SHALL build a plan from values only: for each source, the parsed
`graft.toml` block, the parsed `catalog.yaml`, the resolved sha, and a listing of the paths
each installed item's `from` contributes; plus the parsed `graft.lock`. Building a plan
SHALL read no file, stat no path, run no command, open no network connection, and create,
modify, or delete nothing in the working tree.

A plan SHALL hold exactly three things: the **writes** to perform, the **prune set** to
delete, and the **next lock** to record. Building SHALL be total over its inputs — no
source at all, no lock at all, and an item contributing no files are all legitimate states
rather than failures. On any error the returned plan SHALL be nil, so no caller can act on
a half-computed plan; this is what makes "nothing touches the tree until every check
passes" a property of the type rather than a promise.

#### Scenario: A first plan against no lock

- **WHEN** a plan is built for source `shared` at rev `v1.2.0` resolved to
  `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5`, installing `schema:tdd` with the listing
  `["schema.yaml"]` under `to: "openspec/schemas/{name}"`, and `graft.lock` is absent so the
  lock loads empty
- **THEN** the plan writes `openspec/schemas/tdd/schema.yaml`, prunes nothing, and its next
  lock records source `shared` with item `schema:tdd` holding that one file
- **AND** the working tree is unchanged — `openspec/schemas/tdd/` does not exist and is not
  created, and `graft.lock` is not written

#### Scenario: A plan for a manifest with no sources

- **WHEN** a plan is built from zero sources and an empty lock
- **THEN** the plan writes nothing, prunes nothing, and its next lock holds zero sources
- **AND** no error is returned and the working tree is unchanged

#### Scenario: An item contributing no files still appears in the lock

- **WHEN** the item `schema:empty` is installed and its listing is empty, because `from`
  names an empty directory in the source
- **THEN** the plan writes nothing for it, prunes nothing, and its next lock records
  `schema:empty` with an empty `files` list
- **AND** no directory is created for the destination the item would otherwise have had

#### Scenario: A failing plan is returned as no plan at all

- **WHEN** any check in this specification or in `destination-computation` fails while
  building a plan
- **THEN** the returned plan is nil and only the error is returned
- **AND** no partial write list, prune set, or lock is observable, so nothing downstream can
  begin writing from a plan that failed validation

### Requirement: Every write names the file to copy and where it lands

Each write in a plan SHALL name the source it comes from, the item id it belongs to, the
path of the file within that source's fetched tree, and the repo-relative destination. For
a directory item the source path SHALL be the item's `from` joined with the listed path;
for a file item it SHALL be the item's `from` itself, never `from` joined with its own base
name. Writes SHALL be ordered by destination in ascending byte order, so the same inputs
always produce the same plan. A plan SHALL emit a write for every planned file regardless of
what the working tree already holds: synced files are derived artifacts, always overwritten,
and a plan has no file content to compare.

#### Scenario: A write carries the source path and the destination

- **WHEN** the item `schema:tdd` in source `shared` has `from: extras/openspec-schemas/tdd`
  naming a directory and the listing `["templates/design.md"]` under
  `to: "openspec/schemas/{name}"`
- **THEN** the plan holds one write naming source `shared`, item `schema:tdd`, the source
  path `extras/openspec-schemas/tdd/templates/design.md`, and the destination
  `openspec/schemas/tdd/templates/design.md`

#### Scenario: A file item's write names the file itself, not a path below it

- **WHEN** the item `agent:apply-orchestrator` in source `shared` has
  `from: extras/agents/apply-orchestrator.md` naming a file, with the listing
  `["apply-orchestrator.md"]` under `to: ".claude/agents/"`
- **THEN** the write's source path is `extras/agents/apply-orchestrator.md`, not
  `extras/agents/apply-orchestrator.md/apply-orchestrator.md`
- **AND** the destination is `.claude/agents/apply-orchestrator.md`, so the listing entry
  names the destination's leaf while the source path names the file as the catalog wrote it

#### Scenario: Writes are ordered by destination across sources and items

- **WHEN** a plan is built for sources `zeta` and `alpha`, each installing items whose
  destinations are `b.md` and `a.md` respectively
- **THEN** the writes appear ordered by destination — `a.md` before `b.md` — regardless of
  the order the sources, items, or listings were supplied in
- **AND** rebuilding the plan from the same inputs produces the identical write order

#### Scenario: A file already present in the tree is still written

- **WHEN** the plan's destination `openspec/schemas/tdd/schema.yaml` already exists in the
  working tree with different content, and the lock already claims it
- **THEN** the plan still holds a write for it and does not prune it
- **AND** the plan carries no notion of "unchanged" — a hand-edit is silently overwritten
  when the plan is applied, and `git diff` is the report

### Requirement: The prune set is derived from the lock alone

The prune set SHALL be exactly those file paths recorded in `graft.lock` that the new
resolution no longer produces, ordered by path in ascending byte order. A path SHALL enter
the prune set only by being in the lock, never by being found in a destination directory.
A path produced by the new resolution SHALL NOT be pruned even when the lock also claims
it.

This is the invariant that lets synced files share a directory with files graft does not
own: a file absent from `graft.lock` is invisible to graft and can never be deleted by it.

#### Scenario: A foreign file in a shared destination is never pruned

- **WHEN** the lock records `agent:apply-orchestrator` with
  `.claude/agents/apply-orchestrator.md`, the working tree also holds the repo's own
  `.claude/agents/local-reviewer.md`, and a plan is built that still installs that item
- **THEN** the prune set is empty, and `.claude/agents/local-reviewer.md` appears nowhere in
  the plan — not as a write, not as a prune, not in the next lock
- **AND** the same holds when the item is dropped entirely: only
  `.claude/agents/apply-orchestrator.md` is pruned and the repo-owned agent is untouched

#### Scenario: An item dropped from install has its files pruned

- **WHEN** the lock records `schema:tdd` with `openspec/schemas/tdd/schema.yaml` and
  `openspec/schemas/tdd/templates/design.md`, and the manifest's `install` no longer selects
  it
- **THEN** the prune set is both files, ordered `openspec/schemas/tdd/schema.yaml` then
  `openspec/schemas/tdd/templates/design.md`
- **AND** the next lock no longer records `schema:tdd`

#### Scenario: An item the source stopped providing has its files pruned

- **WHEN** the lock records `agent:phase-orchestrator` with
  `.claude/agents/phase-orchestrator.md`, the manifest installs `agent:*`, and the source's
  catalog at the new resolution no longer provides that item
- **THEN** the prune set is `.claude/agents/phase-orchestrator.md`
- **AND** the selector `agent:*` still matching other items means no no-match error is
  raised

#### Scenario: A source removed from the manifest has all its files pruned

- **WHEN** the lock records source `retired` with two items holding three files in total,
  and `graft.toml` no longer declares that source
- **THEN** the prune set is those three files, ordered by path
- **AND** the next lock holds no `retired` source

#### Scenario: A moved destination prunes the old path and writes the new one

- **WHEN** the lock records `agent:x` at `.claude/agents/x.md` and the manifest now
  overrides `agent = ".codex/agents/"`
- **THEN** the plan writes `.codex/agents/x.md` and prunes `.claude/agents/x.md`
- **AND** the next lock records `agent:x` with `.codex/agents/x.md` only

#### Scenario: A version bump that adds and removes items

- **WHEN** the lock records `schema:tdd` with `schema.yaml` and `templates/old.md` under
  `openspec/schemas/tdd/`, and the new resolution's listing is `["schema.yaml",
  "templates/new.md"]`
- **THEN** the plan writes `openspec/schemas/tdd/schema.yaml` and
  `openspec/schemas/tdd/templates/new.md`, and prunes only
  `openspec/schemas/tdd/templates/old.md`
- **AND** `openspec/schemas/tdd/schema.yaml` is not pruned, because it is in both sets

#### Scenario: A path moving from one source to another is written, not pruned

- **WHEN** the lock records `.claude/agents/x.md` under source `alpha`, `alpha` no longer
  installs the item that produced it, and source `beta` now produces exactly that path
- **THEN** the plan writes `.claude/agents/x.md` from `beta` and the prune set is empty
- **AND** the file is not deleted and re-created, because the prune set is a set difference
  over paths rather than a per-source bookkeeping exercise — a path the new resolution
  produces is never pruned, whichever source produced it before

#### Scenario: An idempotent re-plan prunes nothing

- **WHEN** a plan is built from the same manifest, catalog, listing, and resolved sha the
  lock already records
- **THEN** the prune set is empty and the next lock is byte-identical to the one on disk
- **AND** the plan still holds a write for every file, because a plan never decides that a
  file needs no writing

### Requirement: The next lock records exactly what the plan produces

The plan's next lock SHALL record, for each source in the manifest, its name, `git`, and
`rev` as written in `graft.toml`, the sha it resolved to, and one item per installed item
holding every destination that item produces. Sources SHALL be ordered by name, items by
id, and files by path. A source present in the lock but absent from the manifest SHALL NOT
appear. Serializing the next lock twice from the same inputs SHALL produce byte-identical
output, and the serialized bytes SHALL parse back through `graft.lock`'s own parser without
error — a plan may never build a lock a later `sync` would refuse to read.

#### Scenario: An item placed in two destinations records both files

- **WHEN** `schema:tdd` is installed under `to: ["vendor/schemas/{name}",
  "openspec/schemas/{name}"]` with the listing `["schema.yaml"]`
- **THEN** the next lock records `schema:tdd` with
  `openspec/schemas/tdd/schema.yaml` and `vendor/schemas/tdd/schema.yaml`, in that order
- **AND** both are pruned together if the item is later dropped

#### Scenario: The next lock carries rev and resolved separately

- **WHEN** the manifest declares `rev = "v1.2.0"` for source `shared` and the resolved sha
  supplied is `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5`
- **THEN** the next lock records `rev = "v1.2.0"` and
  `resolved = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"` for that source
- **AND** `git` is recorded exactly as `graft.toml` wrote it, shorthand unexpanded

#### Scenario: The next lock round-trips through the lock parser

- **WHEN** a plan is built for two sources, one of them installing an item that produces no
  files, and its next lock is serialized
- **THEN** parsing those bytes back succeeds and yields a lock holding the same sources,
  items, and files
- **AND** every constraint `graft.lock` enforces on load holds of what the plan produced —
  a 40-character hex `resolved`, unique source names, unique item ids, and no path claimed
  twice — so a plan can never write a lock the next run rejects

#### Scenario: Sources, items, and files are ordered independently of input order

- **WHEN** the same two sources, their items, and their listings are supplied in reversed
  order
- **THEN** the serialized next lock is byte-identical to the one built from the original
  order
- **AND** byte equality — not semantic equality — is what is asserted, because a lock that
  reorders on every run churns the diff of every consumer

### Requirement: No two items resolve to the same path

`internal/plan` SHALL refuse a plan in which two different items produce the same
destination path, whether they belong to one source or to different sources. On refusal it
SHALL fail with

```
source "<a>" item "<a-id>" and source "<b>" item "<b-id>" both resolve to "<path>"
```

naming the two items in the order the deterministic walk reaches them — sources by name,
items by id, destinations in declared order, files by path — and SHALL return no plan.
Collisions SHALL be an error rather than last-writer-wins, because the loser would be a
file the lock claims and a later sync would delete.

#### Scenario: Two items of one source colliding is an error

- **WHEN** source `shared` installs `agent:review` from a file and `agent:pack` from a
  directory whose flattened listing includes `review.md`, both landing at
  `.claude/agents/review.md`
- **THEN** planning fails with
  `source "shared" item "agent:pack" and source "shared" item "agent:review" both resolve to ".claude/agents/review.md"`
- **AND** no plan is returned, so neither file is written and neither is recorded in a lock

#### Scenario: Two sources colliding is an error

- **WHEN** sources `alpha` and `beta` both provide `agent:x` and both place it at
  `.claude/agents/x.md`
- **THEN** planning fails with
  `source "alpha" item "agent:x" and source "beta" item "agent:x" both resolve to ".claude/agents/x.md"`
- **AND** the error is raised before any write is planned, so the tree is untouched

#### Scenario: A path claimed by the lock and by another item is still a collision

- **WHEN** the lock already records `.claude/agents/x.md` for `agent:x`, and the new
  resolution has both `agent:x` and `agent:y` producing it
- **THEN** planning fails with
  `source "shared" item "agent:x" and source "shared" item "agent:y" both resolve to ".claude/agents/x.md"`
- **AND** the lock having previously claimed the path grants no precedence, and nothing in
  the lock is pruned, because a failed build produces no plan at all

#### Scenario: One item producing the same path twice is not this error

- **WHEN** one item produces the same destination twice
- **THEN** the error is the within-item one specified by `destination-computation` — a
  flatten collision or two entries of a list-valued `to` interpolating alike — naming the
  single item once rather than naming it as its own collision partner

### Requirement: Selector failures surface from planning

`internal/plan` SHALL expand each source's `install` selectors against that source's
catalog and SHALL return the expansion error unchanged when one occurs, so typo protection
reaches the user through planning rather than being swallowed by it. A source whose catalog
provides zero items SHALL therefore fail, because `graft.toml` requires at least one
selector and no selector can match nothing.

#### Scenario: A selector matching nothing fails the plan

- **WHEN** source `shared` installs `schema:tdd-workflwo` and its catalog provides
  `agent:apply-orchestrator` and `schema:tdd`
- **THEN** planning fails with
  `source "shared": selector "schema:tdd-workflwo" matches no item; catalog provides agent:apply-orchestrator, schema:tdd`
- **AND** no plan is returned, so a misspelling never silently installs nothing

#### Scenario: A catalog providing zero items fails the plan

- **WHEN** source `shared` installs `agent:*` and its catalog provides no items at all
- **THEN** planning fails with
  `source "shared": selector "agent:*" matches no item; catalog provides no items`
- **AND** nothing in the lock for that source is pruned, because the failure precedes the
  prune set — a plan that fails produces no deletions at all
