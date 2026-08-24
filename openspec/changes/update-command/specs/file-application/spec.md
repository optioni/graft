## MODIFIED Requirements

### Requirement: Applying a plan is the only path that writes to the working tree

`internal/apply` SHALL be the only package in graft permitted to create, modify, or delete
anything in the repository graft is running in. It SHALL expose one entry point taking the
repository root, each source's fetched tree by source name, a `plan.Plan`, and — optionally —
the bytes of a `graft.toml` to write, and SHALL perform exactly the operations that plan
describes, in this order:

1. Check everything checkable, touching nothing.
2. Write the planned files.
3. Delete the prune set.
4. Remove the directories the prune set left empty.
5. Write `graft.toml`, but only when the caller handed over bytes for it.
6. Write `graft.lock`.

Steps 2 through 6 are SPEC.md's resolution step 8. Step 1 is what makes the refusals in this
specification leave the tree byte-identical rather than partly applied — every condition they
test is decidable before the first byte is written, and discovering one halfway through would
guarantee the partial apply the design otherwise works to avoid.

`internal/apply` SHALL derive nothing for itself. It SHALL NOT scan a destination directory,
SHALL NOT compare file contents, SHALL NOT decide a file needs no writing, and SHALL NOT
expand, re-order, or extend the prune set. Every path it touches SHALL come from the plan it
was given, or be one of graft's own two files at the repository root — `graft.lock`, always,
and `graft.toml` when and only when the caller supplied its bytes. Those two are the named
exception rather than a hole in the rule: their paths are fixed and are not derived from
anything, their bytes come from the caller rather than from a source, and both are refused
outright when a *plan* names them. That is what makes `internal/plan`'s purity worth having:
the checks that ran before anything was written are the checks that govern what gets written.

Applying a plan holding no writes, no prunes, and a lock with no sources SHALL succeed,
writing only `graft.lock` when no manifest bytes were given, and `graft.toml` and `graft.lock`
when they were.

#### Scenario: A plan's operations happen in the documented order

- **WHEN** a plan is applied whose prune set names `openspec/schemas/tdd/templates/old.md` and
  whose writes include `openspec/schemas/tdd/templates/new.md`, that directory holding nothing
  else, while a second prune set empties `docs/` entirely
- **THEN** the apply succeeds, `templates/` still exists holding `new.md` only, `old.md` is
  gone, `docs/` is gone, and `graft.lock` records the new state
- **AND** each of those pins one adjacency the order requires: `docs/` disappearing puts the
  directory removal **after** the prune, since a removal computed first would have found
  `docs/` still occupied; `templates/` surviving puts it **after** the writes; and a failed
  prune leaving the previous lock puts the lock **after** everything
- **AND** ordering is asserted through effects the real filesystem produces, not through an
  observation seam: `internal/apply` exposes one function and no injection point. Two orders
  that no effect can tell apart are not distinguished, because a test that claimed to would
  be asserting the implementation rather than the behavior

#### Scenario: An empty plan writes only the lock

- **WHEN** a plan holding no writes, an empty prune set, and a lock with zero sources is
  applied to a repository holding one repo-owned file, with no manifest bytes given
- **THEN** `graft.lock` is written holding the header and `version = 1`
- **AND** the repo-owned file is present and unchanged, no directory was created, and
  `graft.toml` was neither read nor written

#### Scenario: An empty plan with manifest bytes writes graft's two files and nothing else

- **WHEN** the same empty plan is applied with manifest bytes given
- **THEN** `graft.toml` holds exactly those bytes and `graft.lock` holds the header and
  `version = 1`
- **AND** the repo-owned file is present and unchanged and no directory was created

#### Scenario: Nothing outside the plan is touched

- **WHEN** a plan is applied whose single write lands at `.claude/agents/apply.md`, in a
  repository whose `.claude/agents/` already holds `local-reviewer.md` and whose `docs/`
  holds `notes.md`
- **THEN** `.claude/agents/apply.md` is written and `graft.lock` is written
- **AND** `local-reviewer.md` and `docs/notes.md` are byte-identical to what they were, and
  no directory was removed

## ADDED Requirements

### Requirement: graft.toml is rewritten only when the apply is given manifest bytes, and never through an unlinked file

`internal/apply` SHALL accept, as an option beside the plan, the bytes of a `graft.toml` to
write. When no such bytes are given — every `graft sync`, and every `graft update` without
`--to` — it SHALL neither read nor write `graft.toml`, and the file SHALL be byte-identical
after the apply.

When bytes are given, `internal/apply` SHALL write exactly those bytes to `graft.toml` at the
repository root, after every planned write, every deletion, and every directory removal has
succeeded, and immediately before `graft.lock`. If any of those fails, `graft.toml` SHALL NOT
be written at all, so a run that failed partway leaves the consumer's request describing the
pin the lock still records.

The write SHALL go through a temporary file at the repository root followed by a rename over
the destination, rather than through the remove-then-create the planned writes use. At no
point SHALL `graft.toml` be absent or partially written: a reader SHALL see either the old
bytes or the new ones. The remove-then-create exists so that a *source* cannot preserve an
executable bit on a file it replaces, and no source's bytes are involved here — while
`graft.toml` is the one file in the repository graft cannot regenerate, so the window in which
it does not exist is a window worth closing. A temporary file left behind by a failed rename
SHALL be removed.

The destination SHALL be checked in the same pre-flight pass as `graft.lock`'s, and only on a
run that was given manifest bytes: a `graft.toml` that exists and is not a regular file SHALL
fail the apply **before the first byte of the plan is written**, with the wording that check
already produces. Neither of graft's own two files is in the plan, so both are named
explicitly.

The staging path SHALL be checked in that same pre-flight pass, and a leftover that exists and
is not a regular file SHALL fail the apply before the first byte of the plan is written, naming
the staging path rather than the destination — that is the path the user has to go and look at.
It SHALL NOT be removed: deleting an empty directory or a symlink found there would be graft
removing a path no lock claims, which is the one thing it may never do. The removal on failure
SHALL therefore touch the staging path only when it is a regular file.

This is graft writing its own file, not a source placing one. The refusal in *graft never
writes inside `.git` and never writes over its own two files* is unchanged and still applies to
every path in the plan's writes and prune set: a plan naming `graft.toml` as a destination
SHALL be refused even on a run that is rewriting `graft.toml`. The two are told apart by where
the bytes came from — the plan, or the caller — never by the path string.

#### Scenario: Manifest bytes are written just before the lock

- **WHEN** a plan writing two files is applied with manifest bytes given
- **THEN** both files exist, `graft.toml` holds exactly the given bytes, and `graft.lock` holds
  the plan's lock
- **AND** re-reading `graft.toml` through the manifest parser succeeds, because a manifest this
  package writes that the next run refuses to read is the worst failure available here

#### Scenario: An apply with no manifest bytes leaves graft.toml alone

- **WHEN** a plan is applied with no manifest bytes given and `graft.toml` exists holding a
  known sequence of bytes
- **THEN** `graft.toml` holds byte-for-byte the same content afterwards
- **AND** `graft.lock` is written as usual

#### Scenario: A plan naming graft.toml as a destination is still refused

- **WHEN** a plan whose writes include `graft.toml` is applied with manifest bytes also given
- **THEN** the apply fails with `cannot write "graft.toml": graft never writes over "graft.toml"`
- **AND** `graft.toml` is byte-identical and no planned file was written, because the refusal is
  in the pre-flight pass

#### Scenario: A graft.toml that is not a regular file fails before the first write

- **WHEN** `graft.toml` at the repository root is a directory and an apply is given manifest
  bytes
- **THEN** the apply fails with
  `cannot write "graft.toml": it exists and is not a regular file`
- **AND** no planned file was written, nothing was deleted, and `graft.lock` is unchanged

#### Scenario: A leftover staging file that is not a regular file is refused, not removed

- **WHEN** a path beside `graft.toml` where the new manifest would be staged exists as a
  directory — empty in one case, holding a file in the other — and a plan with one write is
  applied with manifest bytes given
- **THEN** the apply fails with
  `cannot write ".graft.toml.tmp": it exists and is not a regular file`
- **AND** that path still exists, `graft.toml` holds exactly the bytes it held before, and the
  plan's write did not happen, because the refusal is in the pre-flight pass

#### Scenario: A failed apply leaves graft.toml unmoved

- **WHEN** a plan's write fails because its destination directory was made read-only after the
  pre-flight pass checked it — the residual failure that pass cannot remove — and manifest bytes
  were given
- **THEN** the apply fails, `graft.toml` holds exactly the bytes it held before, and `graft.lock`
  is unchanged
- **AND** the manifest and the lock therefore still agree, which is the state a re-run can
  recover from

#### Scenario: No temporary file survives a successful or a failed apply

- **WHEN** an apply given manifest bytes succeeds, and separately when one fails after the
  temporary file has been created
- **THEN** the repository holds no leftover temporary file beside `graft.toml` in either case
- **AND** the tree listing after the failed run is exactly the tree listing before it
