## Reviewed Artifacts

- `openspec/changes/destination-and-plan/proposal.md`
- `openspec/changes/destination-and-plan/specs/destination-computation/spec.md`
- `openspec/changes/destination-and-plan/specs/sync-plan/spec.md`
- `openspec/changes/destination-and-plan/design.md`
- `openspec/changes/destination-and-plan/tasks.md`

Read as sources of truth, not reviewed: `SPEC.md`, `PRD.md`, `ENGINEERING.md`, `AGENTS.md`,
`openspec/IMPLEMENTATION-ORDER.md`, `openspec/config.yaml`, the four main specs under
`openspec/specs/`, the archived `manifest-and-lock` and `catalog-and-selectors` changes, and
the existing `internal/{itemid,manifest,lock,catalog}` sources.

**Delegation deviation, stated plainly.** The schema and `openspec/config.yaml` both require
the finding pass to be delegated to reviewers that did not write the plan. This session has
no subagent-dispatch tool. The agents it can address are the workflow orchestrator that
spawned it and a `code-review` agent owned by the workflow's *implementation* phase; neither
is a planning reviewer, and a message to either is fire-and-forget with no reply guaranteed
inside this turn. The finding pass was therefore run by the authoring session against the
review list, mechanically where possible — scenario names were extracted from both spec
files and diffed against design.md's verification matrix and against tasks.md rather than
eyeballed. That is weaker than the rule asks for and is recorded rather than papered over.
tasks.md group 12 still requires an independent reviewer against the implementation diff,
where dispatch is expected to be available; the same deviation was recorded by the previous
change for the same reason.

## Reviewed Against

- This repository HEAD: `69783deaa40f28dbe6915efc2d0c984d29405e86`
- Sibling repository (`optioni/openspec-schemas`) HEAD: `Not applicable` — this change reads
  no catalog, fetches nothing, and installs nothing. The hand-copied `openspec/schemas/tdd/`
  and `.claude/agents/` are inputs to the workflow, not to the code being planned.
- Working tree: clean apart from `openspec/changes/destination-and-plan/`, which is this
  planning package and is intentionally included.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | tasks.md, design.md | The write task said the source path is `path.Join(item.From, rel)` unconditionally. For a **file** item, `Listing.Files` holds `base(From)` by design.md → D2, so the join yields `extras/agents/x.md/x.md` — every copy of every single-file item would fail at apply time, and no scenario would have caught it: the one write scenario used a directory item, and the file-item scenarios all assert destinations, not source paths. | Made the asymmetry explicit in the requirement text and gave it a dedicated scenario; corrected the implementation task; wrote the trap into design.md → Contracts. | specs/sync-plan/spec.md → *Every write names the file to copy and where it lands* (+1 scenario); design.md → Contracts; tasks.md 6.1, 6.4 |
| WARNING | specs/sync-plan/spec.md | Nothing verified that the lock `plan` builds is a lock `graft.lock`'s own parser will accept. `lock.Marshal` validates nothing, so an empty `resolved`, a duplicated source name, or a path claimed twice would serialize happily and be refused on the next `sync` — the failure surfacing one run later, in a different package, against a file the user is told not to edit. | Added the round-trip requirement clause and a scenario asserting `lock.Parse(lock.Marshal(...))` succeeds and preserves sources, items, and files, plus its own GREEN task. This is what checks the whole load-time constraint set without a second validator in `plan`. | specs/sync-plan/spec.md → *The next lock records exactly what the plan produces* (+1 scenario); design.md matrix; tasks.md 8.1, 8.3 |
| WARNING | specs/sync-plan/spec.md | Ownership transfer between sources was unspecified: a path in the lock under source `alpha` that source `beta` now produces. The requirement text covers it ("a path produced by the new resolution SHALL NOT be pruned even when the lock also claims it") but no scenario pinned it, and the obvious per-source implementation — diff each source's lock entry against its own new files — deletes a file it just wrote. | Added a scenario naming both sources and asserting an empty prune set, with the reason: the prune set is a set difference over paths, not per-source bookkeeping. | specs/sync-plan/spec.md → *The prune set is derived from the lock alone* (+1 scenario); design.md matrix; tasks.md 7.1 |
| WARNING | tasks.md | The next-lock task said "sources in manifest order (already sorted by name)", contradicting task 6.3 and design.md → D8, which sort explicitly. `Build` takes a `[]Input` a caller assembled, so "already sorted" is an assumption about a slice nothing in this package controls — and an unsorted one churns every consumer's lock diff, which is the concentration point this change is most exposed to. | Rewrote the task to sort by name in `plan`, with the reason it cannot inherit `manifest.Parse`'s ordering. | tasks.md 8.2 |
| WARNING | design.md | `Build`'s preconditions were unstated. It relies on `catalog.Parse` guaranteeing every item's kind is declared, on `manifest.Parse` giving unique source names, on `git-fetch` giving a 40-hex sha, and on `Listing.Files` holding `base(From)` for a file item. The first is the dangerous one: a catalog violating it makes `Catalog.Kinds[kind]` the zero value, and the item silently plans no writes. | Added a "Preconditions on `Input`" block naming each precondition and what guarantees it, and tied the dangerous case to the round-trip scenario that would catch its consequences. | design.md → Contracts |
| SUGGESTION | specs/sync-plan/spec.md | The scenario for a path claimed by both the lock and a second item said "fails with the collision error" without giving the text, while every sibling error scenario pins its message exactly. Error strings are an asserted contract here; one scenario left vague is one message free to drift. | Wrote the exact message into the scenario and added that nothing is pruned, since a failed build produces no plan. | specs/sync-plan/spec.md → *No two items resolve to the same path* |
| SUGGESTION | tasks.md | The consumer-override group changes how `graft.toml`'s `[sources.*.kinds]` table is interpreted but carried no contract gate, while the two groups touching `catalog.yaml` and `graft.lock` semantics both did. `openspec/config.yaml` asks for a gate on any group changing how those files are read. | Added a contract gate re-reading SPEC.md's `graft.toml` section, and renumbered the group. | tasks.md 5.4 |

Checks that found nothing, named because they are where this repository fails:

- All 49 spec scenarios appear in design.md's verification matrix — 49 rows, extracted and
  diffed by script in both directions, no orphan row and no uncovered scenario — and every
  one is named in a RED, CHECK, or VERIFY task. Counted, not eyeballed.
- No group mixes kinds. Every group carries exactly one marker, and every lifecycle runs
  evidence-first: RED before GREEN in groups 2–10, CHECK before CHANGE in groups 1, 11, 12,
  and 13.
- No `parallel-after` marker is used, correctly: groups 2–10 build one package and share its
  files, so none is independent of the one before it.
- No task invents a collaborator design.md → Test Boundaries does not name. The table gives
  the filesystem two rows (working tree, source tree), and `git`, the network, the fetch
  cache, and fixture git repositories each get one saying **not used** — which is the whole
  point of this change's tier story.
- No test in this change creates a directory, so the fixture-repo trap (`user.name` and
  `user.email` set on the repo, not the machine) cannot arise here. It returns with
  `git-fetch`.
- The prune-set group carries an explicit anti-false-green task: the foreign-file test must
  assert the repo-owned path appears in **no** field of the plan, twice — item kept and item
  dropped — because a test checking only `Prune` would stay green if the path leaked into a
  write.
- Determinism is specified and tasked as byte equality across two builds from shuffled
  inputs, with `reflect.DeepEqual` explicitly ruled out.
- The plan crosses no PRD non-goal: nothing resolves a dependency, no registry appears, no
  file is merged (a planned write is unconditional overwrite, which is the `node_modules`
  contract SPEC.md states), no credential is handled, and nothing here could make a synced
  file require graft at runtime.
- No code path lets a source repository cause anything to execute. A source's only influence
  is the destination it proposes, and the two rules that exist because it is untrusted — the
  repo-root boundary and the consumer override winning — are both enforced before any plan
  value leaves the package.
- Nothing is added to `cmd/graft`, where the coverage gate cannot see it. The change adds one
  package under `./internal/`.
- No RED task is scheduled for plumbing. Group 1 is operational and its evidence is a guard
  watched to fail against a real `import "os"`, not a test asserting a constant.

## Gaps Found During Implementation

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| WARNING | specs/destination-computation/spec.md | The escaping-listing-entry scenario was internally inconsistent. Its listing `["../../../etc/passwd"]` joined under `openspec/schemas/tdd` — three segments, three `..` — cleans to `etc/passwd`, which is *inside* the repo root and therefore raises no error at all, while the scenario's asserted message names `../../etc/passwd`. Implemented as written, the scenario could never go green; implemented to match the message, the input had to change. | Corrected the WHEN's listing to `["../../../../../etc/passwd"]`, the input that actually produces the asserted destination, leaving the error text — the asserted contract — untouched. The alternative, weakening the THEN to `etc/passwd`, was rejected: that path does not escape, so it would have turned an invariant scenario into an acceptance one. | specs/destination-computation/spec.md → *No destination escapes the repo root* |

A consequence was named here rather than left to be discovered: a listing entry with
*fewer* `..` segments than its destination has path segments is absorbed by `path.Join` and
lands somewhere else **inside** the repo. It was left unrefused at the time, on the reading
that SPEC.md's invariant is only "no destination escapes the repo root". **That reading was
wrong**, and the change review below caught it: the repo-root rule is the floor, and a file
landing anywhere other than the destination the consumer was shown defeats the only
mitigation SPEC.md offers against an untrusted source. Refused now, and SPEC.md's
Invariants list says so.

## Change Review — Findings and Dispositions

The finding pass was delegated to a separate `code-review` agent, given only the five
artifacts and `git diff aac80f3..HEAD`, and asked to attack the six concentration points
tasks.md group 12 names. It reported five of the six holding and one failing.

| Severity | Finding | Disposition |
|---|---|---|
| CRITICAL | **A listing entry could climb out of its item, in both directions.** The escape check ran on `path.Join(dest, rel)`, and `path.Join` cleans — so `..` segments in a listing entry were absorbed before the check saw them. An entry with fewer `..` segments than its destination has path segments landed anywhere **inside** the repo, `.git/hooks/pre-commit` among them, with every repo-root check passing and no cooperation needed from the catalog's `to`. The read side was worse: `Write.From` was `path.Join(item.From, rel)` and was never checked at all, so an entry could aim a **read** outside the source's fetched tree at whatever sits beside it in the cache. | **Fixed.** New `insideItem` predicate refuses a listing entry that is empty, `.`, absolute, holds a `..` segment, or is not cleaned — before any join, closing both halves at one site. New spec requirement *A listed path stays inside its item* with six scenarios, including the reviewer's `.git/hooks` and flattened-source-path cases. The repo-root check on the computed path stays and runs first, so both rules are live and both are covered. Verified by deleting the check and watching every new scenario go red. |
| WARNING | **The "two entries interpolate alike" dedupe compared raw strings**, so `["a/{name}", "a/{name}/"]` — one destination for a directory item by D4 — slipped past it, and the item then surfaced through the *cross-item* collision message naming itself as its own partner, which `sync-plan` forbids in as many words. `catalog.Parse`'s own duplicate-destination guard has the same blind spot. | **Fixed.** Keyed on `destKey(dest, dir)`: the destination's meaning rather than its spelling. One destination for a directory item, two for a file item — where they genuinely are two, one naming the file and one a directory to put it in. Both directions have scenarios. The `catalog` half is not this change's to fix and was filed separately. |
| WARNING | **A duplicated listing entry produced the same self-collision.** | **Fixed**, together with the case the reviewer did not report: two `to` entries that are genuinely different destinations, one nested in the other, can still meet on a file (`["a", "a/b"]` with `["b/x.md", "x.md"]`). Listings are `slices.Compact`ed, and a per-item map spanning every destination entry raises a new within-item message, `destinations "a" and "a/b" both place a file at "a/b/x.md"`. *One item producing the same path twice is not this error* is now true unconditionally rather than only for the two originally named causes. |
| SUGGESTION | **The purity guard saw stdlib imports, not filesystem calls through collaborators.** `plan` imports `catalog` and `lock`, both of which export a `Load`; a future `Build` calling one would read a file with the guard still green, because the import set would not change. | **Fixed.** The guard now also walks the AST for `catalog.Load`, `lock.Load`, and `manifest.Load`. Watched it fail against a real `catalog.Load` in `build.go`. |
| SUGGESTION | **"is not in cleaned form" was literally false** for every destination the requirement's own scenarios accept, since the check is applied to the destination with one trailing `/` trimmed — as it must be, or `.claude/agents/` is refused. | **Fixed** in the requirement text. No code change. |
| SUGGESTION | **`Build` can construct a lock `lock.Parse` refuses** when `Resolved` is empty, while the round-trip requirement is written without qualification. | **Accepted, resolved the other way the reviewer offered.** The requirement now names the precondition instead of `Build` re-checking it. design.md's Preconditions block deliberately makes a 40-hex `resolved` `git-fetch`'s guarantee and states that `Build` does not re-validate what collaborators guarantee; validating here would put a third copy of that rule in a third package and would need a design amendment to justify. |

Two of those dispositions were **overturned** after review, and the reasons are recorded
here rather than in a commit message alone, because both strengthen a contract:

| Finding | First disposition | Final disposition |
|---|---|---|
| `Build` can construct a lock `lock.Parse` refuses when `Resolved` is not a 40-character hex sha | Accepted; the requirement named the precondition instead of checking it | **Fixed.** The precondition does not behave like its neighbours in design.md → Contracts: unique source names, unique item ids, and no path claimed twice are consequences of what planning does, so the round-trip scenario catches a violation of any of them. A bad `Resolved` is copied verbatim into the lock, `lock.Marshal` validates nothing, and the failure surfaces one run later in a different package against a file the user is told not to edit. Five lines of check are cheaper than two packages disagreeing about what a valid `graft.lock` is. Recorded as design.md → D11, with a requirement and scenario in `sync-plan`. |
| Two items whose destinations **nest** — `docs/api` as one item's file, `docs/api/index.md` as another's — is not refused | Deferred to `sync-command`, on the grounds that SPEC.md's Invariants list does not name it and inventing an unlisted invariant is what design.md → Q2 declined for `.git/` | **Fixed here, and SPEC.md's Invariants list amended.** The Q2 parallel does not hold: `.git/` is a restriction on what a destination may *name*, which SPEC.md is silent about by choice; a nesting clash is a plan that **cannot be applied at all**. The two paths cannot both exist, so `apply` fails partway through, and because the lock is written last the file already written is left outside `graft.lock` where no prune can reach it. That orphaned state is precisely what the plan-then-apply split exists to make unreachable, so a package whose whole job is to return a value instead of writing must be able to say so before anything is touched. The invariant list is the floor rather than the ceiling. Recorded as design.md → D11, with a requirement and two scenarios in `sync-plan` covering both walk orders, and a third asserting that ordinary siblings in one directory are not a nesting. |

## No Remaining Implementation-Blocking Gaps

None remain. Every gap above is repaired in the artifact that owns it, and the change
validates under `openspec validate destination-and-plan --strict`.

One reading of an ambiguous source document was made deliberately rather than left open, and
is recorded in design.md → D4 and Open Questions Q1 so a reviewer can overturn it cheaply:
SPEC.md's "A trailing `/` means 'into this directory'" does not say whether a *directory*
item's own leaf name is appended, and the chosen reading — a no-op for a directory item,
`base(from)` appended for a file item — is the only one under which both of SPEC.md's worked
examples and its "`from` may move freely without touching any consumer" claim hold at once.
tasks.md group 11 writes the resolved rule back into SPEC.md rather than leaving the next
reader to re-derive it. It does not block implementation.

## Deferred Non-Blocking Notes

- Whether a destination may land inside `.git/` is left unrestricted. SPEC.md's invariant is
  only "no destination escapes the repo root", and inventing a narrower rule here would be
  behavior SPEC.md does not specify. Resolution point recorded in design.md → Open Questions
  Q2: it belongs to a change that argues it against SPEC.md's stated threat model.
- Whether a `Listing` faithfully describes a real fetched tree is not answerable here — this
  change never sees a tree. Resolution point recorded in design.md → Risks: it is
  `git-fetch`'s contract, tested there against real fixture repositories.
- The pin check (`lock.CheckPins`) is deliberately not called from `Build`; it must fire
  before anything is fetched. Recorded in design.md → D6, and it lands with `sync-command`.
- The `catalog.Parse` half of the raw-string duplicate-destination guard — `to: ["a/{name}",
  "a/{name}/"]` is two entries there and one destination for a directory item — is left as
  it is. `internal/catalog` belongs to an archived change, and `internal/plan` now compares
  destinations by meaning, so no plan can be built from that pair regardless. Resolution
  point: whichever change next opens `internal/catalog`.
- One route to a nesting clash was found **within a single item and a single kind** while
  fixing the cross-item case: `to: ["docs/", "docs"]` for a *file* item with the listing
  `["x.md"]` plans a write to `docs/x.md` and a write to `docs`, and the interpolate-alike
  check correctly does not fire, because for a file item those two entries genuinely are two
  destinations. The cross-item check catches it — a nesting is refused whichever items
  produce it, including one item twice — but the message then names that item as both
  parties, which reads oddly even though it is accurate. A within-item wording for it is
  left for whichever change next has cause to touch this code; the behaviour is correct
  today, only the phrasing is imperfect.
- Per-item `added`/`updated`/`removed` reporting is derivable from `Writes`, `Prune`, and the
  old lock without `plan` growing a report type. Recorded in proposal.md → Non-Goals and
  design.md → Risks; it lands with `sync-command`.
