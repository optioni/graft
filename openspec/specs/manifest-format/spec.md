# Manifest Format Specification

## Purpose

`graft.toml` is the consumer's request: which sources this repo wants items from, at
which rev, and which items. It is human-authored and committed. This capability covers
reading and validating it — every rule that can be decided from the file alone, with no
catalog, no git, and no network. It reads; it never writes.

## Requirements

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

### Requirement: A source's rev can be moved in place without disturbing the rest of graft.toml

`internal/manifest` SHALL provide a pure function that takes the bytes of a `graft.toml`, a
source name, and a rev, and returns the bytes of that file with **only** the value of that
source's `rev` replaced. It SHALL create, modify, and delete nothing; returning bytes is the
whole of what it does, and `internal/apply` is the only package that puts them anywhere.

Every other byte SHALL survive: comments, blank lines, key order, the alignment of `=` that
SPEC.md's own example uses, other keys of the same source, every other source, a comment
trailing the `rev` line itself, the file's line endings, and the absence of a final newline.
`graft.toml` is written by a human and reviewed in a diff, so a rewrite that reformats it would
bury the one line that moved.

On success the returned bytes SHALL differ from the input in exactly one line, and within that
line only in the span between the opening and closing quotation marks of the `rev` value.

#### Scenario: The aligned value is replaced and nothing else moves

- **WHEN** a `graft.toml` holding

  ```toml
  # pinned deliberately
  [sources.shared]
  git     = "github.com/optioni/shared"
  rev     = "v1.0.0"
  install = ["schema:tdd"]
  ```

  has `shared`'s rev moved to `v1.1.0`
- **THEN** the result differs in exactly one line, which reads `rev     = "v1.1.0"` with the
  original alignment
- **AND** the comment, the `git` line, and the `install` line are byte-identical

#### Scenario: A comment trailing the rev line survives

- **WHEN** the rev line reads `rev     = "v1.0.0"  # do not bump without reading the changelog`
  and the rev is moved to `v1.1.0`
- **THEN** the line reads `rev     = "v1.1.0"  # do not bump without reading the changelog`
- **AND** a rev value that itself contains `#` is replaced correctly, because the value's end is
  found at its closing quotation mark rather than at the first `#` on the line

#### Scenario: Other sources are untouched

- **WHEN** a manifest declaring `extra` before `shared`, with a comment between them, has
  `shared`'s rev moved
- **THEN** the whole of `[sources.extra]` and the comment are byte-identical
- **AND** re-parsing the result yields `extra` at its original rev and `shared` at the new one

#### Scenario: Line endings and a missing final newline are preserved

- **WHEN** a manifest whose lines end `\r\n` has a rev moved, and separately one whose last line
  carries no trailing newline
- **THEN** every line of the first result still ends `\r\n`, the rev line included
- **AND** the second result still ends without a trailing newline, so no byte is added

### Requirement: The rev key is located exactly, and every other shape is refused rather than guessed at

The replacement SHALL be recognised **only** where `rev` is a plain key of the source's own
standard table. The table header SHALL be matched as the exact key path `sources`, `<name>` —
the name may be written bare or quoted, and whitespace inside the brackets is tolerated — and
the source's table SHALL be considered ended at the next table header of any kind. A sub-table
such as `[sources.<name>.kinds]` is a **different** table: a `rev` key inside it SHALL NOT be
touched, and `kinds` may legally hold a kind named `rev`. An array-of-tables header
(`[[sources.<name>]]`) SHALL NOT match a standard table header.

Within the source's table, the key SHALL be matched as `rev` with optional leading whitespace,
followed by optional whitespace and `=`. A commented-out line SHALL be skipped, so
`# rev = "v1.0.0"` above the real key SHALL NOT be the line that is edited. The first plain
`rev` key found in the table SHALL be the one replaced.

The value SHALL be a single-line quoted string whose closing quotation mark is on the same line.
A multi-line string, a value that is not a quoted string, and a `rev` key that is not found at
all SHALL each be refused rather than rewritten.

Bytes inside a multi-line string SHALL NOT be read as syntax. A `[sources.<name>]` header or a
`rev` key written inside a `"""` or `'''` value belongs to whichever key opened that string, so
the scan SHALL track an open delimiter across lines and treat every line inside one as neither a
header nor a key. This is the failure mode with no upper bound on its damage: rewriting a line
inside another key's value corrupts a value in a source that was never named, and the result can
still parse. A delimiter the scan gets wrong SHALL fail closed — a line skipped in error produces
the refusal below, never a rewrite.

Any shape not covered above — a source written as an inline table, a dotted key at the top
level, a multi-line value, a source the file does not declare at all — SHALL be an error reading

```
graft.toml: source "<name>": cannot move the pin: rev is not a plain key under [sources.<name>]
```

Guessing at a shape it cannot rewrite exactly is the one thing this function may not do: a wrong
guess corrupts the consumer's own request, and a wrong *target* — an edited comment, an edited
sub-table key — is worse than a refusal because it looks like success.

On any error the function SHALL return no bytes at all, so no caller can write a half-edited
file.

#### Scenario: A rev key in a kinds sub-table is not the one edited

- **WHEN** a manifest holds

  ```toml
  [sources.shared]
  git     = "github.com/optioni/shared"
  rev     = "v1.0.0"
  install = ["schema:tdd"]

  [sources.shared.kinds]
  rev = ".codex/revs/"
  ```

  and `shared`'s rev is moved to `v1.1.0`
- **THEN** the `[sources.shared]` table's `rev` line reads `rev     = "v1.1.0"`
- **AND** the `[sources.shared.kinds]` line still reads `rev = ".codex/revs/"`, byte-identical
- **AND** re-parsing yields the `kinds` override unchanged

#### Scenario: A commented-out rev above the real one is skipped

- **WHEN** the source's table holds `# rev     = "v0.9.0"` on the line above
  `rev     = "v1.0.0"` and the rev is moved to `v1.1.0`
- **THEN** the commented line is byte-identical and the real key reads `rev     = "v1.1.0"`

#### Scenario: A quoted table key is recognised

- **WHEN** a manifest declares the source as `[sources."my-source"]` and that source's rev is
  moved
- **THEN** the rev line under that table is the one replaced
- **AND** a header written `[ sources . "my-source" ]` is recognised the same way

#### Scenario: A source written as an inline table is refused

- **WHEN** a manifest declares `[sources]` with `shared = { git = "…", rev = "v1.0.0", install = ["schema:tdd"] }`
  and `shared`'s rev is moved
- **THEN** the error reads
  `graft.toml: source "shared": cannot move the pin: rev is not a plain key under [sources.shared]`
- **AND** no bytes are returned, so nothing downstream can write a half-edited file

#### Scenario: A source the file does not declare is refused the same way

- **WHEN** a rev is moved for a source name no `[sources.<name>]` table declares, and separately
  for one declared only as `[[sources.<name>]]`
- **THEN** each returns the same `cannot move the pin` error, naming that source

#### Scenario: A multi-line rev value is refused rather than half-rewritten

- **WHEN** the source's table holds `rev = """` on one line and `v1.0.0"""` on the next — which
  the manifest parser accepts — and the rev is moved
- **THEN** the same `cannot move the pin` error is returned and no bytes are produced
- **AND** this matters because a line-oriented rewrite would strand the second line and produce
  a `graft.toml` that no longer parses

#### Scenario: A rev inside another key's multi-line string is not the one edited

- **WHEN** a manifest holds

  ```toml
  [sources.a]
  git = """
  [sources.b]
  rev = "zzz"
  """
  rev = "v1"
  ```

  and `b`'s rev is moved to `v2`
- **THEN** the error reads
  `graft.toml: source "b": cannot move the pin: rev is not a plain key under [sources.b]`
- **AND** no bytes are returned, so `a`'s quoted value cannot be corrupted by an edit aimed at a
  source the file does not declare
- **AND** the same holds for a literal multi-line string opened with `'''`

#### Scenario: A pin past a multi-line string still moves

- **WHEN** the source's own table holds a `"""` value containing the text `rev = "decoy"`, and
  the real `rev     = "v1.0.0"` follows the string's closing delimiter
- **THEN** the real key reads `rev     = "v1.1.0"` and the quoted value is byte-identical

### Requirement: A rev that cannot be written literally is refused rather than escaped

A rev containing a quotation mark of either kind, a backslash, or a character for which
`unicode.IsControl` reports true SHALL be refused, before any scanning, with

```
graft.toml: rev "<rev>" contains a quote, a backslash, or a control character
```

None of them can appear in a git ref, and a value that has to be escaped to be written is a
value that was not a rev. The refusal is what keeps a rev from closing the TOML string it is
written into and appending a key, a table, or a whole second source to the consumer's manifest.

The bytes returned SHALL parse through the manifest parser, yielding a manifest identical to the
original except for that source's rev.

#### Scenario: A rev that would have to be escaped is refused

- **WHEN** a rev containing `"` is moved into a manifest, and separately one containing a
  newline, a backslash, and the DEL character
- **THEN** each is refused with
  `graft.toml: rev "<rev>" contains a quote, a backslash, or a control character`
- **AND** the manifest bytes are unchanged, because none were produced

#### Scenario: A rev that would inject a second key is refused

- **WHEN** the rev `v1.0.0"` followed by a newline and `install = []` is moved into a manifest
- **THEN** it is refused by the character rule before any scanning happens
- **AND** the manifest is unchanged, so no source and no key can be appended through a rev

#### Scenario: The result round-trips through the parser

- **WHEN** a manifest with two sources and a per-source `kinds` override has one source's rev
  moved
- **THEN** parsing the result succeeds and yields both sources, both `install` lists, and the
  `kinds` override unchanged
- **AND** the moved source's `rev` is the new value
