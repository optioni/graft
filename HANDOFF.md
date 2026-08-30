# Handoff — graft implementation

Updated 2026-08-31, after `semver-ranges` was finished and archived.

## Where things stand

**10 of 13 changes archived.** `add-command` is next and has not been started.

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
| 9 | `list-command` | archived — `graft list` and `graft list --json` build and run |
| 10 | `semver-ranges` | archived — not in IMPLEMENTATION-ORDER.md; inserted ahead of `add-command` because a range changes the `graft.toml` contract and the sync/update split, neither of which is `add`'s business |
| 11 | `add-command` | **next, not started** |
| 12 | `add-picker` | not started |
| 13 | `self-hosting` | not started |

## How to run the remaining changes

One agent per change, `apply-orchestrator`, strictly sequential. **Nothing else may write
to the tree while an agent holds it** — two writers is what caused commits `29b866a`,
`9e00a34` and part of `54228be` to be reverted by `69783de` on 08-24.

Every agent prompt must carry
`export PATH="$HOME/.nvm/versions/node/v24.18.0/bin:/opt/homebrew/bin:$PATH"` — nvm's node
**must** come first, because Homebrew's `node` is currently broken (it cannot load
`libllhttp.9.3.dylib`) and any `openspec` call resolving to it dies with a dyld error.
`openspec` is a global npm package at `~/.nvm/versions/node/v24.18.0/bin/openspec` and is not
on Homebrew's path at all. A `brew reinstall node` would fix the underlying breakage; you are the only writer, stop rather than revert; never push;
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
- **Homebrew's `node` is broken** — missing `libllhttp.9.3.dylib`. Worked around by putting
  nvm's node first on PATH; `brew reinstall node` is the real fix.
