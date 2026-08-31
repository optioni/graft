## Reviewed Artifacts

- `proposal.md`
- `specs/selector-picker/spec.md` (new capability, 5 requirements, 17 scenarios)
- `specs/add-execution/spec.md` (one REMOVED requirement with its replacement ADDED, 5 scenarios)
- `design.md`
- `tasks.md` (9 groups, 40 tasks)

**Not delegated**, on the same standing instruction as `add-command`'s, and at the same cost:
these findings come from reading the artifacts against the live code rather than from
independent eyes. Every claim about existing code below was checked by opening the file.

## Reviewed Against

- `SPEC.md` — the `## graft add` section's picker paragraphs, the Output section's "no
  behavior that exists only on a TTY apart from the `add` picker", and the failure-mode row
  this change makes false.
- The live code: `internal/add/run.go` and `list.go` (where the chooser has to sit),
  `internal/plan/destination.go` (`ItemDestinations`), `internal/cli/main.go` (the Options
  shape), `internal/ui/render.go` (`Pad`), `go.mod` (`golang.org/x/term` already direct).
- `openspec/specs/add-execution/spec.md` — the requirement this change retires.

## Gaps Found and Fixed

### 1. The delta was written as MODIFIED and would have carried a false scenario — CRITICAL

The first draft modified *`add` without selectors is refused, naming what it needed* in
place. `openspec validate --strict` refused it: a MODIFIED block may not drop a scenario, and
the live requirement carries *No selectors on a terminal is the same refusal* — which this
change makes false. Keeping the name and inverting its content would have left the archived
spec asserting the opposite of what the name says.

**Fixed**: the requirement is REMOVED with a Reason and a Migration, and replaced by
*`add` without selectors opens the picker on a terminal, and is refused off one*, which
carries the unchanged non-interactive wording, exit code, and selector-syntax rule verbatim.

### 2. Calling the picker from the command surface would have resolved the rev twice — WARNING

The obvious wiring is: `internal/cli` fetches the catalog, runs the picker, then calls
`add.Run` with the chosen selectors. `add.Run` resolves the rev itself, so an add with no
`@rev` would make two `ls-remote` calls — and a tag published between them means the list
shown and the tree installed come from different commits.

**Fixed**: the chooser is a hook on `add.Request`, called after the one resolution and the
one fetch. Task 5.2 names the restructuring that needs.

### 3. A pseudo-terminal in the test suite — WARNING, designed out

The natural way to test a terminal widget is to open a pty, which needs a dependency, tests
the operating system's line discipline as much as the widget, and would be the flakiest thing
in this suite. The driver instead takes an `io.Reader`, an `io.Writer`, and a raw-mode
function, so every line but `term.MakeRaw` is reachable from a `bytes.Reader`. Task 4.3 is
the check that this stayed true.

## No Remaining Implementation-Blocking Gaps

Verified rather than assumed:

- **No new module dependency is needed.** `golang.org/x/term` is already a direct requirement
  in `go.mod`, and it provides both `MakeRaw` and `Restore`.
- **`plan.ItemDestinations` exists and is pure**, added by `add-command`, so the picker's
  destinations are the same computation `--list` prints rather than a second one.
- **No import cycle.** `internal/picker` imports `internal/ui` only; `internal/add` imports
  `internal/picker`; nothing imports `internal/add` but `internal/cli`.
- **`cli.Options` already carries `IsTerminal`**, so adding `Stdin`, `Interactive`, and
  `MakeRaw` follows a shape the package already has rather than introducing process lookups.
- **Every one of the 22 spec scenarios has a matrix row**, checked by comparing the two sets,
  and every row names a tier, its collaborators, and a command.
- **Every task group carries exactly one kind marker**, and no group is marked parallel: they
  are a chain from the model outward, each building on the last.

## Deferred Non-Blocking Notes

- **No outer-loop acceptance test.** Stated in design.md → Test Strategy with the reason: the
  compiled binary's `add` path is covered by `add-command`'s acceptance suite, and the branch
  this change adds cannot be entered without a terminal that harness deliberately lacks.
- **Terminal height.** The window falls back to a fixed height when the terminal's size
  cannot be read. A resize mid-picker redraws at the old height until the next key.
- **No search or filtering.** A catalog is tens of items. If one ever is not, this is an
  additive change to the model alone.

## Implementation Review (group 8)

Not delegated, on the same standing instruction, at the same cost.

### Fixed

1. **A test that could not fail — WARNING, fixed.** `TestAddWithoutSelectorsIsRefused` looped
   over a terminal flag and asserted the same refusal either way. Once the picker existed,
   the `true` case should have stopped refusing — but the flag it set was `IsTerminal`, which
   the new decision does not consult, and under `go test` standard input is never a terminal,
   so the test passed for a reason unrelated to what it claimed. It now drives `Interactive`
   directly and is split in two, and the second half was checked by removing the guard and
   watching it go red.

2. **A one-element loop dressed as a table — SUGGESTION, fixed.** The cancellation test
   iterated over a slice holding one key. Flattened.

### Verified rather than assumed

- **The picker works through a real pseudo-terminal.** Driven with `script -q /dev/null`, a
  scripted `space`, `enter` selected `agent:reviewer`, wrote the manifest, and installed the
  file; `q` printed `graft: add cancelled` and left the directory empty. The suite needs no
  pty, and this was still worth doing once by hand — the frames, the redraw, the erase, and
  raw mode are all things a byte-stream test cannot see.
- Every one of the 22 spec scenarios has a test. `internal/picker` is 92.1% covered and
  `internal/add` 89.8%; the repository is at 93.5% against a floor of 80%.
- `task lint` is clean and `task test` is green under `-race`.
- Nothing new writes to the working tree: the picker writes to a stream, and every file
  operation still goes through `internal/apply`.

### Deferred, with reasons

- **`--list` on a declared source shows the source's default pin, not the manifest's rev,
  while the picker shows the manifest's.** Each is defensible — `--list` answers "what does
  this source offer", the picker answers "what will I install" — and no scenario pins either.
  Left alone rather than changed inside a review; worth deciding deliberately if it ever
  confuses anyone.
- **A terminal resized mid-picker redraws at the old height until the next key.** Handling
  SIGWINCH means a signal handler in a package that currently has no concurrency at all.
- **`term.MakeRaw` is the one line the suite does not reach**, by design: it lives in
  `internal/cli`'s default `MakeRaw`, and the pty run above is what exercises it.
