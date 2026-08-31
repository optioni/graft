# AGENTS.md

## Project Overview

`graft` vendors files from a source git repository into the repository it runs in —
agent definitions, OpenSpec schemas, skills — pinned by a lockfile and committed.

**The tool is not built yet.** This repo currently holds design documents and scaffolding.
Read [SPEC.md](SPEC.md) before writing any code; it is the contract, not a sketch.

- [PRD.md](PRD.md) — problem, goals, non-goals, why existing tools do not fit
- [SPEC.md](SPEC.md) — file formats, commands, resolution, invariants, failure modes
- [ENGINEERING.md](ENGINEERING.md) — toolchain, layout, testing, CI, releases

## Setup

Go 1.27, plus tooling:

```sh
brew install go go-task golangci-lint gofumpt goreleaser
```

## Commands

`Taskfile.yml` is the single definition of every command; CI runs `task ci` and nothing
else. Never spell a step out twice.

```sh
task fmt      # gofumpt -w .
task lint     # golangci-lint + gofumpt -l
task test     # go test -race ./...
task cover    # coverage, fails under 80% over ./internal/...
task build    # local binary with version ldflags
task ci       # lint + cover + build
```

## Architecture

```
cmd/graft/          flag wiring and exit codes, nothing else
internal/manifest/  graft.toml
internal/lock/      graft.lock
internal/catalog/   catalog.yaml, selector expansion
internal/source/    git resolution, content-addressed fetch cache
internal/plan/      pure: manifest + lock + catalog -> file operations
internal/apply/     the only package that writes to the working tree
internal/sync/      the resolution sequence: fetch, plan, apply, and the report
internal/list/      graft.lock -> the listing `graft list` prints, in both forms
internal/cli/       the cobra command surface; internal/ui owns both streams
```

`plan` is pure and `apply` is the sole writer. This is not stylistic — it is what makes
"nothing touches the tree until every check passes" a testable invariant. A write outside
`apply`, or a filesystem read inside `plan`, breaks the design rather than bending it.

## Rules that are easy to get wrong

- **Never delete a file absent from `graft.lock`.** `.claude/agents/` holds repo-owned
  agents beside synced ones. The lock's per-item file list exists solely to make removal
  safe; any change near the prune set needs a test proving a foreign file survives.
- **An `os.Root` is half the containment, not all of it.** It refuses a path that leaves its
  root and **follows** a symlink that stays inside it. So every ancestor of a path graft
  writes or deletes is checked with `Lstat` and must be a directory — a symlink to a directory
  is not one. Without that, a write through a symlinked parent lands where `graft.lock` does
  not say, and a lock claiming `vendor/x.md` where `vendor` became a link to `docs` deletes
  `docs/x.md`. The same trap sits in the empty-directory walk: unlinking a symlink succeeds
  however full its target is, so "a non-empty directory fails harmlessly" is false for links.
- **No path a *plan* names is ever inside `.git`, or is `graft.toml` or `graft.lock`.** Neither
  escapes the repo root, so no planning rule catches them. A file in `.git/` is invisible to the
  `git diff` that is graft's entire review story, and `.git/config` turns placing a file into
  running a program. The rule is on the first path segment — `.github/` and `.gitignore` are
  ordinary destinations. It is about provenance, not a path string: graft writes its own two
  files from bytes it built itself — `graft.lock` on every run, `graft.toml` under
  `graft update --to` — and writes inside `.git` on no path at all.
- **Both artifacts graft emits are byte-deterministic** — `graft.lock` and the
  `graft list --json` document alike, ordered sources by name, items by id, files by path.
  Assert byte equality across two runs *and* across two inputs holding the same content in a
  different order, never semantic equality: a lock that churns makes every sync a diff nobody
  reads, and a document that churns breaks the consumer the JSON form exists for. The
  document's field names, its field order, its two-space indentation, its single trailing
  newline, and its rule that an empty collection renders as `[]` and never as `null` at every
  level are contract as much as its ordering is — changing one is a breaking change to argue
  for, not an incidental edit, and the two ways it silently breaks a consumer are both
  invisible to a test that only decodes. The document's own `version` is **2**: it moved from
  `1` when every source object gained `matched`, sitting between `rev` and `resolved` and
  present unconditionally — empty for a ref, so a consumer branches on the field's value, never
  its presence. `graft.lock`'s own `version` stayed `1` for the same addition; the two version
  numbers name different formats and are free to move independently.
- **`sync` never re-resolves a pin — a semver range included.** It installs what the lock
  says, however far a branch has since moved or however many newer tags satisfy a range.
  Only `update` moves a pin, re-resolving a range against the source's own tags and
  recording the tag it picked as `matched` in `graft.lock`. Re-resolving on sync would make
  `rev = "main"` or `rev = "^1.2.0"` drift silently, which defeats the point of pinning — the
  one exception is a source `sync` finds no lock entry for at all, which it resolves once
  because a first resolution is not a re-evaluation.
- **`graft.toml` is a human's file.** graft moves a pin, appends a source table, and amends an
  `install` array — each by editing text in place, never by re-serializing a parsed manifest,
  which deletes every comment and turns a one-line change into a whole-file diff nobody reads.
  A shape the editor cannot rewrite exactly is refused rather than guessed at: a wrong guess
  corrupts the consumer's own request, and a wrong *target* looks like success. Every edit needs
  a test bounding what it touched — one changed line for a pin move, the original bytes as a
  *prefix* for an append, and for an amendment the inserted lines plus at most the one comma an
  element that had none must gain. The amendment anchors on the array's last element rather than
  its closing bracket, so a comment between them survives with no rule of its own. `add` is the
  only command that may create the file; every other one fails on its absence, which is the
  error that tells a user they are not in a graft repository.
- **Error strings are asserted by tests.** For a CLI this small the failure-mode table is
  the product. Changing a message is a deliberate contract change.
- **graft executes nothing from a source, but it does place.** It reads `catalog.yaml`
  and copies files — no build step, no hook, no script a source can cause to run. The
  claim stops there: kinds are arbitrary, so a catalog can name a destination that
  something *else* executes (`.github/workflows/` runs in CI, `.claude/agents/` is
  instructions an agent follows). That is why `add` shows every item's destination before
  writing and why a consumer override beats the catalog. Any feature that would run
  source-provided content changes the threat model and needs arguing for explicitly.
- **"Executes nothing" is not self-enforcing, and `internal/source` holds three guards
  that look like details.** A `git` value beginning with `-` becomes a git *option*, and
  `--upload-pack=` runs a program — hence the refusal in `CloneURL` and the `--` before
  every URL. A committed `.gitattributes` rewrites checked-out bytes and can select a
  `filter=` driver whose command comes from the *consumer's* config — hence
  `attr.tree=<empty tree>` on the checkout. And every read below a cache entry goes
  through `os.Root`, because `os.Lstat` resolves every component of a path but the last,
  so a symlinked parent reads straight out of the entry. Removing any of the three
  reopens execution or an out-of-tree read; each has a test that goes red.
- **A cache entry is never written in place.** It is keyed by an immutable commit sha, so
  nothing ever re-fetches one that exists — which means a partial entry is wrong forever.
  Fetch into a sibling temporary directory and publish by rename, and treat a lost race
  on that rename as a hit rather than an error.
- **A test outside `./internal/...` never runs.** `task ci` is lint, cover, and build, and
  `cover` is the only one that runs `go test` — over `./internal/...` So logic in
  `cmd/graft` is not merely invisible to the coverage gate, it is unexecuted by CI. That is
  why `cli.Main` returns an exit code and `main` is one call.
- **Every byte a user sees is written by `internal/ui`**, which owns both streams and the
  one colour decision. Anything that renders its own output — cobra's help — is handed
  `ui.Out()`/`ui.Err()` rather than the real streams, because `cobra.Command.Help()`
  discards its renderer's write error and would exit 0 having printed nothing.
- **Fixture git repos need `user.name` and `user.email` set on the repo**, not the
  machine, or commits fail on a clean CI runner.

## Workflow

This repo uses **OpenSpec with the `tdd` schema**. Non-trivial work starts as a change
proposal (`/opsx:propose`), not ad-hoc planning; bug fixes and trivial changes go direct.
Run `openspec validate --strict` before presenting a proposal as ready. Commit after each
task group; archive a change once implemented and verified.

Conventional Commits. Commits land on `main` directly unless the work needs isolation.
Never push or open a PR without being asked.

## Synced files

`openspec/schemas/tdd/` and `.claude/agents/` are **vendored by graft** from
[openspec-schemas](https://github.com/optioni/openspec-schemas), declared in this repo's own
`graft.toml` and pinned by `graft.lock`. Editing them here is pointless — the next
`graft sync` overwrites them, silently, because a synced file is a derived artifact. Edit
them in that repository, publish a tag, and `graft update` here.

This repo is graft's first consumer, which is what the `dogfood` CI job tests: it syncs and
fails if the tree moves. A red dogfood job means the committed tree and `graft.lock`
disagree, which is the one thing worth failing a build over.
