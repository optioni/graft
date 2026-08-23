## Reviewed Artifacts

- `openspec/changes/catalog-and-selectors/proposal.md`
- `openspec/changes/catalog-and-selectors/specs/catalog-format/spec.md`
- `openspec/changes/catalog-and-selectors/specs/selector-expansion/spec.md`
- `openspec/changes/catalog-and-selectors/design.md`
- `openspec/changes/catalog-and-selectors/tasks.md`

Read as sources of truth, not reviewed: `SPEC.md`, `PRD.md`, `ENGINEERING.md`, `AGENTS.md`,
`openspec/IMPLEMENTATION-ORDER.md`, `openspec/config.yaml`, `openspec/specs/manifest-format/`,
`openspec/specs/lock-format/`, the archived `manifest-and-lock` change, `Taskfile.yml`,
`go.mod`, and the existing `internal/{itemid,manifest,lock}` sources.

**Delegation deviation, stated plainly.** The schema and `openspec/config.yaml` both require
the finding pass to be delegated to reviewers that did not write the plan. This session has
no subagent dispatch available: the only addressable agents are the orchestrator that
spawned it and peer sessions belonging to unrelated work, neither of which is a valid
reviewer. The finding pass was therefore run by the authoring session against the review
list. That is weaker than the rule asks for and is recorded here rather than papered over.
The gaps below are real findings with real repairs; treat the pass as one-eyed, and note
that tasks.md group 9 still requires an independent reviewer against the implementation
diff, where delegation is expected to be available.

## Reviewed Against

- This repository HEAD: `35fed5d2c2b6fc9f12713a084c60d5ba3e8a8cfc`
- Sibling repository (`optioni/openspec-schemas`) HEAD: `Not applicable` — this change reads
  no catalog and installs nothing; the schema copies under `openspec/schemas/tdd/` are inputs
  to the workflow, not to the code being planned.
- Working tree: clean apart from `openspec/changes/catalog-and-selectors/`, which is this
  planning package and is intentionally included.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | tasks.md | The spec scenario `A glob matching nothing is an error` had a verification-matrix row but no RED task. Counting tasks against scenarios found 40 of 41 covered; a glob whose kind exists but whose name matches nothing would have shipped unverified. | Added it to the glob group's RED task, with `hook:*` against a catalog holding no `hook` items. | tasks.md 7.2 |
| WARNING | design.md, tasks.md | Nothing said in what order the `kinds` mapping is walked. Go randomises map iteration, so a catalog with two faulty kinds would report a different error on different runs — the same failure mode `internal/manifest` already avoids by sorting source names, and one the specs' single-fault fixtures would never catch. | Added a Decisions entry requiring sorted key order everywhere a map is walked, and wrote it into the kind-extraction task. | design.md → Decisions; tasks.md 3.4 |
| WARNING | design.md | Test Boundaries said `internal/lock` is "not involved", while tasks.md 5.4 compares this package's unknown-key walk against `internal/lock`'s. A task appeared to reach a collaborator the table denied. | Narrowed the row: reading source during a refactor decision is not a test collaborator, and if the walks were ever shared the code would move to a third package rather than make `catalog` import `lock`. | design.md → Test Boundaries |
| WARNING | specs/catalog-format/spec.md | The document-shape surface was unspecified. A `catalog.yaml` holding a valid YAML sequence, or holding zero bytes, is neither a syntax error nor a mapping the parser can read — the plan would have left the implementer to invent a message, and the generic-`any` decode makes both reachable on day one. | Added two scenarios: a non-mapping document is `catalog.yaml: top level must be a mapping`, and an empty file is treated as an empty mapping so it reports `version is required`. | specs/catalog-format/spec.md → Catalog loading and absence; design.md matrix; tasks.md 2.2 |
| WARNING | tasks.md | The change corrects nothing in any maintained document, but `openspec/config.yaml`'s `context` asserts "the module has no dependencies" — already false since `manifest-and-lock`, and made more false here. Left alone it argues against a dependency a future change legitimately needs. | Added a Documentation group that rewrites that one clause, with document, audience, section, and durable reason named, and renumbered Change Review and Lint & Verify. | tasks.md group 8 |
| SUGGESTION | design.md | The glob decision named `path.Match` without saying what happens to a malformed pattern, which is the difference between a typo being reported and a typo silently becoming "no match". | Made the `ErrBadPattern` behavior explicit in Decisions and named the rejected alternative. | design.md → Decisions |

Checks that found nothing and are worth naming, because they are where this repository fails:

- All 41 spec scenarios appear in design.md's verification matrix (41 rows) and, after the
  repair above, every one is named in a RED task. Counted, not eyeballed.
- No group mixes kinds; every group carries exactly one marker, and every lifecycle runs
  evidence-first — RED before GREEN, CHECK before CHANGE.
- No `parallel-after` marker is used. Groups 2 through 7 build one package and share its
  files, so none of them is independent of the one before it.
- No task invents a collaborator the Test Boundaries table does not name, after the repair
  above.
- Every group that changes `catalog.yaml` parsing carries a contract gate, as
  `openspec/config.yaml` requires — groups 2, 3, 4, and 5 — and the two selector groups
  carry one against SPEC.md's `install` bullet.
- The plan crosses no PRD non-goal: nothing resolves a dependency (`requires` is rejected as
  an unknown key, which is what keeps SPEC.md's own open question askable), no registry
  appears, no file is merged, no credential is handled, and nothing here could make a synced
  file require graft at runtime.
- No code path lets a source repository cause anything to execute. This change reads a
  source-authored file and validates it; the two rules that exist because the author is
  untrusted — `from` containment and undeclared kinds — are checked before any value leaves
  the package.
- The change adds no write path and no code to `internal/plan`, and every test bar two
  (`Load` and its absence case) takes bytes or an in-memory value, so no test needs a real
  directory to exercise parse or expansion logic.
- No RED task is scheduled for plumbing: the `go.mod` change is operational and `task build`
  is its evidence.

## No Remaining Implementation-Blocking Gaps

None remain. Every gap above is repaired in the artifact that owns it, and the change
validates under `openspec validate catalog-and-selectors --strict`.

Two readings of ambiguous source documents were made deliberately rather than left open, and
both are recorded in design.md → Decisions and Open Questions so a reviewer can overturn them
cheaply: the glob applies to the name position only, so `*:tdd` falls out as a no-match
error; and the format version is validated before unknown keys, so a future catalog is
answered with "upgrade graft" rather than with a complaint about a key that version
legitimately defines. Neither blocks implementation.

## Deferred Non-Blocking Notes

- Whether a `provides[].from` names a path that actually exists in the source tree is not
  answerable here — this change never sees the tree. Resolution point recorded in design.md
  → Open Questions: `destination-and-plan` or `sync-command` meets the real files.
- Whether `catalog.yaml` needs a `requires` field is SPEC.md's own open question and a step
  toward dependency resolution, a PRD non-goal. Recorded in design.md → Open Questions; the
  key is rejected at version 1, which leaves the question open rather than answering it.
- Destination computation, the repo-root escape check over computed destinations, and the
  cross-item collision check are all `destination-and-plan`, and tasks.md 3.7 carries a
  concentration-point check that none of them leaked into this package early.
- `openspec/config.yaml`'s stale "the module has no dependencies" clause is repaired by
  tasks.md group 8 rather than during planning, so the correction lands with the change that
  makes it wrong for the second time.
