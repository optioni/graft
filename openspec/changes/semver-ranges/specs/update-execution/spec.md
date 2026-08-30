## MODIFIED Requirements

### Requirement: `graft update` re-resolves every pin and then syncs

`graft update`, invoked with no argument, SHALL re-resolve the `rev` of every source
`graft.toml` declares — a tag or branch through `git ls-remote`, a full sha passed through, a
range through the source's tag list as `rev-ranges` specifies — and SHALL then perform every
step `graft sync` performs with the result: fetch each sha into
the content-addressed cache, read each source's `catalog.yaml`, expand its selectors, list each
installed item's files, build the plan, apply it, and write `graft.lock` last.

Re-resolving a range SHALL be the **only** way a range's matched tag moves. `graft sync` SHALL
NOT re-evaluate a range under any circumstance: a range in `graft.toml` beside a sha in
`graft.lock` is not drift, it is a resolved pin, and sync installs it exactly as it installs
any other. Without that, a range would make every sync non-reproducible and offline sync
impossible — which is the whole reason `sync` never re-resolves.

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

#### Scenario: `--to` can write a range into graft.toml

- **WHEN** `graft update --to "^1.2.0" shared` runs against a manifest pinning
  `rev     = "v1.0.0"`
- **THEN** `graft.toml` differs from the original in exactly one line, holding
  `rev     = "^1.2.0"` with its original alignment and every comment intact
- **AND** the run then resolves that range, selects the highest satisfying tag, and records it
  as `matched` in `graft.lock`

#### Scenario: `--to` can write a range containing a space

- **WHEN** `graft update --to ">=1.2.0 <2.0.0" shared` runs
- **THEN** the manifest holds `rev     = ">=1.2.0 <2.0.0"` — the space is inside a TOML string
  and needs no escaping, so the in-place editor writes it literally
- **AND** the value is refused only if it contains a quotation mark, a backslash, or a control
  character, exactly as any other rev is

#### Scenario: A new tag satisfying a range moves the pin

- **WHEN** `graft.lock` records source `shared` at `rev = "^1.2.0"` with `matched = "v1.2.0"`
  resolved to `<old sha>`, the source has since published `v1.3.0`, and `graft update` runs
- **THEN** `graft.lock` records `matched = "v1.3.0"` and `resolved` is `v1.3.0`'s sha
- **AND** `rev` is still `^1.2.0`, unchanged, because the consumer's request did not move
- **AND** `graft.toml` is byte-identical, because `update` without `--to` never writes it

#### Scenario: A new tag outside the range does not move the pin

- **WHEN** `graft.lock` records source `shared` at `rev = "^1.2.0"` with `matched = "v1.3.0"`,
  the source has since published `v2.0.0`, and `graft update` runs
- **THEN** the error stream holds exactly `up to date` and one newline
- **AND** `graft.lock` is byte-identical, because `v2.0.0` crosses a major the range excludes

#### Scenario: `graft sync` does not re-evaluate a range

- **WHEN** `graft.lock` records source `shared` at `rev = "^1.2.0"` with `matched = "v1.2.0"`,
  the source has since published `v1.3.0`, and `graft sync` runs
- **THEN** the sha installed is `v1.2.0`'s, the lock still records `matched = "v1.2.0"`, and
  the exit code is `0`
- **AND** no `git ls-remote --tags` is run, because sync resolves nothing

#### Scenario: A range that stops matching is an update failure, not a sync failure

- **WHEN** `graft.lock` records source `shared` at `rev = "^1.2.0"`, every tag satisfying it
  has been deleted from the source, and `graft update` runs
- **THEN** the run fails with
  `source "shared": rev "^1.2.0" matches none of the source's semver tags`
- **AND** the working tree is byte-identical and `graft.lock` is unwritten
- **AND** `graft sync` on the same repository still succeeds, because it re-resolves nothing
