## Reviewed Artifacts

- `openspec/changes/command-surface/proposal.md`
- `openspec/changes/command-surface/specs/command-invocation/spec.md`
- `openspec/changes/command-surface/specs/command-output/spec.md`
- `openspec/changes/command-surface/design.md`
- `openspec/changes/command-surface/tasks.md`

Read as sources of truth, not reviewed: `SPEC.md`, `PRD.md`, `ENGINEERING.md`, `AGENTS.md`,
`Taskfile.yml`, `.golangci.yml`, `.github/workflows/`, `go.mod`, `cmd/graft/main.go`,
`internal/buildinfo/`, `openspec/IMPLEMENTATION-ORDER.md`, `openspec/config.yaml`, the nine
main specs under `openspec/specs/`, and the four archived changes.

**Delegation.** The finding pass was delegated to **two** fresh subagents, neither of which
wrote these artifacts and neither a fork of the authoring session, each given the change
directory and one slice of `openspec/config.yaml`'s review list — capability coverage,
scenario quality, task alignment, and cross-artifact contradictions to the first; design
completeness, test boundaries, tier placement, and verification-that-stops-short to the
second. Both were told to report findings only and to edit nothing, and both were asked to
verify the plan's factual claims empirically in a scratch directory outside the repository.
Every repair below was made by the authoring session; the reviewers changed nothing.

The empirical instruction earned its keep: the two CRITICALs and three of the WARNINGs were
found by *running* the plan's claims rather than by reading them, and both reviewers
independently found the same wrong import set.

## Reviewed Against

- This repository HEAD: `da483f6aa86f836d5ead592778bf3b2cacc82b54`
- Sibling repository (`optioni/openspec-schemas`) HEAD: `Not applicable` — this change reads
  no catalog, installs nothing, and does not touch the hand-copied `openspec/schemas/tdd/`
  or `.claude/agents/`.
- Working tree: clean apart from `openspec/changes/command-surface/`, which is this planning
  package and is intentionally included.
- Toolchain used for every empirical check: Go `1.27.0` (darwin/arm64), cobra `v1.10.2`,
  `golang.org/x/term` `v0.45.0`, `openspec` `1.9.0`.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | specs/command-invocation/spec.md → *cmd/graft holds no decision of its own*; design.md → verification matrix; tasks.md 6.3 | **The one scenario that polices `cmd/graft` asserted an import set a correct implementation cannot have.** It required `os`, `internal/buildinfo`, and `internal/cli`, but design.md → Contracts spells `main()` out in full and it carries the build strings as *strings*: `buildinfo.Format` is called from `internal/cli`. Both reviewers built the design's exact `main.go` and got `github.com/optioni/graft/internal/cli` and `os`, two lines, lexically sorted. The scenario would have failed against correct code, and the obvious "fix" during implementation is to add a spurious `buildinfo` import — the exact opposite of what the requirement is for. | Corrected to exactly `github.com/optioni/graft/internal/cli` and `os`, in that order, with an explicit clause naming a `buildinfo` import in `cmd/graft` as itself the failure. | specs/command-invocation/spec.md; design.md → Boundaries (`internal/buildinfo` and `cmd/graft` rows) and verification matrix; tasks.md 6.3 |
| CRITICAL | design.md → D2; specs/command-invocation/spec.md → exit-code requirement; specs/command-output/spec.md → stream requirement | **The write-failure guarantee was false on the two most common invocations.** D2 handed cobra the process's real streams, and `cobra.Command.Help()` returns `nil` unconditionally, discarding its renderer's write error. Reproduced in a faithful prototype: with a stdout failing every write, `graft --version` correctly exits `1`, while `graft --help` and `graft` with no arguments exit **`0` with both streams empty** — two failed writes, zero bytes of diagnosis. The plan would have shipped green tests beside a SHALL its own design violated, and the "`ui` is the only package that writes" requirement was untrue of help output regardless. | `ui` now wraps each stream in a recorder and exposes `Out()`/`Err()`; cobra is given those instead of the real streams, so every byte it renders is recorded and a failed help write reaches `WriteError()`. New requirement clause, new scenario *Help that cannot be written is not a silent success either*, new RED task, and the stream requirement reworded from "only package that writes" to "owns both streams; every stream any other component is handed is one `ui` wrapped". | design.md → D2, D11, Contracts (`Out`/`Err`); specs/command-invocation/spec.md (+1 scenario, requirement clause); specs/command-output/spec.md (requirement reworded); tasks.md 2.5, 4.4, 5.4a |
| WARNING | tasks.md 3.4; design.md → Risks, Test Boundaries | **The "best-effort pty" task would have failed permanently rather than skipped.** On darwin `os.OpenFile("/dev/ptmx")` *succeeds* and `term.IsTerminal` on the master returns **false** — verified, and it stays false through `TIOCPTYGRANT`/`TIOCPTYUNLK`; it turns true only once the slave (`/dev/ttysNNN` via `TIOCPTYGNAME`) is opened too. The stated skip predicate ("the platform will not give one") therefore never fires, and the task goes red here and on the `macos-latest` CI leg. An implementer coding the skip off the false answer would have turned it into a test that cannot fail. | Pty case deleted. The mitigation the design already stated is now the whole of it, said plainly: the negative direction against four real objects, the pure `ColorEnabled` in both directions, and an identity assertion on the wiring; the single unasserted step is `term.IsTerminal` returning true, which is x/term's own contract. Taking `creack/pty` for one assertion was rejected as contradicting D8's own argument about what a dependency must earn. | design.md → Risks (rewritten), Test Boundaries (terminal row); tasks.md 3.3 (merged), 3.4–3.7 renumbered |
| WARNING | proposal.md → Non-Goals; design.md → D6; specs/command-invocation/spec.md | **Disabling the `completion` command does not disable completion.** `CompletionOptions.DisableDefaultCmd = true` removes the visible command, but cobra's `initCompleteCmd` is a separate, ungated registration on every `Execute`. Verified with the flag set: `graft __complete ''` exits **`0`**, writes `:0` to stdout and a directive line to stderr — bypassing `ui`, bypassing the argument validator, and producing precisely the undocumented surface D6 exists to prevent. | `Main` refuses `__complete` and `__completeNoDesc` as unrecognised first arguments, through the same `usagef` path, with the two literal names matched rather than a `__` prefix so a future command cannot be swallowed. New scenario, new RED task, D6 rewritten to say the removal costs two lines and why they belong adjacent. | proposal.md → Non-Goals; design.md → D6, Error surface; specs/command-invocation/spec.md (+1 scenario); tasks.md 5.1a, 5.6a |
| WARNING | design.md → verification matrix | Two matrix rows named `-run 'Unknown\|Help'` and `-run 'ColorEnabled\|Style'`. `-run` takes an RE2 pattern where `\|` is a **literal** pipe, so both commands match nothing and print `testing: warning: no tests to run` — a verification entry that vouches for a test it never ran. | Backslashes removed throughout the matrix. | design.md → verification matrix |
| WARNING | design.md → verification matrix; tasks.md 6.3 | The only verification for the `cmd/graft` requirement was a `go list` invocation run by hand and recorded. Nothing would go red when `sync-command` later adds an import to `main.go` — which is the regression the requirement exists to prevent, in the one directory CI never runs a test over, and design.md → Persistence and Rollout explicitly leaves `Taskfile.yml` unedited so no task step would run it either. | Promoted to `TestCmdGraftImports`, a Go test in `internal/cli` shelling out to `go list`. The `go` toolchain was already a named collaborator because the acceptance harness shells out to `go build`, so no new boundary was invented. | design.md → verification matrix, Test Boundaries (`go` toolchain row); tasks.md 6.3, 6.4 |
| WARNING | design.md → Contracts | The proposed API would have failed `task lint` as written: `Print`/`Printf` and `Bold`/`Dim` shared one doc comment each, and revive's `exported` rule — enabled in `.golangci.yml`, with the repo's baseline at zero issues — counts a shared comment as a comment on neither. Reproduced: two `exported` diagnostics. | Every declaration given its own doc comment in the Contracts block, with the reason stated so it is not "tidied" back. `Printf` and the variadic `Note` were dropped in the same pass: `unparam` flags a format parameter never given arguments, and neither had a consumer this change ships. | design.md → Contracts; tasks.md 2.4 |
| WARNING | tasks.md 0.2, 7.3 | "Build the binary once per test binary" into `t.TempDir()` is self-contradictory: a `t.TempDir()` is removed when *its* test ends, so a package-level `sync.Once` build performed inside the first acceptance test hands every later test a path that no longer exists. | Rewritten as one parent test that owns the `t.TempDir()` and runs each case as a subtest. Measured cost: ~150 ms warm, so nothing is bought by being cleverer. | design.md → Risks; tasks.md 0.1, 7.3 |
| WARNING | specs/command-invocation/spec.md → *No completion command is offered*; tasks.md 4.2 | The clause "the help text lists no `completion` command" cannot fail. Cobra omits the completion command from root help while the root has no other subcommands, so the assertion stays green with `DisableDefaultCmd` **removed** — verified. The sibling clause (`graft completion` → `unknown command`) was mutation-tested and does go red, so the requirement itself was covered. | Non-discriminating clause dropped from the scenario and from task 4.2, with the reason recorded in design.md → Test Strategy so it is not reinstated as an "obvious" extra assertion. | specs/command-invocation/spec.md; design.md → Test Strategy; tasks.md 4.2 |
| WARNING | tasks.md 5.5; design.md → verification matrix | The colour-follows-stdout task asserted "the decision reaching the `*ui.UI`", which nothing can observe: no styled output passes through `Main` in this change and `UI`'s colour flag is unexported. As written the test would be a no-op or would force an accessor open purely for the test. | Rewritten as an **identity** assertion: the `IsTerminal` stub records every writer it is asked about, and the test asserts it was asked about `Options.Stdout` and never about `Options.Stderr`. That goes red if the wiring is inverted. A matching spec scenario was added so the assertion has an owner. | specs/command-output/spec.md (+1 scenario); design.md → verification matrix; tasks.md 5.5 |
| WARNING | specs/command-invocation/spec.md | `graft: cannot write output: <err>` was declared a contract in design.md → Error surface and used in a task, but appeared in no requirement — the scenario said only "an error naming the write failure". Every other message in this change is pinned in a spec, and `openspec/config.yaml` makes error text part of the specification. | The literal prefix, and the full expected line `graft: cannot write output: disk full`, written into the requirement and into both write-failure scenarios. | specs/command-invocation/spec.md |
| SUGGESTION | specs/command-invocation/spec.md | No scenario covered a **shorthand** flag. pflag's wording differs — `unknown shorthand flag: 'v' in -v`, not `unknown flag: …` — and D5 deliberately reserves `-v`, so `graft -v` is a realistic invocation whose message was unpinned. | New scenario and a case in task 5.1, with the message asserted as pflag's own. | specs/command-invocation/spec.md (+1 scenario); design.md → Error surface, verification matrix; tasks.md 5.1 |
| SUGGESTION | tasks.md group 0 | Group 0 opened with a CHANGE (creating `internal/cli/cli.go`) before its RED task, reading as CHANGE → RED in a behavior group. | Folded into 0.1, the template's "set up the harness" slot, and the group renumbered so the RED task is 0.2. | tasks.md 0.1–0.3 |
| SUGGESTION | proposal.md → Impact | Impact listed packages, specs, `cmd/graft`, and `go.mod`, and named what is *not* touched — but omitted the documentation edits tasks 8.2 and 8.3 make to SPEC.md (the product contract) and AGENTS.md. | Both added, with 8.3 noted as a rewrite rather than an addition. | proposal.md → Impact |
| SUGGESTION | design.md → D11 | A failed write to **stderr** would be reported as `cannot write output:` and exit `1`, which is defensible but was unstated either way. | Stated: both streams fail the run with the same message, and why the wording does not distinguish them. | design.md → D11 |
| SUGGESTION | specs/command-invocation/spec.md | The spec said `help` was "deliberately left unspecified", which understated it: today it deterministically yields `unknown command "help"`, and it silently becomes cobra's help command the moment `sync` is registered. | Said explicitly, with the note that `sync-command` inherits the transition. | specs/command-invocation/spec.md |
| SUGGESTION | design.md → Test Boundaries | The filesystem row said "not used at all. No unit test creates, reads, or deletes a file", which tasks 3.3's `/dev/null` and `os.Pipe` contradict; the `go` toolchain row named only `go build` while task 6.3 uses `go list`; the network row said "not used" while group 1 contacts the module proxy. | Filesystem row split and scoped to the repository graft runs in, with a separate build-output row; toolchain row names both commands; a module-proxy row added stating that group 1's contact is a human running `go get`, not a test. | design.md → Test Boundaries |
| SUGGESTION | specs/command-output/spec.md | The scenarios for an unset `NO_COLOR` and a present-but-empty `NO_COLOR` are the same input: a `Getenv`-shaped function reports both as `""`. Counting them as two distinct verifications overstated the coverage. | Requirement now says the two are deliberately one case and why; the scenario says so too. | specs/command-output/spec.md |
| SUGGESTION | tasks.md | `openspec/config.yaml` names six task concentration points; four do not apply here (prune set, `internal/plan` purity, lock determinism, fixture git repos) and were silently absent, where the previous change's review treated stating the N/A as the standard. | A leading comment records all four as not applicable, with one reason each. | tasks.md (head) |

## Checks That Found Nothing

Named because most were run mechanically rather than by eye, and because a check that found
nothing is only worth recording if it could have found something.

- **Capability coverage.** Both capabilities named in the proposal have delta specs; neither
  name collides with the nine live specs under `openspec/specs/`; nothing in design.md or
  tasks.md introduces a capability the proposal does not name; "Modified: None" is accurate.
- **Scenario ↔ matrix ↔ task coverage.** All 30 `#### Scenario:` headings — 17 in
  `command-invocation`, 13 in `command-output` — appear in design.md's verification matrix,
  one row each, and every one has a task. Counted independently by both reviewers before the
  repairs and re-counted after.
- **Group structure.** Every group carries exactly one kind marker. Behavior groups keep
  RED → GREEN → REFACTOR; operational groups keep CHECK → CHANGE → VERIFY. Group 1 correctly
  treats the `go.mod` additions as operational with `task build`/`task lint` as the evidence,
  rather than inventing a RED test that asserts a dependency string.
- **SPEC.md fidelity.** The colour rule is honoured literally — one decision, taken from
  stdout, applied to both streams — with D7 arguing the surprising half rather than silently
  refining it. Exit codes are `0` and `1` with no third code.
- **PRD non-goals.** None crossed. No dependency resolution, no registry, no merge behavior,
  no auth layer, no runtime dependency on graft. No new code path lets a source repository
  cause anything to execute: this change contacts no source, reads no catalog, registers no
  command, and opens no file for writing.
- **Cobra behavior, verified against v1.10.2 rather than recalled.** `SilenceErrors` plus
  `SilenceUsage` makes `Execute` return the error and print nothing. The root's own `Args`
  validator **is** consulted for an unrecognised argument both with and without subcommands
  registered, so D3's claim that it keeps working once `sync` lands holds.
  `DisableDefaultCmd` does remove the visible `completion` command. The built-in `help`
  command is absent while the root has no subcommands. pflag's message is exactly
  `unknown flag: --nope`.
- **D8's justification, verified.** For an `*os.File` opened at `/dev/null`,
  `term.IsTerminal` is false while `fi.Mode()&os.ModeCharDevice != 0` is true — so the
  dependency-free idiom would have emitted colour into `graft sync > /dev/null`, and the
  dependency genuinely earns its place. Pipes and buffers answer false as expected.
- **Dependency versions and resolution.** cobra `v1.10.2` and `golang.org/x/term` `v0.45.0`
  are the newest stable releases on the proxy today; `go get` plus `go mod tidy` resolve
  cleanly on Go 1.27.0, and the indirect closure is exactly `pflag v1.0.9`,
  `mousetrap v1.1.0`, `x/sys v0.47.0` as the proposal states.
- **The acceptance-test placement argument.** `task ci` is `lint` + `cover` + `build`;
  `cover` runs `go test` over `./internal/...`; `.github/workflows/` runs `task ci` and
  nothing else in the `ci` job and only `task build` in the dogfood job. A test beside
  `main.go` would indeed never run in CI, and one in `internal/cli` does. `exec.Cmd` with
  separate stdout/stderr buffers and an exit status via `errors.As` on `*exec.ExitError`
  behaves as the plan assumes.
- **Every error string and exit code**, reproduced byte-exactly against a cobra prototype:
  the three `unknown command` cases with their hint line, `unknown flag: --nope`,
  `source "shared": rev "v9.9.9" not found` with **no** hint line, `hello` on stdout for a
  succeeding command, empty stdout on every failure, no usage block leaking, `--help`
  byte-identical to no arguments, and `frobnicate wibble` naming only `frobnicate`.
- **Baseline attribution.** `golangci-lint run` on the repository at HEAD reports no issues,
  so the two `exported` diagnostics found above are attributable to this change's proposed
  API and not to pre-existing debt.
- **`openspec validate command-surface --strict`** — valid, before and after the repairs.

## No Remaining Implementation-Blocking Gaps

None remain. Both CRITICALs are repaired in the artifacts that own them, every WARNING is
either fixed or replaced by a stated alternative, and every SUGGESTION is applied. No
unresolved decision requires user input.

Two of the repairs changed the shape of the code rather than the words describing it — the
recording writers handed to cobra, and the `__complete` refusal — and both were written back
into design.md → Contracts and into the tasks that implement them, so no task now depends on
a boundary the design does not name.

## Deferred Non-Blocking Notes

- **A subprocess timeout** (`git-fetch` → planning review, and Q1 here). Not taken: no
  command runs `git` yet, and a timeout with no operation to bound is a number nobody can
  defend. Resolution point recorded in design.md → Q1: `sync-command`.
- **The wording of `internal/source`'s "rev not found" message** (`git-fetch` → Q3, and Q2
  here). Not taken: it is an archived capability's asserted contract, and changing it from a
  different change is the incidental edit `openspec/config.yaml` warns against. Resolution
  point recorded in design.md → Q2: `sync-command`, where it first reaches a user.
- **Shell completion** (design.md → Q3). Disabled and actively refused; revisiting it after
  `add-command`, when the command surface stops changing, is recorded as the resolution
  point.
- **Per-stream colour detection** (design.md → Q4). Rejected in favour of SPEC.md's literal
  rule; settling it differently means editing SPEC.md's Output section, and the resolution
  point is a user report or the first change with enough coloured stderr output for the loss
  to be felt.
- **`term.IsTerminal` returning true is asserted nowhere.** Accepted, argued in
  design.md → Risks, and deliberately not bought with a third dependency. It is x/term's own
  contract; graft asserts the negative direction, the pure decision, and the wiring identity.
