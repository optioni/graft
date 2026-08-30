# list-execution Specification

## Purpose
TBD - created by archiving change list-command. Update Purpose after archive.

## Requirements

### Requirement: `graft list` reports what `graft.lock` records, and reads nothing else

`graft list` SHALL read `graft.lock` from the repository graft runs in, and SHALL derive
everything it prints from that file alone. It SHALL NOT read `graft.toml`, SHALL NOT read the
working tree, SHALL NOT read or write the fetch cache, and SHALL NOT reach the network.

The lock is the record of what was *installed*; `graft.toml` is the record of what was
*requested*. A manifest whose `rev` has moved ahead of the lock SHALL NOT be reported here and
SHALL NOT make `list` fail. That disagreement is a failure mode of `graft sync` and the thing
`graft update` repairs, and an informational read that failed because of it would be a fourth
place keeping one rule.

For the same reason `list` SHALL NOT compare the lock against the tree. It reports a file as
installed because the lock claims it, not because it is there. SPEC.md admits no verification
command — `git status` is the report of what a sync did — and a `list` that stat-ed its way to
an answer would be that command under another name.

#### Scenario: A lock with two sources is reported as two blocks

- **GIVEN** a repository whose `graft.lock` records source `openspec-schemas` with items
  `agent:apply-orchestrator` and `schema:tdd`, and source `skills` with item `skill:tmux`
- **WHEN** `graft list` is invoked in it
- **THEN** the standard output stream holds a block for `openspec-schemas` and a block for
  `skills`, in that order
- **AND** the exit code is `0`
- **AND** the error stream is empty

#### Scenario: A manifest whose rev moved ahead of the lock is not reported

- **GIVEN** a repository whose `graft.lock` records source `shared` at `rev = "v1.0.0"` and
  whose `graft.toml` says `rev = "v2.0.0"`
- **WHEN** `graft list` is invoked in it
- **THEN** the listing names `v1.0.0`, the rev the lock records
- **AND** it does not name `v2.0.0`
- **AND** the exit code is `0`, rather than the pin-disagreement error `graft sync` raises on
  the same repository

#### Scenario: A lock claiming a file that is not there still lists it

- **GIVEN** a synced repository from which `.claude/agents/reviewer.md` has been deleted by
  hand, leaving `graft.lock` unchanged
- **WHEN** `graft list --json` is invoked in it
- **THEN** the document still lists `.claude/agents/reviewer.md` under `agent:reviewer`
- **AND** the exit code is `0`

#### Scenario: A listing runs with no source repository reachable

- **GIVEN** a synced repository whose source repository's directory has been deleted and whose
  fetch cache has been emptied
- **WHEN** `graft list` is invoked in it
- **THEN** it prints the same listing it printed before and exits `0`
- **AND** the fetch cache root remains empty, because nothing was fetched

### Requirement: A repository with nothing installed says so, and leaves stdout clean

A repository with no `graft.lock`, and a repository whose `graft.lock` declares no source,
SHALL be reported identically: there is nothing installed. Neither SHALL be an error. A
repository that has never synced is a legitimate starting state, and `graft.lock` is a file
only graft writes.

In the plain form the error stream SHALL hold exactly `nothing installed` and one newline, and
the standard output stream SHALL be byte-empty. It is a note about the absence of content
rather than content, so it goes where notes go — and a caller piping `graft list` receives
zero bytes rather than a sentence that parses as an item.

In the `--json` form the standard output stream SHALL hold the complete document with an empty
`sources` array, and the error stream SHALL be empty. A machine-readable form that printed
nothing would make "nothing is installed" indistinguishable from "the command did not run".

#### Scenario: A repository with no lock prints a note on stderr

- **GIVEN** a directory holding a `graft.toml` and no `graft.lock`
- **WHEN** `graft list` is invoked in it
- **THEN** the error stream holds exactly `nothing installed` and one newline
- **AND** the standard output stream is byte-empty
- **AND** the exit code is `0`
- **AND** no file or directory is created in the working directory

#### Scenario: A repository with no lock still prints a JSON document

- **GIVEN** a directory holding a `graft.toml` and no `graft.lock`
- **WHEN** `graft list --json` is invoked in it
- **THEN** the standard output stream holds exactly:

```json
{
  "version": 1,
  "sources": []
}
```

  followed by one newline
- **AND** the error stream is empty and the exit code is `0`

#### Scenario: A lock declaring no source is the same as no lock

- **GIVEN** a repository whose `graft.lock` holds `version = 1` and no `[[source]]` block
- **WHEN** `graft list` is invoked, and separately `graft list --json`
- **THEN** the first writes `nothing installed` to the error stream and nothing to standard
  output
- **AND** the second writes the same empty document as a repository with no lock at all

#### Scenario: A directory with no graft files at all is not an error

- **GIVEN** an empty directory
- **WHEN** `graft list` is invoked in it
- **THEN** the error stream holds `nothing installed`
- **AND** the exit code is `0`, because `list` never reads `graft.toml` and so never raises
  `graft.toml not found`

### Requirement: The plain listing is one block per source on standard output

`graft list` without `--json` SHALL write its listing to the **standard output** stream. The
listing is the content the caller asked for, in the same way `--version` and help are; it is
not a summary of an action, and the two commands that do print summaries print them on the
error stream because they are reports about something that happened.

Each source SHALL be rendered as a block:

- a header line `<name>  <rev>  (<short sha>)`, two spaces between fields, the sha the first
  seven characters of the lock's `resolved` value in parentheses;
- one blank line;
- one line per item, indented by two spaces, holding the item's id padded to the width of the
  longest id in that block, two spaces, and the item's file count rendered as `1 file` or
  `<n> files`.

Blocks SHALL be separated by one blank line. A source with no items SHALL render as its header
alone, with no blank line after it. No line SHALL carry trailing whitespace, and the listing
SHALL end with no blank line and no summary line: a total of what is visible immediately above
it tells the reader nothing they cannot already see.

The listing SHALL carry no colour. The two things the sync report styles — a verb and a
trailing note — do not exist here, and inventing a style for a listing to demonstrate that
colour works is decoration.

Sources SHALL appear in name order and items in id order, whatever order they appear in on
disk, so that two locks describing the same installation list identically.

#### Scenario: SPEC.md's own lock renders as one block

- **GIVEN** a repository whose `graft.lock` is the example in SPEC.md — source
  `openspec-schemas`, `git = "github.com/optioni/openspec-schemas"`, `rev = "v1.2.0"`,
  `resolved = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"`, item `agent:apply-orchestrator`
  with one file and item `schema:tdd` with six
- **WHEN** `graft list` is invoked in it
- **THEN** the standard output stream holds exactly these lines, each followed by one newline:

```
openspec-schemas  v1.2.0  (fae2a30)

  agent:apply-orchestrator  1 file
  schema:tdd                6 files
```

- **AND** no line carries trailing whitespace
- **AND** the error stream is empty and the exit code is `0`

#### Scenario: Two sources are separated by one blank line

- **GIVEN** a lock recording source `a` with one item and source `b` with one item
- **WHEN** the listing is rendered
- **THEN** the lines are `a`'s header, a blank line, `a`'s item line, a blank line, `b`'s
  header, a blank line, `b`'s item line
- **AND** the last line is `b`'s item line: the listing does not end with a blank line

#### Scenario: A source with no installed items is its header alone

- **GIVEN** a lock recording source `empty` at `rev = "main"` with no `[[source.item]]` block
- **WHEN** the listing is rendered
- **THEN** the block is the single line `empty  main  (<short sha>)`
- **AND** no blank line follows it inside the block

#### Scenario: A scrambled lock lists in the same order as a canonical one

- **GIVEN** two locks with identical content, one written with sources and items and files in
  sorted order and one with all three scrambled
- **WHEN** each is rendered
- **THEN** the two listings are byte-identical

#### Scenario: A resolved sha shorter than seven characters is printed whole

- **GIVEN** a listing whose source carries a resolved value of `abc`
- **WHEN** the header is rendered
- **THEN** it reads `<name>  <rev>  (abc)` rather than being truncated or padded

### Requirement: `graft list --json` prints one document whose shape is a contract

`graft list --json` SHALL write a single JSON document to the **standard output** stream and
nothing to the error stream. The document is a published interface: its field names, its field
order, its ordering of every collection, how it renders an empty collection, its indentation,
and its trailing newline are all part of the contract, and changing any of them is a breaking
change to be argued for rather than an incidental edit.

The document SHALL be an object with exactly two members, in this order:

- `version` — an integer, the version of **this document's** format, currently `1`. It is not
  `graft.lock`'s version: the lock's format and the document's are free to move independently,
  and one number meaning two things would tie them together.
- `sources` — an array, in source-name order.

Each source SHALL be an object with exactly five members, in this order: `name`, `git`, `rev`,
`resolved`, `items`. `resolved` SHALL be the **full** forty-character sha — the seven-character
form exists to be read by a person, and a program that wants to compare shas needs all of it.
`items` SHALL be an array in item-id order.

Each item SHALL be an object with exactly four members, in this order: `id`, `kind`, `name`,
`files`. `kind` and `name` are the two halves of the id. They are derivable from it, and they
are carried anyway because `kind:name` is graft's grammar and not the consumer's: a caller
filtering by kind should not have to re-implement graft's own parsing to do it. `files` SHALL
be an array of repository-relative paths in path order.

An empty collection SHALL be rendered as `[]` and never as `null`, at every level — a
repository with nothing installed, a source with no items, and an item with no files alike.

The document SHALL be indented with two spaces per level and SHALL end with exactly one
newline. It SHALL NOT escape `<`, `>`, or `&`: a git URL is a value graft was given, and
returning it with three characters replaced by escapes would mean the document does not
round-trip what the lock holds.

Two invocations against the same lock SHALL produce byte-identical documents, and two locks
with the same content written in different orders SHALL produce byte-identical documents.

#### Scenario: SPEC.md's own lock renders as this exact document

- **GIVEN** the repository whose `graft.lock` is the example in SPEC.md
- **WHEN** `graft list --json` is invoked in it
- **THEN** the standard output stream holds exactly:

```json
{
  "version": 1,
  "sources": [
    {
      "name": "openspec-schemas",
      "git": "github.com/optioni/openspec-schemas",
      "rev": "v1.2.0",
      "resolved": "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
      "items": [
        {
          "id": "agent:apply-orchestrator",
          "kind": "agent",
          "name": "apply-orchestrator",
          "files": [
            ".claude/agents/apply-orchestrator.md"
          ]
        },
        {
          "id": "schema:tdd",
          "kind": "schema",
          "name": "tdd",
          "files": [
            "openspec/schemas/tdd/schema.yaml",
            "openspec/schemas/tdd/templates/design.md",
            "openspec/schemas/tdd/templates/planning-review.md",
            "openspec/schemas/tdd/templates/proposal.md",
            "openspec/schemas/tdd/templates/spec.md",
            "openspec/schemas/tdd/templates/tasks.md"
          ]
        }
      ]
    }
  ]
}
```

  followed by exactly one newline
- **AND** the error stream is empty and the exit code is `0`

#### Scenario: The document is valid JSON that round-trips

- **WHEN** the document above is decoded by a JSON parser and re-encoded
- **THEN** it decodes without error
- **AND** every value it holds equals the corresponding value in `graft.lock`, the full sha
  included

#### Scenario: A source with no items renders `[]` rather than null

- **GIVEN** a lock recording source `empty` with no `[[source.item]]` block
- **WHEN** `graft list --json` is invoked
- **THEN** that source's `items` member is `[]`
- **AND** the document contains no `null` anywhere

#### Scenario: An item with no files renders `[]` rather than null

- **GIVEN** a lock recording item `agent:none` with `files = []`
- **WHEN** `graft list --json` is invoked
- **THEN** that item's `files` member is `[]`

#### Scenario: A scrambled lock produces the same document as a canonical one

- **GIVEN** two locks with identical content, one written with sources, items, and files in
  sorted order and one with all three scrambled
- **WHEN** each is rendered as JSON
- **THEN** the two documents are byte-identical
- **AND** running the same lock twice produces byte-identical documents

#### Scenario: A git URL containing an ampersand is not escaped

- **GIVEN** a lock whose source records `git = "https://example.test/r?a=1&b=2"`
- **WHEN** `graft list --json` is invoked
- **THEN** the `git` member holds `https://example.test/r?a=1&b=2` literally
- **AND** the document contains no `\u0026`, which is what Go's JSON encoder emits by
  default

#### Scenario: The kind and name halves match the id

- **GIVEN** a lock recording item `schema:tdd`
- **WHEN** `graft list --json` is invoked
- **THEN** that item's `id` is `schema:tdd`, its `kind` is `schema`, and its `name` is `tdd`

### Requirement: `graft list` changes nothing

`graft list` SHALL create, modify, and delete nothing: not in the working tree, not in
`graft.toml`, not in `graft.lock`, and not in the fetch cache. It SHALL create no directory,
including the cache root. It SHALL resolve no rev and fetch no source, so it SHALL succeed with
no network and with no `git` on `PATH`.

This is not a promise about `internal/apply` being bypassed carefully — `list` reaches no code
that writes at all.

#### Scenario: A listing leaves the working tree byte-identical

- **GIVEN** a synced repository
- **WHEN** `graft list` is invoked, and separately `graft list --json`
- **THEN** every file in the tree is byte-identical to what it was before, `graft.toml` and
  `graft.lock` included
- **AND** the set of paths in the tree is unchanged

#### Scenario: A listing creates no cache directory

- **GIVEN** a synced repository and a cache root that does not exist
- **WHEN** `graft list` is invoked
- **THEN** the cache root still does not exist
- **AND** the exit code is `0`

### Requirement: A `graft.lock` that cannot be read is refused with the lock's own message

A `graft.lock` that exists but cannot be parsed or validated SHALL fail the run, exit `1`, and
report the message `internal/lock` already words for that condition, unaltered and with no
second layer of context. `list` adds no failure mode of its own to a file it only reads.

The standard output stream SHALL be byte-empty on such a run, in both forms. A `--json`
invocation that failed SHALL NOT emit a partial document: the document is written only after
the lock has parsed.

#### Scenario: A lock from a newer graft is refused

- **GIVEN** a repository whose `graft.lock` holds `version = 2`
- **WHEN** `graft list` is invoked, and separately `graft list --json`
- **THEN** both write
  `graft: graft.lock: version 2 is not supported by this graft; upgrade graft` to the error
  stream
- **AND** both leave the standard output stream byte-empty
- **AND** both exit `1`

#### Scenario: A malformed lock is refused before anything is printed

- **GIVEN** a repository whose `graft.lock` holds `version = 1` and a `[[source]]` block with
  no `git` key
- **WHEN** `graft list --json` is invoked
- **THEN** the error stream holds `graft: graft.lock: source "shared": git is required`
- **AND** the standard output stream is byte-empty — no opening brace, no partial document
- **AND** the exit code is `1`

#### Scenario: A lock that is a directory is refused

- **GIVEN** a repository where `graft.lock` is a directory
- **WHEN** `graft list` is invoked
- **THEN** the error stream holds a message prefixed `graft: graft.lock: `
- **AND** the standard output stream is byte-empty and the exit code is `1`

### Requirement: `graft list` takes no positional arguments

`graft list` SHALL accept the `--json` flag and nothing else. A positional argument SHALL be
refused as `unknown argument "<argument>"`, naming the first one, with the hint line
`run "graft --help" for usage` and exit code `1` — the wording `graft sync` already uses, so
that the refusal is graft's contract rather than a detail of how cobra reports it.

There is no `--source` filter and no selector argument. A caller who wants one source pipes
`--json` into a tool that filters JSON.

#### Scenario: A positional argument is a usage error

- **WHEN** `graft list shared` is invoked
- **THEN** the error stream holds `graft: unknown argument "shared"` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is byte-empty and the exit code is `1`

#### Scenario: Only the first positional argument is named

- **WHEN** `graft list shared other` is invoked
- **THEN** the error stream names `shared` and does not name `other`
- **AND** the exit code is `1`

#### Scenario: An unknown flag is a usage error

- **WHEN** `graft list --format=yaml` is invoked
- **THEN** the error stream holds `graft: unknown flag: --format` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is byte-empty and the exit code is `1`
