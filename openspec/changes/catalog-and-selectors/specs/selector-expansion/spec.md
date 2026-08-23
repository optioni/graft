## ADDED Requirements

### Requirement: Selector expansion against a catalog

`internal/catalog` SHALL expand a source's `install` selectors against that source's parsed
`provides`, returning the deduplicated union of the items they match, ordered by item id.
Expansion SHALL be a pure function of a catalog, a source name, and a selector list: it
SHALL read no file, run no command, and create, modify, or delete nothing.

#### Scenario: A plain selector selects exactly one item

- **WHEN** a catalog provides `agent:apply-orchestrator`, `agent:tdd-reviewer`, and
  `schema:tdd`, and the selectors are `["schema:tdd"]`
- **THEN** expansion returns exactly the item `schema:tdd`
- **AND** the tree is unchanged — no file is created, modified, or deleted

#### Scenario: Several selectors produce the union ordered by id

- **WHEN** the same catalog is expanded with `["schema:tdd", "agent:tdd-reviewer"]`
- **THEN** expansion returns `agent:tdd-reviewer` then `schema:tdd`, in that order
- **AND** the declared selector order does not affect the result order, so nothing
  downstream depends on how a consumer wrote its `install` list

#### Scenario: Overlapping selectors yield each item once

- **WHEN** the same catalog is expanded with `["agent:*", "agent:tdd-reviewer"]`
- **THEN** expansion returns `agent:apply-orchestrator` and `agent:tdd-reviewer` exactly
  once each
- **AND** no error is returned, because both selectors matched something

#### Scenario: An empty selector list expands to nothing

- **WHEN** the same catalog is expanded with an empty selector list
- **THEN** expansion returns zero items and no error
- **AND** nothing is read or written — an empty request is not a failure at this layer;
  `graft.toml` is where an empty `install` is already rejected

### Requirement: Globs in the name position

A selector SHALL be `kind:name`, where the name position MAY contain the glob
metacharacters `*` and `?`. The kind position SHALL be matched literally, because SPEC.md
places globbing in the name position only. A syntactically malformed glob pattern SHALL be
an error naming the selector.

#### Scenario: A trailing star selects every item of a kind

- **WHEN** a catalog providing `agent:apply-orchestrator`, `agent:tdd-reviewer`, and
  `schema:tdd` is expanded with `["agent:*"]`
- **THEN** expansion returns `agent:apply-orchestrator` and `agent:tdd-reviewer`, ordered by
  id, and does not return `schema:tdd`

#### Scenario: A prefix glob selects a subset

- **WHEN** the same catalog is expanded with `["agent:tdd-*"]`
- **THEN** expansion returns only `agent:tdd-reviewer`

#### Scenario: A question mark matches exactly one character

- **WHEN** a catalog providing `schema:tdd` and `schema:td` is expanded with
  `["schema:td?"]`
- **THEN** expansion returns only `schema:tdd`, because `?` matches one character and never
  zero

#### Scenario: The kind position is matched literally

- **WHEN** a catalog providing `agent:tdd-reviewer` and `schema:tdd` is expanded with
  `["*:tdd"]`
- **THEN** an error is returned rather than both kinds being matched, because no kind is
  literally named `*`
- **AND** the error is the no-match error, exactly
  `source "openspec-schemas": selector "*:tdd" matches no item; catalog provides agent:tdd-reviewer, schema:tdd`

#### Scenario: A malformed glob pattern is an error

- **WHEN** the same catalog is expanded with `["agent:[tdd"]`
- **THEN** the error message is exactly
  `source "openspec-schemas": invalid selector pattern "agent:[tdd"`
- **AND** no items are returned, and the selector is not silently treated as a literal name

### Requirement: A selector matching nothing is an error

A selector that matches no item SHALL be an error, never a warning and never a silent empty
result. The error SHALL name the source and the selector and SHALL list every item the
catalog does provide, ordered by id, so a typo is visible against the real vocabulary.
Every selector SHALL be checked, so one selector matching cannot excuse another that does
not.

#### Scenario: A misspelled selector is an error listing what the catalog provides

- **WHEN** a catalog providing `agent:apply-orchestrator`, `agent:tdd-reviewer`, and
  `schema:tdd` for source `openspec-schemas` is expanded with `["schema:tdd-workflow"]`
- **THEN** the error message is exactly
  `source "openspec-schemas": selector "schema:tdd-workflow" matches no item; catalog provides agent:apply-orchestrator, agent:tdd-reviewer, schema:tdd`
- **AND** no items are returned, so no caller can proceed on a partial expansion

#### Scenario: One selector matching does not excuse another that does not

- **WHEN** the same catalog is expanded with `["agent:*", "schema:missing"]`
- **THEN** the error names `schema:missing`, in the same form as above
- **AND** no items are returned, even though `agent:*` matched two items

#### Scenario: A glob matching nothing is an error

- **WHEN** the same catalog is expanded with `["hook:*"]`
- **THEN** the error names `hook:*` in the same form
- **AND** no items are returned — a glob that adopts future items still has to match one
  today

#### Scenario: Any selector against a catalog providing zero items is an error

- **WHEN** a catalog with an empty `provides` list, for source `empty-source`, is expanded
  with `["agent:*"]`
- **THEN** the error message is exactly
  `source "empty-source": selector "agent:*" matches no item; catalog provides no items`
- **AND** no items are returned
