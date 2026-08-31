## 0. The prerequisite, in the source repository
<!-- kind: operational -->

Not this repository's work, and not graft's. It is listed because nothing below can start
until it is done, and because a task list that hid the blocker would make this change look
stalled rather than waiting.

- [x] 0.1 CHECK: Confirm `github.com/optioni/openspec-schemas` publishes `catalog.yaml` on
  the ref this repository will pin — `git ls-remote` for the ref, then read the file
- [x] 0.2 CHANGE: If it does not, add the `catalog.yaml` design.md reproduces, push the
  outstanding commits on `main`, and tag — all three in that repository, by its owner
- [x] 0.3 VERIFY: `git ls-remote --tags` names the tag, and a clone of it holds
  `catalog.yaml`, `extras/openspec-schemas/tdd/`, and `extras/agents/`

## 1. Declare the source
<!-- kind: operational -->

- [x] 1.1 CHECK: Record the bytes that are there now — `git status` clean, and a checksum of
  every file under `openspec/schemas/tdd/` and `.claude/agents/` — so the next step's claim
  is measurable rather than eyeballed
- [x] 1.2 CHANGE: `task build`, then
  `./graft add github.com/optioni/openspec-schemas@<tag> schema:tdd 'agent:*'`
- [x] 1.3 VERIFY: `git diff --stat` names `graft.toml` and `graft.lock`, and any other path is
  read for its **direction** before anything is committed — bytes this repository holds that
  the source does not are an upstream change that never landed and must be resolved there;
  bytes the source holds that this repository does not are a hand copy that fell behind.
  *Outcome: two files, both additions from the source — `.claude/agents/apply-orchestrator.md`
  (+23) and `openspec/schemas/tdd/schema.yaml` (+27), the worktree-isolation guidance. The
  copies here were stale, not ahead. Taken.*

## 2. Confirm the sync is a no-op
<!-- kind: operational -->

- [x] 2.1 CHECK: `./graft sync` a second time — it prints `up to date` and writes nothing
- [x] 2.2 VERIFY: `git diff --exit-code` passes, which is precisely what the dogfood job runs
- [x] 2.3 VERIFY: `./graft list` names the source, its pin, its sha, and five items —
  `schema:tdd` plus four agents — and `./graft list --json` parses
- [x] 2.4 VERIFY: `graft.lock` names every file under both destinations, so a later removal
  from the source prunes them rather than leaving orphans

## 3. Documentation
<!-- kind: operational -->

- [ ] 3.1 CHECK: Re-read AGENTS.md's **Synced files** section, which says the two directories
  are copied by hand and will convert to a `graft.toml` entry "once `sync` works"
- [ ] 3.2 CHANGE: Rewrite it in place to say graft owns them, that editing them here is
  pointless because the next sync overwrites, and where to edit instead — deleting the
  now-false sentence rather than appending beside it
- [ ] 3.3 CHANGE: Mark `self-hosting` done in `openspec/IMPLEMENTATION-ORDER.md`, and say
  there that the roadmap is complete
- [ ] 3.4 VERIFY: No other document still describes the copy as manual

## 4. The dogfood job
<!-- kind: operational -->

- [ ] 4.1 CHECK: Re-read `.github/workflows/ci.yml`'s dogfood step and confirm its skip is
  now unreachable — it skips only when `graft.toml` is absent
- [ ] 4.2 CHANGE: Remove the skip branch and its "inert until" comment, leaving
  `./graft sync` and `git diff --exit-code`; a job that can silently skip is a job that will
  silently skip
- [ ] 4.3 VERIFY: The next CI run's dogfood job syncs and passes, and its log shows the sync
  rather than the skip

## 5. Change Review
<!-- kind: operational -->

- [ ] 5.1 CHECK: Review the applied change against proposal.md, design.md, and this list —
  in particular that no file under either destination differs from what was committed before
- [ ] 5.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING, note
  SUGGESTIONs
- [ ] 5.3 VERIFY: Confirm no blocking or unowned finding remains

## 6. Lint & Verify
<!-- kind: operational -->

- [ ] 6.1 CHECK: Inspect the intended verification commands and the tiers they cover
- [ ] 6.2 VERIFY: Run `task lint` — 0 errors
- [ ] 6.3 VERIFY: Run `task test` — green
- [ ] 6.4 VERIFY: Run `task cover` — the 80% floor over `./internal/...` holds
- [ ] 6.5 VERIFY: Run `task build` — the binary builds
- [ ] 6.6 VERIFY: Run `openspec validate self-hosting --strict`
