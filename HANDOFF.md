# Handoff — graft implementation

Updated 2026-08-24, after `update-command` was finished and archived.

## Where things stand

**8 of 12 changes archived.** `list-command` is next and has not been started.

| # | Change | State |
|---|--------|-------|
| 1 | `manifest-and-lock` | archived |
| 2 | `catalog-and-selectors` | archived |
| 3 | `destination-and-plan` | archived |
| 4 | `git-fetch` | archived |
| 5 | `command-surface` | archived |
| 6 | `catalog-hardening` | archived (not in IMPLEMENTATION-ORDER.md — added to close four findings `catalog-and-selectors` deferred) |
| 7 | `sync-command` | archived — `graft sync` builds and runs |
| 8 | `update-command` | archived — `graft update`, `graft update <source>` and `graft update --to` all build and run |
| 9 | `list-command` | **next, not started** |
| 10 | `add-command` | not started |
| 11 | `add-picker` | not started |
| 12 | `self-hosting` | not started |

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
