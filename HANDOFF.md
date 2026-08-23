# Handoff — unattended implementation run

Started 2026-08-23 ~22:55 local. Session limit resets 03:10.

## What is running

A background workflow (`graft-implementation`, run `wf_0d58089b-bd1`) implements the 11
changes in `openspec/IMPLEMENTATION-ORDER.md` **sequentially, in dependency order**. Two
agents per change:

1. **artifacts** — `opsx:ff` generates proposal, specs, design, tasks, planning-review;
   `openspec validate --strict`; commit.
2. **implement** — works every task group in its declared lifecycle, dispatches a separate
   reviewer for the Change Review group, runs `task ci`, then `opsx:archive`.

Sequential rather than parallel on purpose: parallel changes in one repo means git
conflicts with nobody awake to resolve them.

**Fail-fast.** If either agent reports failure, the run halts there and skips the rest
rather than stacking nine changes on a broken foundation. Every completed change is
committed and archived, so the halt point is a clean boundary.

## Where to look

- `/workflows` — live progress
- Transcript: `~/.claude/projects/-Users-juusopiikkila-Code-openspec-schemas/b1f64033-a64b-40c7-ac7f-e72a5e8c2a5f/subagents/workflows/wf_0d58089b-bd1`
- `journal.jsonl` in that directory records each agent's actual return value
- `git log --oneline` — one or more commits per completed change
- `openspec/changes/archive/` — completed changes
- `openspec/changes/<name>/` (not archive) — a change that was started but not finished

## To resume after a halt

```sh
export PATH="/opt/homebrew/bin:$PATH"
cd ~/Code/graft
git log --oneline -20
ls openspec/changes/           # in-progress change, if any
task ci                        # is the tree green?
```

Then either continue the in-progress change with `/opsx:apply <name>`, or re-run the
workflow from the first unfinished change.

## State when this started

- 9 commits on `main`, **none pushed**. `origin` is `git@github.com:optioni/graft.git`.
- `~/Code/openspec-schemas` is **3 commits ahead of its origin**, also unpushed.
- `task ci` green: lint, 100% coverage over `./internal`, build.
- `goreleaser check` clean.
- Go 1.27.0, task, gofumpt, golangci-lint, goreleaser installed via brew this session.

## Known gaps, deliberate

- `HOMEBREW_TAP_TOKEN` is not set as a repo secret, and `optioni/homebrew-tap` has no
  commits. No release can succeed until both are done. GoReleaser creates `Casks/graft.rb`
  on the first release.
- The dogfood CI job is inert until `self-hosting` (change 11) lands `graft.toml`.
- `openspec/schemas/tdd/` and `.claude/agents/` are hand-copied from `openspec-schemas`.
  Agents were told not to edit them.

## Environment trap

`/opt/homebrew/bin` is **not** on a non-interactive shell's PATH. Every agent was told to
`export PATH="/opt/homebrew/bin:$PATH"` before any go/task command. If something failed
with "command not found: go", that is why.
