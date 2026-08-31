# add-execution Specification

## Purpose
TBD - created by archiving change add-command. Update Purpose after archive.

## Requirements

### Requirement: `graft add <source> <selector...>` declares a source and syncs

`graft add`, given a source and at least one selector, SHALL amend `graft.toml` so that the
source is declared with those selectors and SHALL then perform every step `graft sync`
performs against the amended manifest: fetch, read each catalog, expand selectors, list each
item's files, build the plan, apply it, and write `graft.lock` last.

The amended manifest that the run resolves SHALL be the same bytes that reach disk. `add`
SHALL re-parse the bytes it produced and resolve from that parse, never from the manifest it
read plus an in-memory edit, so that a run cannot install one thing while `graft.toml`
describes another.

`add` SHALL be the only command permitted to create `graft.toml`. When the file does not
exist, `add` SHALL create it holding exactly the source's table; every other command SHALL
continue to fail with `graft.toml not found`.

The manifest SHALL reach disk through `internal/apply` like every other write, after the
planned writes and immediately before `graft.lock`, so a failure anywhere in the sequence
leaves `graft.toml` describing the pin the lock still records.

#### Scenario: A first source is added to a repository with no graft.toml

- **WHEN** the repository holds neither `graft.toml` nor `graft.lock`, the source repository
  publishes a catalog offering `agent:reviewer` at `.claude/agents/`, and
  `graft add optioni/shared@v1.0.0 agent:reviewer` runs
- **THEN** the exit code is `0` and `.claude/agents/reviewer.md` holds the source's bytes
- **AND** `graft.toml` exists and parses, declaring `[sources.shared]` with
  `git = "optioni/shared"`, `rev = "v1.0.0"`, and `install = ["agent:reviewer"]`
- **AND** `graft.lock` records source `shared` at that rev with the item's file listed

#### Scenario: A second source is added beside an existing one

- **WHEN** `graft.toml` already declares source `other`, and
  `graft add optioni/shared@v1.0.0 agent:reviewer` runs
- **THEN** the exit code is `0`, both sources are declared, and `other`'s block is
  byte-identical to what it was
- **AND** `other`'s sha in `graft.lock` is the one the lock already recorded — adding a
  source re-resolves no other source's pin

#### Scenario: Several selectors are written in the order given

- **WHEN** `graft add optioni/shared@v1.0.0 schema:tdd 'agent:*'` runs against a catalog
  offering both
- **THEN** `install` reads `["schema:tdd", "agent:*"]`, in that order
- **AND** every file both selectors expand to exists at its destination

#### Scenario: A selector matching nothing leaves graft.toml unwritten

- **WHEN** `graft add optioni/shared@v1.0.0 agent:nonexistent` runs against a catalog that
  offers no such item
- **THEN** the exit code is `1` and the failure is `internal/catalog`'s own message naming
  what the catalog does provide
- **AND** `graft.toml` does not exist, because expansion fails before anything is applied

### Requirement: A source's name is derived from its git value

`add` SHALL derive the name of the `[sources.<name>]` table from the git value: the final
path segment, after the last `/` or `:`, with a trailing `.git` removed and any trailing `/`
ignored. `optioni/shared`, `https://github.com/optioni/shared.git`, and
`git@github.com:optioni/shared` SHALL all derive `shared`.

The derived name SHALL match `^[A-Za-z0-9_-]+$` — a TOML bare key, and a dot excluded because
`[sources.my.repo]` is a different table than the source named `my.repo`. A name outside that
set SHALL be refused with `cannot derive a source name from "<git>"` rather than quoted into
the table: `graft.toml` is a human's file, and a name needing quotes is a name the consumer
should choose.

The git value itself SHALL be written verbatim, exactly as given on the command line. `add`
SHALL NOT expand shorthand to a URL, because `graft.toml` records the request and
`internal/source` owns the expansion.

#### Scenario: Three spellings of one repository derive one name

- **WHEN** `add` derives names for `optioni/shared`, `https://github.com/optioni/shared.git`,
  and `git@github.com:optioni/shared`
- **THEN** each derives `shared`
- **AND** the `git` value written for each is the string as given, unexpanded

#### Scenario: A git value with no usable last segment is refused

- **WHEN** `graft add 'optioni/sh ared' agent:reviewer` runs
- **THEN** the exit code is `1` and the failure is
  `cannot derive a source name from "optioni/sh ared"`
- **AND** nothing is fetched and `graft.toml` is byte-identical

### Requirement: `@rev` pins the source, and its absence resolves a default pin

`add` SHALL read a rev from the source argument when it carries one: the text after the last
`@`, where the text before that `@` contains a `/` or a `:`. That condition is what keeps
`git@github.com:optioni/shared` a git value rather than a repository named `git` pinned to a
rev. The rev SHALL be written into `graft.toml` verbatim and SHALL be anything a `rev` may
be, a semver range included.

An `@` with nothing after it SHALL be a usage error: `rev may not be empty`.

Without `@rev`, `add` SHALL resolve a default pin as `rev-resolution` specifies — the source's
highest semver tag, falling back to its default branch's name — and write that. The written
rev SHALL be a ref, never a range: a default pin is what the source offers today, and a range
is a policy only the consumer can choose.

#### Scenario: An explicit rev is written verbatim

- **WHEN** `graft add optioni/shared@^1.2.0 agent:reviewer` runs against a source publishing
  `v1.2.0` and `v1.3.0`
- **THEN** `graft.toml` holds `rev = "^1.2.0"`
- **AND** `graft.lock` records `matched = "v1.3.0"` and the sha of that tag, as a range pin
  does under every other command

#### Scenario: A source with tags gets its highest tag as the default pin

- **WHEN** the source publishes `v1.2.0`, `v1.3.0`, and `v2.0.0-rc.1`, and
  `graft add optioni/shared agent:reviewer` runs
- **THEN** `graft.toml` holds `rev = "v1.3.0"` — the highest stable tag, the release
  candidate excluded
- **AND** `graft.lock` records no `matched`, because the written rev is a ref

#### Scenario: A source with no tags gets its default branch

- **WHEN** the source publishes no tags at all and its default branch is `main`, and
  `graft add optioni/shared agent:reviewer` runs
- **THEN** `graft.toml` holds `rev = "main"`

#### Scenario: An empty rev is a usage error

- **WHEN** `graft add optioni/shared@ agent:reviewer` runs
- **THEN** the exit code is `1`, the failure is `rev may not be empty`, and the hint line
  follows it
- **AND** nothing is fetched and `graft.toml` is byte-identical

### Requirement: A source already declared is amended, never duplicated

When `graft.toml` already declares a source of the derived name, `add` SHALL amend that
source's `install` list rather than append a second table. Selectors already present SHALL
NOT be written twice, and selectors given twice on one command line SHALL be written once.
The existing order SHALL be preserved and new selectors appended after it.

An existing source whose `git` value differs from the one given SHALL be refused with
`graft.toml: source "<name>": already declared with git "<git>"`, naming the value the
manifest holds. Two different repositories cannot share a source name, and silently
retargeting a declared source would move every one of its files.

An invocation whose selectors are all already declared, against a source whose pin does not
move, SHALL leave `graft.toml` byte-identical and SHALL still sync.

#### Scenario: A new selector joins an existing source

- **WHEN** `graft.toml` declares `shared` with `install = ["agent:reviewer"]`, and
  `graft add optioni/shared schema:tdd` runs
- **THEN** `install` reads `["agent:reviewer", "schema:tdd"]` and no second `[sources.shared]`
  table exists
- **AND** the source's `rev` line is byte-identical: an `add` naming no rev moves no pin

#### Scenario: A selector already declared is not written twice

- **WHEN** `graft.toml` declares `shared` with `install = ["agent:reviewer"]` and
  `graft add optioni/shared agent:reviewer` runs
- **THEN** `graft.toml` is byte-identical, the report says `graft.toml: unchanged`, and the
  sync still runs
- **AND** the exit code is `0`

#### Scenario: The same selector given twice is written once

- **WHEN** `graft add optioni/shared@v1.0.0 agent:reviewer agent:reviewer` runs against a
  manifest that does not declare the source
- **THEN** `install` reads `["agent:reviewer"]`
- **AND** the manifest parses, which it would not with the duplicate

#### Scenario: A different repository under a taken name is refused

- **WHEN** `graft.toml` declares `shared` with `git = "optioni/shared"` and
  `graft add other/shared agent:reviewer` runs
- **THEN** the exit code is `1` and the failure is
  `graft.toml: source "shared": already declared with git "optioni/shared"`
- **AND** `graft.toml` is byte-identical and nothing is fetched

### Requirement: `add` moves a pin only when it names a different rev, and re-resolves that source alone

An `add` whose `@rev` differs from the rev `graft.toml` already holds for that source SHALL
move the pin in place, exactly as `graft update --to` does, and SHALL re-resolve that source
and no other. Every other source's sha SHALL still come from `graft.lock`.

An `add` naming no rev against a source already declared SHALL NOT move the pin and SHALL NOT
re-resolve it: a command that quietly bumped a pin whenever it added a selector would defeat
the promise that only an explicit act moves one.

The pin check that stops a run when `graft.toml` and `graft.lock` disagree SHALL apply to
every source this run does not re-resolve, exactly as it does under `update`.

#### Scenario: An explicit rev on a declared source moves the pin

- **WHEN** `graft.toml` and `graft.lock` agree that `shared` is pinned at `v1.0.0`, and
  `graft add optioni/shared@v2.0.0 schema:tdd` runs
- **THEN** `graft.toml` holds `rev = "v2.0.0"`, `graft.lock` records the sha `v2.0.0` names,
  and the report says the pin moved
- **AND** exactly one line of `graft.toml` differs from before, plus the amended `install`

#### Scenario: Adding a selector to a branch pin does not move it

- **WHEN** `graft.lock` records `shared` at `rev = "main"` resolved to an older sha, the
  branch has since advanced, and `graft add optioni/shared schema:tdd` runs
- **THEN** `graft.lock`'s `resolved` for `shared` is still the older sha
- **AND** the new selector's files are written from that same older sha

### Requirement: `add` without selectors is refused, naming what it needed

`graft add <source>` with no selector and without `--list` SHALL be an error naming what it
needed, on a terminal and off one alike:
`add requires at least one selector, or --list to see what the source offers`.

It SHALL NOT hang, SHALL NOT prompt, and SHALL NOT choose a default set. The interactive
picker is a later change; until it exists, this refusal is the whole behavior, and when the
picker arrives it narrows this refusal to the case where standard input is not a terminal.

Each selector SHALL be checked for `kind:name` syntax before anything is resolved, fetched,
or written, and an invalid one SHALL be a usage error:
`invalid selector "<selector>": want kind:name`.

#### Scenario: No selectors, no TTY

- **WHEN** `graft add optioni/shared` runs with neither stream a terminal
- **THEN** the exit code is `1`, the failure is
  `add requires at least one selector, or --list to see what the source offers`, and the hint
  line follows it
- **AND** nothing is fetched and `graft.toml` is byte-identical

#### Scenario: No selectors on a terminal is the same refusal

- **WHEN** `graft add optioni/shared` runs with both streams reported as terminals
- **THEN** the failure and the exit code are exactly those of the previous scenario

#### Scenario: A malformed selector is refused before the network

- **WHEN** `graft add optioni/shared reviewer` runs
- **THEN** the exit code is `1` and the failure is `invalid selector "reviewer": want kind:name`
- **AND** no `git` process is run and `graft.toml` is byte-identical

### Requirement: `graft add <source> --list` prints the catalog and writes nothing

`graft add <source> --list` SHALL resolve the source's rev exactly as an `add` would — the
`@rev` given, or the default pin — fetch that sha, read its `catalog.yaml`, and print to
**standard output** one line per item: the item's id and the destination its files would be
written to.

Items SHALL be printed in ascending id order, ids padded to a common width, so two runs
against one catalog print byte-identical text. An item a kind places at several destinations
SHALL name all of them on its line, separated by `, `. A destination that is a directory
SHALL be printed with a trailing `/`.

The destinations printed SHALL be the ones this consumer would get: when `graft.toml` already
declares the source, its `[sources.<name>.kinds]` overrides SHALL beat the catalog's, because
the destination is what a consumer actually agrees to. When the source is not declared, or
`graft.toml` does not exist, the catalog's own destinations SHALL be printed and the absent
manifest SHALL NOT be an error.

`--list` SHALL write nothing to the repository: no `graft.toml`, no `graft.lock`, no
destination file, and no directory. It fetches into the content-addressed cache, exactly as
`--dry-run` does, and the cache is not the working tree.

#### Scenario: A catalog is listed with its destinations

- **WHEN** a source's catalog offers `agent:reviewer` at `.claude/agents/` and `schema:tdd` as
  a directory at `openspec/schemas/`, and `graft add optioni/shared@v1.0.0 --list` runs
- **THEN** standard output holds a header naming the source and the rev, then
  `agent:reviewer  .claude/agents/reviewer.md` and `schema:tdd  openspec/schemas/tdd/`, ids
  aligned and sorted ascending
- **AND** the repository holds no `graft.toml`, no `graft.lock`, and no written file

#### Scenario: A consumer override is reflected in the listing

- **WHEN** `graft.toml` declares `shared` with `[sources.shared.kinds] agent = ".codex/agents/"`
  and `graft add optioni/shared --list` runs
- **THEN** the destination printed for `agent:reviewer` is `.codex/agents/reviewer.md`

#### Scenario: A source offering no items lists none

- **WHEN** the source's catalog declares kinds but provides no items, and
  `graft add optioni/shared@v1.0.0 --list` runs
- **THEN** the exit code is `0`, standard output holds the header and `(no items)`
- **AND** nothing is written to the repository

#### Scenario: An ungraftable source is refused under --list

- **WHEN** the source repository holds no `catalog.yaml` and
  `graft add optioni/shared@v1.0.0 --list` runs
- **THEN** the exit code is `1`, the failure is `internal/catalog`'s not-graftable message,
  and standard output is byte-empty

### Requirement: `--no-sync` writes the manifest and stops

`graft add <source> <selector...> --no-sync` SHALL amend `graft.toml` and SHALL then stop:
nothing is fetched for the plan, no file is written to a destination, nothing is pruned, and
`graft.lock` SHALL NOT be written or created.

Because nothing is fetched, `--no-sync` SHALL NOT verify that the selectors match anything the
source offers. That is the trade the flag makes, and it SHALL be the only way `add` writes a
selector it has not checked against a catalog.

With `@rev` given, `--no-sync` SHALL make no network call at all: the rev is known, and
nothing else needs resolving. Without `@rev` it SHALL resolve the default pin, which needs
one.

#### Scenario: The manifest is written and nothing else is

- **WHEN** the repository holds no `graft.toml`, and
  `graft add optioni/shared@v1.0.0 agent:reviewer --no-sync` runs
- **THEN** the exit code is `0`, `graft.toml` declares the source, and `graft.lock` does not
  exist
- **AND** `.claude/agents/` does not exist, and no `git` process was run

#### Scenario: An unverified selector is written

- **WHEN** `graft add optioni/shared@v1.0.0 agent:nonexistent --no-sync` runs
- **THEN** the exit code is `0` and `install` holds `agent:nonexistent`
- **AND** a subsequent `graft sync` fails with `internal/catalog`'s no-such-item message

### Requirement: `add` reports what it wrote to `graft.toml`

`add` SHALL print its manifest edit to the **error stream**, before the sync report, as one
line per kind of edit, in this order and with this wording:

- `graft.toml: added source "<name>" at <rev>` — a table was appended
- `graft.toml: moved source "<name>" to <rev>` — an existing pin moved
- `graft.toml: added <selector>[, <selector>...] to source "<name>"` — selectors were unioned
  into an existing list
- `graft.toml: unchanged` — the manifest already said what was asked for

A new source produces the first line only: its selectors are part of the table it names. The
sync report follows unchanged, so `add` adds a line and rewrites nothing about how a sync
reports itself.

#### Scenario: Adding a source reports one line

- **WHEN** `graft add optioni/shared@v1.0.0 agent:reviewer` succeeds against a repository with
  no manifest
- **THEN** the error stream holds `graft.toml: added source "shared" at v1.0.0` followed by
  the ordinary sync report
- **AND** standard output is byte-empty

#### Scenario: Moving a pin and adding a selector reports both, in order

- **WHEN** `graft add optioni/shared@v2.0.0 schema:tdd` succeeds against a manifest declaring
  `shared` at `v1.0.0` with `install = ["agent:reviewer"]`
- **THEN** the error stream holds `graft.toml: moved source "shared" to v2.0.0` and then
  `graft.toml: added schema:tdd to source "shared"`

#### Scenario: An invocation that changes nothing says so

- **WHEN** `graft add optioni/shared agent:reviewer` succeeds against a manifest that already
  declares exactly that
- **THEN** the error stream holds `graft.toml: unchanged`

### Requirement: `graft add` takes one source, any number of selectors, and exactly two flags

`graft add` SHALL accept one required positional source, zero or more selectors after it, and
the flags `--list` and `--no-sync` and no others. `--dry-run` SHALL NOT be one of them: `add`
without `--no-sync` writes, and `--list` already is the read-only form.

Each of the following SHALL be a usage error, reported with the `graft: ` prefix and followed
by the hint line, before anything is resolved or fetched:

- no source at all: `add requires a source`
- an empty source: `source may not be empty`
- `--list` given with selectors: `--list takes no selectors`
- `--list` given with `--no-sync`: `--list and --no-sync cannot be combined`

#### Scenario: No arguments

- **WHEN** `graft add` runs
- **THEN** the exit code is `1`, the failure is `add requires a source`, and the hint line
  follows it

#### Scenario: An empty source argument

- **WHEN** `graft add ""` runs, as an unset shell variable produces
- **THEN** the exit code is `1` and the failure is `source may not be empty`
- **AND** nothing is fetched

#### Scenario: --list with selectors

- **WHEN** `graft add optioni/shared --list agent:reviewer` runs
- **THEN** the exit code is `1` and the failure is `--list takes no selectors`

#### Scenario: --list with --no-sync

- **WHEN** `graft add optioni/shared --list --no-sync` runs
- **THEN** the exit code is `1` and the failure is `--list and --no-sync cannot be combined`

#### Scenario: An unknown flag

- **WHEN** `graft add optioni/shared --dry-run agent:reviewer` runs
- **THEN** the exit code is `1`, the failure names the unknown flag, and the hint line follows

### Requirement: Every failure leaves `graft.toml` and the working tree untouched

No failure in `add` SHALL leave a partially written `graft.toml`. Name derivation, selector
syntax, the manifest amendment, rev resolution, fetching, catalog reading, expansion, listing,
and planning SHALL all happen before the first byte reaches the repository, and any failure
among them SHALL leave `graft.toml` byte-identical — absent if it was absent — and
`graft.lock` and every destination unchanged.

A manifest amendment that cannot be performed exactly SHALL be refused with
`internal/manifest`'s own wording, unaltered, and SHALL NOT be approximated.

The amended bytes SHALL be re-parsed and checked before they are used: the source SHALL read
back with the git, the rev, and every selector that was asked for. A text edit that parsed but
landed on the wrong line SHALL fail the run with
`graft.toml: source "<name>": the amendment did not take effect` rather than reach disk.

#### Scenario: An unreachable source leaves no manifest behind

- **WHEN** the repository holds no `graft.toml` and `graft add optioni/shared agent:reviewer`
  runs against a git value no remote answers for
- **THEN** the exit code is `1`, the failure names the URL it could not reach
- **AND** `graft.toml` still does not exist

#### Scenario: An unparsable graft.toml is refused before anything is resolved

- **WHEN** `graft.toml` exists holding a key no source may have, and
  `graft add optioni/shared@v1.0.0 agent:reviewer` runs
- **THEN** the exit code is `1` and the failure is `internal/manifest`'s own unknown-key
  message, unaltered
- **AND** `graft.toml` is byte-identical and nothing is fetched

#### Scenario: An unamendable manifest is refused in the amender's words

- **WHEN** `graft.toml` declares its sources as an inline table —
  `sources = { shared = { git = "…", rev = "…", install = ["agent:reviewer"] } }`, which
  parses and which no `[sources.shared]` header covers — and
  `graft add optioni/shared schema:tdd` runs
- **THEN** the exit code is `1` and the failure is `internal/manifest`'s refusal, naming the
  source and the key
- **AND** `graft.toml` is byte-identical and nothing is fetched
