## Context

Three changes have landed and every one of them is pure. `manifest-and-lock` reads
`graft.toml` and writes `graft.lock` deterministically; `catalog-and-selectors` reads
`catalog.yaml` and expands selectors; `destination-and-plan` turns manifest plus lock plus
catalog into a list of file operations. Between them graft can say what was asked for, what
is on offer, and where each file would land.

Nothing has ever contacted a source repository. SPEC.md's resolution sequence numbers eight
steps and this change owns two and a half of them:

> 2. …A source with no lock entry is resolved once — `git ls-remote` for tags and branches,
>    a full SHA passes through — and recorded.
> 3. Fetch that SHA into `~/.cache/graft/<host>/<owner>/<repo>/<sha>/`, a content-addressed
>    cache. An existing entry is reused, so a resolved sync works offline.
> 4. Read `catalog.yaml` from the fetched tree.

Step 5 (selector expansion) is `catalog.Expand`'s and already exists. Steps 6–7 are
`plan.Build`'s and already exist. Step 8 is `internal/apply`'s and arrives with
`sync-command`. What is missing between them is the producer of `plan.Input.Resolved` and
`plan.Input.Items`: `destination-and-plan` defined the shape those values must have and
deferred, in as many words, "whether a `Listing` faithfully describes a real fetched tree"
to this change. That deferred note is discharged here, against real fixture repositories.

Two constraints shape everything below.

**`internal/source` is the only package that runs `git`, and it writes only under the cache
root.** `internal/apply` remains the sole writer of the repository graft runs in. A cache
entry is not the working tree, and the test for that claim is not a comment: every fetch
test names its own root under `t.TempDir()` and asserts nothing appeared outside it.

**A cache hit must run no git command at all.** SPEC.md's failure-mode table says "network
unavailable, cache hit: proceeds". That is not "tolerates a failure" — it is "does not try".
The only implementation that satisfies it is one where the existence of the entry directory
short-circuits before any subprocess is started, which in turn means an entry may never
exist unless it is complete.

## Goals / Non-Goals

**Goals:**

- Expand a source's `git` value to a clone URL as a pure function of the string.
- Resolve a `rev` — tag, branch, or full sha — to the 40-character lowercase hex commit sha
  `graft.lock` records, with a full sha short-circuiting before any subprocess starts.
- Derive a content-addressed cache path from the remote's identity and the sha, purely, and
  with no remote able to aim an entry outside the cache root.
- Fetch a sha into that path atomically: a temporary sibling, published by rename, so the
  entry's existence means the entry is complete.
- Reuse an existing entry with zero git invocations, which is the whole of "works offline".
- Read `catalog.yaml` out of a fetched entry, adding no error wording of graft's own.
- List one item's `from` as the `plan.Listing` `plan.Build` already consumes — sorted,
  slash-separated, symlinks excluded — and prove against a real fixture repository that the
  values flow into `plan.Build` unchanged.
- Pin every error string this package can produce.

**Non-Goals:**

- **No working-tree writes.** This package's only writes are under the cache root it is
  given. `internal/apply` still does not exist and is still the only future writer of the
  consumer's repository.
- **No planning.** Destinations, the prune set, and the next lock belong to `internal/plan`
  and are not touched.
- **No command surface.** No cobra, no `sync`, no progress output, no `--dry-run`, no
  `added`/`updated`/`removed` report.
- **No latest-semver-tag default.** `add` defaults an omitted `rev` to a source's newest
  semver tag. That rule, and the tag enumeration it needs, land with `add-command`.
- **No auth layer.** No credential store, no token flag, no prompt. Private repos work
  exactly as far as the user's existing git credentials already reach.
- **No cache eviction, no `graft cache` command, no size cap, no locking protocol.** An
  entry is immutable and the disk is the user's.
- **No shallow-history or submodule support beyond what one commit's tree needs.** graft
  copies files; it does not reproduce a repository.
- **No change to `graft.toml`, `graft.lock`, or `catalog.yaml`.** `graft.lock` stays at
  `version = 1`.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/source` | **new** | The whole change. Runs `git`, writes cache entries, reads a fetched tree. |
| `internal/plan` | read-only | `source.List` returns `plan.Listing`, the type `destination-and-plan` defined. `plan` gains nothing and loses nothing; it still imports no filesystem package. The edge is one-directional — `plan` never imports `source` — so no cycle exists. |
| `internal/catalog` | read-only | `source.ReadCatalog` reads `catalog.yaml` through the entry's `os.Root` and parses it with `catalog.Parse`, delegating to `catalog.Load` only for the absent case so the not-graftable wording keeps one owner (D12). `source.List` reads `catalog.Item{ID, From}`. No requirement changes. |
| `internal/manifest` | untouched | `Resolve` and `Fetch` take a source's name, `git`, and `rev` as strings rather than a `manifest.Source`, for the same reason `catalog.Expand` takes a source name: a package that talks to git has no business knowing the consumer's file format. |
| `internal/lock` | untouched | The sha this package returns is what a caller puts in `Input.Resolved`; `lock` never sees this package. |
| `internal/itemid` | untouched | Item identity is already settled by both parsers. |
| `internal/apply` | does not exist yet | **This change adds no working-tree write path.** Its only writes are cache entries under a root the caller names, which is not the repository graft runs in, so there is nothing here that could belong in `apply`. |
| `cmd/graft` | untouched | No flag, no command. Everything this change adds is under `./internal/`, where the coverage gate can see it. |

New pieces follow patterns already in the tree:

- **Errors are built in one place per condition and asserted by tests**, as in
  `catalog.errf`, `manifest.validate`'s `fail` closure, and `plan.itemErrf`. `source` gets
  the same shape: a `sourceErrf(name)` closure and an `itemErrf(name, id)` closure.
- **A small path predicate lives with the rule it enforces.** `catalog.inSource` constrains
  `from`, `lock.isRepoRelative` constrains what a lock may authorise deleting, `plan.insideRepo`
  constrains a destination. `source` gets `safeSegment`, constraining what a remote may
  contribute to a cache path, with its own wording.
- **A sha predicate is duplicated rather than shared**, exactly as `lock.isSHA` and
  `plan.isSHA` already are, and produces the identical message. Three copies of six lines is
  the price of three packages that do not depend on each other; a shared one would put the
  definition of a valid `resolved` somewhere none of them owns.
- **Deterministic walks and sorted output**, as in `catalog.parseItems` and `plan.Build`, so
  a listing is byte-stable across platforms and across two runs.

## Contracts

`internal/source` is a new internal package; nothing outside this module can depend on it,
so nothing here is breaking. The API `sync-command`, `update-command`, and `add-command`
will all code against:

```go
// CloneURL expands a source's `git` value into something `git` accepts, refusing a
// value that begins with "-" (D2). Pure: it contacts nothing and creates nothing.
func CloneURL(name, git string) (string, error)

// Resolve turns rev — a tag, a branch, or a full sha — into the 40-character lowercase
// hex commit sha graft.lock records as `resolved`. name locates errors.
func Resolve(name, git, rev string) (string, error)

// DefaultCacheRoot is $XDG_CACHE_HOME/graft, or <home>/.cache/graft. It creates nothing.
func DefaultCacheRoot() (string, error)

// Cache is a content-addressed store of fetched source trees.
type Cache struct{ Root string }

// Entry is the path a source's tree at sha occupies. Pure: it creates nothing and
// contacts nothing, so a caller may ask where an entry would be without making one.
func (c Cache) Entry(name, git, sha string) (string, error)

// Fetch returns the path to the tree at sha, fetching it if the cache lacks it. On a
// hit it runs no git command.
func (c Cache) Fetch(name, git, sha string) (string, error)

// ReadCatalog reads catalog.yaml from the root of a fetched entry.
func ReadCatalog(entry string) (*catalog.Catalog, error)

// List turns one item's From, resolved inside entry, into the plan.Listing that
// plan.Build consumes. name locates errors.
func List(entry, name string, it catalog.Item) (plan.Listing, error)
```

**`List` returns `plan.Listing` itself, not a convertible twin.** `source-listing`'s last
requirement — "usable as `plan.Input.Items` with no adaptation, same type" — is enforced by
the type system rather than by a test, and the integration scenario then proves the values
are right as well as the types.

**Preconditions.** `Entry` and `Fetch` validate `sha` themselves and are total over any
string. `Resolve` is total over any `rev`. `ReadCatalog` and `List` assume `entry` names a
directory this package produced; a caller passing something else gets the ordinary
not-found error, which is correct and needs no separate wording. `List` assumes
`catalog.Parse` already refused a `from` that is absolute, uncleaned, `.`, or carrying a
`..` segment — that is `catalog.inSource`, and re-checking it here would put the same rule
in a second package. The defence this package *does* own is the one `catalog` cannot give:
`from` is checked against what is actually on disk, and a symlink is refused.

**Error surface** — every message is asserted by a test and is a deliberate contract:

| Condition | Message |
|---|---|
| `git` absent from `PATH` | `git not found on PATH` |
| `git` value begins with `-` | `source "shared": git "--upload-pack=./pwn.sh" may not begin with "-"` |
| `rev` is empty | `source "shared": rev is empty` |
| No queried ref matches | `source "shared": rev "v9.9.9" not found` |
| Remote unreachable while resolving | `source "shared": cannot reach "/nope": ` + git's first stderr line, e.g. `fatal: '/nope' does not appear to be a git repository` |
| `sha` is not 40 lowercase hex | `source "shared": resolved "not-a-sha" is not a 40-character hex sha` |
| Fetch fails — unreachable, or sha absent | `source "shared": cannot fetch "0000…0000" from "/nope": ` + git's first stderr line, e.g. `fatal: git upload-pack: not our ref 0000…0000` |
| Cache root unusable | `source "shared": cannot create cache entry for "fae2a30…": ` + the OS error |
| `catalog.yaml` absent | *(unchanged, `catalog.Load`'s `catalog.yaml not found: the source is not graftable`)* |
| `catalog.yaml` unreadable for any other reason | `catalog.yaml: <err>` — `catalog.Load`'s own format for the same case, not a second wording (D12) |
| `from` names nothing | `source "shared": item "schema:tdd": from "extras/gone" not found in the source tree` |
| `from` is a symlink, socket, or device, or leaves the entry | `source "shared": item "schema:tdd": from "extras/tdd" is not a regular file or directory` |
| Home directory undeterminable | `cannot determine the cache root: <reason>` |

**Only the graft-owned prefix of a git-derived message is asserted.** The two rows above
that embed git's output carry **git's first stderr line only**, trimmed — everything after
it is advice for a human at a terminal ("Please make sure you have the correct access
rights…"), and a multi-line error inside a per-source failure buries the identifying line.
The examples are illustrative rather than pinned: over the local transport two processes
write the same pipe, so which line arrives first is not deterministic, and a test pinning
git's exact text would flake. Tests assert the graft-owned prefix exactly, assert the
message contains no newline, and assert nothing about git's own wording.

No pagination, no streaming, no compatibility surface. Every function is additive.

## Persistence and Rollout

- **Migration**: none. No file format changes.
- **Backfill**: none.
- **Seeding**: none.
- **Cache invalidation**: **none, by construction, and this is the design's load-bearing
  claim.** A cache entry is keyed by a commit sha, and a commit's tree is immutable, so an
  entry can never be stale and nothing ever needs to invalidate one. The corollary is the
  atomic-publish rule: since nothing will ever re-fetch an entry that exists, an incomplete
  entry would be wrong forever. That is why a fetch builds in a sibling directory and
  publishes by rename rather than fetching into the entry's own path. There is no eviction
  and no size cap; a stale-but-correct entry costs disk, and disk is the user's.
- **Index rebuild**: none.
- **Authorization**: none of graft's own. Access to a private source is whatever the user's
  existing git credentials already grant; graft adds no credential handling and, by
  disabling terminal prompting, converts what would be a hang into the unreachable-remote
  error.
- **Observability**: none. No logging, no metrics, no telemetry — SPEC.md forbids telemetry
  outright, and output belongs to `command-surface`.
- **Deployment**: none. No release is cut for this change; it ships when a command uses it.

**Effect on `graft.lock`'s format: none.** `version` stays `1`. This change produces the
`resolved` sha the existing format already carries and adds no key.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem (consumer's working tree) | n/a — no acceptance test in this change; see Test Strategy | **not used at all.** No test in this change creates, modifies, or reads anything in the repository graft runs in. The fetch tests assert it positively: a fixture consumer tree is built under `t.TempDir()` and compared file-for-file before and after. |
| Filesystem (cache root) | n/a | **real**, always under `t.TempDir()`. `Cache.Root` is a field, never a global, so no test can reach the developer's `~/.cache/graft`. |
| Filesystem (fetched entry / source tree) | n/a | **real**, under `t.TempDir()`, and reached only through `os.Root` (D10). `List` and `ReadCatalog` walk a real directory — that is the whole point of the change, and a listing over a fake filesystem would prove nothing about the deferred note it discharges, nor about a symlink escaping it. The escape tests plant real symlinks pointing at real files outside the entry and assert those files' contents appear nowhere. |
| `git` binary | n/a | **real.** ENGINEERING.md already makes `git` on `PATH` the one runtime dependency, and CI runs on `ubuntu-latest` and `macos-latest`, both of which have it. A fake git would test the fake. `PATH` is narrowed with `t.Setenv` to an empty directory for the one scenario that needs `git` absent. |
| Network | n/a | **not used.** Every remote is a local filesystem path under `t.TempDir()`, which exercises the same `git` code paths — `ls-remote`, `fetch`, `upload-pack` — without a socket. No test in this change reaches the internet, so CI is offline-safe and cannot flake on GitHub. |
| Fetch cache (`~/.cache/graft/`) | n/a | **never touched.** `DefaultCacheRoot` is tested through an unexported seam taking `getenv` and `home` functions (D7), so even the default-root scenarios read no real environment and create no directory. |
| Fixture git repositories | n/a | **real**, built in `t.TempDir()` by a helper that runs `git init`, then `git config user.name` and `git config user.email` **on the repository** — not `--global` — or commits fail on a clean CI runner. Also sets `-c init.defaultBranch=main`, so a fixture's branch name does not depend on the runner's git configuration. |
| `internal/catalog` | n/a | **real.** `ReadCatalog` calls the real `catalog.Load` against a real file; the not-graftable message is asserted as `catalog`'s own. |
| `internal/plan` | n/a | **real.** The integration scenario calls the real `plan.Build` with values this package produced. |
| `internal/manifest`, `internal/lock` | n/a | **real, as values only.** Neither is imported by the package's production code — `Resolve` and `Fetch` take strings, not a `manifest.Source`. Group 12's integration test does import both, because `plan.Input.Source` *is* a `manifest.Source` and because the byte-equality assertion calls the real `lock.Marshal`. Neither file is read from or written to disk in any test. |
| Environment (`XDG_CACHE_HOME`, `HOME`) | n/a | **replaced** by the `getenv`/`home` seam in the default-root tests. `PATH` is the exception and is real, narrowed with `t.Setenv`. |
| Clock, randomness | n/a | **not used** for behavior. The temporary directory's name uses `os.MkdirTemp`'s randomness, which no assertion depends on: tests assert that no temporary directory *remains*, never what one was called. |

## Test Strategy

Two tiers, both under `./internal/source/` and both run by `go test ./internal/source/...`,
gated by `task cover` at 80% over `./internal/...`:

- **unit** — pure functions over values: `CloneURL`, `Cache.Entry`, the default-root seam,
  and the sha check. No directory is created.
- **integration** — a real `git` against a real fixture repository in `t.TempDir()`. Every
  scenario touching `ls-remote`, `fetch`, the cache, or a directory walk is here. These are
  ordinary `go test` runs with no build tag: they need no network and no service, so
  splitting them behind a tag would only mean the coverage gate stopped seeing them.

**This change takes no outer-loop acceptance group, and it is deleted from tasks.md.** The
reason is the same one `destination-and-plan` recorded and it has not changed: no command
exists. `command-surface` has not landed, `graft sync` does not exist, and `internal/apply`
— the only thing that could make any of this observable in a working tree — arrives with
`sync-command`. An end-to-end group here could only drive an invented harness, a boundary
the table above does not name. The nearest equivalent is deliberately placed instead: task
group 9 is one integration test that runs the real chain — fixture repo → `Resolve` →
`Fetch` → `ReadCatalog` → `catalog.Expand` → `List` → `plan.Build` — and it is scheduled
last, because it is the only test that can fail for a reason none of the others can.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| Shorthand expands to HTTPS | `TestCloneURL` table case | unit | none | `go test ./internal/source/ -run CloneURL` |
| A URL carrying a scheme is passed through | `TestCloneURL` table case | unit | none | `go test ./internal/source/ -run CloneURL` |
| An scp-style address is passed through | `TestCloneURL` table case | unit | none | `go test ./internal/source/ -run CloneURL` |
| A filesystem path is passed through | `TestCloneURL` table case (both `/srv/...` and `../sibling-repo`) | unit | none | `go test ./internal/source/ -run CloneURL` |
| A branch resolves to its tip | `TestResolveBranch` | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| A lightweight tag resolves to its commit | `TestResolveLightweightTag` | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| An annotated tag resolves to the commit, not the tag object | `TestResolveAnnotatedTag`, asserting the returned sha equals the commit and differs from the tag object | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| A tag wins over a branch of the same name | `TestResolvePrecedence` | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| A full sha passes through without contacting the remote | `TestResolveFullSHAOffline`, whose clone URL names a path that does not exist | integration | real `git` (never invoked) | `go test ./internal/source/ -run Resolve` |
| An uppercase sha is not treated as a sha | `TestResolveUppercaseSHA`, asserting the not-found error | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| A rev no ref matches | `TestResolveErrors` case, exact message | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| An abbreviated sha is not a rev | `TestResolveErrors` case, exact message | integration | real `git`, fixture repo | `go test ./internal/source/ -run Resolve` |
| An unreachable remote | `TestResolveErrors` case, asserting the prefix and a single-line message | integration | real `git`, no repo | `go test ./internal/source/ -run Resolve` |
| An empty rev | `TestResolveErrors` case, exact message, with `PATH` emptied to prove no git ran | unit | none | `go test ./internal/source/ -run Resolve` |
| git is not on PATH | `TestGitNotOnPATH`, `t.Setenv("PATH", t.TempDir())` | unit | `PATH` real, narrowed | `go test ./internal/source/ -run PATH` |
| A remote that looks like an option is refused | `TestCloneURLRefusesOption` plus `TestResolveErrors` and `TestFetchErrors` cases, all with `PATH` emptied so no git can run | unit | none | `go test ./internal/source/ -run 'CloneURL\|Resolve\|Fetch'` |
| The cache path mirrors the remote and the sha | `TestEntryPath` table case, plus an assertion that the root was not created | unit | none | `go test ./internal/source/ -run Entry` |
| The same repository over ssh and over HTTPS is one entry | `TestEntryPath` table case comparing two derivations | unit | none | `go test ./internal/source/ -run Entry` |
| A filesystem remote gets an entry under `local` | `TestEntryPath` table case | unit | none | `go test ./internal/source/ -run Entry` |
| A hostile remote cannot escape the cache root | `TestEntryPathCannotEscape`, asserting `filepath.Rel(root, entry)` has no `..` for a table of hostile URLs | unit | none | `go test ./internal/source/ -run Entry` |
| A sha that is not a sha is refused | `TestEntryPath` error case, exact message, plus `Fetch` asserting the root is unchanged | unit + integration | filesystem real for the `Fetch` half | `go test ./internal/source/ -run 'Entry\|Fetch'` |
| The default root under a home directory | `TestDefaultCacheRoot` case through the `getenv`/`home` seam, asserting nothing was created | unit | none | `go test ./internal/source/ -run DefaultCacheRoot` |
| `XDG_CACHE_HOME` moves the default root | `TestDefaultCacheRoot` case | unit | none | `go test ./internal/source/ -run DefaultCacheRoot` |
| A relative `XDG_CACHE_HOME` is ignored | `TestDefaultCacheRoot` case | unit | none | `go test ./internal/source/ -run DefaultCacheRoot` |
| No home directory and no `XDG_CACHE_HOME` is an error | `TestDefaultCacheRoot` case, the `home` stub returning an error | unit | none | `go test ./internal/source/ -run DefaultCacheRoot` |
| A first fetch writes the tree | `TestFetchWritesTree`, comparing contents and asserting no `.git` | integration | real `git`, fixture repo, real cache root | `go test ./internal/source/ -run Fetch` |
| A fetch of an older commit gets that commit's tree | `TestFetchOlderCommit` | integration | real `git`, fixture repo | `go test ./internal/source/ -run Fetch` |
| A source's `.gitattributes` does not alter the cached bytes | `TestFetchIgnoresGitattributes`, comparing the entry against `git cat-file blob` | integration | real `git`, fixture repo | `go test ./internal/source/ -run Fetch` |
| A fetch writes nothing outside the cache root | `TestFetchWritesOnlyUnderRoot`, snapshotting a fixture consumer tree before and after | integration | real `git`, fixture repo, two temp dirs | `go test ./internal/source/ -run Fetch` |
| A second fetch of the same sha works with the remote gone | `TestFetchCacheHitOffline`: fetch, `os.RemoveAll` the source repo **and** empty `PATH`, fetch again | integration | real `git` (unavailable on the second call) | `go test ./internal/source/ -run Fetch` |
| A cache miss with no reachable remote is an error naming both | `TestFetchErrors` case, asserting the prefix and a single-line message | integration | real `git`, no repo | `go test ./internal/source/ -run Fetch` |
| A sha that the remote does not have is the same error | `TestFetchErrors` case, plus an assertion that the entry path does not exist | integration | real `git`, fixture repo | `go test ./internal/source/ -run Fetch` |
| A failed fetch leaves the cache as it found it | `TestFetchLeavesNoPartialEntry`, asserting the entry is absent and the entry's parent holds no leftover directory | integration | real `git`, fixture repo | `go test ./internal/source/ -run Fetch` |
| A fetch into an unusable cache root fails without a partial entry | `TestFetchUnusableRoot`, with the root a regular file, asserting the message and the file's bytes | integration | filesystem real | `go test ./internal/source/ -run Fetch` |
| A catalog in the fetched tree parses | `TestReadCatalog` | integration | real `catalog`, real filesystem | `go test ./internal/source/ -run ReadCatalog` |
| A source with no catalog is not graftable | `TestReadCatalogMissing`, exact message, asserted as `catalog`'s own | integration | real `catalog` | `go test ./internal/source/ -run ReadCatalog` |
| A `catalog.yaml` leaving the entry is not read | `TestReadCatalogSymlinkEscape`, with the link pointing at a real file outside the entry | integration | real filesystem | `go test ./internal/source/ -run ReadCatalog` |
| A `from` naming a file lists exactly that file | `TestListFile` | integration | real filesystem | `go test ./internal/source/ -run List` |
| A `from` naming a directory lists its tree | `TestListDirectory`, asserting exact slice equality including order | integration | real filesystem | `go test ./internal/source/ -run List` |
| An empty directory lists nothing and is still a directory | `TestListEmptyDirectory` | integration | real filesystem | `go test ./internal/source/ -run List` |
| A directory holding only empty subdirectories lists nothing | `TestListEmptyDirectory` sibling case | integration | real filesystem | `go test ./internal/source/ -run List` |
| A symlink is not listed | `TestListSkipsSymlink`, with the link pointing outside the tree | integration | real filesystem | `go test ./internal/source/ -run List` |
| A `from` that does not exist is an error naming the item | `TestListErrors` case, exact message | integration | real filesystem | `go test ./internal/source/ -run List` |
| A `from` naming a symlink is refused | `TestListErrors` case, exact message, asserting the link target is never read | integration | real filesystem | `go test ./internal/source/ -run List` |
| A `from` reached through a symlinked parent is refused | `TestListSymlinkedParent`, asserting the outside file's name and contents appear nowhere | integration | real filesystem | `go test ./internal/source/ -run List` |
| A `from` naming a submodule lists nothing | `TestListSubmodule`, against a real fixture with a committed gitlink | integration | real `git`, two fixture repos | `go test ./internal/source/ -run List` |
| Listing changes nothing | `TestListChangesNothing`, snapshotting the entry's paths, sizes, and contents before and after every listing above | integration | real filesystem | `go test ./internal/source/ -run List` |
| A fetched fixture plans the writes its tree implies | `TestFetchedSourcePlansItsTree` — the full chain into the real `plan.Build`, asserting each `Write.From` names a file that exists in the entry | integration | real `git`, fixture repo, real `catalog`, real `plan` | `go test ./internal/source/ -run Plans` |

## Decisions

**D1 — `source` imports `plan`, not the reverse.** `source.List` returns `plan.Listing`
itself. The alternative, a `source.Listing` converted at the call site, was rejected: the
spec requirement is "usable with no adaptation, same type", and a conversion function is
exactly the adaptation it forbids — plus a place for the two shapes to drift. A third
package holding the shared type was rejected as a package existing only to hold one struct.
The edge is safe because it is one-directional and because `plan` is the pure package: it
depends on nothing that could drag a filesystem call back into it.

**D2 — `git` is invoked through `os/exec` with an explicit argv, never a shell, and a `git`
value beginning with `-` is refused before any of that.** No shell means no quoting rules
and no `sh -c`. That is necessary and it is not sufficient: an explicit argv still lets a
value *become* a flag, because argv position is not what git uses to tell an option from an
operand. `git ls-remote --upload-pack=./pwn.sh refs/tags/v1` parses the first word as an
option, promotes the refspec to the repository operand, and runs the script — verified
against git 2.48.1. A `graft.toml` in a pull request plus a committed script would then own
any machine that syncs it, which is an execution path the whole design claims not to have.

Two independent guards, because either alone is one behavior change away from failing:

- `CloneURL` refuses a value beginning with `-` outright, with its own message. This is the
  guard that does not depend on git.
- Every invocation separates options from operands with `--`:
  `git ls-remote -- <url> <refs…>`. Verified: the same attack then fails with
  `fatal: strange pathname '--upload-pack=./pwn.sh' blocked` and nothing runs, while an
  ordinary URL is unaffected.

`git remote add` already rejects an option-shaped URL on its own, so the fetch path was
never exposed; it takes the same refusal anyway rather than relying on that.

**D3 — Terminal prompting is disabled by setting `GIT_TERMINAL_PROMPT=0` in the child's
environment, and nothing else is scrubbed.** The failure this prevents is a hang: a private
source with no usable credentials would otherwise block on a password prompt forever inside
what a caller believes is a function call. Everything else in the environment is left
alone deliberately — `GIT_ASKPASS`, `SSH_AUTH_SOCK`, and the user's `credential.helper` are
how a private source works at all, and SPEC.md's "private repos work exactly as far as the
user's existing git credentials reach" is a promise not to interfere. Rejected: `ssh
-o BatchMode=yes`, which would also suppress the host-key confirmation on a first
connection and turn a one-time prompt into a permanent failure.

**D4 — Rev resolution queries three explicit refs in one `ls-remote` and matches ref names
exactly in the parse.** The invocation is
`git ls-remote <url> refs/tags/<rev> refs/tags/<rev>^{} refs/heads/<rev>`. The patterns are
passed to keep the response small, but they are **not** trusted to be exact:
`git ls-remote` matches a pattern against the tail of a ref name, so `refs/tags/v1` would
also match a ref named `refs/heads/x/refs/tags/v1`. The parse therefore compares the ref
column to the three expected names with `==`. This is what `rev-resolution` means by
"explicitly rather than by pattern, so a rev can never match a ref whose name merely ends
with it". Precedence on multiple matches is peeled tag → tag → branch, which is git's own
`rev-parse` order and the reading under which a pin names the immutable thing.

An empty result set is exit status 0 with no output, which is the not-found error. A
non-zero exit is the unreachable error. The two are distinguishable because they are
different signals, not because of anything in the message — which matters, since one is a
typo in `graft.toml` and the other is a network or permission problem.

**D5 — A 40-lowercase-hex `rev` returns before `exec.LookPath` is called.** Not merely
before the fetch: before anything that could fail for an environmental reason. A sha is
already the answer, so a round trip could only turn a working resolution into a broken one.
Uppercase is deliberately *not* a sha: `graft.lock` records lowercase, and silently
lowercasing would make `graft.toml` and `graft.lock` disagree about what was asked for, so
an uppercase rev falls through to ref lookup and fails as not found — which is what it is.

**D6 — A cache entry's path is `<root>/<host>/<owner>/<repo>/<sha>/`, derived from the
URL's identity.** Identity is host plus path; scheme, user, port, a trailing `/`, and a trailing `.git` are
stripped **in that order** — trimming the slash first, or `…/b.git/` and `…/b.git` become
two entries for one repository — so the same repository over HTTPS and over ssh is fetched
once. Parsing covers
three forms — a URL with a scheme, an scp-style `user@host:path`, and a bare filesystem
path — because those are the three forms `CloneURL` can produce. A URL with no host derives
under the literal segment `local`, so a fixture or a local mirror is not a special case with
no entry.

Every derived segment passes through `safeSegment`: anything outside `[A-Za-z0-9._-]`
becomes `-`, and a segment of `.` or `..` is prefixed with `_`. The rule is applied per
segment after splitting, so a segment can neither contain a separator nor climb. Empty
segments are dropped. This is the containment guarantee: **no remote can aim a cache entry
outside the cache root**, and it is asserted with `filepath.Rel` rather than by string
inspection, because `filepath.Rel` is what actually answers the question.

**D7 — `DefaultCacheRoot` reads the environment; `defaultCacheRoot(getenv, home)` does the
work.** The exported function calls the unexported one with `os.Getenv` and
`os.UserHomeDir`. That seam is why the three default-root scenarios are unit tests that
create nothing and read nothing: `t.Setenv` would work, but it also forbids `t.Parallel`,
and — more importantly — a test that has to set `HOME` is one edit away from a test that
writes to the developer's real cache. `XDG_CACHE_HOME` is honoured only when absolute: a
relative one would give the same source a different entry per working directory, which is
the one thing a content-addressed cache may not do.

**D8 — A fetch builds in a sibling temporary directory and publishes by `os.Rename`.**
The sequence, with `tmp` created by `os.MkdirTemp` in the entry's **parent** so the rename
is within one filesystem:

```
os.Mkdir(tmp/tree)                                  // checkout fails without it
git init -q --bare        tmp/git
git --git-dir=tmp/git     remote add origin -- <url>
git --git-dir=tmp/git     fetch --depth 1 --no-tags -q origin <sha>
git --git-dir=tmp/git \
    -c attr.tree=4b825dc642cb6eb9a060e54bf8d69288fbee4904 \
    -c core.bare=false --work-tree=tmp/tree checkout -q --detach FETCH_HEAD
os.RemoveAll(tmp/git)
os.Rename(tmp/tree, entry)
```

The bare git directory sits **beside** the work tree rather than inside it, so no `.git`
ever exists within the tree that is published — which is a stronger guarantee than deleting
one afterwards, where an interrupted run could leave the repository behind. `--depth 1`
fetches one commit's history and `--no-tags` skips tag objects the tree does not need.
`os.Mkdir(tmp/tree)` is not incidental: without it `checkout --work-tree` fails with
`fatal: this operation must be run in a work tree`.

`attr.tree` set to the **empty tree** is what makes the entry hold the bytes the commit
recorded. Without it `git checkout` honours the source's own committed `.gitattributes`,
which is a file the source controls: `* text eol=crlf` rewrites every line ending (verified:
the blob `hello\nworld\n` lands as `hello\r\nworld\r\n`), `ident` expands `$Id$` into a real
hash, and — the one that matters — `filter=lfs` selects a filter driver whose *command* is
read from the consumer's own git config, so a source-controlled file causes a program to
run on any machine with git-lfs installed. That is precisely the execution path
ENGINEERING.md says a source does not have. `-c core.autocrlf=false -c core.eol=lf` does not
close it, because it is the in-tree attributes rather than the config that select the
filter; pointing `attr.tree` at the empty tree disables in-tree attributes wholesale and was
verified to restore both the LF bytes and the literal `$Id$`.

`defer os.RemoveAll(tmp)` runs on every path including success, and is the whole of "leaves
no temporary directory behind". Because the publish is a rename out of `tmp`, the deferred
cleanup on the success path removes only the now-empty scaffold.

If the rename fails because the destination already exists, the fetch **re-checks the entry
and treats it as a hit**. Two graft runs racing on one sha both want the same immutable
tree, and turning that into an error would make concurrent syncs fail for no reason. Note
that `os.Rename` on Unix silently replaces an existing *empty* directory and fails with
`ENOTEMPTY` on a non-empty one — either outcome leaves a complete tree at the entry, so the
recheck is a `Stat`, not a retry.

**D9 — Fetch-by-sha rather than fetch-a-ref-then-checkout.** `git fetch origin <sha>` is
supported by GitHub, GitLab, and local transports, is exactly one round trip, and needs no
knowledge of which ref contained the commit — which graft does not have, since the ref was
consumed by `Resolve` and only the sha survives into `graft.lock`. Rejected: fetching all
branches and checking out the sha, which downloads history graft will never read and would
still fail for a commit reachable only from a tag.

**D10 — Every read below an entry goes through `os.Root`, not through a joined path.** This
is the decision that changed after review, and the reason is worth stating rather than
burying: refusing a symlink at the *last* component of `from` closes nothing. `os.Lstat`
does not follow the final element but does resolve every intermediate one, so a source that
commits `extras` as a symlink to `../..` and declares `from: extras/secrets` reads a
directory outside the entry entirely — verified — while `catalog.inSource` sees a relative,
cleaned path with no `..` segment and correctly finds nothing wrong with the *string*. The
same hole reaches `catalog.yaml`: a source may commit it as a symlink to `/etc/hosts`, and
an ordinary `os.ReadFile` follows it.

`os.Root`, which Go 1.27 gives for free, is the whole answer: it refuses any name whose
components leave the root, absolute symlink targets included. The package therefore holds
one `*os.Root` per entry and does every read through it:

- `ReadCatalog` uses `root.ReadFile("catalog.yaml")`.
- `List` uses `root.Lstat(from)` for the file-or-directory question, then
  `root.OpenRoot(from)` and `fs.WalkDir(fromRoot.FS(), ".")` for a directory.

Nesting the second root at `from` is what enforces the *item* half of SPEC.md's invariant
rather than only the entry half: the walk cannot leave the item's own subtree, and
`fs.WalkDir` never follows a symlink, so a link below `from` is skipped rather than
traversed. A symlink that resolves *inside* the entry is deliberately allowed through
`Lstat`'s intermediate components — that is the source's own tree, which the source already
controls in full, and refusing it would break a source that merely organises itself with
links.

Within the walk, only `d.Type().IsRegular()` is admitted. A symlink, socket, device, or fifo
is skipped **silently** rather than refused, so one stray link cannot make an otherwise valid
source unusable. `from` itself is different and is refused rather than skipped: a `from` that
is a symlink is a claim about where an entire item lives, and silently listing zero files for
it would install nothing while reporting success. Paths are made relative with `filepath.Rel`,
converted with `filepath.ToSlash`, and sorted, so a listing is byte-stable across platforms —
which is what keeps `graft.lock`'s `files` from churning.

**D11 — The file/directory distinction comes from `Root.Lstat` on `from`, and `Listing.Dir`
carries it.** `plan` cannot infer it: a directory holding one identically named file is
indistinguishable from a file by the listing alone, and the two place their contents
differently. `Lstat` rather than `Stat` is the point — `Stat` would follow a symlink and
answer about its target, which is exactly the `from`-is-a-symlink case that must be refused.

**D12 — `catalog`'s error wording keeps one owner, without letting `catalog.Load` follow a
link.** `ReadCatalog` reads the bytes through the entry's root and hands them to
`catalog.Parse`. On `fs.ErrNotExist` — and only then, when there is by definition no link to
follow — it delegates to `catalog.Load` on the joined path, which returns
`catalog.yaml not found: the source is not graftable` verbatim. Any other read failure is
reported as `catalog.yaml: <err>`, which is not a new wording either: it is the format
`catalog.Load` itself uses for a read that fails for a reason other than absence. Rejected:
exporting a sentinel from `internal/catalog`, which would mean modifying an archived
change's capability to serve a caller that can already produce the exact string.

**D13 — Every git invocation captures stderr and reports its first line.** `exec.Cmd` with
`Stderr` set to a `bytes.Buffer`, and a helper reducing the buffer to its first non-empty
trimmed line. Git's later lines are terminal advice; embedding them inside a per-source
error would bury the identifying line. Tests assert both the exact prefix and the absence
of a newline.

## Risks / Trade-offs

**[A server rejecting fetch-by-sha]** → `uploadpack.allowReachableSHA1InWant` is off by
default in vanilla git, so a self-hosted remote could refuse `git fetch origin <sha>`. Local
transport, GitHub, and GitLab all accept it, which covers every source graft is built for
and every fixture in this change. The failure is not silent — it surfaces as the
`cannot fetch` error naming the sha and the URL — and the fallback, fetching every branch
and tag and then checking the sha out, is a change with its own trade-offs (much more data,
and still no guarantee for a commit no ref reaches). Deferred until a real source needs it,
recorded here as the resolution point.

**[Cache growth is unbounded]** → An entry is never evicted, so a repository synced across
many pins accumulates one tree per sha. Each is a single commit's worth of files, so the
cost is small, and eviction requires a policy question — how old is stale, which run is
allowed to delete another's entry — that this change has no reason to answer. `rm -rf
~/.cache/graft` is a complete solution today. Resolution point: whichever change first has a
user with a real cache-size complaint.

**[Concurrent syncs on one entry]** → Two processes fetching the same sha both build in
their own temporary directory and race on the rename. Both outcomes are correct because the
tree is immutable and the loser treats the failure as a hit (D8). No lock file is
introduced: a lock is a new failure mode — a stale one blocks every future run — bought to
prevent an outcome that is already correct.

**[`git` behavior varying across versions]** → The invocations are old and stable
(`ls-remote` with refspecs, `init --bare`, `fetch --depth`, `checkout --detach`), and CI
runs on two runner images. A version too old for `--depth` over the local transport would
fail loudly in CI rather than silently in a user's tree.

**[Tests depend on a real `git`]** → They do, and deliberately: a fake git would test the
fake, and the deferred note this change discharges is specifically about fidelity to a real
tree. ENGINEERING.md already makes `git` the one runtime dependency, both CI runners ship
it, and no test needs the network.

## Migration Plan

None. No format changes, no stored state to migrate, no deploy order. The package is new
and unreferenced until `sync-command` calls it; nothing observable changes for a user of
this repository when it lands.

## Open Questions

**Q1 — Should a cache entry record which URL produced it?** Two distinct repositories can
sanitize to one path — `example.com/a-b` and `example.com/a/b` both reduce toward the same
segments under an aggressive rule — and an entry carrying its origin URL would let a
mismatch be detected rather than silently reused. Left out: the sanitizer only replaces
characters within a segment and never joins or splits segments, so the collision needs two
genuinely different remotes whose host and path differ only in characters outside
`[A-Za-z0-9._-]`, and the two would still have to share a commit sha to be confused. A
sidecar file also stops the entry from being purely a tree. Resolution point: whichever
change first has cause to add metadata to an entry.

**Q2 — Should `Fetch` verify that a cache hit's tree is intact?** Today the entry's
existence is trusted, which is what makes a hit cost zero git invocations. A user who
deletes half an entry by hand gets a wrong sync with no error. Not addressed: verification
means either a manifest of the entry's files, which is state the atomic publish exists to
avoid needing, or re-running git on every hit, which forfeits the offline guarantee that is
the point of the cache. Resolution point: `sync-command`, if its integration tests find a
real way to produce a half-entry that is not a hand edit.

**Q3 — Should `Resolve` accept an abbreviated sha?** It does not: SPEC.md admits a tag, a
branch, or a full sha, and an abbreviation is not stable enough to pin. Recorded because the
error a user sees is `rev "47f73fc" not found`, which describes the outcome without
explaining the rule. Improving that message is `command-surface`'s business, since it owns
the error format. Not blocking.
