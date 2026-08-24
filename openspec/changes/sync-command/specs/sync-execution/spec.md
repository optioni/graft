## ADDED Requirements

### Requirement: `graft sync` makes the tree match the lock, in one fixed order

`graft sync` SHALL perform SPEC.md's resolution sequence in this order, against the
repository rooted at the process's working directory:

1. Load `graft.toml`, and `graft.lock` if it exists.
2. Refuse a manifest and lock that disagree on any pin, **before** anything is fetched.
3. For each source in the manifest, take its sha from the lock; resolve it only if the lock
   has no entry for that source.
4. Fetch that sha into the content-addressed cache, reusing an existing entry.
5. Read `catalog.yaml` from the fetched tree.
6. Expand the source's selectors and list each installed item's files from the fetched tree.
7. Build the plan.
8. Apply it.

Steps 1 through 7 SHALL create, modify, or delete nothing in the repository. A failure at any
of them SHALL leave the working tree exactly as it was — no file written, no file deleted, no
directory created, and no `graft.lock` — so a validation failure never leaves partial state.

The orchestration SHALL take the repository root and the fetch-cache root as values rather
than reading a global, so that a test names its own and cannot reach the developer's real
cache. `graft sync` SHALL pass the working directory and `internal/source`'s default cache
root.

A source in `graft.lock` that `graft.toml` no longer declares SHALL NOT be resolved and SHALL
NOT be fetched. Its files are pruned from the lock alone, which is the only record of them
that exists.

#### Scenario: A first sync installs what the manifest asks for

- **WHEN** `graft.toml` declares source `shared` at `rev = "v1.0.0"` installing `schema:tdd`
  and `agent:*`, no `graft.lock` exists, and the source repository provides those items
- **THEN** every file each item places exists at its destination with the source's bytes
- **AND** `graft.lock` records the source's `git` and `rev` as written, the sha `v1.0.0`
  resolved to, and each item's files sorted by path
- **AND** the exit code is `0`

#### Scenario: A second sync changes nothing

- **WHEN** `graft sync` is run twice in a row against an unchanged manifest and source
- **THEN** the second run exits `0` and `graft.lock` is byte-identical after both runs
- **AND** every installed file is byte-identical after both runs, and no file was deleted

#### Scenario: A missing manifest is refused before anything else happens

- **WHEN** `graft sync` runs in a directory holding no `graft.toml`
- **THEN** the error stream holds `graft: graft.toml not found` and the exit code is `1`
- **AND** no `graft.lock` is created, no directory is created, and nothing is fetched

#### Scenario: A source dropped from the manifest is pruned without being fetched

- **WHEN** `graft.lock` records source `retired` with two files and `graft.toml` no longer
  declares it
- **THEN** both files are deleted and `graft.lock` no longer holds that source
- **AND** no network access and no cache lookup is attempted for `retired`, because its rev
  is no longer declared anywhere

### Requirement: sync installs the lock's pin and never re-resolves it

`graft sync` SHALL take each source's resolved sha from `graft.lock` whenever the lock holds
an entry for that source, and SHALL NOT contact the remote to re-resolve it. This holds for
every kind of `rev`, a branch name included: `rev = "main"` SHALL install the sha the lock
records however far the branch has since moved. Moving a pin is `graft update`, always
explicit.

There SHALL be no `--force` flag and no `--frozen` flag. Sync always overwrites, and sync is
always frozen; a flag that made it do its job would be the bug it exists to prevent.

A source the lock has no entry for SHALL be resolved exactly once — a tag or branch through
`git ls-remote`, a full sha passed through — and the result recorded in the lock it writes.

#### Scenario: A moved branch does not move the pin

- **WHEN** `graft.lock` records source `shared` at `rev = "main"` resolved to a sha, the
  branch `main` in the source repository has since advanced with a new file, and `graft sync`
  runs
- **THEN** the tree holds the files of the recorded sha and not the new file
- **AND** `graft.lock`'s `resolved` is unchanged, and `git ls-remote` is not run

#### Scenario: A source with no lock entry is resolved once and recorded

- **WHEN** `graft.toml` declares a second source `extra` at `rev = "v2.0.0"` and `graft.lock`
  holds only `shared`
- **THEN** `extra` is resolved to the sha its tag names, fetched, and installed
- **AND** `graft.lock` afterwards records both sources, ordered by name, and `shared`'s
  `resolved` is unchanged

#### Scenario: A rev that no longer exists fails, naming the rev and the source

- **WHEN** a source with no lock entry declares `rev = "v9.9.9"` and the source repository has
  no such tag or branch
- **THEN** the error stream holds `graft: source "shared": rev "v9.9.9" not found` and the
  exit code is `1`
- **AND** no file is written, no `graft.lock` is created, and no other source in the manifest
  is installed either

#### Scenario: There is no flag to make sync re-resolve or refuse to overwrite

- **WHEN** `graft sync --force` is invoked, and separately `graft sync --frozen`
- **THEN** each is refused as an unknown flag with the exit code `1`
- **AND** the hint line `run "graft --help" for usage` follows each, because asking graft for
  something it does not have is a usage error

### Requirement: A manifest and lock that disagree on a pin stop the run before anything is fetched

When `graft.toml` and `graft.lock` both name a source and give it different revs, `graft sync`
SHALL fail with

```
graft.toml has rev "<manifest>" for source "<source>" but graft.lock has "<lock>"; run `graft update` to move the pin
```

and SHALL exit `1`. The check SHALL run before any resolution, any fetch, and any read of a
catalog, so a manifest that moved cannot cause network access, let alone a write.

A source named by only one of the two files SHALL NOT be a disagreement: one with no lock
entry is resolved on this run, and one no longer in the manifest has its files pruned.

#### Scenario: A bumped rev in the manifest points at graft update

- **WHEN** `graft.toml` says `rev = "v1.3.0"` for source `shared` and `graft.lock` says
  `v1.2.0`
- **THEN** the error stream holds

  ```
  graft: graft.toml has rev "v1.3.0" for source "shared" but graft.lock has "v1.2.0"; run `graft update` to move the pin
  ```

- **AND** the exit code is `1`, nothing was fetched, and every installed file and `graft.lock`
  are unchanged

#### Scenario: The pin check precedes the network

- **WHEN** the pins disagree and the source repository is unreachable
- **THEN** the failure is the pin disagreement, not a fetch failure
- **AND** the cache holds no new entry

### Requirement: Every planning failure leaves the working tree untouched

`graft sync` SHALL surface the error from whichever step failed, unaltered, through graft's
one-line error format, and SHALL exit `1`. Because nothing is written before the plan is
built, a failure in loading, resolution, fetching, catalog reading, listing, or planning SHALL
leave every file in the repository byte-identical to what it was and SHALL NOT create or
modify `graft.lock`.

Each of SPEC.md's failure modes that a sync can reach SHALL be covered: a `rev` not found, a
missing or invalid `catalog.yaml`, a selector matching no item, two items resolving to one
path, a destination outside the repository root, a source's listing climbing out of its item,
a cache miss with no network, and a manifest and lock disagreeing on a pin.

#### Scenario: A source without a catalog is not graftable

- **WHEN** a source's fetched tree holds no `catalog.yaml`
- **THEN** the error stream holds `graft: catalog.yaml not found: the source is not graftable`
  and the exit code is `1`
- **AND** the working tree is unchanged and no `graft.lock` is written

#### Scenario: A selector matching nothing fails the run

- **WHEN** `graft.toml` installs `schema:tdd-workflwo` from a source whose catalog provides
  `agent:apply-orchestrator` and `schema:tdd`
- **THEN** the error stream holds
  `graft: source "shared": selector "schema:tdd-workflwo" matches no item; catalog provides agent:apply-orchestrator, schema:tdd`
- **AND** the exit code is `1` and nothing was written, so a misspelling never silently
  installs nothing

#### Scenario: Two items resolving to one path fail the run before any of it is written

- **WHEN** two sources both place an item at `.claude/agents/x.md`
- **THEN** the error stream holds the collision message naming both sources, both items, and
  the path, and the exit code is `1`
- **AND** `.claude/agents/x.md` does not exist afterwards, because the plan that failed
  produced no writes at all

#### Scenario: A cache miss with no network names what it needed

- **WHEN** the lock pins a sha the cache does not hold and the source repository cannot be
  reached
- **THEN** the run fails naming the sha and the remote, with the exit code `1`
- **AND** the working tree and `graft.lock` are unchanged

#### Scenario: A cache hit with no network succeeds

- **WHEN** the lock pins a sha the cache already holds and the source repository has been
  made unreachable
- **THEN** the sync succeeds and installs the cached tree, with the exit code `0`
- **AND** no git subprocess contacted the remote

#### Scenario: An invalid catalog fails the run

- **WHEN** a source's fetched tree holds a `catalog.yaml` that is not valid — a YAML syntax
  error, and separately a `provides` entry naming a kind the catalog does not declare
- **THEN** each fails with `internal/catalog`'s own message, prefixed `graft: catalog.yaml: `,
  and the exit code is `1`
- **AND** the working tree is unchanged and no `graft.lock` is written, so an invalid catalog
  is as inert as a missing one

#### Scenario: A destination outside the repository root fails the run

- **WHEN** a source's catalog declares `to: "../outside/{name}"` for a kind the manifest
  installs
- **THEN** the run fails with
  `graft: source "shared": item "schema:tdd": destination "../outside/tdd" escapes the repo root`
  and the exit code is `1`
- **AND** no file exists at or beside `../outside/`, and this is the path a real `plan.Build`
  refuses, not a hand-built plan reaching `internal/apply`

#### Scenario: A source's listing climbing out of its item fails the run

- **WHEN** a source's `from` names a directory that is a committed symlink pointing out of the
  fetched tree
- **THEN** the run fails with
  `graft: source "shared": item "schema:tdd": from "extras/link" is not a regular file or directory`
  and the exit code is `1`
- **AND** nothing outside the source's tree was read and nothing in the repository was written

### Requirement: `--dry-run` prints the plan and touches nothing

`graft sync --dry-run` SHALL perform every step through building the plan, print the report
the sync would have produced, exit `0`, and create, modify, or delete nothing. No file SHALL
be written, no file deleted, **no directory created**, and `graft.lock` SHALL NOT be written —
not even when the repository has never been synced.

A source the lock has never seen SHALL still be resolved and fetched under `--dry-run`,
because there is no plan without a catalog and no catalog without a fetch. The fetch cache is
not the working tree.

`--dry-run` SHALL be the only flag `graft sync` accepts.

A dry run stops after the plan is built, so it exercises none of `internal/apply`'s refusals —
a destination that is a directory, a prune path that is a symlink, an ancestor that is not a
directory, a reserved path. A clean dry run therefore says the plan is valid, not that the
sync will succeed, and the specification says so rather than letting a reader infer the
stronger claim.

#### Scenario: A dry run of a first sync creates nothing

- **WHEN** `graft sync --dry-run` runs in a repository with a manifest, no lock, and nothing
  installed
- **THEN** the report names every item that would be added and the exit code is `0`
- **AND** no destination directory exists, no destination file exists, and `graft.lock` does
  not exist

#### Scenario: A dry run of a removal deletes nothing

- **WHEN** an item is dropped from `install` and `graft sync --dry-run` runs
- **THEN** the report names the item as removed with its file count and the exit code is `0`
- **AND** every file that item placed is still on disk and `graft.lock` still records it

#### Scenario: A dry run of a failing plan fails the same way

- **WHEN** `graft sync --dry-run` runs against a manifest whose selector matches no item
- **THEN** the error is the same selector error, on the same stream, with the exit code `1`
- **AND** nothing is written, which is also true without the flag

### Requirement: `graft sync` takes no arguments and writes nothing to standard output

`graft sync` SHALL accept no positional argument. An argument SHALL be refused as a usage
error reading `unknown argument "<argument>"`, followed by the hint line, with the exit code
`1`. An unrecognised flag SHALL be refused as a usage error in the same way.

`graft sync` SHALL write nothing to the **standard output** stream on any path. Its report is
a summary and its errors are errors, and SPEC.md sends both to the **error** stream so a pipe
is never corrupted by text a human was meant to read.

`graft sync` SHALL exit `0` when the sync completes and `1` on any failure. There is no third
outcome and no partial success: a run that could not finish reports the reason and leaves the
tree as it found it.

#### Scenario: An argument to sync is a usage error

- **WHEN** `graft sync shared` is invoked
- **THEN** the error stream holds `graft: unknown argument "shared"` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty and the exit code is `1`

#### Scenario: A successful sync leaves standard output byte-empty

- **WHEN** `graft sync` installs two items successfully
- **THEN** the standard output stream is byte-empty
- **AND** the report appears on the error stream and the exit code is `0`

#### Scenario: A failing sync leaves standard output byte-empty

- **WHEN** `graft sync` fails because `graft.toml` is missing
- **THEN** the standard output stream is byte-empty
- **AND** the error stream holds exactly the one-line report and the exit code is `1`
