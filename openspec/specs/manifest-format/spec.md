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

### Requirement: A source's table is appended without disturbing an existing byte

`internal/manifest` SHALL provide, beside the in-place pin move, an append that returns the
bytes of a `graft.toml` with one new `[sources.<name>]` table added and every existing byte
left exactly as it was. The original bytes SHALL be a prefix of the result, with one
exception: a file whose last byte is not a newline SHALL gain one, because a table appended
onto a truncated final line would corrupt that line.

The appended table SHALL be rendered as SPEC.md's own example renders one — the header, then
`git`, `rev`, and `install` in that order, each key padded with spaces to the width of
`install` so the `=` signs align:

```toml
[sources.shared]
git     = "optioni/shared"
rev     = "v1.0.0"
install = ["agent:reviewer", "schema:tdd"]
```

`install` SHALL be one line however many selectors it holds. A file already holding content
SHALL be separated from the appended table by exactly one blank line; a file that is empty or
holds only whitespace SHALL receive the table alone, with no leading blank line.

The append SHALL create, modify, and delete nothing. It returns bytes; `internal/apply` is
the only package that puts them anywhere. On any error it SHALL return no bytes at all, so no
caller can write a half-appended file.

#### Scenario: A table is appended to a manifest holding one source

- **WHEN** a `graft.toml` holding a `[sources.other]` block with a comment above it and
  hand-aligned keys is appended to with source `shared`
- **THEN** the original bytes are a prefix of the result — the comment, the alignment, and
  every existing key byte-identical
- **AND** the result parses, declares both sources, and the added block reads
  `[sources.shared]` followed by aligned `git`, `rev`, and `install` keys

#### Scenario: An empty file gets the table alone

- **WHEN** zero bytes are appended to with source `shared`
- **THEN** the result begins with `[sources.shared]` — no leading blank line — and ends with
  exactly one newline

#### Scenario: A file with no final newline gains one

- **WHEN** a manifest whose last line has no terminator is appended to
- **THEN** the result is the original bytes, a newline, a blank line, and the new table
- **AND** the original's own last line is otherwise byte-identical

#### Scenario: Several selectors render on one line, in order

- **WHEN** source `shared` is appended with selectors `schema:tdd` and `agent:*`
- **THEN** the appended block holds `install = ["schema:tdd", "agent:*"]` on one line

### Requirement: Selectors are added to a source's install list in place

`internal/manifest` SHALL provide an amendment that returns the bytes of a `graft.toml` with
one or more selectors added to an existing source's `install` array, every other byte of the
file left exactly as it was, and the array's own formatting preserved.

The insertion point SHALL be immediately after the array's last element, and after that
element's trailing comma when it has one, so a comment sitting on its own line between the
last element and the closing bracket survives untouched. A comment trailing the last element
*on that element's own line* SHALL stay with it: the insertion SHALL begin after it, because
a comment explaining an element that ends up beside a different one is a manifest that lies,
produced by the edit whose whole purpose is not to disturb one. The comma an element without
one must gain SHALL still be written at the element's own end, ahead of that comment.

The array's shape SHALL decide the rendering:

- An array written on one line SHALL be amended on that line: `, "<selector>"` after the last
  element, and `"<selector>"` alone when the array was empty. Exactly one line of the file
  differs.
- An array written across several lines SHALL gain one new line per selector, each carrying
  the indentation of the line the last element sat on. When the array's last element already
  carried a trailing comma, every inserted line SHALL carry one too and no existing line SHALL
  be rewritten. When it did not, a comma SHALL be appended to that element — the one existing
  byte the amendment may add — and the inserted lines SHALL follow the same style, the last of
  them carrying no comma.

Selectors SHALL be added in the order given, after every element already present. A selector
the array already holds SHALL NOT be added again; an amendment adding nothing SHALL return
the original bytes unchanged rather than an equivalent re-rendering.

Like the append and the pin move, the amendment SHALL create, modify, and delete nothing, and
SHALL return no bytes at all on any error.

#### Scenario: A one-line array gains a selector on its own line

- **WHEN** `install = ["agent:reviewer"]` is amended with `schema:tdd`
- **THEN** that line reads `install = ["agent:reviewer", "schema:tdd"]`
- **AND** exactly one line of the file differs from the original

#### Scenario: A multi-line array gains a line matching its indentation

- **WHEN** an `install` array written as one element per line, indented four spaces, each
  ending with a comma, is amended with `schema:tdd`
- **THEN** one new line is inserted after the last element, indented four spaces, reading
  `"schema:tdd",`
- **AND** every other line of the file, the closing bracket's included, is byte-identical

#### Scenario: A multi-line array with no trailing comma keeps that style

- **WHEN** an `install` array written one element per line whose last element ends without a
  comma is amended with `schema:tdd`
- **THEN** exactly two lines differ: the previous last element's line, which gains a single
  `,` at its end, and the inserted line, which reads `"schema:tdd"` with no comma
- **AND** the result parses as a manifest declaring both selectors, in that order

#### Scenario: A comment trailing the last element stays with it

- **WHEN** a multi-line array's last element reads `"agent:reviewer",   # the one we use`
  and it is amended with `schema:tdd`
- **THEN** that line is byte-identical, comment included, and the new element is written on
  the line after it
- **AND** with the comma absent from that element, the comma is added at the element's own
  end — before the comment — and the result parses

#### Scenario: A selector already present is not added twice

- **WHEN** `install = ["agent:reviewer"]` is amended with `agent:reviewer`
- **THEN** the returned bytes are byte-identical to the original

#### Scenario: A comment after the last element survives

- **WHEN** a multi-line array holds a `# keep this last` comment between its final element and
  its closing bracket, and it is amended with `schema:tdd`
- **THEN** the new element is inserted before the comment, and the comment's own line is
  byte-identical

### Requirement: Every amendment shape that cannot be rewritten exactly is refused

Both edits SHALL refuse rather than guess, for the same reason the pin move does: `graft.toml`
is a file graft did not write, and a wrong guess corrupts the consumer's own request while
looking like success.

The append SHALL refuse when the manifest already declares a source of that name, with
`graft.toml: source "<name>": already declared`.

The amendment SHALL refuse, with
`graft.toml: source "<name>": cannot amend install: install is not a plain array of strings under [sources.<name>]`,
whenever it cannot locate exactly one `install` array under the source's own standard table.
That covers: no `[sources.<name>]` header at all — a source written as an inline table
qualifies — no `install` key under it, a value that is not an array, an array holding
anything but single-line quoted strings, an element carrying a backslash escape, and an array
whose closing bracket the file never reaches.

An escaped element is refused rather than decoded because the amendment has to compare each
element against the selectors it was given: a comparison against undecoded text would miss a
match, add a selector the array already holds, and produce a manifest the next parse refuses
for declaring a duplicate.

Both edits SHALL refuse any value that cannot be written literally into a TOML string —
containing a quotation mark of either kind, a backslash, or a control character — with
`graft.toml: <key> "<value>" contains a quote, a backslash, or a control character`, where
`<key>` is `git`, `rev`, or `selector`. The wording for `rev` SHALL be byte-identical to the
one the pin move already produces, because a second spelling of one refusal is a second
contract.

The name a table is appended under SHALL be a TOML bare key — matching `^[A-Za-z0-9_-]+$` —
and SHALL be refused with `graft.toml: source name "<name>" is not a bare key` otherwise. A
quoted key would parse, and would leave a file whose next in-place edit has a shape to guess
at.

#### Scenario: Appending a name already declared is refused

- **WHEN** a manifest declaring `[sources.shared]` is appended to with source `shared`
- **THEN** the failure is `graft.toml: source "shared": already declared` and no bytes are
  returned

#### Scenario: An install that is not an array is refused

- **WHEN** a source's `install` is written as a string rather than an array, and an amendment
  is attempted
- **THEN** the failure is
  `graft.toml: source "shared": cannot amend install: install is not a plain array of strings under [sources.shared]`
  and no bytes are returned

#### Scenario: A source written as an inline table is refused

- **WHEN** `graft.toml` declares its sources as `sources = { shared = { git = "...", rev = "...", install = ["agent:reviewer"] } }`, which parses, and an amendment is attempted
- **THEN** the amendment is refused with that same message and no bytes are returned

#### Scenario: An element carrying an escape is refused

- **WHEN** a source's `install` holds `"agent:\u0072eviewer"`, which parses as
  `agent:reviewer`, and an amendment is attempted
- **THEN** the amendment is refused with that same message and no bytes are returned

#### Scenario: An unterminated array is refused

- **WHEN** a source's `install` opens `[` and the file ends before any `]`
- **THEN** the amendment is refused with that same message and no bytes are returned

#### Scenario: A selector carrying a quote is refused

- **WHEN** an append or an amendment is given the selector `agent:"x"`
- **THEN** the failure is
  `graft.toml: selector "agent:\"x\"" contains a quote, a backslash, or a control character`
  and no bytes are returned

#### Scenario: A name that is not a bare key is refused

- **WHEN** an append is given the source name `my.repo`
- **THEN** the failure is `graft.toml: source name "my.repo" is not a bare key` and no bytes
  are returned
