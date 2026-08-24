## Reviewed Artifacts

- `openspec/changes/sync-command/proposal.md`
- `openspec/changes/sync-command/design.md`
- `openspec/changes/sync-command/tasks.md`
- `openspec/changes/sync-command/specs/file-application/spec.md`
- `openspec/changes/sync-command/specs/sync-execution/spec.md`
- `openspec/changes/sync-command/specs/sync-report/spec.md`
- `openspec/changes/sync-command/specs/command-invocation/spec.md`

Reviewed by two independent subagents dispatched for the purpose, neither of which wrote any
part of the plan and neither of which was a fork of the planning session. One took coherence
and completeness — matrix-to-scenario-to-task correspondence, contradictions with SPEC.md and
with the archived specs, the schema's artifact rules. The other took safety — whether any path
in the plan can delete a file absent from `graft.lock`, what a failed apply leaves behind, and
whether anything opens an execution route. The safety reviewer verified three of its findings
empirically against Go 1.27's `os.Root` rather than reasoning about the documentation.

## Reviewed Against

- This repository HEAD: `669dff5ca038fc6138ac8dd7802b863a3705c724`
- Sibling repository HEAD: Not applicable — no sibling repository is involved in this change
- Working tree: clean apart from `openspec/changes/sync-command/`, the planning files under
  review

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | specs/file-application | The empty-directory walk specified a bare `Remove` on each ancestor, reasoning that "a non-empty directory fails harmlessly". True of directories, false of symlinks: unlinking a symlink succeeds however full its target is, so a user's `vendor -> docs` link in a pruned path's ancestry would be deleted — a path absent from `graft.lock`. | A removal candidate is examined without following it and removed only if it **is a directory**. A removal that fails for any reason is ignored rather than fatal. | `file-application` → "Directories left empty…", + scenario *A symlinked ancestor of a pruned path is not removed*; design.md → Decisions; tasks 6.1–6.3 |
| CRITICAL | specs/file-application | `Lstat` declines to follow only the **last** component, so a prune path whose parent has become a symlink deletes a file at the resolved location. A lock claiming `vendor/x.md` would destroy `docs/x.md` — reached, and absent from the lock. | Every existing ancestor of a prune path must be a directory, checked without following. New error `cannot remove "<path>": "<ancestor>" is not a directory`. | `file-application` → "Only the prune set is deleted…", + scenarios *A prune path under a symlinked parent is refused* and *A prune path whose parent is a regular file is refused*; tasks 5.2–5.3 |
| CRITICAL | specs/file-application | The same hole on the write side: a write through a symlinked parent lands at a path `graft.lock` does not name, so the file an item places does not stay inside that item's destination and a later prune aims wherever the link points. design.md → Risks previously called this "consistent". | Same ancestor rule for destinations, with `cannot write "<path>": "<ancestor>" is not a directory`. The Risks bullet was rewritten to say what is refused and why. | `file-application` → "A destination that is not a regular file…", + scenarios *A destination under a symlinked parent is refused* and *A destination whose parent is a regular file is named*; design.md → Risks; tasks 3.1–3.3 |
| CRITICAL | specs/file-application | Mode `0644` was specified but not achievable: the permission argument to a create-and-truncate open applies only on creation, so overwriting a file someone once ran `chmod +x` on leaves it executable while graft replaces its contents with source-controlled bytes. The only mode scenario tested a fresh write. | An existing destination is removed and recreated with `O_EXCL` rather than truncated. New scenario for the overwrite path. | `file-application` → "Every planned write…", + scenario *An executable destination is made non-executable*; design.md → Decisions; tasks 2.2–2.3 |
| CRITICAL | specs/file-application (absent) | Nothing refused a destination inside `.git/`. `internal/plan` refuses a destination that *escapes* the root, and `.git/config` does not escape it; kinds are arbitrary, so no rule constrains what a `to` may name. A file placed there is invisible to `git diff` — SPEC.md's entire mitigation — and `.git/config` turns placing a file into running a program via `core.fsmonitor`, `core.sshCommand`, or an alias. `graft.toml` and `graft.lock` had the same gap. | New requirement refusing `.git` as a first path segment and graft's own two files, as destinations and as prune paths, with four pinned messages. Placed in `internal/apply` as a floor under the writer; the reasoning and the `--dry-run` consequence are recorded. | `file-application` → "graft never writes inside `.git`…" (4 scenarios); design.md → Decisions and Risks; tasks group 4; SPEC.md update in task 17.2 |
| CRITICAL | specs/sync-report | The padding rule said every column is "followed by two spaces", which emits trailing whitespace on any line with no note — SPEC.md's own example has none. Two normative statements in conflict, discovered by the task that diffs them. | Padding restated: verb and id always followed by two spaces; the count padded and followed by two spaces **only when a note follows**. "No report line SHALL carry trailing whitespace" stated explicitly and asserted in the alignment scenario. Column widths made per-block for all three columns. | `sync-report` → "Each changed item gets one line…"; tasks 14.1–14.2 |
| WARNING | specs/file-application | Every refusal was discovered mid-apply, so a tree in an odd state guaranteed a partial apply — and, because the lock is never written, the identical failure repeated on every subsequent sync with the user given no remedy. All the conditions are decidable from `Lstat` alone. | A pre-flight pass validates the whole plan before the first write. design.md → Goals restated to match ("every failure graft can see coming"), and the two "leaves the previous lock" scenarios replaced with three that assert nothing was applied at all. | `file-application` → "Everything checkable is checked before the first write" (3 scenarios); design.md → Goals, Decisions; tasks group 7 |
| WARNING | specs/sync-execution | Three failure modes the requirement itself said "SHALL be covered" had no scenario, no matrix row, and no task: an **invalid** `catalog.yaml`, a destination outside the repository root reaching the user through a real `plan.Build`, and a source listing climbing out of its item. | Three scenarios added, three matrix rows, and task 11.2/11.1 bullets. The destination-escape one goes through the real planner rather than a hand-built plan, which is what makes it a different test from the `internal/apply` one. | `sync-execution` → "Every planning failure…"; design.md → Test Strategy; tasks 11.1–11.2 |
| WARNING | design.md | The ordering scenario was to be verified by "an observing wrapper", a collaborator the Test Boundaries table forbids and the one-function contract provides no seam for. A second scenario asserted no directory "was even inspected", which is likewise unobservable. | Ordering is verified by composed effect — a prune and a write in one directory, where only the documented order leaves the directory intact with the new file in it. Both scenarios reworded; the Boundaries table gained an explicit "no seam" row. | `file-application` scenarios *A plan's operations happen…* and *An unrelated empty directory…*; design.md → Test Boundaries, Test Strategy |
| WARNING | design.md | The Test Boundaries fetch-cache row claimed the cache root is "passed in as a value" and that `source.DefaultCacheRoot` is "never called from a test", while tasks had the `sync` command call exactly that and the acceptance test drive `cli.Main`. | Row split by tier. The acceptance path names `DefaultCacheRoot`, `XDG_CACHE_HOME`, and the fact that `defaultCacheRoot` honours it **only when absolute** — otherwise falling through to the developer's real `~/.cache`. | design.md → Test Boundaries, Risks; task 0.1 |
| WARNING | design.md | `graft help` was to be suppressed with a `SetHelpCommand` placeholder. Cobra unconditionally adds that placeholder to the root, so its `Use` string becomes a working, undocumented command — trading one leak for another, of exactly the class the archived spec already pins for `__complete`. | Refuse the literal argument `help` in `Main`, beside the existing `__complete` guards. No command name is added at all. | design.md → Decisions, Boundaries; task 15.3 |
| WARNING | design.md | Goals said "every failure leaves the tree byte-identical", contradicting the Non-Goal on transactional apply and the spec's own mid-flight scenario. | Goal scoped to failures graft can see coming, which the pre-flight pass makes exhaustive over the specified refusals, with the residual I/O case named. | design.md → Goals |
| WARNING | design.md | The partial-apply risk called orphaned files benign — "the next successful sync will overwrite or leave alone". "Leave alone" is the unrecoverable case: a user who drops the source in response to a failing sync leaves files no lock ever claimed, unreachable by graft forever. | Stated plainly, with `git status` named as the only recovery. | design.md → Risks |
| WARNING | (all artifacts) | Eleven new user-facing error strings and one new invariant, none of them in SPEC.md's failure-mode table, which AGENTS.md calls the product for a CLI this size. | New operational group updating SPEC.md's Failure modes, Invariants, and Output sections, and the two stale layout blocks. | tasks group 17; design.md → Boundaries, Decisions |
| WARNING | specs/file-application | Filesystem errors that are neither "missing" nor "not a regular file" — `ENOTDIR` from a parent that is a regular file — were unspecified, so the user would get a raw syscall string in place of graft's error format. | Covered by the ancestor rule, which names the offending ancestor in graft's own wording. Two scenarios assert it on the write and prune sides. | `file-application`, scenarios *A destination whose parent is a regular file is named* and *A prune path whose parent is a regular file is refused* |
| WARNING | specs/file-application | Whether a failed directory removal aborts the apply was unspecified; an implementer returning the error would fail every sync with a non-empty ancestor, after the prunes had already run. | "A removal that fails for any reason SHALL be ignored and SHALL NOT fail the apply", with the reason. | `file-application` → "Directories left empty…"; task 6.2 |
| WARNING | tasks.md | Task 16.5 used `openspec validate --change sync-command --strict`, which the installed openspec rejects. | Corrected to `openspec validate sync-command --strict`. | task 19.5 |
| WARNING | specs/sync-report | `up to date` is printed for a run that restores files the user deleted by hand — a user-visible disagreement with SPEC.md's "every sync's effect is a git diff" that the plan did not acknowledge. | Kept the predicate (narrowing it means the tree-scanning this design refuses) and stated the case explicitly in the requirement and in design.md → Decisions. | `sync-report` → "A sync with nothing to do…"; design.md → Decisions |
| NOTE | specs/sync-report | Source ordering was "the order the lock records them", undefined for a source that appears only in the **old** lock. | "In name order over the union of the sources in the old lock and in the new one." | `sync-report` → "A source whose pin or items moved…"; task 13.3 |
| NOTE | specs/sync-report | What `--dry-run` prints when there is nothing to do was unspecified. | `up to date`, with its own scenario. `--dry-run` changes the summary line, not the meaning of "nothing to do". | `sync-report`, scenario *A dry run with nothing to do reports nothing* |
| NOTE | specs/sync-execution | A clean `--dry-run` could be read as a promise the sync will succeed, though it exercises none of `internal/apply`'s refusals. | One paragraph saying so. | `sync-execution` → "`--dry-run` prints the plan…" |
| NOTE | specs/file-application | No scenario covered a prune path at the repository root, though the requirement asserts the root is never removed. | Scenario added. | `file-application`, scenario *A pruned path at the repository root removes nothing* |
| NOTE | specs/file-application | The foreign-file scenarios all placed the foreign file *beside* a synced file; none exercised the ancestry, which is where the two symlink criticals live. | The ancestry cases were added to the prune and directory-removal requirements rather than as a separate concern, and a scenario asserts a directory of ten unrecorded files is never enumerated. | `file-application`, scenarios *Unrecorded files in a destination directory are never enumerated*, *A prune path under a symlinked parent is refused*, *A symlinked ancestor of a pruned path is not removed* |
| NOTE | tasks.md | Group 14 (acceptance GREEN) added two new tests with no RED step, and several behavior groups carried a bare `CHECK` that was not a contract gate. | Acceptance GREEN now has explicit RED and GREEN steps. Non-gate `CHECK`s became `REFACTOR` steps; the two that remain are contract gates — the `graft.lock` layout gate and the existing-`cli`-surface gate. | tasks groups 3, 5, 6, 8, 10, 12, 15, 16 |
| NOTE | ENGINEERING.md, AGENTS.md | Both layout blocks list six packages and name neither `cli`, `ui`, nor the new `sync`. Drift predates this change, but this change adds a package. | Task added to update both. | task 17.3 |

## No Remaining Implementation-Blocking Gaps

None remain. Every CRITICAL and every WARNING above is repaired in the artifacts rather than
deferred, and the coherence reviewer's mechanical checks — a bijection between spec scenarios
and verification-matrix rows, every scenario owned by a task, no task inventing a boundary the
Test Boundaries table omits, every Go signature the design names existing or created by this
change — were re-established after the repairs.

No unresolved decision requires user input.

## Deferred Non-Blocking Notes

- **`os.Getwd()` and `source.DefaultCacheRoot()` failures have no scenario.** Both are
  reachable from `graft sync` and both already produce a located message that graft's existing
  error format renders correctly, with the working tree untouched because neither can fail
  after a write. They live in `internal/cli`, where a scenario would pin a message the caller
  cannot influence and a failure the test cannot induce without breaking the process's own
  working directory. Resolution point: if `add-command` or `update-command` needs either
  message to differ, it inherits the decision and specifies it then. Recorded in design.md →
  Open Questions.
- **`--dry-run` does not exercise `internal/apply`'s refusals.** A dry run stops after the
  plan, so a clean dry run followed by a refused sync is a legitimate outcome. Stated in
  `sync-execution` rather than closed, because closing it means running the pre-flight pass
  without a plan to apply — a second entry point into the only writer, which is a cost this
  change should not pay. Resolution point: reconsider if a user reports the surprise.
