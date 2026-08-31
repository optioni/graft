## MODIFIED Requirements

### Requirement: Each changed item gets one line naming the verb, the item, and its file count

Under a source's header, each item with something to report SHALL be one line, indented two
spaces, holding the verb, the item id, the file count, and — for a removed item, or an item
that replaced content graft did not own — a note. The words `added`, `adopted`, `updated`, and
`removed` SHALL be used rather than symbols.

The verb SHALL be:

- `added` when the lock had no entry for the item under that source,
- `adopted` when the lock had no entry **and** at least one of the item's files replaced
  content at a destination the previous lock did not claim,
- `removed` when the lock had one and the new lock does not,
- `updated` when both have one and either the source's resolved sha moved or the item's file
  list changed.

`adopted` exists because `added` is otherwise a false statement: nothing was added, something
was replaced. An item whose verb is `updated` SHALL keep that verb even when it replaced
unclaimed content — the verb is corrected only where it would say the opposite of what
happened — and SHALL carry the note below like any other.

An item present in both locks whose files and source sha are all unchanged SHALL produce no
line at all.

The file count SHALL be the number of files the item holds in the new lock, or in the old lock
for a removed item, rendered as `1 file` or `<n> files`.

A removed item SHALL carry one of three notes: `no longer provided` when the source's catalog
at the resolved sha no longer offers it, `no longer installed` when the catalog still offers
it but no selector matches it, and `source removed` when `graft.toml` no longer declares the
source at all.

An item that replaced content at a destination the previous lock did not claim SHALL carry the
note `replaced existing content`, whether its verb is `adopted` or `updated`. Every other added
or updated item SHALL carry no note.

A destination the previous lock **did** claim is graft's own file being rewritten, which is
what a sync does and not something to report; and a destination whose existing bytes are
identical to what is written replaced nothing. Neither SHALL count.

Replacement is a fact about the filesystem, so a run that writes nothing cannot observe it:
under `--dry-run` no item SHALL be reported `adopted` and none SHALL carry this note. A dry run
says what a plan would do, and this is not in the plan.

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

#### Scenario: An item that replaced a hand-written file is adopted

- **WHEN** `.claude/agents/reviewer.md` exists with content of its own, no lock claims it, and
  a sync writes that path for `agent:reviewer`
- **THEN** the line reads `  adopted  agent:reviewer  1 file  replaced existing content`
- **AND** the file holds the source's bytes afterwards, because adoption is reported rather
  than refused

#### Scenario: An updated item that replaced a hand-written file keeps its verb

- **WHEN** `schema:tdd` is already in the lock, gains a file at a path that already existed
  unclaimed, and the sync writes it
- **THEN** the line's verb is `updated` and its note is `replaced existing content`

#### Scenario: A destination the lock already claimed is not a replacement

- **WHEN** a sync rewrites every file of an item the lock already claims, with new bytes
- **THEN** the verb is `updated` and there is no note: those are graft's own files

#### Scenario: Identical bytes replace nothing

- **WHEN** a destination exists, no lock claims it, and its bytes are exactly what the sync is
  about to write
- **THEN** the item is reported `added` with no note

#### Scenario: A dry run reports no adoption

- **WHEN** the same repository as the first scenario is synced with `--dry-run`
- **THEN** the line reads `  added  agent:reviewer  1 file` and carries no note
- **AND** the file on disk is untouched

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

When any planned write replaced content at a destination the previous lock did not claim, the
written half SHALL be followed by that count in parentheses:

```
7 files written (1 replaced existing content), 0 removed - review with `git diff`
```

The parenthetical SHALL be absent when the count is zero, so a summary that has nothing to say
about replacement says nothing rather than `(0 replaced existing content)`. The count is of
files, not of items: one item replacing four files reads `(4 replaced existing content)`.

Under `--dry-run` the same line SHALL instead read `<n> files to write, <m> to remove -
nothing written`, so a reader can never mistake a plan for a result.

#### Scenario: The summary names how many files replaced something

- **WHEN** a sync writes seven files, one of which replaced a hand-written file no lock claimed
- **THEN** the summary reads
  ``7 files written (1 replaced existing content), 0 removed - review with `git diff` ``

#### Scenario: A sync that replaced nothing carries no parenthetical

- **WHEN** a sync writes six files and every destination was either absent or already claimed
  by the lock
- **THEN** the summary reads ``6 files written, 1 removed - review with `git diff` `` — byte
  identical to what graft printed before this was reported at all

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
