## Context

`graft sync`, `graft update`, and `graft list` all assume a `graft.toml` that already exists.
Writing that first block is manual work today, and it is the step where a consumer has the
least information: the source name, the current tag, and the selectors the catalog actually
offers are all things the tool knows and the user is guessing at.

Two constraints shape everything below. `graft.toml` is a human's file — the pin move added
by `update-command` edits it as *text*, refusing any shape it cannot rewrite exactly, because
re-serializing a parsed manifest deletes comments and alignment and turns a one-line change
into a whole-file diff. And `internal/apply` is the only package that writes to the working
tree, which is what makes "nothing touches the tree until every check passes" testable.
`add` does strictly more surgery than `update --to` did, and it must do it inside both rules.

## Goals / Non-Goals

**Goals:**
- `graft add <source>[@rev] [selector...]`, `--list`, and `--no-sync`, exactly as SPEC.md's
  `## graft add` section describes them, minus the picker.
- Append a `[sources.<name>]` table and amend an `install` list without disturbing a byte the
  edit does not have to touch, or refusing when that is not possible.
- A default pin resolved from the source itself, so the common invocation names no rev.

**Non-Goals:**
- The interactive picker, the `kind:*` collapse offer, and the no-TTY narrowing of the
  no-selector refusal. All three are `add-picker`.
- `--dry-run` on `add`. `--list` is the read-only form and `--no-sync` is the write-only one;
  a third mode would be a third thing to test for no behavior SPEC.md names.
- `--as <name>`, and any source removal.

## Boundaries

| Piece | Package | Pattern it follows |
|---|---|---|
| Sequence for `add` | `internal/add` (new) | `internal/sync` — owns an order, decides nothing another package decides |
| Command wiring | `internal/cli/add.go` (new) | `internal/cli/update.go` — argument validation and `usagef`, nothing else |
| Table append, install amend | `internal/manifest` | `SetRev` — a line scanner that refuses every shape it cannot rewrite exactly |
| Default rev | `internal/source` | `resolveRange` — one `gitOutput` call, `CloneURL` first, this package's error wording |
| Catalog listing for `--list` | `internal/source` + `internal/plan` | `ReadCatalog` plus `plan`'s destination computation, exported as `ItemDestinations(Input, Item)` — pure, told whether the item is a directory by the listing the caller already has |
| Manifest-only write | `internal/apply` | The existing `WithManifest` staging-and-rename path, reused rather than re-implemented |
| Pre-edited manifest into a sync | `internal/sync` | `movePin` — the run resolves the bytes it will write |

`internal/plan` gains no filesystem access: `--list` hands it a catalog and an optional
override map and gets destinations back, exactly as `plan.Build` does today.

`internal/add` is a sequence package, not a second writer. It produces manifest bytes and
hands them to `internal/apply` — through `sync.Run` when it syncs, through the new
manifest-only entry point when `--no-sync` says not to.

## Contracts

The three files' formats are unchanged. `graft.lock`'s `version` stays `1` and the
`graft list --json` document's stays `2`: no key is added, removed, or re-ordered anywhere.

New internal surfaces, all additive:

- `manifest.AddSource(data []byte, name, git, rev string, install []string) ([]byte, error)` and
  `manifest.AddInstall(data []byte, name string, selectors []string) ([]byte, error)`.
  Both return bytes or an error and never both, exactly as `SetRev` does.
- `source.DefaultRev(name, git string) (string, error)`.
- `apply.Manifest(root string, data []byte) error`.
- `sync.Options.Manifest []byte` — when non-nil, the run honours and writes these bytes
  rather than reading `graft.toml` from disk.

The one externally visible addition is `graft add` itself and its four usage errors. No
existing command's output, exit code, or error text moves.

## Persistence and Rollout

- Migration: none. No format changes.
- Backfill: none.
- Seeding: none — though `add` may now *create* `graft.toml`, which is the one new file any
  command brings into existence.
- Cache invalidation: none. `add` fetches into the same content-addressed cache under the
  same key; `--list` and `--no-sync` may populate it and write nothing to the tree.
- Index rebuild: none.
- Authorization: unchanged — whatever the user's existing git credentials reach.
- Observability: the manifest-edit lines on stderr, described in `add-execution`.
- Deployment: none beyond the next release.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem (working tree) | real, under `t.TempDir()` | real for `internal/apply`; absent for `internal/manifest` and `internal/add`, which are pure over bytes |
| `git` binary | real, on PATH | real in `internal/source`; not reached at all in `internal/manifest`, `internal/add`, or the `internal/cli` usage-error tests |
| Source repository (network) | real local fixture repo built with `git init`, addressed by `file://` path | same for `internal/source`; unreachable-URL cases use a path no repo exists at |
| Fetch cache | real, rooted in `t.TempDir()` | real, same |
| `graft.toml` / `graft.lock` | real files in the temp repository | bytes in memory |
| Terminal detection | `Options.IsTerminal` replaced with a stub returning true or false | same |
| Output streams | `bytes.Buffer` through `cli.Options` | same |
| Clock, randomness, network beyond git | not used | not used |

No fixture repo is shared between tests: each builds its own, with `user.name` and
`user.email` set on the repo rather than the machine, as every existing fixture does.

## Test Strategy

Tiers, as ENGINEERING.md defines them: unit tests over pure logic (`internal/manifest`,
`internal/add`'s derivation, `internal/cli`'s argument validation) and integration or
acceptance tests against fixture git repositories built in `t.TempDir()`
(`internal/source`, `internal/apply`, `internal/cli`'s end-to-end `add` runs).

This change **does** take the outer-loop acceptance test: `add` is a command, its contract is
what a user observes from a shell, and three of its hardest properties — that a failed run
leaves no `graft.toml`, that `--no-sync` makes no network call, that a moved pin re-resolves
one source and no other — are only observable end to end.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| A first source is added to a repository with no graft.toml | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A second source is added beside an existing one | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| Several selectors are written in the order given | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A selector matching nothing leaves graft.toml unwritten | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| Three spellings of one repository derive one name | Table test over the derivation function | unit | none — pure over strings | `go test ./internal/add/` |
| A git value with no usable last segment is refused | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| An explicit rev is written verbatim | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A source with tags gets its highest tag as the default pin | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A source with no tags gets its default branch | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An empty rev is a usage error | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| A new selector joins an existing source | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A selector already declared is not written twice | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| The same selector given twice is written once | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A different repository under a taken name is refused | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An explicit rev on a declared source moves the pin | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| Adding a selector to a branch pin does not move it | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| No selectors, no TTY | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| No selectors on a terminal is the same refusal | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| A malformed selector is refused before the network | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| A catalog is listed with its destinations | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A consumer override is reflected in the listing | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| A source offering no items lists none | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An ungraftable source is refused under --list | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| The manifest is written and nothing else is | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An unverified selector is written | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| Adding a source reports one line | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| Moving a pin and adding a selector reports both, in order | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An invocation that changes nothing says so | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| No arguments | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| An empty source argument | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| --list with selectors | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| --list with --no-sync | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| An unknown flag | `cli.Main` with recorded streams, asserting exit code and stderr text | unit | streams recorded; no git, no fixture | `go test ./internal/cli/ -run TestAdd` |
| An unreachable source leaves no manifest behind | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An unamendable manifest is refused in the amender's words | End-to-end `cli.Main` against a fixture source repo | acceptance | real filesystem, real `git`, fixture remote, cache root in `t.TempDir()` | `go test ./internal/cli/ -run TestAdd` |
| An unparsable graft.toml is refused before anything is resolved | End-to-end `cli.Main` against a temp repository holding a bad manifest | acceptance | real filesystem; no git, no fixture remote | `go test ./internal/cli/ -run TestAdd` |
| A table is appended to a manifest holding one source | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| An empty file gets the table alone | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A file with no final newline gains one | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| Several selectors render on one line, in order | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A one-line array gains a selector on its own line | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A multi-line array gains a line matching its indentation | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A multi-line array with no trailing comma keeps that style | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A selector already present is not added twice | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A comment after the last element survives | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| Appending a name already declared is refused | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| An install that is not an array is refused | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A source written as an inline table is refused | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| An unterminated array is refused | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A selector carrying a quote is refused | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| A name that is not a bare key is refused | Table test over bytes in, bytes out | unit | none — pure over bytes | `go test ./internal/manifest/` |
| The highest stable tag wins | Direct call against a fixture repo with the named refs | integration | real `git`, fixture remote in `t.TempDir()` | `go test ./internal/source/` |
| A source with no semver tags falls back to its branch | Direct call against a fixture repo with the named refs | integration | real `git`, fixture remote in `t.TempDir()` | `go test ./internal/source/` |
| An empty repository is an error, not an empty rev | Direct call against a fixture repo with the named refs | integration | real `git`, fixture remote in `t.TempDir()` | `go test ./internal/source/` |
| An unreachable source is not reported as an empty one | Direct call against a fixture repo with the named refs | integration | real `git`, fixture remote in `t.TempDir()` | `go test ./internal/source/` |
| A git value beginning with a dash is refused before the call | Direct call, asserting the refusal before any exec | unit | none — refused before `exec` | `go test ./internal/source/` |
| Only graft.toml appears | Direct call against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| An existing lock is left alone | Direct call against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| A graft.toml that is not a regular file is refused | Direct call against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |
| A symlinked ancestor is refused | Direct call against a repository built in `t.TempDir()` | integration | real filesystem | `go test ./internal/apply/` |

<!-- 60 rows, one per spec scenario -->
## Decisions

**The install amendment inserts after the last element, not before the closing bracket.**
Inserting before `]` would place a new selector after a comment a consumer left at the end of
the array, or after a blank line, and would need a rule for both. Anchoring on the last
element makes the insertion point a token rather than a shape, and leaves everything between
that element and the bracket untouched. The cost is the one existing byte the amendment may
add: a comma on an element that had none, which invalid TOML would otherwise follow.
*Alternative considered:* refuse multi-line arrays entirely. Rejected — a hand-written
`install` of six selectors is exactly the file a consumer formats across lines, and refusing
it would make `add` useless on the manifests it most needs to amend.

**The appended table is rendered by graft, the existing ones are never re-rendered.** A new
block has no formatting to preserve, so it gets SPEC.md's own alignment. An existing block has
formatting that is the consumer's, so nothing but the pin value and the install array is ever
touched. This is the same split `SetRev` already draws.
*Alternative considered:* re-serializing the whole manifest through the TOML encoder. Rejected
by CLAUDE.md's standing rule, and for the reason behind it: it deletes every comment.

**`add` never re-resolves a pin it did not move.** Adding a selector to a source pinned at
`main` installs from the sha the lock records, not from wherever `main` points today. The
alternative — refreshing the source you happen to be touching — would make `add` a second
`update` and reintroduce exactly the drift `sync` exists to prevent. When `@rev` does move the
pin, that source alone is re-resolved, which is `update`'s existing carve-out reused.

**The default pin is a ref, never a range.** `graft add optioni/shared` writes `v1.3.0`, not
`^1.3.0`. A range is a policy — "adopt future minors of this source" — and inferring a policy
from an invocation that stated none would make every `add` a standing subscription. A
consumer who wants one writes `@^1.3.0`, which costs nothing.

**The default pin and the range `*` share one selection rule.** Both mean "the highest stable
semver tag", and two implementations would eventually disagree about a prerelease or a
`v`-prefixed tag. `DefaultRev` calls `MatchRange(name, "*", tags)` and treats its two failure
messages as "no semver tags", falling through to the default branch.

**`--no-sync` gets its own entry point in `internal/apply` rather than an empty plan.** An
empty plan applied against a populated lock has a prune set of everything the lock claims: the
convenient path is a data-loss bug. A manifest-only apply cannot express that, and reuses the
same staging, rename, pre-flight, and containment the plan-carrying path already has.

**`add` is the only command that may create `graft.toml`.** Every other command failing on its
absence is a useful error — it says "you are not in a graft repository" — and `add` is the
answer to it. Creating it elsewhere would turn a typo'd directory into a new project.

**Source names are derived, never chosen.** A `--as` flag is one more thing to spell in two
places, and the collision it solves is rare enough to be worth a clear refusal instead. The
refusal names the git value the manifest already holds, which is the fact the user needs.

## Risks / Trade-offs

[A multi-line `install` array in a shape the scanner mis-reads, amended in the wrong place] →
The scanner refuses on anything but single-line quoted strings between the brackets, and the
amended bytes are re-parsed and checked to declare every selector asked for before they reach
disk. Both halves must pass; a wrong-place edit that still parses is caught by the second.

[`--no-sync` writes selectors nobody checked against a catalog] → Stated in the spec as the
trade the flag makes, and the next `sync` fails with the catalog's own message. The flag
exists for the offline case, where checking is impossible by definition.

[A source name derived from a repo whose last segment collides with an unrelated declared
source] → Refused, naming the `git` value already declared. The consumer renames the block by
hand; graft never silently retargets a declared source.

[`add` fetches during `--list` and `--no-sync`-without-`@rev`, so a "writes nothing" run still
touches the cache] → Same caveat `--dry-run` already carries, and stated in the spec: the
cache is not the working tree.

[The new `sync.Options.Manifest` field could be set together with `Update.To`, giving two
sources of manifest bytes] → `internal/add` never sets `Update.To`; it hands over already
edited bytes and, when the pin moved, only names the source to re-resolve. A test asserts the
two fields are never both set.

## Migration Plan

None required. The change is additive: a new command, three new internal functions, one new
optional field on an existing options struct. An existing `graft.toml` is only ever read
until an `add` names its source, and a `graft.lock` written before this change is honoured
unchanged. Rollback is reverting the commits; nothing on disk needs undoing.

## Open Questions

None. The two that existed when this was scoped are decided above: the default pin is a ref
rather than a range, and multi-line `install` arrays are amended rather than refused.
