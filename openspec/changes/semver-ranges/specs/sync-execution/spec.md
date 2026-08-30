## MODIFIED Requirements

### Requirement: sync installs the lock's pin and never re-resolves it

`graft sync` SHALL take each source's resolved sha from `graft.lock` whenever the lock holds
an entry for that source, and SHALL NOT contact the remote to re-resolve it. This holds for
every kind of `rev`, a branch name and a **range** included: `rev = "main"` SHALL install the
sha the lock records however far the branch has since moved, and `rev = "^1.2.0"` SHALL
install the sha the lock records however many newer tags satisfy it. Moving a pin is
`graft update`, always explicit.

There SHALL be no `--force` flag and no `--frozen` flag. Sync always overwrites, and sync is
always frozen; a flag that made it do its job would be the bug it exists to prevent.

A source the lock has no entry for SHALL be resolved exactly once — a tag or branch through
`git ls-remote`, a full sha passed through, a range through the source's tag list — and the
result recorded in the lock it writes, the matched tag included.

That first resolution is not a re-evaluation, and the distinction is the whole rule: `sync`
resolves what the lock does not know and never re-resolves what it does. A source added to
`graft.toml` by hand must be installable without reaching for `update`, and a range is the
same case as a tag. Once its entry exists, no later `sync` consults the tag list again.

#### Scenario: A moved branch does not move the pin

- **WHEN** `graft.lock` records source `shared` at `rev = "main"` resolved to a sha, the
  branch `main` in the source repository has since advanced with a new file, and `graft sync`
  runs
- **THEN** the tree holds the files of the recorded sha and not the new file
- **AND** `graft.lock`'s `resolved` is unchanged, and `git ls-remote` is not run

#### Scenario: A newer tag satisfying a range does not move the pin

- **WHEN** `graft.lock` records source `shared` at `rev = "^1.2.0"` with `matched = "v1.2.0"`,
  the source has since published `v1.3.0`, and `graft sync` runs
- **THEN** the tree holds `v1.2.0`'s files and `graft.lock` is byte-identical
- **AND** no `git ls-remote --tags` is run, which is what makes a sync reproducible offline

#### Scenario: A source with no lock entry is resolved once and recorded

- **WHEN** `graft.toml` declares a second source `extra` at `rev = "v2.0.0"` and `graft.lock`
  holds only `shared`
- **THEN** `extra` is resolved to the sha its tag names, fetched, and installed
- **AND** `graft.lock` afterwards records both sources, ordered by name, and `shared`'s
  `resolved` is unchanged

#### Scenario: A range source with no lock entry is resolved once and recorded

- **WHEN** `graft.toml` declares a second source `extra` at `rev = "^2.0.0"`, the source
  publishes `v2.0.0` and `v2.1.0`, and `graft.lock` holds only `shared`
- **THEN** `extra` is resolved by listing tags, selects `v2.1.0`, and is fetched and installed
- **AND** `graft.lock` afterwards records `rev = "^2.0.0"` and `matched = "v2.1.0"` for
  `extra`, and `shared`'s entry is untouched
- **AND** this is the one path on which `sync` lists tags, because the lock had no answer to
  install

#### Scenario: A range with no lock entry that no tag satisfies fails the run

- **WHEN** `graft.toml` declares a source `extra` at `rev = "^9.0.0"`, the source publishes
  only `v1.0.0`, and `graft.lock` holds no entry for it
- **THEN** the error stream holds
  `graft: source "extra": rev "^9.0.0" matches none of the source's semver tags` and the exit
  code is `1`
- **AND** no file is written, no `graft.lock` is created, and no other source is installed

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
