# Command Output Specification

## MODIFIED Requirements

### Requirement: The standard output stream carries the content a caller asked for

`internal/ui` SHALL own both of graft's output streams and SHALL carry them separately. The
**standard output** stream SHALL carry the content a caller asked graft for — output shaped
for a program, and equally output a person asked to see: `--version`, help, and a listing.
Progress, summaries, notes about the *absence* of content, and errors SHALL go to the
**error** stream, so a pipe is never corrupted by text a human was meant to read.

The split is by audience rather than by severity, and the audience is decided by what was
asked for rather than by whether the text is machine-readable. A listing is the content the
command exists to emit, so `graft list | grep agent:` has to work; a sync report is a summary
of something that happened, so it does not go where a pipe would carry it. `nothing
installed` is a note about there being no content, so it goes where notes go and stdout stays
byte-empty.

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

#### Scenario: Content a person asked for goes to stdout too

- **WHEN** `graft list` prints a listing in a repository with something installed
- **THEN** the standard output stream holds the listing and the error stream is empty
- **AND** when the same repository has nothing installed, the note `nothing installed` goes
  to the error stream instead and the standard output stream is byte-empty

## ADDED Requirements

### Requirement: The render vocabulary two commands share is one decision

`internal/ui` SHALL own the small phrases more than one command renders, so that two commands
cannot say the same thing two ways. There are three, and each is a single decision for the
whole tool:

- **A file count** SHALL render as `1 file` for exactly one and `<n> files` for every other
  count, zero included.
- **A short sha** SHALL be the first seven characters of a resolved sha. A value of seven
  characters or fewer SHALL be returned unchanged rather than padded or truncated.
- **Column padding** SHALL be the spaces that take a field of a given length up to a given
  width, and SHALL be empty when the field is already at or beyond that width.

These are output decisions, and `internal/ui` is where graft's output decisions live. Holding
them anywhere else means holding them in each renderer that needs them, and two renderers that
disagree about `1 files` or about six characters of sha are a defect no test of either
renderer alone would catch.

Padding SHALL be computed on unstyled text, so that enabling colour never moves a column.

#### Scenario: One file is singular and every other count is plural

- **WHEN** the file count is rendered for `1`, `6`, and `0`
- **THEN** the results are `1 file`, `6 files`, and `0 files`

#### Scenario: A short sha is the first seven characters

- **WHEN** the short form of `fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5` is rendered
- **THEN** it is `fae2a30`

#### Scenario: A sha too short to shorten is returned as it is

- **WHEN** the short form of `abc` is rendered, and separately of `abcdefg`
- **THEN** the results are `abc` and `abcdefg`, neither padded nor truncated

#### Scenario: Padding fills to the width and never beyond it

- **WHEN** padding is computed for a field of length `3` at width `7`
- **THEN** the result is four spaces
- **AND** padding for a field of length `7` at width `7`, and for length `9` at width `7`, is
  the empty string

#### Scenario: The sync report is unchanged by where these live

- **WHEN** the report for SPEC.md's own example is rendered
- **THEN** it is byte-identical to what it was before these phrases moved, alignment included
