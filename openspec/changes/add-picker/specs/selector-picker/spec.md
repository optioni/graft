## ADDED Requirements

### Requirement: The picker is a pure model over key events

`internal/picker` SHALL express the multi-select as a value: a model holding the items, the
cursor, the selected set, and which screen is showing, updated by one function from a key
event to a new model, and rendered by another from the model to lines of text.

Neither function SHALL read a file, a terminal, an environment variable, or a clock. The
model is what makes the picker a widget that can be tested by pressing keys at it rather
than a program that can only be tested by driving a terminal, and it is why the driver
below has almost nothing left to get wrong.

The model SHALL be constructed from items carrying an id, a kind, and the destinations the
item would be written to — the same destinations `graft add --list` prints, computed by
`internal/plan`. The picker SHALL compute no destination of its own.

#### Scenario: Pressing keys moves a cursor and selects

- **WHEN** a model over three items receives `down`, `down`, `space`, `up`, `space`
- **THEN** the cursor is on the second item and the second and third items are selected
- **AND** no other item is selected

#### Scenario: The cursor stops at both ends

- **WHEN** a model over three items receives `up` at the first item, or `down` at the last
- **THEN** the cursor does not move, and nothing is selected or deselected
- **AND** the model is otherwise byte-identical to the one before the key

#### Scenario: Rendering names every item's destination

- **WHEN** a model over `agent:reviewer` at `.claude/agents/reviewer.md` and `schema:tdd` at
  `openspec/schemas/tdd/` is rendered
- **THEN** each item's line carries its id and its destination, ids padded to a common width
- **AND** the selected ones are marked and the cursor's line is marked, distinguishably

#### Scenario: An item with several destinations names all of them

- **WHEN** an item's kind places it at two destinations
- **THEN** its line names both, separated by `, `, exactly as `--list` renders them

### Requirement: `space` selects, `a` toggles all, `enter` confirms

The picker SHALL bind exactly these keys, and no others SHALL have an effect:

- `up`/`k` and `down`/`j` move the cursor
- `space` toggles the item under the cursor
- `a` selects every item, or — when every item is already selected — deselects every item
- `enter` confirms the current selection
- `q`, `esc`, and `ctrl-c` cancel

A key with no binding SHALL leave the model unchanged rather than being reported as an
error: a user pressing an unbound key has not made a mistake worth stopping for.

#### Scenario: `a` selects all and then clears all

- **WHEN** a model over three items with one selected receives `a`
- **THEN** all three are selected
- **AND** a second `a` leaves none selected

#### Scenario: An unbound key changes nothing

- **WHEN** a model receives `x`, `tab`, or a key it does not bind
- **THEN** the model is unchanged and the picker is still running

#### Scenario: `enter` confirms and stops

- **WHEN** a model with two items selected receives `enter` and no kind is wholly selected
- **THEN** the picker reports it is done and its chosen selectors are those two ids, in
  catalog order rather than the order they were clicked

### Requirement: A wholly selected kind is offered as `kind:*`

When `enter` confirms a selection in which **every** item of some kind is selected and that
kind has more than one item, the picker SHALL show a second screen offering to collapse
that kind's items to `kind:*`, one offer per such kind, each individually accepted or
declined.

The offer SHALL state what it means rather than ask for confirmation: `kind:*` adopts items
the source adds later, and an explicit list does not. A kind with exactly one item SHALL NOT
be offered — `agent:*` and `agent:reviewer` name the same thing today and different things
tomorrow, and offering that as a formality would teach the user to accept it without
reading.

Accepting SHALL replace that kind's ids with the single selector `kind:*`; declining SHALL
leave the ids. The resulting selectors SHALL be ordered by kind as the catalog orders them,
and within a kind by id.

#### Scenario: Selecting both agents offers the glob

- **WHEN** a catalog offers `agent:reviewer`, `agent:planner`, and `schema:tdd`, both agents
  are selected, and `enter` is pressed
- **THEN** the picker shows the collapse offer for `agent`, naming that `agent:*` adopts
  agents the source adds later
- **AND** accepting yields the selectors `agent:*`, and declining yields `agent:planner` and
  `agent:reviewer`

#### Scenario: A kind with one item is never offered

- **WHEN** the only `schema` item is selected and `enter` is pressed
- **THEN** no collapse offer is shown for `schema` and the selector stays `schema:tdd`

#### Scenario: Two wholly selected kinds are offered separately

- **WHEN** every item of two kinds is selected and `enter` is pressed
- **THEN** each kind gets its own offer, and accepting one and declining the other yields
  one glob and the other kind's explicit ids

#### Scenario: Cancelling at the offer cancels the whole picker

- **WHEN** `ctrl-c` is pressed at the collapse offer
- **THEN** the picker reports cancellation and returns no selectors, exactly as cancelling
  at the list does

### Requirement: Cancelling returns nothing, and an empty confirmation is a cancellation

`q`, `esc`, and `ctrl-c` SHALL end the picker with no selectors and a cancellation, at any
screen. Confirming with nothing selected SHALL be the same outcome: an add with no selectors
is not a thing to write, and treating it as one would append a source whose `install` list
the next parse refuses.

#### Scenario: Cancelling with a selection made discards it

- **WHEN** two items are selected and `q` is pressed
- **THEN** the picker reports cancellation and returns no selectors

#### Scenario: Confirming nothing is a cancellation

- **WHEN** `enter` is pressed with no item selected
- **THEN** the picker reports cancellation and returns no selectors

### Requirement: The driver owns raw mode, the terminal, and nothing else

`internal/picker` SHALL provide a driver that reads key events from a byte stream, writes
frames to a writer, and returns the model's chosen selectors. Raw mode SHALL be entered
through a function the caller supplies and SHALL be restored before the driver returns,
including when it returns an error and when the model cancels.

The driver SHALL be usable with an ordinary `io.Reader` and `io.Writer` and a raw-mode
function that does nothing, which is how it is tested: a pseudo-terminal is not needed to
prove that pressing `j`, `space`, and `enter` yields a selector.

It SHALL decode at least the escape sequences for the four arrow keys alongside the plain
keys, and SHALL treat an unrecognised escape sequence as an unbound key rather than as its
final byte — reading `esc [ A` as a bare `esc` would cancel the picker on an arrow press.

Input ending before the model finishes — a closed stream — SHALL be a cancellation, not a
hang and not a confirmation.

#### Scenario: A scripted key stream chooses a selector

- **WHEN** the driver is run over a reader holding `j`, `space`, `\r` with a no-op raw-mode
  function
- **THEN** it returns the second item's id and no error
- **AND** the writer holds at least one rendered frame

#### Scenario: An arrow key is one event, not an escape and two letters

- **WHEN** the driver reads the three bytes `\x1b[B`
- **THEN** the cursor moves down once and the picker is still running

#### Scenario: Raw mode is restored on every path

- **WHEN** the driver returns, whether by confirmation, cancellation, or a read error
- **THEN** the restore function supplied with raw mode has been called exactly once

#### Scenario: A closed input cancels

- **WHEN** the reader is empty and returns EOF immediately
- **THEN** the driver reports cancellation, restores raw mode, and does not block
