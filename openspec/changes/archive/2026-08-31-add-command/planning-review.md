## Reviewed Artifacts

- `proposal.md`
- `specs/add-execution/spec.md` (new capability, 36 scenarios)
- `specs/manifest-format/spec.md` (delta, ADDED only, 15 scenarios)
- `specs/rev-resolution/spec.md` (delta, ADDED only, 5 scenarios)
- `specs/file-application/spec.md` (delta, ADDED only, 4 scenarios)
- `design.md`
- `tasks.md` (16 groups, 73 tasks)

**The finding pass was not delegated.** The schema requires reviewers that did not write the
planning package, and that rule earned its place — the same pass on `semver-ranges` caught a
design that would not have compiled. It was skipped here on the user's explicit instruction
for this session to do the work itself, and the cost is recorded rather than hidden: the
findings below come from reading the artifacts against the live code, not from independent
eyes. Every claim one of them makes about the existing code was checked by opening the file
rather than by recalling it, which is the part of the delegated pass that can be reproduced
alone. Group 14 still dispatches an independent reviewer against the *implementation*.

Every delta uses `## ADDED Requirements` only. No existing requirement is modified anywhere in
this change, which removes the failure mode that produced five CRITICALs last time: a
MODIFIED block that reproduces its original partially loses the missing half permanently at
archive.

## Reviewed Against

- `SPEC.md` — the `## graft add` section, the command table, the failure-mode table, the
  `graft.toml` example's own key alignment.
- The live code: `internal/manifest/setrev.go` (the editing discipline the two new edits
  follow), `internal/sync/run.go` (`movePin`, `resolve`, the pin check), `internal/plan/destination.go`,
  `internal/lock/pins.go`, `internal/source/{resolve,rangeselect,git}.go`, `internal/cli/*`.
- `openspec/specs/` — capability names, and the requirements each delta sits beside.
- `AGENTS.md` — the standing rules, in particular *`graft.toml` is a human's file* and
  *`internal/apply` is the sole writer*.

## Gaps Found and Fixed

### 1. `--list` needed a destination computation `internal/plan` does not have — CRITICAL

`design.md` and task group 5 first said `--list` would compute destinations "from a catalog
plus an optional override map, with no filesystem access". Reading `internal/plan/destination.go`
shows that is not enough: `destinations` decides a destination differently depending on
`listing.Dir`, and whether an item's `from` is a directory is a filesystem fact the catalog
does not carry. A plan built on the original wording would have produced the wrong destination
for every directory item, or pulled a filesystem read into the one package that may not have
one.

**Fixed** in `design.md` → Boundaries and task 5.1/5.2: the exported function is
`ItemDestinations(Input, Item)`, still pure, told whether the item is a directory by the
`Listing` the caller already holds from `source.List`. Task 5.2 now also names the factoring —
entry interpolation, override lookup, repo-root check — so the two callers share one copy
rather than growing a second opinion about where a kind places things.

### 2. A `graft.toml` that does not parse had no scenario — WARNING

Every other failure path was specified, including an unreachable source and an unamendable
manifest, but the most ordinary one was missing: `add` reads `graft.toml` before it can amend
it, and a manifest with a typo in it fails there. Without a scenario, the natural
implementation — treat a read failure as "no manifest yet" — would silently *overwrite* a
broken manifest with a fresh one holding a single source, destroying the consumer's file.

**Fixed**: added *An unparsable graft.toml is refused before anything is resolved* to
`add-execution`, with its matrix row and a test in task 8.1. Absence and unreadability are now
explicitly different things.

### 3. The multi-line-array scenario contradicted itself — WARNING

The first draft of *A multi-line array with no trailing comma keeps that style* said the last
element's line gaining a comma is "not what happens", then described text that requires one.
An array whose last element carries no comma cannot gain a sibling without one somewhere, so
the requirement as written was unimplementable.

**Fixed** in `manifest-format`: the amendment may add exactly one byte to one existing line — a
comma on an element that had none — and the scenario now asserts exactly two changed lines.
This is the single deliberate exception to "no existing line is rewritten", and it is stated as
such in the requirement rather than discovered during implementation.

### 4. A dotted source name would have produced a table that means something else — WARNING

The derivation rule first allowed `.` in a source name. `[sources.my.repo]` is not the source
named `my.repo`; it is a sub-table of `my`. The append would have written a file that parses,
declares a source nobody asked for, and whose `install` array the amender can never find again.

**Fixed**: the derived name must match `^[A-Za-z0-9_-]+$`, stated in both `add-execution` and
`manifest-format`, with the reason attached and a refusal scenario in each.

### 5. `--no-sync` could not be implemented through the plan path — CRITICAL, caught in design

The obvious implementation of "write the manifest and stop" is an apply with an empty plan.
`internal/plan`'s prune set is the lock's files minus the new resolution, so an empty plan
against a populated lock deletes everything the lock claims — the one thing `AGENTS.md` says
graft may never do. This is why `file-application` gains a manifest-only entry point rather
than `add` reusing the existing one, and why task 4.3 asserts a populated lock's files all
survive.

## No Remaining Implementation-Blocking Gaps

Verified against the live code rather than assumed:

- **`lock.Load` returns an empty lock for an absent file** (`internal/lock/lock.go`), so `add`
  in a repository with neither file needs no special case.
- **`lock.CheckPins` skips a source the lock does not name** (`internal/lock/pins.go`), so a
  brand-new source passes the pin check, and `resolve` in `internal/sync/run.go` already
  resolves a source with no lock entry — the carve-out `semver-ranges` established.
- **`itemid.Valid` accepts a glob in the name position**, which is what lets task 11.1 check
  `agent:*` from the command line with the same predicate `manifest.validate` uses.
- **No import cycle.** `internal/add` imports `manifest`, `source`, `plan`, `apply`, and
  `sync`; nothing imports `internal/add` except `internal/cli`. The cycle that sank the first
  `semver-ranges` design — `lock → source → plan → lock` — is not approached here.
- **`sourceKeyWidth` is unaffected.** No key is added to `graft.lock`, so its marshalling is
  untouched and `version` stays `1`. The `graft list --json` document's `version` stays `2`.
- **Every one of the 60 spec scenarios has a matrix row**, checked programmatically by
  comparing the two sets, and every row names a tier, its collaborators, and a command.
- **Every task group carries exactly one kind marker**, and the two `parallel-after: 0` groups
  — 3 (`internal/source`) and 4 (`internal/apply`) — touch disjoint packages with no shared
  state. Group 5 is marked parallel too and touches only `internal/plan`.

## Deferred Non-Blocking Notes

- **`add` fetches during `--list` and during `--no-sync` without `@rev`.** Both are stated in
  the specs as touching the cache and not the tree, which is the caveat `--dry-run` already
  carries. Not worth a flag to suppress.
- **`--as <name>` is refused rather than solved.** Two repositories whose last path segment
  collides cannot both be added without hand-editing `graft.toml`. Recorded as a non-goal; if
  it turns up in practice, it is a small additive change.
- **The no-selector refusal ignores the terminal.** `add-picker` will narrow it to the no-TTY
  case, which is a MODIFIED requirement in that change rather than a gap in this one. The
  scenario asserting the refusal is identical on a terminal exists so that narrowing has
  something to move.

## Implementation Review (group 14)

Also not delegated, for the same reason and at the same cost. What follows is what the pass
found, recorded so that a reader can judge it rather than take it on trust.

### Fixed

1. **The re-resolution decision was read out of a human-readable message — WARNING, fixed.**
   `add.Run` decided whether to re-resolve a source by testing whether any report line began
   with `graft.toml: moved source `. Rewording that message — an ordinary edit, and one the
   error-strings rule invites — would have silently stopped the pin move from being
   re-resolved, with nothing going red until a user noticed a lock that disagreed with its
   manifest. The amendment now returns an explicit `pinMoved` field. The acceptance test
   *An explicit rev on a declared source moves the pin* is what goes red if it regresses:
   without the re-resolution the pin check refuses the run outright.

2. **A spec scenario named a manifest shape that cannot reach the amender — WARNING, fixed.**
   *An unamendable manifest is refused in the amender's words* described an `install` written
   as a multi-line string. `manifest.Parse` refuses that before any amendment is attempted, so
   the scenario tested the parser rather than the amender. It now names an inline table, which
   parses and which no `[sources.<name>]` header covers.

3. **`AddInstall` had to refuse an escaped element, which the spec did not say — WARNING,
   fixed.** The amendment compares each element against the selectors it was given; comparing
   undecoded text would miss a match and write a duplicate the next parse refuses. The refusal
   was implemented and the spec gained the scenario and the reason, rather than the code
   carrying a rule the contract did not.

4. **Three matrix rows named a tier the tests do not use — WARNING, fixed.** Seven
   add-execution scenarios, the three report scenarios, and the four `--list` scenarios are
   tested at `internal/add` against a fixture repository rather than through the process
   boundary, which is the fastest tier that can express them. The matrix now says so. The
   scenarios that genuinely need the boundary — the stream split, a failed run leaving no
   manifest, `--no-sync` making no network call — stayed acceptance.

### Deferred, with reasons

- **`graft add` in a repository holding `graft.lock` and no `graft.toml` prunes every other
  source's files.** It follows directly from the documented rule — a source the manifest no
  longer declares has its files pruned from the lock alone — and `add` creating the manifest
  is the documented behavior. It is still the one path where a user who deleted `graft.toml`
  by hand gets files removed without editing a manifest. Left as is because changing it means
  a new refusal nothing specifies; worth raising for `self-hosting`.
- **The hidden `help` command appears in `graft --help` with an empty description, and then
  refuses when invoked.** Verified pre-existing by building `48464b9`: it is `command-surface`'s
  and not this change's, and fixing it here would be an undiscussed change to another
  capability's output.
- **`--list` uses a declared source's kind overrides without checking that its `git` matches.**
  An override is a statement about where a kind belongs in this repository, so applying it is
  defensible; a run that went on to *write* would refuse the mismatch before anything landed.

### Verified rather than assumed

- Every one of the 61 spec scenarios has a test or a deterministic check, and every matrix row
  names the tier its test actually runs at.
- `task lint` is clean, `task cover` reports 93.4% over `./internal/...` against a floor of 80%,
  and `task build` produces a binary whose `--help` lists `add`.
- Nothing was added to `cmd/graft`, which CI never executes.
- `graft.lock`'s format and `version`, and the `graft list --json` document's, are untouched.
