## Why

`openspec/schemas/tdd/` and `.claude/agents/` in this repository are hand-copied from
[openspec-schemas](https://github.com/optioni/openspec-schemas). AGENTS.md says so, and says
they convert to a `graft.toml` entry "as soon as `sync` works". `sync`, `update`, `list`, and
`add` all work now. This is the change that makes graft its own first consumer — and turns on
the dogfood CI job, which has been inert since the scaffold went in.

## What Changes

- `graft.toml` declares one source, `openspec-schemas`, installing `schema:tdd` and
  `agent:*`.
- `graft.lock` is committed, recording the sha and the files each item placed.
- The hand-copied files stay exactly where they are — the same paths, the same bytes. graft
  writes them from now on rather than a person.
- AGENTS.md's **Synced files** section stops describing a manual copy.
- The dogfood job in `.github/workflows/ci.yml` stops being inert: it already runs
  `./graft sync && git diff --exit-code` the moment a `graft.toml` exists, so this change is
  what makes it a real test rather than a skip.

## Prerequisite, and it is not in this repository

`graft sync` reads `catalog.yaml` from the **source**, and `openspec-schemas` publishes none.
Three things must land there first, and each is a push to a repository this change cannot
reach:

1. **`catalog.yaml` at its root.** Its content is settled — the layout already matches
   SPEC.md's own example — and it is reproduced in design.md.
2. **The four unpushed commits on its `main`.** Its remote is four behind local, and two of
   the differing files are ones graft would install: syncing from the published `main` today
   would write older bytes than the copies in this repository hold, and the dogfood job's
   `git diff --exit-code` would fail — correctly, having found a real disagreement.
3. **A tag, ideally.** It publishes none, so the pin would be `rev = "main"`, which graft
   handles but which makes every `graft update` a fresh unreviewed sha. `v0.1.0` on the
   published head would let this repository pin a version.

Until they land, this change is planned and not applied. Nothing here is blocked on graft.

## Non-Goals

- **Vendoring anything else.** One source, two selectors. Skills, workflows, and whatever
  else that repository grows are separate decisions.
- **Publishing graft itself.** `optioni/graft` has no remote content yet; this change does
  not push it and does not depend on it.
- **Changing the destinations.** The files land exactly where they live now. A consumer
  override is available if that ever stops being true, and is not needed today.
- **Removing skillshare from the other repository.** How the source is maintained is its own
  business; graft only reads what it publishes.
- **Any change to graft's behavior.** This is configuration and CI, which is why it declares
  `skip_specs`: no requirement of any capability moves.

## Capabilities

### New Capabilities
None. This change adds no behavior to the tool.

### Modified Capabilities
None. `skip_specs` is declared in `.openspec.yaml`: the change is configuration, the CI job
it activates, and one documentation correction.

## Impact

- New: `graft.toml`, `graft.lock` at the repository root.
- Changed: AGENTS.md's **Synced files** section; `openspec/IMPLEMENTATION-ORDER.md`'s final
  row; `.github/workflows/ci.yml` only if its skip needs tightening once the job runs for
  real.
- Unchanged: every file under `openspec/schemas/tdd/` and `.claude/agents/` — they must be
  byte-identical after the first sync, which is the whole verification.
- **External**: `optioni/openspec-schemas` needs `catalog.yaml`, its unpushed commits, and a
  tag. That is the prerequisite above and the reason this change is not applied.
