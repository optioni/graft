## Context

Two changes have landed. `manifest-and-lock` parses `graft.toml` and `graft.lock` and
serializes a lock deterministically; `catalog-and-selectors` parses `catalog.yaml` and
expands a consumer's selectors against what a source provides. Between them they can say
*what was asked for* and *what is on offer*. Nothing yet says **which files land where, and
which are deleted**.

SPEC.md's resolution sequence numbers eight steps and draws the line after seven:

> Steps 1–7 are pure. Nothing touches the working tree until step 8, so a validation
> failure leaves no partial state.

This change implements steps 6 and 7 — destination computation and the prune-set diff —
and turns "pure" from an aspiration into a property of a type: `plan.Build` returns either
a complete plan or an error and a nil plan, and the working tree is not reachable from
inside it. Steps 3–5 (fetch, read the catalog from the fetched tree, expand selectors) are
partly done and partly `git-fetch`'s; step 8 is `internal/apply`, added by `sync-command`.

The constraint that shapes everything below: **`internal/plan` may not read the
filesystem.** An item's `from` may name a directory, and knowing which files a directory
holds is a filesystem question. The design answers it by making the file listing an input.

## Goals / Non-Goals

**Goals:**

- Compute every destination from `kinds.<kind>.to` — `{name}`, the trailing-slash rule,
  `flatten`, a list-valued `to` — with the consumer's `[sources.*.kinds]` override winning.
- Derive the prune set from `graft.lock` alone, so a file graft did not write can never be
  deleted.
- Build the next lock as a value, ordered so that serializing it twice is byte-identical.
- Enforce the two planning invariants — no destination escapes the repo root, no two items
  share a destination — before any plan is returned.
- Keep `internal/plan` provably pure, with a test that fails if it ever imports `os`.

**Non-Goals:**

- No writing of any kind. No copy, no delete, no `MkdirAll`, no lock write.
- No fetching, no `git`, no network, no cache. The resolved sha and the file listing are
  inputs.
- No command surface, no `--dry-run`, no `added`/`updated`/`removed` report. Plan holds
  three fields and no presentation.
- No empty-directory removal — a property of a real tree, not of a plan.
- No change to `graft.toml`, `graft.lock`, or `catalog.yaml`.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/plan` | **new** | The whole change. Pure: values in, a plan value out. |
| `internal/manifest` | read-only | `plan.Input` carries a `manifest.Source` for name, git, rev, install, and the kind overrides. No requirement changes. |
| `internal/catalog` | read-only | `plan` calls `catalog.Expand` and reads `catalog.Kind{To, Flatten}` and `catalog.Item{ID, Kind, Name, From}`. No requirement changes. |
| `internal/lock` | read-only + construction | `plan` reads the current `*lock.Lock` for the prune diff and builds the next one out of `lock.Source`/`lock.Item`. It does not call `lock.Marshal`; `apply` will. |
| `internal/itemid` | untouched | The grammar is already enforced upstream by both parsers. |
| `internal/source` | does not exist yet | Its future job — fetch, and enumerate the fetched tree — is the producer of `plan.Input.Items`. This change defines the shape it must produce. |
| `internal/apply` | does not exist yet | Added by `sync-command`. **This change adds no write path**, so there is nothing that could belong there: `plan` returns a description of writes, and describing a write is not performing one. |
| `cmd/graft` | untouched | No flag, no command. Nothing this change adds is invisible to the coverage gate. |

New pieces follow patterns already in the tree:

- **Errors are built in one place per condition and asserted by tests**, as in
  `catalog.errf` and `manifest.validate`'s `fail` closure. `plan` gets the same shape: a
  per-source and per-item closure that prefixes `source "x": item "y": `.
- **Deterministic walks over sorted keys**, as in `catalog.sortedKeys` and
  `manifest.Parse`, so an error message never depends on Go's randomised map iteration.
- **A small path predicate lives with the rule it enforces**, not in a shared package.
  `catalog.inSource` constrains `from`; `lock.isRepoRelative` constrains what a lock may
  authorise deleting; `plan` gets its own `insideRepo` constraining a destination, with its
  own wording. `catalog`'s comment argues this explicitly and is followed rather than
  reversed.

## Contracts

`internal/plan` is a new internal package; nothing outside this module can depend on it, so
nothing here is breaking. The API this change establishes, which `sync-command` and
`git-fetch` will both code against:

```go
// Input is one source's fully resolved inputs: what the consumer asked for, what the
// source's catalog offers, the sha its rev resolved to, and what its tree holds.
type Input struct {
	Source   manifest.Source  // name, git, rev, install, and the kinds overrides
	Resolved string           // the 40-character sha Rev resolved to
	Catalog  *catalog.Catalog // read from the fetched tree
	Items    map[string]Listing
}

// Listing is what one item's From contributes, keyed in Input.Items by item id.
type Listing struct {
	Dir   bool     // From names a directory rather than a file
	Files []string // paths relative to From; for a file item, exactly base(From)
}

type Plan struct {
	Writes []Write   // ordered by Dest
	Prune  []string  // ordered by path
	Lock   *lock.Lock
}

type Write struct {
	Source string // source name, so apply knows which fetched tree to copy from
	Item   string // item id, for the per-item report sync-command will print
	From   string // path within that fetched tree
	Dest   string // repo-relative destination
}

func Build(inputs []Input, lk *lock.Lock) (*Plan, error)
```

`Write.From` is **not** unconditionally `path.Join(item.From, rel)`. For a directory item it
is. For a file item, whose single `rel` is `base(From)` by D2, it is `item.From` itself —
joining there would produce `extras/agents/x.md/x.md` and every copy would fail at apply
time. The asymmetry is the price of D2 plus D3 and is pinned by its own spec scenario rather
than left to be discovered.

**Preconditions on `Input`** — `Build` is total over well-formed inputs and does not
re-validate what its collaborators already guarantee. It relies on: every installed item's
kind being declared in `Catalog.Kinds` (guaranteed by `catalog.Parse`, which refuses an item
whose kind is undeclared); source names being unique (guaranteed by `manifest.Parse`, whose
sources come from a TOML table); and `Listing.Files` holding exactly `base(From)` when `Dir`
is false. A caller violating the first would silently plan no writes for that item, which is
why the round-trip scenario exists: the constructed lock is parsed back with `lock.Parse`,
and every constraint `graft.lock` enforces on load is therefore checked against what `plan`
produced.

`Resolved` is the exception, and is **checked rather than assumed** — see D11. It was
drafted as `git-fetch`'s guarantee alongside the others, and it does not behave like them:
the rest are consequences of what planning itself does, so the round-trip catches a
violation, while a bad `Resolved` is carried through untouched into a lock `lock.Parse`
would refuse.

A listing entry is not a precondition either. `Listing.Files` arrives from a source
repository by way of whatever enumerated its tree, and an entry that is not a relative path
inside the item is refused rather than trusted — the containment invariant, which the
repo-root check alone does not give.

**Error surface** — every message is asserted by a test and is a deliberate contract:

| Condition | Message |
|---|---|
| Selector matches nothing | *(unchanged, returned from `catalog.Expand`)* |
| Override names an undeclared kind | `source "s": kind override "agnet" names a kind the catalog does not declare` |
| Two `to` entries interpolate alike | `source "s": item "schema:tdd": destinations "a/{name}" and "a/tdd" both interpolate to "a/tdd"` |
| Flatten maps two files alike | `source "s": item "agent:pack": flatten maps "a/dup.md" and "b/dup.md" to the same destination ".claude/agents/dup.md"` |
| Destination escapes the repo root | `source "s": item "agent:x": destination "../outside/agents/" escapes the repo root` |
| Two items resolve alike | `source "a" item "agent:x" and source "b" item "agent:y" both resolve to ".claude/agents/x.md"` |
| One item's file is another's directory | `source "s" item "doc:api" writes "docs/api" and source "s" item "schema:api" writes "docs/api/index.md": one cannot contain the other` |
| A listing entry leaves its item | `source "s": item "schema:tdd": file "../../../etc/passwd" is not a relative path inside the item` |
| `Resolved` is not a sha | `source "s": resolved "" is not a 40-character hex sha` |

No pagination, no streaming, no compatibility surface. `Build` is additive.

## Persistence and Rollout

- **Migration**: none. No format changes and no on-disk state is produced by this change.
- **Backfill**: none.
- **Seeding**: none.
- **Cache invalidation**: none — the fetch cache does not exist yet and `plan` never
  consults it.
- **Index rebuild**: none.
- **Authorization**: none. There is no privileged operation; the repo root boundary is the
  only access rule and it is enforced in `insideRepo`.
- **Observability**: none. No logging, no metrics, no telemetry — SPEC.md forbids telemetry
  outright, and output belongs to `command-surface`.
- **Deployment**: none. No release is cut for this change; it ships when a command uses it.

**Effect on `graft.lock`'s format: none.** `version` stays `1`. This change constructs
`lock.Lock` values with exactly the fields `manifest-and-lock` already defined and relies
on `lock.Marshal`'s existing normalization; it adds no key, removes none, and reorders
nothing.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem (working tree) | n/a — no acceptance test in this change | **not used at all**. `plan` never opens, stats, or walks a path. A guard test parses the package's own imports and fails if `os`, `io/fs`, `path/filepath`, `os/exec`, or `net` appears. |
| Filesystem (source tree) | n/a | **replaced by a value**: `Input.Items` is a literal `map[string]Listing` in each test. No `t.TempDir()`, no fixture directory. |
| `git` binary | n/a | **not used**. `plan` runs no command. Covered by the same import guard (`os/exec`). |
| Network | n/a | **not used**. Covered by the same import guard (`net`, `net/http`). |
| Fetch cache (`~/.cache/graft/`) | n/a | **not used**. The resolved sha is a string field on `Input`; the cache path never enters `plan`. |
| Fixture git repositories | n/a | **not used**. Nothing in this change needs a repo, so the `user.name`/`user.email`-on-the-repo trap does not arise here — it returns with `git-fetch`. |
| `internal/catalog` | n/a | **real**. Catalogs are built either as literal `catalog.Catalog` values or by `catalog.Parse` on a byte literal; never loaded from disk. |
| `internal/manifest` | n/a | **real**. Sources are literal `manifest.Source` values or `manifest.Parse` on a byte literal. |
| `internal/lock` | n/a | **real**. The current lock is a literal `lock.Lock` or `lock.Parse` on a byte literal; the byte-equality assertions call the real `lock.Marshal`. |
| Clock, randomness, environment | n/a | **not used**. `plan` reads no environment variable and has no time-dependent behavior. |

Every literal above is an in-memory value. No test in this change creates a directory.

## Test Strategy

Everything in this change is pure logic over values, so **every scenario is a unit test in
`./internal/plan/`**, run by `go test ./internal/plan/...` and gated by `task cover` at 80%
over `./internal/...`.

**This change takes no outer-loop acceptance test, and the acceptance group is deleted from
tasks.md.** The reason: no command exists. `command-surface` has not landed, `graft sync`
does not exist, and `internal/apply` — the only thing that could make a plan observable in a
tree — is added by `sync-command`. An end-to-end test here could only assert against an
invented harness, which is a boundary the table above does not name. The end-to-end
evidence for this logic arrives in `sync-command`, whose integration tests run first sync,
idempotent re-sync, a version bump, a dropped item, and the surviving foreign file against
fixture repositories.

Two verifications are not ordinary table tests and are named as such below:

- **Import guard** — `TestPackageImportsNothingImpure` parses every non-test file in the
  package with `go/parser` and fails on any import of `os`, `io/fs`, `path/filepath`,
  `os/exec`, `net`, or `net/http`. This is the enforcement of the concentration point that
  `plan` stays pure; a future filesystem read fails the build rather than being noticed in
  review.
- **Byte equality** — determinism is asserted as `bytes.Equal(lock.Marshal(a), lock.Marshal(b))`
  across two builds from shuffled inputs, not as `reflect.DeepEqual` of the structs.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| Computing destinations touches nothing | `TestPackageImportsNothingImpure` plus a destination table case | unit | none real | `go test ./internal/plan/ -run 'Impure\|Destination'` |
| An item contributing no files computes no destinations | table case: empty `Listing.Files` | unit | none | `go test ./internal/plan/ -run Destination` |
| A directory item preserves its structure under an interpolated destination | table case | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| A trailing slash places a file item inside the directory | table case | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| Without a trailing slash a file item lands at the destination itself | table case | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| A trailing slash is a no-op for a directory item | table case asserting equality with the slashless result | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| A destination with no `{name}` is used as written | table case, two items | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| Nested files are flattened into the destination root | table case, `Flatten: true` | unit | `catalog` real | `go test ./internal/plan/ -run Flatten` |
| Without flatten the same item preserves its structure | table case, same listing, `Flatten: false` | unit | `catalog` real | `go test ./internal/plan/ -run Flatten` |
| Two files flattening onto one path is an error | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Flatten` |
| One item lands in two destinations | table case, list-valued `To` | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| Two entries interpolating to one destination is an error | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Destination` |
| An override moves a kind's items | table case with `manifest.Source.Kinds` | unit | `manifest`, `catalog` real | `go test ./internal/plan/ -run Override` |
| An override replaces a list-valued destination entirely | table case | unit | `manifest`, `catalog` real | `go test ./internal/plan/ -run Override` |
| An override keeps the catalog's flatten | table case | unit | `manifest`, `catalog` real | `go test ./internal/plan/ -run Override` |
| An override applies to its own source only | two-`Input` `Build` case | unit | `manifest`, `catalog`, `lock` real | `go test ./internal/plan/ -run Override` |
| An override for an undeclared kind is an error | error-text assertion | unit | `manifest`, `catalog` real | `go test ./internal/plan/ -run Override` |
| A `to` climbing out of the repo is refused | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Escape` |
| An absolute `to` is refused | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Escape` |
| An escaping consumer override is refused | error-text assertion | unit | `manifest`, `catalog` real | `go test ./internal/plan/ -run Escape` |
| A listing entry climbing out of its item is refused | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Escape` |
| A `to` escaping with no files to place is still refused | error-text assertion, empty `Files` | unit | `catalog` real | `go test ./internal/plan/ -run Escape` |
| A destination at the repo root itself is accepted | table case | unit | `catalog` real | `go test ./internal/plan/ -run Escape` |
| A first plan against no lock | `Build` with `&lock.Lock{Version: 1}` | unit | `manifest`, `catalog`, `lock` real | `go test ./internal/plan/ -run Build` |
| A plan for a manifest with no sources | `Build(nil, empty)` | unit | `lock` real | `go test ./internal/plan/ -run Build` |
| An item contributing no files still appears in the lock | `Build` case asserting `Items[i].Files` is empty and present | unit | `lock` real | `go test ./internal/plan/ -run Build` |
| A failing plan is returned as no plan at all | every error case asserts the returned `*Plan` is nil | unit | all real | `go test ./internal/plan/` |
| A write carries the source path and the destination | `Build` case asserting all four `Write` fields | unit | `catalog` real | `go test ./internal/plan/ -run Build` |
| A file item's write names the file itself, not a path below it | `Build` case asserting `Write.From` equals the item's `from` | unit | `catalog` real | `go test ./internal/plan/ -run Build` |
| Writes are ordered by destination across sources and items | `Build` from shuffled inputs, assert write order | unit | all real | `go test ./internal/plan/ -run Determinism` |
| A file already present in the tree is still written | `Build` case where the lock already claims the path; asserts a write and an empty prune set. The tree itself is deliberately absent from the test — that it cannot matter *is* the property | unit | `lock` real | `go test ./internal/plan/ -run Build` |
| A foreign file in a shared destination is never pruned | `Build` case, twice: item kept and item dropped; asserts the foreign path appears in no field of the plan | unit | `lock` real | `go test ./internal/plan/ -run Prune` |
| An item dropped from install has its files pruned | `Build` case | unit | `lock`, `catalog` real | `go test ./internal/plan/ -run Prune` |
| An item the source stopped providing has its files pruned | `Build` case, `agent:*` still matching another item | unit | `lock`, `catalog` real | `go test ./internal/plan/ -run Prune` |
| A source removed from the manifest has all its files pruned | `Build` case | unit | `lock` real | `go test ./internal/plan/ -run Prune` |
| A moved destination prunes the old path and writes the new one | `Build` case with an override added | unit | `manifest`, `lock` real | `go test ./internal/plan/ -run Prune` |
| A version bump that adds and removes items | `Build` case with a changed listing | unit | `lock` real | `go test ./internal/plan/ -run Prune` |
| A path moving from one source to another is written, not pruned | `Build` case with two sources and one migrating path | unit | `lock`, `catalog` real | `go test ./internal/plan/ -run Prune` |
| An idempotent re-plan prunes nothing | `Build`, feed its lock back in, `Build` again, compare `lock.Marshal` bytes | unit | `lock` real | `go test ./internal/plan/ -run Determinism` |
| An item placed in two destinations records both files | `Build` case asserting lock file order | unit | `lock`, `catalog` real | `go test ./internal/plan/ -run Build` |
| The next lock carries rev and resolved separately | `Build` case asserting the serialized lock bytes | unit | `manifest`, `lock` real | `go test ./internal/plan/ -run Build` |
| The next lock round-trips through the lock parser | `lock.Parse(lock.Marshal(p.Lock))` succeeds and round-trips | unit | `lock` real | `go test ./internal/plan/ -run Build` |
| Sources, items, and files are ordered independently of input order | two `Build` calls from shuffled inputs, `bytes.Equal` on `lock.Marshal` | unit | `lock` real | `go test ./internal/plan/ -run Determinism` |
| Two items of one source colliding is an error | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Collision` |
| Two sources colliding is an error | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Collision` |
| A path claimed by the lock and by another item is still a collision | error-text assertion with a populated lock | unit | `lock`, `catalog` real | `go test ./internal/plan/ -run Collision` |
| One item producing the same path twice is not this error | asserts the within-item message, not the cross-item one | unit | `catalog` real | `go test ./internal/plan/ -run Collision` |
| A selector matching nothing fails the plan | error-text assertion, message identical to `catalog.Expand`'s | unit | `catalog` real | `go test ./internal/plan/ -run Selector` |
| A catalog providing zero items fails the plan | error-text assertion | unit | `catalog` real | `go test ./internal/plan/ -run Selector` |

## Decisions

**D1 — The source file listing is an input, not something `plan` gathers.**
An item's `from` may be a directory, and its contents are a filesystem fact. `plan` takes
`Listing` values so the fact arrives as data. *Alternatives:* (a) `plan` walks the fetched
tree with `filepath.WalkDir` — rejected outright, it is exactly the filesystem read AGENTS.md
forbids and would make "no partial writes on failure" untestable without a temp dir; (b)
inject an `fs.FS` and use `fstest.MapFS` in tests — rejected because an `fs.FS` parameter is
a standing invitation to hand it `os.DirFS`, and because enumeration policy (symlinks,
ignored files, permissions) belongs with the package that fetched the tree, not with the one
computing paths. A plain map cannot be a real filesystem by accident.

**D2 — Listing paths are relative to the item's `from`.**
`Listing.Files` holds `templates/design.md`, not `extras/openspec-schemas/tdd/templates/design.md`.
*Alternative:* source-relative paths, with `plan` trimming the `from` prefix — rejected: the
trim is a silent mis-derivation when a listing entry is not actually under `from`, and it
puts a second copy of the `from` grammar in `plan`. Relative paths make `Write.From` a
`path.Join(item.From, rel)` and nothing else.

**D3 — `Listing.Dir` is explicit rather than inferred.**
The file-versus-directory distinction changes the trailing-slash rule, so it must be exact.
*Alternative:* infer it as `len(Files) == 1 && Files[0] == path.Base(From)` — rejected: a
directory holding exactly one identically named file is indistinguishable from a file, and
the failure mode is a destination one level off with no error.

**D4 — A trailing `/` changes the result only for a file item.**
For a directory item, `to: "openspec/schemas/{name}"` and `to: "openspec/schemas/{name}/"`
produce the same destinations. *Alternative:* treat "into this directory" literally for
directory items too, appending `base(from)` — rejected on two grounds. It would produce
`openspec/schemas/tdd/tdd/schema.yaml` for SPEC.md's own example written with a slash, and
it would put `from`'s leaf into every consumer's tree, contradicting SPEC.md's "`from` may
move freely without touching any consumer". The chosen reading is the only one under which
both of SPEC.md's worked examples and that stability claim all hold. Recorded in Open
Questions as a reading of an underspecified sentence.

**D5 — An override naming an undeclared kind is an error.**
*Alternative:* ignore it, since an override for a kind with no items is harmless — rejected.
SPEC.md makes exactly this trade for selectors ("A selector matching nothing is an error,
not a warning") and gives the reason ("Typo protection"). An ignored override is worse than
an ignored selector: the consumer believes files moved, and they did not.

**D6 — `plan` calls `catalog.Expand` but not `lock.CheckPins`.**
Expansion is part of turning a manifest into a file set, and SPEC.md's step 5 sits inside
the pure range. The pin check is step 1's business and must fire before anything is fetched,
so it belongs to the caller — running it inside `Build` would mean a network round trip
happening before a check that could have refused it.

**D7 — `plan` builds the next lock; `apply` only serializes it.**
The lock's `files` list *is* the new file set, so computing it anywhere else means computing
it twice. `apply` calls `lock.Marshal(p.Lock)` last, after every file operation succeeds,
which is the SPEC invariant. *Alternative:* have `apply` derive the lock from the writes it
performed — rejected: it would make the lock a record of what happened rather than of what
was planned, and a partially-applied run could then write a lock.

**D8 — Collisions are detected in one deterministic walk and the first is reported.**
Sources by name, items by id, destinations in declared order, files by ascending path. A map
from destination to owner is filled as the walk proceeds; the first second-claimant is the
error. *Alternative:* collect every collision and report them all — rejected for now: one
message is the failure-mode table's shape, and a stable *first* is what makes the error text
assertable.

**D9 — Within-item collisions get their own messages.**
A flatten collision and two `to` entries interpolating alike are one item colliding with
itself; reporting them with the cross-item message would print the same item id twice and
name neither cause. Separate messages name the two `from`-relative paths, or the two `to`
entries, which is the information needed to fix the catalog.

**D10 — The repo-root predicate is written in `plan`, not shared.**
It is the third copy of a five-line path rule (`catalog.inSource`, `lock.isRepoRelative`).
`catalog`'s own comment argues the case for keeping each with the rule it enforces rather
than hoisting a shared package, and each has different wording and a different subject —
source path, deletable path, destination path. Consistency with the existing precedent beats
removing fifteen lines of duplication.

**D11 — `Resolved` is validated in `Build`, and the nesting invariant is enforced there.**
Both strengthen what the draft said, and both were argued after the change-review pass.

`Resolved` sits in the Preconditions list above as `git-fetch`'s guarantee, but it is not
like its neighbours: the others are properties of what planning does, so the round-trip
scenario catches a violation of any of them, whereas a bad `Resolved` is copied verbatim
into the lock and `lock.Marshal` validates nothing. The failure would surface one run
later, in `lock.Parse`, against a file the user is told not to edit. *Alternative:* state
the precondition in the requirement and leave the check to `git-fetch` — rejected: it makes
two packages disagree about what a valid `graft.lock` is, and the cost of agreeing is five
lines.

The nesting invariant — one item's file may not be the directory another item's file needs
— is not in SPEC.md's original Invariants list. It is added to it by this change, because
the list is the floor rather than the ceiling and this case has the consequence the list
exists to prevent: `docs/api` and `docs/api/index.md` cannot both exist, so applying the
plan fails partway through, and since the lock is written last the file already written is
left outside `graft.lock` where no prune can reach it. *Alternative:* defer it to
`sync-command`, where `internal/apply` exists — rejected: a plan whose application cannot
succeed is not a valid plan, and the whole reason this package returns a value rather than
performing writes is so that it can say so before anything is touched.

## Risks / Trade-offs

- **The trailing-slash reading (D4) is a choice SPEC.md does not settle.** → Both of
  SPEC.md's worked examples are pinned as scenarios, the alternative is written down here
  and in Open Questions, and the reading is the one that preserves SPEC.md's stated
  `from`-mobility property. If it proves wrong, it changes one function and its table test.
- **`plan` trusts `Listing` to describe a real tree.** A listing naming a file that does not
  exist plans a write that will fail at apply time. → The fidelity of the listing is
  `git-fetch`'s contract and is tested there against real fixture repositories; `plan`'s
  only guard is that a listing entry cannot aim a destination outside the repo, which is
  tested here.
- **The undeclared-override error (D5) can block a consumer whose override went stale when a
  source renamed a kind.** → The message names the kind and the source, which is the fix;
  and the same failure would otherwise be silent file placement at the old destination.
- **`Plan` could accrete reporting fields as later changes need them.** → It has exactly
  three fields and the Non-Goals say why. `sync-command` can derive `added`/`updated`/
  `removed` from `Writes`, `Prune`, and the old lock without `plan` growing a report type.
- **The import guard is a lint-by-test and could be seen as unusual.** → It is the cheapest
  enforcement of the one architectural rule this package exists to hold, it uses only
  `go/parser`, and it fails loudly with the offending import named.

## Migration Plan

None needed. `internal/plan` is new, has no callers until `sync-command`, produces no
on-disk state, and changes no file format. There is nothing to deploy, backfill, or roll
back; reverting the change is deleting the package.

## Open Questions

**Q1 — Does a trailing `/` mean anything for a directory item?**
SPEC.md says "A trailing `/` means 'into this directory'" without saying whether the item's
own leaf name is appended. Resolved as D4: a trailing slash is a no-op for a directory item
and appends `base(from)` for a file item. This is the reading under which both SPEC.md
examples and its `from`-mobility claim hold simultaneously. Recorded rather than silently
chosen, and written back into SPEC.md's `to` bullet by this change's group 11, so the next
reader does not have to re-derive it.

**Q2 — May a destination land inside `.git/`?**
SPEC.md's invariant is only "no destination escapes the repo root", and `.git/` is inside
it. This change does not add a `.git/` restriction, because inventing one would be behavior
SPEC.md does not specify. It is a real hazard for a hostile source and belongs in a change
that argues for it against the threat model SPEC.md's Security section states.

**Q3 — Is an override for a declared kind with no installed items an error?**
No. The kind exists, so it is not a typo; a consumer overriding `hook` before installing any
hook is legitimate. Only an override naming a kind the catalog never declared is refused.

**Q4 — What happens to an installed item with no entry in `Input.Items`?**
It is treated as an empty listing — identical to an item whose `from` is an empty directory:
no writes, and an item with an empty `files` list in the lock, which `lock-format` already
declares valid. `Build` stays total; supplying an entry per installed item is `git-fetch`'s
obligation, verified there.
