## Reviewed Artifacts

- `openspec/changes/git-fetch/proposal.md`
- `openspec/changes/git-fetch/specs/rev-resolution/spec.md`
- `openspec/changes/git-fetch/specs/fetch-cache/spec.md`
- `openspec/changes/git-fetch/specs/source-listing/spec.md`
- `openspec/changes/git-fetch/design.md`
- `openspec/changes/git-fetch/tasks.md`

Read as sources of truth, not reviewed: `SPEC.md`, `PRD.md`, `ENGINEERING.md`, `AGENTS.md`,
`openspec/IMPLEMENTATION-ORDER.md`, `openspec/config.yaml`, the six main specs under
`openspec/specs/`, the three archived changes, and the existing
`internal/{itemid,manifest,lock,catalog,plan}` sources.

**Delegation.** The finding pass was delegated to a fresh subagent that did not write these
artifacts, given the review list and the ten checks below, and instructed to report findings
only and edit nothing. Every repair in the table below was made by the authoring session
against that reviewer's report, after independently reproducing each CRITICAL. This is what
`openspec/config.yaml` asks for and what the previous two changes had to record a deviation
from; dispatch was available this time.

## Reviewed Against

- This repository HEAD: `985fe28ab425feef5f47d905934f2fd5d7b0ebc8`
- Sibling repository (`optioni/openspec-schemas`) HEAD: `Not applicable` — this change reads
  no catalog from it, installs nothing, and does not touch the hand-copied
  `openspec/schemas/tdd/` or `.claude/agents/`.
- Working tree: clean apart from `openspec/changes/git-fetch/`, which is this planning
  package and is intentionally included.
- git version used for every empirical check: `2.48.1`. Go: `1.27.0`.

## Gaps Found and Fixed

| Severity | Source Artifact | Problem | Repair | Updated Location |
|---|---|---|---|---|
| CRITICAL | design.md → D2 | **A `git` value could become a git option, which is arbitrary code execution.** D2 claimed an explicit argv was enough and that "a leading `-` would be rejected by git rather than executed by graft". It is not: `git ls-remote --upload-pack=./pwn.sh refs/tags/v1` parses the first word as an **option**, promotes the refspec to the repository operand, and runs the script. Reproduced independently. `manifest.validate` only requires `git` to be non-empty and `CloneURL` passed the value through unchanged, so a `graft.toml` in a pull request plus a committed script would own any machine that ran `graft sync` — in a tool whose central security claim is that it executes nothing from a source. | Two independent guards, because either alone is one behavior change from failing. `CloneURL` gained an error return and refuses a value beginning with `-` with its own message; every invocation taking a URL now separates options from operands with `--`, verified to produce `fatal: strange pathname … blocked` with nothing executed, while an ordinary URL is unaffected. New spec scenario, new RED task asserting the refusal from all three entry points with `PATH` emptied so a green test cannot mean "git ran and declined". | design.md → D2, Contracts (`CloneURL` signature), Error surface; specs/rev-resolution/spec.md → *A source's git value expands to a clone URL* and the error requirement (+1 scenario); tasks.md 2.2, 2.3, 2.4, 3.3, 4.4 |
| CRITICAL | design.md → D10, D11; specs/source-listing/spec.md | **The symlink defence guarded only the last component of `from`.** `os.Lstat` does not follow the final element but does resolve every intermediate one, so a source committing `extras` as a symlink to `../..` and declaring `from: extras/secrets` reads a directory outside the fetched entry entirely — reproduced, with a planted `id_rsa` listed and readable. `catalog.inSource` passes it because nothing is wrong with the *string*, and `plan.insideItem` passes because the listed paths are relative. This breaks SPEC.md's invariant that "every file it reads stays inside that item's own `from`" while every existing check reports success. The same hole reached `catalog.yaml`, which a source may commit as a symlink to an absolute path. | Every read below an entry now goes through `os.Root`, which Go 1.27 provides and which refuses any name whose components leave the root: `root.Lstat(from)`, then `root.OpenRoot(from)` and `fs.WalkDir(fromRoot.FS(), ".")`, so the walk is contained to the item's own subtree as well as to the entry. `ReadCatalog` reads through the same root. Two new spec scenarios and three new RED tasks, each planting a real file outside the entry and asserting its contents appear nowhere. | design.md → D10 (rewritten), D11, D12 (new), Boundaries, Test Boundaries; specs/source-listing/spec.md → both requirements (+2 scenarios); tasks.md 10.2, 10.4, 11.4, 11.8 |
| CRITICAL | design.md → D8; specs/fetch-cache/spec.md | **A cache entry did not hold the bytes the commit recorded, and a source could cause a program to run.** `git checkout` honours the source's own committed `.gitattributes`: `* text eol=crlf` turned the blob `hello\nworld\n` into `hello\r\nworld\r\n` and `ident` expanded `$Id$` into a real hash — both reproduced. The serious case is `filter=lfs`, which selects a filter driver whose *command* comes from the **consumer's** git configuration, so a source-controlled file causes a program to run on any machine with git-lfs installed. `-c core.autocrlf=false -c core.eol=lf` does not close it, because the in-tree attributes rather than the config select the filter. | The checkout now runs with `-c attr.tree=4b825dc642cb6eb9a060e54bf8d69288fbee4904` — the empty tree — which disables in-tree attributes wholesale; verified to restore both the LF bytes and the literal `$Id$`. New spec requirement clause and scenario asserting the entry against `git cat-file blob` rather than against a literal a test author guessed, with its own RED task. | design.md → D8; specs/fetch-cache/spec.md → *A fetch populates the cache with the tree at that sha* (+1 scenario); proposal.md → Non-Goals; tasks.md 8.2, 8.5 |
| WARNING | design.md → D8, tasks.md | **Group 8 could not have gone green as written.** The published fetch sequence never created the work tree, and `git --git-dir=… --work-tree=tree checkout` fails with `fatal: this operation must be run in a work tree` when it does not exist — reproduced, exit 128. | Added `os.Mkdir(tmp/tree)` as an explicit step in the D8 code block and in the task, with the failure it prevents named so it cannot be dropped as noise. | design.md → D8; tasks.md 8.4 |
| WARNING | design.md → Test Boundaries | The table said `internal/manifest` and `internal/lock` are "**not used.** Neither is imported", which group 12 cannot satisfy: `plan.Input.Source` *is* a `manifest.Source`, and task 12.3 calls `lock.Marshal` for the byte-equality assertion. This was the one place a task relied on a collaborator the design named as absent — exactly the check `openspec/config.yaml` asks the planning review to make. | Rewrote the row as "**real, as values only**", naming both uses and stating that neither file is read from or written to disk in any test. | design.md → Test Boundaries |
| WARNING | design.md → Error surface | The `cannot fetch` example quoted git's **second** stderr line (`fatal: remote error: upload-pack: not our ref …`); the first is `fatal: git upload-pack: not our ref …`. Worse than a wrong example: over the local transport two processes write the same pipe, so *which* line arrives first is not deterministic, and a test pinning git's text would flake in CI. | Marked both git-derived rows as prefix + git's first line, made the examples explicitly illustrative, and added the flake reason to the design and to the task, so the tests assert the graft-owned prefix and the absence of a newline and nothing about git's wording. | design.md → Error surface, D13; tasks.md 9.2 |
| WARNING | design.md → Error surface; specs/fetch-cache/spec.md | Two messages were declared "asserted by a test and a deliberate contract" but appeared in no requirement and no task — the unusable-cache-root error and the undeterminable-home error — and the cache-root requirement had no failure scenario at all, which `openspec/config.yaml`'s specs rule requires wherever a failure path exists. | Added the missing scenario for an undeterminable home, pinned the exact prefix in the unusable-root scenario, and stated the requirement's failure clause. Nothing now claims a contract no test holds. | specs/fetch-cache/spec.md → *The cache root defaults to `~/.cache/graft`* (+1 scenario) and *A partial fetch never becomes a cache entry*; tasks.md 7.1, 9.2 |
| SUGGESTION | tasks.md 6.1 | Claimed the sha message is "the wording `internal/lock` and `internal/plan` already use". `internal/plan/build.go:161` matches byte for byte; `internal/lock/lock.go:225` does not — it carries a `graft.lock: ` prefix. A task that misdescribes an asserted contract is how the wrong assertion gets written. | Corrected to "byte for byte the message `internal/plan` already produces, and the tail of `internal/lock`'s". | tasks.md 6.1 |
| SUGGESTION | design.md → D6 | `.git` was trimmed before a trailing slash, so `…/b.git` and `…/b.git/` derived two entries for one repository. The reviewer ran D6 as written against 16 hostile and odd URLs and found no escape, so containment itself was sound; this is a duplication bug rather than a safety one. | Trim the trailing slash first, stated in both the spec and D6, with the combined case added to the ssh/HTTPS scenario and to the RED task. | design.md → D6; specs/fetch-cache/spec.md → *The same repository over ssh and over HTTPS is one entry*; tasks.md 6.1, 6.5 |
| SUGGESTION | specs/source-listing/spec.md | A `from` naming a submodule lists zero files and installs nothing, reported as success. Verified that checkout leaves the gitlink as an empty directory and that nothing is cloned even with the consumer's `submodule.recurse = true` — so submodules are **not** an execution vector, which is worth pinning rather than rediscovering. | Added a scenario recording the empty-directory outcome and the absence of a second remote contact, with its own RED task. | specs/source-listing/spec.md (+1 scenario); tasks.md 11.5 |
| SUGGESTION | specs/fetch-cache/spec.md | The hostile-remote scenario read "characters like `/`-adjacent separators after parsing", which is garbled and does not say what is tested. | Rewritten to name the three input classes concretely, and to require containment be established with `filepath.Rel` rather than by inspecting the string for substrings. | specs/fetch-cache/spec.md → *A hostile remote cannot escape the cache root* |

The three CRITICALs share one root cause, recorded here because it is the sentence the next
reader needs: **a fetched tree is untrusted input, and both `git checkout` and ordinary `os`
path calls honour metadata the source controls.** graft's security claim — "executes
nothing from a source" — is not self-enforcing. It held for three changes because none of
them touched a source; this is the first that does, and it had three ways to break it. That
is also why tasks.md group 14 now names the three closed holes to its reviewer explicitly
and asks for a fourth: a reviewer given only the general shape re-derives the same three.

## Checks That Found Nothing

Named because these are where this repository fails, and because most were run
mechanically rather than by eye.

- **All 48 spec scenarios appear in design.md's verification matrix** — 48 rows, extracted
  and diffed by script in both directions, no orphan row and no uncovered scenario — and all
  48 appear in tasks.md. Counted after every repair above, not before. No scenario name is
  duplicated.
- **No group mixes kinds.** All 15 groups carry exactly one marker (11 behavior, 4
  operational, 0 refactor). Every behavior group runs RED before GREEN before REFACTOR;
  every operational group runs CHECK before CHANGE before VERIFY. Group 15's CHECK → VERIFY
  with no CHANGE matches the `tdd` schema's own `Lint & Verify` shape.
- **No `parallel-after` marker is used**, correctly: groups 2–12 build one package and share
  its files, and group 12 depends on every group before it by construction.
- **No task invents a collaborator design.md → Test Boundaries does not name**, once the
  `manifest`/`lock` row above was corrected. The table gives the filesystem three rows
  (working tree, cache root, fetched entry) and `git`, the network, fixture repositories, the
  environment, and the clock each get one.
- **The change crosses no PRD non-goal.** No dependency resolution, no registry, no merge
  behavior, no auth layer — D3 declines credential handling explicitly and rejects `ssh -o
  BatchMode=yes` with a stated reason — and nothing here makes a synced file require graft at
  runtime. `graft.lock` stays at `version = 1` and no format key is added.
- **Three further execution routes were attacked and found closed.** A repo-level
  `uploadpack.packObjectsHook` in a local-path source is ignored by git (no execution on
  either `fetch` or `ls-remote`); `ext::` transport is refused by default
  (`fatal: transport 'ext' not allowed`); submodules are not populated by `checkout` even
  with the consumer's `submodule.recurse = true`. The bare-git-dir-beside-work-tree approach
  leaves **no `.git`** in a published entry, confirmed by walking a real fetched entry.
- **Cache-path containment holds.** D6's derivation was run against 16 hostile and odd URLs
  — `..` segments, `..%2f..%2f`, `%2e%2e`, `git@..:../../etc/x`, `https://../a`, an empty
  string, `/` — and `filepath.Rel(root, entry)` came back clean in every case, with no `..`
  in any result.
- **Every git mechanic the design depends on was verified empirically** rather than assumed:
  `ls-remote` with the three explicit refspecs returns the peeled line under the literal ref
  name `refs/tags/v1.0.0^{}`; not-found is exit 0 with no output and unreachable is exit 128,
  so D4's "different signals, not different messages" holds; the tail-match hazard is real
  (`refs/tags/v1` returned `refs/heads/x/refs/tags/v1`), which makes the `==` comparison in
  task 4.4 load-bearing rather than decorative; `fetch --depth 1 --no-tags origin <sha>`
  succeeds against a local path remote for both a branch tip and a mid-history commit, so
  the older-commit fixture in group 8 is sound.
- **Every concentration point `openspec/config.yaml` names is present.** Fixture repos use
  repo-scoped `git config user.name`/`user.email` with an explicit `-c
  init.defaultBranch=main`, and task 1.3 reproduces the clean-runner condition locally with
  `env -u HOME GIT_CONFIG_GLOBAL=/dev/null`. `internal/plan`'s purity is re-checked in task
  12.5 against the real guard test name. Determinism is asserted as byte equality through
  `lock.Marshal`, with `reflect.DeepEqual` explicitly ruled out. Nothing is added to
  `cmd/graft`. The prune-set concentration point does not apply — this change computes no
  prune set and deletes nothing — and that is stated rather than silently skipped.
- **Contract gates are present and aimed correctly.** Task 2.4 → SPEC.md `graft.toml`; 4.6
  and 11.9 → SPEC.md `graft.lock`; 10.5 → SPEC.md `catalog.yaml`; 8.6 is the persistence
  gate. An independent Change Review group (14) precedes final verification, and group 15
  ends with `task lint`, `task cover`, and `task build` as separate commands.
- **No RED task is scheduled for plumbing.** Group 1 is operational and its evidence is a
  fixture harness watched to pass under a stripped environment, not a test asserting a
  constant.
- `openspec validate git-fetch --strict` → valid.

## Change Review — Findings and Dispositions

The finding pass was delegated to a separate reviewer subagent, given the five artifacts,
`git diff 180319d..HEAD`, and the eight concentration points tasks.md group 14 names, and
asked explicitly to look for a **fourth** execution route beyond the three the planning
review had closed. It found one, and it is the most serious finding of the change.

| Severity | Finding | Disposition |
|---|---|---|
| CRITICAL | **The child git process inherited the consumer repository's state.** `cmd.Env = append(os.Environ(), …)` passes through `GIT_DIR`, `GIT_WORK_TREE`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, and `GIT_COMMON_DIR`, which git itself exports whenever it runs something — a `post-merge` hook, `git rebase --exec`, `git bisect run`. With `GIT_INDEX_FILE` inherited, graft's internal checkout **rewrites the consumer's index**. Reproduced by hand before fixing: the index grew from 137 to 198 bytes and `git status` afterwards reported a phantom `AD catalog.yaml`. `GIT_OBJECT_DIRECTORY` deposits the source's objects in the consumer's `.git/objects`. This breaks the change's headline invariant verbatim, and design.md → D3 had decided "nothing else is scrubbed" while reasoning only about credentials. | **Fixed.** `gitEnv` strips nine repo-state variables from every git invocation and nothing else — `GIT_ASKPASS`, `SSH_AUTH_SOCK`, and `credential.helper` still reach git, which is D3's actual promise. `TestFetchIgnoresInheritedGitState` runs one subtest per variable, snapshotting the consumer repository including `.git`, because they fail differently and the quiet ones are the dangerous ones: `GIT_WORK_TREE` fails the fetch loudly while `GIT_INDEX_FILE` corrupts and reports success. Mutation-tested: with the scrub removed, four of the nine subtests go red. D3 rewritten to say it was wrong and why; new spec clause and scenario. |
| WARNING | **`TestReadCatalogSymlinkEscape` passed against an implementation that follows the symlink.** Its outside fixture was invalid YAML, so the parse failed either way — and AGENTS.md had just been given a line claiming each of the three guards "has a test that goes red". For that guard it was false. | **Fixed.** The outside file is now a *valid* catalog, and the test asserts the read is refused and pins the `catalog.yaml: ` prefix. Mutation-tested: replacing `ReadCatalog`'s body with a plain `catalog.Load` now fails with `ReadCatalog parsed a catalog from outside the entry`. This also discharges the reviewer's separate note that the `catalog.yaml: <err>` row was asserted by nothing. |
| WARNING | **The lost-rename-race branch — a spec requirement and a GREEN task — was executed by no test.** Coverage confirmed three uncovered blocks. The behavior was correct; nothing guarded it. | **Fixed.** `TestFetchConcurrentOnOneSHA` runs twelve goroutines against one sha under `-race` and asserts one entry, correct content, and no scaffold left behind. `TestFetchEntryPathSquattedByAFile` covers the other half, where the destination exists but is not a directory. |
| WARNING | **`attr.tree` needs git ≥ 2.40, and git ignores an unknown `-c` key in silence.** On an older git the `.gitattributes` defence would fail open with nothing said — the worst way for a security control to break — and no document declared a minimum git version. | **Fixed.** `requireVersion` runs one `git --version` on a cache miss and refuses below 2.40, unframed like `git not found on PATH`. Unparseable output is accepted deliberately: a git that reports its version unusually is more likely a wrapper than an ancient binary. ENGINEERING.md now declares the runtime floor and why it is not arbitrary. New spec clause and scenario; `TestCheckVersion` tables the parse. |
| WARNING | **The internal checkout ran the consumer's git hooks.** A globally configured `core.hooksPath` applies to every repository, including the one graft creates in its scaffold; a `post-checkout` hook fired during graft's own fetch and can write anywhere. Not source-controlled, so not an "executes nothing from a source" breach — but the same containment claim, made untrue by an ordinary user setup. | **Fixed.** `-c core.hooksPath=` at a path inside the scaffold that does not exist, plus `git init --template=` so the user's init templates cannot install one either. Mutation-tested: without the flag, `TestFetchSuppressesConsumerHooks` reports the hook ran. Recorded as design.md → D14 with the version check. |
| SUGGESTION | **`ReadCatalog` returned a bare `open /abs/path: …` when it could not open the entry at all** — no `source` framing, and not a row in design.md's error table. Unreachable today, since `Fetch` guarantees the entry. | **Fixed** by giving it the same `catalog.yaml: <err>` shape as every other non-absence read failure, rather than leaving a latent inconsistency for the next caller to meet. |
| SUGGESTION | **No context or timeout on any git invocation.** `GIT_TERMINAL_PROMPT=0` prevents the password-prompt hang, but D3 deliberately keeps ssh's host-key prompt, and a stalled transport blocks forever inside a function call. | **Deferred, recorded below.** A timeout is a policy — how long is too long — and the value belongs with the layer that has a user to tell, which is `command-surface`. Nothing here would have to change: `exec.CommandContext` is a one-line substitution at the single site that starts a subprocess. |

Checks the reviewer ran that found nothing, named because they are the ones that would have
mattered:

- **A cache hit runs no git command**, proved by two mutations rather than by reading:
  deleting the early return, and inserting a verification before honouring the hit. Both
  turn `TestFetchCacheHitNeedsNoGit` red.
- **Containment of cache writes**, over a 40-entry hostile table *and* `testing/quick` at
  200,000 random strings, asserting `filepath.Rel(root, entry)` carries no `..`. No escape.
  A second 200,000-case property checked that every emitted segment is non-empty, is not
  `.` or `..`, holds no separator, and equals its own `filepath.Clean`.
- **The three planned guards are each load-bearing**, each demonstrated by removing it: the
  `.gitattributes` guard was verified against a *real* smudge filter that ran without it and
  did not run with it, which is stronger evidence than the byte comparison alone.
- **Other execution routes are closed by git's own defaults**: `ext::` and `fd::` transports
  are refused (`fatal: transport 'ext' not allowed`), submodules are not populated, and the
  git directory is created fresh by graft, so no source-supplied repo config, `.lfsconfig`,
  alias, `core.fsmonitor`, or `credential.helper` can reach it.
- **All thirteen error-surface rows matched** the specs and the code character for character.
- `cmd/graft` is untouched; `internal/plan` is unchanged and still passes its purity guard;
  `go test -race ./...` and `golangci-lint run` are clean; no `defer` in a loop, no leaked
  handles, no data race.

## No Remaining Implementation-Blocking Gaps

None remain. Every gap above is repaired in the artifact that owns it, and each of the three
CRITICALs was reproduced independently before its fix was written and after — the fix
verified against the same reproduction, not merely reasoned about.

One judgement was made deliberately rather than left open, and is recorded in design.md →
D10 so a reviewer can overturn it cheaply: a symlink that resolves *inside* an entry is
allowed through `from`'s intermediate components. The reasoning is that the entry is the
source's own tree, which the source already controls in full, and that refusing it would
break a source that merely organises itself with links. The rule that is enforced is that a
read may not **leave** the entry, and the walk below `from` is separately contained to the
item's own subtree. It does not block implementation.

## Deferred Non-Blocking Notes

- **A server rejecting fetch-by-sha.** `uploadpack.allowReachableSHA1InWant` is off by
  default in vanilla git, so a self-hosted remote could refuse `git fetch origin <sha>`.
  Local transport, GitHub, and GitLab all accept it. The failure is loud, not silent, and
  the fallback carries its own costs. Resolution point recorded in design.md → Risks.
- **Cache growth is unbounded.** No eviction and no size cap; `rm -rf ~/.cache/graft` is a
  complete solution today. Resolution point recorded in design.md → Risks: whichever change
  first has a user with a real complaint.
- **A cache hit's tree is trusted, not verified.** Verifying it means either a manifest of
  the entry's files — state the atomic publish exists to avoid needing — or re-running git on
  every hit, which forfeits the offline guarantee. Resolution point recorded in design.md →
  Q2: `sync-command`, if its integration tests find a way to produce a half-entry that is not
  a hand edit.
- **A cache entry records no origin URL**, so two remotes sanitizing to one path would share
  an entry. They would also have to share a commit sha to be confused, and a sidecar file
  stops the entry from being purely a tree. Resolution point recorded in design.md → Q1.
- **`rev` does not accept an abbreviated sha**, and the error a user sees describes the
  outcome without explaining the rule. Improving that wording belongs to `command-surface`,
  which owns the error format. Recorded in design.md → Q3.
- **No timeout on a git invocation.** A stalled transport blocks inside a function call.
  The value is a policy that belongs with the layer that has a user to tell; `command-surface`
  owns it, and the change is a one-line `exec.CommandContext` at the single site that starts
  a subprocess.
- **The latest-semver-tag default for an omitted `rev`** is `add-command`'s, together with
  the tag enumeration it needs. Recorded in proposal.md → Non-Goals.
- **The `catalog.Parse` raw-string duplicate-destination guard**, inherited from
  `destination-and-plan`'s review, is untouched here. This change does not open
  `internal/catalog`.
