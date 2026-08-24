## Context

Six changes have landed. `internal/manifest`, `internal/lock`, and `internal/catalog` parse
the three files; `internal/plan` turns them into a list of writes, a prune set, and the next
lock without touching anything; `internal/source` resolves a rev, fetches a tree into a
content-addressed cache, and lists what an item contributes; `internal/cli` and `internal/ui`
own the argument surface, the two streams, the error format, and the colour decision.

Nothing joins them. `graft` has no `sync` command and no package permitted to write to the
working tree, so every guarantee the six changes established — the prune set derived from the
lock alone, the collision check, the repo-root rule — is currently a property of values nobody
acts on.

This change adds the two pieces that make them real: `internal/apply`, which performs a plan's
file operations and nothing else, and `internal/sync`, which walks SPEC.md's resolution
sequence and hands `apply` a plan. It is the largest change in the roadmap and the only one
that can destroy a user's work, so the constraints below are stated as boundaries rather than
as intentions.

The planning review pushed hard on exactly that and changed the design in four ways worth
naming up front: `internal/apply` gained a **pre-flight pass** so its refusals leave the tree
byte-identical; **every ancestor of a destination or a prune path must be a directory**,
checked without following it, because `os.Root` deliberately follows a symlink that stays
inside its root; **an existing destination is removed and recreated** rather than truncated,
because truncation preserves the old mode; and **`.git/`, `graft.toml`, and `graft.lock` are
refused outright**, because a file placed in `.git/` is invisible to the `git diff` that is
SPEC.md's whole review story.

## Goals / Non-Goals

**Goals:**

- One package writes to the working tree, and it derives nothing: every path it touches comes
  from the plan it was given.
- A file absent from `graft.lock` is unreachable by graft. Not "not deleted" — unreachable:
  it is never in a prune set, nothing scans a destination directory to find one, and no path
  resolves through a symlink to reach one.
- `graft sync` installs the lock's pin. Nothing on the sync path can move one.
- `graft.lock` is written last, and only after every file operation succeeded.
- The report is derivable from the old lock, the new lock, and the plan — no content
  comparison, no second walk of the tree.
- **Every failure graft can see coming leaves the tree byte-identical to what it was.** That
  covers every condition this change specifies as a refusal, because the pre-flight pass
  decides all of them before the first byte is written. It does not cover an I/O failure that
  strikes mid-write; the Non-Goal below and the Risks section say what that leaves.

**Non-Goals:**

- Transactional apply. `internal/plan` refuses the plans whose partial application would be
  unrecoverable, and the pre-flight pass removes the refusals; a staging tree for the residual
  I/O failure buys little and costs a lot.
- Content comparison. "This file did not change" is git's answer, not graft's.
- `update`, `add`, `list`, and the picker. Later changes.
- Running `graft sync` against this repository's own `openspec/schemas/tdd/` or
  `.claude/agents/`. That is `self-hosting`, and nothing in this change's tests goes near
  them.

## Boundaries

| Package | Role in this change | Pattern followed |
|---|---|---|
| `internal/apply` | **New.** The only writer. Takes a repository root, the fetched tree per source name, and a `*plan.Plan`; runs a pre-flight validation pass, then writes, prunes, removes emptied directories, and writes the lock, in that order. | `internal/source`'s containment style: every path goes through an `os.Root`, `Lstat` never `Stat`. |
| `internal/sync` | **New.** SPEC.md's resolution sequence as one function taking its roots as values, plus the report as a value built from the two locks and the plan. | `internal/cli`'s `Options` shape: nothing read from a global, everything injected. |
| `internal/cli` | Gains `sync.go`: the cobra subcommand, its `--dry-run` flag, its argument validator, and the refusal of the `help` argument in `Main`. Renders the report through `internal/ui`. | The existing `__complete` refusal in `Main`, which is the mechanism this codebase already uses for a command cobra registers behind graft's back. |
| `internal/lock` | Gains `Filename = "graft.lock"`. Nothing else. | Additive constant. |
| `internal/manifest` | Gains `Filename = "graft.toml"`. Nothing else. | Additive constant. |
| `internal/plan` | **Untouched.** It stays pure: no filesystem access, no new field, no new function. A test that needs a real directory to exercise plan logic would mean the boundary moved. | — |
| `internal/catalog`, `internal/source`, `internal/ui`, `internal/itemid`, `cmd/graft` | **Untouched.** | — |
| `SPEC.md`, `ENGINEERING.md`, `AGENTS.md` | Documentation, updated by the operational group: SPEC.md's failure-mode table gains this change's new conditions and its Invariants gain the `.git` rule; the two layout blocks gain `internal/sync`. | — |

**The new write path is `internal/apply` and only `internal/apply`.** `internal/sync` calls
it; `internal/cli` calls `internal/sync`. Neither opens a file for writing, creates a
directory, or removes anything. `internal/source` continues to write under the cache root
only, which is not the working tree.

## Contracts

`internal/apply` exposes one function:

```go
// Run performs a plan's file operations against the repository at root.
// trees maps a source name to the path of its fetched tree.
func Run(root string, trees map[string]string, p *plan.Plan) error
```

There is no observation seam and no injection point. Ordering is verified by effect, not by
recording calls — see Test Strategy.

`internal/sync` exposes:

```go
type Options struct {
    Root      string // repository root; graft.toml and graft.lock live here
    CacheRoot string // fetch cache root
    DryRun    bool
}

func Run(o Options) (*Report, error)

type Report struct { ... }
func (r *Report) Lines(u *ui.UI) []string
```

Both are internal; graft publishes no Go API, so there is no external consumer and no
compatibility question. The **user-facing** contracts this change creates are the error
strings in `file-application` and `sync-execution` and the report layout in `sync-report`.
They are asserted by tests, and changing one is a deliberate contract change. They are also
added to SPEC.md's failure-mode table, which AGENTS.md calls the product for a CLI this size.

The complete set of new error strings:

```
source "<source>": no fetched tree
source "<source>": cannot read "<path>": <reason>
cannot write "<path>": it exists and is not a regular file
cannot write "<path>": "<ancestor>" is not a directory
cannot write "<path>": graft never writes inside ".git"
cannot write "<path>": graft never writes over "<name>"
cannot remove "<path>": it is not a regular file
cannot remove "<path>": "<ancestor>" is not a directory
cannot remove "<path>": graft never removes inside ".git"
cannot remove "<path>": graft never removes "<name>"
cannot open the repository root "<path>": <reason>
unknown argument "<argument>"
```

`graft.lock`'s format is unchanged: `version = 1`, the same layout `internal/lock` already
serializes, byte for byte. `graft.toml` and `catalog.yaml` are unchanged. No parser is
touched, so the contract gate on the format applies to the group that writes the lock: re-read
SPEC.md's `graft.lock` section and confirm the bytes still match the documented example.

## Persistence and Rollout

- **migration**: none. `graft.lock` keeps `version = 1`; a lock written by this change parses
  under the parser that already exists.
- **backfill**: none.
- **seeding**: none.
- **cache invalidation**: none. The fetch cache is content-addressed on the commit sha, and
  this change adds no reason to invalidate an entry.
- **index rebuild**: none.
- **authorization**: none. Private sources work exactly as far as the user's existing git
  credentials reach, which `internal/source` already arranges.
- **observability**: the sync report on the error stream. No telemetry, per ENGINEERING.md.
- **deployment**: none. graft is a binary a user runs; a consumer repository sitting on an
  older pin is a legitimate state.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| The consumer's working tree (filesystem) | real, under `t.TempDir()` | real, under `t.TempDir()` — `internal/apply` is a filesystem package; a fake would test the fake |
| `git` (subprocess) | real, against fixture repositories built in `t.TempDir()` | not used by `internal/apply`; real in `internal/sync` |
| Fixture source repositories | real git repositories created in `t.TempDir()`, with `user.name` and `user.email` set **on the repository** | same |
| The network | never reached: every fixture remote is a local filesystem path | same. "Unreachable" is a path that does not exist, not a firewall |
| The fetch cache | real. `internal/cli` calls `source.DefaultCacheRoot()`, which is the one place a test reaches it; the test points it at an **absolute** `t.TempDir()` with `t.Setenv("XDG_CACHE_HOME", …)` — `defaultCacheRoot` honours that variable only when absolute and otherwise falls through to the developer's real `~/.cache` | real, rooted at a `t.TempDir()` passed in as a value. `source.DefaultCacheRoot` is never called |
| `$HOME` and the real `~/.cache` | never read, per the row above | never read |
| The process working directory | real, via `t.Chdir` in the acceptance test only | not used; `internal/sync` takes its root as a value |
| Output streams | `bytes.Buffer` passed through `cli.Options` | `bytes.Buffer` through `ui.New` |
| Terminal detection | injected `IsTerminal` returning false | injected, both values |
| `NO_COLOR` | injected `Getenv` | injected |
| `internal/plan` | real | real. Plans are built by `plan.Build` from values, or hand-built where a scenario needs a plan `plan.Build` would refuse |
| Call-sequence observation inside `internal/apply` | **none — there is no seam.** Ordering is asserted through composed filesystem effects | none |

No task may invent a boundary this table omits. In particular there is no filesystem fake, no
git fake, no HTTP layer, and no call recorder inside `internal/apply`.

## Test Strategy

This change **takes the outer-loop acceptance test**. It is the first change to alter
end-to-end command behavior — until now `graft` had one flag and no command — and the risk it
carries is precisely wiring: a plan applied against the wrong root, a report on the wrong
stream, a cache root read from a global. Group 0 drives `cli.Main` with real arguments, a real
fixture repository, and a temp working directory, and it is the only test that can fail for a
reason none of the others can.

Tiers, from the project context: **unit** for logic reachable from values, **integration (fs)**
for `internal/apply` against a real temp directory with no git, **integration (git)** for
`internal/sync` against fixture repositories, and **acceptance** for `cli.Main` end to end.

Every test below runs under `go test -race`. Commands name a focused `-run` selector; the
whole suite is `task cover`.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| A plan's operations happen in the documented order | `TestRunOrdersOperations` — a prune and a write in one directory; only the documented order leaves `new.md` present and `old.md` gone with the directory intact | integration (fs) | real fs | `go test ./internal/apply/ -run Order` |
| An empty plan writes only the lock | `TestRunEmptyPlanWritesOnlyLock` | integration (fs) | real fs | `go test ./internal/apply/ -run Empty` |
| Nothing outside the plan is touched | `TestRunTouchesNothingOutsideThePlan` | integration (fs) | real fs | `go test ./internal/apply/ -run Outside` |
| A refused destination leaves every write unapplied | `TestPreflightRefusesBeforeAnyWrite/destination` | integration (fs) | real fs | `go test ./internal/apply/ -run Preflight` |
| A refused prune path leaves every write unapplied | `TestPreflightRefusesBeforeAnyWrite/prune` | integration (fs) | real fs | `go test ./internal/apply/ -run Preflight` |
| A missing source file is refused before anything is written | `TestPreflightRefusesBeforeAnyWrite/source` | integration (fs) | real fs | `go test ./internal/apply/ -run Preflight` |
| A file is copied into a directory that does not exist yet | `TestRunCreatesParentDirectories` | integration (fs) | real fs | `go test ./internal/apply/ -run Parent` |
| A hand-edited synced file is overwritten | `TestRunOverwritesHandEdits` | integration (fs) | real fs | `go test ./internal/apply/ -run Overwrite` |
| An executable source file lands as a non-executable one | `TestRunNormalizesMode/fresh` | integration (fs) | real fs | `go test ./internal/apply/ -run Mode` |
| An executable destination is made non-executable | `TestRunNormalizesMode/existing` | integration (fs) | real fs | `go test ./internal/apply/ -run Mode` |
| A source file that cannot be read fails the apply | `TestRunUnreadableSourceFile` | integration (fs) | real fs | `go test ./internal/apply/ -run Unreadable` |
| A write naming an unregistered source fails before it is attempted | `TestRunUnregisteredSource` | integration (fs) | real fs | `go test ./internal/apply/ -run Unregistered` |
| A foreign file in a shared destination survives every operation | `TestRunForeignFileSurvives` (three variants) | integration (fs) | real fs | `go test ./internal/apply/ -run Foreign` |
| Unrecorded files in a destination directory are never enumerated | `TestRunNeverEnumeratesADirectory` | integration (fs) | real fs | `go test ./internal/apply/ -run Enumerate` |
| A prune path that is already gone is not an error | `TestRunPruneMissingPath` | integration (fs) | real fs | `go test ./internal/apply/ -run PruneMissing` |
| A prune path that is a directory is refused | `TestRunPruneDirectoryRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run PruneDirectory` |
| A prune path that is a symlink is refused | `TestRunPruneSymlinkRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run PruneSymlink` |
| A prune path under a symlinked parent is refused | `TestRunPruneSymlinkedParentRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run PruneSymlinkedParent` |
| A prune path whose parent is a regular file is refused | `TestRunPruneFileParentRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run PruneFileParent` |
| An emptied directory chain is removed | `TestRunRemovesEmptiedDirectories` | integration (fs) | real fs | `go test ./internal/apply/ -run Emptied` |
| A directory still holding a foreign file is kept | `TestRunKeepsNonEmptyDirectory` | integration (fs) | real fs | `go test ./internal/apply/ -run KeepsNonEmpty` |
| A directory a write still fills is kept | `TestRunOrdersOperations` (the same composed effect) | integration (fs) | real fs | `go test ./internal/apply/ -run Order` |
| An unrelated empty directory is left alone | `TestRunLeavesUnrelatedEmptyDirectory` | integration (fs) | real fs | `go test ./internal/apply/ -run Unrelated` |
| A symlinked ancestor of a pruned path is not removed | `TestRunKeepsSymlinkedAncestor` | integration (fs) | real fs, hand-built plan | `go test ./internal/apply/ -run SymlinkedAncestor` |
| A pruned path at the repository root removes nothing | `TestRunPruneAtRoot` | integration (fs) | real fs | `go test ./internal/apply/ -run PruneAtRoot` |
| A destination inside .git is refused | `TestRunReservedPaths/write-git` | integration (fs) | real fs, hand-built plan | `go test ./internal/apply/ -run Reserved` |
| A prune path inside .git is refused | `TestRunReservedPaths/prune-git` | integration (fs) | real fs, hand-built plan | `go test ./internal/apply/ -run Reserved` |
| A destination of graft.toml or graft.lock is refused | `TestRunReservedPaths/own-files` | integration (fs) | real fs, hand-built plan | `go test ./internal/apply/ -run Reserved` |
| A path merely beginning with .git is not inside it | `TestRunReservedPaths/dotgithub` | integration (fs) | real fs | `go test ./internal/apply/ -run Reserved` |
| A write that fails mid-flight leaves the previous lock in place | `TestRunMidFlightFailureKeepsPreviousLock` — the destination directory is made read-only after the pre-flight pass by a plan whose first write creates it | integration (fs) | real fs | `go test ./internal/apply/ -run MidFlight` |
| An unchanged sync still writes the lock | `TestRunTwiceWritesIdenticalLock` | integration (fs) | real fs | `go test ./internal/apply/ -run IdenticalLock` |
| The lock that is written is the plan's lock | `TestRunWritesThePlansLock` | integration (fs) | real fs, `lock.Parse` | `go test ./internal/apply/ -run PlansLock` |
| A directory at a destination is refused | `TestRunDestinationDirectoryRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run DestDirectory` |
| A symlink at a destination is refused rather than followed | `TestRunDestinationSymlinkRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run DestSymlink` |
| A destination under a symlinked parent is refused | `TestRunDestinationSymlinkedParentRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run DestSymlinkedParent` |
| A destination whose parent is a regular file is named | `TestRunDestinationFileParentRefused` | integration (fs) | real fs | `go test ./internal/apply/ -run DestFileParent` |
| A destination escaping the root fails rather than being written | `TestRunDestinationEscapeRefused` | integration (fs) | real fs, hand-built plan | `go test ./internal/apply/ -run Escape` |
| A source path escaping the fetched tree fails rather than being read | `TestRunSourceEscapeRefused` | integration (fs) | real fs, hand-built plan | `go test ./internal/apply/ -run SourceEscape` |
| A missing repository root is named | `TestRunMissingRoot` | integration (fs) | real fs | `go test ./internal/apply/ -run MissingRoot` |
| A first sync installs what the manifest asks for | `TestRunFirstSync` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run FirstSync` |
| A second sync changes nothing | `TestRunIsIdempotent` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run Idempotent` |
| A missing manifest is refused before anything else happens | `TestRunMissingManifest` | integration (git) | real fs | `go test ./internal/sync/ -run MissingManifest` |
| A source dropped from the manifest is pruned without being fetched | `TestRunPrunesDroppedSource` | integration (git) | real fs, empty cache | `go test ./internal/sync/ -run DroppedSource` |
| A moved branch does not move the pin | `TestRunNeverReResolves` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run ReResolve` |
| A source with no lock entry is resolved once and recorded | `TestRunResolvesNewSource` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run NewSource` |
| A rev that no longer exists fails, naming the rev and the source | `TestRunRevNotFound` | integration (git) | real git, real fs | `go test ./internal/sync/ -run RevNotFound` |
| There is no flag to make sync re-resolve or refuse to overwrite | `TestSyncRejectsForceAndFrozen` | acceptance | `cli.Main`, buffers | `go test ./internal/cli/ -run ForceFrozen` |
| A bumped rev in the manifest points at graft update | `TestRunPinDisagreement` | integration (git) | real fs | `go test ./internal/sync/ -run PinDisagreement` |
| The pin check precedes the network | `TestRunPinCheckPrecedesFetch` | integration (git) | real fs, unreachable remote, empty cache | `go test ./internal/sync/ -run PinCheckPrecedes` |
| A source without a catalog is not graftable | `TestRunNoCatalog` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run NoCatalog` |
| A selector matching nothing fails the run | `TestRunSelectorMatchesNothing` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run SelectorMatches` |
| Two items resolving to one path fail the run before any of it is written | `TestRunCollisionLeavesTreeUntouched` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run Collision` |
| A cache miss with no network names what it needed | `TestRunCacheMissOffline` | integration (git) | real fs, deleted remote, empty cache | `go test ./internal/sync/ -run CacheMiss` |
| A cache hit with no network succeeds | `TestRunCacheHitOffline` | integration (git) | real fs, deleted remote, warm cache | `go test ./internal/sync/ -run CacheHit` |
| An invalid catalog fails the run | `TestRunInvalidCatalog` (two variants) | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run InvalidCatalog` |
| A destination outside the repository root fails the run | `TestRunDestinationEscapesRepoRoot` | integration (git) | real git, real fs, temp cache, real `plan.Build` | `go test ./internal/sync/ -run EscapesRepoRoot` |
| A source's listing climbing out of its item fails the run | `TestRunListingClimbsOut` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run ClimbsOut` |
| A dry run of a first sync creates nothing | `TestRunDryRunCreatesNothing` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run DryRunCreates` |
| A dry run of a removal deletes nothing | `TestRunDryRunDeletesNothing` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run DryRunDeletes` |
| A dry run of a failing plan fails the same way | `TestRunDryRunFailsAlike` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run DryRunFails` |
| An argument to sync is a usage error | `TestSyncTakesNoArguments` | acceptance | `cli.Main`, buffers | `go test ./internal/cli/ -run NoArguments` |
| A successful sync leaves standard output byte-empty | `TestSyncStdoutEmptyOnSuccess` | acceptance | `cli.Main`, real git, temp cache, `t.Chdir` | `go test ./internal/cli/ -run StdoutEmptyOnSuccess` |
| A failing sync leaves standard output byte-empty | `TestSyncStdoutEmptyOnFailure` | acceptance | `cli.Main`, buffers, `t.Chdir` | `go test ./internal/cli/ -run StdoutEmptyOnFailure` |
| A repeated sync reports nothing | `TestReportUpToDate` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run UpToDate` |
| A sync that only rewrites identical files is still nothing to do | `TestReportUpToDateAfterHandDeletion` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run UpToDateAfterHand` |
| A first sync is never nothing to do | `TestReportFirstSyncIsNotUpToDate` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run FirstSyncIsNot` |
| A dry run with nothing to do reports nothing | `TestReportDryRunUpToDate` | integration (git) | real git, real fs, temp cache | `go test ./internal/sync/ -run DryRunUpToDate` |
| A version bump shows both revs and both shas | `TestReportHeaderBothMoved` | unit | none — a `Report` built from two locks | `go test ./internal/sync/ -run HeaderBoth` |
| A newly added source shows one rev and one sha | `TestReportHeaderNewSource` | unit | none | `go test ./internal/sync/ -run HeaderNew` |
| A branch pin whose sha moved shows both shas and one rev | `TestReportHeaderShaOnly` | unit | none | `go test ./internal/sync/ -run HeaderShaOnly` |
| Two sources are separated by a blank line | `TestReportSourceSeparation` | unit | none | `go test ./internal/sync/ -run Separation` |
| An updated and a removed item align in one block | `TestReportAlignment` — pins SPEC.md's example byte for byte, trailing whitespace included | unit | none | `go test ./internal/sync/ -run Alignment` |
| A newly installed item is added | `TestReportVerbAdded` | unit | none | `go test ./internal/sync/ -run VerbAdded` |
| An item dropped from install says so | `TestReportNoteNoLongerInstalled` | unit | none | `go test ./internal/sync/ -run NoteInstalled` |
| An item of a source dropped from the manifest says so | `TestReportNoteSourceRemoved` | unit | none | `go test ./internal/sync/ -run NoteSourceRemoved` |
| An unchanged item under a moved pin is still updated | `TestReportVerbUpdatedOnMovedPin` | unit | none | `go test ./internal/sync/ -run MovedPin` |
| An unchanged item under an unchanged pin produces no line | `TestReportSilentItem` | unit | none | `go test ./internal/sync/ -run SilentItem` |
| The summary counts every planned write | `TestReportSummaryCounts` | unit | none | `go test ./internal/sync/ -run SummaryCounts` |
| A sync that only removes still reports zero written | `TestReportSummaryZeroWritten` | unit | none | `go test ./internal/sync/ -run SummaryZero` |
| A single file is reported in the singular | `TestReportSummarySingular` | unit | none | `go test ./internal/sync/ -run SummarySingular` |
| A dry run says nothing was written | `TestReportSummaryDryRun` | unit | none | `go test ./internal/sync/ -run SummaryDryRun` |
| The report never reaches standard output | `TestSyncReportGoesToStderr` | acceptance | `cli.Main`, real git, temp cache, `t.Chdir` | `go test ./internal/cli/ -run ReportGoesTo` |
| With colour off the report is plain text | `TestReportPlainWithColorOff` | unit | `ui.New(..., false)` | `go test ./internal/sync/ -run PlainWithColor` |
| With colour on only the verb and the note are styled | `TestReportStyledWithColorOn` | unit | `ui.New(..., true)` | `go test ./internal/sync/ -run StyledWithColor` |
| No arguments prints help and succeeds | existing `internal/cli` test, re-run unchanged | acceptance | buffers | `go test ./internal/cli/` |
| Help lists the commands graft has | `TestHelpListsSync` | acceptance | buffers | `go test ./internal/cli/ -run HelpLists` |
| `--help` prints the same text as no arguments at all | existing `internal/cli` test, re-run unchanged | acceptance | buffers | `go test ./internal/cli/` |
| `help` is not a command | `TestHelpIsNotACommand` | acceptance | buffers | `go test ./internal/cli/ -run HelpIsNot` |
| `help sync` is not a command either | `TestHelpIsNotACommand/with-argument` | acceptance | buffers | `go test ./internal/cli/ -run HelpIsNot` |

## Decisions

**`internal/sync` exists rather than putting the orchestration in `internal/cli`.** Two
reasons. The sequence in `sync-execution` is the thing most worth testing directly, and
driving it through cobra to do so would make every one of those tests an argument-parsing test
as well. And the orchestration needs its repository root and its cache root injected — a test
that reached `source.DefaultCacheRoot()` would write into the developer's real `~/.cache/graft`
— while `cli.Options` deliberately carries only what a process gives a command. `internal/cli`
is left with the few lines that turn a working directory and a default cache root into
`sync.Options`, and the acceptance test is the one place `DefaultCacheRoot` is reached, with
`XDG_CACHE_HOME` pointed at an absolute temp directory.

The package is named `sync`, which shadows the standard library's `sync` for any file that
imports both. No file in graft does, and no file in graft is likely to: the tool is a
single-threaded copy of files. The alternative names — `syncer`, `syncrun`, `vendoring` — all
name the package after the collision rather than after the job.

**`apply.Run` takes the fetched trees as a map from source name.** A `plan.Write` names its
source, not a path outside the repository, because `internal/plan` is pure and knows nothing
about where a tree was fetched. Passing the map keeps that true and makes the apply testable
with no git at all: an `internal/apply` test builds a "fetched tree" with `os.WriteFile`.

**A pre-flight pass runs before the first write.** Every refusal this change specifies — a
destination that is not a regular file, an ancestor that is not a directory, a prune path that
is not a regular file, a reserved path, an unregistered source, a source file that is not
there — is decidable from `Lstat` alone. Discovering one halfway through would guarantee a
partial apply, and the failure would repeat identically on every subsequent sync because the
lock is never written; the user would face a stuck sync and a tree in a state graft cannot
describe. The pass is not a lock on the filesystem: a condition can change between the check
and the write, and that case is allowed to fail mid-flight. It removes the failures graft can
see coming, which is all a pre-flight pass can ever do.

**Every ancestor of a destination or a prune path must be a directory, checked without
following it.** `os.Root` refuses a path that leaves the root but deliberately follows a
symlink that stays inside it — its documentation says so — so the root alone is not the floor
under the prune guarantee it looks like. A lock claiming `vendor/x.md` where `vendor` has since
become a link to `docs` would delete `docs/x.md`: a path no lock claims, removed by graft,
silently. The write side has the same shape, and there the damage is to the invariant rather
than to a file: a write through a symlinked parent lands somewhere `graft.lock` does not name,
so the file an item places does not stay inside that item's destination and a later prune aims
wherever the link points that day. Refusing is strict — a repository that symlinks `.claude`
elsewhere cannot sync into it — and the error names the offending ancestor, which is what makes
it actionable.

**Emptied-directory removal skips a candidate that is not a directory.** The reasoning that a
bare `Remove` "fails harmlessly on a non-empty directory" is true of directories and false of
symlinks: unlinking a symlink succeeds however full its target is. Without the check, a user's
`vendor -> docs` link in the ancestry of a pruned path is deleted — again a path absent from
`graft.lock`. A removal that fails for any other reason is ignored rather than fatal: it
happens after the prunes and before the lock write, so failing there would strand the sync in
the state it can least explain.

**Empty-directory removal considers only the ancestors of pruned paths.** "Remove directories
left empty" is SPEC.md's wording, and a walk of the whole tree looking for empty directories
would be graft deleting something it has no record of — the same mistake as scanning a
destination directory for files to prune.

**An existing destination is removed and recreated rather than truncated.** The permission
argument to a create-and-truncate open applies only when the file is created, so overwriting a
file a user once ran `chmod +x` on would leave the executable bit in place while graft
replaced its contents with source-controlled bytes. That is the one hole in "a source cannot
produce an executable file in a consumer's repository", and remove-then-create with `O_EXCL`
closes it in one line.

**Written files take mode `0644`, whatever the source's mode.** graft executes nothing a
source provides, and the cheapest way to keep that from eroding is that a source cannot produce
an executable file at all. It is a real restriction — a source shipping a shell hook gets a
non-executable one — and it is the right side of ENGINEERING.md's security note. A source that
needs an executable bit can ask for the feature, and it will be a decision rather than an
accident of `io.Copy`.

**`.git/`, `graft.toml`, and `graft.lock` are refused as destinations and as prune paths.**
SPEC.md's entire mitigation for an untrusted source is "every sync's effect is a git diff", and
a file placed inside `.git/` is invisible to it — untracked, so `git status` says nothing — while
`.git/config` alone converts placing a file into running a program on the user's next git
command, through `core.fsmonitor`, `core.sshCommand`, or an alias. Nothing upstream catches it:
`internal/plan` refuses a destination that *escapes* the repository root, and `.git/config` does
not escape it; kinds are arbitrary and no rule anywhere constrains what a `to` may name.

The refusal lives in `internal/apply` rather than `internal/plan` for two reasons. It is a floor
under the writer, so it holds whichever planner produced the plan — the same argument the
`os.Root` already makes. And `destination-computation` is an archived spec whose rule is about
escaping the root; a reserved path *inside* the root is a different rule with a different
reason, and bolting it onto that requirement would blur both. The cost is that `--dry-run`,
which stops after the plan, does not surface it; `sync-execution` says so rather than letting a
reader infer that a clean dry run guarantees a clean sync.

**The report is built from the two locks and the plan, never from the tree.** The verb for an
item is decided by whether the old lock had it, the new lock has it, and whether the source's
sha or the item's file list moved. That is the only honest answer available: every planned
file is written on every sync, so "updated" cannot mean "the bytes changed" and no comparison
in this package could make it mean that.

**`up to date` is `bytes.Equal(lock.Marshal(old), lock.Marshal(new)) && len(prune) == 0`.**
One predicate over the two artifacts a reader would diff, rather than a conjunction of the
conditions that produce report lines. It also covers the case those conditions miss: a source
whose `git` value changed in `graft.toml` with the same rev produces a different lock and no
item lines, and must not be reported as nothing to do. Its known cost is the reverse case — a
user who deleted installed files by hand and re-syncs is told `up to date` while `git status`
shows six restored files — and `sync-report` states that rather than leaving the next reader to
file it as a bug.

**`--dry-run` still fetches.** There is no plan without a catalog and no catalog without a
fetch. SPEC.md's promise is that `--dry-run` touches nothing, "including creating no
directories" — a promise about the working tree. The fetch cache is not the working tree, and
`internal/source` writes only under the cache root it is given.

**`graft help` is refused in `Main`, before `Execute`.** `command-invocation` left the
transition open and named this change as its owner, and refusing follows the same spec's
`version` rule and its completion rule. The mechanism matters: cobra's `SetHelpCommand` adds
whatever placeholder it is given to the root as a real command, so `graft <placeholder>` would
become a working, undocumented command — trading one leak for another. Refusing the literal
argument `help` in `Main`, beside the existing `__complete` and `__completeNoDesc` guards, adds
no command name at all and reuses the mechanism this codebase already chose for exactly this
problem.

**`sync` takes its own argument validator.** `cobra.NoArgs` produces `unknown command "x" for
"graft sync"`, which is a second error format inside a tool whose error format is the
contract. `unknown argument "x"` is graft's, and it is a usage error, so it earns the hint
line.

**`manifest.Filename` and `lock.Filename` become exported constants.** Two packages now name
each file, and a string literal in two places is a rename waiting to go half-done.

**SPEC.md gains rows rather than being left behind.** AGENTS.md calls the failure-mode table
the product for a CLI this small, and this change adds eleven user-facing error strings and one
invariant. Updating it is an operational task in this change, not a follow-up.

## Risks / Trade-offs

- **[The applier deletes something a user wrote.]** → The prune set comes from
  `internal/plan`, which derives it from `graft.lock` alone; `internal/apply` never scans a
  directory and never extends the set; a prune path that is not a regular file, or whose
  ancestry contains anything that is not a directory, is refused rather than removed; a
  directory-removal candidate that is not a directory is skipped; and the foreign-file
  guarantee gets its own tests beside a synced file, in the ancestry of a pruned path, and
  against a directory holding ten unrecorded files.
- **[A partial apply leaves files outside the lock.]** → Reduced, not eliminated. The
  pre-flight pass moves every specified refusal ahead of the first write, so what remains is an
  I/O failure mid-write. Those files are on disk and the previous lock does not claim them. If
  the next resolution still produces them they are overwritten; **if it does not — because the
  user dropped that source in response to the failing sync — they are orphaned, and graft can
  never reach them again.** `git status` is the only recovery, which is also how the user finds
  them. Stated here rather than described as benign.
- **[A repository that symlinks a destination directory can no longer sync.]** → Accepted.
  Writing through the link would put the file at a path `graft.lock` does not name and aim
  every later prune at whatever the link resolves to then. The error names the ancestor, so the
  user can replace the link with a real directory or override the destination in `graft.toml`.
- **[The `.git` refusal is a rule SPEC.md did not have.]** → It is added to SPEC.md's
  Invariants by this change rather than being left as undocumented behavior, and its scenario
  pins that `.github/` and `.gitignore` are unaffected — the rule is on the first path segment,
  not a prefix match.
- **[`internal/sync` grows into a god package.]** → It holds the sequence and the report and
  nothing else. Every decision it could make is already made somewhere: expansion in
  `internal/catalog`, destinations and pruning in `internal/plan`, fetching in
  `internal/source`, writing in `internal/apply`.
- **[The report format drifts from SPEC.md.]** → The alignment scenario pins the exact bytes
  of SPEC.md's own example, trailing whitespace included.
- **[A test writes into the developer's real cache or home directory.]** → No unit or
  integration test calls `source.DefaultCacheRoot`; the acceptance test does, and points
  `XDG_CACHE_HOME` at an absolute `t.TempDir()`. `defaultCacheRoot` honours that variable only
  when absolute, so the temp directory being absolute is load-bearing rather than incidental.

## Migration Plan

None is needed. `graft` has never written a file, so there is no state to migrate and no
older behavior to preserve. `graft.lock` keeps `version = 1`, and a lock this change writes is
readable by the parser that already exists. Rollback is reverting the commits: the packages
that existed before this change are untouched apart from two additive constants and the
subcommand registration.

## Open Questions

None remain. Four questions were resolved rather than deferred and are recorded in Decisions
above: what `graft help` does now that a subcommand exists, whether `--dry-run` belongs to this
change, where the `.git` refusal lives, and how `up to date` is defined. Two failure paths are
deliberately left unspecified and are recorded in planning-review.md as accepted: `os.Getwd()`
failing and `source.DefaultCacheRoot()` failing, both in `internal/cli`, both surfacing their
own error through the existing format with no working tree touched.
