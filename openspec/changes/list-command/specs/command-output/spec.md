# Command Output Specification

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
