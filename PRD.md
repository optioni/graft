# graft — PRD

> Status: draft. Name is provisional.

## Problem

Some files must be byte-identical across many repositories: an OpenSpec schema, the
agents that implement its lifecycle, shared skills and slash commands. They are authored
in one place and must arrive, unchanged, everywhere else.

Two properties make this harder than it sounds:

1. **They cannot be fetched at runtime.** Claude Code reads `.claude/agents/` directly and
   openspec reads `openspec/schemas/` directly. A cloud session at claude.ai/code clones a
   repo and starts working — no install step ever runs. The files must already be in the
   tree, committed.
2. **They share directories with files nobody syncs.** `.claude/agents/` holds a repo's own
   agents beside the shared ones. The sync cannot own the directory, only its own files.

Today this is done with [skillshare](https://skillshare.runkids.cc) in push mode, and the
model fights the problem:

- The list of targets lives in `config.yaml` on one laptop, gitignored because the paths
  are absolute. It does not travel. A new machine re-declares every target by hand.
- `sync` without `--force` skips changed files and still reports success — a silent no-op
  that looks like a green run.
- Nothing records which version a repo holds. Answering "is this repo current?" means
  opening it and reading the files.
- Distribution is push, so the source must know every consumer. A repo cannot ask for what
  it wants.

## Goal

A consumer repo declares what it needs, in a committed file, and gets exactly that —
reproducibly, on any machine, in CI, with no global state.

## Users

- **A developer** adopting the shared assets in a repo, or rolling one forward.
- **Coding agents** working in a repo that must find the assets already present.
- **CI**, which must fail when a repo has drifted from what it declared.

## Non-goals

- **Not a package manager.** No transitive dependencies, no version solving, no semver
  ranges. A source is pinned to a tag, branch, or SHA.
- **Not a registry.** Git is the registry. No hosting, no publishing step, no accounts.
- **Not a merge tool.** Synced files are derived artifacts, overwritten without ceremony —
  the `node_modules` contract. Editing one in place is a user error, not a merge conflict.
- **Not an auth layer.** Whatever `git` can already clone, graft can already fetch.
- **Not a runtime dependency.** Nothing graft installs may require graft to be present to
  work.

## Success criteria

1. Adopting the shared assets in a fresh repo is one file and one command.
2. `graft sync` on any machine, from a clean checkout, produces a byte-identical tree.
3. A stale or locally-edited repo fails CI with no bespoke tooling — `graft sync && git
   diff --exit-code`.
4. Which version a repo holds is answerable by reading one committed file.
5. Adding a new kind of asset — a skill, a slash command, a hook — requires no release of
   graft.
6. A source repo can restructure itself without breaking any consumer's config.

## Journeys

**Adopt.** Add a `[sources.*]` block to `graft.toml` naming a git repo, a rev, and what to
install. Run `graft sync`. Commit the synced files and `graft.lock`.

**Publish a change.** Edit the source repo, tag it. Consumers move when they choose.

**Roll a repo forward.** `graft update` re-resolves the pins and rewrites the lock. The
git diff shows exactly which files changed, and which were removed because the source
stopped providing them.

**Enforce.** CI runs `graft sync --frozen && git diff --exit-code`. A repo that is behind,
or whose synced files were edited in place, fails.

**Drop something.** Remove it from `install` and sync. Its files are deleted, because the
lockfile recorded which ones were ours. Nothing else in the directory is touched.

## Why not an existing tool

- **skillshare** — push-based with machine-local config. The problems above are structural,
  not bugs. Its `extras` mechanism resolves sources from local paths only, so a consumer
  cannot pull.
- **vendir** — the closest fit, and worth reaching for if graft ever grows past a weekend.
  It vendors git sources into paths with a lock file. It loses on ergonomics: every
  consumer spells out full path mappings, so a source that restructures breaks all of
  them, and there is no notion of a source publishing what it offers.
- **git submodule / subtree** — puts a whole repo in the tree, not selected files, and
  cannot place them at conventional paths like `.claude/agents/`.
- **degit** — one-shot copy, no lock, no pruning, no selection.
