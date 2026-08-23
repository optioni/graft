# graft

Vendor files from a git repository into the repository you are standing in — pinned,
reproducible, and committed.

> **Status: design.** The specs are written; the tool is not built yet.

Some files have to be byte-identical across many repos: agent definitions, OpenSpec
schemas, shared skills. They cannot be fetched at runtime, because the things that read
them — Claude Code, openspec — run in checkouts where no install step happens. So they
must be committed. And they live in directories that also hold files nobody syncs, so
whatever manages them cannot own the directory, only its own files.

graft does that, and nothing else.

```toml
# graft.toml
[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
rev     = "v1.2.0"
install = ["schema:tdd", "agent:*"]
```

```sh
graft sync      # make the tree match the lock
graft update    # move the pins, then sync
graft list      # what is installed here, and at which SHA
graft search    # what a source offers
```

Commit the synced files and `graft.lock` alongside your own code.

## How it works

A source repo publishes a `catalog.yaml` saying what it offers and where each kind of
thing belongs. A consumer names the items it wants. graft itself knows nothing about
agents or schemas — every convention arrives as data, so a source can restructure itself
without breaking anyone's config.

`graft.lock` records the resolved SHA and, per item, **the files graft wrote**. That list
is what makes removal safe: graft deletes only files it put there, so your own agents can
sit in `.claude/agents/` beside the synced ones and never be touched.

Synced files are derived artifacts. `sync` overwrites them without asking, the way `npm
install` overwrites `node_modules` — edit the source, not the copy.

## Install

```sh
brew install --cask optioni/tap/graft      # macOS
go install github.com/optioni/graft@latest # anywhere
```

Or download a binary from the releases page. Builds are published for macOS and Linux on
`amd64` and `arm64`; the Homebrew tap is a cask, so it is macOS-only.

## Design

- [PRD.md](PRD.md) — the problem, and why existing tools do not fit
- [SPEC.md](SPEC.md) — formats, commands, resolution, invariants, failure modes
- [ENGINEERING.md](ENGINEERING.md) — how this repo is built and shipped

## License

MIT
