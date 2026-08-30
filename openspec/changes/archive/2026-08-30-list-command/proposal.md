## Why

`graft.lock` is the record of what graft installed, and today the only way to read it is to
open it. That is fine for a human reviewing a diff and useless for everything else: a script
that wants to know which files graft owns, a CI step checking which sha a repository sits on,
an agent asked "where did this file come from". SPEC.md's command table has named
`graft list` — *items installed here, with source and resolved SHA* — since the table was
written, and it is the only v1 command still missing.

The reason it is worth its own change rather than a `cat` alias is the second half:
`--json`. graft has no machine-readable surface at all beyond `--version`. `graft.lock`'s
format is published in SPEC.md and stays that way — but it is graft's *working* file, shaped
by what a sync needs to write and a prune needs to read, and tying a consumer's parser to it
means the lock cannot gain a field without breaking that consumer. `--json` is a stable
projection of it, which is what lets the two move independently, and being stable means being
a contract: pinned in a spec, asserted byte-for-byte, and not free to drift.

## What Changes

- **New `graft list`** — one block per source in `graft.lock`, naming the source, its `rev`,
  its short resolved sha, and the items installed from it with a file count each. It goes to
  the **standard output** stream, because a listing is the content the caller asked for, not
  a summary of something that happened.
- **New `graft list --json`** — the same information as a single JSON document on standard
  output: every source, every item, and every file path, with the full 40-character sha. Its
  shape is a **contract** — field names, field order, collection ordering, empty-collection
  rendering, indentation, and the trailing newline are all pinned by spec and asserted as
  exact bytes.
- **`list` reads `graft.lock` and nothing else.** Not `graft.toml`, not the working tree, not
  the network, and not the fetch cache. It resolves nothing, fetches nothing, and writes
  nothing — a repository with no lock is not an error, it is a repository with nothing
  installed.
- **`internal/list`** — a new package that turns a parsed lock into the listing and renders
  both forms. `internal/cli` gains a `list` command that is wiring only.
- **`internal/ui` gains the shared render vocabulary** the two commands must agree on: the
  file-count phrase (`1 file` / `6 files`) and the short-sha width. They move out of
  `internal/sync`'s renderer, unchanged, so that `list` and the sync report cannot disagree
  about how graft says the same thing twice.
- **`internal/itemid` gains `Split`** — `kind:name` is graft's grammar, and the JSON document
  carries the two halves so a consumer never has to re-implement it to filter by kind.
- **`graft`'s help gains a third command.** Additive; no existing line changes.

Not breaking: no change to `graft.toml`, `graft.lock`, or `catalog.yaml`, no change to what
`sync` or `update` do or print, and no `version` bump anywhere. The `version` field inside the
JSON document is that document's own, and starts at `1`.

## Non-Goals

- **No verification, no drift report, no tree scanning.** `list` prints what the lock says.
  It does not stat a destination, does not compare bytes, and does not report a file someone
  edited or deleted by hand. SPEC.md is explicit that there is no verification command
  because `git status` already is one, and a `list` that quietly grew one would be that
  command under another name.
- **No pin-drift check.** `list` does not read `graft.toml`, so a manifest whose `rev` moved
  ahead of the lock is not reported here. That disagreement is `sync`'s and `update`'s
  business and is already a failure mode on both; making an informational read fail because
  of it would be a fourth place to keep one rule.
- **No filtering, no selectors, no `--source`.** `graft list` takes no positional arguments.
  A caller who wants one source pipes `--json` into `jq`; a flag surface invented ahead of a
  request is a surface to keep working.
- **No second output format.** No `--format`, no template, no CSV. Two forms — one for a
  person, one for a program — and the second is versioned so it can be extended without a
  third.
- **No `--json` on `sync`, `update`, or anything else.** The report is a different artifact
  with a different audience and it would need its own contract; this change does not open
  that.
- **No self-hosting.** This repository's own `.claude/agents/` and `openspec/schemas/` are
  still hand-copied, and no test in this change runs graft against them.

## Capabilities

**New Capabilities**

- `list-execution` — what `graft list` reads and refuses to read, its argument surface, the
  two forms it prints, which stream each goes to, and the JSON document contract.

**Modified Capabilities**

- `command-invocation` — the help's commands section names the subcommands graft has, and
  there is a third one now. The requirement already generalises to "every subcommand", so the
  rule does not change; the sentence naming today's commands does, its *Help lists the commands
  graft has* scenario names `list` alongside `sync` and `update`, and one scenario is added for
  `graft list --help`.
- `command-output` — two changes. Its stream rule is generalised: stdout carries the content
  a caller asked for — help, `--version`, and now a listing — rather than only output shaped for
  a program, while a note about the *absence* of content stays on the error stream. And it gains
  the render vocabulary two commands now share: the file-count phrase and the short-sha width,
  one decision rather than one per renderer.

Unmodified and deliberately so: `lock-format` — `list` is a reader and adds no requirement to
the format it reads. `sync-report` — its lines are unchanged, and `list` deliberately does not
reuse its block structure beyond the two phrases named above.

## Impact

- `internal/list` — new: the listing value, the JSON document, the plain rendering.
- `internal/ui` — three exported render helpers, moved from `internal/sync` unchanged.
- `internal/sync` — calls them instead of its own copies. No behavior change; its existing
  render tests are the characterization.
- `internal/itemid` — `Split`, with `Valid` expressed through it.
- `internal/cli` — the `list` command, and the working-directory step extracted from the tail
  `sync` and `update` share so `list` uses the same one rather than a second copy.
- `cmd/graft` — nothing. It stays one call and one exit.
- `SPEC.md` — the `list` section it has never had, and the failure-mode rows this change adds.
- No external service, no background job, no sibling repository.
