## ADDED Requirements

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
