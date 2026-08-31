## ADDED Requirements

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
