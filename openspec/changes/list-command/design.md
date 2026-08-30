## Context

`graft sync` and `graft update` both end by writing `graft.lock`, and nothing reads it back
out for anybody. SPEC.md's command table has named `graft list` — *items installed here, with
source and resolved SHA* — since the table was written, and it is the last v1 command with no
implementation.

What exists to build on:

- `internal/lock` already parses and validates the file, and `lock.Load` treats an absent file
  as an empty lock at the current version — precisely the "nothing installed" case, already
  decided in the right place. What it does **not** do is order anything: `Parse` appends in
  on-disk order, and only `lock.Marshal` sorts. Ordering the listing is therefore this change's
  job, under the rule `Marshal` already follows — sources by name, items by id, files by path.
- `internal/ui` owns both streams, the error format, and the colour decision.
- `internal/cli` has a settled command shape: cobra, graft's own argument validator producing
  `unknown argument "<arg>"`, and a `perform` tail that `sync` and `update` share.
- `internal/sync/render.go` holds three formatting helpers — `short`, `fileCount`, `pad` — that
  a second renderer needs and cannot reach.

What does not exist: any machine-readable output beyond `--version`, and therefore any
precedent for what a graft JSON document looks like. That is the part of this change worth
designing rather than typing.

The constraint that shapes everything below: `list` is a **read-only** command. It reads one
file. It resolves nothing, fetches nothing, and writes nothing, so `internal/plan` and
`internal/apply` are not merely unused — they are unreachable from it.

## Goals / Non-Goals

**Goals**

- `graft list` prints what `graft.lock` records, on standard output, in a form a person reads.
- `graft list --json` prints the same information as a document a program consumes, with a
  shape pinned as exactly as SPEC.md's report example is pinned.
- Both forms are deterministic: same content, same bytes, whatever order the lock is written
  in.
- A repository with nothing installed is a normal outcome in both forms, not an error and not
  an empty screen.
- The phrases `list` and the sync report share become one decision rather than two copies.

**Non-Goals**

- No tree scanning, no drift detection, no verification. `git status` is that report.
- No reading of `graft.toml`, and therefore no pin-drift check inside `list`.
- No filtering flags, no second output format, no `--json` anywhere else.
- No change to what `sync` or `update` do or print. The helper move is structure only, and the
  existing render tests are what prove it.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/lock` | no | `Load` and `Parse` give `list` a validated lock in **on-disk order**, absent file included; ordering it is `FromLock`'s job. |
| `internal/manifest` | no | `list` never reads `graft.toml`. Reachable transitively — `lock.CheckPins` takes `[]manifest.Source` — and never called, which is why the import assertion below is over direct imports. |
| `internal/itemid` | yes | `Split`, used to fill the document's `kind` and `name`. |
| `internal/catalog` | no | — |
| `internal/source` | no | nothing is resolved and nothing is fetched. |
| `internal/plan` | no | still pure, still unreachable from a command that plans nothing. |
| `internal/apply` | no | unreachable. `list` writes nothing, so it does not need the sole writer; it needs no writer at all. |
| `internal/sync` | yes | three unexported helpers move to `internal/ui` and are called from there. No behavior change. |
| `internal/ui` | yes | gains `FileCount`, `ShortSHA`, and `Pad`, moved unchanged from `internal/sync`, and its stdout rule widens to name content the caller asked for. |
| `internal/list` | new | the listing value, its two renderings, and the one file it reads. |
| `internal/cli` | yes | the `list` command, and the working-directory step lifted out of `perform` so `list` uses the same one. |
| `cmd/graft` | no | one call, one exit. |

`internal/list` follows `internal/sync`'s shape: a `Run` that reads what it needs from a root
given as a value, and a result value whose renderings are pure functions of it. It is a
separate package rather than a function in `internal/sync` because `internal/sync` is the
resolution sequence — fetch, plan, apply — and `list` performs none of it. Putting a read-only
command inside the package that owns the write path would make "this command cannot write" a
claim about discipline rather than about imports.

## Contracts

**The JSON document is the published interface this change adds.** It is specified in full in
`specs/list-execution/spec.md` and reproduced here only as the Go shape that produces it:

```go
type Listing struct {
    Version int      `json:"version"`
    Sources []Source `json:"sources"`
}
type Source struct {
    Name     string `json:"name"`
    Git      string `json:"git"`
    Rev      string `json:"rev"`
    Resolved string `json:"resolved"`
    Items    []Item `json:"items"`
}
type Item struct {
    ID    string   `json:"id"`
    Kind  string   `json:"kind"`
    Name  string   `json:"name"`
    Files []string `json:"files"`
}
```

Field order in the document is struct field order, which `encoding/json` guarantees. Collection
order is imposed by the builder, not inherited from the lock. Every slice is allocated with
`make(..., 0, n)` so an empty one marshals as `[]` rather than `null` — the single most common
way a JSON contract breaks a consumer, and invisible in any test that only decodes.

`version` is the **document's** version, not the lock's. Both are `1` today and they are free
to move apart; the field's documentation says which one it is, because a reader will otherwise
assume the other.

Consumers affected: none yet — this is a new interface. `add-command` and `self-hosting` are
the next changes and neither consumes it. The contract exists from the moment it ships, which
is the reason to pin it now rather than after someone depends on it.

**Errors.** `list` introduces none of its own. Everything it can fail at is a message
`internal/lock` or `internal/cli` already words:

| Message | Raised by | Class |
|---|---|---|
| `graft.lock: version 2 is not supported by this graft; upgrade graft` | `internal/lock` | domain |
| `graft.lock: source "<name>": git is required`, and every other validation message | `internal/lock` | domain |
| `graft.lock: <io error>` | `internal/lock` | domain |
| `unknown argument "<arg>"` | `internal/cli` | usage — carries the hint line |
| `unknown flag: --<flag>` | cobra, through `internal/cli` | usage — carries the hint line |
| `cannot determine the working directory: <err>` | `internal/cli` | domain — the message `perform` already uses |

`nothing installed` is a note, not an error: no `graft: ` prefix, no hint line, exit `0`.

## Persistence and Rollout

- **Migration**: none. No format changes.
- **Backfill**: none.
- **Seeding**: none.
- **Cache invalidation**: none. `list` never touches the fetch cache and never creates its
  root.
- **Index rebuild**: none.
- **Authorization**: none. graft has no auth layer, and `list` reaches no remote.
- **Observability**: none. No telemetry, and this change adds none.
- **Deployment**: none. A binary release.
- **`graft.lock` format**: unchanged, `version` stays `1`. `list` is a reader.
- **`graft.toml` format**: unchanged and unread.
- **JSON document format**: new, starting at `version = 1`.

## Test Boundaries

| Dependency | In acceptance test | In integration tests | In unit tests |
|---|---|---|---|
| The compiled `graft` binary | real, run as a subprocess with its own `cmd.Dir` and `cmd.Env` | not used — `list.Run` is called in-process | not used |
| The consumer's working tree | real `t.TempDir()` | real `t.TempDir()` holding a hand-written `graft.lock` | **not used** — `FromLock`, `JSON`, and `Lines` are values in and values out, and `internal/ui`'s helpers are pure functions |
| `graft.lock` | real file, written by a real `graft sync` in the headline case and by hand elsewhere | real file, written by hand | in-memory `*lock.Lock` values |
| `graft.toml` | present but deliberately irrelevant — one case asserts a manifest whose rev disagrees changes nothing | present in one case for the same reason | not used |
| The source repository | real git repository in `t.TempDir()`, `user.name`/`user.email` set **on the repo**, used only to produce a real lock to list | not used — a hand-written lock is the input, and it is the input `list` actually has | not used |
| `git` on `PATH` | real for the sync that seeds the fixture; **unused by the listing itself**, and one case proves it by deleting the source repository first | not used | not used |
| The network | never reached | never reached | never reached |
| The fetch cache | real, rooted at an absolute `t.TempDir()` passed as `XDG_CACHE_HOME`; one case asserts `list` creates no entry under it | not used | not used |
| `internal/lock` | real | real | real, driven by hand-built `*lock.Lock` values |
| `internal/itemid`, `internal/ui` | real | real | real — both are pure functions with no filesystem and no state |
| The in-process command surface (`cli.Main` over `bytes.Buffer`) | not used | not used | real, for the argument surface: `internal/cli`'s existing `sync_test.go` / `update_test.go` pattern, which needs no working directory |
| `internal/plan`, `internal/apply`, `internal/source`, `internal/manifest` | unreachable from `list` | unreachable | unreachable |
| Output streams | real OS pipes across a process boundary | `[]byte` and `[]string` returned as values | `[]byte` and `[]string`; `internal/list`'s renderings take no `*ui.UI` at all, so no unit test here needs one |
| The developer's `~/.cache/graft` | unreachable — `XDG_CACHE_HOME` is set on the child, and `list` reads neither | unreachable | unreachable |
| The developer's git identity | unreachable — identity is set on the fixture repository | n/a | n/a |
| This repository's own tree, `.claude/agents/`, `openspec/schemas/` | unreachable — every root is a `t.TempDir()` | unreachable | unreachable |

Nothing is mocked. The one collaborator worth naming for its absence is `git`: `list` must work
with no network and no source repository, and the honest way to demonstrate that is to delete
the source repository and empty the cache before running it, which is what the acceptance tier
does.

## Test Strategy

Three tiers, the ones `sync-command` established:

- **unit** — `go test ./internal/list/ ./internal/ui/ ./internal/itemid/ ./internal/sync/`;
  values in, values out, no filesystem.
- **integration** — `go test ./internal/list/`; `list.Run` against a real `t.TempDir()` holding
  a hand-written `graft.lock`. No git repository is involved, because `list` has no source.
- **acceptance** — `go test ./internal/cli/`; the compiled binary as a subprocess, with a lock
  produced by a real `graft sync` in the headline case.

**This change takes the outer-loop acceptance test.** `graft list --json` is a new
client-visible command whose end-to-end wiring is exactly the risk this tier catches: a
document on the wrong stream, a trailing newline added twice by writing it through a
line-oriented printer, a listing built against the wrong root, an exit code that says success
while the document went nowhere. The `sync_acceptance_test.go` harness already builds the
binary, seeds a source repository, and carries an `XDG_CACHE_HOME`, so the cost is one more
subtest rather than a new tier.

### Verification matrix

One row per scenario across the three spec files, plus one for the outer loop, which has no
spec scenario of its own. Rows marked *existing* restate a scenario a MODIFIED requirement
carries unchanged; its test already exists and must still pass, which is the verification.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| **outer loop** (no spec scenario) | real binary in a repository synced by a real `graft sync`; `graft list --json` writes the exact document to stdout, stderr empty, exit `0`, and the tree is byte-identical afterwards | acceptance | real binary, real git, real cache, real tree | `go test ./internal/cli/` |
| list-execution: A lock with two sources is reported as two blocks | two-source lock built by hand; assert the rendered lines and their order | unit | none | `go test ./internal/list/` |
| list-execution: A manifest whose rev moved ahead of the lock is not reported | a `t.TempDir()` holding both files with disagreeing revs; assert the listing names the lock's rev and `Run` returns no error | integration | real tree | `go test ./internal/list/` |
| list-execution: A lock claiming a file that is not there still lists it | delete a destination after syncing, then list; assert the path is still in the document | acceptance | real binary, real tree | `go test ./internal/cli/` |
| list-execution: A listing runs with no source repository reachable | delete the fixture repository and the cache entry, then list; assert the same output and exit `0` | acceptance | real binary, real tree | `go test ./internal/cli/` |
| list-execution: A repository with no lock prints a note on stderr | `graft list` in a directory holding only `graft.toml`; exact stderr, byte-empty stdout, exit `0`, and a directory listing proving nothing was created | acceptance | real binary, real tree | `go test ./internal/cli/` |
| list-execution: A repository with no lock still prints a JSON document | exact stdout bytes for the empty document | acceptance | real binary | `go test ./internal/cli/` |
| list-execution: A lock declaring no source is the same as no lock | `version = 1` and no `[[source]]`; assert both renderings equal the no-lock ones | integration | real tree | `go test ./internal/list/` |
| list-execution: A directory with no graft files at all is not an error | `graft list` in an empty `t.TempDir()`; exact stderr and exit `0` — the case that would fail if `list` read `graft.toml` | acceptance | real binary | `go test ./internal/cli/` |
| list-execution: SPEC.md's own lock renders as one block | **contract fixture**: SPEC.md's `graft.lock` example as the input, its rendering asserted line for line, trailing whitespace checked | unit | none | `go test ./internal/list/` |
| list-execution: Two sources are separated by one blank line | exact line slice | unit | none | `go test ./internal/list/` |
| list-execution: A source with no installed items is its header alone | exact line slice; the block is one line with no blank after it | unit | none | `go test ./internal/list/` |
| list-execution: A scrambled lock lists in the same order as a canonical one | two locks parsed from two fixture files; assert the renderings are equal | integration | real tree | `go test ./internal/list/` |
| list-execution: A resolved sha shorter than seven characters is printed whole | header rendered from a listing whose resolved value is `abc` | unit | none | `go test ./internal/list/` |
| list-execution: SPEC.md's own lock renders as this exact document | **contract fixture**: the document asserted as exact bytes against a golden string held in the test | unit | none | `go test ./internal/list/` |
| list-execution: The document is valid JSON that round-trips | decode into `map[string]any`, compare every value against the source lock, assert the full 40-character sha | unit | none | `go test ./internal/list/` |
| list-execution: A source with no items renders `[]` rather than null | exact bytes, plus an assertion that `null` appears nowhere in the document | unit | none | `go test ./internal/list/` |
| list-execution: An item with no files renders `[]` rather than null | exact bytes | unit | none | `go test ./internal/list/` |
| list-execution: A scrambled lock produces the same document as a canonical one | byte equality across two locks, and byte equality across two runs of the same lock | integration | real tree | `go test ./internal/list/` |
| list-execution: A git URL containing an ampersand is not escaped | assert the document contains the literal URL and no `&` | unit | none | `go test ./internal/list/` |
| list-execution: The kind and name halves match the id | assert the three fields for `schema:tdd`, driven through `itemid.Split` | unit | none | `go test ./internal/list/` |
| list-execution: A listing leaves the working tree byte-identical | full recursive tree snapshot — paths and bytes — before and after both forms | acceptance | real binary, real tree | `go test ./internal/cli/` |
| list-execution: A listing creates no cache directory | `XDG_CACHE_HOME` set to a `t.TempDir()`; assert no `graft` entry exists under it afterwards | acceptance | real binary, real cache root | `go test ./internal/cli/` |
| list-execution: A lock from a newer graft is refused | exact stderr, byte-empty stdout, exit `1`, for both forms | acceptance | real binary | `go test ./internal/cli/` |
| list-execution: A malformed lock is refused before anything is printed | exact stderr, and stdout asserted byte-empty — no opening brace | acceptance | real binary | `go test ./internal/cli/` |
| list-execution: A lock that is a directory is refused | `graft.lock` created as a directory; assert the `graft: graft.lock: ` prefix, empty stdout, exit `1` | integration | real tree | `go test ./internal/list/` and `go test ./internal/cli/` |
| list-execution: A positional argument is a usage error | exact stderr, hint line, byte-empty stdout, exit `1` | acceptance | real binary | `go test ./internal/cli/` |
| list-execution: Only the first positional argument is named | assert `shared` appears and `other` does not | acceptance | real binary | `go test ./internal/cli/` |
| list-execution: An unknown flag is a usage error | exact stderr `graft: unknown flag: --format`, hint line, exit `1` | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: No arguments prints help and succeeds | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: Help lists the commands graft has | *existing* test, extended to name `list` | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: `--help` prints the same text as no arguments at all | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: `help` is not a command | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: `help sync` is not a command either | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: A subcommand's own help goes to standard output | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: A read-only subcommand's help names only the flag it has | `graft list --help`: stdout names `list` and `--json` and not `--dry-run`; stderr empty; exit `0` | acceptance | real binary | `go test ./internal/cli/` |
| command-output: One file is singular and every other count is plural | table test over `1`, `6`, `0` | unit | none | `go test ./internal/ui/` |
| command-output: A short sha is the first seven characters | exact string | unit | none | `go test ./internal/ui/` |
| command-output: A sha too short to shorten is returned as it is | exact strings for `abc` and `abcdefg` | unit | none | `go test ./internal/ui/` |
| command-output: Padding fills to the width and never beyond it | exact strings for the three cases | unit | none | `go test ./internal/ui/` |
| command-output: The sync report is unchanged by where these live | **characterization**: `internal/sync`'s existing `TestReportAlignment` against SPEC.md's example, unchanged, still green | unit | none | `go test ./internal/sync/` |

Every scenario appears. No task may reach for a collaborator the Test Boundaries table does not
name.

**Visual design source.** Not applicable: this change ships no user-facing view and no email
template. Its entire output surface is two text renderings, both specified as exact bytes in
`specs/list-execution/spec.md`.

## Decisions

**The plain listing goes to standard output, not to the error stream.**
Alternative considered: the error stream, matching the sync report. Rejected. SPEC.md splits
the streams by audience — machine-readable output to stdout, *progress, summaries, and errors*
to stderr — and a listing is none of those three. It is the content the caller asked for, which
is why `--version` and help already go to stdout. Sending it to stderr would leave stdout
byte-empty on the one command whose entire purpose is to emit content, and would make `--json`
the only usable form rather than the machine-readable one. `graft list | grep agent:` has to
work, and it is the reason the command exists at all.

The consequence, stated rather than hidden: `nothing installed` is *not* on stdout. It is a
note about the absence of content, so it goes where notes go, and a caller piping the plain
form gets zero bytes rather than a sentence that parses as an item. `--json` is the form that
answers "nothing" in the same shape as "something".

**`list` reads `graft.lock` and not `graft.toml`.**
Alternative considered: read both and report drift — a `rev` in the manifest ahead of the
lock's. Rejected on two grounds. First, ownership: that disagreement is already a failure mode
of `sync` and the thing `update` repairs, and a third and fourth statement of one rule is three
places to keep it true. Second, failure surface: reading `graft.toml` means `graft.toml not
found` becomes an error of `list`, so the command would fail in exactly the repository where
someone is most likely to run it to find out what is going on. The command answers *what is
installed*, and the lock is the only record of that.

**`--json` is a document with its own `version`, not the lock in JSON.**
Alternative considered: marshal `lock.Lock` directly. Rejected: `graft.lock`'s keys are graft's
private business, and a consumer told to parse them is a consumer graft can no longer refactor
around. The document is a projection with its own version, which is what lets the lock gain a
field without changing the contract, and the contract gain a field without changing the lock.

**The document carries `kind` and `name` beside `id`.**
Alternative considered: `id` alone, since `kind:name` splits on a colon. Rejected: the grammar
is graft's — exactly one colon, non-empty halves, enforced by `internal/itemid` — and a
consumer filtering by kind would have to re-implement it, in a language where the obvious
`split(":")` is wrong for a name containing one. Carrying six bytes of derived data is cheaper
than exporting a parsing rule.

**Rendering is `encoding/json` with the encoder configured, not `json.Marshal`.**
`json.NewEncoder` with `SetIndent("", "  ")` and `SetEscapeHTML(false)`, into a
`bytes.Buffer`. `SetEscapeHTML(false)` is the load-bearing half: Go escapes `<`, `>`, and `&`
by default, so a git URL with a query string comes back with `&` in it and the document
stops round-tripping what the lock holds. `Encode` also appends the trailing newline, which is
why `JSON()` returns the complete document rather than a string the command surface adds a
newline to — one owner for the document's bytes, and a unit test that pins all of them.

`Encode`'s error is discarded with an explicit `_ =` and a comment: the value is a tree of
strings, ints, and slices of those, and the writer is a `bytes.Buffer`. Neither can fail.
Returning an `error` from `JSON()` would put an unreachable branch in every caller and a hole
in the coverage gate.

**Three formatting helpers move from `internal/sync` to `internal/ui`.**
Alternative considered: copy them into `internal/list`. Rejected — `1 file` versus `1 files`
and seven characters of sha versus eight are user-visible strings that two commands must agree
on, and two copies is the agreement site that fails silently. `internal/ui` is where graft's
output decisions live and it already owns the colour rule that the padding must not disturb.
The move is structure only: `internal/sync`'s existing render tests assert SPEC.md's example
byte for byte and are the characterization that proves it.

**`internal/list` is a package, not a function in `internal/sync`.**
`internal/sync` is the resolution sequence and the package that reaches `internal/apply`.
`list` performs none of it. Keeping them apart makes "this command cannot write" an observable
property of the import graph rather than a claim in a comment.

**The working-directory lookup is lifted out of `perform` rather than copied.**
`perform` is the tail `sync` and `update` share; `list` needs its first two lines and not its
last four. Extracting the lookup gives all three commands one `cannot determine the working
directory: ` message, and leaves `perform` doing exactly what it did.

## Risks / Trade-offs

**[A listing can disagree with the tree and says nothing about it]** → Accepted and specified.
`list` reports the lock; a file someone deleted by hand is still listed. Mitigation is that
this is the same trade `sync` already makes when it reports `up to date` after restoring
hand-deleted files, and SPEC.md is explicit that `git status` is where a sync's effect shows
up. A `list` that stat-ed every destination would be the verification command SPEC.md refuses.

**[The JSON contract is pinned before anyone consumes it]** → Mitigated by keeping the document
a projection of the lock, which is itself specified, and by giving it a `version`. The shapes
most likely to be regretted — a map keyed by source name, a flat list of files with no item
structure — are the ones that cannot be extended; an array of objects with a version can gain a
field without breaking a consumer.

**[Two commands' output could drift apart anyway]** → Mitigated only partly. The three moved
helpers are shared; the block structure is not, and `list` deliberately renders a different
block from the sync report. A future change that unifies them further should do it on purpose,
not by accident.

**[`--json` on a huge lock builds the whole document in memory]** → Accepted. The lock is
already fully in memory, having been parsed, and the largest realistic lock is a few hundred
paths. Streaming a JSON encoder over it would trade a contract that can be asserted as bytes
for one that cannot.

## Migration Plan

None required. `graft list` is additive: no file format changes, no behavior change to `sync`
or `update`, and no state to migrate. A user on an older graft who upgrades gains a command;
a user who does not, loses nothing. Rollback is removing the command.

## Open Questions

None blocking. One noted for a later change, not this one: if `add-command` or `self-hosting`
turns out to want a machine-readable view of what a *source* offers — the catalog rather than
the lock — that is `graft add --list`'s business under its own change, and it should reuse this
document's conventions (a versioned object, arrays never null, `kind`/`name` beside `id`)
rather than invent a second dialect.
