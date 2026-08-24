# Sync Report Specification

## Purpose

What a sync prints: a block per source whose pin or items moved, a line per item naming what
happened to it, and one summary line. It goes to the error stream, because a summary is not
machine-readable output and SPEC.md keeps a pipe clean of text a human was meant to read.

The report is derived from the lock that was on disk, the lock the plan produced, and the
plan's counts — never from the tree. Every planned file is written on every sync, so "updated"
cannot mean "the bytes changed", and no comparison available here could make it mean that.

A sync with nothing to do prints `up to date` and nothing else. Output that appears when
nothing happened trains the reader to stop reading it.

## Requirements

### Requirement: A sync with nothing to do prints `up to date` and nothing else

A sync SHALL be reported as having nothing to do when the lock it would write is
byte-identical to the lock already on disk **and** its prune set is empty. In that case the
error stream SHALL hold exactly `up to date` and one newline, and nothing else — no source
header, no item line, and no summary.

Byte equality of the serialized lock is the test rather than a comparison of installed file
contents, because a plan carries no notion of "unchanged": every planned file is written on
every sync. What a reader wants to know is whether this run changed anything they would see
in a diff, and the lock plus the prune set is exactly that. Output that appears when nothing
happened trains the reader to stop reading it.

There is one case where the predicate and the tree disagree, and it is stated rather than
papered over: a user who deletes installed files by hand and re-syncs gets them back and is
told `up to date`, because the lock did not move and nothing was pruned. Narrowing the
predicate to cover it would mean checking every destination's presence, which is the
tree-scanning this design does not do — and the restored files still appear where SPEC.md says
a sync's effect appears, in `git status`.

A dry run with nothing to do SHALL print `up to date` as well. `--dry-run` changes what the
summary line says, not what "nothing to do" means.

#### Scenario: A repeated sync reports nothing

- **WHEN** `graft sync` runs twice against an unchanged manifest, lock, and source
- **THEN** the second run's error stream holds exactly `up to date` and one newline
- **AND** the standard output stream is byte-empty and the exit code is `0`

#### Scenario: A sync that only rewrites identical files is still nothing to do

- **WHEN** every installed file has been deleted from the working tree by hand and
  `graft sync` reinstalls all of them from the same pin
- **THEN** the run reports `up to date`, because the lock it writes is byte-identical and it
  prunes nothing
- **AND** every file is back on disk, so the report describes the lock rather than the tree

#### Scenario: A first sync is never nothing to do

- **WHEN** `graft sync` runs against a repository with no `graft.lock` and a manifest that
  installs at least one item
- **THEN** the report is not `up to date`, because the lock it writes differs from the empty
  one

#### Scenario: A dry run with nothing to do reports nothing

- **WHEN** `graft sync --dry-run` runs against a repository already in sync
- **THEN** the error stream holds exactly `up to date` and one newline
- **AND** the dry-run summary line does not appear, because there is no summary when there is
  nothing to summarize

### Requirement: A source whose pin or items moved gets a header naming what moved

For each source with something to report, the error stream SHALL carry a header line holding
the source name, then two spaces, then the rev, then two spaces, then the short sha in
parentheses. When the rev or the sha moved, both halves SHALL be rendered as
`<old> -> <new>`:

```
openspec-schemas  v1.2.0 -> v1.3.0  (fae2a30 -> 9c1e77a)
```

A source the lock had no entry for, and one whose rev and sha are unchanged, SHALL render each
half once — `openspec-schemas  v1.2.0  (fae2a30)`. A short sha SHALL be the first seven
characters of the resolved sha.

A source present in `graft.lock` but no longer in `graft.toml` SHALL be reported with the rev
and sha the lock recorded, and every item it held reported as removed.

One blank line SHALL follow the header, and one blank line SHALL separate consecutive source
blocks. Sources SHALL be reported in name order over the **union** of the sources in the old
lock and in the new one — a source dropped from `graft.toml` appears in the old lock only, and
is still reported.

#### Scenario: A version bump shows both revs and both shas

- **WHEN** source `openspec-schemas` moves from `v1.2.0` at `fae2a30…` to `v1.3.0` at
  `9c1e77a…`
- **THEN** the header reads `openspec-schemas  v1.2.0 -> v1.3.0  (fae2a30 -> 9c1e77a)`
- **AND** it is followed by a blank line and then the item lines

#### Scenario: A newly added source shows one rev and one sha

- **WHEN** source `extra` is installed for the first time at `v2.0.0` resolving to
  `0123456789abcdef0123456789abcdef01234567`
- **THEN** the header reads `extra  v2.0.0  (0123456)`

#### Scenario: A branch pin whose sha moved shows both shas and one rev

- **WHEN** a source pinned at `rev = "main"` is moved by `graft update` from one sha to
  another
- **THEN** the header reads `shared  main  (aaaaaaa -> bbbbbbb)`, the rev rendered once
  because it did not move

#### Scenario: Two sources are separated by a blank line

- **WHEN** two sources both have items to report
- **THEN** the second source's header is preceded by exactly one blank line
- **AND** the sources appear in name order

### Requirement: Each changed item gets one line naming the verb, the item, and its file count

Under a source's header, each item with something to report SHALL be one line, indented two
spaces, holding the verb, the item id, the file count, and — for a removed item — a note
saying why it went. The words `added`, `updated`, and `removed` SHALL be used rather than
symbols.

The verb SHALL be:

- `added` when the lock had no entry for the item under that source,
- `removed` when the lock had one and the new lock does not,
- `updated` when both have one and either the source's resolved sha moved or the item's file
  list changed.

An item present in both locks whose files and source sha are all unchanged SHALL produce no
line at all.

The file count SHALL be the number of files the item holds in the new lock, or in the old lock
for a removed item, rendered as `1 file` or `<n> files`.

A removed item SHALL carry one of three notes: `no longer provided` when the source's catalog
at the resolved sha no longer offers it, `no longer installed` when the catalog still offers
it but no selector matches it, and `source removed` when `graft.toml` no longer declares the
source at all. Added and updated items SHALL carry no note.

The verb and the item id SHALL each be padded to the widest of their column **in that source's
block** and followed by two spaces. The file count SHALL be padded to the widest count in that
block and followed by two spaces **only when a note follows it**; otherwise the line ends after
the count. No report line SHALL carry trailing whitespace — SPEC.md's own example has none, and
trailing spaces are invisible in a diff and rot without anyone noticing.

Padding SHALL be computed on the unstyled text, so enabling colour never moves a column.

#### Scenario: An updated and a removed item align in one block

- **WHEN** a source reports `schema:tdd` updated with six files and
  `agent:phase-orchestrator` removed with one file that the catalog no longer provides
- **THEN** the two lines read exactly

  ```
    updated  schema:tdd                6 files
    removed  agent:phase-orchestrator  1 file   no longer provided
  ```

- **AND** the id column is padded to the widest id in the block, the count column to the widest
  count, and the first line ends immediately after `6 files` with no trailing whitespace

#### Scenario: A newly installed item is added

- **WHEN** the lock had no entry for `agent:reviewer` and the new lock records it with one
  file
- **THEN** the line reads `  added  agent:reviewer  1 file`
- **AND** it carries no note

#### Scenario: An item dropped from install says so

- **WHEN** `agent:reviewer` is dropped from the manifest's `install` while the source's
  catalog still provides it
- **THEN** the line reads `  removed  agent:reviewer  1 file  no longer installed`

#### Scenario: An item of a source dropped from the manifest says so

- **WHEN** source `retired` is deleted from `graft.toml` and the lock recorded two items for
  it
- **THEN** both lines carry the note `source removed`
- **AND** no catalog was read for that source, which is why the note cannot be one of the
  other two

#### Scenario: An unchanged item under a moved pin is still updated

- **WHEN** a source moves from `v1.2.0` to `v1.3.0` and one of its items produces exactly the
  same file list at the new sha
- **THEN** that item is reported `updated`, because the bytes behind an unchanged path may
  well have changed and graft has no content comparison to say otherwise

#### Scenario: An unchanged item under an unchanged pin produces no line

- **WHEN** a source's sha is unchanged, one item's file list is unchanged, and another item
  gained a file
- **THEN** only the second item appears in the block
- **AND** the block is still emitted, because the source has something to report

### Requirement: The summary names how many files were written and removed

After the source blocks, separated from them by one blank line, the error stream SHALL carry
one summary line:

```
6 files written, 1 removed - review with `git diff`
```

The written count SHALL be the number of writes in the plan — every planned file, because
every one of them is written — and the removed count the size of the prune set. Each count
SHALL be rendered as `1 file` or `<n> files` for the written half and as a bare number for the
removed half.

Under `--dry-run` the same line SHALL instead read `<n> files to write, <m> to remove -
nothing written`, so a reader can never mistake a plan for a result.

#### Scenario: The summary counts every planned write

- **WHEN** a sync writes six files across two items and prunes one
- **THEN** the summary reads ``6 files written, 1 removed - review with `git diff` ``
- **AND** the count is the plan's write count, not the number of files whose content changed

#### Scenario: A sync that only removes still reports zero written

- **WHEN** the only source is dropped from `graft.toml` and three files are pruned
- **THEN** the summary reads ``0 files written, 3 removed - review with `git diff` ``

#### Scenario: A single file is reported in the singular

- **WHEN** one file is written and none removed
- **THEN** the summary reads ``1 file written, 0 removed - review with `git diff` ``

#### Scenario: A dry run says nothing was written

- **WHEN** `graft sync --dry-run` would write six files and remove one
- **THEN** the summary reads `6 files to write, 1 to remove - nothing written`
- **AND** the source blocks above it are identical to what the same sync would print without
  the flag

### Requirement: The report goes to the error stream and carries colour only when the run does

Every line of the report — headers, item lines, blank lines, the summary, and `up to date` —
SHALL be written to the **error** stream. Nothing SHALL be written to the standard output
stream by a sync.

When colour is enabled the item verb SHALL be rendered bold and a removed item's note dimmed.
No other part of the report SHALL be styled. When colour is disabled every line SHALL be
byte-identical to the same line with no escape sequence anywhere, including its padding.

The colour decision SHALL be the one `internal/ui` already takes for the whole run from
standard output and `NO_COLOR`, not a second decision taken here.

#### Scenario: The report never reaches standard output

- **WHEN** a sync that adds, updates, and removes items completes
- **THEN** the standard output stream is byte-empty
- **AND** the error stream holds the whole report

#### Scenario: With colour off the report is plain text

- **WHEN** the report is rendered with colour disabled
- **THEN** no line contains an escape byte, and the item lines are exactly the padded plain
  text
- **AND** this is the rendering asserted by the specification's other scenarios

#### Scenario: With colour on only the verb and the note are styled

- **WHEN** the report is rendered with colour enabled
- **THEN** the verb is wrapped in the bold sequence and a removed item's note in the dim
  sequence
- **AND** stripping every escape sequence yields exactly the plain rendering, so the columns
  did not move
