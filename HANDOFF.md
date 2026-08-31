# Handoff — graft implementation

Updated 2026-08-31, after `self-hosting` was finished and archived. **The roadmap is done.**

## Where things stand

**13 of 13 changes archived**, plus two the roadmap did not anticipate. The tool is
complete — `sync`, `update`, `list`, and `add` with its picker all build and run — and graft
now vendors its own agents and schema through its own `graft.toml`, pinned at
`openspec-schemas@v0.1.0`.

Nothing is in flight. The only work left is not implementation: see **Not done** below.

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
| 13 | `self-hosting` | archived — `graft.toml` declares `openspec-schemas@v0.1.0`. The dogfood CI job was dropped rather than activated: the source is private and CI has no credential for it |

## If a vendored file ever moves

`openspec/schemas/tdd/` and `.claude/agents/` are graft's now. Editing them here does
nothing — the next sync overwrites them. Edit them in `~/Code/openspec-schemas`, tag, and
`./graft update` here.

If a sync moves one unexpectedly, read the **direction** before committing anything. Bytes
this repository holds that the source does not are an upstream change that never landed, and
committing over them destroys work. Bytes the source holds that this repository does not are
a copy that fell behind. The first sync hit the second case, in two files.

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

- **The dogfood check is manual now.** `./graft sync && git diff --exit-code`, by hand, when
  a pin moves. The CI job that did it was dropped: `openspec-schemas` is private, `GITHUB_TOKEN`
  only reaches the repo it runs in, and a job that cannot read its source is a red build
  everyone learns to ignore. Making that repo public, or giving graft a read token as a secret,
  would let it come back.
- `HOMEBREW_TAP_TOKEN` is not set as a repo secret; `~/Code/homebrew-tap` has no commits.
- `openspec-schemas` now publishes `catalog.yaml` and `v0.1.0`; nothing is outstanding there.
- **Homebrew's `node` is broken** — missing `libllhttp.9.3.dylib`. Worked around by putting
  nvm's node first on PATH; `brew reinstall node` is the real fix.
