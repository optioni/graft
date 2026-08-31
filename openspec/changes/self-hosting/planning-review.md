## Reviewed Artifacts

- `proposal.md`
- `design.md`
- `tasks.md` (7 groups, 26 tasks)
- No spec deltas: the change declares `skip_specs` in `.openspec.yaml`, and `openspec status`
  reports the `specs` artifact as skipped rather than missing.

Not delegated, on the same standing instruction as the two changes before it.

## Reviewed Against

- `AGENTS.md`'s **Synced files** section, which promises exactly this conversion.
- `ENGINEERING.md`'s dogfood paragraph and `.github/workflows/ci.yml`'s dogfood job, read
  line by line: it skips when `graft.toml` is absent and otherwise runs
  `./graft sync && git diff --exit-code`.
- `openspec/IMPLEMENTATION-ORDER.md`, whose final row this change closes.
- The live state of `github.com/optioni/openspec-schemas`, checked rather than assumed.

## Gaps Found and Fixed

### 1. The change was blocked and the plan did not say so — CRITICAL

The first sketch treated this as ordinary work in this repository. It is not: `graft sync`
reads `catalog.yaml` **from the source**, and the source publishes none. A `graft.toml`
committed today would make the dogfood job — which goes live the moment that file exists —
fail on every run.

**Fixed**: the prerequisite is a section of proposal.md and group 0 of tasks.md, stated as
what it is: three pushes to a repository this change cannot reach.

### 2. Two of the source's files are behind their published versions — CRITICAL, found by looking

`git fetch` then `git rev-list --count origin/main..HEAD` in the source repository reports
**four** unpushed commits, and `git diff --stat origin/main HEAD -- extras` shows two of the
differing files are ones graft would install: `extras/agents/apply-orchestrator.md` (+23
lines) and `extras/openspec-schemas/tdd/schema.yaml` (+27). Syncing from the published `main`
today would overwrite this repository's copies with older bytes, and `git diff --exit-code`
would fail — correctly, having found a real disagreement rather than a graft bug.

**Fixed**: it is prerequisite 2, and task 1.3 makes the check explicit — any path other than
`graft.toml` and `graft.lock` in the diff means the prerequisite is incomplete, and the
resolution is in the source rather than in a commit of whatever graft wrote.

### 3. The source publishes no tags — WARNING

`git ls-remote --tags` returns nothing, so the pin would be `rev = "main"`. graft handles
that — `sync` never re-resolves, so it does not drift — but every `graft update` then moves to
an unreviewed sha rather than a version.

**Fixed**: recorded as prerequisite 3 and as the design's one open question, with the
consequence stated rather than the tag demanded. It is the source owner's call.

### 4. `skip_specs` had to be declared rather than assumed — WARNING

The change adds no behavior, so writing a spec delta would record a fact about this
repository in the document that describes the tool. `openspec` supports that only when
`.openspec.yaml` says so; without the declaration, `openspec status` reports `specs` as a
missing artifact and the change never reaches `planning-review`.

**Fixed**: `skip_specs: true` is in the change's `.openspec.yaml`, verified by
`openspec status` reporting `specs` as `skipped`.

## No Remaining Implementation-Blocking Gaps — in this repository

Verified rather than assumed:

- **The source is public and clones anonymously**: `git ls-remote https://github.com/optioni/openspec-schemas`
  answers with no credentials, which is what lets the dogfood job run with no secret.
- **The layout matches SPEC.md's own example**: `extras/agents/*.md` and
  `extras/openspec-schemas/tdd/`, so the `catalog.yaml` in design.md needs no rearranging of
  the source and no consumer override here.
- **The destinations reproduce this repository's existing paths exactly** — `schema:tdd` fills
  `openspec/schemas/tdd/` and the agents land flattened in `.claude/agents/` — which is what
  makes "the first sync moves nothing" a testable claim rather than a hope.
- **The dogfood job needs one edit, not a rewrite**: it already runs the right two commands,
  and group 4 removes only the skip that will have become unreachable.

## Deferred Non-Blocking Notes

- **`agent:*` adopts agents the source adds later.** Deliberate, and the reason the glob was
  chosen over four ids. A new file arriving in `.claude/agents/` is visible in the sync report
  and in `git diff`.
- **This repository has no published remote.** `git ls-remote https://github.com/optioni/graft`
  returns nothing. Nothing in this change depends on that, but the dogfood job only runs once
  the repository is somewhere CI can see it.
