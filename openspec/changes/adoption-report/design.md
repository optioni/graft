## Context

A report that says `added` about a file it overwrote is worse than silence: a reader who
checks the report and moves on has been told the wrong thing. The case that surfaced it is a
repository that had been hand-editing a synced file for months; the first `graft add` there
would replace it and print `added`.

Nothing about the write changes. Synced files are derived artifacts with `node_modules`
semantics — always overwritten, never merged — and adoption is how any repository starts
using graft. What changes is that graft says so.

## Goals / Non-Goals

**Goals:**
- A verb that is true, a note that says what happened, and a count in the summary.
- The observation costs one read of a destination graft is about to write anyway.
- `internal/plan` stays pure.

**Non-Goals:**
- A prompt, a `--force`, a backup, or a refusal.
- Showing destinations before writing under the non-interactive `add` — SPEC.md's sentence
  claiming that is corrected instead, to describe `--list` and the picker, which do.
- Any content comparison that survives the run: no hashes in the lock, no `unchanged` verb.

## Boundaries

| Piece | Package | Pattern it follows |
|---|---|---|
| Was this destination already ours? | `internal/plan` | The prune set — a set operation over the lock it is handed, no filesystem |
| Did it exist, and did it differ? | `internal/apply` | The pre-flight checks — the only package that may look at the tree |
| Verb, note, count | `internal/sync` | The existing report, which already carries a per-item note |

The flow is one direction: plan marks, apply observes, sync reports. No package asks another
for a fact it could have derived, and no filesystem question moves out of `internal/apply`.

## Contracts

- `plan.Write` gains `Claimed bool`.
- `apply.Run` returns `([]string, error)` — the destinations it replaced, ordered as the plan
  ordered its writes. Its only caller is `internal/sync`.
- `sync.ItemReport` gains nothing: it already has `Verb` and `Note`. `sync.Report` gains
  `Replaced int` for the summary.

`graft.lock` and the `graft list --json` document are untouched; both version numbers stay.
The one externally visible change is the report's own text, which is why the proposal marks
it **BREAKING**.

## Persistence and Rollout

Migration, backfill, seeding, cache invalidation, index rebuild, authorization: none.
Observability: the change *is* observability. Deployment: none beyond the next release.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem (working tree) | real, under `t.TempDir()` | real for `internal/apply`; absent for `internal/plan` and the renderer, which are pure |
| `git` binary | real, against a fixture repo | not reached by `plan`, `apply`, or the renderer |
| Fetch cache | real, rooted in `t.TempDir()` | not reached |
| Output streams | `bytes.Buffer` through `cli.Options` | a `ui.UI` over discarded buffers |
| Clock, randomness, network | not used | not used |

## Test Strategy

Three tiers, each holding the question it can answer: the renderer's text as a unit test over
a `Report` value, the claimed flag as a unit test over `plan.Build`, the filesystem comparison
as an integration test over a real directory. One acceptance test covers the end-to-end shape,
because "the verb a user sees when they overwrite their own file" is the whole point and it
crosses every layer.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| An updated and a removed item align in one block | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| An item that replaced a hand-written file is adopted | Report values rendered to lines, asserted as text, plus an end-to-end `graft add` over a fixture repo with a hand-written file at a destination | acceptance | real filesystem, real `git`, fixture remote | `go test ./internal/cli/ -run TestGraftAdd` |
| An updated item that replaced a hand-written file keeps its verb | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| A destination the lock already claimed is not a replacement | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| Identical bytes replace nothing | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| A dry run reports no adoption | Report values rendered to lines, asserted as text, plus an end-to-end `graft add` over a fixture repo with a hand-written file at a destination | acceptance | real filesystem, real `git`, fixture remote | `go test ./internal/cli/ -run TestGraftAdd` |
| A newly installed item is added | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| An item dropped from install says so | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| An item of a source dropped from the manifest says so | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| An unchanged item under a moved pin is still updated | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| An unchanged item under an unchanged pin produces no line | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| The summary names how many files replaced something | Report values rendered to lines, asserted as text, plus an end-to-end `graft add` over a fixture repo with a hand-written file at a destination | acceptance | real filesystem, real `git`, fixture remote | `go test ./internal/cli/ -run TestGraftAdd` |
| A sync that replaced nothing carries no parenthetical | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| The summary counts every planned write | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| A sync that only removes still reports zero written | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| A single file is reported in the singular | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| A dry run says nothing was written | Report values rendered to lines, asserted as text | unit | none — pure over a Report value | `go test ./internal/sync/ -run TestReport` |
| A path the lock claims is marked claimed | plan.Build over in-memory inputs, asserting the flag per write | unit | none — pure over values | `go test ./internal/plan/` |
| A path claimed under a different source or item still counts as claimed | plan.Build over in-memory inputs, asserting the flag per write | unit | none — pure over values | `go test ./internal/plan/` |
| A path no lock claims is not marked | plan.Build over in-memory inputs, asserting the flag per write | unit | none — pure over values | `go test ./internal/plan/` |
| An empty lock claims nothing | plan.Build over in-memory inputs, asserting the flag per write | unit | none — pure over values | `go test ./internal/plan/` |
| A hand-written file at a destination is reported | apply.Run against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| A claimed destination is not reported | apply.Run against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| Identical bytes are not a replacement | apply.Run against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| An absent destination is not a replacement | apply.Run against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| A failed apply reports nothing | apply.Run against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |

<!-- 26 rows, one per spec scenario -->
## Decisions

**The verb is corrected only where it is false.** `added` becomes `adopted`; `updated` stays
`updated` and takes the note. An item already in the lock that gains a file at an occupied
path has genuinely been updated — inventing a fourth verb for the combination would put a word
in the report that describes an intersection rather than an event.
*Alternative considered:* the note alone, with no verb change. Rejected: the reader who skims
verbs is exactly the reader this is for, and `added` is the word that misleads them.

**The comparison is against the bytes being written, not a hash.** `internal/apply` already
holds the source file's content when it writes; comparing is one read of the destination and
no bookkeeping that outlives the run. Hashes in the lock were refused by SPEC.md for the same
reason they would be refused here — git already stores this.
*Alternative considered:* comparing only existence, not content. Rejected: a destination whose
bytes already match replaced nothing, and reporting it would make the count noise.

**`--dry-run` reports no adoption.** It reaches no write, and inventing a read-only pass in
the applier so a dry run could answer would put a second filesystem path into the package
whose whole discipline is that there is one. The spec says so plainly rather than leaving a
reader to notice.

**`plan` marks, `apply` looks.** The previous lock is a planning input and the filesystem is
the applier's. Passing the lock into the applier so it could ask both questions would give two
packages a record of what graft owns, which is one more than the number that can stay in
agreement.

## Risks / Trade-offs

[One extra read per unclaimed destination] → Bounded by the plan's own size, on files graft is
about to overwrite anyway, and skipped entirely for claimed paths — which is every file of an
established consumer. A first sync pays it; a steady-state sync does not.

[`adopted` reads as approval rather than as a warning] → The note carries the meaning:
`replaced existing content`. The verb column is a category, and the note is where this report
has always put the sentence that explains a line.

[A reader assumes the count means data was lost] → It means graft wrote over something. `git
diff` says what, and the summary already points there. Nothing tracked is lost; something
untracked is, and no report can undo that.

## Migration Plan

None. Additive to two internal signatures and to the report's text. Rollback is reverting the
commits; nothing on disk changes shape.

## Open Questions

None.
