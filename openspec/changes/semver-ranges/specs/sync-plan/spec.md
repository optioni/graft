## MODIFIED Requirements

### Requirement: The next lock records exactly what the plan produces

The plan's next lock SHALL record, for each source in the manifest, its name, `git`, and
`rev` as written in `graft.toml`, the tag a range matched, the sha it resolved to, and one
item per installed item holding every destination that item produces. The matched tag SHALL
be carried through exactly as resolution reported it — the tag name for a range, and empty
for a ref — because `graft.lock` requires it for the one and refuses it for the other, and
a plan may never build a lock a later `sync` would refuse to read.

Sources SHALL be ordered by name, items by
id, and files by path. A source present in the lock but absent from the manifest SHALL NOT
appear. Serializing the next lock twice from the same inputs SHALL produce byte-identical
output, and the serialized bytes SHALL parse back through `graft.lock`'s own parser without
error. A plan may never build a lock a later `sync` would refuse to read.

Because `graft.lock` requires a 40-character lowercase hex `resolved`, `internal/plan` SHALL
refuse a source whose resolved sha is not one, before anything is planned for that source,
failing with

```
source "<source>": resolved "<value>" is not a 40-character hex sha
```

and returning no plan. This one constraint is checked rather than assumed, and the others
are not, because it is the only one a caller can violate silently: unique source names,
unique item ids, and no path claimed twice are all consequences of what this specification
already requires, so the round-trip scenario below catches a violation of any of them, while
a bad `resolved` is carried verbatim into the lock by a serializer that validates nothing.

The matched tag SHALL NOT be checked the same way. `internal/plan` SHALL carry it verbatim
and SHALL NOT decide whether a rev is a range, because that predicate belongs to the packages
that interpret a rev, and a third opinion about it is a third place for the three to disagree.

#### Scenario: A resolved sha that is not a sha fails the plan

- **WHEN** source `shared` is supplied with `resolved` empty, or `v1.2.0`, or a 40-character
  hex string written in upper case
- **THEN** planning fails with
  `source "shared": resolved "" is not a 40-character hex sha` — naming the value it was
  given — and no plan is returned
- **AND** the check runs before anything is planned for that source, so a lock `lock.Parse`
  would refuse is never built even partway

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

#### Scenario: A range source's next lock carries the matched tag and round-trips

- **WHEN** a plan is built for a source whose `rev` is `^1.2.0`, resolved to `v1.3.0` at a
  valid sha
- **THEN** the next lock records `rev = "^1.2.0"` and `matched = "v1.3.0"`
- **AND** serializing it and parsing it back through `graft.lock`'s own parser succeeds,
  which it would not if `matched` were dropped

#### Scenario: A ref source's next lock carries no matched tag

- **WHEN** a plan is built for a source whose `rev` is `v1.2.0`
- **THEN** the next lock's matched value is empty
- **AND** the serialized bytes contain no `matched` line, and parse back without error, which
  they would not if an empty `matched` were written

#### Scenario: A bad resolved sha is refused before anything is planned

- **WHEN** a plan is built for a source whose resolved sha is `v1.2.0`
- **THEN** it fails with `source "shared": resolved "v1.2.0" is not a 40-character hex sha`
- **AND** no plan is returned

#### Scenario: The next lock round-trips through the lock's own parser

- **WHEN** a next lock is serialized from a plan built over several sources and items
- **THEN** parsing those bytes with `graft.lock`'s parser returns a lock with identical
  content
- **AND** serializing the same plan twice produces byte-identical output
