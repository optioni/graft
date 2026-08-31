## Reviewed Artifacts

- `proposal.md`
- `specs/sync-report/spec.md` (2 MODIFIED requirements, 15 scenarios)
- `specs/sync-plan/spec.md` (1 ADDED requirement, 4 scenarios)
- `specs/file-application/spec.md` (1 ADDED requirement, 5 scenarios)
- `design.md`
- `tasks.md` (8 groups, 30 tasks)

Not delegated, on the same standing instruction as the four changes before it.

## Reviewed Against

- The live behavior, reproduced rather than assumed: a temp repository with a committed local
  edit at a destination, then `graft add`, which printed `added agent:reviewer 1 file` and
  replaced the file.
- `openspec/specs/sync-report/spec.md`, whose two requirements are reproduced whole rather
  than summarised — both MODIFIED blocks were built by script from the live text so no
  scenario could be dropped, and `openspec validate --strict` confirms none was.
- `internal/sync/render.go`, which already pads a verb column and already renders a per-item
  note, so this needs no renderer change beyond the summary line.
- `internal/plan/build.go`, which already receives the previous lock and computes the prune
  set from it.
- SPEC.md's Output section and its destination sentence.

## Gaps Found and Fixed

### 1. The first shape put a filesystem read in `internal/plan` — CRITICAL

The obvious implementation asks "does this destination already exist with different content"
where the destinations are computed. `internal/plan` may not look at a filesystem, and a test
needing a real directory to exercise plan logic is the repository's own stated signal that the
boundary has moved.

**Fixed**: split in two. `plan` answers the part that is a set operation over the lock it
already holds (`Claimed`), and `apply` — the only package permitted to look — answers the part
that requires looking. The flow is one direction and no fact is derived twice.

### 2. `--dry-run` cannot answer, and the first draft did not say so — WARNING

A dry run stops before the applier, so it observes no replacement. Left unstated, the first
person to run `graft sync --dry-run` over a repository full of hand-written files would read
`added` and conclude nothing would be replaced.

**Fixed**: stated in the sync-report requirement and given its own scenario, and named in
design.md as a decision with the alternative — a read-only pass inside the applier — and the
reason it was refused.

### 3. `updated` items were going to be relabelled too — WARNING

An item already in the lock that gains a file at an occupied path would have become `adopted`
under the first rule, which is false in the other direction: it *was* updated. The verb is now
corrected only where it would otherwise say the opposite of what happened, and the note
carries the fact in both cases.

### 4. Identical bytes would have counted as a replacement — WARNING

Existence alone is the cheap test, and it is wrong: a destination whose bytes already equal
what is being written replaced nothing, and counting it would put a number in the summary that
a reader learns to ignore. All three conditions — unclaimed, exists, differs — are now
required, each with its own scenario.

## No Remaining Implementation-Blocking Gaps

Verified rather than assumed:

- **The renderer already supports this.** `itemLines` pads the verb column to the widest verb
  in the block and already emits a note after a padded count, so `adopted` and its note need
  no new rendering machinery — only the summary line changes.
- **`plan.Build` already has the previous lock**, which is what makes `Claimed` a pure field
  rather than a new input.
- **`apply.Run`'s only caller is `internal/sync`**, so returning a value alongside its error
  changes one call site.
- **No format moves.** `graft.lock`'s `version` stays `1` and the `graft list --json`
  document's stays `2`; neither gains a key.
- **Every one of the 26 spec scenarios has a matrix row**, checked by comparing the two sets.
- **A report with nothing to say must be byte-identical to today's**, which is task 3.4 rather
  than an assumption — this is a change to output that every existing test asserts.

## Deferred Non-Blocking Notes

- **SPEC.md's "`add` shows the destination for every item before writing" is corrected rather
  than implemented.** The owner chose the smaller fix knowingly; `--list` and the picker do
  show destinations first, and the sentence will say that.
- **Nothing is reported for a file that existed and was *not* written** — a stray file in a
  vendored directory is invisible here, as it is to the prune set, and deliberately so: graft
  never touches a path no lock claims.

## Implementation Review (group 6)

Not delegated, on the same standing instruction.

### Found while implementing

1. **An existing determinism test broke, and it broke for the right reason — WARNING, fixed
   into a stronger test.** `TestDeterminism_AnIdempotentReplanPrunesNothing` compared two
   plans' `Writes` with `slices.Equal`, which now includes `Claimed`. The first plan is built
   against an empty lock and the second against the first's own lock, so the field is
   *supposed* to differ: nothing claimed, then everything claimed. Comparing whole structs
   would have forced the field to be meaningless. The test now compares writes on identity —
   which file moves where — and separately asserts the transition in both directions, which
   says more than the original did.

2. **`apply.Run` returning a value touched 55 test call sites.** All one shape
   (`err := apply.Run(`), so the change was mechanical, plus one call nested inside an
   assertion helper that had to be unwrapped. Worth noting only because a signature change
   that looks small in the diff of the package can be large in the diff of its tests, and the
   count is the reason a pointer-out parameter is tempting — it was refused because the spec
   says the applier *reports*, and a return value is what that means.

3. **A dry run needed its own test, not just its own summary.** The summary was covered, but
   the scenario also asserts the item line stays `added` and the file on disk is untouched.
   Added at `internal/sync` against a fixture, which is the tier that can see both.

### Verified rather than assumed

- **A report with nothing replaced is byte-identical to what it was before this change.**
  Asserted directly — the summary with `Replaced == 0` and a full render compared before and
  after `adopt(p, nil)`. Every existing report test still passes unchanged, which is the
  broader form of the same claim.
- **The real binary prints what the design promised**, checked against a throwaway repository
  holding a hand-written `.claude/agents/reviewer.md`:

  ```
    adopted  agent:reviewer  1 file  replaced existing content
    added    schema:tdd      1 file

  2 files written (1 replaced existing content), 0 removed - review with `git diff`
  ```

- **Adoption is a one-time event.** A second sync over the same repository reports none: the
  lock claims the path by then, so the write is graft rewriting its own file. Asserted end to
  end.
- **The prune set is untouched.** Every existing prune and empty-directory test passes, and
  nothing in this change reads a directory or extends what may be deleted.
- **`task lint` clean, `task test` green under `-race`, 93.6% coverage, `task build` builds.**

### Deferred, with reasons

- **A file sitting in a vendored directory that no write targets is invisible here**, exactly
  as it is to the prune set. graft never touches a path no lock claims, and reporting files it
  is not writing would be the tree-scanning this design refuses everywhere else.
- **`--dry-run` still says `added` over a file it would replace.** Stated in the spec and
  given a scenario rather than fixed: answering would mean a read-only filesystem pass inside
  the applier, which is a second path through the one package whose discipline is that there
  is one.
