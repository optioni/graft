## Context

This repository holds two directories it did not write: `openspec/schemas/tdd/` and
`.claude/agents/`, copied by hand from `openspec-schemas`. graft exists to make that copy a
declaration. Everything the tool needs is built; what is missing is on the other side of the
wire, and the prerequisite in proposal.md is the whole reason this design exists on paper
before it exists on disk.

## Goals / Non-Goals

**Goals:**
- One `graft.toml` source, two selectors, a committed `graft.lock`.
- The synced files byte-identical to the ones committed today, so the first sync is a no-op
  in the working tree and the dogfood job passes on the first run rather than after a fixup.
- The dogfood CI job stops skipping.

**Non-Goals:**
- Any change to graft's behavior — hence `skip_specs`.
- Vendoring skills, workflows, or anything the source may grow later.
- Changing where the files land.

## Boundaries

Nothing in `internal/` changes. This is `graft.toml`, `graft.lock`, one section of AGENTS.md,
and a CI job that already exists and already does the right thing when a `graft.toml` is
there.

## Contracts

**The `catalog.yaml` the source must publish**, matching its existing layout exactly:

```yaml
version: 1

kinds:
  schema:
    to: "openspec/schemas/{name}"
  agent:
    to: ".claude/agents/"
    flatten: true

provides:
  - { kind: schema, name: tdd,                        from: extras/openspec-schemas/tdd }
  - { kind: agent,  name: apply-orchestrator,         from: extras/agents/apply-orchestrator.md }
  - { kind: agent,  name: outside-in-tdd-implementer, from: extras/agents/outside-in-tdd-implementer.md }
  - { kind: agent,  name: outside-in-tdd-reviewer,    from: extras/agents/outside-in-tdd-reviewer.md }
  - { kind: agent,  name: phase-orchestrator,         from: extras/agents/phase-orchestrator.md }
```

The destinations are chosen so that this repository's existing paths come out unchanged:
`schema:tdd` fills `openspec/schemas/tdd/`, and each agent lands flattened in
`.claude/agents/`. A consumer wanting `.codex/agents/` overrides the kind; this one does not
need to.

**The `graft.toml` this repository gains:**

```toml
[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
rev     = "v0.1.0"
install = ["schema:tdd", "agent:*"]
```

`agent:*` rather than four ids, deliberately: this repository wants whatever orchestration
agents that source publishes, which is exactly what the glob means and exactly what an
explicit list would refuse. If no tag is published, `rev = "main"` works and costs
reproducibility between updates.

## Persistence and Rollout

- Migration: none. The files already exist at the destinations; graft takes over writing them.
- Backfill, seeding, index rebuild: none.
- Cache invalidation: none. A first sync populates `~/.cache/graft` on each machine.
- Authorization: the source is public and clones anonymously, which is what lets the dogfood
  job run with no secret.
- Observability: the dogfood job is the observability — `graft sync && git diff --exit-code`
  fails the build if the tree and the lock ever disagree.
- Deployment: none.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| `openspec-schemas` (network) | real, cloned anonymously over HTTPS by CI and by the local verification | not reached — no unit test in this change |
| `git` binary | real | not reached |
| Fetch cache | real, the developer's own `~/.cache/graft` locally and a fresh one in CI | not reached |
| Working tree | real — this repository, which is the point | not reached |
| GitHub Actions | real, on the next push | not reached |

There are no unit tests in this change and no new ones are warranted: it adds no code. The
verification is the tool doing its job against the real source, which is what the dogfood job
was scaffolded for.

## Test Strategy

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| No spec scenarios — `skip_specs` is declared | The change adds no behavior; its evidence is the two checks below | — | — | — |
| The first sync moves nothing | `./graft sync` then `git diff --exit-code`, run locally before committing | acceptance | real source, real cache, this working tree | `task build && ./graft sync && git diff --exit-code` |
| `graft list` reports what the lock records | `./graft list` names the source, the pin, and five items | acceptance | `graft.lock` alone | `./graft list` |
| The dogfood job stops skipping | The job's own output on the next CI run says it synced rather than that it skipped | acceptance | GitHub Actions | the CI run |

The first row is the load-bearing one: if `graft sync` changes a byte of
`openspec/schemas/tdd/` or `.claude/agents/`, the published source and the hand-copied files
disagree, and that has to be resolved in the source before this change lands — not papered
over by committing whatever graft wrote.

## Decisions

**`agent:*` rather than four explicit ids.** This repository wants the orchestration agents
that source publishes, whatever they come to be. That is what a glob means, and adopting a
new agent automatically is the behavior wanted here — the same reasoning the picker's
collapse offer states out loud.

**Pin a tag if one exists, `main` if not.** `sync` never re-resolves either, so both are
reproducible; a tag makes `graft update` a readable version bump rather than an unreviewed
sha. The tag is a request to the source, not a requirement of this change.

**Do not apply this change until the source publishes.** The alternative — commit
`graft.toml` now and let the dogfood job skip or fail — trades a real check for a red build
or a lie. The job goes live the moment `graft.toml` exists, which makes committing it early
strictly worse than waiting.

**`skip_specs`.** No capability's requirements move: `sync` behaves exactly as its spec says,
against a source it has not been pointed at before. Writing a spec delta for "this repository
now has a graft.toml" would be recording a fact about the repository in the document that
describes the tool.

## Risks / Trade-offs

[The published source's bytes differ from the hand-copied files, and the first sync rewrites
them] → Found by running the sync locally before committing anything, which is task 2.1. The
resolution is in the source, not here: the copies in this repository are the newer ones today.

[`agent:*` adopts an agent this repository does not want] → It is a file in `.claude/agents/`,
visible in the sync report and in `git diff`, and removable by narrowing the selector. The
opposite failure — an agent silently not arriving — is the one that goes unnoticed.

[The dogfood job becomes flaky if the source moves] → It cannot: `sync` installs what
`graft.lock` records and never re-resolves. The job only fails if this repository's committed
tree and its own lock disagree, which is exactly the thing worth failing over.

[Vendoring the reviewer agent that reviews this repository's own changes] → Already true
today by hand. graft makes the version explicit rather than implicit, which is an improvement
on the status quo rather than a new exposure.

## Migration Plan

1. In `openspec-schemas`: add `catalog.yaml`, push the outstanding commits, tag.
2. Here: `graft add github.com/optioni/openspec-schemas@<tag> schema:tdd 'agent:*'`.
3. Confirm `git diff` shows only `graft.toml` and `graft.lock`. Anything else means step 1 is
   incomplete.
4. Commit both, update AGENTS.md, push, and watch the dogfood job.

Rollback is deleting the two files: the vendored copies stay exactly where they are, because
graft never removes what a lock does not claim and the lock would be gone.

## Open Questions

**Does the source want a tag, and which version?** `v0.1.0` is the obvious first. This
repository can pin `main` without one, at the cost of readable updates. It is the source
owner's call, and the only question this change cannot answer for itself.
