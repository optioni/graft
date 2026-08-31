# File Application Specification

## Purpose

`internal/apply` is the only package in graft permitted to create, modify, or delete
anything in the repository graft runs in. Everything else parses, plans, or fetches; this is
where a plan stops being a value and becomes a change to someone's working tree.

It derives nothing. Every path it touches comes from the plan it was given — it never scans a
destination directory, never compares file contents, and never extends the prune set. That is
what makes `internal/plan`'s purity worth having: the checks that ran before anything was
written are the checks that govern what gets written.

Containment takes two mechanisms rather than one. An `os.Root` refuses a path that leaves its
root but **follows** a symlink that stays inside it, so every ancestor of every path is
checked with `Lstat` and must be a directory. And because a refusal discovered halfway through
would guarantee a partly applied tree, everything checkable is checked before the first byte
is written.

## Requirements

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

### Requirement: Everything checkable is checked before the first write

`internal/apply` SHALL validate the whole plan before performing any operation, and SHALL
fail without having created, modified, or deleted anything when any check fails. The checks
are, for every write: that its source has a registered fetched tree, that its source path
names a regular file in that tree, that its destination is not a reserved path, that every
existing ancestor of its destination is a directory, and that its destination, if it exists,
is a regular file. For every prune path: that it is not a reserved path, that every existing
ancestor of it is a directory, and that it, if it exists, is a regular file.

Ancestors SHALL be examined without following them. A symlink to a directory is not a
directory for this purpose: joining a path through one puts the file somewhere other than
where the plan says, and `graft.lock` would then record a path that does not name where the
bytes went — so the file an item places would not stay inside that item's own destination,
and a later prune would aim at whatever the link resolves to on that day.

A condition observed in step 1 and changed by another process before step 2 SHALL be allowed
to fail the apply mid-flight. The pre-flight pass is not a lock on the filesystem; it removes
the failures graft can see coming, not the ones another program causes.

#### Scenario: A refused destination leaves every write unapplied

- **WHEN** a plan holds three writes in destination order and the third's destination is a
  directory
- **THEN** the apply fails and **none** of the three files was written, including the two
  whose destinations were fine
- **AND** `graft.lock` is unchanged, so a failed apply leaves the tree exactly as it found it

#### Scenario: graft.lock's own destination is checked before anything is applied

- **WHEN** `graft.lock` exists as a directory, and the plan holds a valid write and a valid
  prune path
- **THEN** the apply fails with
  `cannot write "graft.lock": it exists and is not a regular file`, nothing is written, and
  nothing is deleted
- **AND** the lock is not in the plan's writes, so it is checked explicitly: leaving it out
  would apply the whole plan and fail at the final step, with files written, files deleted,
  and no record of either

#### Scenario: A refused prune path leaves every write unapplied

- **WHEN** every write in a plan is valid and one prune path is a directory
- **THEN** the apply fails, no file was written, no file was deleted, and `graft.lock` is
  unchanged
- **AND** the same sync run again fails identically, because nothing changed — which is why
  the failure has to be a message the user can act on rather than a partial state

#### Scenario: A missing source file is refused before anything is written

- **WHEN** a plan's second write names a source path the fetched tree does not hold
- **THEN** the apply fails with an error beginning `source "shared": cannot read "extras/gone.md": `
- **AND** the first write's destination does not exist, because the read was checked before
  any write was performed

### Requirement: Every planned write is copied from its source's fetched tree

For each write in the plan, `internal/apply` SHALL read the file named by the write's source
path from the fetched tree registered under that write's source name, create the
destination's parent directories if they do not exist, and write the bytes to the write's
destination. It SHALL write every planned file regardless of what the destination already
holds: synced files are derived artifacts, always overwritten, never merged. A hand-edited
synced file SHALL be silently overwritten, because `git diff` is the report.

Written files SHALL take mode `0644` and created directories mode `0755`, whatever the mode
of the file in the source tree **and whatever the mode of a file already at the destination**.
Truncating an existing file preserves its mode, so a destination a user or another tool once
made executable would stay executable while graft replaced its contents with
source-controlled bytes; the existing file SHALL therefore be removed and a new one created
rather than truncated in place. graft executes nothing a source provides, and neither a
source nor an accident of history SHALL be able to leave an executable file in a consumer's
repository under graft's name.

A write naming a source with no registered fetched tree SHALL be an error reading
`source "<source>": no fetched tree`, and a source file that is missing, is not a regular
file, or cannot be read SHALL be an error reading
`source "<source>": cannot read "<path>": <reason>`.

#### Scenario: A file is copied into a directory that does not exist yet

- **WHEN** a plan writes `openspec/schemas/tdd/templates/design.md` from the source path
  `extras/openspec-schemas/tdd/templates/design.md`, and the repository holds no `openspec/`
  directory at all
- **THEN** the file exists at that destination with the bytes the source tree held, at mode
  `0644`
- **AND** `openspec/`, `openspec/schemas/`, `openspec/schemas/tdd/`, and
  `openspec/schemas/tdd/templates/` were created at mode `0755`

#### Scenario: A hand-edited synced file is overwritten

- **WHEN** the destination `openspec/schemas/tdd/schema.yaml` already holds
  `edited by hand` and the plan writes it from a source file holding `version: 1`
- **THEN** the destination holds `version: 1` afterwards
- **AND** no backup, no `.orig`, and no merge conflict marker is written anywhere

#### Scenario: An executable source file lands as a non-executable one

- **WHEN** the source tree's file is mode `0755` and the plan writes it to `.claude/hooks/x`
- **THEN** the written file's mode is `0644`
- **AND** the same holds for a source file with the setuid bit set, which is likewise not
  carried across

#### Scenario: An executable destination is made non-executable

- **WHEN** the destination already exists as a regular file at mode `0755` — a file a user
  ran `chmod +x` on and committed, so every checkout has the bit
- **THEN** after the apply the file holds the source's bytes **and** its mode is `0644`
- **AND** this is asserted separately from the fresh-write case, because the permission
  argument to a create-and-truncate open applies only when the file is created

#### Scenario: A source file that cannot be read fails the apply

- **WHEN** a write names the source path `extras/gone.md` and the source's fetched tree does
  not hold it
- **THEN** the apply fails with an error beginning `source "shared": cannot read "extras/gone.md": `
- **AND** the error names the source, so a run with several sources says which one failed

#### Scenario: A write naming an unregistered source fails before it is attempted

- **WHEN** a plan holds a write for source `shared` and no fetched tree is registered under
  that name
- **THEN** the apply fails with `source "shared": no fetched tree`
- **AND** `graft.lock` is not written, because the lock is written only after every file
  operation has succeeded

### Requirement: Only the prune set is deleted, and only when it names a regular file

`internal/apply` SHALL delete exactly the paths in the plan's prune set, and SHALL delete
nothing else under any circumstance. A path in the prune set that does not exist SHALL be
skipped without error, so a sync stays idempotent after a user deletes a synced file by hand.

A prune path that exists but is not a regular file SHALL be an error reading
`cannot remove "<path>": it is not a regular file`, and SHALL NOT be removed. graft only ever
writes regular files, so a directory, a symlink, or a device at a path the lock claims means
the tree is not what the lock says it is, and recursively deleting it would be the one
mistake this whole design exists to prevent.

A prune path with an existing ancestor that is not a directory SHALL be an error reading
`cannot remove "<path>": "<ancestor>" is not a directory`, naming the shallowest such
ancestor, and nothing SHALL be removed. A symlink in the ancestry is the dangerous half: a
lock claiming `vendor/x.md` where `vendor` has become a link to `docs` would otherwise delete
`docs/x.md` — a path no lock claims, deleted by graft, with nothing said.

The prune set arrives from `internal/plan`, which derives it from `graft.lock` alone. A file
absent from `graft.lock` is therefore invisible to `internal/apply`: it cannot be pruned,
because it is never in the set, and `internal/apply` never looks in a directory to find one.

#### Scenario: A foreign file in a shared destination survives every operation

- **WHEN** `.claude/agents/` holds the synced `apply-orchestrator.md` and the repository's own
  `local-reviewer.md`, and a plan is applied that prunes `.claude/agents/apply-orchestrator.md`
  and writes nothing there
- **THEN** `.claude/agents/apply-orchestrator.md` is gone and `.claude/agents/local-reviewer.md`
  is present with its original bytes
- **AND** `.claude/agents/` still exists, because it is not empty
- **AND** the same holds when the plan instead rewrites the synced file, and when the plan
  writes a second synced agent beside it: `local-reviewer.md` is never read, never written,
  and never deleted

#### Scenario: Unrecorded files in a destination directory are never enumerated

- **WHEN** a destination directory holds ten files the lock does not record and one it does,
  and the plan's prune set names that one
- **THEN** exactly that one file is deleted and the other ten are untouched
- **AND** no directory listing was performed to reach that outcome: the prune set handed in is
  the only source of deletions this package has

#### Scenario: A prune path that is already gone is not an error

- **WHEN** the prune set names `openspec/schemas/tdd/schema.yaml` and the user has already
  deleted it
- **THEN** the apply succeeds and `graft.lock` is written
- **AND** nothing else in `openspec/` is deleted
- **AND** the directory it used to live in is **not** removed even if it is empty: nothing was
  unlinked, so graft did not empty it, and removing it would be graft deleting something it
  has no record of

#### Scenario: A file this run just wrote is never pruned

- **WHEN** a source renames `Foo.md` to `foo.md`, so the plan writes `.claude/agents/foo.md`
  and prunes `.claude/agents/Foo.md`, and the repository is on a case-insensitive filesystem
  where those name one file
- **THEN** `.claude/agents/foo.md` holds the source's bytes afterwards, and `graft.lock`
  claims a path that is on disk
- **AND** the two are told apart by file identity rather than by folded strings, so on a
  case-sensitive filesystem — where they really are two files — both operations still happen

#### Scenario: A prune path that is a directory is refused

- **WHEN** the prune set names `docs/api` and `docs/api` is a directory holding `index.md`
- **THEN** the apply fails with `cannot remove "docs/api": it is not a regular file`
- **AND** `docs/api/index.md` still exists, and `graft.lock` is not written

#### Scenario: A prune path that is a symlink is refused

- **WHEN** the prune set names `.claude/agents/x.md` and that path is a symlink to
  `.claude/agents/local-reviewer.md`
- **THEN** the apply fails with `cannot remove ".claude/agents/x.md": it is not a regular file`
- **AND** `local-reviewer.md` still exists with its original bytes, because deleting a symlink
  graft did not write is a claim about the tree the lock cannot support

#### Scenario: A prune path under a symlinked parent is refused

- **WHEN** the prune set names `vendor/x.md`, `vendor` is a symlink to the repository's own
  `docs/`, and `docs/x.md` exists
- **THEN** the apply fails with `cannot remove "vendor/x.md": "vendor" is not a directory`
- **AND** `docs/x.md` still exists with its original bytes, which is the whole point: the lock
  claims `vendor/x.md`, and the file behind that name today is one graft never wrote

#### Scenario: A prune path whose parent is a regular file is refused

- **WHEN** the prune set names `docs/x.md` and `docs` has been replaced by a regular file
- **THEN** the apply fails with `cannot remove "docs/x.md": "docs" is not a directory`
- **AND** the message names the offending ancestor rather than surfacing a raw syscall error,
  because the user's next move is to look at that path

### Requirement: Directories left empty by the prune set are removed

After the prune set has been executed, `internal/apply` SHALL remove every directory that a
pruned path leaves empty, walking upward from each pruned path's parent toward the repository
root, deepest first. A candidate SHALL be examined without following it and SHALL be removed
only if it is a directory: unlinking a symlink succeeds however full its target is, so a bare
removal would delete a user's `vendor -> docs` link — a path absent from `graft.lock` — while
the reasoning that a non-empty directory "fails harmlessly" looked sound.

A removal that fails for any reason SHALL be ignored and SHALL NOT fail the apply. A non-empty
directory is the ordinary case and is not an error; a directory that cannot be removed for
some other reason is a tidying step that did not happen, and failing the run over it — after
the prunes have already been performed and before the lock has been written — would strand
the sync in the state it is least able to explain.

The repository root SHALL never be a candidate. Only the ancestors of pruned paths SHALL be
considered: a directory that was already empty before the sync and that no pruned path lived
in SHALL be left alone, because graft did not empty it and removing it would be graft deleting
something it has no record of.

#### Scenario: An emptied directory chain is removed

- **WHEN** the prune set names `openspec/schemas/tdd/templates/design.md` and
  `openspec/schemas/tdd/schema.yaml`, those are the only files under `openspec/`, and nothing
  is written there
- **THEN** all of `openspec/schemas/tdd/templates`, `openspec/schemas/tdd`,
  `openspec/schemas`, and `openspec` are gone
- **AND** the repository root still exists and its other entries are untouched

#### Scenario: A directory still holding a foreign file is kept

- **WHEN** the prune set names `.claude/agents/apply-orchestrator.md` and
  `.claude/agents/local-reviewer.md` is a repo-owned file beside it
- **THEN** `.claude/agents/` and `.claude/` still exist
- **AND** `local-reviewer.md` is untouched

#### Scenario: A directory a write still fills is kept

- **WHEN** the prune set names `openspec/schemas/tdd/templates/old.md` and the same apply
  writes `openspec/schemas/tdd/templates/new.md`
- **THEN** `openspec/schemas/tdd/templates/` still exists holding `new.md` only
- **AND** the removal ran after the writes, which is why the directory was not empty when it
  was considered

#### Scenario: An unrelated empty directory is left alone

- **WHEN** the repository holds an empty `scratch/` directory that no pruned path lives in,
  and the prune set names `docs/old.md` where `docs/` holds nothing else
- **THEN** `docs/` is removed and `scratch/` still exists
- **AND** the candidate set is the ancestry of the pruned paths, so `scratch/` was never a
  candidate

#### Scenario: A symlinked ancestor of a pruned path never becomes a candidate

- **WHEN** the prune set names `agents/x.md`, `agents` is a symlink to the repository's own
  `shared/`, and `shared/` holds other files
- **THEN** the apply fails at the pre-flight pass with
  `cannot remove "agents/x.md": "agents" is not a directory`, and the symlink and `shared/`
  are both untouched
- **AND** the removal walk is never reached, because the ancestor rule refuses the prune path
  first. Its own non-directory check is therefore unreachable through an apply, and is kept as
  a floor rather than as behavior this scenario observes: it costs one `Lstat`, and the pass
  that makes it redundant is one refactor away from not doing so

#### Scenario: A pruned path at the repository root removes nothing

- **WHEN** the prune set names `README.md` at the repository root, and it is the only entry
- **THEN** `README.md` is deleted and the repository root still exists
- **AND** the root was never a removal candidate

### Requirement: graft never writes inside `.git` and never writes over its own two files

A write destination or a prune path whose first path segment is `.git` SHALL be refused,
naming the path:

```
cannot write "<path>": graft never writes inside ".git"
cannot remove "<path>": graft never removes inside ".git"
```

A write destination or a prune path equal to `graft.toml` or `graft.lock` SHALL be refused the
same way, with `graft never writes over "<name>"` and `graft never removes "<name>"`. A path
that names nothing — the empty string, or `.` — SHALL be refused with
`it does not name a file`.

All three comparisons SHALL fold case, and SHALL do so on every platform rather than only
where the filesystem does. On APFS and on NTFS `.GIT/config` **is** `.git/config`: a
byte-exact comparison is no refusal at all there, and the write lands in the real file while
a prune aimed at it takes the repository with it. A directory genuinely named `.GIT` on a
case-sensitive filesystem is not a destination worth supporting, and one rule everywhere
beats a rule that holds on some machines.

This is a floor under the writer rather than a restatement of a planning rule. `internal/plan`
refuses a destination that escapes the repository root, and `.git/config` does not escape it;
`kinds` are arbitrary and no rule anywhere constrains what a `to` may *name*. The reason the
floor is here and not merely upstream is what SPEC.md offers as the whole mitigation for an
untrusted source: "every sync's effect is a git diff". A file placed in `.git/` is invisible to
that — it is not tracked, so `git status` says nothing — and `.git/config` alone turns placing
a file into running a program, through `core.fsmonitor`, `core.sshCommand`, or an alias, on the
user's next git command. The one thing standing between "graft executes nothing a source
provides" and its opposite would be a path string nothing checks.

`graft.toml` is the consumer's own request and `graft.lock` is graft's own record; an item
placed at either would be silently destroyed by the run that installed it, and the lock would
then claim a file a later prune would delete.

#### Scenario: A destination inside .git is refused

- **WHEN** a plan writes `.git/config`
- **THEN** the apply fails with `cannot write ".git/config": graft never writes inside ".git"`
- **AND** `.git/config` is byte-identical to what it was, and nothing else in the plan was
  written either, because the check runs before the first write

#### Scenario: A prune path inside .git is refused

- **WHEN** a hand-edited `graft.lock` claims `.git/hooks/pre-commit` and the prune set
  therefore names it
- **THEN** the apply fails with
  `cannot remove ".git/hooks/pre-commit": graft never removes inside ".git"`
- **AND** the file still exists

#### Scenario: A destination of graft.toml or graft.lock is refused

- **WHEN** a plan writes `graft.toml`, and separately `graft.lock`
- **THEN** each fails with `cannot write "graft.toml": graft never writes over "graft.toml"`
  and `cannot write "graft.lock": graft never writes over "graft.lock"`
- **AND** both files are byte-identical to what they were

#### Scenario: A respelling of .git in another case is refused too

- **WHEN** a plan writes `.GIT/config`, and separately `.Git/config`, `GRAFT.TOML`, and
  `Graft.Lock`, and separately prunes `.GIT/hooks/pre-commit`
- **THEN** each is refused, and the real `.git/config` and `.git/hooks/pre-commit` are
  byte-identical to what they were
- **AND** this holds on a case-sensitive filesystem as well, where the refusal is
  unnecessary: the rule does not depend on which filesystem the repository is on

#### Scenario: A path merely beginning with .git is not inside it

- **WHEN** a plan writes `.github/workflows/ci.yml` and `.gitignore`
- **THEN** both are written, because the rule is on the first path **segment** and neither
  segment is `.git`
- **AND** placing a workflow file remains something ENGINEERING.md's security note accepts
  and names: CI runs it, and trusting a source means trusting what it places

### Requirement: graft.lock is written last

`internal/apply` SHALL write `graft.lock` at the repository root, serialized by
`internal/lock`, only after every write, every deletion, and every directory removal has
succeeded. If any of those fails, `graft.lock` SHALL NOT be written at all, so a run that
failed partway leaves a lock describing the previous state rather than a state that never
existed.

The lock SHALL be written on every successful apply, including one that changed nothing:
identical bytes produce no diff, and a conditional write would make the file's presence
depend on a comparison this package is forbidden to make.

#### Scenario: A write that fails mid-flight leaves the previous lock in place

- **WHEN** a plan's second write fails for a reason the pre-flight pass could not see — the
  destination directory was made read-only by another process after it was checked — and
  `graft.lock` already exists from an earlier sync
- **THEN** the apply fails and `graft.lock` still holds exactly the bytes it held before
- **AND** the first write did land, which is why the lock is the previous one rather than a
  new one: a lock written now would claim a file the run never wrote

#### Scenario: An unchanged sync still writes the lock

- **WHEN** a plan is applied twice in a row from the same inputs
- **THEN** both applies succeed and `graft.lock` holds byte-identical content after each
- **AND** byte equality is asserted, not semantic equality, because a lock that reorders on
  every run churns the diff of every consumer

#### Scenario: The lock that is written is the plan's lock

- **WHEN** a plan whose next lock records source `shared` with item `schema:tdd` holding two
  files is applied
- **THEN** `graft.lock` parses back through `internal/lock` yielding exactly that source, item,
  and file list
- **AND** the file begins with the generated-file header line

### Requirement: A destination that is not a regular file is refused rather than overwritten

`internal/apply` SHALL check the destination without following it. A destination that exists
and is not a regular file SHALL be an error reading
`cannot write "<path>": it exists and is not a regular file`, and SHALL NOT be written,
opened, or truncated. A destination with an existing ancestor that is not a directory SHALL be
an error reading `cannot write "<path>": "<ancestor>" is not a directory`, naming the
shallowest such ancestor.

graft only ever writes regular files, so anything else at a destination is something graft
did not put there. Following a symlink would let a file the repository owns redirect a write
to another path in the tree — a path no lock claims and no prune could ever reach — and
truncating a directory is not a thing that can be done at all.

#### Scenario: A directory at a destination is refused

- **WHEN** the plan writes `docs/api` and `docs/api` is a directory
- **THEN** the apply fails with `cannot write "docs/api": it exists and is not a regular file`
- **AND** nothing under `docs/api/` is modified or removed, and `graft.lock` is not written

#### Scenario: A symlink at a destination is refused rather than followed

- **WHEN** the plan writes `.claude/agents/x.md` and that path is a symlink to
  `.claude/agents/local-reviewer.md`
- **THEN** the apply fails with
  `cannot write ".claude/agents/x.md": it exists and is not a regular file`
- **AND** `local-reviewer.md` holds exactly its original bytes, because the check ran before
  anything was opened for writing

#### Scenario: A destination under a symlinked parent is refused

- **WHEN** the plan writes `.claude/agents/x.md` and `.claude` is a symlink to a directory
  elsewhere in the repository
- **THEN** the apply fails with
  `cannot write ".claude/agents/x.md": ".claude" is not a directory`
- **AND** nothing is written through the link, because a file placed there would sit at a path
  `graft.lock` does not name and a later prune would aim somewhere else entirely

#### Scenario: A destination whose parent is a regular file is named

- **WHEN** the plan writes `openspec/schemas/x.yaml` and `openspec` is a regular file
- **THEN** the apply fails with
  `cannot write "openspec/schemas/x.yaml": "openspec" is not a directory`
- **AND** the message is graft's own rather than a raw `not a directory` syscall error, which
  names neither the destination nor the path the user has to fix

### Requirement: Application is contained by the repository root

Every path `internal/apply` reads, writes, creates, or removes in the consumer's repository
SHALL be resolved through an `os.Root` opened at the repository root, and every path it reads
from a source SHALL be resolved through an `os.Root` opened at that source's fetched tree. A
path that resolves outside its root SHALL fail rather than being reached.

`internal/plan` already refuses a destination that escapes the repository root, and
`internal/source` already contains its own reads. This is the floor under both rather than a
restatement of either: it is the check that holds when a check upstream is wrong, and it costs
one call. It is not by itself sufficient — an `os.Root` follows a symlink that stays inside its
root, which is why the ancestor rules above exist — so the two are stated separately and tested
separately.

A repository root that does not exist or cannot be opened SHALL be an error reading
`cannot open the repository root "<path>": <reason>`, raised before any file operation is
attempted.

#### Scenario: A destination escaping the root fails rather than being written

- **WHEN** a plan is applied — bypassing `internal/plan`, which would have refused it —
  holding a write whose destination is `../outside.md`
- **THEN** the apply fails, naming the path, and no file named `outside.md` exists beside the
  repository root
- **AND** the same holds for a prune path of `../outside.md`, which deletes nothing

#### Scenario: A source path escaping the fetched tree fails rather than being read

- **WHEN** a write's source path is `../elsewhere/secret.md` and a file exists at that
  location beside the fetched tree
- **THEN** the apply fails with an error naming the source and that path
- **AND** the destination is not created, so nothing from outside the source tree reaches the
  repository

#### Scenario: A missing repository root is named

- **WHEN** a plan is applied against a root that does not exist
- **THEN** the apply fails with an error beginning `cannot open the repository root `
- **AND** no directory is created, because graft never creates the repository it runs in

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

### Requirement: A manifest-only apply writes graft.toml and nothing else

`internal/apply` SHALL provide a manifest-only entry point, for `graft add --no-sync`, that
writes the given `graft.toml` bytes to the repository root and does nothing else: no planned
write, no deletion, no empty-directory removal, and no `graft.lock`. A run with nothing to
sync must still be able to record what the consumer asked for, and it may not reach that
through a plan — an empty plan against a populated lock is a prune of everything.

It SHALL write through the same mechanism the plan-carrying path uses: the same staging path
at the repository root, the same rename over the destination, the same pre-flight refusal of
a `graft.toml` or a staging path that exists and is not a regular file, and the same removal
of a temporary file left behind by a failed rename. At no point SHALL `graft.toml` be absent
or partially written.

It SHALL be contained exactly as every other write is: through the repository root's
`os.Root`, with every ancestor of the destination checked with `Lstat` and required to be a
real directory rather than a symlink to one.

`internal/apply` SHALL remain the only package that writes to the working tree. This entry
point widens what a caller may ask for; it does not add a second writer.

#### Scenario: Only graft.toml appears

- **WHEN** a manifest-only apply runs in a repository holding no `graft.toml` and no
  `graft.lock`
- **THEN** `graft.toml` holds exactly the given bytes
- **AND** `graft.lock` does not exist and no other file was created

#### Scenario: An existing lock is left alone

- **WHEN** a manifest-only apply runs in a repository whose `graft.lock` records two sources
  and whose destinations hold their files
- **THEN** `graft.lock` is byte-identical and every destination file still exists
- **AND** nothing is pruned, because a manifest-only apply has no prune set

#### Scenario: A graft.toml that is not a regular file is refused

- **WHEN** `graft.toml` exists as a directory and a manifest-only apply runs
- **THEN** the apply fails with the wording the pre-flight check already produces, naming the
  path
- **AND** nothing is written and no staging file is left behind

#### Scenario: A staging leftover that is not a regular file is refused

- **WHEN** `.graft.toml.tmp` exists as a symlink to a file outside the repository and a
  manifest-only apply runs
- **THEN** the apply fails naming the staging path, with the wording that check already
  produces
- **AND** `graft.toml` is not written, the symlink is not removed, and its target is
  untouched — deleting a path no lock claims is the one thing graft may never do

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
