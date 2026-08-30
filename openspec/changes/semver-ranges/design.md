## Context

`rev` names one immutable thing. Moving a source forward is a hand edit to `graft.toml`,
and `graft update` cannot help: it re-resolves the rev it was given rather than choosing a
newer one. The only way to track a source today is `rev = "main"`, which pins nothing.

A range is the missing middle, and it lands against a codebase that already has the shape
for it. `internal/source.Resolve(name, git, rev) (string, error)` is the single place a rev
becomes a sha; `internal/sync` takes re-resolution as a parameter, which is the one
difference between `sync` and `update`; and `graft.lock` already records the request beside
what it became. Nothing here needs a new layer — it needs a second branch inside `Resolve`
and one more column in three renderings.

The constraint that shapes every decision below: **`sync` must stay reproducible and
offline**. A range that re-evaluated on sync would break both.

## Goals / Non-Goals

**Goals:**

- A `rev` may be a semver range, resolved to a concrete tag and recorded as a sha.
- The range-versus-ref decision is syntactic, decidable offline, and identical everywhere
  it is asked — resolution and lock validation call one function.
- `graft.lock` records which tag a range matched, so a version bump stays reviewable as a
  diff.
- Locks and reports containing no range are byte-identical to what graft produces today.

**Non-Goals:**

- No dependency resolution, no transitive sources, no constraint solving across sources.
- No re-evaluation on `sync`, under any flag.
- No `graft add` behavior. `add-command` is the next change and decides what it writes.
- No `graft outdated` and no drift report against the tag list.
- No ordering for sources whose tags are not semver.

## Boundaries

| Package | Change | Pattern it follows |
|---|---|---|
| `internal/rev` | **New leaf package.** `IsRange`, and nothing else. Imports `strings` and no package of graft's own. | `internal/itemid` — a one-predicate leaf that states a piece of graft's grammar once. |
| `internal/source` | New `MatchRange` and tag listing. `Resolve` gains one branch and a second return value. | `resolve.go`'s existing `ls-remote` call — same `gitOutput` helper, same URL guard, same prompt-disabling. |
| `internal/lock` | The `matched` field: parse, validate, serialize, omit. | The `resolved` field beside it, in every one of those four places. |
| `internal/plan` | `Input` gains `Matched`, carried verbatim into the `lock.Source` it builds. It stays pure — one more string travelling through, and no opinion about what it means. | `Resolved` beside it, which `plan` also carries without interpreting. |
| `internal/sync` | Carries the matched tag into the report, from resolution on the refresh path and **from the previous lock** on the non-resolving path. | `pinned` in `run.go`, which already carries the sha exactly that way. |
| `internal/list` | One field in `Source`, one column in `Lines()`. | The `rev` field beside it. |
| `internal/manifest` | **Nothing.** It never judged a rev's syntax. | — |
| `internal/apply` | **Nothing.** This change adds no write path. The only bytes it causes to be written are `graft.lock`'s, which `apply` already writes. | — |
| `internal/catalog` | **Nothing.** | — |

**Why `IsRange` gets its own package rather than living in `internal/source`.** Both
`internal/lock` and `internal/source` must ask it, and they may never disagree — a lock that
disagrees with resolution about whether `1.x` is a range would demand `matched` for a pin
that has none, or accept a lock resolution can never reproduce. The obvious home is
`internal/source`, and it does not compile: `internal/source/listing.go` imports
`internal/plan`, and `internal/plan/build.go` imports `internal/lock`, so `lock -> source`
closes a cycle. A leaf package is the only shape that lets one definition serve both, and
`internal/lock` already imports exactly such a package in `internal/itemid`.

`internal/plan` calls `IsRange` for **nothing**. It carries `Matched` verbatim and forms no
opinion about whether a rev is a range, because a third opinion is a third thing to keep in
agreement. The lock validates; the plan transports.

## Contracts

Three interfaces a separate consumer depends on, and all three move.

**`graft.toml`** — additive. A value that graft rejected at resolution time is now
accepted. No existing manifest changes meaning: every rev valid today is still classified a
ref and resolves identically. Consumers affected: none, until one writes a range.

**`graft.lock`** — additive, and `version` does **not** move. The lock gains an optional
`matched` key written only for a range. `version = 1` stays, because a v1 reader
encountering a lock without ranges sees the identical bytes it sees today, and a lock *with*
a range could not have been produced by an older graft anyway. The alignment column is
already `resolved`'s width, which is wider than `matched`, so no existing line shifts.
Consumers affected: `internal/lock`, and any consumer parsing the lock directly — SPEC.md
publishes the format, so this is documented, not silent.

**`graft list --json`** — **breaking**, and `version` moves from `1` to `2`. Each source
object gains a `matched` member between `rev` and `resolved`, present unconditionally and
empty for a ref. The alternative — omitting it for refs — keeps a v1 document byte-identical
but makes every consumer branch on presence, and the document's own rule is that an empty
thing renders as an empty thing rather than as absence. A consumer pinned to version 1
learns from the number rather than from a decode failure, which is what the field is for.

Error surface: three new messages, all in `rev-ranges`' wording, all carrying the existing
`source "<name>": ` prefix. No existing message changes. Two new lock validation messages
follow `internal/lock`'s existing `graft.lock: source "<name>": ` shape.

No pagination, no streaming, nothing else.

## Persistence and Rollout

- **Migration**: none. Existing locks parse unchanged and re-serialize to identical bytes.
- **Backfill**: none. `matched` is meaningless for a ref and is never written for one.
- **Seeding**: none.
- **Cache invalidation**: none. The fetch cache is keyed by commit sha, and a range produces
  a sha like anything else; a range that selects an already-cached sha is a cache hit.
- **Index rebuild**: none.
- **Authorization**: unchanged. Tag listing runs against the same clone URL through the same
  guard, with terminal prompting disabled, so a private source fails rather than hangs.
- **Observability**: none beyond the report line, which is the product.
- **Deployment**: none. graft is a binary a consumer runs; there is nothing to roll out.

## Test Boundaries

| Dependency | In acceptance test | In integration tests | In unit tests |
|---|---|---|---|
| The built `graft` binary | real — run as a child process via `buildGraft` / `runGraftIn` | not used | not used |
| `git` binary | real, on `PATH` | real | not started at all — classification, parsing, and selection run no process |
| Source git repository | real, built in `t.TempDir()` with `user.name`/`user.email` set **on the repo**, tags via the existing `(*sourceRepo).tag` | real, same shape | replaced by a plain `[]string` of tag names |
| A source repository that existed and was then **deleted** | real — removed mid-test to prove no tag listing happens on the sync path | real, same | not used |
| Network | none — every remote is a local path | none | none |
| Filesystem (working tree) | real `t.TempDir()`, snapshotted before and after | real `t.TempDir()` | none |
| Package `testdata` fixture files | not used | not used | **real, read-only** — `internal/lock/testdata/canonical.lock` and `scrambled.lock`, which the byte-identity proof in task 5.1 reads |
| Fetch cache | real, under an absolute `XDG_CACHE_HOME` per test | real, and snapshotted in task 4.4 to prove a range resolution writes nothing | none |
| `graft.lock` | real file, written and re-read | real file | replaced by a hand-built `*lock.Lock` |
| `graft.toml` | real file, hand-written | real file | replaced by a hand-built `*manifest.Manifest` |
| `internal/rev` | real | real | real — a pure predicate; replacing it would test a fake |
| `internal/plan` | real | real | real — values in, a plan out, no filesystem |
| `github.com/Masterminds/semver/v3` | real | real | real — pure, deterministic, no I/O |
| Environment | `XDG_CACHE_HOME` set per test; `gitEnv` appends `GIT_TERMINAL_PROMPT=0` to `os.Environ()` | same | not touched |
| Clock, user | not touched | not touched | not touched |

No test reaches the network. The one network access this change causes is `go get` fetching the
new dependency at build time, in task 1.2.

**Why nothing else in `internal/source` applies to the new call site.** `ls-remote` transfers
no working-tree bytes, so `fetch.go`'s `attr.tree` guard and its `requireVersion` check are
checkout concerns and correctly irrelevant here; the `os.OpenRoot` reads in `listing.go` are
cache-entry reads and equally so. What does carry over is `CloneURL`'s leading-dash refusal,
the `--` separator, and `gitEnv`'s prompt-disabling — all three reused rather than reimplemented.
This is recorded because the question will be asked again.

## Test Strategy

Three tiers, as the repository already has them: **unit** over pure logic in
`./internal/...`, **integration** against fixture git repositories in `t.TempDir()`, and
**acceptance** in `internal/cli` running the built binary as a child process.

The rule that decides placement: anything that can be decided from a string or a value goes
to unit. Anything that needs a real tag to exist goes to integration. Anything a user could
observe as output on a stream goes to acceptance.

**This change takes an outer-loop acceptance test.** The end-to-end risk is real and not
covered by any tier below it: a range in `graft.toml`, resolved by `graft update`, recorded
in `graft.lock` with its matched tag, rendered in the report, and read back by
`graft list --json` at version 2 — six packages agreeing about one new field. The
concentration point is the pair of promises that a lock with no range is byte-identical and
that `sync` never re-evaluates: both are invisible to any single-package test.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| list-execution: A repository with no lock prints a note on stderr | `list-execution` test asserting the stated bytes/error | acceptance | real binary, real filesystem, no network | `go test ./internal/cli/` |
| list-execution: A repository with no lock still prints a JSON document | `list-execution` test asserting the stated bytes/error | acceptance | real binary, real filesystem, no network | `go test ./internal/cli/` |
| list-execution: A lock declaring no source is the same as no lock | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: A directory with no graft files at all is not an error | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: SPEC.md's own lock renders as one block | `list-execution` test asserting the stated bytes/error | unit | none — a built `*list.Listing` | `go test ./internal/list/` |
| list-execution: A range source names the tag it matched | `list-execution` test asserting the stated bytes/error | unit | none — a built `*list.Listing` | `go test ./internal/list/` |
| list-execution: Two sources are separated by one blank line | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: A source with no installed items is its header alone | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: A scrambled lock lists in the same order as a canonical one | existing `internal/list/run_test.go` case, real fixture lock files | integration | real filesystem, `repoWith` writing canonical and scrambled locks | `go test ./internal/list/` |
| list-execution: A resolved sha shorter than seven characters is printed whole | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: A ref source's header gains no column | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: SPEC.md's own lock renders as this exact document | `list-execution` test asserting the stated bytes/error | unit | none — a built `*list.Listing` | `go test ./internal/list/` |
| list-execution: A range source carries the tag it matched | `list-execution` test asserting the stated bytes/error | unit | none — a built `*list.Listing` | `go test ./internal/list/` |
| list-execution: The document is valid JSON that round-trips | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: A source with no items renders `[]` rather than null | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: An item with no files renders `[]` rather than null | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: A scrambled lock produces the same document as a canonical one | existing `internal/list/run_test.go` case, real fixture lock files | integration | real filesystem, `repoWith` writing canonical and scrambled locks | `go test ./internal/list/` |
| list-execution: A git URL containing an ampersand is not escaped | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| list-execution: The kind and name halves match the id | `list-execution` test asserting the stated bytes/error | unit | none — a built listing value | `go test ./internal/list/` |
| lock-format: A range's lock carries the matched tag | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A ref's lock carries no matched key | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A lock with no ranges is byte-identical after this change | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A matched key on a ref pin is refused | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A range pin without a matched key is refused | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An empty matched value is refused | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A missing version is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A newer version fails and says to upgrade | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A version below 1 is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A range-bearing lock still declares version 1 | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An older graft reports the unknown key rather than the upgrade message | `lock-format` test asserting the stated bytes/error | unit | none — asserted as the documented degradation, not executed | `go test ./internal/lock/` |
| lock-format: A missing resolved sha is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A malformed resolved sha is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A duplicate source name is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A malformed item id is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A duplicate item id within a source is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An item with no files is valid | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An escaping file path is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A path that is not in cleaned form is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: One path claimed by two items is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An unknown key written as an inline table is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A duplicate file within an item is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An unknown key is an error | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: The range test is the same one resolution uses | `lock-format` test asserting the stated bytes/error | unit | real `internal/rev` — the agreement site | `go test ./internal/lock/` |
| lock-format: A populated lock serializes to the documented layout | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A range source serializes matched between rev and resolved | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A files list of one is inline and a list of many is exploded | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: An empty lock serializes to header and version only | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| lock-format: A path needing escaping round-trips | `lock-format` test asserting the stated bytes/error | unit | none — bytes in, bytes out | `go test ./internal/lock/` |
| rev-ranges: A caret rev is a range | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: A plain tag is a ref | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: A branch name containing a dash is a ref | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: A compound range with a space is a range | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: An alternation is a range | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: A bare x-range is a ref, not a range | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: A full sha is a ref and never a range | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/rev/` |
| rev-ranges: A caret range parses | `rev-ranges` test asserting the stated bytes/error | unit | none — pure string logic | `go test ./internal/rev/ ./internal/source/` |
| rev-ranges: A malformed range is refused without a network call | `rev-ranges` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/source/` |
| rev-ranges: A malformed range does not fall back to a ref lookup | `rev-ranges` test asserting the stated bytes/error | integration | real `git`, fixture repo with that branch | `go test ./internal/source/` |
| rev-ranges: The highest satisfying tag wins | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: A tag without the v prefix is accepted | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: Unparseable tags are ignored rather than refused | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: An annotated tag resolves to its commit | `rev-ranges` test asserting the stated bytes/error | integration | real `git ls-remote`, fixture repo | `go test ./internal/source/` |
| rev-ranges: A range matching exactly one tag | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: An exact-version range selects that version | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: A prerelease is not selected by a plain range | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: A range naming a prerelease admits it | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: Only prereleases exist and the range names none | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: No tag satisfies the range | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: The source publishes no semver tags | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: The source publishes no tags at all | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: An unreachable remote under a range reports the network failure | `rev-ranges` test asserting the stated bytes/error | integration | real `git`, a path that does not exist | `go test ./internal/source/` |
| rev-ranges: Tag order from the remote does not affect the result | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: Two tags naming the same version | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| rev-ranges: Resolving a range writes nothing | `rev-ranges` test asserting the stated bytes/error | integration | real `git`, fixture repo, tree snapshot | `go test ./internal/source/` |
| rev-resolution: A branch resolves to its tip | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, fixture repo in `t.TempDir()` | `go test ./internal/source/` |
| rev-resolution: A lightweight tag resolves to its commit | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, fixture repo in `t.TempDir()` | `go test ./internal/source/` |
| rev-resolution: An annotated tag resolves to the commit, not the tag object | `rev-resolution` test asserting the stated bytes/error | integration | real `git ls-remote`, fixture repo | `go test ./internal/source/` |
| rev-resolution: A tag wins over a branch of the same name | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, fixture repo in `t.TempDir()` | `go test ./internal/source/` |
| rev-resolution: A full sha passes through without contacting the remote | `rev-resolution` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/source/` |
| rev-resolution: An uppercase sha is not treated as a sha | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, fixture repo in `t.TempDir()` | `go test ./internal/source/` |
| rev-resolution: A range resolves through the range path and reports its tag | `rev-resolution` test asserting the stated bytes/error | integration | real `git ls-remote`, fixture repo | `go test ./internal/source/` |
| rev-resolution: A rev no ref matches | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, fixture repo in `t.TempDir()` | `go test ./internal/source/` |
| rev-resolution: An abbreviated sha is not a rev | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, fixture repo in `t.TempDir()` | `go test ./internal/source/` |
| rev-resolution: An unreachable remote | `rev-resolution` test asserting the stated bytes/error | integration | real `git`, a path that does not exist | `go test ./internal/source/` |
| rev-resolution: An empty rev | `rev-resolution` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/source/` |
| rev-resolution: A remote that looks like an option is refused | `rev-resolution` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/source/` |
| rev-resolution: git is not on PATH | `rev-resolution` test asserting the stated bytes/error | unit | a PATH with no git | `go test ./internal/source/` |
| rev-resolution: An option-shaped remote is refused under a range too | `rev-resolution` test asserting the stated bytes/error | unit | none — no git process is started | `go test ./internal/source/` |
| sync-execution: A moved branch does not move the pin | `sync-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| sync-execution: A newer tag satisfying a range does not move the pin | `sync-execution` test asserting the stated bytes/error | acceptance | real binary, fixture repo deleted after sync | `go test ./internal/cli/` |
| sync-execution: A source with no lock entry is resolved once and recorded | `sync-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| sync-execution: A range source with no lock entry is resolved once and recorded | `sync-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| sync-execution: A range with no lock entry that no tag satisfies fails the run | `sync-execution` test asserting the stated bytes/error | integration | real `git ls-remote`, fixture repo | `go test ./internal/source/` |
| sync-execution: A rev that no longer exists fails, naming the rev and the source | `sync-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| sync-execution: There is no flag to make sync re-resolve or refuse to overwrite | `sync-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| sync-plan: A resolved sha that is not a sha fails the plan | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: An item placed in two destinations records both files | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: The next lock carries rev and resolved separately | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: The next lock round-trips through the lock parser | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: Sources, items, and files are ordered independently of input order | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: A range source's next lock carries the matched tag and round-trips | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: A ref source's next lock carries no matched tag | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: A bad resolved sha is refused before anything is planned | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-plan: The next lock round-trips through the lock's own parser | `sync-plan` test asserting the stated bytes/error | unit | none — values in, a plan out | `go test ./internal/plan/` |
| sync-report: A version bump shows both revs and both shas | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: A newly added source shows one rev and one sha | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: A branch pin whose sha moved shows both shas and one rev | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: Two sources are separated by a blank line | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: A range whose matched tag moved shows the range once and the tag twice | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: A newly added range source shows the range and its tag once each | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: A range whose tag did not move renders every half once | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| sync-report: A report with no range is unchanged | `sync-report` test asserting the stated bytes/error | unit | none — a built report value | `go test ./internal/sync/` |
| update-execution: A moved branch moves the pin | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: An update that finds nothing new reports nothing | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: An update in a repository with no lock installs everything | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: A source dropped from the manifest is pruned without being re-resolved | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: A rev that no longer exists fails without touching the tree | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: An item the new rev no longer provides is removed, and a repo-owned file beside it survives | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: A manifest declaring no sources updates nothing | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: A new tag satisfying a range moves the pin | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: A new tag outside the range does not move the pin | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| update-execution: `graft sync` does not re-evaluate a range | end-to-end, with the source repository deleted — expressible at integration, kept at acceptance deliberately as this change's outer-loop concentration point | acceptance | real binary, real `git`, fixture repo deleted mid-test, temp cache | `go test ./internal/cli/` |
| update-execution: A range that stops matching is an update failure, not a sync failure | `update-execution` test asserting the stated bytes/error | integration | real `git`, fixture repo, temp cache | `go test ./internal/sync/` |
| rev-ranges: Build metadata does not affect precedence | `rev-ranges` test asserting the stated bytes/error | unit | none — selection over a given tag slice | `go test ./internal/source/` |
| sync-report: A matched tag that moved onto the same commit still gets a header | `sync-report` test over a built report value, asserting the header is present at all | unit | none — a built report value | `go test ./internal/sync/` |
| update-execution: `--to` can write a range into graft.toml | `update-execution` test asserting the manifest differs in exactly one line | integration | real filesystem, a hand-written `graft.toml`, real `git` | `go test ./internal/sync/` |
| update-execution: `--to` can write a range containing a space | `update-execution` test asserting the value is written literally | integration | real filesystem, a hand-written `graft.toml` | `go test ./internal/manifest/ ./internal/sync/` |

<!-- 127 rows, one per spec scenario -->

## Decisions

**A range is decided by syntax, never by lookup.** The alternative — try it as a ref, fall
back to a range — makes a pin's meaning depend on what the remote contains that day, so a
rev that is a ref today silently becomes a range tomorrow when someone deletes a branch.
The syntactic rule is decidable offline, testable without git, and identical in
`internal/lock` and `internal/source`.

**`>`, `<`, and `=` are claimed as range-leading characters, and `1.x` is not a range.**
`^`, `~`, `*`, and space are already illegal in git ref names, so claiming them costs
nothing. `>`/`<`/`=` are legal but no real tag begins with one, so the cost is theoretical
and the alternative — omitting them — leaves `>=1.2.0` silently looked up as a ref and
reported "not found", naming the wrong problem. `1.x` goes the other way: it is a plausible
branch name, so the ambiguity is resolved toward the existing meaning. A rule with an
ambiguous case is a rule that silently picks wrong, so the rule has none.

**`Masterminds/semver/v3` rather than a hand-rolled parser.** Range grammar and precedence —
prerelease ordering, `^0.x` narrowing to patch, build metadata ignored in comparison — is a
specification with corners, and a subtly wrong implementation picks the wrong tag while
looking like it worked. That is the worst failure this change can have: not a crash, a
silently wrong version. Alternative considered: implementing `^` and `~` only, by hand.
Rejected — the corners are in `^0.x` and prereleases, exactly the cases a hand-rolled
version gets wrong. The dependency is parse-and-compare: no network, no filesystem, no
process.

**Prereleases excluded unless the range names one.** The npm reading. A consumer asking for
`^1.0.0` is asking for a release; adopting an rc because it sorts higher would be the pin
choosing risk on the consumer's behalf. Alternative considered: include them and let the
consumer exclude. Rejected — the safe default belongs on the safe side.

**The range predicate is a leaf package, discovered by trying the obvious thing first.** The
design initially put `IsRange` in `internal/source` and had `internal/lock` call it. That does
not compile — `source -> plan -> lock` is an existing chain, so `lock -> source` closes a
cycle. Alternatives considered: duplicating the predicate in both packages (rejected: this is
precisely the agreement site the design exists to protect, and graft already carries three
copies of `isSHA` for a reason that does not apply here — those three packages genuinely do
not depend on one another, whereas these two must agree on a rule that decides whether a lock
is valid); or moving the tag-listing code into `internal/lock` (rejected: it would put git
execution inside the package that parses a file). A leaf package costs one directory and ends
the question.

**On the sync path the matched tag comes from the previous lock, not from resolution.**
`internal/sync`'s `resolve` deliberately skips `source.Resolve` when the lock already holds a
sha — that skip *is* the difference between sync and update. So `matched` has to travel the
same road the sha does: read from `current.Sources` into the `pinned` map, and only come from
resolution on the branch that actually resolves. Getting this wrong is not a cosmetic bug: a
sync over a range would write a lock with an empty `matched`, which the lock's own validation
then refuses, and every sync of a range repository would fail.

**`matched` in the lock rather than deriving it later.** The alternative is recording only
the sha and letting a reader work out which tag it was. That requires the network, which
makes reading a lock a network operation, and it can be ambiguous — two tags on one commit.
Recording the answer costs one line and keeps the lock diff the review story SPEC.md says
it is.

**`graft.lock`'s `version` does not move; the JSON document's does.** A lock without a range
is byte-identical, and a lock with one could not have come from an older graft, so the lock's
version carries no information a reader needs. The JSON document is different: every source
object changes shape for every consumer, range or not, which is exactly what a version number
is for.

**`matched` is unconditional in JSON and conditional in the lock.** They look inconsistent
and are not. The lock is a human-reviewed diff where an empty line is noise; the document is
a parsed contract where a conditional member is a branch in every consumer. Each optimizes
for its own reader.

## Risks / Trade-offs

- **A hand-rolled range test disagreeing with the library's parser** → `IsRange` decides only
  *whether* a value is a range; the library decides whether it is *valid*. A value `IsRange`
  claims and the library rejects produces the explicit "not a valid semver range" refusal,
  never a fallback. The two never both interpret a string.
- **A tag named `>=1.2.0` becomes unpinnable** → accepted knowingly; documented in
  `rev-ranges`. The consumer can still pin the sha.
- **A source that retags — moving `v1.3.0` to a new commit** → out of scope and unchanged by
  this design: `sync` installs the lock's sha, so a moved tag is only picked up by `update`,
  which is exactly what happens with a ref pin today.
- **A range silently adopting a bad release** → this is what a range *is*, and it is the
  consumer's choice; the mitigation is that `update` is explicit, the report names the tag,
  and `git diff` shows every file.
- **The report gaining a third column breaking a downstream scraper** → the report was never
  a contract (`sync-report` is prose for humans and `--json` exists for programs), and the
  column appears only when a range is used.
- **Two graft versions writing the same lock** → an older graft parsing a lock with `matched`
  rejects it as an unknown key, which is loud rather than silent. Acceptable: the consumer
  who added a range chose the newer graft.

## Migration Plan

None required, and that is a designed property rather than a happy accident: `matched` is
absent for every pin that exists today, so no lock is rewritten, no file moves, and there is
no ordering constraint between updating graft and updating a repository. A consumer upgrades
the binary and nothing changes until they write a range.

Rollback: downgrade the binary. A repository that has not used a range is unaffected. One
that has must change its `rev` back to a concrete tag, which the error message from the older
binary (`rev "^1.2.0" not found`) points at directly.

## Open Questions

None. The three that were open when this change was proposed are decided above: whether the
range test is syntactic (yes), whether `sync` ever re-evaluates (no), and whether the lock
records the matched tag (yes).
