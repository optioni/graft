## Reviewed Artifacts

- `openspec/changes/catalog-hardening/proposal.md`
- `openspec/changes/catalog-hardening/specs/catalog-format/spec.md`
- `openspec/changes/catalog-hardening/design.md`
- `openspec/changes/catalog-hardening/tasks.md`

Read as sources of truth, not reviewed: `SPEC.md`, `PRD.md`, `ENGINEERING.md`, `AGENTS.md`,
`openspec/config.yaml`, the eleven live specs under `openspec/specs/` (chiefly
`catalog-format`, `destination-computation`, and `source-listing`), `internal/catalog/`,
`internal/plan/`, `internal/source/`, and the five archived changes — in particular
`2026-08-24-catalog-and-selectors/tasks.md` §9.4, which states the four deferred findings
this change exists to close and the rejections it must not reopen.

**Delegation.** The finding pass was delegated to **two** fresh subagents, neither of which
wrote these artifacts and neither a fork of the authoring session. One took capability
coverage, scenario quality, task alignment, cross-artifact contradictions, and scope
fidelity; the other took design completeness, test boundaries, tier placement, and the
empirical truth of the plan's factual claims. Both were told to report findings only and to
edit nothing; both were asked to verify claims by running them, in a scratch Go module
outside the repository pinned to the same `goccy/go-yaml v1.19.2`. Both complied — `git
status` showed only this planning directory throughout — and every repair below was made by
the authoring session.

The empirical instruction earned its keep twice over. Both reviewers independently found
the same CRITICAL by *building* the planned implementation and running the plan's own
scenarios against it; one of them then found a second CRITICAL that no amount of reading
would have produced, because it turns on a decoder quirk. The authoring session reproduced
both before repairing anything.

## Reviewed Against

- This repository HEAD: `232414f695925fe132828399147b2774a6ec404d`
- Sibling repository (`optioni/openspec-schemas`) HEAD: `Not applicable` — this change reads
  no catalog, installs nothing, and does not touch the hand-copied `openspec/schemas/tdd/`
  or `.claude/agents/`.
- Working tree: clean apart from `openspec/changes/catalog-hardening/`, which is this
  planning package and is intentionally included.
- Toolchain used for every empirical check: Go `1.27.0` (darwin/arm64),
  `github.com/goccy/go-yaml` `v1.19.2` (the version `go.mod` pins), `openspec` `1.9.0`.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | design.md → D2/D3; specs → *Content after a separator is reported even when it is malformed*; tasks.md 1.1, 1.4–1.5 | **The planned multi-document guard could not satisfy its own scenario.** The plan counted documents by decoding twice and treating anything but `io.EOF` from the second `Decode` as the fault. goccy parses the whole stream on the *first* `Decode`, so for `<valid>\n---\nkinds: [unclosed` the syntax error surfaces there and the second decode returns `io.EOF`. Both reviewers implemented the plan verbatim and got `catalog.yaml: [6:8] sequence end token ']' not found` with the file quoted back — the decoder's complaint, which the scenario says must not be reported. The RED test task 1.1 prescribes could never have gone green. | Replaced the mechanism, not the scenario. The count is now taken from `lexer.Tokenize`'s token stream **before** decoding, which is error-tolerant: the lexer reports two documents for that input and the multiple-documents message is returned without the decoder ever running. Verified. | design.md → D2, D3, Boundaries; tasks.md group 1 (rewritten); specs → requirement text |
| CRITICAL | specs → *A catalog is a single YAML document*; design.md → D1; tasks.md 1.4 | **The planned guard did not close the finding it exists to close.** goccy returns `io.EOF` after the first document when two markers are adjacent, so `version: 1\n---\n---\nkinds:\n  b:\n    to: "y/"\n` decodes to the first document with a **nil error** and kind `b` silently gone — the exact silent under-install the requirement is for. The mirror case `---\n---\nversion: 1\n` decodes to nothing and reports `version is required` for a file that plainly declares one. Reproduced by the authoring session before repair. | The token-level count answers both correctly: an empty region *between* two markers is a document, while an empty region before the first marker or after the last is not — which also keeps a leading separator, a trailing separator, and both together as one document. Two new scenarios pin the two cases, and task 1.3 names them as distinct expected failures so an implementer cannot satisfy the group with a decoder-based count. | specs (+2 scenarios, requirement text); design.md → D2; tasks.md 1.1, 1.3, 1.4 |
| WARNING | proposal.md → Why bullet 4; design.md → D7 | **The stated harm was false.** "Every item of that kind would be written into one directory twice" — it would not. `destination-computation` requires a destination in cleaned form (`insideRepo` demands `path.Clean(p) == p`), so `internal/plan` already refuses `.claude/agents//` with `destination ".claude/agents//" escapes the repo root`. Measured against the real `compute`. The finding is a message-quality and layering repair, not a repair of a double write. | Both restated: the catalog is refused two layers later, for the wrong fault, and only if the kind has an item at all. D7 now reconciles with `destination-computation`'s cleaned-form requirement as well as its trailing-slash one. | proposal.md → Why; design.md → D7 |
| WARNING | proposal.md → Why ¶1; design.md → Context | **"`sync-command` … first hands `Load` a path inside a fetched repository" is false.** `internal/source.ReadCatalog` (landed in `git-fetch`) reads through an `os.Root` and calls `catalog.**Parse**`; it calls `Load` only on the absence branch. So findings 1, 3 and 4 become reachable at `sync-command` through `Parse`, while finding 2 stays unreachable through that caller even afterwards — design.md → D4 admitted this while the Why contradicted it. | Reworded in both: three findings are user-reachable at `sync-command`; the symlink check is hardening of an exported entry point whose doc comment claims something untrue, stated as that and not dressed up. | proposal.md → Why ¶1; design.md → Context |
| WARNING | design.md → Boundaries, `internal/source` row | **An observable `internal/source` behavior change was asserted not to happen.** A *dangling relative* link named `catalog.yaml` inside a fetched entry makes `os.Root.ReadFile` return `fs.ErrNotExist`, so `ReadCatalog` delegates to `Load` — which after this change answers `catalog.yaml: not a regular file` where it previously answered the not-graftable message. Demonstrated on two trees, pre- and post-change. | The row now names the case, and states why no `source-listing` delta is needed: its not-graftable scenario is a tree with *no* `catalog.yaml`, and its requirement already says an invalid catalog surfaces `internal/catalog`'s message. Task 2.6 checks it by hand rather than leaving it to be discovered. Impact in proposal.md names it too. | design.md → Boundaries; tasks.md 2.6; proposal.md → Impact |
| WARNING | design.md → Test Boundaries (`git`, network, fetch cache rows) | The rows said flatly "not involved — no fixture git repository, no `$HOME`", while tasks 2.6, 4.9 and 7.3 run the `internal/source` suite, which builds fixture git repositories and reads `$HOME`. The design gave `internal/plan` exactly this carve-out and not these. | Same carve-out extended and made explicit — "not involved in any test this change writes; running a neighbouring suite unchanged is a command, not a collaborator" — with the tasks named, plus a dedicated `internal/source` row. | design.md → Test Boundaries |
| WARNING | tasks.md 5.4, 7.5 | Both prescribed `openspec validate --change catalog-hardening --strict`, which the installed CLI rejects: `error: unknown option '--change' (Did you mean --changes?)`. | Corrected to `openspec validate catalog-hardening --strict`, the form the archived changes use, with the flag trap noted so it is not "fixed" back. | tasks.md 5.4, 7.5 |
| WARNING | tasks.md 1.2, 4.2, 4.3 | Three tasks labelled **RED** whose tests pass against today's code — task 4.4 said so outright. Under the `tdd` schema RED means a test that fails first, and an implementer following the label literally would distort a fixture to force a failure. | Relabelled **GUARD (expected green, before and after)**, with what each guards stated. Only genuinely failing cases remain under RED. | tasks.md 1.2, 4.2, 4.3, 4.4 |
| SUGGESTION | design.md → Test Strategy | "33 scenarios, of which 22 are regressions" — 33 was right before the repairs, but the regression count was 18, and the delta has since grown to 37. | Corrected and moved to a single sentence: 37 scenarios, 18 regressions, 19 new. Re-counted after every repair. | design.md → Test Strategy |
| SUGGESTION | design.md → verification matrix | `go test ./internal/catalog/ -run TestParse_Empty` was given as the command for a row naming two tests; it never runs `TestParse_NoKindsNoProvides`. | Pattern widened to `-run 'TestParse_Empty\|TestParse_NoKindsNoProvides'`. Every other `-run` pattern in the matrix was checked against the real test names and matches. | design.md → verification matrix |
| SUGGESTION | design.md → D6; tasks.md 3.4 | The shape rule admitted `_` separators *and* defined "too wide" as "`ParseUint`/`ParseInt` cannot hold it". `ParseUint("99…9_0", 10, 64)` fails with `ErrSyntax`, not `ErrRange`, so an implementer conflating the two would route an underscore-separated wide literal back to the non-integer message the rule exists to avoid. | Stated explicitly in the requirement, the decision, and the task: strip `_` before the range test, and count only `strconv.ErrRange`. | specs → version requirement; design.md → D6; tasks.md 3.4 |
| SUGGESTION | design.md → Risks; tasks.md 7.3 | The coverage note was wrong in both directions: the baseline is **99.5%**, not 99.8%, and a prototype of the four repairs lands at **96.9%** with eight uncovered statements — not "two lines". | Real numbers written in, the uncovered statements named, and three version scenarios (`+99…`, the quoted wide literal, the quoted `"-1"`) added partly to cover the shape helper's branches. Task 7.3 now asks for the measured number rather than restating an estimate. | design.md → Risks; tasks.md 7.3; specs (+1 scenario, +1 case) |
| SUGGESTION | specs → *A catalog is a single YAML document* | The requirement said a separator followed by "whitespace **or comments**" is not an error, but no scenario pinned the comment case. | Folded into the trailing-separator scenario, which now covers the bare and comment-followed spellings. A scenario for `---` inside a quoted string and inside a block scalar was added at the same time, since the token-level count makes that claim load-bearing. | specs (+1 scenario, 1 extended) |
| SUGGESTION | specs → version gating; design.md → Risks | Risks accepted that a *quoted* 26-digit string reports "upgrade graft", but no scenario pinned it while its sibling `version: "1"` was pinned — an accepted consequence recorded only in a footnote. | Stated in the requirement text and pinned by the new *A sign or separators do not change the answer* scenario. | specs → version requirement (+1 scenario) |
| SUGGESTION | specs → Kind declarations; design.md → D7 | The duplicate rule reads as if it collapses every redundant spelling, but `a/b/.` and `a/b/` are not collapsed — cleaning removes the `.` and leaves no trailing slash to compare. | One clause added saying so, and why nothing rests on it: `destination-computation` refuses an uncleaned destination outright. | specs → Kind declarations |
| SUGGESTION | specs → *An uncleaned destination is carried verbatim* | The scenario documents a catalog `internal/plan` refuses outright; a reader could take it as a supported catalog. | An **AND** clause naming that, and why the two rules are separate. | specs → Kind declarations |
| SUGGESTION | design.md → D3 | "`Decode` on empty input returns `io.EOF` leaving the target untouched" — it returns `io.EOF` but zeroes the target. Harmless for `var doc any`, but the stated reason was not the real one. | Moot after the CRITICAL repair: `yaml.Unmarshal` stays, so the claim was deleted rather than corrected, and D3 now argues why the decode path is left alone. | design.md → D3 |
| SUGGESTION | proposal.md → What Changes | `openspec/config.yaml` → `rules.proposal` says to mark **BREAKING** any change to the `catalog.yaml` format; the proposal argued an exemption instead of marking it. | Marked **BREAKING (format)** on the document rule, with the scope of the break stated plainly in the paragraph that follows — including why `version` stays at `1`. The argument was worth keeping; skipping the marker was not. | proposal.md → What Changes |

## Checks That Found Nothing

Recorded because most were run mechanically rather than by eye.

- **Capability coverage.** `specs/` holds only `catalog-format`; the proposal declares New =
  none and Modified = `catalog-format`; no artifact introduces a capability the proposal does
  not name. The Impact line "one requirement added, three rewritten" matches the delta.
- **MODIFIED requirements restated in full.** A programmatic diff of every scenario in the
  three MODIFIED requirements against `openspec/specs/catalog-format/spec.md`: all 18 live
  scenarios present and **byte-identical**, none dropped, none silently altered, relative
  order preserved, and the prose changes confined to the three declared rules.
- **Scenario ↔ matrix ↔ task.** All 37 delta scenarios have exactly one verification-matrix
  row, and every non-regression scenario maps to a task. Re-counted after each repair.
- **Discriminating scenarios.** Each new refusing case was confirmed to fail against today's
  code, and for the *right* reason: the symlink case fails by succeeding; the wide and
  negative literals currently report "must be an integer"; the two-spelling duplicates
  currently parse; the four document cases each fail differently, which task 1.3 now records.
- **Error strings.** Every asserted string matches character for character across specs,
  design's Contracts table, and tasks. No message this change does not claim to move is
  touched — checked against the 29 strings the previous change's review audited.
- **Implementability.** One reviewer applied all four repairs to a scratch copy of the real
  package and ran the whole suite: green, 10 packages, including `TestParse_BadYAML`,
  `TestParse_Shape`'s empty-file case, `TestLoad_Unreadable`, `TestReadCatalogSymlinkEscape`,
  and `internal/plan`'s two trailing-slash tests. Every new scenario except the two CRITICALs
  passed with the exact spec text.
- **D7 against `destination-computation`.** The proposed key is isomorphic to
  `internal/plan`'s `destKey` with `dir=false`: `docs/{name}` and `docs/{name}/` stay two
  destinations at parse time, so the archived requirement *The same pair is two destinations
  for a file item* survives. No plan test builds a catalog through `catalog.Parse` — all use
  `catalog.Kind{To: …}` literals — so no existing test changes meaning.
- **Decoder and lexer facts** (both reviewers, independently, against the pinned version):
  trailing `---`, `---` with no newline, CRLF, a comment-only trailing document, and `...`
  all leave the count at one; a second mapping, a scalar, `null`, `~`, and `--- foo` are all
  second documents; `---` inside a block scalar and inside a quoted string produce **no**
  marker tokens; the lexer tolerates a malformed second document and still counts it.
  Literal types: 26-digit and `+`/`-` 26-digit and wide `0x…`/`0o…`/`1e3` and
  underscore-separated wide → `string`; `9223372036854775808` and `18446744073709551615` and
  `007` and `1_0` → `uint64`; `1.5` → `float64`; `true` → `bool` — so the existing
  `uint64 > MaxInt64` branch stays live and the shape rule separates the rest exactly as D6
  claims.
- **Scope fidelity.** Exactly the four §9.4 deferred findings, one behavior group each.
  Neither rejection is reopened: `internal/itemid` is not touched anywhere, and no
  nil-receiver guard on `Expand` is proposed. Both are restated as Non-Goals.
- **PRD non-goals.** No dependency resolution, registry, merge behavior, auth layer, or
  runtime dependency on graft. `go.mod` is unchanged.
- **No execution path.** Every added code path is an `os.Lstat`, a token scan, a `switch`
  branch, and a map key. Nothing a source repository provides is run, and no new file is
  opened from a source tree.
- **Structural task rules.** Seven groups, each with exactly one `<!-- kind: -->` marker
  (four behavior, three operational); contract gates in all four parsing groups; an
  independent Change Review group before final verification; `task lint`, `task cover`, and
  `task build` as separate commands; the four inapplicable concentration points stated rather
  than omitted.
- **Design rules.** All six required elements present and now truthful: package-by-package
  Boundaries, the explicit "no new write path in `internal/apply`", Test Boundaries with rows
  for filesystem, `git`, network, and fetch cache, a verification matrix with one row per
  scenario, and the statement that `graft.lock`'s format is unaffected and its `version` does
  not move.
- **Task 5.1's premise.** `openspec/IMPLEMENTATION-ORDER.md` has a Phase 1 table and a "Notes
  on the ordering" section and does not mention `catalog-hardening`; `sync-command` has not
  started.
- **Baseline.** `openspec validate catalog-hardening --strict` passes, and the repository
  suite was green before any probing.

## No Remaining Implementation-Blocking Gaps

None remain. Both CRITICALs were repaired by changing the mechanism rather than the
requirement, and the repaired mechanism was verified against every case either reviewer
raised, including the two that defeated the original one. No finding is unowned, and no
decision is waiting on the user.

## Deferred Non-Blocking Notes

- **`a/b/.` is not collapsed against `a/b/`** by the duplicate rule. Resolution point: it
  never needs to be — `destination-computation` refuses an uncleaned destination outright —
  and the spec now says so rather than leaving the gap for a reader to find.
- **A quoted literal too wide to hold is reported as a version**, not as a string.
  Resolution point: pinned by a scenario and argued in design.md → Risks; separating it would
  require holding the YAML AST rather than the decoded value, which is a structural change to
  `Parse` for a distinction with no consequence.
- **The TOCTOU window between `Lstat` and `ReadFile`** stays open. Resolution point:
  design.md → D4 and Risks; closing it needs `O_NOFOLLOW`, and the window matters only to an
  attacker who already has write access to the consumer's fetch cache.
- **The `os.ReadFile` error branch is left uncovered**, taking `internal/catalog` from 99.5%
  to roughly 97%. Resolution point: design.md → Risks; the only portable way to reach it is a
  permission denial, and such a test inverts under a root CI runner.
