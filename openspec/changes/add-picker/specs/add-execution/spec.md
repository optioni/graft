## REMOVED Requirements

### Requirement: `add` without selectors is refused, naming what it needed

**Reason**: The refusal was written as the whole behavior on a terminal and off one alike,
with its own text saying the picker would narrow it. It is now half of a requirement whose
other half is the picker, and one of its scenarios — *No selectors on a terminal is the same
refusal* — asserts the opposite of what this change makes true. Replaced rather than edited,
because a MODIFIED block may not drop a scenario, and keeping a scenario name whose content
now says the reverse is worse than retiring the name.

**Migration**: None for a user: the non-interactive wording, the exit code, and the
selector-syntax check are unchanged, and are carried into the requirement below verbatim.

## ADDED Requirements

### Requirement: `add` without selectors opens the picker on a terminal, and is refused off one

`graft add <source>` with no selector and without `--list` SHALL present the source's
catalog as a multi-select list when — and only when — both standard input and the error
stream are terminals. The selectors it returns SHALL enter the sequence exactly where a
command line's would: the same amendment, the same checks, the same sync. `add` SHALL hold
no behavior that only the picker can reach.

Off a terminal it SHALL be an error naming what it needed, with the wording unchanged:
`add requires at least one selector, or --list to see what the source offers`.

It SHALL NOT hang, SHALL NOT prompt without a terminal, and SHALL NOT choose a default set.

The picker SHALL be reached only after the source's rev is resolved, its tree fetched, and
its catalog read, because the list it shows is that catalog — so a source that cannot be
reached, or is not graftable, SHALL fail with that failure rather than with an empty list.
Nothing SHALL be written before the selection is made: a cancelled picker leaves the
repository byte-identical, `graft.toml` included, and exits `1` with `add cancelled`.

Each selector given on the command line SHALL be checked for `kind:name` syntax before
anything is resolved, fetched, or written, and an invalid one SHALL be a usage error:
`invalid selector "<selector>": want kind:name`. Selectors the picker returns need no such
check: it offers only ids the catalog declares, and a glob it forms itself.

#### Scenario: No selectors, no TTY

- **WHEN** `graft add optioni/shared` runs with neither stream a terminal
- **THEN** the exit code is `1`, the failure is
  `add requires at least one selector, or --list to see what the source offers`, and the hint
  line follows it
- **AND** nothing is fetched and `graft.toml` is byte-identical

#### Scenario: No selectors on a terminal opens the picker

- **WHEN** `graft add optioni/shared` runs with standard input and the error stream both
  terminals, and the user selects `agent:reviewer` and confirms
- **THEN** `graft.toml` declares the source with `install = ["agent:reviewer"]` and the sync
  installs it
- **AND** the manifest is byte-identical to the one `graft add optioni/shared agent:reviewer`
  would have written

#### Scenario: A cancelled picker writes nothing

- **WHEN** the same invocation runs and the user cancels
- **THEN** the exit code is `1`, the failure is `add cancelled`, and `graft.toml` does not
  exist
- **AND** no file was written to any destination and `graft.lock` was not created

#### Scenario: A malformed selector is refused before the network

- **WHEN** `graft add optioni/shared reviewer` runs
- **THEN** the exit code is `1` and the failure is `invalid selector "reviewer": want kind:name`
- **AND** no `git` process is run and `graft.toml` is byte-identical

#### Scenario: An ungraftable source fails before any list is shown

- **WHEN** `graft add optioni/shared` runs on a terminal against a source with no
  `catalog.yaml`
- **THEN** the exit code is `1` and the failure is `internal/catalog`'s not-graftable message
- **AND** nothing is drawn on the error stream beyond that failure
