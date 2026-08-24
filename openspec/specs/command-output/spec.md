# Command Output Specification

## Purpose

Every byte a user of `graft` sees comes from here: which stream carries it, what an error
report looks like, and when colour is emitted.

The split is by audience rather than by severity. Machine-readable output goes to stdout so
a pipe carries only what a program can consume; progress, summaries, notes, and errors go
to stderr. `internal/ui` owns both, which means every stream any other component is handed
is one it wrapped — a dependency that renders its own output, such as cobra's help, must
not be able to write around it or fail silently behind its back.

An error is `graft: ` and the message, on one line. Every message SPEC.md's failure-mode
table names already locates its own problem, so the report adds nothing to it.

Colour is one decision for the whole run, taken from stdout, exactly as SPEC.md words it.

## Requirements

### Requirement: Machine-readable output goes to stdout and everything else to stderr

`internal/ui` SHALL own both of graft's output streams and SHALL carry them separately.
Machine-readable output — the thing a caller would pipe into another program — SHALL go to
the **standard output** stream. Progress, summaries, notes, and errors SHALL go to the
**error** stream, so a pipe is never corrupted by text a human was meant to read.

Ownership means every stream any other component is handed SHALL be a writer `internal/ui`
wrapped, so that output written by a dependency — cobra's help and usage renderers, which
graft does not format itself — still lands on the stream `internal/ui` chose and still has
its write failures recorded. No component SHALL be given the process's real streams
directly.

A run that fails SHALL write **nothing at all** to the standard output stream from the
failure onward, and a run whose only output is a note or an error SHALL leave the standard
output stream byte-empty.

#### Scenario: A note leaves stdout untouched

- **WHEN** a note is written through the output surface
- **THEN** the error stream holds the note followed by one newline
- **AND** the standard output stream is byte-empty

#### Scenario: An error report leaves stdout untouched

- **WHEN** the error `source "shared": rev "v9.9.9" not found` is reported through the
  output surface
- **THEN** the error stream holds the report
- **AND** the standard output stream is byte-empty

#### Scenario: Machine-readable output goes to stdout only

- **WHEN** the line `openspec-schemas v1.2.0` is printed through the output surface
- **THEN** the standard output stream holds `openspec-schemas v1.2.0` followed by one
  newline
- **AND** the error stream is byte-empty

### Requirement: An error is reported on one line prefixed with `graft: `

An error SHALL be rendered as `graft: ` followed by the error's own message and one newline,
on the error stream. The message SHALL be passed through unaltered: SPEC.md's failure-mode
table is written as the messages the packages already produce, each of which locates its own
problem — `source "shared": …`, `graft.toml: …`, `catalog.yaml: …` — and a second layer of
context would say the same thing twice.

The output surface SHALL add no prefix, suffix, or wrapping of its own beyond `graft: ` and
the newline, and reporting a `nil` error SHALL write nothing.

#### Scenario: A failure-mode-table message is reported verbatim after the prefix

- **WHEN** the error `source "shared": selector "agent:*" matches no item; catalog provides
  schema:tdd` is reported
- **THEN** the error stream holds exactly
  `graft: source "shared": selector "agent:*" matches no item; catalog provides schema:tdd`
  followed by one newline

#### Scenario: A nil error reports nothing

- **WHEN** a `nil` error is reported
- **THEN** both streams are byte-empty

#### Scenario: A usage error carries a hint on its own line

- **WHEN** a usage error is reported and followed by the hint `run "graft --help" for usage`
- **THEN** the error stream holds the `graft: ` line first and the hint on the line after it
- **AND** the hint carries no `graft: ` prefix, because it is not a second failure

### Requirement: Colour is dropped when stdout is not a terminal or NO_COLOR is set

The output surface SHALL take **one** colour decision for the whole run and apply it to
both streams. Colour SHALL be enabled only when the standard output stream is a terminal
**and** the `NO_COLOR` environment variable is unset or empty. This is SPEC.md's rule
verbatim, including the part that reads oddly — a run whose standard output is redirected
emits no colour on the error stream either — because one decision cannot make the two
streams disagree, and a colourless terminal is a smaller harm than an escape sequence in a
captured log.

`NO_COLOR` SHALL disable colour when it is present and non-empty, whatever its value, and
SHALL NOT disable colour when it is absent or present and empty. That is the `NO_COLOR`
convention as published, and following it means `NO_COLOR=` behaves the way a user who
cleared it expects. Absent and present-but-empty are deliberately **one** case: the
environment is read with a `Getenv`-shaped function, which reports both as the empty string,
and graft has no reason to tell them apart.

Terminal detection SHALL ask the operating system whether the stream is a terminal rather
than inspecting the file's mode bits, because a character device is not a terminal:
`/dev/null` has the character-device bit set and a mode check reports it as one, which is
exactly the redirection the rule exists to catch.

With colour disabled, every styling helper SHALL return its argument unchanged, byte for
byte, rather than returning a string with empty escape sequences around it.

#### Scenario: A terminal with NO_COLOR unset gets colour

- **WHEN** the colour decision is taken with `NO_COLOR` unset and the standard output stream
  reported as a terminal
- **THEN** colour is enabled
- **AND** styling `updated` yields a string that both begins with an escape sequence and
  contains `updated`

#### Scenario: NO_COLOR set to any non-empty value drops colour

- **WHEN** the colour decision is taken with `NO_COLOR` set to `1`, and separately to `0`,
  and separately to `false`, with the standard output stream reported as a terminal
- **THEN** colour is disabled in every one of those cases
- **AND** styling `updated` yields exactly `updated`

#### Scenario: NO_COLOR set to the empty string does not drop colour

- **WHEN** the colour decision is taken with `NO_COLOR` set to the empty string and the
  standard output stream reported as a terminal
- **THEN** colour is enabled
- **AND** this is the same input as an unset `NO_COLOR`, which is the point: a cleared
  variable must not behave differently from a removed one

#### Scenario: The colour decision follows stdout and never stderr

- **WHEN** the command surface takes the colour decision
- **THEN** the terminal test is asked about the standard output stream
- **AND** it is never asked about the error stream, so a run whose error stream is a
  terminal and whose standard output is redirected still emits no colour

#### Scenario: A redirected stdout drops colour

- **WHEN** the colour decision is taken with `NO_COLOR` unset and the standard output stream
  reported as not a terminal
- **THEN** colour is disabled

#### Scenario: A character device that is not a terminal is not a terminal

- **WHEN** terminal detection is asked about a real file opened at `/dev/null`
- **THEN** it answers no
- **AND** it answers no for an in-memory buffer and for the read and write ends of a real
  pipe

#### Scenario: Styling with colour off is byte-identical to the input

- **WHEN** each styling helper is applied to `removed  agent:phase-orchestrator` with colour
  disabled
- **THEN** each returns exactly `removed  agent:phase-orchestrator`, with no escape
  sequence, no reset sequence, and no change in length
