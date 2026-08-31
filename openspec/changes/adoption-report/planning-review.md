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
