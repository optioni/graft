## ADDED Requirements

### Requirement: Destination computation is pure

`internal/plan` SHALL compute every destination from values alone — a parsed catalog, a
parsed manifest source, and a listing of the paths an item's `from` contributes, each
relative to that `from`. It SHALL read no file, stat no path, run no command, and create,
modify, or delete nothing in the working tree. A destination SHALL be a repo-relative
path in cleaned form, using `/` as the separator, with no leading `/`.

The file listing is an input rather than something this package gathers: enumerating a
fetched source tree is `internal/source`'s work, and writing is `internal/apply`'s. A test
that needs a real directory on disk to exercise destination computation is evidence the
boundary moved.

#### Scenario: Computing destinations touches nothing

- **WHEN** destinations are computed for a catalog declaring `schema` with
  `to: "openspec/schemas/{name}"`, an item `schema:tdd` with
  `from: extras/openspec-schemas/tdd`, and the listing
  `["schema.yaml", "templates/design.md"]`, in a working tree holding neither
  `openspec/schemas/` nor any file named by the result
- **THEN** the computed destinations are `openspec/schemas/tdd/schema.yaml` and
  `openspec/schemas/tdd/templates/design.md`
- **AND** the working tree is unchanged: no file is created, modified, or deleted, and no
  directory — including `openspec/schemas/tdd/` — is created

#### Scenario: An item contributing no files computes no destinations

- **WHEN** the same catalog and item are given an empty listing, because `from` names an
  empty directory in the source
- **THEN** the item computes zero destinations and no error
- **AND** the working tree is unchanged, and no directory is created for the destination
  the item would otherwise have had

### Requirement: A kind's `to` places an item's files

For each item, `internal/plan` SHALL take the `to` of the kind the item declares, replace
every occurrence of `{name}` with the item's name, and map the item's files under the
result:

- When the item's `from` names a **directory**, each listed path SHALL be joined to the
  interpolated destination, preserving the structure below `from`.
- When the item's `from` names a **file** and the interpolated destination does **not**
  end in `/`, the file SHALL land at the interpolated destination exactly — the
  destination names the file.
- When the item's `from` names a **file** and the interpolated destination **does** end in
  `/`, the file SHALL land inside that directory under its own base name — a trailing `/`
  means "into this directory".

A trailing `/` SHALL make no difference for a directory item: `to` names the destination
directory whether or not it is written with one. The path of `from` within the source
SHALL NOT otherwise appear in any destination, so a source may restructure `from` freely
without moving a consumer's files.

#### Scenario: A directory item preserves its structure under an interpolated destination

- **WHEN** the kind `schema` declares `to: "openspec/schemas/{name}"`, the item
  `schema:tdd` has `from: extras/openspec-schemas/tdd` naming a directory, and its listing
  is `["schema.yaml", "templates/design.md", "templates/proposal.md"]`
- **THEN** the destinations are `openspec/schemas/tdd/schema.yaml`,
  `openspec/schemas/tdd/templates/design.md`, and
  `openspec/schemas/tdd/templates/proposal.md`
- **AND** no destination mentions `extras` or `openspec-schemas`, so moving `from` in the
  source leaves every consumer's tree at the same paths

#### Scenario: A trailing slash places a file item inside the directory

- **WHEN** the kind `agent` declares `to: ".claude/agents/"`, the item
  `agent:apply-orchestrator` has `from: extras/agents/apply-orchestrator.md` naming a
  file, and its listing is `["apply-orchestrator.md"]`
- **THEN** the single destination is `.claude/agents/apply-orchestrator.md`
- **AND** no other path under `.claude/agents/` is named by the result, so a repo-owned
  agent already living there is not referred to at all

#### Scenario: Without a trailing slash a file item lands at the destination itself

- **WHEN** the kind `command` declares `to: "docs/{name}.md"` and the item
  `command:release` has `from: extras/commands/release-notes.md` naming a file, with the
  listing `["release-notes.md"]`
- **THEN** the single destination is `docs/release.md`, not
  `docs/release.md/release-notes.md`

#### Scenario: A trailing slash is a no-op for a directory item

- **WHEN** the kind `schema` declares `to: "openspec/schemas/{name}/"` and the item
  `schema:tdd` from a directory has the listing `["schema.yaml"]`
- **THEN** the destination is `openspec/schemas/tdd/schema.yaml` — identical to the result
  for `to: "openspec/schemas/{name}"`, with no repeated `tdd/tdd` segment

#### Scenario: A destination with no `{name}` is used as written

- **WHEN** the kind `agent` declares `to: ".claude/agents/"` and two file items
  `agent:one` and `agent:two` are computed against it
- **THEN** the destinations are `.claude/agents/one.md` and `.claude/agents/two.md`,
  taking each file's own base name

### Requirement: `flatten` discards nested structure

When a kind declares `flatten: true`, `internal/plan` SHALL place every file of an item of
that kind directly in the interpolated destination directory under the file's base name,
discarding the directories below `from`. When two files of one item flatten onto the same
destination, planning SHALL fail with

```
source "<source>": item "<id>": flatten maps "<a>" and "<b>" to the same destination "<dest>"
```

naming the two `from`-relative paths in ascending order, and SHALL return no plan.

#### Scenario: Nested files are flattened into the destination root

- **WHEN** the kind `agent` declares `to: ".claude/agents/"` with `flatten: true`, and the
  item `agent:pack` from a directory has the listing
  `["review/outside-in.md", "apply/orchestrator.md"]`
- **THEN** the destinations are `.claude/agents/orchestrator.md` and
  `.claude/agents/outside-in.md`
- **AND** no `review/` or `apply/` directory is named by the result

#### Scenario: Without flatten the same item preserves its structure

- **WHEN** the same item is computed against a kind declaring `to: ".claude/agents/"`
  without `flatten`
- **THEN** the destinations are `.claude/agents/apply/orchestrator.md` and
  `.claude/agents/review/outside-in.md`

#### Scenario: Two files flattening onto one path is an error

- **WHEN** the kind `agent` declares `to: ".claude/agents/"` with `flatten: true`, and the
  item `agent:pack` in source `shared` has the listing `["b/dup.md", "a/dup.md"]`
- **THEN** planning fails with
  `source "shared": item "agent:pack": flatten maps "a/dup.md" and "b/dup.md" to the same destination ".claude/agents/dup.md"`
- **AND** no plan is returned, so no caller can act on a partially computed destination set

### Requirement: A list-valued `to` places one item several times

When a kind's `to` is a list, `internal/plan` SHALL compute the item's files under every
destination in the list, in declared order, and SHALL treat all of them as belonging to the
one item. When two entries of the list interpolate to the same destination for a given
item, planning SHALL fail with

```
source "<source>": item "<id>": destinations "<a>" and "<b>" both interpolate to "<dest>"
```

naming the two entries in declared order and the destination in cleaned form, and SHALL
return no plan.

Two entries SHALL be the same destination when they *mean* the same destination, not only
when they are spelled alike. Because a trailing `/` is a no-op for a directory item and
significant for a file item, `a/{name}` and `a/{name}/` SHALL be one destination for an
item whose `from` names a directory and two for one whose `from` names a file. Comparing
the entries as written would let the first case through, and the item would then produce
one path twice.

Two entries that are genuinely different destinations may still place a file at one path,
when one destination lies inside another. Planning SHALL fail with

```
source "<source>": item "<id>": destinations "<a>" and "<b>" both place a file at "<dest>"
```

naming the two entries in declared order, and SHALL return no plan. Together with the two
rules above this leaves no way for one item to produce a path twice, which is what keeps
the cross-item collision message from ever naming an item as its own partner.

#### Scenario: One item lands in two destinations

- **WHEN** the kind `schema` declares
  `to: ["openspec/schemas/{name}", "vendor/schemas/{name}"]` and the item `schema:tdd` from
  a directory has the listing `["schema.yaml"]`
- **THEN** the destinations are `openspec/schemas/tdd/schema.yaml` and
  `vendor/schemas/tdd/schema.yaml`
- **AND** both belong to `schema:tdd`, so dropping the item later removes both

#### Scenario: Two entries interpolating to one destination is an error

- **WHEN** the kind `schema` in source `shared` declares
  `to: ["openspec/schemas/{name}", "openspec/schemas/tdd"]` and the item `schema:tdd` is
  computed against it
- **THEN** planning fails with
  `source "shared": item "schema:tdd": destinations "openspec/schemas/{name}" and "openspec/schemas/tdd" both interpolate to "openspec/schemas/tdd"`
- **AND** no plan is returned

#### Scenario: Two entries differing only by a trailing slash are one destination

- **WHEN** the kind `schema` in source `shared` declares `to: ["a/{name}", "a/{name}/"]`
  and the item `schema:tdd`, whose `from` names a directory, has the listing
  `["schema.yaml"]`
- **THEN** planning fails with
  `source "shared": item "schema:tdd": destinations "a/{name}" and "a/{name}/" both interpolate to "a/tdd"`
- **AND** the failure is this within-item error rather than the cross-item collision one,
  which would name `schema:tdd` twice and give no cause

#### Scenario: The same pair is two destinations for a file item

- **WHEN** the kind `doc` declares `to: ["docs/{name}", "docs/{name}/"]` and the item
  `doc:notes`, whose `from` names the file `extras/notes.md`, has the listing
  `["notes.md"]`
- **THEN** the destinations are `docs/notes` and `docs/notes/notes.md`, and planning
  succeeds — one entry names the file, the other a directory to put it in

#### Scenario: Two entries meeting on one file is an error

- **WHEN** the kind `schema` in source `shared` declares `to: ["a", "a/b"]` and the item
  `schema:tdd` from a directory has the listing `["b/x.md", "x.md"]`, so `a` places
  `a/b/x.md` and `a/b` places it again
- **THEN** planning fails with
  `source "shared": item "schema:tdd": destinations "a" and "a/b" both place a file at "a/b/x.md"`
- **AND** no plan is returned

### Requirement: A consumer override beats the catalog

For a source whose `graft.toml` block declares `[sources.<name>.kinds]`, `internal/plan`
SHALL use the override destination for every item of that kind instead of the catalog's
`to`, applying `{name}` interpolation and the trailing-slash rule to the override exactly
as to a catalog destination. An override SHALL replace the whole `to`, including a
list-valued one, and SHALL leave the kind's `flatten` unchanged, since `graft.toml`
declares no `flatten`. An override SHALL apply only to the source that declares it.

An override naming a kind the source's catalog does not declare SHALL fail planning with

```
source "<source>": kind override "<kind>" names a kind the catalog does not declare
```

reporting the lowest-sorting such kind, and SHALL return no plan. This is typo protection:
an override silently doing nothing would leave files at the catalog's destination while
the manifest claims otherwise.

#### Scenario: An override moves a kind's items

- **WHEN** the catalog declares `agent` with `to: ".claude/agents/"` and `flatten: true`,
  the manifest declares `[sources.shared.kinds]` with `agent = ".codex/agents/"`, and the
  item `agent:apply-orchestrator` is a file item
- **THEN** the destination is `.codex/agents/apply-orchestrator.md`
- **AND** nothing is placed under `.claude/agents/`, so an override is what the consumer
  actually agreed to

#### Scenario: An override replaces a list-valued destination entirely

- **WHEN** the catalog declares `schema` with
  `to: ["openspec/schemas/{name}", "vendor/schemas/{name}"]` and the manifest overrides
  `schema = "openspec/schemas/{name}"`
- **THEN** the item `schema:tdd` with the listing `["schema.yaml"]` produces the single
  destination `openspec/schemas/tdd/schema.yaml`
- **AND** nothing is placed under `vendor/`

#### Scenario: An override keeps the catalog's flatten

- **WHEN** the catalog declares `agent` with `flatten: true` and the manifest overrides
  `agent = ".codex/agents/"`, and the item `agent:pack` from a directory has the listing
  `["review/outside-in.md"]`
- **THEN** the destination is `.codex/agents/outside-in.md`, flattened

#### Scenario: An override applies to its own source only

- **WHEN** two sources `shared` and `other` both provide `agent:x` from catalogs declaring
  `to: ".claude/agents/"`, and only `shared` overrides `agent = ".codex/agents/"`
- **THEN** `shared`'s item lands at `.codex/agents/x.md` and `other`'s at
  `.claude/agents/x.md`

#### Scenario: An override for an undeclared kind is an error

- **WHEN** the catalog of source `shared` declares only `agent` and `schema`, and the
  manifest declares `[sources.shared.kinds]` with `agnet = ".codex/agents/"`
- **THEN** planning fails with
  `source "shared": kind override "agnet" names a kind the catalog does not declare`
- **AND** no plan is returned, so no file is written at the catalog's destination while the
  manifest appears to have moved it

### Requirement: A listed path stays inside its item

`internal/plan` SHALL reject a listing entry that is not a relative path inside the item's
`from`. An entry SHALL be refused when it is empty, absolute, equal to `.`, contains a `..`
segment, or is not in cleaned form. On refusal planning SHALL fail with

```
source "<source>": item "<id>": file "<path>" is not a relative path inside the item
```

naming the first offending entry in ascending path order, and SHALL return no plan.

The repo-root boundary does not cover this, in either direction. Joining absorbs leading
`..` segments, so an entry with fewer of them than its destination has path segments lands
somewhere else **inside** the repository — `.git/hooks/` among them — while every
repo-root check passes; and under `flatten` the destination is contained whatever the
entry said, because only its base name survives. The same entry is also joined to `from`
to name the **file to read**, which nothing downstream re-checks, so an unchecked one aims
a read outside the source's fetched tree and at whatever sits beside it in the cache.

Refusing the entry rather than the path it produces closes both halves at one point, and
names what a source actually wrote rather than a path derived from it.

#### Scenario: An entry climbing out of its item is refused

- **WHEN** the kind `schema` declares `to: "openspec/schemas/{name}"` and the item
  `schema:tdd` in source `shared` has the listing `["../../../etc/passwd"]`, which joins
  and cleans to `etc/passwd` — inside the repository, so the repo-root check passes
- **THEN** planning fails with
  `source "shared": item "schema:tdd": file "../../../etc/passwd" is not a relative path inside the item`
- **AND** no plan is returned, and no write names `etc/passwd`

#### Scenario: An entry reaching `.git/` under an unrelated destination is refused

- **WHEN** the kind `agent` declares `to: ".claude/agents/"` and the item `agent:pack` in
  source `shared` has the listing `["../../.git/hooks/pre-commit"]`
- **THEN** planning fails with
  `source "shared": item "agent:pack": file "../../.git/hooks/pre-commit" is not a relative path inside the item`
- **AND** the catalog's `to` never named `.git`, so no rule about what a destination may
  *name* would have caught it

#### Scenario: An entry climbing into a sibling directory is refused

- **WHEN** the kind `agent` declares `to: ".claude/agents/"` and the item `agent:pack` in
  source `shared` has the listing `["../hooks/x.md"]`
- **THEN** planning fails with
  `source "shared": item "agent:pack": file "../hooks/x.md" is not a relative path inside the item`
- **AND** the consumer agreed to `.claude/agents/`, not to `.claude/hooks/`, so a file
  landing there would defeat the one mitigation an untrusted source has — that the
  destination is shown before install

#### Scenario: A flattened entry whose source path leaves the item is refused

- **WHEN** the kind `agent` declares `to: ".claude/agents/"` with `flatten: true` and the
  item `agent:pack` in source `shared`, with `from: extras/agents/pack`, has the listing
  `["../../secret.md"]`
- **THEN** planning fails with
  `source "shared": item "agent:pack": file "../../secret.md" is not a relative path inside the item`
- **AND** the destination `.claude/agents/secret.md` was contained, because `flatten`
  keeps only the base name — what escaped is the file graft would have read

#### Scenario: An absolute or uncleaned entry is refused

- **WHEN** the item `schema:tdd` in source `shared` has the listing `["/etc/passwd"]`
- **THEN** planning fails with
  `source "shared": item "schema:tdd": file "/etc/passwd" is not a relative path inside the item`
- **AND** the same holds for `["./schema.yaml"]`, which names the same file as
  `schema.yaml` while looking different to every later comparison

#### Scenario: A repeated entry is one write, not a collision

- **WHEN** the item `schema:tdd` has the listing `["schema.yaml", "schema.yaml"]`
- **THEN** the single destination is `openspec/schemas/tdd/schema.yaml` and planning
  succeeds — a repeated entry names one file, so it is one write

### Requirement: No destination escapes the repo root

`internal/plan` SHALL reject any destination that is not a relative path inside the
consumer's repository. A destination SHALL be refused when it is empty, absolute, equal to
`.`, contains a `..` segment, or is not in cleaned form apart from at most one trailing
`/`, which a `to` uses to mean "into this directory". The interpolated destination
itself SHALL be checked as well as every file path computed under it, so a `to` that
escapes is refused even for an item contributing no files. On refusal planning SHALL fail
with

```
source "<source>": item "<id>": destination "<path>" escapes the repo root
```

naming the first offending path in destination order, and SHALL return no plan.

#### Scenario: A `to` climbing out of the repo is refused

- **WHEN** the catalog of source `shared` declares `agent` with `to: "../outside/agents/"`
  and the item `agent:x` is a file item with the listing `["x.md"]`
- **THEN** planning fails with
  `source "shared": item "agent:x": destination "../outside/agents/" escapes the repo root`
- **AND** no plan is returned and nothing outside the repository is named, created, or
  modified

#### Scenario: An absolute `to` is refused

- **WHEN** the catalog of source `shared` declares `agent` with `to: "/etc/agents/"`
- **THEN** planning fails with
  `source "shared": item "agent:x": destination "/etc/agents/" escapes the repo root`

#### Scenario: An escaping consumer override is refused

- **WHEN** the manifest for source `shared` overrides `agent = "../../elsewhere/"`
- **THEN** planning fails with
  `source "shared": item "agent:x": destination "../../elsewhere/" escapes the repo root`
- **AND** the override being the consumer's own words does not exempt it — the repo root is
  the boundary, not the source

#### Scenario: A listing entry aiming outside the repository is refused

- **WHEN** the kind `schema` declares `to: "openspec/schemas/{name}"` and the item
  `schema:tdd` in source `shared` has the listing `["../../../../../etc/passwd"]`, whose
  five `..` segments outrun the destination's three
- **THEN** planning fails with
  `source "shared": item "schema:tdd": destination "../../etc/passwd" escapes the repo root`
- **AND** no plan is returned, so a malformed listing cannot aim a write outside the tree.
  An entry that stays inside the repository is refused too, by *A listed path stays inside
  its item* — the repo-root boundary is the floor, not the whole rule

#### Scenario: A `to` escaping with no files to place is still refused

- **WHEN** the catalog of source `shared` declares `agent` with `to: "../outside/agents/"`
  and the item `agent:x` has an empty listing
- **THEN** planning fails with
  `source "shared": item "agent:x": destination "../outside/agents/" escapes the repo root`
- **AND** the check does not depend on the item happening to contribute a file

#### Scenario: A destination at the repo root itself is accepted

- **WHEN** the kind `doc` declares `to: "{name}.md"` and the item `doc:README` is a file
  item with the listing `["README.md"]`
- **THEN** the destination is `README.md`, at the top of the repository, and planning
  succeeds
