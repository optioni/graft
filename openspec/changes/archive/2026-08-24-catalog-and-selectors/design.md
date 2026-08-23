## Context

`internal/manifest` and `internal/lock` landed in `manifest-and-lock`, together with
`internal/itemid`, which holds the shared `kind:name` grammar. `internal/catalog`,
`internal/source`, `internal/plan`, and `internal/apply` do not exist yet.
`openspec/IMPLEMENTATION-ORDER.md` puts this change second in Phase 1, depending on
nothing and feeding `destination-and-plan`.

Three constraints shape everything below.

1. **`catalog.yaml` is written by a *source* repository, not by the consumer.** It is the
   least trusted of the three files: a human on the other side of a git URL writes it, and
   graft acts on it. Every field it carries is validated at the door, and `from` staying
   inside the source tree is one of SPEC.md's invariants rather than a nicety.
2. **Kinds are arbitrary and always will be.** graft holds no list of valid kinds, so
   validation can only be structural — a kind is well-formed or it is not; it is never
   "recognised".
3. **`internal/apply` is the only package permitted to write.** This change reads bytes and
   returns values. It adds no write path anywhere.

The module has one dependency, `github.com/BurntSushi/toml`. YAML decoding is the second.

## Goals / Non-Goals

**Goals:**

- `internal/catalog` loads and validates `catalog.yaml` into a value `destination-and-plan`
  consumes.
- Selector expansion turns a source's `install` list into the set of items it names, with
  globs in the name position and a no-match error that prints the real vocabulary.
- Parse-time validation covers the catalog's internal consistency: declared kinds, unique
  item ids, and `from` containment.
- Every error message in the package is fixed text a test can assert.

**Non-Goals:**

- **Destination computation.** `{name}` interpolation, trailing-slash semantics, `flatten`,
  list-valued `to` fan-out, and consumer overrides are `destination-and-plan`. This change
  carries `to` and `flatten` verbatim and interprets neither.
- The prune set, the repo-root escape check over *computed* destinations, and the
  cross-item collision check — all `destination-and-plan`.
- Fetching, cloning, rev resolution, and the cache — `git-fetch`. This change never learns
  where the bytes came from.
- Writing any file, and any code in `internal/plan` or `internal/apply`.
- Any CLI surface: no command, flag, exit code, or stderr formatting.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/catalog` | **new** | `Parse([]byte, filename) (*Catalog, error)`, a thin `Load(path)`, and `Expand(*Catalog, source string, selectors []string) ([]Item, error)`. Pure over decoded values; the only disk access is `Load`'s single `os.ReadFile`. |
| `internal/itemid` | **caller added** | `Valid` is reused for `provides` ids and for selectors. No change to the package — the grammar is already the one both other files use. |
| `internal/manifest` | untouched | `Expand` takes a source name and a `[]string`, not a `manifest.Source`, so `catalog` does not import `manifest`. Selector *syntax* is already validated there; `catalog` validates *matching*. |
| `internal/lock` | untouched | Nothing here reads or writes a lock. |
| `internal/source` | untouched | `git-fetch`. `Load` takes a path; who fetched the tree it sits in is not this package's business. |
| `internal/plan` | untouched | This change adds **no** code to `plan`, so its purity is unaffected. No test in this change needs a real directory to exercise catalog logic — `Parse` and `Expand` take bytes and values. |
| `internal/apply` | untouched | This change adds **no write path**. `Parse` and `Expand` return values; `Load` opens one file read-only. There is nothing to write: a catalog is an input, never an output. |
| `cmd/graft` | untouched | Nothing is wired to a command yet, and coverage is measured over `./internal/...` only — logic placed in `cmd/graft` would be invisible to the gate. |

`internal/catalog` follows the shape `internal/manifest` and `internal/lock` established: a
small exported surface, `Parse`/`Load` split, no package-level state, no `init()`, errors
returned rather than logged, and error text prefixed with the file it came from.

## Contracts

**External consumers: none.** No released `graft` reads `catalog.yaml`, and no repository
publishes one for graft to read. The format is *introduced* here rather than changed, so
its `version` is established at `1`. Nothing in this change is **BREAKING**.

**Effect on `graft.lock`'s format: none, and `version` does not move.** This change writes
no lock, reads no lock, and adds no field to one. It does supply the item ids that a lock
will later record, and those ids come from `internal/itemid`'s existing grammar — the same
grammar `lock` already validates — so no lock this change influences can hold an id the
current lock parser would reject.

**Internal consumers**, all downstream changes:

- `catalog.Catalog` — `Version int`, `Kinds map[string]Kind`, `Items []Item`.
- `catalog.Kind` — `To []string` (always a list; a string-valued `to` becomes a
  one-element list) and `Flatten bool`. Destinations are verbatim, uninterpolated,
  uncleaned.
- `catalog.Item` — `ID`, `Kind`, `Name`, `From`. `ID` is `Kind + ":" + Name` and is what
  the lock records. `Items` is sorted by `ID`.
- `catalog.Expand` returns `[]Item` sorted by `ID`, deduplicated.
- Error surface: every failure is a plain `error` whose `Error()` is the exact string the
  specs name. Parse errors are prefixed with the filename; expansion errors are prefixed
  with `source "<name>": ` instead, because expansion is not a property of one file — it
  is a consumer's request meeting a source's offer. YAML syntax failures are wrapped as
  `catalog.yaml: <decoder message>`; only the prefix is a fixed contract, because the
  decoder's own text is not ours to pin. No sentinel errors and no error types are exported
  until a caller needs to branch on one.
- Absence: `catalog.Load` on a missing file is an error — unlike `lock.Load`, and like
  `manifest.Load` — because SPEC.md's failure-mode table makes a missing catalog mean "the
  repo is not graftable", explicitly with no fallback.

## Visual Design

Not applicable. This change builds no user-facing view and no email template — it adds one
non-visual Go package with no CLI surface — so there is no design source to import and this
section is deliberately empty rather than invented.

## Persistence and Rollout

- Migration: none. No datastore.
- Backfill: none.
- Seeding: none.
- Cache invalidation: none — the fetch cache does not exist yet and is not read here.
- Index rebuild: none.
- Authorization: none. graft has no accounts and no auth layer.
- Observability: none. There is no telemetry of any kind, by design.
- Deployment: none. Nothing is wired to a command, so building this changes no released
  behavior. `go.mod` gains `github.com/goccy/go-yaml`; Dependabot already covers Go modules.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem | no acceptance test in this change | **real**, via `t.TempDir()`, and only for `Load` — every parse, validate, and expand test passes `[]byte` or a constructed `*Catalog` and touches no disk |
| `git` binary | no acceptance test in this change | **not involved** — this change runs no git command and needs no fixture repository, so the `user.name`/`user.email`-on-the-repo hazard does not arise here |
| Network (runtime) | no acceptance test in this change | **not involved** — no code path resolves, clones, or fetches; no test opens a socket |
| Go module proxy (build time) | no acceptance test in this change | **real, once** — tasks.md group 1 runs `go get` and `go mod tidy` against it. After that, `go.sum` pins the dependency and the suite needs no network |
| Fetch cache (`~/.cache/graft/`) | no acceptance test in this change | **not involved** — never read, never created; no test may reference `$HOME` |
| YAML decoder (`github.com/goccy/go-yaml`) | no acceptance test in this change | **real** — the library is the unit under test as much as our code is; its generic decode into `any` is what the unknown-key walk and the `to` type switch are built on |
| `internal/itemid` | no acceptance test in this change | **real** — the shared grammar is called directly, not restated |
| `internal/manifest`, `internal/lock` | no acceptance test in this change | **not involved** — neither is imported and no test constructs one. `internal/lock`'s unknown-key walk is *read* during tasks.md 5.4 to decide whether the two walks should share code; reading source is not a test collaborator, and if that refactor ever landed it would move code into a third package rather than make `catalog` depend on `lock` |
| `internal/plan`, `internal/apply` | no acceptance test in this change | **not involved** — neither is imported, and neither gains code |
| `path.Match` (stdlib glob) | no acceptance test in this change | **real** — the selector matcher is a thin call over it, and its `ErrBadPattern` is what the malformed-pattern error reports |
| Golden fixtures | no acceptance test in this change | **not involved** — this change has no byte-level output to pin, so `internal/catalog` gets no `testdata/` directory; every input is an inline YAML literal |
| Clock, environment variables, working directory | no acceptance test in this change | **not involved** — no output depends on time, `$HOME`, `NO_COLOR`, or the process CWD; every path in a test is inside a `t.TempDir()` |

## Test Strategy

Two tiers only, matching `manifest-and-lock`.

- **unit** — `[]byte` or a constructed `*Catalog` in, value or error out. No filesystem. Run
  with `go test ./internal/catalog/...`.
- **fs** — a `t.TempDir()` with real files, exercising `Load` and absence. Still fast, still
  under `./internal/...`, still no git and no network. Same command.

Full gate is `task test` (race detector) and `task cover` (80% floor over `./internal/...`).

**No outer-loop acceptance test.** This change alters no end-to-end command behavior: it
adds no command, no flag, and no output. There is nothing to drive from the outside, so
`tasks.md` deletes the outer-loop acceptance group rather than inventing a CLI to justify
it. The first acceptance test in this project belongs to `sync-command`.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| catalog-format · A valid catalog loads | `TestLoad_Valid` writes the SPEC.md example into a `t.TempDir()` and asserts kinds, items, and that no file changed | fs | filesystem real, YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A catalog with zero provides loads | table case in `TestParse_Empty`, asserts zero items and nil error | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A catalog with neither kinds nor provides loads | table case in `TestParse_Empty` | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A missing catalog is the not-graftable error | `TestLoad_Missing` in an empty `t.TempDir()`, asserts exact text and that no file was created | fs | filesystem real | `go test ./internal/catalog/...` |
| catalog-format · Malformed YAML is an error | `TestParse_BadYAML` asserts the `catalog.yaml: ` prefix only | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A catalog that is not a mapping is an error | table case in `TestParse_Shape`, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · An empty catalog file is a missing-version error | table case in `TestParse_Shape`, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A missing version is an error | table case in `TestParse_VersionErrors`, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A newer version fails and says to upgrade | table case, exact text, input also carries an unknown top-level key, and asserts no catalog is returned | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A version below 1 is an error | table case, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A string-valued to is carried verbatim | `TestParse_Kinds` asserts a one-element `To` and `Flatten == false` | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A trailing slash is preserved | same test, asserts the trailing `/` and `Flatten == true` | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A list-valued to is carried in declared order | same test, asserts both destinations in order | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · An empty kind name is an error | table case in `TestParse_KindErrors`, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A missing or empty to is an error | table case, three inputs (absent, `""`, `[]`), same exact text each | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · An empty destination inside a list is an error | table case, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A to of the wrong type is an error | table case, both a mapping and a number, same exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A repeated destination within one kind is an error | table case, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · Items are carried with kind, name, and from | `TestParse_Items` asserts id, kind, name, and from | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · Items are ordered by id | `TestParse_ItemOrder` builds the file in a deliberately wrong order and asserts the sorted ids | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A missing field is an error | table case in `TestParse_ItemErrors`, three inputs and three exact messages | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A kind or name containing a colon is an error | table case, exact text | unit | YAML decoder real, `internal/itemid` real | `go test ./internal/catalog/...` |
| catalog-format · An item naming an undeclared kind is an error | table case, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A duplicate item is an error | table case, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · A from outside the source tree is an error | table case, four inputs (`../outside`, `/etc/passwd`, `.`, `./extras/tdd`), exact text each | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · An unknown top-level key is an error | table case in `TestParse_UnknownKey`, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · An unknown key inside a kind is an error | table case, exact text | unit | YAML decoder real | `go test ./internal/catalog/...` |
| catalog-format · An unknown key inside a provides entry is an error | table case, exact text, asserting the 0-based index | unit | YAML decoder real | `go test ./internal/catalog/...` |
| selector-expansion · A plain selector selects exactly one item | table case in `TestExpand`, asserts the single id and that no file was touched | unit | none — the catalog is constructed in memory | `go test ./internal/catalog/...` |
| selector-expansion · Several selectors produce the union ordered by id | table case, selectors given in reverse id order | unit | none | `go test ./internal/catalog/...` |
| selector-expansion · Overlapping selectors yield each item once | table case, asserts length as well as ids | unit | none | `go test ./internal/catalog/...` |
| selector-expansion · An empty selector list expands to nothing | table case, zero items and nil error | unit | none | `go test ./internal/catalog/...` |
| selector-expansion · A trailing star selects every item of a kind | table case in `TestExpand_Globs` | unit | `path.Match` real | `go test ./internal/catalog/...` |
| selector-expansion · A prefix glob selects a subset | table case | unit | `path.Match` real | `go test ./internal/catalog/...` |
| selector-expansion · A question mark matches exactly one character | table case, catalog holds both `schema:td` and `schema:tdd` | unit | `path.Match` real | `go test ./internal/catalog/...` |
| selector-expansion · The kind position is matched literally | table case in `TestExpand_Errors`, exact no-match text | unit | `path.Match` real | `go test ./internal/catalog/...` |
| selector-expansion · A malformed glob pattern is an error | table case, exact text, asserts no items returned | unit | `path.Match` real (`ErrBadPattern`) | `go test ./internal/catalog/...` |
| selector-expansion · A misspelled selector is an error listing what the catalog provides | table case in `TestExpand_Errors`, exact text including the full provides list | unit | none | `go test ./internal/catalog/...` |
| selector-expansion · One selector matching does not excuse another that does not | table case, exact text, asserts no items returned | unit | `path.Match` real | `go test ./internal/catalog/...` |
| selector-expansion · A glob matching nothing is an error | table case, exact text | unit | `path.Match` real | `go test ./internal/catalog/...` |
| selector-expansion · Any selector against a catalog providing zero items is an error | table case, exact `catalog provides no items` text | unit | none | `go test ./internal/catalog/...` |

## Decisions

**Decode with `github.com/goccy/go-yaml`.** It is actively maintained, has zero transitive
dependencies, and decodes cleanly into `any`. *Alternatives:* `gopkg.in/yaml.v3` — the
obvious default, but its repository has been archived and in maintenance-only state since
2022, and adopting an unmaintained parser for the one file graft reads from an untrusted
source is the wrong trade; `sigs.k8s.io/yaml` — converts through JSON, which would flatten
the string-or-list `to` distinction into the same `any` handling anyway while adding a
dependency on `yaml.v2` underneath. The version is pinned in `go.mod` and covered by
Dependabot.

**Decode once into `any`, then walk it.** Rather than a typed struct plus the library's
`DisallowUnknownField`, the parser decodes the document into `map[string]any` and reads
fields off it. Three things fall out of that and out of no other approach: unknown keys can
be attributed precisely — `kind "agent"`, `provides[1]` — where a struct-based option can
only name a field; the string-or-list `to` is a type switch instead of a custom
`UnmarshalYAML` whose interaction with strict decoding would need its own tests; and the
"wrong type" errors become one uniform message instead of the decoder's prose. This is the
same shape `internal/lock` settled on for the same reason, so the two packages read alike.
*Alternative rejected:* typed decode plus a second generic decode purely for the key walk —
two decodes of the same bytes that can disagree.

**Version is validated before anything else, including unknown keys.** A `catalog.yaml`
written for a future graft will carry keys this binary does not know. Checking the version
first means such a file is answered with "upgrade graft" rather than with a confusing
complaint about a key that a newer format legitimately defines. ENGINEERING.md's
compatibility rule — "a graft that meets a format version it does not know fails and says
to upgrade, it never guesses, and never half-reads a newer file" — reads as requiring
exactly this ordering. YAML integers arrive from the decoder as `uint64`, `int64`, or `int`
depending on the literal, so the version reader accepts any integer kind and rejects
non-integers.

**Map keys are walked in sorted order, everywhere.** `kinds` is a YAML mapping and
`provides` entries carry a `kind` key, so validation order would otherwise follow Go's
randomised map iteration and a catalog with two faults would report a different message on
every run. `internal/manifest` sorts its source names for the same reason. The specs pin
one message per single-fault fixture, which no amount of sorting would guarantee on a
multi-fault file — sorting is what makes it guaranteed, and it costs one `sort.Strings`.

**`to` is normalised to `[]string` at parse, and to nothing else.** A one-element list is
the simplest representation that lets `destination-and-plan` treat both spellings with one
code path, and it discards nothing: the strings are carried exactly as written, with
`{name}` uninterpolated and trailing slashes intact. Cleaning or interpolating here would
put destination semantics in two packages.

**Globs are `path.Match` over the name only.** SPEC.md places the glob "in the name
position", so the kind is compared literally. `path.Match` is the standard library's
answer, needs no dependency, and gives `*`, `?`, and character classes with documented
semantics. `path.Match`'s one surprising rule — that `*` does not cross a separator —
cannot bite here, but only because parsing makes it so: `plainName` in `items.go` refuses
an item name holding `/`, so the premise this matcher rests on is enforced at the point a
catalog mints an id rather than assumed. Its `ErrBadPattern` becomes the malformed-pattern
error rather than being swallowed into a silent no-match, because a selector like
`agent:[tdd` is a typo and typo protection is the point of the no-match rule.
*Alternative rejected:* treating a bad pattern as a literal name, which would report "no
match" for a selector that is not a name at all.

**`Expand` takes a source name, not a `manifest.Source`.** The dependency would point the
wrong way — `catalog` describes a source repository's offer and has no business knowing the
consumer's file format — and the name is needed only to make the error message locate the
problem. `catalog` therefore imports `itemid` and nothing else of ours.

**Every selector must match, and the result is the deduplicated union sorted by id.**
Per-selector checking is what SPEC.md's "a selector matching nothing is an error" means:
a union check would let `agent:*` cover for a misspelled `schema:tdd-workflwo`. Sorting by
id rather than preserving `install` order is what keeps the lock's item order — also by id
— independent of how a consumer wrote its manifest.

**`from` must be a cleaned relative path with no `..` segment, and `.` is rejected.** This
is SPEC.md's "`from` stays inside the source tree" invariant, enforced at the door for the
same reason `internal/lock` enforces it on `files`: the rule is worthless if it is checked
somewhere later, after the value has been joined to a real path. `.` is rejected because it
names the whole source tree, and requiring cleaned form rejects aliases such as
`./extras/tdd` that would otherwise defeat any later comparison.

**A kind declared but never provided is not an error.** A source may declare `hook` and ship
no hooks yet. The reverse — an item naming an undeclared kind — is an error, because that
item has no destination and could never be installed.

## Risks / Trade-offs

- **A generic-`any` parser is more code than a typed decode**, and each field's extraction
  is hand-written → every extraction path has a table case with an exact error message, and
  the coverage gate is measured over exactly this package.
- **Asserted error strings are brittle on purpose**, so a wording tweak breaks tests → that
  is the intended cost; tasks.md carries a contract gate so a message change is a deliberate
  act and SPEC.md is re-read alongside it.
- **`path.Match`'s character classes are a wider grammar than SPEC.md advertises** — SPEC.md
  names only `*`-style globs, but `agent:[ab]*` will work → accepted, because narrowing it
  would mean writing a matcher, and the extra syntax fails safe: an unintended class still
  either matches items or produces the no-match error.
- **The second module dependency arrives here** → maintained, single-purpose, zero
  transitive dependencies, pinned in `go.mod`, covered by Dependabot.
- **An item name is chosen by the source repository and reaches three places that read it
  differently** — `path.Match` as a selector target, `{name}` as a destination fragment, and
  the lock as an id → `plainName` restricts a name to letters, digits, dot, dash, and
  underscore, and refuses `.` and `..` outright, so a name can neither hide from a `kind:*`
  selector, nor collide with a neighbour through a glob metacharacter, nor steer a write
  above the destination its own `kind` declared. The rule lives in `internal/catalog`
  rather than `internal/itemid`: `itemid` says what an id *is*, and all three files that
  carry one have to agree on that, whereas this says what a source may *publish* — and a
  catalog is the only thing that ever mints an id.
- **A `from` path could still leave the source tree through a symlink**, since containment is
  checked on the string and nothing has stat'd the repository → not addressed here, and
  deliberately: this change reads no file from a source tree, so there is no point at which a
  link could be resolved. It belongs to whichever change first opens a `from` path, alongside
  `destination-and-plan`'s repo-root invariant.
- **`catalog.yaml` is source-controlled input**, so a hostile catalog is the realistic threat
  → this change executes nothing from it, and the containment rule on `from` plus the
  undeclared-kind rule are checked before any consumer sees a value. The destination side of
  the threat is `destination-and-plan`'s repo-root invariant, which is why this change does
  not claim to have closed it.

## Migration Plan

None needed. There is no released `graft`, no `catalog.yaml` in any repository graft reads,
no datastore, and no deploy order — the change adds one package and one module dependency
and wires neither to a command. Rollback is reverting the commits.

## Open Questions

- **SPEC.md does not say whether the glob may appear in the kind position.** It says "a glob
  in the name position (`agent:*`, `agent:outside-in-*`)" and gives no example of a globbed
  kind. Resolved here in favour of a literal kind, which is the reading most consistent with
  the rest of SPEC.md: `graft list` groups by source and item, `graft add` offers to collapse
  a selection to `kind:*`, and the lock records ids — all of which treat the kind as a name a
  human types, never a pattern. `*:tdd` therefore falls out as a no-match error rather than
  needing a rule of its own.
- **SPEC.md does not say whether a catalog may declare a kind it never provides.** Allowed
  here, on the grounds that rejecting it would be a rule SPEC.md does not state and that a
  source adding a kind before its first item is ordinary.
- **Should `provides[].from` be allowed to name a path that does not exist in the source
  tree?** Not answerable here — this change never sees the tree. `destination-and-plan` or
  `sync-command` meets the real files and owns the missing-`from` failure.
- **Does `catalog.yaml` need a `requires` field?** SPEC.md's own open question, and a step
  toward dependency resolution, which the PRD lists as a non-goal. Left open; this change
  rejects `requires` as an unknown key at version 1, which is the behavior that keeps the
  question askable later.
- **Should a catalog be allowed to declare zero kinds while providing items?** It cannot: an
  item naming an undeclared kind is an error, so a catalog with items and no `kinds` fails on
  the first item. Recorded because the failure arrives from the item rule rather than from a
  rule about `kinds` itself.
