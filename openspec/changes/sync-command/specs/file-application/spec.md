## ADDED Requirements

### Requirement: Applying a plan is the only path that writes to the working tree

`internal/apply` SHALL be the only package in graft permitted to create, modify, or delete
anything in the repository graft is running in. It SHALL expose one entry point taking the
repository root, each source's fetched tree by source name, and a `plan.Plan`, and SHALL
perform exactly the operations that plan describes, in this order:

1. Check everything checkable, touching nothing.
2. Write the planned files.
3. Delete the prune set.
4. Remove the directories the prune set left empty.
5. Write `graft.lock`.

Steps 2 through 5 are SPEC.md's resolution step 8. Step 1 is what makes the refusals in this
specification leave the tree byte-identical rather than partly applied — every condition they
test is decidable before the first byte is written, and discovering one halfway through would
guarantee the partial apply the design otherwise works to avoid.

`internal/apply` SHALL derive nothing for itself. It SHALL NOT scan a destination directory,
SHALL NOT compare file contents, SHALL NOT decide a file needs no writing, and SHALL NOT
expand, re-order, or extend the prune set. Every path it touches SHALL come from the plan it
was given, which is what makes `internal/plan`'s purity worth having: the checks that ran
before anything was written are the checks that govern what gets written.

Applying a plan holding no writes, no prunes, and a lock with no sources SHALL succeed,
writing only `graft.lock`.

#### Scenario: A plan's operations happen in the documented order

- **WHEN** a plan is applied whose prune set names `openspec/schemas/tdd/templates/old.md` and
  whose writes include `openspec/schemas/tdd/templates/new.md`, that directory holding nothing
  else
- **THEN** the apply succeeds, `templates/` still exists holding `new.md` only, and `old.md`
  is gone — an outcome only the documented order produces, since pruning first would have
  emptied the directory before the write refilled it, and removing directories before pruning
  would have left `old.md` behind
- **AND** ordering is asserted through effects the real filesystem produces, not through an
  observation seam: `internal/apply` exposes one function and no injection point

#### Scenario: An empty plan writes only the lock

- **WHEN** a plan holding no writes, an empty prune set, and a lock with zero sources is
  applied to a repository holding one repo-owned file
- **THEN** `graft.lock` is written holding the header and `version = 1`
- **AND** the repo-owned file is present and unchanged, and no directory was created

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

#### Scenario: A symlinked ancestor of a pruned path is not removed

- **WHEN** the prune set names `agents/x.md`, `agents` is a symlink to the repository's own
  `shared/`, `shared/` holds other files, and the prune path itself passes every check because
  the plan was hand-built
- **THEN** the symlink `agents` still exists and `shared/` still holds its files
- **AND** the candidate was skipped because it is not a directory, rather than removed because
  removing it would have "failed harmlessly"

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
same way, with `graft never writes over "<name>"` and `graft never removes "<name>"`.

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
