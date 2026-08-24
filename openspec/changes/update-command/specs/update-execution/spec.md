## ADDED Requirements

### Requirement: `graft update` re-resolves every pin and then syncs

`graft update`, invoked with no argument, SHALL re-resolve the `rev` of every source
`graft.toml` declares — a tag or branch through `git ls-remote`, a full sha passed through —
and SHALL then perform every step `graft sync` performs with the result: fetch each sha into
the content-addressed cache, read each source's `catalog.yaml`, expand its selectors, list each
installed item's files, build the plan, apply it, and write `graft.lock` last.

It SHALL NOT take a source's sha from `graft.lock` because the lock happens to hold one. That
is the single behavioral difference between `update` and `sync`, and it is the reason this
command exists: `sync` installs what the lock says, and `update` is the one command permitted
to move what the lock says.

Everything else `sync` guarantees SHALL hold unchanged. Nothing SHALL be created, modified, or
deleted in the repository until the plan is built, so a failure in loading, resolution,
fetching, catalog reading, listing, or planning SHALL leave the working tree byte-identical and
`graft.lock` unwritten. The prune set SHALL still come from `graft.lock` alone, so a file the
lock does not claim SHALL NOT be deleted. `graft.lock` SHALL still be written last, after every
file operation has succeeded.

A source recorded in `graft.lock` that `graft.toml` no longer declares SHALL NOT be resolved
and SHALL NOT be fetched, exactly as under `sync`: its rev is declared nowhere, so there is
nothing to re-resolve, and its files are pruned from the lock alone.

The orchestration SHALL take the repository root and the fetch-cache root as values rather than
reading a global, and `graft update` SHALL pass the working directory and `internal/source`'s
default cache root.

#### Scenario: A moved branch moves the pin

- **WHEN** `graft.lock` records source `shared` at `rev = "main"` resolved to `<old sha>`, the
  branch `main` in the source repository has since advanced with a new file added to an
  installed item, and `graft update` runs
- **THEN** the new file exists at its destination and the exit code is `0`
- **AND** `graft.lock`'s `resolved` for `shared` is the sha `main` names now, not `<old sha>`
- **AND** this is the case `graft sync` refuses: the same repository synced instead of updated
  keeps `<old sha>` and does not write the new file

#### Scenario: An update that finds nothing new reports nothing

- **WHEN** `graft.lock` records source `shared` at `rev = "v1.0.0"`, the tag has not moved, and
  `graft update` runs
- **THEN** the error stream holds exactly `up to date` and one newline
- **AND** the exit code is `0`, `graft.lock` is byte-identical, and every installed file is
  byte-identical

#### Scenario: An update in a repository with no lock installs everything

- **WHEN** `graft.toml` declares source `shared` at `rev = "v1.0.0"` installing `schema:tdd`
  and `agent:*`, and no `graft.lock` exists
- **THEN** every file each item places exists at its destination and `graft.lock` records the
  source, its resolved sha, and each item's files
- **AND** the exit code is `0`, so an update in a never-synced repository is not a special case

#### Scenario: A source dropped from the manifest is pruned without being re-resolved

- **WHEN** `graft.lock` records source `retired` with two files, `graft.toml` no longer declares
  it, and `graft update` runs
- **THEN** both files are deleted and `graft.lock` no longer holds that source
- **AND** no resolution and no fetch is attempted for `retired`, because its rev is declared
  nowhere

#### Scenario: A rev that no longer exists fails without touching the tree

- **WHEN** `graft.toml` declares `rev = "v9.9.9"` for source `shared`, the source repository has
  no such tag or branch, and `graft update` runs
- **THEN** the error stream holds `graft: source "shared": rev "v9.9.9" not found` and the exit
  code is `1`
- **AND** every installed file and `graft.lock` are byte-identical to what they were

#### Scenario: An item the new rev no longer provides is removed, and a repo-owned file beside it survives

- **WHEN** source `shared` is pinned at a rev providing `agent:reviewer` and `agent:planner`,
  the destination `.claude/agents/` also holds a repo-owned `local-reviewer.md` that no lock
  claims, the source's newer rev stops providing `agent:planner`, and `graft update` runs
- **THEN** `.claude/agents/planner.md` is deleted, the report reports `agent:planner` as
  `removed` with the note `no longer provided`, and the exit code is `0`
- **AND** `.claude/agents/local-reviewer.md` is byte-identical and still present, because the
  prune set comes from `graft.lock` alone and the lock never claimed it
- **AND** `.claude/agents/` still exists, because it is not empty

#### Scenario: A manifest declaring no sources updates nothing

- **WHEN** `graft.toml` parses and declares no source at all, and `graft update` runs against a
  repository with no `graft.lock`
- **THEN** the error stream holds exactly `up to date` and one newline and the exit code is `0`
- **AND** nothing was resolved, nothing was fetched, no directory was created, and `graft.lock`
  is written holding the header and `version = 1`

### Requirement: `graft update <source>` moves that source's pin and no other

`graft update <source>` SHALL re-resolve only the named source. Every other source the manifest
declares SHALL have its sha taken from `graft.lock` and SHALL NOT be re-resolved, so moving one
pin can never move another. A source the lock has no entry for is resolved whether or not it is
named, because there is no pin to take.

A `<source>` that `graft.toml` does not declare SHALL fail with

```
graft.toml has no source "<source>"
```

and SHALL exit `1`. The check SHALL run before any resolution and any fetch, so a mistyped
source name causes no network access and no write. A source present in `graft.lock` but absent
from `graft.toml` is not a source `update` can move: its rev is declared nowhere.

#### Scenario: Only the named source is re-resolved

- **WHEN** `graft.toml` declares `shared` at `rev = "main"` and `extra` at `rev = "main"`, both
  branches have advanced since the lock was written, and `graft update shared` runs
- **THEN** `graft.lock`'s `resolved` for `shared` is the sha `main` names now
- **AND** `graft.lock`'s `resolved` for `extra` is unchanged, and `extra`'s files on disk are
  those of the recorded sha

#### Scenario: A source name the manifest does not declare is refused

- **WHEN** `graft update sharde` runs against a manifest declaring `shared`
- **THEN** the error stream holds `graft: graft.toml has no source "sharde"` and the exit code
  is `1`
- **AND** nothing was fetched, no file was written, and `graft.lock` is byte-identical

#### Scenario: A source in the lock but not the manifest cannot be updated

- **WHEN** `graft.lock` records `retired` and `graft.toml` does not declare it, and
  `graft update retired` runs
- **THEN** the error stream holds `graft: graft.toml has no source "retired"` and the exit code
  is `1`
- **AND** `retired`'s files are still on disk, because a refused run prunes nothing

### Requirement: `graft update --to <rev> <source>` moves the pin in `graft.toml` first

`graft update --to <rev> <source>` SHALL rewrite `graft.toml` so that the named source's `rev`
is `<rev>`, and SHALL then re-resolve that source at the new rev and sync exactly as
`graft update <source>` does.

The rewrite SHALL preserve every other byte of the file — comments, key order, key alignment,
blank lines, and every other source — because `graft.toml` is written by a human and reviewed
in a diff.

`graft.toml` SHALL be written only after every check has passed and every file write, deletion,
and directory removal has succeeded, immediately before `graft.lock`. A run that fails at any
earlier point SHALL leave `graft.toml` byte-identical, so a failed update never leaves the
manifest pointing somewhere the lock has not followed.

**The order of the two refusals is fixed.** The source's membership in `graft.toml` SHALL be
checked first, so a mistyped source name produces `graft.toml has no source "<name>"` rather
than the manifest-editing refusal, which would be technically true and useless. The edit SHALL
then be attempted, and a manifest shape the editor cannot rewrite exactly SHALL fail the run
with the message `manifest-format` specifies — before any resolution and any fetch, and with
`graft.toml` byte-identical.

**The edited bytes SHALL be re-parsed before the run uses them, and SHALL be checked against
what was asked for.** If they do not parse, or if the named source's `rev` in the re-parsed
manifest is not `<rev>`, the run SHALL fail without writing anything. What goes to disk and what
the run resolves must be the same thing, and re-parsing is the only way to prove it; the check
on the value is what catches an edit that landed on the wrong line — a commented-out `rev`
above the real one, or a key in a sub-table — which a parse alone would accept.

`--to` SHALL require a source. `graft update --to <rev>` with no positional argument SHALL be a
usage error reading `--to requires a source`, followed by the hint line, with the exit code `1`.
`--to` with an empty value SHALL be a usage error reading `--to requires a rev`, in the same
form. Neither SHALL read `graft.toml`, resolve anything, or fetch anything.

`graft update` SHALL write `graft.toml` only when `--to` is given. Without it the manifest SHALL
be byte-identical however far a pin moves. (`graft add` also writes `graft.toml`, under its own
capability; this requirement bounds `update` alone.)

#### Scenario: The pin moves and the rest of the file survives

- **WHEN** `graft.toml` holds a leading comment, an aligned `[sources.shared]` block at
  `rev     = "v1.0.0"`, and a second source `extra`, and `graft update --to v1.1.0 shared` runs
- **THEN** `graft.toml` afterwards differs from before in exactly one line, which reads
  `rev     = "v1.1.0"` with its original alignment
- **AND** the comment, the key order, and the whole of `[sources.extra]` are byte-identical
- **AND** `graft.lock` records `shared` at `rev = "v1.1.0"` and the sha that tag names

#### Scenario: An update without `--to` never writes the manifest

- **WHEN** source `shared` is pinned at `rev = "main"`, the branch has advanced, and
  `graft update` runs
- **THEN** `graft.lock`'s `resolved` moves and `graft.toml` is byte-identical to what it was
- **AND** the `rev` in the lock is still `main`, because the request did not change — only what
  it resolved to did

#### Scenario: `--to` with no source is a usage error

- **WHEN** `graft update --to v1.1.0` is invoked
- **THEN** the error stream holds `graft: --to requires a source` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty, the exit code is `1`, and `graft.toml` is
  byte-identical

#### Scenario: `--to` with an empty rev is a usage error

- **WHEN** `graft update --to "" shared` is invoked
- **THEN** the error stream holds `graft: --to requires a rev` followed by the hint line, and
  the exit code is `1`
- **AND** `graft.toml` is byte-identical

#### Scenario: A `--to` rev that does not exist leaves the manifest where it was

- **WHEN** `graft update --to v9.9.9 shared` runs and the source repository has no such tag or
  branch
- **THEN** the error stream holds `graft: source "shared": rev "v9.9.9" not found` and the exit
  code is `1`
- **AND** `graft.toml` still says `rev = "v1.0.0"`, `graft.lock` is byte-identical, and every
  installed file is byte-identical

#### Scenario: A `--to` naming a source the manifest does not declare is refused

- **WHEN** `graft update --to v1.1.0 sharde` runs against a manifest declaring `shared`
- **THEN** the error stream holds `graft: graft.toml has no source "sharde"` and the exit code
  is `1`
- **AND** `graft.toml` is byte-identical and nothing was resolved or fetched
- **AND** the message is the membership one rather than the manifest-editing one, because the
  membership check runs first

#### Scenario: A `--to` against a manifest shape the editor cannot rewrite is refused

- **WHEN** `graft.toml` declares its sources as inline tables under `[sources]` and
  `graft update --to v1.1.0 shared` runs
- **THEN** the error stream holds
  `graft: graft.toml: source "shared": cannot move the pin: rev is not a plain key under [sources.shared]`
  and the exit code is `1`
- **AND** `graft.toml` is byte-identical, `graft.lock` is byte-identical, and nothing was
  resolved or fetched
- **AND** `graft update shared` against the same manifest succeeds, because only `--to` needs to
  edit the file

### Requirement: The pin check applies only to the sources this run does not re-resolve

`graft sync` refuses a `graft.toml` and `graft.lock` that disagree on a rev, pointing at
`graft update`. `graft update` SHALL skip that check for every source it re-resolves and SHALL
apply it, unchanged, to every source it does not — the disagreement is real for a source whose
sha still comes from the lock, and it is precisely what an update of that source repairs.

`graft update` with no argument therefore SHALL NOT fail the pin check for any source the
manifest declares. `graft update <source>` SHALL still fail it when a *different* source
disagrees, with the message that command already produces, before anything is fetched.

#### Scenario: An update repairs a manifest whose rev was moved by hand

- **WHEN** `graft.toml` says `rev = "v1.1.0"` for source `shared` while `graft.lock` says
  `v1.0.0`, and `graft update` runs
- **THEN** the run succeeds with the exit code `0` and `graft.lock` afterwards records
  `rev = "v1.1.0"` and the sha that tag names
- **AND** this is the run `graft sync` refuses with
  ``graft: graft.toml has rev "v1.1.0" for source "shared" but graft.lock has "v1.0.0"; run `graft update` to move the pin``

#### Scenario: Updating one source still refuses another source's disagreement

- **WHEN** `graft.toml` and `graft.lock` disagree on source `extra`'s rev, and
  `graft update shared` runs
- **THEN** the error stream holds
  ``graft: graft.toml has rev "<manifest>" for source "extra" but graft.lock has "<lock>"; run `graft update` to move the pin``
- **AND** the exit code is `1`, nothing was fetched, and `graft.lock` is byte-identical

### Requirement: `graft update` reports what moved

`graft update` SHALL render the report the `sync-report` capability specifies, unaltered and to
the **error** stream: the same source headers, the same item lines and notes, the same summary
line, the same `up to date` predicate, and the same colour rule. There is no second rendering
and no update-specific wording — a reader who has read one report has read both.

What this requirement adds is that an update is the run in which `sync-report`'s two-sided
header forms actually appear, since a pin moving is what produces them, and the scenarios below
pin that. `sync-report` is not restated here; it is the owner.

Nothing SHALL be written to the **standard output** stream on any path, success or failure.

#### Scenario: A bumped tag shows both revs and both shas

- **WHEN** `graft update --to v1.1.0 shared` moves the source from `v1.0.0` at `<sha a>` to
  `v1.1.0` at `<sha b>`
- **THEN** the error stream carries the header
  `shared  v1.0.0 -> v1.1.0  (<a[:7]> -> <b[:7]>)` followed by a blank line and the item lines
- **AND** the summary line reads ``<n> files written, <m> removed - review with `git diff` ``

#### Scenario: A branch whose sha moved shows one rev and both shas

- **WHEN** a source pinned at `rev = "main"` is moved by `graft update` from `<sha a>` to
  `<sha b>`
- **THEN** the header reads `shared  main  (<a[:7]> -> <b[:7]>)`, the rev rendered once because
  it did not move

#### Scenario: An update leaves standard output byte-empty

- **WHEN** `graft update` completes successfully, and separately fails because `graft.toml` is
  missing
- **THEN** the standard output stream is byte-empty in both cases
- **AND** the report is on the error stream in the first and the one-line error in the second

### Requirement: `graft update --dry-run` prints the plan and touches nothing

`graft update --dry-run` SHALL perform every step through building the plan — resolution and
fetching included — print the report the update would have produced, exit `0`, and create,
modify, or delete nothing. No file SHALL be written, no file deleted, **no directory created**,
`graft.lock` SHALL NOT be written, and `graft.toml` SHALL NOT be moved even when `--to` is
given.

The summary line SHALL read `<n> files to write, <m> to remove - nothing written`, and an update
with nothing to do SHALL still print `up to date`: `--dry-run` changes what the summary says,
not what "nothing to do" means.

A dry run stops after the plan is built and therefore reaches none of `internal/apply`'s
refusals. A clean dry run says the plan is valid, not that the update will succeed.

#### Scenario: A dry run of an update writes neither of graft's files

- **WHEN** a source pinned at `rev = "main"` has advanced and `graft update --dry-run` runs
- **THEN** the report names the items that would be updated, the summary reads
  `<n> files to write, <m> to remove - nothing written`, and the exit code is `0`
- **AND** `graft.lock` still records the old sha and every installed file is byte-identical

#### Scenario: A dry run of `--to` leaves the manifest where it was

- **WHEN** `graft update --dry-run --to v1.1.0 shared` runs and the tag exists
- **THEN** the report shows the header `shared  v1.0.0 -> v1.1.0  (…)` and the exit code is `0`
- **AND** `graft.toml` still says `rev = "v1.0.0"` and `graft.lock` is byte-identical

#### Scenario: A dry run of a first update creates no directory

- **WHEN** `graft update --dry-run` runs in a repository with a manifest, no lock, and nothing
  installed
- **THEN** the exit code is `0` and the report names every item that would be added
- **AND** no destination directory exists, no destination file exists, and `graft.lock` does not
  exist

### Requirement: `graft update` takes at most one argument and adds exactly two flags

`graft update` SHALL accept at most one positional argument, the source name. A second argument
SHALL be refused as a usage error reading `unknown argument "<argument>"`, followed by the hint
line, with the exit code `1`. An unrecognised flag SHALL be refused as a usage error in the same
way, so there is no `--force` and no `--frozen` here either.

`--to` and `--dry-run` SHALL be the only flags `graft update` adds; `--help` exists on every
command and goes to the standard output stream with the exit code `0`.

`graft update` SHALL exit `0` when the update completes and `1` on any failure. There is no
third outcome: an update that could not finish reports the reason and leaves the tree as it
found it.

That `graft`'s help lists `update` follows from `command-invocation`'s rule that the commands
section lists every subcommand, and is specified there rather than restated here.

#### Scenario: A second argument is a usage error

- **WHEN** `graft update shared extra` is invoked
- **THEN** the error stream holds `graft: unknown argument "extra"` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty and the exit code is `1`

#### Scenario: An unknown flag is a usage error

- **WHEN** `graft update --force` is invoked
- **THEN** the error stream holds `graft: unknown flag: --force` followed by
  `run "graft --help" for usage`, which is cobra's own wording reported through graft's format,
  exactly as `command-invocation` already specifies for the root
- **AND** the exit code is `1`, the standard output stream is empty, and nothing was read,
  resolved, or written
