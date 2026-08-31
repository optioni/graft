## Why

`graft add <source>` with no selectors is currently a refusal telling the user to go and
find out what the source offers. `--list` answers that, but the user then has to retype the
ids by hand. SPEC.md's `## graft add` section promises a multi-select list instead, and this
is the change that keeps that promise — the last piece of the command surface.

## What Changes

- `graft add <source>` with no selectors, when both standard input and the error stream are
  terminals, presents the source's catalog as a **multi-select list** showing each item's
  destination, and installs what was chosen.
- When every item of a kind is selected and that kind has more than one item, the picker
  **offers to collapse** the selection to `kind:*`. It is a semantic choice rather than a
  confirmation: `agent:*` adopts items the source adds later, an explicit list does not.
- Without a TTY the existing refusal is unchanged, and its wording does not move. This is
  the narrowing `add-command` said would come.
- Cancelling — `q`, `esc`, or `ctrl-c` — writes nothing and exits `1` with `add cancelled`.
  Confirming an empty selection is the same thing.
- The picker is drawn on the **error stream**. Standard output stays byte-empty for a
  syncing `add`, exactly as it is today.
- `internal/picker` is a new package: a pure model over key events, and a thin driver that
  owns raw mode. No new module dependency — `golang.org/x/term` is already direct.

Nothing about `graft.toml`, `graft.lock`, or `catalog.yaml` changes, and no non-interactive
behavior moves. There is no **BREAKING** change here.

## Non-Goals

- **Any power beyond choosing selectors.** The picker returns a selector list; every effect
  runs through the same `internal/add` sequence the flag form runs through.
- **Choosing a rev, a destination, or an override.** `@rev` and `graft.toml` own those.
- **Search, filtering, or mouse support.** Arrow keys, `space`, `a`, `enter`, and the three
  cancel keys. A catalog is tens of items, not thousands.
- **A picker for `update` or `sync`.** Neither has a choice to offer.
- **Anything on a non-TTY path.** No `--interactive` flag to force it, and no prompt that
  reads from a pipe.

## Capabilities

### New Capabilities
- `selector-picker`: the multi-select model, its key bindings, the destinations it shows,
  the collapse offer, cancellation, and the terminal driver's containment.

### Modified Capabilities
- `add-execution`: the no-selector path becomes the picker on a TTY and the existing
  refusal off one, and the selectors it returns enter the sequence exactly where a flag's
  would.

## Impact

- New: `internal/picker`.
- Changed: `internal/add` (a chooser the sequence calls when no selector was given, after
  the rev is resolved and the catalog read), `internal/cli` (three new `Options` fields for
  the input stream, the interactivity test, and raw mode; the `add` command wires them).
- No new dependency, no format change, no migration.
