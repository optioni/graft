# Handoff — graft implementation

Updated 2026-08-31, after `add-command` and `add-picker` were finished and archived.

## Where things stand

**12 of 13 changes archived.** The tool is feature-complete: `sync`, `update`, `list`, and
`add` — with its picker — all build and run. The one change left, `self-hosting`, is
proposed and **blocked on another repository**; see below.

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
| 11 | `add-command` | archived — `graft add`, `--list`, and `--no-sync` build and run; the manifest is amended by surgical text edit |
| 12 | `add-picker` | archived — the interactive multi-select and the `kind:*` collapse offer; verified through a real pty as well as by unit tests |
| 13 | `self-hosting` | **proposed, blocked.** Not on graft — on `openspec-schemas`, which must publish a `catalog.yaml`, push its four outstanding commits, and tag. See `openspec/changes/self-hosting/` |

## What unblocks the last change

Three things, all in `~/Code/openspec-schemas`, all of them pushes:

1. Add `catalog.yaml` at its root. The exact content is in
   `openspec/changes/self-hosting/design.md` — it matches the layout that repository already
   has, so nothing there needs rearranging.
2. Push the four commits its `main` is behind by. Two of the differing files are ones graft
   would install, so syncing from the published `main` today would overwrite this
   repository's newer copies and the dogfood CI job would fail — correctly.
3. Tag it, ideally `v0.1.0`. Without a tag the pin is `rev = "main"`, which works but makes
   every `graft update` an unreviewed sha rather than a version.

Then, here: `./graft add github.com/optioni/openspec-schemas@v0.1.0 schema:tdd 'agent:*'`,
confirm `git diff --stat` names only `graft.toml` and `graft.lock`, and follow
`openspec/changes/self-hosting/tasks.md` from group 1.

## How to run a change

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
- `openspec-schemas` has **4 unpushed commits and no `catalog.yaml`**, which is exactly
  what blocks `self-hosting`.
- **Homebrew's `node` is broken** — missing `libllhttp.9.3.dylib`. Worked around by putting
  nvm's node first on PATH; `brew reinstall node` is the real fix.
