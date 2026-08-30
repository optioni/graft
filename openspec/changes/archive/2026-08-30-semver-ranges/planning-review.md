## Reviewed Artifacts

- `proposal.md`
- `design.md`
- `tasks.md`
- `specs/rev-ranges/spec.md` (new capability)
- `specs/rev-resolution/spec.md`
- `specs/lock-format/spec.md`
- `specs/update-execution/spec.md`
- `specs/sync-report/spec.md`
- `specs/list-execution/spec.md`
- `specs/sync-execution/spec.md` (added during this review)
- `specs/sync-plan/spec.md` (added during this review)

The finding pass was delegated to three reviewers that did not write the package, each given
one slice — capability and scenario coverage, design and verification, task alignment — and
told to report findings only. They edited nothing. This session merged the findings, verified
every structural claim against the code before accepting it, repaired the owning artifact, and
wrote this log.

## Reviewed Against

- This repository HEAD: `9ebc3b66f38b876417767e6f4c2572430e5e15b0`
- Sibling repositories: Not applicable — this change touches no other repository. `origin` for
  this one is empty and nothing is pushed.
- Working tree: clean apart from `openspec/changes/semver-ranges/`, this change's own untracked
  planning files, intentionally included.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | design.md, tasks.md | `internal/lock` cannot import `internal/source`: `source/listing.go` → `plan`, `plan/build.go` → `lock`, so `lock → source` closes a cycle. The design's central claim — one range predicate asked by both — was unbuildable, and would have died at group 5's GREEN with the second copy of the predicate as the fix reached for under pressure. | `IsRange` moved to a new leaf package `internal/rev`, importing only `strings`, following the `internal/itemid` precedent `lock` already depends on. Cycle recorded so the reason survives. | design.md → Boundaries, Decisions; tasks.md group 2, 4.6, 5.4 |
| CRITICAL | design.md, tasks.md | The matched tag could not reach `graft.lock`. `plan.Input` has no such field and `internal/plan/build.go:176` is the only place a `lock.Source` is constructed, so every range would produce `matched = ""` — which group 5's own validation then refuses. design.md listed `internal/plan` as changing "Nothing", and no task touched it. Group 0's acceptance assertion had nothing that could make it pass. | `plan.Input` gains `Matched`, carried verbatim into `lock.Source`. `plan` stays pure and asks `IsRange` nothing. New task group 6 with its own RED/GREEN. | design.md → Boundaries; new `specs/sync-plan/spec.md`; tasks.md group 6 |
| CRITICAL | proposal.md, specs, tasks.md | `graft sync` over a range was unspecified and unimplemented. `sync-execution:87` already requires a source with no lock entry to be resolved exactly once, and `internal/sync/run.go` skips `source.Resolve` only when the lock holds a sha — so `matched` must be carried from the **previous lock**, exactly as `pinned` carries the sha. Nothing scheduled it, and the proposal's "no re-evaluation on sync, ever" did not settle it: a first resolution is not a re-evaluation. | Added the `sync-execution` delta naming the range as a fourth case on the first-resolution path. Split the sync group so `matched` is carried on both branches. Proposal's non-goal reworded. | proposal.md → What Changes, Non-Goals, Capabilities; new `specs/sync-execution/spec.md`; design.md → Decisions; tasks.md group 7 |
| CRITICAL | 4 delta specs | 17 scenarios silently dropped from `MODIFIED` blocks, which are lost permanently at archive time: 5 from `update-execution` (including the prune-safety case CLAUDE.md calls load-bearing), 7 from `lock-format` (including three guards stopping a hand-edited lock from aiming a delete outside the repo), 1 from `rev-resolution`, 5 from `sync-plan`. Two further bullets truncated inside retained scenarios. In every case the delta's own description still required the dropped behavior. | Every MODIFIED block rebuilt from the live text: all original scenarios restored verbatim and in order, edited copies preserved where the edit was intentional, new scenarios appended. Verified programmatically that no MODIFIED requirement is missing any live scenario. | `specs/update-execution/`, `specs/lock-format/`, `specs/rev-resolution/`, `specs/sync-plan/` |
| CRITICAL | specs/list-execution/spec.md | Only two of three affected requirements were modified. The unmodified one still asserted `"version": 1` for the empty document while the delta asserted `2` — two contradictory scenarios, merged into one spec at archive. | The third requirement reproduced in full at version 2; the duplicate empty-repo scenario removed from the `--json` requirement. | `specs/list-execution/spec.md` |
| WARNING | tasks.md | `rejectUnknown`'s allow-list (`internal/lock/lock.go:161`) runs **before** `validate`, so without `"matched"` in it every range lock fails as `unknown key "matched"` and all three new validation messages are unreachable. design.md's "four places" was five. | Task 5.4 names all five sites and why the allow-list is the one that fails silently. | tasks.md 5.4; design.md → Boundaries |
| WARNING | tasks.md | The new `ls-remote` call site would swallow the `errNoGit` passthrough (`resolve.go:38-40`), reporting `cannot reach "<url>"` where a missing git must report `git not found on PATH` — an asserted error string. | Named in task 4.5. | tasks.md 4.5 |
| WARNING | specs/lock-format/spec.md | The lock's `version` stays `1` while the format gains a key, so an older graft reports `unknown key "matched"` rather than the upgrade message that requirement exists to produce. Undocumented. | The requirement now states the accepted degradation and why a global version bump would be worse than a per-source key, with two scenarios. | `specs/lock-format/spec.md` |
| WARNING | proposal.md | The proposal described the **abandoned** range rule (git's forbidden characters) rather than the one the specs pin (leading `^ ~ > < =`, space, `\|\|`, bare `*`), and its "no re-evaluation on sync, ever" non-goal was false for a first resolution. | Both rewritten to match the specs. | proposal.md → What Changes, Non-Goals |
| WARNING | proposal.md | The `sync-report` delta silently changed a rendering contract from "both halves move together" to per-half, undeclared. It is a genuine correction — the live sentence already contradicted its own scenario — but an undeclared one. | Declared in What Changes as a correction, with the contradiction named. | proposal.md → What Changes |
| WARNING | design.md | Verification matrix tiers wrong in 15 rows: 11 `rev-ranges` selection rows at integration that tasks.md itself implements as unit tests over a `[]string`, and 4 `list-execution` rows at acceptance that are decidable from a built `*list.Listing` — two of which the archived `list-command` change already established as unit. | All 15 re-tiered. Two scrambled-lock rows restored to integration, matching how they are actually implemented today. | design.md → Test Strategy |
| WARNING | design.md, tasks.md | The matrix put *An empty repository still declares the new version* at acceptance correctly, while task 8.2 listed it among unit tests. A repository with no lock cannot be expressed as a built value. | Task 8.2 corrected and the scenario moved to the acceptance task. | tasks.md 8.2, 8.6 |
| WARNING | design.md | Test Boundaries had only two columns while Test Strategy declares three tiers, producing a row contradicting its own tier. It also claimed unit tests touch no filesystem — false for task 5.1, whose byte-identity proof reads `internal/lock/testdata/canonical.lock` — and claimed the environment is untouched while `XDG_CACHE_HOME` is set per test. | Rebuilt with three columns and rows for the `graft` binary, testdata fixtures, the deleted source repository, `internal/rev`, `internal/plan`, and the environment. Network scoped to the build-time dependency fetch. | design.md → Test Boundaries |
| WARNING | tasks.md | Three tasks labelled RED cannot go red: the no-range byte-identity guard, `TestReportAlignment`, and the version-2 contract rules. They are valuable guards, but the label conflated "written first" with "fails first". | Relabelled CHARACTERIZE, still scheduled before the change tasks. | tasks.md 5.1, 7.5 |
| WARNING | tasks.md | Two `internal/cli` `--json` goldens go stale the moment the document version moves, and no task edited them; group 8 ran only `./internal/list/`. The acceptance tier design.md assigns was unimplemented. | Tasks added to update both goldens, add the acceptance cases, grep for any remaining version-1 golden, and widen the group's test command. | tasks.md 8.5, 8.6, 8.8, 8.9 |
| WARNING | tasks.md | SPEC.md's `graft list` sections were unowned: it publishes the `--json` document, the plain-listing header, and the sync-report header example — all three falsified by this change, and all three asserted character for character by tests. | Task 10.4 names all three. | tasks.md 10.4 |
| WARNING | specs/rev-ranges/spec.md | *Two tags naming the same version* could not drive a test: it required determinism without naming which tag wins, so two implementations returning different shas both pass. | The tie-break is now stated in the requirement — lower tag name, byte-wise — and the scenario names the winner. | `specs/rev-ranges/spec.md` |
| SUGGESTION | specs/rev-ranges/spec.md | The classification rationale miscounted: it called `*` and the space "leading characters" and claimed four of five are git-illegal. | Rewritten to say what is actually true. | `specs/rev-ranges/spec.md` |
| SUGGESTION | specs, design.md | Two uncovered cases: `graft update --to "^1.2.0"` — a range written into `graft.toml` by the only path that edits it, with a space inside the value for the compound form — and build-metadata precedence, cited in the proposal as a reason to take the dependency. | Three scenarios added, with matrix rows. | `specs/update-execution/`, `specs/rev-ranges/` |
| SUGGESTION | tasks.md | `internal/sync/report.go:112`'s per-source skip compares only rev and resolved, so a retag onto the **same commit** changes the lock but prints a summary with no block explaining it. | Scenario added and a RED/GREEN pair scheduled to add `b.Matched == a.Matched`. | `specs/sync-report/spec.md`; tasks.md 7.9, 7.10 |
| SUGGESTION | tasks.md | Three premises were factually wrong: `(*sourceRepo).tag` already exists (0.1 said to build it), `transition()` already exists (the refactor proposed writing it), and `IsRange("") == false` passes by construction. | 0.1 reuses the helper, the refactor reuses `transition`, and 2.2 asserts the observable `rev is empty` failure instead. | tasks.md 0.1, 2.2, 7.11, 9.3 |
| SUGGESTION | tasks.md | Task 4.7's regression checklist omitted the three ref failure messages, and the empty-matched clauses added to two `rev-resolution` scenarios had no asserting task. | Both named. | tasks.md 4.6, 4.7 |

Verified true rather than repaired: the alignment claim (`sourceKeyWidth = len("resolved")` = 8 >
`len("matched")` = 7, so no existing lock line shifts); the security carry-over on the new
`ls-remote` call site (`CloneURL`'s leading-dash refusal, the `--` separator, and
`GIT_TERMINAL_PROMPT=0` all apply, and `attr.tree` and the `os.OpenRoot` reads are correctly
irrelevant to a ref listing — now recorded in design.md so the question is not reopened); that
`internal/apply`, `internal/manifest`, `internal/catalog` and `lock.CheckPins` genuinely need
nothing; and that exactly one production caller of `Resolve` exists.

## No Remaining Implementation-Blocking Gaps

None remain. All five CRITICALs are repaired and each was verified against the code before
being accepted. Every MODIFIED requirement was checked programmatically to be lossless against
its live counterpart, all 127 spec scenarios appear in the verification matrix exactly once,
every task group carries exactly one kind marker with its tasks numbered sequentially, and
`openspec validate semver-ranges --strict` passes.

One decision was taken by the owner rather than derived, and is recorded rather than resolved:
semver ranges overturn a stated non-goal in PRD.md and SPEC.md. The reasoning behind the
original non-goal — no dependency graph, no cross-source solving, no registry — still holds and
is preserved in the proposal; only the range syntax is admitted.

## Deferred Non-Blocking Notes

- **`graft add` and ranges** — whether `add` writes a range, and what it defaults to when no
  `@rev` is given, belongs to `add-command`. Recorded as a non-goal in proposal.md, and
  `add-command` is the next change in `openspec/IMPLEMENTATION-ORDER.md`.
- **CLAUDE.md's pin rule wording** — this change widens it to cover ranges. The separate
  rewording that makes `add` a second pin-mover is `add-command`'s, and is named in
  proposal.md → Impact so it is not lost.
- **A tag literally named `>=1.2.0` is unpinnable** — accepted knowingly, documented in
  `rev-ranges` and in design.md → Risks. The consumer can still pin the sha.
