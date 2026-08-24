## Reviewed Artifacts

- `proposal.md`
- `specs/update-execution/spec.md`
- `specs/manifest-format/spec.md`
- `specs/file-application/spec.md`
- `specs/command-invocation/spec.md` — added during this review; see G2
- `design.md`
- `tasks.md`

## Reviewed Against

- This repository HEAD: `7952e66cb3b467bedfe71b00816eeba30f8fde36`
- Sibling repository HEAD: Not applicable — graft has no sibling repository and no library
  consumers
- Working tree: clean apart from `openspec/changes/update-command/`, the planning package under
  review

Three reviewers ran the finding pass, none of them a fork of the planning session and none with
write access to the artifacts. Each was given the change directory, the live specs, and one slice
of the review list:

1. **capability coverage, scenario quality, scenario → task mapping, tasks discipline**
2. **technical soundness against the code that exists** — `internal/{sync,apply,manifest,lock,cli,source,plan}`, the vendored cobra, and SPEC.md
3. **project rules, threat model, PRD non-goals, and documentation net-size**

No reviewer returned a CRITICAL. The findings below are the merged WARNINGs and the SUGGESTIONs
that were acted on; each names the artifact that owned the gap and where the repair landed.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| WARNING | `specs/file-application/spec.md` | The delta was ADDED only, while the live requirement *Applying a plan is the only path that writes to the working tree* pins the entry point's arguments, a five-step ordered list, "every path it touches SHALL come from the plan", and "an empty plan writes only the lock". All four become false. Archiving would have left the capability self-contradicting. | Converted to a MODIFIED requirement carrying the full original block: the entry point gains the optional manifest bytes, the step list gains a conditional step 5, the derives-nothing paragraph gains its one named exception, and the empty-plan sentence is qualified. Added a scenario for an empty plan *with* manifest bytes. | `specs/file-application/spec.md` → MODIFIED Requirements |
| WARNING | `proposal.md`, `specs/update-execution/spec.md` | Root help gains a command with no delta on `command-invocation`, whose live requirement reads "its help SHALL list **that subcommand**" (singular) and whose scenario asserts only `sync`. Two capabilities would have owned root-help behavior, and the owning one would have been stale. | Added a MODIFIED delta on `command-invocation` generalising to every subcommand, with the scenario naming both and a new one for `graft update --help`. Removed the duplicate *Help lists both commands* scenario from `update-execution`, which now points at the owner. `proposal.md` → Capabilities lists the modification. | `specs/command-invocation/spec.md`; `specs/update-execution/spec.md` R7; `proposal.md` |
| WARNING | `specs/manifest-format/spec.md`, `tasks.md` | The specified line scanner had no stated boundary for `[sources.<name>.kinds]`. `Kinds` is `map[string]string` with no constraint on key names, and SPEC.md's own example puts the sub-table right after the source table — so a prefix match, or one that only stops at the next `[sources.` header, silently rewrites a kind named `rev` and passes every test that was listed. | Split the spec into a *preserving* requirement and a *locating* requirement. The header is matched as the exact key path and the table ends at the **next header of any kind**; `[[sources.x]]` does not match. Added the `kinds` sub-table scenario. tasks 1.2 and 1.4 carry it. | `specs/manifest-format/spec.md` R2; `tasks.md` 1.2, 1.4 |
| WARNING | `specs/manifest-format/spec.md`, `tasks.md` | "Every other byte SHALL survive: comments" and "preserving the bytes before `=`" contradicted each other — the latter deletes a comment trailing the rev line. The converse mistake (cutting the value at the first `#`) corrupts a rev containing `#`, which git permits. | Specified the value's extent explicitly: it ends at its closing quotation mark, and only the span between the quotes is replaced. Added *A comment trailing the rev line survives*, including the `#`-in-value case. tasks 1.5 states the mechanism. | `specs/manifest-format/spec.md` R1, R2; `tasks.md` 1.1, 1.5 |
| WARNING | `specs/manifest-format/spec.md` | A multi-line `rev` value parses through `manifest.Parse`, so it reaches `SetRev` in the real pipeline; a line-oriented rewrite strands the continuation and produces a `graft.toml` that no longer parses — which the spec's own round-trip clause forbids. | Required the value's closing quote to be on the same line and refused everything else with the existing `cannot move the pin` message. Added *A multi-line rev value is refused rather than half-rewritten*. | `specs/manifest-format/spec.md` R2 |
| WARNING | `specs/update-execution/spec.md`, `tasks.md` | Nothing checked that the edit landed on the key it meant to. A commented-out `rev` above the real one, or a sub-table key, would be edited while the run silently resolved the old rev. A re-parse alone catches corruption, not a wrong target. | Made the re-parse a SHALL and added the value assertion beside it: the run fails unless the re-parsed manifest's rev for that source is exactly what was asked for. Comment lines are skipped by spec. tasks 4.4 owns it, and design.md → Decisions explains why it exists. | `specs/update-execution/spec.md` R3; `tasks.md` 1.5, 4.4; `design.md` → Decisions |
| WARNING | `design.md`, `tasks.md` | Writing `graft.toml` through `apply`'s `writeFile` unlinks it before recreating it with `O_EXCL`. `graft.toml` is the one file graft cannot regenerate, and the removal's rationale — stopping a *source* from preserving an exec bit — does not apply to graft's own write. A failure in that window deletes the consumer's request, contradicting the spec's own "leaves `graft.toml` byte-identical". | Specified a temporary file plus `(*os.Root).Rename`, with the temporary removed on failure, and a scenario asserting no temporary survives either outcome. design.md → Decisions records the reasoning and states the mode consequence rather than hiding it. | `specs/file-application/spec.md` ADDED; `tasks.md` 2.3; `design.md` → Decisions, Risks |
| WARNING | `specs/update-execution/spec.md`, `tasks.md` | `openspec/config.yaml`'s tasks rules name a concentration point — a change touching the prune set needs a test proving a foreign file in a shared destination survives — and the change adds a new way to reach the prune set (a moved pin whose new rev stops providing an item) with no scenario, no matrix row, and no task. | Added *An item the new rev no longer provides is removed, and a repo-owned file beside it survives*, its matrix row, its own RED task, and the concentration-point block on group 3. | `specs/update-execution/spec.md` R1; `tasks.md` group 3 header, 3.2; `design.md` → matrix |
| WARNING | `specs/update-execution/spec.md` | The `--to` requirement never said what happens when `SetRev` refuses the manifest's shape end to end, nor in which order the two possible refusals fire. `graft update --to v1.1.0 sharde` has two true messages available and only one is useful. | Fixed the order in the requirement — membership first, edit second — and added *A `--to` against a manifest shape the editor cannot rewrite is refused*, asserting nothing is fetched and that `graft update shared` still works on the same manifest. | `specs/update-execution/spec.md` R3; `tasks.md` 3.6, 4.1 |
| WARNING | `design.md` → Test Boundaries | The table said the consumer's working tree is "not used" at the unit tier while five matrix rows and task 2.1 drive `internal/apply` against a real `t.TempDir()`. A task would have been inventing a boundary the table denies. | Split the cell: real `t.TempDir()` for `internal/apply`, not used for `SetRev`, the report, and the renderer. Added a row asserting this repository's own tree is unreachable from every tier. | `design.md` → Test Boundaries |
| WARNING | `tasks.md` group 8 | The documentation group amended SPEC.md's Failure modes table but left SPEC.md → Invariants reading "graft never writes inside `.git`, and never over `graft.toml` or `graft.lock`", which `--to` makes false. | Added task 8.2 scoping that invariant to paths arriving in a plan and naming graft's own manifest write as the exception, after confirming the sentence is not duplicated in AGENTS.md or ENGINEERING.md. | `tasks.md` 8.2 |
| SUGGESTION | `proposal.md`, `specs/update-execution/spec.md` | "`--to` SHALL be the only way `graft` writes `graft.toml`" is false against SPEC.md's own command table, which lists `graft add`. Archived, it would have become a constraint `add-command` violates. | Scoped to `graft update`, with `add` named as the other writer. | `specs/update-execution/spec.md` R3; `proposal.md` → What Changes |
| SUGGESTION | `specs/update-execution/spec.md` | The report requirement restated `sync-report`'s header, summary, and colour contract, and duplicated a scenario `sync-report` already owns — a second copy of a format to keep in sync, against a proposal that claims the capability is "reused unaltered". | Trimmed to a reference: `sync-report` is the owner, and this requirement adds only that an update is the run in which its two-sided header forms appear. | `specs/update-execution/spec.md` R5 |
| SUGGESTION | `specs/update-execution/spec.md` | Three under-specified scenarios: an unknown flag with no asserted text, and no coverage of a zero-source `graft.toml` (which `manifest-format` makes a supported state). | Spelled `graft: unknown flag: --force`, and added *A manifest declaring no sources updates nothing*. Two `file-application` scenarios were reworded to name their assertion and their trigger. | `specs/update-execution/spec.md` R1, R7; `specs/file-application/spec.md` |
| SUGGESTION | `design.md` | Two factual slips: "thirty-odd" `apply.Run` call sites (there are 56), and "the two help scenarios that assert byte-identical stdout" when `internal/cli` asserts substrings plus a relation, with a comment saying so deliberately. | Corrected the count; rewrote task 6.5 to name the scenarios rather than a golden file. | `design.md` → Contracts; `tasks.md` 6.5 |
| SUGGESTION | `design.md`, `tasks.md` | "Control character" was undefined; a `< 0x20` reading admits DEL and the C1 range, both invalid inside a TOML basic string. | Pinned to `unicode.IsControl` in the spec, the design, and task 1.4. | `specs/manifest-format/spec.md` R3; `design.md`; `tasks.md` 1.4 |
| SUGGESTION | `design.md` | The property that makes the character denylist sufficient — that a rev can never reach git as an operand, because `source.Resolve` interpolates it into `refs/...` and passes `--` — was load-bearing and unwritten, so a later refactor could remove the prefixing without noticing. | Recorded in design.md → Decisions beside the character rule. | `design.md` → Decisions |
| SUGGESTION | `design.md` | `sync.Options.Update{Source: "", To: "x"}` was unguarded, and `internal/sync` is the API the integration tier drives directly. | Documented as a precondition on the field with the reason a guard was not added: the command surface refuses it as a usage error, and an unreachable branch is a hole in the coverage gate rather than a safety net. | `design.md` → Contracts |
| SUGGESTION | `tasks.md` | Group 4 is what actually causes `graft.toml` to be rewritten and had no contract gate; task 2.4 read as an unconditional pre-flight addition, which would have contradicted "`graft sync` neither reads nor writes it"; task 8.4 invited a needless rewrite of a rule this change depends on; task 1.6 paraphrased a contract string inside quotes. | Added the group 4 gate (4.6), scoped 2.4 to runs given manifest bytes, changed 8.4 to state the expected outcome is *no edit* to the `sync` rule, and quoted the contract string exactly. | `tasks.md` 2.4, 4.6, 8.4, 1.6 |
| SUGGESTION | `design.md` → matrix | One matrix row named the outer loop with a `update-execution:` prefix, making it look like a spec scenario and breaking a mechanical one-row-per-scenario audit. | Relabelled **outer loop** (no spec scenario), and the matrix now carries all 56 scenarios across the four spec files with *existing* marking the ones a MODIFIED requirement carries unchanged. | `design.md` → Verification matrix |

## No Remaining Implementation-Blocking Gaps

None remain. `openspec validate update-command --strict` is clean after the repairs. Every
scenario in all four spec files has a verification-matrix row and a task; no task reaches for a
collaborator the Test Boundaries table does not name; no PRD non-goal is crossed; and all three
reviewers agreed no new code path lets a source repository cause anything to execute, influence
which rev is resolved, or influence what is written into `graft.toml`.

## Deferred Non-Blocking Notes

- **Byte-level `SetRev` cases that are safe by refusal rather than by handling** — indented keys
  and a whitespace-decorated header are handled; array-of-tables and a duplicated `[sources.x]`
  cannot reach `SetRev` through the real pipeline because `manifest.Parse` refuses them first, and
  exist only as unit-tier cases. Recorded here so a later reader does not mistake the unit tests
  for the reachable surface.
- **A shared `graft.toml` text-editing helper.** `add-command` will also write `graft.toml`, to
  insert a source rather than to move a rev. Whether that becomes one helper beside `SetRev` is a
  decision for that change, when there are two callers to generalise over rather than one to
  speculate about. Resolution point: `add-command`'s own design.md.
- **A two-file atomic commit of `graft.toml` and `graft.lock`.** The rename closes the window in
  which the manifest is absent; the window in which the manifest has moved and the lock has not is
  still open, by one write. It is recoverable — the next `graft sync` names both revs and points at
  `graft update` — and closing it means a temporary directory and a rename dance for a state a
  `git diff` already shows. Recorded in design.md → Decisions and Risks; not planned.
