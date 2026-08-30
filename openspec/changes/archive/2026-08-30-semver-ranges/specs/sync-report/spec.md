## MODIFIED Requirements

### Requirement: A source whose pin or items moved gets a header naming what moved

For each source with something to report, the error stream SHALL carry a header line holding
the source name, then two spaces, then the rev, then — when the rev is a range — two spaces
and the matched tag, then two spaces, then the short sha in parentheses. When the rev, the
matched tag, or the sha moved, that half SHALL be rendered as `<old> -> <new>`:

```
openspec-schemas  v1.2.0 -> v1.3.0  (fae2a30 -> 9c1e77a)
```

A range renders its own request unchanged — a range does not move unless the consumer edits
it — and shows the movement in the matched column instead:

```
openspec-schemas  ^1.2.0  v1.2.0 -> v1.3.0  (fae2a30 -> 9c1e77a)
```

The matched column SHALL appear only for a source whose rev is a range, so a report
containing no range is byte-identical to what graft prints today.

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

#### Scenario: A range whose matched tag moved shows the range once and the tag twice

- **WHEN** source `openspec-schemas` pinned at `rev = "^1.2.0"` is moved by `graft update`
  from matched `v1.2.0` at `fae2a30…` to matched `v1.3.0` at `9c1e77a…`
- **THEN** the header reads
  `openspec-schemas  ^1.2.0  v1.2.0 -> v1.3.0  (fae2a30 -> 9c1e77a)`
- **AND** the range itself is rendered once, because the consumer's request did not change

#### Scenario: A newly added range source shows the range and its tag once each

- **WHEN** source `extra` pinned at `rev = "^2.0.0"` is installed for the first time,
  matching `v2.1.0` at `0123456789abcdef0123456789abcdef01234567`
- **THEN** the header reads `extra  ^2.0.0  v2.1.0  (0123456)`

#### Scenario: A range whose tag did not move renders every half once

- **WHEN** a source pinned at `rev = "^1.2.0"` re-resolves to the same matched tag and the
  same sha, and it has items to report for another reason
- **THEN** the header reads `openspec-schemas  ^1.2.0  v1.3.0  (9c1e77a)`

#### Scenario: A matched tag that moved onto the same commit still gets a header

- **WHEN** a source pinned at `rev = "^1.2.0"` moves from matched `v1.2.0` to matched `v1.3.0`
  and both tags name the **same** commit, so the resolved sha did not change
- **THEN** the source still gets a header reading
  `openspec-schemas  ^1.2.0  v1.2.0 -> v1.3.0  (fae2a30)`
- **AND** it is not skipped as having nothing to say: `graft.lock` changed, so a report that
  omitted the source would print a summary describing a diff it never explained

#### Scenario: A report with no range is unchanged

- **WHEN** every source in a run pins a ref
- **THEN** every header is byte-identical to what graft printed before this change
- **AND** no empty matched column and no extra spacing appears anywhere
