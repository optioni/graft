## Context

`graft add <source>` without selectors refuses today and tells the user to run `--list`.
SPEC.md promises a multi-select list there, and says the thing that matters about it: **the
picker chooses selectors and has no other powers.** Everything it can do a flag can do, and
every effect it has runs through `internal/add`'s existing sequence.

The risk in an interactive layer is that it becomes a second implementation of the tool — a
place where a destination is computed differently, or a write happens on a path the
non-interactive form never takes. The design below closes that by making the picker a
function from a catalog to a list of strings, and by giving it no way to reach anything else.

## Goals / Non-Goals

**Goals:**
- A multi-select over the source's catalog, showing each item's destination, on a TTY only.
- The `kind:*` collapse offer, as a semantic choice rather than a confirmation.
- A widget testable by pressing keys at it, with no pseudo-terminal anywhere in the suite.

**Non-Goals:**
- Any power beyond returning selectors — no rev choice, no destination editing, no override.
- Search, filtering, mouse, or a full-screen alternate buffer.
- An `--interactive` flag, or any prompt that reads from a pipe.
- A picker for `sync` or `update`.

## Boundaries

| Piece | Package | Pattern it follows |
|---|---|---|
| Model, key bindings, rendering, collapse offer | `internal/picker` (new) | `internal/plan` — pure over values, no filesystem, no clock |
| Terminal driver | `internal/picker` | `internal/ui` — owns one mechanism, injected rather than looked up |
| The chooser hook | `internal/add` | `sync.Options.Update` — an optional behavior the sequence takes as a parameter |
| Input stream, interactivity test, raw mode | `internal/cli` | `Options.IsTerminal` — everything the process supplies arrives as a value |

`internal/picker` imports `internal/ui` for `Pad` and nothing else of graft's. It never sees
a manifest, a lock, a cache, or a repository root.

## Contracts

- `picker.Item{ID, Kind string; Destinations []string}` — built by `internal/add` from the
  catalog and `plan.ItemDestinations`, which is what makes the picker's destinations and
  `--list`'s the same computation rather than two that agree today.
- `picker.Model` with `Update(Key) Model`, `View() []string`, `Done() bool`,
  `Cancelled() bool`, `Selectors() []string`.
- `picker.Run(t Terminal, items []Item) ([]string, error)` where
  `Terminal{In io.Reader; Out io.Writer; MakeRaw func() (func(), error)}`.
- `add.Request.Choose func([]picker.Item) ([]string, error)` — called only when `Install` is
  empty, after the rev is resolved and the catalog read. A nil `Choose` with no selectors is
  an internal invariant the command surface never produces.
- `cli.Options` gains `Stdin io.Reader`, `Interactive func() bool`, and
  `MakeRaw func() (func(), error)`, each defaulting to the real process when nil.

All additive. No file format, no command output, and no existing error string moves.

## Persistence and Rollout

- Migration, backfill, seeding, cache invalidation, index rebuild: none.
- Authorization: unchanged.
- Observability: the picker draws on the error stream and leaves nothing behind; a cancelled
  run reports `add cancelled`.
- Deployment: none beyond the next release.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Terminal / raw mode | replaced — `MakeRaw` is a function returning a recorded restore func; no pseudo-terminal anywhere | same |
| Standard input | replaced — a `bytes.Reader` of key bytes | same |
| Error stream | `bytes.Buffer` through `cli.Options` | same |
| Interactivity test | replaced — `Options.Interactive` returns a value the test chooses | same |
| Filesystem (working tree) | real, under `t.TempDir()` | absent for `internal/picker`, which is pure |
| `git` binary | real, against a fixture repo built in `t.TempDir()` | not reached by `internal/picker` at all |
| Fetch cache | real, rooted in `t.TempDir()` | not reached |
| Clock, randomness, network | not used | not used |

There is deliberately **no pseudo-terminal**. A pty would test the operating system's line
discipline, take a dependency to create one, and be the flakiest thing in the suite; the
driver is shaped so that a byte stream and a no-op raw-mode function reach every line of it
but `term.MakeRaw` itself.

## Test Strategy

`internal/picker` is a unit-tested package end to end: the model by pressing keys at values,
the driver by handing it a scripted byte stream. `internal/add` gets integration tests with a
scripted chooser against a fixture repository, which is where "the picker's selectors enter
the sequence exactly where a flag's would" is provable. `internal/cli` keeps the
usage-error tests it has.

This change does **not** take a new outer-loop acceptance test through the compiled binary:
the binary's `add` path is already covered end to end by `add-command`'s acceptance suite,
and the only thing this change adds to it is a branch that cannot be entered without a
terminal the acceptance harness deliberately does not have. The equivalent evidence — that
a chosen selector produces the same manifest a typed one does — is asserted at
`internal/add` against a real fixture repository.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| Pressing keys moves a cursor and selects | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| The cursor stops at both ends | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| Rendering names every item's destination | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| An item with several destinations names all of them | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| `a` selects all and then clears all | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| An unbound key changes nothing | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| `enter` confirms and stops | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| Selecting both agents offers the glob | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| A kind with one item is never offered | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| Two wholly selected kinds are offered separately | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| Cancelling at the offer cancels the whole picker | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| Cancelling with a selection made discards it | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| Confirming nothing is a cancellation | Key events applied to a model value, asserting the model and its rendered lines | unit | none — pure over values | `go test ./internal/picker/` |
| A scripted key stream chooses a selector | The driver over a `bytes.Reader` of keys and a `bytes.Buffer`, with a no-op raw-mode function | unit | none — no terminal, no pseudo-terminal | `go test ./internal/picker/` |
| An arrow key is one event, not an escape and two letters | The driver over a `bytes.Reader` of keys and a `bytes.Buffer`, with a no-op raw-mode function | unit | none — no terminal, no pseudo-terminal | `go test ./internal/picker/` |
| Raw mode is restored on every path | The driver over a `bytes.Reader` of keys and a `bytes.Buffer`, with a no-op raw-mode function | unit | none — no terminal, no pseudo-terminal | `go test ./internal/picker/` |
| A closed input cancels | The driver over a `bytes.Reader` of keys and a `bytes.Buffer`, with a no-op raw-mode function | unit | none — no terminal, no pseudo-terminal | `go test ./internal/picker/` |
| No selectors, no TTY | `cli.Main` with recorded streams and a stubbed interactivity test | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| No selectors on a terminal opens the picker | `add.Run` against a fixture source repo with a scripted chooser | integration | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/add/` |
| A cancelled picker writes nothing | `add.Run` against a fixture source repo with a scripted chooser | integration | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/add/` |
| A malformed selector is refused before the network | `cli.Main` with recorded streams and a stubbed interactivity test | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| An ungraftable source fails before any list is shown | `add.Run` against a fixture source repo with a scripted chooser | integration | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/add/` |

<!-- 22 rows, one per spec scenario -->
## Decisions

**No new dependency.** `golang.org/x/term` is already a direct dependency for
`ui.IsTerminal`, and it provides `MakeRaw` and `Restore`. Bubbletea or huh would each pull a
tree of modules to draw one list, into a tool whose whole argument is that it is small enough
to read. *Alternative considered:* `charmbracelet/huh`, which would be perhaps thirty lines
of caller code. Rejected on dependency weight for a widget this small, and because the model
this design needs — testable without a terminal — is the part huh would hide.

**The model is a value and the driver is a loop.** Every behavior worth asserting is a pure
function of a model and a key. *Alternative considered:* driving a real pty in tests.
Rejected: it tests the OS, needs a dependency, and is the flakiest thing that would then be
in the suite.

**The chooser is a hook on the request, not a call from the command surface.** `internal/cli`
could fetch the catalog, run the picker, and then call `add.Run` with the result — but that
resolves the rev twice, and between the two resolutions the remote can move, so the list
shown and the tree installed would be from different commits. Passing a `Choose` function
keeps one resolution and one fetch.

**Interactive means both standard input and the error stream are terminals.** Input, because
the picker reads keys; the error stream, because that is where it draws. Standard output is
deliberately not consulted: `graft add … | tee log` should still be able to prompt, and
stdout stays byte-empty for a syncing add either way.

**The collapse offer is a screen, not a flag.** It is a semantic choice — `agent:*` adopts
what the source adds later — and the only place a user has the context to make it is right
after selecting every item of that kind. A kind with one item is never offered, because the
two spellings are identical today and the offer would be a formality that teaches the user
to accept without reading.

**Confirming an empty selection cancels.** The alternative is writing a source whose
`install` is empty, which `manifest.Parse` refuses — the run would fail after the picker
succeeded, which is the worst place to put the refusal.

## Risks / Trade-offs

[Raw mode left on when the process dies mid-picker] → The driver restores it on every return
path, including a read error, and the restore is asserted in a test. A SIGKILL is beyond
reach for any design here, as it is for every terminal program.

[An escape sequence read as a bare `esc` cancels the picker on an arrow press] → The decoder
treats an unrecognised sequence as an unbound key rather than as its first byte, and the
three-byte arrow sequences are asserted directly.

[The picker's destinations drift from what a sync writes] → They are `plan.ItemDestinations`,
the same function `--list` calls. A second computation is the thing this avoids.

[A terminal too short for the catalog] → The list is windowed to the cursor. A terminal whose
height cannot be read falls back to a fixed window rather than printing an unbounded list.

## Migration Plan

None. Additive: a new package, one optional field on an existing request, three optional
fields on `cli.Options`. Rollback is reverting the commits.

## Open Questions

None.
