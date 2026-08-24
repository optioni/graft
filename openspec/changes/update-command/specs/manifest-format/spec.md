## ADDED Requirements

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

### Requirement: A rev that cannot be written literally is refused rather than escaped

A rev containing a quotation mark, a backslash, or a character for which `unicode.IsControl`
reports true SHALL be refused, before any scanning, with

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
