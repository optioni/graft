## ADDED Requirements

### Requirement: Applying a plan reports which destinations it replaced

`internal/apply` SHALL report, for a run that completed, the destinations at which it replaced
existing content: a planned write whose destination existed as a regular file that the plan
did not mark as claimed by the lock, and whose bytes differed from the bytes written.

It is the only package that may answer this, because it is the only one permitted to look at
the working tree. The comparison SHALL be against bytes it is already holding — the source
file it is about to write — so it costs one read of the destination and no second pass.

All three conditions SHALL hold. A destination the lock claimed is graft's own file being
rewritten. A destination whose bytes already equal what is written replaced nothing. A
destination that does not exist is an ordinary write. None of the three is a replacement, and
reporting any of them would make the count noise a reader learns to skip.

Reporting SHALL change nothing about what is written: every planned write still happens, in
the same order, with the same containment. This adds an observation, not a decision — graft
does not refuse to overwrite, because a synced file is a derived artifact and adoption is how a
repository starts using graft at all.

A run that fails SHALL report nothing, for the same reason it writes no lock: a partial run's
account of what it replaced would describe a state that never existed.

#### Scenario: A hand-written file at a destination is reported

- **WHEN** `.claude/agents/reviewer.md` holds content of its own, the plan writes that path,
  and no lock claimed it
- **THEN** the apply succeeds, the file holds the source's bytes, and that path is reported as
  replaced

#### Scenario: A claimed destination is not reported

- **WHEN** the same path is written but the plan marked it claimed by the lock
- **THEN** the apply succeeds and reports no replacement

#### Scenario: Identical bytes are not a replacement

- **WHEN** an unclaimed destination already holds exactly the bytes about to be written
- **THEN** the apply succeeds and reports no replacement

#### Scenario: An absent destination is not a replacement

- **WHEN** a planned write's destination does not exist
- **THEN** the file is created and no replacement is reported

#### Scenario: A failed apply reports nothing

- **WHEN** a plan writes two files and the second fails because its destination is a directory
- **THEN** the apply returns that failure and reports no replacements, even though the first
  write replaced something
