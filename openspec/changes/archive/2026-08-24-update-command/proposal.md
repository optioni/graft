## Why

`graft sync` installs what `graft.lock` says and never re-resolves a pin. That is deliberate —
it is what keeps `rev = "main"` from drifting under a user between syncs — but it leaves the
tool with no way to move a pin at all. A consumer who wants a newer version of a source today
has three options, and all three are wrong: hand-edit `graft.lock`, which is the one file
SPEC.md says not to edit; hand-edit `graft.toml`, which makes the next `sync` fail with a pin
disagreement pointing at a command that does not exist; or delete the lock and lose every
other source's pin along with the one being moved.

`graft update` is the command that half of `sync`'s guarantee has been pointing at since the
guarantee was written. Its whole job is to be the one place a pin moves, so that everywhere
else can promise it does not.

## What Changes

- **New `graft update`** — re-resolve every source's `rev` in `graft.toml` to the sha it names
  today, then do everything `sync` does with the result: fetch, plan, write, prune, and write
  the lock. The report the sync already renders is what says what moved.
- **New `graft update <source>`** — the same, for one source. Every other source's sha still
  comes from the lock and is not re-resolved, so moving one pin cannot move another.
- **New `graft update --to <rev> <source>`** — move the pin in `graft.toml` first, then
  re-resolve and sync. It is the only path in `graft update` that writes `graft.toml`; `graft add`
  will write it too, under its own change.
- **`internal/sync` gains an `Update` option** rather than a second orchestration beside it.
  `sync` passes none and keeps its pin exactly as it is today; `update` names which sources
  re-resolve. The sequence, the failure surface, the apply path, and the report are one
  implementation with one caller-visible difference.
- **`internal/manifest` gains `SetRev`** — pure, returning bytes. It replaces the value of one
  source's `rev` and leaves every other byte of `graft.toml` alone: comments, key order, and
  the alignment SPEC.md's own example uses all survive, because `graft.toml` is a
  human-written file that a consumer reviews in a diff. A shape it cannot rewrite exactly is
  refused rather than guessed at, and `internal/sync` re-parses the result and checks the rev
  it asked for actually landed — a text edit's real failure is landing on the wrong line.
- **`internal/apply` gains an option to write those bytes**, immediately before `graft.lock`
  and only after every other file operation has succeeded, through a temporary file and a
  rename so the consumer's manifest is never absent or half-written. It stays the sole writer;
  the refusal of `graft.toml` and `graft.lock` as *plan* destinations is unchanged, because a
  source placing a file there is a different thing from graft moving its own pin.
- **`graft`'s help gains a second command.** This is additive — no existing line changes — and
  is not marked breaking.

Not breaking: no format changes to `graft.toml`, `graft.lock`, or `catalog.yaml`, no change to
`graft sync`'s behavior or output, and no `version` bump anywhere.

## Non-Goals

- **No version ranges and no semver resolution.** `rev` is a tag, a branch, or a full sha, and
  `--to` takes exactly one of those. `graft update` moves a pin to what its rev names *today*;
  it does not decide that one rev is newer than another. Choosing the latest semver tag is
  `add`'s default in a later change and stays there.
- **No dependency resolution.** A source's catalog cannot cause another source to update, and
  `update` never consults one source to decide another's pin. PRD non-goal, untouched.
- **No `--check`, no `--dry-run`-shaped exit code.** `--dry-run` prints the plan and exits `0`
  whether or not anything moved, exactly as it does for `sync`. A flag that exits non-zero to
  mean "an update is available" is a CI feature this change does not argue for.
- **No `graft.toml` reformatting.** `SetRev` moves one value. It does not normalize alignment,
  sort sources, or rewrite anything it was not asked to.
- **No new interactive behavior.** There is no confirmation prompt and no TTY-only path; the
  `add` picker remains the only interactive thing graft will have.
- **No self-hosting.** This repository's own `.claude/agents/` and `openspec/schemas/` are
  still hand-copied. Converting them is `self-hosting`, the last change in the roadmap.

## Capabilities

**New Capabilities**

- `update-execution` — what `graft update` re-resolves, what it refuses to, its argument
  surface, `--to`, `--dry-run`, and how it reports what moved.

**Modified Capabilities**

- `manifest-format` — gains moving a source's `rev` in place, as bytes, without disturbing the
  rest of the file.
- `file-application` — its entry point gains an optional set of manifest bytes and its ordered
  step list gains a conditional `graft.toml` write, so *Applying a plan is the only path that
  writes to the working tree* is **MODIFIED** rather than merely supplemented; the write itself
  is an added requirement beside it.
- `command-invocation` — the help's commands section is generalised from "that subcommand" to
  every subcommand, so `update` appearing there is the rule rather than an amendment.

Unmodified and deliberately so: `sync-execution` scopes every one of its requirements to
`graft sync`, and none of them changes. `sync-report` already anticipated this command — one of
its scenarios reads "a source pinned at `rev = "main"` is moved by `graft update`" — and is
reused unaltered rather than restated, so there is one owner of the report's format.

## Impact

- `internal/sync` — `Options` gains `Update`; the resolve loop gains a refresh set; the pin
  check narrows to the sources not being refreshed.
- `internal/manifest` — `SetRev`, and a `Read` that returns the bytes alongside the parsed
  manifest so the edit works on what was actually on disk.
- `internal/apply` — one option, one write, one pre-flight entry.
- `internal/cli` — an `update` command, and the shared tail `sync` and `update` both end in.
- `cmd/graft` — nothing. It stays one call and one exit.
- No external service, no background job, no sibling repository.
