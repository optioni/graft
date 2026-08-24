# Handoff — graft implementation

Updated 2026-08-24 ~17:55 local, paused on a 5-hour session limit that resets **20:09**.

## Where things stand

**7 of 12 changes archived.** `update-command` is the 8th and is roughly half implemented.

| # | Change | State |
|---|--------|-------|
| 1 | `manifest-and-lock` | archived |
| 2 | `catalog-and-selectors` | archived |
| 3 | `destination-and-plan` | archived |
| 4 | `git-fetch` | archived |
| 5 | `command-surface` | archived |
| 6 | `catalog-hardening` | archived (not in IMPLEMENTATION-ORDER.md — added to close four findings `catalog-and-selectors` deferred) |
| 7 | `sync-command` | archived — `graft sync` builds and runs |
| 8 | `update-command` | **in flight, see below** |
| 9 | `list-command` | not started |
| 10 | `add-command` | not started |
| 11 | `add-picker` | not started |
| 12 | `self-hosting` | not started |

## Resuming `update-command`

Artifacts are complete and committed (`4baf738`). `openspec/changes/update-command/` holds
proposal, design, tasks, planning-review and delta specs.

**`tasks.md` checkboxes are all unticked and are wrong** — the agent implemented without
ticking them. Trust the commit log, not the checkboxes. Actually complete:

- Group 1, manifest: move one rev in place — `84e2eda`
- Group 2, apply: write `graft.toml`, immediately before the lock — `8f9cfcc`
- Group 3, sync: re-resolution as a parameter, not a second sequence — `a2ca57f`

**Group 4 is in progress.** `internal/sync/updateto_test.go` is committed as WIP: it is the
RED test for `--to` moving the pin, and it does not compile yet. Start there.

Remaining after group 4: group 0/7 (the outer acceptance loop — 0 was never written, so
write it before 7 can go green), 5 (`--dry-run` under an update), 6 (the `update` CLI
command), 8 (SPEC.md docs), 9 (Change Review), 10 (Lint & Verify), then archive.

## How to run the remaining changes

One agent per change, `apply-orchestrator`, strictly sequential. **Nothing else may write
to the tree while an agent holds it** — two writers is what caused commits `29b866a`,
`9e00a34` and part of `54228be` to be reverted by `69783de` on 08-24.

Every agent prompt must carry: `export PATH="/opt/homebrew/bin:$PATH"` before `task`/`go`/
`openspec`/`golangci-lint`; you are the only writer, stop rather than revert; never push;
coverage is 80% over `./internal/...`; `internal/plan` stays pure and `internal/apply` is
the sole writer; fixture git repos need repo-level `user.name`/`user.email`; error strings
are asserted contract; retry API server errors rather than abandoning the step.

Do not run `graft sync`/`graft update` against this repo's own `.claude/agents/` or
`openspec/schemas/` until `self-hosting` — test against fixtures in a temp directory.

## Not done, and not forgotten

- **Nothing is pushed.** `origin` (`git@github.com:optioni/graft.git`) is empty. This is
  why a scheduled *cloud* resume cannot work — it would clone nothing.
- `HOMEBREW_TAP_TOKEN` is not set as a repo secret; `~/Code/homebrew-tap` has no commits.
- `openspec-schemas` has 3 unpushed commits.
