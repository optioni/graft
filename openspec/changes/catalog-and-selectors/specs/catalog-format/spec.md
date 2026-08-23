## ADDED Requirements

### Requirement: Catalog loading and absence

`internal/catalog` SHALL load `catalog.yaml` from a path and return a parsed catalog, or an
error. A `catalog.yaml` that does not exist SHALL be an error saying the source is not
graftable — graft never falls back to guessing a source's layout. Loading SHALL NOT create,
modify, or delete any file, and SHALL read no path other than the one it was given.

#### Scenario: A valid catalog loads

- **WHEN** the tree contains only `catalog.yaml` holding:
  ```yaml
  version: 1
  kinds:
    schema:
      to: "openspec/schemas/{name}"
    agent:
      to: ".claude/agents/"
      flatten: true
  provides:
    - { kind: schema, name: tdd, from: extras/openspec-schemas/tdd }
    - { kind: agent, name: apply-orchestrator, from: extras/agents/apply-orchestrator.md }
  ```
- **THEN** loading returns a catalog at version `1` with two kinds and two items
- **AND** the tree is unchanged — no file is created, modified, or deleted

#### Scenario: A catalog with zero provides loads

- **WHEN** `catalog.yaml` holds `version: 1` and a `kinds` map but no `provides` key
- **THEN** loading succeeds and returns a catalog with zero items
- **AND** no error is returned, because a source that offers nothing yet is a legitimate
  state; the failure belongs to any selector aimed at it

#### Scenario: A catalog with neither kinds nor provides loads

- **WHEN** `catalog.yaml` holds only `version: 1`
- **THEN** loading succeeds and returns a catalog with zero kinds and zero items

#### Scenario: A missing catalog is the not-graftable error

- **WHEN** loading is asked for `catalog.yaml` in a directory that has no such file
- **THEN** the error message is exactly `catalog.yaml not found: the source is not graftable`
- **AND** the tree is unchanged — no `catalog.yaml` is created

#### Scenario: Malformed YAML is an error

- **WHEN** `catalog.yaml` holds `version: 1` followed by the line `kinds: [unclosed`
- **THEN** an error is returned whose message begins with `catalog.yaml: `
- **AND** no catalog is returned, so no caller can act on a half-decoded file

#### Scenario: A catalog that is not a mapping is an error

- **WHEN** `catalog.yaml` holds a valid YAML sequence rather than a mapping, such as
  `- schema:tdd`
- **THEN** the error message is exactly `catalog.yaml: top level must be a mapping`
- **AND** no catalog is returned

#### Scenario: An empty catalog file is a missing-version error

- **WHEN** `catalog.yaml` exists and is zero bytes
- **THEN** the error message is exactly `catalog.yaml: version is required`
- **AND** the file's emptiness is treated as an empty mapping rather than as a syntax
  error, so the message names what is actually missing

### Requirement: Catalog format version gating

The catalog SHALL carry `version`, and this graft SHALL accept only version `1`. A version
this binary does not know SHALL fail and say to upgrade graft, and SHALL be checked before
any other validation so a future format's new keys are reported as "upgrade" rather than as
unknown keys.

#### Scenario: A missing version is an error

- **WHEN** `catalog.yaml` declares `kinds` and `provides` but no `version` key
- **THEN** the error message is exactly `catalog.yaml: version is required`
- **AND** no catalog is returned

#### Scenario: A newer version fails and says to upgrade

- **WHEN** `catalog.yaml` holds `version: 2` together with a top-level key this graft does
  not know, such as `requires: []`
- **THEN** the error message is exactly
  `catalog.yaml: version 2 is not supported by this graft; upgrade graft`
- **AND** the unknown key is not reported instead, because version is checked first
- **AND** no catalog is returned

#### Scenario: A version below 1 is an error

- **WHEN** `catalog.yaml` holds `version: 0`
- **THEN** the error message is exactly `catalog.yaml: version 0 is not a known catalog version`
- **AND** no catalog is returned

### Requirement: Kind declarations

`kinds.<kind>` SHALL declare where a class of thing belongs. `to` SHALL be either a
non-empty string or a non-empty list of non-empty strings, and SHALL be carried verbatim —
graft SHALL NOT interpolate `{name}`, resolve a trailing `/`, or clean the path at parse
time, because computing a destination belongs to a later change. `flatten` SHALL be a
boolean defaulting to `false`.

#### Scenario: A string-valued to is carried verbatim

- **WHEN** a catalog declares kind `schema` with `to: "openspec/schemas/{name}"`
- **THEN** the parsed kind carries exactly one destination, the string
  `openspec/schemas/{name}`, with `{name}` uninterpolated
- **AND** its `flatten` is `false`, because the key was absent

#### Scenario: A trailing slash is preserved

- **WHEN** a catalog declares kind `agent` with `to: ".claude/agents/"` and `flatten: true`
- **THEN** the parsed kind carries the destination `.claude/agents/` with its trailing slash
  intact, because the slash is what later means "into this directory"
- **AND** its `flatten` is `true`

#### Scenario: A list-valued to is carried in declared order

- **WHEN** a catalog declares kind `agent` with
  `to: [".claude/agents/", ".codex/agents/"]`
- **THEN** the parsed kind carries both destinations in that order

#### Scenario: An empty kind name is an error

- **WHEN** a catalog declares a kind whose key is the empty string (`"": { to: "x/" }`)
- **THEN** the error message is exactly `catalog.yaml: kind name is empty`
- **AND** no catalog is returned

#### Scenario: A missing or empty to is an error

- **WHEN** a catalog declares kind `agent` with no `to` key, or with `to: ""`, or with
  `to: []`
- **THEN** the error message is exactly `catalog.yaml: kind "agent": to is required`
- **AND** no catalog is returned

#### Scenario: An empty destination inside a list is an error

- **WHEN** a catalog declares kind `agent` with `to: [".claude/agents/", ""]`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": to contains an empty destination`
- **AND** no catalog is returned

#### Scenario: A to of the wrong type is an error

- **WHEN** a catalog declares kind `agent` with `to: { dir: ".claude/agents/" }`, or with
  `to: 7`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": to must be a string or a list of strings`
- **AND** no catalog is returned

#### Scenario: A repeated destination within one kind is an error

- **WHEN** a catalog declares kind `agent` with
  `to: [".claude/agents/", ".claude/agents/"]`
- **THEN** the error message is exactly
  `catalog.yaml: kind "agent": duplicate destination ".claude/agents/"`
- **AND** no catalog is returned, because the kind would otherwise write every one of its
  items to the same path twice

### Requirement: Provided items

Each `provides` entry SHALL carry a non-empty `kind`, `name`, and `from`. The item's
identity SHALL be `kind:name`, SHALL be unique within the catalog, and SHALL be valid under
the same `kind:name` grammar `graft.toml` selectors and `graft.lock` item ids use. An entry
naming a kind the catalog does not declare SHALL be an error. `from` SHALL be a cleaned
relative path with no `..` segment, so a catalog cannot read outside the source tree. The
parsed items SHALL be ordered by item id regardless of the order they appear in the file.

#### Scenario: Items are carried with kind, name, and from

- **WHEN** a catalog provides
  `{ kind: schema, name: tdd, from: extras/openspec-schemas/tdd }`
- **THEN** the parsed catalog holds one item whose id is `schema:tdd`, whose kind is
  `schema`, whose name is `tdd`, and whose `from` is `extras/openspec-schemas/tdd`

#### Scenario: Items are ordered by id

- **WHEN** a catalog lists `schema:tdd` before `agent:apply-orchestrator` before
  `agent:tdd-reviewer`
- **THEN** the parsed items appear as `agent:apply-orchestrator`, `agent:tdd-reviewer`,
  `schema:tdd`
- **AND** the file order is not preserved, so nothing downstream can depend on how a source
  happened to write its list

#### Scenario: A missing field is an error

- **WHEN** the first `provides` entry omits `kind`, or omits `name`, or omits `from`
- **THEN** the error message is exactly `catalog.yaml: provides[0]: kind is required`,
  `catalog.yaml: provides[0]: name is required`, or
  `catalog.yaml: provides[0]: from is required` respectively, identifying the entry by its
  0-based position because it may not yet have a usable identity
- **AND** no catalog is returned

#### Scenario: A kind or name containing a colon is an error

- **WHEN** the first `provides` entry holds `{ kind: agent, name: "a:b", from: x.md }`
- **THEN** the error message is exactly
  `catalog.yaml: provides[0]: invalid item id "agent:a:b": want kind:name`
- **AND** no catalog is returned, because an ambiguous id could not be selected or locked

#### Scenario: An item naming an undeclared kind is an error

- **WHEN** a catalog declares only kind `agent` and provides
  `{ kind: schema, name: tdd, from: extras/tdd }`
- **THEN** the error message is exactly
  `catalog.yaml: item "schema:tdd": kind "schema" is not declared`
- **AND** no catalog is returned, because the item has no destination

#### Scenario: A duplicate item is an error

- **WHEN** a catalog provides `{ kind: agent, name: tdd, from: a.md }` and then
  `{ kind: agent, name: tdd, from: b.md }`
- **THEN** the error message is exactly `catalog.yaml: duplicate item "agent:tdd"`
- **AND** no catalog is returned

#### Scenario: A from outside the source tree is an error

- **WHEN** a `provides` entry holds `from: ../outside`, or `from: /etc/passwd`, or
  `from: .`, or `from: ./extras/tdd`
- **THEN** the error message is exactly
  `catalog.yaml: item "schema:tdd": from "<value>" is not a relative path inside the source`
  with `<value>` the string as written
- **AND** no catalog is returned, because `from` staying inside the source tree is an
  invariant, and requiring cleaned form removes the aliases that would slip past it

### Requirement: An item name is restricted to what a selector and a destination can carry

An item's `name` SHALL be a single segment of letters, digits, `.`, `-`, or `_`, and
SHALL NOT be `.` or `..`. The name is chosen by the source repository and is read three
different ways downstream — as a `path.Match` target when a selector is expanded, as the
`{name}` fragment of a destination, and as half of the id the lock records — so a name
that misbehaves in any of them SHALL be refused where the catalog mints it rather than
where a later package meets it.

#### Scenario: A name holding a path separator is an error

- **WHEN** a `provides` entry holds `{ kind: agent, name: "nested/thing", from: a.md }`
- **THEN** the error message is exactly
  `catalog.yaml: provides[0]: invalid name "nested/thing": want letters, digits, dot, dash, or underscore`
- **AND** no catalog is returned, because `path.Match`'s `*` does not cross `/`: such an
  item would be invisible to an `agent:*` selector while that same selector matched its
  siblings, so the install would silently drop it and the no-match error would never fire

#### Scenario: A name holding a glob metacharacter is an error

- **WHEN** a `provides` entry holds a name of `a*b`, or of `[x]`
- **THEN** the error message is exactly
  `catalog.yaml: provides[0]: invalid name "<value>": want letters, digits, dot, dash, or underscore`
  with `<value>` the name as written
- **AND** no catalog is returned, because a consumer's exact selector `agent:a*b` would
  otherwise also select `agent:ab`, and a name of `[x]` could not be selected at all

#### Scenario: A name of dot or dot-dot is an error

- **WHEN** a `provides` entry holds a name of `.`, or of `..`
- **THEN** the error message is exactly
  `catalog.yaml: provides[0]: invalid name "<value>": a name may not be "." or ".."`
  with `<value>` the name as written
- **AND** no catalog is returned, because `{name}` is interpolated into a destination by
  a later package, and such a name would aim a write above the directory the item's own
  kind declared — defeating the guarantee that the destination shown before install is
  the destination written

#### Scenario: Ordinary names are accepted

- **WHEN** a catalog provides items named `tdd`, `apply-orchestrator`, `outside_in`,
  `v1.2`, and `TDD9`
- **THEN** every one of them parses, so the rule that closes the holes above does not
  also reject the vocabulary a source is expected to use

### Requirement: Unknown keys are rejected

Decoding SHALL be strict: any key the catalog format does not define SHALL be an error
naming the key and where it appeared, rather than being silently ignored. A misspelled key
in a source's catalog would otherwise install nothing and say nothing.

#### Scenario: An unknown top-level key is an error

- **WHEN** `catalog.yaml` holds `version: 1` and a top-level `kind:` key (singular)
- **THEN** the error message is exactly `catalog.yaml: unknown key "kind"`
- **AND** no catalog is returned

#### Scenario: An unknown key inside a kind is an error

- **WHEN** a catalog declares kind `agent` with `to: ".claude/agents/"` and `flat: true`
- **THEN** the error message is exactly `catalog.yaml: kind "agent": unknown key "flat"`
- **AND** no catalog is returned

#### Scenario: An unknown key inside a provides entry is an error

- **WHEN** the second `provides` entry holds
  `{ kind: agent, name: tdd, path: agents/tdd.md }`
- **THEN** the error message is exactly `catalog.yaml: provides[1]: unknown key "path"`
- **AND** no catalog is returned
