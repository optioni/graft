## ADDED Requirements

### Requirement: Manifest loading and absence

`internal/manifest` SHALL load `graft.toml` from a path and return a parsed manifest, or an
error. A `graft.toml` that does not exist SHALL be an error, because the consumer's request
is the input graft cannot infer. Loading SHALL NOT create, modify, or delete any file.

#### Scenario: Minimal valid manifest loads

- **WHEN** the tree contains only `graft.toml` holding:
  ```toml
  [sources.openspec-schemas]
  git     = "github.com/optioni/openspec-schemas"
  rev     = "v1.2.0"
  install = ["schema:tdd", "agent:*"]
  ```
- **THEN** loading returns a manifest with exactly one source named `openspec-schemas`,
  whose `git` is `github.com/optioni/openspec-schemas`, `rev` is `v1.2.0`, and whose
  `install` is the two selectors in the order written
- **AND** the tree is unchanged — no file is created, modified, or deleted

#### Scenario: Manifest with no sources is valid

- **WHEN** `graft.toml` exists and is empty (zero bytes)
- **THEN** loading succeeds and returns a manifest with zero sources
- **AND** no error is returned, because a repo that declares nothing has nothing to sync

#### Scenario: Missing manifest is an error

- **WHEN** the tree contains no `graft.toml`
- **THEN** loading returns an error whose message is exactly `graft.toml not found`
- **AND** the tree is unchanged

#### Scenario: Malformed TOML is an error

- **WHEN** `graft.toml` contains `[sources.a` with no closing bracket
- **THEN** loading returns an error whose message begins with `graft.toml: ` and carries the
  decoder's own description of the syntax problem
- **AND** no partially populated manifest is returned

### Requirement: Required source fields

Every `[sources.<name>]` block SHALL declare a non-empty `git`, a non-empty `rev`, and an
`install` list holding at least one selector. A missing or empty value SHALL be an error
naming the source and the field. The source name itself SHALL be non-empty.

#### Scenario: Missing git is an error

- **WHEN** `graft.toml` declares `[sources.openspec-schemas]` with `rev` and `install` but no
  `git`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": git is required`
- **AND** no source is returned

#### Scenario: Missing rev is an error

- **WHEN** `graft.toml` declares `[sources.openspec-schemas]` with `git` and `install` but no
  `rev`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": rev is required`

#### Scenario: Empty install list is an error

- **WHEN** `graft.toml` declares `[sources.openspec-schemas]` with `git`, `rev`, and
  `install = []`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": install must list at least one selector`
- **AND** the same error is returned when `install` is absent entirely

#### Scenario: Empty source name is an error

- **WHEN** `graft.toml` declares `[sources.""]` with every field populated
- **THEN** the error message is exactly `graft.toml: source name is empty`

### Requirement: Selector syntax

Each entry of `install` SHALL be syntactically `kind:name`: exactly one colon, a non-empty
kind, and a non-empty name. A name containing `*` or `?` SHALL be accepted as written —
matching it against a catalog is out of scope for this capability. A selector that is not
syntactically `kind:name` SHALL be an error naming the source and the offending selector. A
selector repeated within one source SHALL be an error.

#### Scenario: Plain and glob selectors are accepted

- **WHEN** `install = ["schema:tdd", "agent:*", "agent:outside-in-*"]`
- **THEN** loading succeeds and the manifest holds all three selectors verbatim, with no
  expansion, normalization, or reordering applied

#### Scenario: A selector with no kind separator is an error

- **WHEN** `install = ["tdd"]` for source `openspec-schemas`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": invalid selector "tdd": want kind:name`

#### Scenario: A selector with an empty half is an error

- **WHEN** `install` holds `"schema:"`, or `":tdd"`, or `"schema:tdd:extra"`
- **THEN** each returns the error
  `graft.toml: source "openspec-schemas": invalid selector "<selector>": want kind:name`,
  with `<selector>` the offending string

#### Scenario: A duplicate selector is an error

- **WHEN** `install = ["agent:*", "agent:*"]` for source `openspec-schemas`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": duplicate selector "agent:*"`

### Requirement: Per-source kind destination overrides

A source MAY declare `[sources.<name>.kinds]`, mapping a kind name to a destination that
overrides whatever the source's catalog proposes. The override SHALL be carried verbatim; a
kind whose destination is the empty string SHALL be an error. Absence of the table SHALL
mean no overrides, not an empty-destination override.

#### Scenario: An override is carried verbatim

- **WHEN** `graft.toml` declares `[sources.openspec-schemas.kinds]` with
  `agent = ".codex/agents/"`
- **THEN** the loaded source reports one kind override, `agent` to `.codex/agents/`, with the
  trailing slash preserved
- **AND** no destination is resolved, joined, or checked against the repo root here

#### Scenario: No kinds table means no overrides

- **WHEN** a source declares no `kinds` table
- **THEN** the loaded source reports zero kind overrides

#### Scenario: An empty override destination is an error

- **WHEN** `[sources.openspec-schemas.kinds]` declares `agent = ""`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": kind "agent": destination is required`

### Requirement: Unknown keys are rejected

Decoding SHALL be strict: any key in `graft.toml` that this graft does not define SHALL be
an error naming the key. A misspelled field must not be silently ignored, because the
resulting sync would look successful while installing the wrong thing.

#### Scenario: A misspelled source field is an error

- **WHEN** a source declares `revision = "v1.2.0"` alongside `git` and `install`
- **THEN** the error message is exactly
  `graft.toml: source "openspec-schemas": unknown key "revision"`

#### Scenario: An unknown top-level key is an error

- **WHEN** `graft.toml` declares `version = 1` at top level
- **THEN** the error message is exactly `graft.toml: unknown key "version"`
- **AND** `graft.toml` therefore carries no format version, unlike `graft.lock`

### Requirement: The git field is preserved verbatim

The `git` value SHALL be stored exactly as written. Shorthand SHALL NOT be expanded to a
clone URL, and no host, owner, or repo decomposition SHALL be performed here; that belongs
to the package that talks to git.

#### Scenario: Shorthand is not expanded

- **WHEN** `git = "github.com/optioni/openspec-schemas"`
- **THEN** the loaded source's `git` is the identical string, with no `https://` prefix and
  no `.git` suffix added

#### Scenario: A full URL is not rewritten

- **WHEN** `git = "git@github.com:optioni/openspec-schemas.git"`
- **THEN** the loaded source's `git` is that identical string
