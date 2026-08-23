## Reviewed Artifacts

- `openspec/changes/manifest-and-lock/proposal.md`
- `openspec/changes/manifest-and-lock/specs/manifest-format/spec.md`
- `openspec/changes/manifest-and-lock/specs/lock-format/spec.md`
- `openspec/changes/manifest-and-lock/design.md`
- `openspec/changes/manifest-and-lock/tasks.md`

Read as sources of truth, not reviewed: `SPEC.md`, `PRD.md`, `ENGINEERING.md`, `AGENTS.md`,
`openspec/IMPLEMENTATION-ORDER.md`, `openspec/config.yaml`, `Taskfile.yml`, `go.mod`.

**Delegation deviation, stated plainly.** The schema and `openspec/config.yaml` both require
the finding pass to be delegated to reviewers that did not write the plan. This session had
no subagent dispatch available — `ListAgents` showed only unrelated interactive sessions
belonging to other work, which are not valid reviewers — so the finding pass was run by the
authoring session against the review list. That is weaker than the rule asks for and is
recorded here rather than papered over. The gaps below are real findings with real repairs;
treat the pass as one-eyed, and note that tasks.md group 9 still requires an independent
reviewer against the implementation diff, where delegation is available.

## Reviewed Against

- This repository HEAD: `18b73fed3d28bb729dc5d7b2bea4f01ae5ff5f38`
- Sibling repository (`optioni/openspec-schemas`) HEAD: `Not applicable` — this change reads
  no catalog and installs nothing; the schema copies under `openspec/schemas/tdd/` are inputs
  to the workflow, not to the code being planned.
- Working tree: clean apart from `openspec/changes/manifest-and-lock/`, which is this
  planning package and is intentionally included.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | tasks.md | The `parallel-after: 1` marker on the `internal/lock` group was false. That group's refactor task extracted a shared `kind:name` check touching `internal/manifest`, so the two "independent" groups would have edited the same file tree concurrently. | Narrowed the lock group's refactor to formatting inside its own package, and moved the cross-package extraction into a new refactor group that runs after both behavior groups. | tasks.md 3.11; new group 4 |
| CRITICAL | design.md | Test Boundaries said the network is "not involved", but tasks.md group 1 runs `go get`, which reaches the Go module proxy. A task therefore used a collaborator the table denied. | Split the row: runtime network stays not involved; a new build-time row names the module proxy as real, once, and notes `go.sum` removes it from the suite. | design.md → Test Boundaries; tasks.md 1.2 |
| WARNING | design.md | The golden lock fixtures under `internal/lock/testdata/` appeared as collaborators in the verification matrix but had no row in Test Boundaries, so four tasks rested on an unnamed boundary. | Added a Test Boundaries row for the golden fixtures (real files, read only, never written at test time) and made the matrix wording match it. | design.md → Test Boundaries and Test Strategy |
| WARNING | tasks.md | The serialization group had no contract gate, although it defines the byte layout of `graft.lock` — a format `openspec/config.yaml` names as gate-worthy. | Added a contract gate re-reading SPEC.md's lock example for key alignment, the two-space item indent, and the exploded-array trailing comma. | tasks.md 5.9 |
| WARNING | tasks.md | Three behavior groups (determinism, pin agreement, and the lock group) ended without a REFACTOR task or an explicit statement that none was needed, breaking the RED → GREEN → REFACTOR lifecycle the schema requires. | Added a REFACTOR task to each, with the pin-agreement one stating outright that none is expected for a single comparison. | tasks.md 3.11, 6.7, 7.4 |
| WARNING | tasks.md | The documentation group ran CHANGE before one of its CHECK tasks, inverting the operational CHECK → CHANGE → VERIFY order. | Reordered so both CHECK tasks precede the SPEC.md edit. | tasks.md 8.1–8.4 |
| WARNING | design.md | The open question about SPEC.md's self-contradicting lock example said the correction was out of scope, while the change plainly benefits from the example matching its own rule — and no other change would own it. | Resolved it: the documentation group reorders the example's two items, changing no rule text; design.md now says so. | design.md → Open Questions; tasks.md group 8 |
| SUGGESTION | proposal.md | First draft ran to 576 words against the schema's 400-word guidance, mostly through restating non-goals already implied. | Condensed to 451 words while keeping every explicit Non-Goal, which `openspec/config.yaml` requires. | proposal.md |

Checks that found nothing and are worth naming, because they are where this repository fails:

- All 48 spec scenarios appear in design.md's verification matrix (48 rows) and every one is
  named in a RED task in tasks.md. Counted, not eyeballed.
- No group mixes kinds; every group carries exactly one marker.
- No task invents a collaborator the Test Boundaries table does not name, after the two
  repairs above.
- The plan crosses no PRD non-goal: nothing resolves a dependency, no registry appears, no
  file is merged, no credential is handled, and nothing this change plans could make a synced
  file require graft at runtime.
- No code path lets a source repository cause anything to execute. This change never reads a
  source repo at all — it parses two consumer-owned files and returns bytes.
- No RED task is scheduled for plumbing: the `go.mod` change is operational and `task build`
  is its evidence.

## No Remaining Implementation-Blocking Gaps

None remain. Every gap above is repaired in the artifact that owns it, and the change
validates under `openspec validate manifest-and-lock --strict`.

Two readings of ambiguous source documents were made deliberately rather than left open, and
both are recorded in design.md → Decisions and Open Questions so a reviewer can overturn them
cheaply: items sort by id despite SPEC.md's example showing otherwise, and
IMPLEMENTATION-ORDER's "deterministic serialization" is read as applying to `graft.lock` only.
Neither blocks implementation.

## Deferred Non-Blocking Notes

- `graft.toml` serialization is deferred to `add-command`, which delivers manifest amendment
  and is the first caller that must decide whether to rewrite the file or edit it in place.
  Resolution point recorded in design.md → Decisions.
- Whether `resolved` should accept a 64-character SHA-256 object id is deferred to
  `git-fetch`, which produces the value. Recorded in design.md → Open Questions.
- Whether a lock item with zero `files` can legitimately arise is deferred to
  `destination-and-plan`, which computes the file sets. Recorded in design.md → Open Questions.
- Whether a `kinds` override destination may escape the repo root is deferred to
  `destination-and-plan`, which owns that invariant over resolved destinations and must check
  catalog-supplied destinations regardless. Recorded in design.md → Open Questions.
- `openspec instructions` prints `Rules for 'specs' must be an array of strings, ignoring this
  artifact's rules` for this repository's `openspec/config.yaml`, so the four `rules.specs`
  entries were applied by hand rather than delivered by the tool. The config is valid YAML and
  the schema copies are synced from `openspec-schemas`, so neither may be edited here; noted
  so a future session does not assume the specs rules were enforced automatically.
