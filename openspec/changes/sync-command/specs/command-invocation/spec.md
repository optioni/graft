## MODIFIED Requirements

### Requirement: An invocation that asks for nothing prints help

`graft` invoked with no arguments SHALL print its help — the summary, the usage line, the
commands it has, and its flags — to the **standard output** stream and exit `0`. `graft
--help` SHALL print byte-identical text to the same stream with the same exit code. Help is
a thing the user asked for, not a diagnostic, so it never goes to the error stream.

Now that `graft` has a subcommand, its help SHALL list that subcommand, so the commands
section is no longer empty.

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
- **THEN** the standard output stream names `sync` and describes it
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
