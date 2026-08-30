# Lock Format Specification

## Purpose

`graft.lock` is graft's record of what it actually installed: the sha each source
resolved to and, per item, the files graft wrote. It is generated and committed, and
every sync's effect is reviewed as its git diff — so its bytes are part of the product.
This capability covers reading it, validating it, and serializing it back to one
canonical byte-for-byte layout. It returns bytes; `internal/apply` writes them.

## Requirements

### Requirement: Lock loading and absence

`internal/lock` SHALL load `graft.lock` from a path and return a parsed lock, or an error. An
absent `graft.lock` SHALL NOT be an error: it SHALL load as a lock at the current format
version holding zero sources, because a repo that has never synced is a legitimate starting
state. Loading SHALL NOT create, modify, or delete any file.

#### Scenario: An absent lock loads as an empty lock

- **WHEN** the tree contains `graft.toml` but no `graft.lock`
- **THEN** loading returns a lock with `version` 1 and zero sources, and no error
- **AND** the tree is unchanged — in particular, no `graft.lock` is created

#### Scenario: A lock with zero sources loads

- **WHEN** `graft.lock` contains only the generated header comment and `version = 1`
- **THEN** loading succeeds and returns a lock with zero sources

#### Scenario: A populated lock loads

- **WHEN** `graft.lock` contains one `[[source]]` named `openspec-schemas` with two
  `[[source.item]]` tables, `schema:tdd` listing two files and `agent:apply-orchestrator`
  listing one
- **THEN** loading returns one source whose `rev` is `v1.2.0`, whose `resolved` is the
  recorded 40-character sha, and whose two items carry exactly those file lists
- **AND** the tree is unchanged

#### Scenario: Malformed TOML is an error

- **WHEN** `graft.lock` contains `[[source]` with no closing bracket
- **THEN** the error message begins with `graft.lock: ` and carries the decoder's own
  description of the syntax problem
- **AND** no partially populated lock is returned

### Requirement: Lock format version compatibility

`graft.lock` SHALL carry `version = 1`. A lock with no `version` SHALL be an error. A lock
whose `version` is greater than the version this binary knows SHALL fail with an error
telling the reader to upgrade graft — it SHALL never be partially read, and its contents
SHALL NOT be used to derive a prune set.

`version` SHALL NOT move for the addition of `matched`. The consequence is accepted
knowingly: an **older** graft handed a lock that carries `matched` reports
`graft.lock: source "<name>": unknown key "matched"` rather than the upgrade message this
requirement exists to produce. Bumping `version` would produce the better message for that
case and a worse outcome for every other, because a lock's version is global while `matched`
is per-source — a repository with one range would tell every older graft to upgrade before it
could read sources that have nothing to do with ranges. The degradation is bounded: it is
loud rather than silent, it names the key, and it can only be reached by a consumer who wrote
a range and then downgraded.

#### Scenario: A missing version is an error

- **WHEN** `graft.lock` declares a `[[source]]` but no top-level `version`
- **THEN** the error message is exactly `graft.lock: version is required`

#### Scenario: A newer version fails and says to upgrade

- **WHEN** `graft.lock` declares `version = 2`
- **THEN** the error message is exactly
  `graft.lock: version 2 is not supported by this graft; upgrade graft`
- **AND** no source, item, or file from that lock is returned to the caller

#### Scenario: A version below 1 is an error

- **WHEN** `graft.lock` declares `version = 0`
- **THEN** the error message is exactly `graft.lock: version 0 is not a known lock version`

#### Scenario: A range-bearing lock still declares version 1

- **WHEN** a lock holding a source with `rev = "^1.2.0"` and `matched = "v1.3.0"` is
  serialized
- **THEN** its top-level `version` is `1`
- **AND** a current graft reads it without error

#### Scenario: An older graft reports the unknown key rather than the upgrade message

- **WHEN** a lock carrying `matched` is read by a graft built before this change
- **THEN** it fails with `graft.lock: source "<name>": unknown key "matched"`
- **AND** the failure is loud and names the offending key, which is the accepted degradation
  rather than an oversight

### Requirement: A range's matched tag is recorded, and only a range's

A `[[source]]` whose `rev` is a range SHALL declare a `matched` key naming the tag that
range selected, exactly as the remote spells it. A `[[source]]` whose `rev` is a ref SHALL
NOT declare `matched` at all — not as an empty string, and not as an absent-but-present key.

`rev` records the request and `resolved` the sha it became; for a ref those two are enough,
because the request names the thing. For a range they are not: `^1.2.0` alongside a bare sha
turns a version bump into an unreadable diff, and "reviewing a version bump means reading the
lock diff" stops being true. `matched` is what restores it.

Omitting `matched` for a ref is what keeps this change byte-compatible: every lock graft has
already written contains no range, so every such lock re-serializes to the identical bytes it
holds today.

#### Scenario: A range's lock carries the matched tag

- **WHEN** a source pins `rev = "^1.2.0"` and resolution selected `v1.3.0` at
  `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5`
- **THEN** its `[[source]]` block declares `rev = "^1.2.0"`, `matched = "v1.3.0"`, and
  `resolved = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"`

#### Scenario: A ref's lock carries no matched key

- **WHEN** a source pins `rev = "v1.2.0"`
- **THEN** its `[[source]]` block contains no `matched` line whatsoever
- **AND** the serialized bytes are identical to what graft writes for that source today

#### Scenario: A lock with no ranges is byte-identical after this change

- **WHEN** a `graft.lock` written before this change, containing only ref pins, is parsed and
  re-serialized
- **THEN** the bytes are identical to the file on disk

#### Scenario: A matched key on a ref pin is refused

- **WHEN** a `[[source]]` declares `rev = "v1.2.0"` and `matched = "v1.2.0"`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": matched is only valid when rev is a range`
- **AND** no lock is returned

#### Scenario: A range pin without a matched key is refused

- **WHEN** a `[[source]]` declares `rev = "^1.2.0"` and no `matched`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": matched is required when rev is a range`

#### Scenario: An empty matched value is refused

- **WHEN** a `[[source]]` declares `rev = "^1.2.0"` and `matched = ""`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": matched is empty`

### Requirement: Lock source and item validation

Every `[[source]]` SHALL declare a non-empty `name`, `git`, `rev`, and a `resolved` that is a
40-character lowercase hex sha, and SHALL declare a non-empty `matched` when and only when
`rev` is a range. Source names SHALL be unique. Every `[[source.item]]` SHALL
declare an `id` in `kind:name` form, unique within its source. Every entry of `files` SHALL be
a non-empty relative path in cleaned form — it does not begin with `/`, contains no `..`
segment, is not `.`, and is byte-identical to its own cleaned form — and SHALL be unique
across the whole lock, not merely within its item. Unknown keys SHALL be rejected, in
every TOML spelling of a table.

#### Scenario: A missing resolved sha is an error

- **WHEN** a `[[source]]` named `openspec-schemas` declares `name`, `git`, and `rev` but no
  `resolved`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": resolved is required`

#### Scenario: A malformed resolved sha is an error

- **WHEN** a source declares `resolved = "v1.2.0"`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": resolved "v1.2.0" is not a 40-character hex sha`

#### Scenario: A duplicate source name is an error

- **WHEN** `graft.lock` declares two `[[source]]` tables both named `openspec-schemas`
- **THEN** the error message is exactly `graft.lock: duplicate source "openspec-schemas"`

#### Scenario: A malformed item id is an error

- **WHEN** a source's item declares `id = "tdd"`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": invalid item id "tdd": want kind:name`

#### Scenario: A duplicate item id within a source is an error

- **WHEN** one source declares two items both with `id = "schema:tdd"`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": duplicate item "schema:tdd"`

#### Scenario: An item with no files is valid

- **WHEN** a source's item declares `id = "schema:tdd"` and `files = []`
- **THEN** loading succeeds and that item reports zero files
- **AND** the item contributes nothing to any later prune set, because the lock claims no
  file for it

#### Scenario: An escaping file path is an error

- **WHEN** an item declares `files = ["../outside.md"]`, or `files = ["/etc/passwd"]`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": item "schema:tdd": file "<path>" is not a relative path inside the repo`,
  with `<path>` the offending entry
- **AND** no file list is returned, so a hand-edited lock cannot aim graft's deletion at a
  path outside the repo

#### Scenario: A path that is not in cleaned form is an error

- **WHEN** an item declares `files = ["."]`, or `files = ["./a.md"]`, or `files = ["a/"]`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": item "schema:tdd": file "<path>" is not a relative path inside the repo`,
  with `<path>` the offending entry
- **AND** `.` in particular is refused because it names the repo root, so a lock claiming it
  would authorise a later prune over the whole worktree

#### Scenario: One path claimed by two items is an error

- **WHEN** two `[[source.item]]` tables list the same file path, whether in one `[[source]]`
  or in two different ones
- **THEN** the error names the second item to claim it, as
  `graft.lock: source "<source>": item "<item>": duplicate file "<path>"`
- **AND** no lock is returned, because a path owned by two items would hand a later prune set
  a file another item is still responsible for

#### Scenario: An unknown key written as an inline table is an error

- **WHEN** a source is written as `source = [{name = "openspec-schemas", ..., sha = "x"}]`
  rather than as a `[[source]]` table
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": unknown key "sha"`
- **AND** the same holds for an item written as `item = [{id = "schema:tdd", hashes = []}]`,
  because the two spellings are the same document to a reader

#### Scenario: A duplicate file within an item is an error

- **WHEN** an item declares the same path twice in `files`
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": item "schema:tdd": duplicate file "openspec/schemas/tdd/schema.yaml"`

#### Scenario: An unknown key is an error

- **WHEN** a `[[source]]` declares `sha = "fae2a30..."` alongside its known fields
- **THEN** the error message is exactly
  `graft.lock: source "openspec-schemas": unknown key "sha"`

#### Scenario: The range test is the same one resolution uses

- **WHEN** a `[[source]]` declares `rev = "1.x"` and no `matched`
- **THEN** the lock parses, because `1.x` is classified as a ref by `rev-ranges`' rule and a
  ref requires no `matched`
- **AND** the lock and resolution can never disagree about whether a rev is a range, because
  both ask the same function

### Requirement: Canonical lock serialization

`internal/lock` SHALL serialize a lock to bytes in exactly one layout: the generated header
comment, `version = 1`, then one `[[source]]` block per source with keys `name`, `git`, `rev`,
the optional `matched`, and `resolved` in that order and aligned on `=`, then each
`[[source.item]]` indented two spaces with `id` and
`files` aligned on `=`. Blocks SHALL be separated by exactly one blank line, lines SHALL end
with `\n`, and the output SHALL end with exactly one trailing newline. Serialization SHALL
return bytes and SHALL NOT write to the working tree.

The alignment column SHALL remain the width of `resolved`, which is wider than `matched`, so
a lock that gains no range gains no realignment.

#### Scenario: A populated lock serializes to the documented layout

- **WHEN** a lock holding source `openspec-schemas` at `rev` `v1.2.0` and `resolved`
  `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5`, with item `agent:apply-orchestrator` holding
  `.claude/agents/apply-orchestrator.md` and item `schema:tdd` holding
  `openspec/schemas/tdd/schema.yaml` and `openspec/schemas/tdd/templates/design.md`, is
  serialized
- **THEN** the returned bytes are exactly:
  ```
  # graft.lock — generated by `graft sync`. Do not edit.
  version = 1

  [[source]]
  name     = "openspec-schemas"
  git      = "github.com/optioni/openspec-schemas"
  rev      = "v1.2.0"
  resolved = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"

    [[source.item]]
    id    = "agent:apply-orchestrator"
    files = [".claude/agents/apply-orchestrator.md"]

    [[source.item]]
    id    = "schema:tdd"
    files = [
      "openspec/schemas/tdd/schema.yaml",
      "openspec/schemas/tdd/templates/design.md",
    ]
  ```
- **AND** no file is created, modified, or deleted by serializing

#### Scenario: A range source serializes matched between rev and resolved

- **WHEN** the same lock instead pins `rev = "^1.2.0"` with `matched = "v1.3.0"`
- **THEN** its source block's first four lines are exactly:
  ```
  name     = "openspec-schemas"
  git      = "github.com/optioni/openspec-schemas"
  rev      = "^1.2.0"
  matched  = "v1.3.0"
  ```
- **AND** `resolved` follows on the next line, still aligned in the same column

#### Scenario: A files list of one is inline and a list of many is exploded

- **WHEN** an item holds exactly one file
- **THEN** its line is `  files = ["<path>"]` on one line
- **AND** an item holding two or more files renders `files = [` , one four-space-indented
  quoted path per line each ending in a comma including the last, then `  ]`
- **AND** an item holding zero files renders `  files = []`

#### Scenario: An empty lock serializes to header and version only

- **WHEN** a lock with zero sources is serialized
- **THEN** the returned bytes are exactly the header comment line, then `version = 1`, then a
  single trailing newline, with no blank line at the end

#### Scenario: A path needing escaping round-trips

- **WHEN** an item holds the file path `dir/od"d\name.md`
- **THEN** the serialized line quotes it as a TOML basic string with `"` and `\` escaped
- **AND** parsing the serialized bytes returns that identical path

### Requirement: Deterministic ordering

Serialization SHALL order sources by name, items by id, and files by path, each by byte-wise
string comparison, regardless of the order the lock was built or parsed in. Ordering SHALL
NOT depend on map iteration, locale, or platform.

#### Scenario: Input order does not change output

- **WHEN** two locks are built holding identical sources, items, and files, one with every
  collection in ascending order and one with every collection reversed
- **THEN** serializing both returns byte-identical output
- **AND** in that output sources appear name-ascending, items id-ascending, and files
  path-ascending

#### Scenario: Serializing the same lock twice is byte-identical

- **WHEN** the same lock value is serialized twice in one process
- **THEN** the two byte slices are equal under a byte-for-byte comparison, not merely
  semantically equivalent

### Requirement: Byte-stable round trip

Parsing canonical lock bytes and serializing the result SHALL return those same bytes. This
is the property that keeps a no-op sync out of the git diff.

#### Scenario: Canonical bytes survive a parse and serialize

- **WHEN** the canonical bytes from the serialization scenario above are parsed and the
  resulting lock is serialized
- **THEN** the output is byte-identical to the input

#### Scenario: Non-canonical input is normalized, then stable

- **WHEN** a hand-written `graft.lock` with the same content but sources, items, and files in
  reverse order, single-space `=` padding, and no header comment is parsed and serialized
- **THEN** the output is the canonical bytes
- **AND** parsing and serializing that output again returns the identical bytes, so the
  normalization happens exactly once

### Requirement: Manifest and lock pin agreement

A source present in both `graft.toml` and `graft.lock` SHALL carry the same `rev` in each. A
mismatch SHALL be an error naming the manifest rev, the lock rev, the source, and pointing at
`graft update`, because the manifest moved and `sync` — which never re-resolves a pin —
cannot honour it. A source present in only one of the two files SHALL NOT be an error.

#### Scenario: Agreeing pins pass

- **WHEN** `graft.toml` and `graft.lock` both record `rev = "v1.2.0"` for `openspec-schemas`
- **THEN** the check returns no error

#### Scenario: A moved manifest pin is an error

- **WHEN** `graft.toml` records `rev = "v1.3.0"` and `graft.lock` records `rev = "v1.2.0"` for
  `openspec-schemas`
- **THEN** the error message is exactly
  ``graft.toml has rev "v1.3.0" for source "openspec-schemas" but graft.lock has "v1.2.0"; run `graft update` to move the pin``

#### Scenario: A source only in the manifest is not an error

- **WHEN** `graft.toml` declares `openspec-schemas` and `graft.lock` holds zero sources
- **THEN** the check returns no error, because a source with no lock entry is resolved on the
  next sync

#### Scenario: A source only in the lock is not an error

- **WHEN** `graft.lock` records `openspec-schemas` and `graft.toml` no longer declares it
- **THEN** the check returns no error, because its files are pruned rather than re-pinned

#### Scenario: Two empty files agree

- **WHEN** `graft.toml` declares zero sources and `graft.lock` holds zero sources
- **THEN** the check returns no error
