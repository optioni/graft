# Command Invocation Specification

## Purpose

`graft` is one command. This capability covers what that means as a program: how it is
built and run, what `--version` and help do, how an argument or flag it does not recognise
is refused, and what exit code every outcome produces.

The shape is not incidental. Coverage is measured over `./internal/...` only, and
`task ci` runs `go test` over nothing else — so a decision made in `cmd/graft` is neither
measured nor executed by CI. Everything below is therefore reachable from a package under
`./internal/`, and `cmd/graft` holds one call and one exit.

SPEC.md admits two exit codes, `0` and `1`, and this capability adds none. A failure to
write the output a command produced counts as a failure of the run, help included:
reporting success while the thing that was asked for went nowhere is the one outcome a
caller cannot detect for itself.

## Requirements

### Requirement: graft is one command whose behavior is decided by its arguments

`graft` SHALL be a single root command built on cobra, taking its arguments, its two output
streams, its environment reader, and the linker-injected build strings from its caller.
Building and running the command SHALL be reachable from a package under `./internal/`, so
that every decision the surface makes is visible to the coverage gate, and `cmd/graft` SHALL
hold nothing but that call and the exit it produces.

The root command SHALL print no banner, no progress, and no confirmation on a successful run
that had nothing to report. Output that appears when nothing happened trains the reader to
stop reading it.

#### Scenario: A successful invocation exits 0

- **WHEN** `graft --version` is invoked with the build strings `v1.2.0`, `abc1234`, and
  `2026-08-23`
- **THEN** the process exits `0`
- **AND** nothing is written to the error stream

#### Scenario: The process's own arguments are never read

- **WHEN** the command surface is invoked with an argument list of `nil` while the process
  was started with the argument `frobnicate`
- **THEN** it prints help and exits `0`, because the caller passed no arguments
- **AND** `frobnicate` appears nowhere in either stream: cobra falls back to `os.Args[1:]`
  whenever the slice it is handed is nil, and that fallback is closed

#### Scenario: cmd/graft holds no decision of its own

- **WHEN** the import set of `./cmd/graft` is enumerated
- **THEN** it is exactly `github.com/optioni/graft/internal/cli` and `os`, in that order
- **AND** `github.com/optioni/graft/internal/buildinfo` is **not** among them: the build
  strings are carried to the command surface as strings, and `buildinfo.Format` is called
  from `internal/cli`, where the coverage gate can see it — an import of `buildinfo` in
  `cmd/graft` is itself the failure this scenario exists to catch
- **AND** exactly one package exists under `./cmd/`, so nothing outside `./internal/...` can
  hold a decision the coverage gate cannot see

### Requirement: `--version` prints the build line to stdout

The root command SHALL accept `--version` and print the single line produced by
`internal/buildinfo` — the version, the commit, and the build date — to the **standard
output** stream, then exit `0`. A binary built without linker flags SHALL still print a
line, with each unset field rendered as `unknown`, rather than printing nothing.

`--version` SHALL be the only spelling. A bare `version` argument SHALL NOT be accepted,
because SPEC.md's command table does not name a `version` command, and a second spelling is
a second thing to keep working.

#### Scenario: The version line goes to stdout

- **WHEN** `graft --version` is invoked with the build strings `v1.2.0`, `abc1234`, and
  `2026-08-23`
- **THEN** the standard output stream holds exactly `graft v1.2.0 (abc1234, built 2026-08-23)`
  followed by one newline
- **AND** the error stream is empty
- **AND** the exit code is `0`

#### Scenario: An unbuilt binary still prints a version line

- **WHEN** `graft --version` is invoked with all three build strings empty
- **THEN** the standard output stream holds `graft unknown (unknown, built unknown)`
- **AND** the exit code is `0`

#### Scenario: `version` is not a command

- **WHEN** `graft version` is invoked
- **THEN** the error stream holds `graft: unknown command "version"`
- **AND** the standard output stream is empty
- **AND** the exit code is `1`

### Requirement: An invocation that asks for nothing prints help

`graft` invoked with no arguments SHALL print its help — the summary, the usage line, the
commands it has, and its flags — to the **standard output** stream and exit `0`. `graft
--help` SHALL print byte-identical text to the same stream with the same exit code. Help is
a thing the user asked for, not a diagnostic, so it never goes to the error stream.

The commands section SHALL list **every** subcommand `graft` has. It listed one when `sync`
was the only one; it lists `sync` and `update` now, and a command added later is listed by the
same rule rather than by an amendment here. What it SHALL NOT list is a command SPEC.md's
command table does not name.

`--help` SHALL be the only spelling. A bare `help` **argument** SHALL NOT be accepted, and
SHALL be refused as `unknown command "help"` like any other unrecognised argument. This
closes the transition this requirement previously left open: cobra installs a built-in `help`
command as soon as a subcommand is registered, and SPEC.md's command table names no `help`
command. It is the same trade the `--version` requirement already makes — one spelling, and
a second one is a second thing to keep working — and the same trade the completion command
gets, for the same reason.

Registering a subcommand SHALL NOT change how an unrecognised argument is refused: the root
command's own argument validator SHALL still produce `unknown command "<argument>"`, so the
wording stays graft's contract rather than becoming a detail of how cobra resolves a
subcommand name.

#### Scenario: No arguments prints help and succeeds

- **WHEN** `graft` is invoked with no arguments
- **THEN** the standard output stream holds text naming `graft`, containing a `Usage:`
  section, and listing both `--version` and `--help`
- **AND** the error stream is empty
- **AND** the exit code is `0`

#### Scenario: Help lists the commands graft has

- **WHEN** `graft --help` is invoked
- **THEN** the standard output stream names `sync` and describes it, and names `update` and
  describes it
- **AND** it names no `help` command and no `completion` command

#### Scenario: `--help` prints the same text as no arguments at all

- **WHEN** `graft --help` is invoked
- **THEN** it writes to the standard output stream text byte-identical to what `graft` with
  no arguments writes
- **AND** the error stream is empty and the exit code is `0`

#### Scenario: `help` is not a command

- **WHEN** `graft help` is invoked
- **THEN** the error stream holds `graft: unknown command "help"` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty and the exit code is `1`

#### Scenario: `help sync` is not a command either

- **WHEN** `graft help sync` is invoked
- **THEN** the error stream holds `graft: unknown command "help"`, naming the first
  unrecognised argument only
- **AND** the standard output stream is empty and the exit code is `1`

#### Scenario: A subcommand's own help goes to standard output

- **WHEN** `graft update --help` is invoked
- **THEN** the standard output stream holds text naming `update`, its `--to` flag, and its
  `--dry-run` flag
- **AND** the error stream is empty and the exit code is `0`

### Requirement: An unknown command or flag is refused as a usage error

An argument the root command does not recognise SHALL produce the error
`unknown command "<argument>"`, naming the first unrecognised argument. An unrecognised flag
SHALL produce cobra's own `unknown flag: <flag>`. Both SHALL be reported through graft's
error format, SHALL be followed by the line `run "graft --help" for usage`, and SHALL exit
`1`. Neither SHALL print the full usage text: a wall of usage buries the sentence that says
what went wrong.

The unrecognised-argument check SHALL be the root command's own argument validator rather
than a match against cobra's message text, so the wording is graft's contract and not a
detail of a dependency.

#### Scenario: An unknown command names the argument

- **WHEN** `graft frobnicate` is invoked
- **THEN** the error stream holds `graft: unknown command "frobnicate"` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty
- **AND** the exit code is `1`

#### Scenario: An unknown flag is a usage error too

- **WHEN** `graft --nope` is invoked
- **THEN** the error stream holds `graft: unknown flag: --nope` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty
- **AND** the exit code is `1`

#### Scenario: Only the first unrecognised argument is named

- **WHEN** `graft frobnicate wibble` is invoked
- **THEN** the error stream names `frobnicate` and does not name `wibble`
- **AND** the exit code is `1`

#### Scenario: No completion command is offered

- **WHEN** `graft completion` is invoked
- **THEN** the error stream holds `graft: unknown command "completion"`
- **AND** the standard output stream is empty and the exit code is `1`

#### Scenario: Cobra's hidden completion protocol is refused too

- **WHEN** `graft __complete ''` is invoked, and separately `graft __completeNoDesc ''`
- **THEN** the error stream holds `graft: unknown command "__complete"` and
  `graft: unknown command "__completeNoDesc"` respectively
- **AND** the standard output stream is empty and the exit code is `1`
- **AND** neither writes cobra's `:0` completion directive anywhere, because that output
  reaches the standard output stream without passing through graft's own surface

#### Scenario: An unknown shorthand flag is a usage error

- **WHEN** `graft -v` is invoked
- **THEN** the error stream holds `graft: unknown shorthand flag: 'v' in -v` followed by
  `run "graft --help" for usage`
- **AND** the standard output stream is empty and the exit code is `1`

### Requirement: The exit code is 0 for success and 1 for every failure

The command surface SHALL return `0` when the invoked command returns no error and every
write to its output streams succeeded, and `1` otherwise. It SHALL define no third code:
SPEC.md admits `0` success and `1` error, and a code invented for a class of failure is a
contract a caller will come to depend on.

A failure to **write** the output a command produced SHALL be a failure of the run, reported
as `graft: cannot write output: ` followed by the underlying error. Reporting success while
the thing that was asked for went nowhere is the one outcome a caller cannot detect for
itself.

This SHALL hold for **every** byte graft emits, including help — which cobra renders itself
and whose write error cobra's `Help()` discards. Both streams handed to cobra SHALL
therefore be writers graft owns and records, so that no output path can fail silently by
going around the output surface.

#### Scenario: A command that returns an error exits 1

- **WHEN** a registered command returns the error
  `source "shared": rev "v9.9.9" not found`
- **THEN** the error stream holds `graft: source "shared": rev "v9.9.9" not found`
- **AND** no `run "graft --help" for usage` line is written, because this is not a usage
  error
- **AND** the standard output stream is empty
- **AND** the exit code is `1`

#### Scenario: A command that succeeds exits 0

- **WHEN** a registered command writes `hello` to the machine-readable stream and returns no
  error
- **THEN** the standard output stream holds `hello` followed by one newline
- **AND** the error stream is empty
- **AND** the exit code is `0`

#### Scenario: Output that cannot be written is not a silent success

- **WHEN** `graft --version` is invoked with a standard output stream whose every write
  fails with `disk full`
- **THEN** the error stream holds `graft: cannot write output: disk full`
- **AND** the exit code is `1`

#### Scenario: Help that cannot be written is not a silent success either

- **WHEN** `graft --help` is invoked with a standard output stream whose every write fails
  with `disk full`, and separately `graft` with no arguments against the same stream
- **THEN** both write `graft: cannot write output: disk full` to the error stream
- **AND** both exit `1`, rather than exiting `0` because cobra's help renderer discarded the
  failure
