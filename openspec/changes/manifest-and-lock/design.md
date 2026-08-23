## Context

`graft` is scaffolding: `cmd/graft` prints a version and `internal/buildinfo` is the only
package. SPEC.md documents `graft.toml` and `graft.lock` down to their bytes, and
`openspec/IMPLEMENTATION-ORDER.md` puts this change first in Phase 1 with nothing depending
on it and two changes depending on it.

Two constraints shape everything below.

1. **`internal/apply` is the only package permitted to write to the working tree.** So this
   change reads files and returns bytes; it never writes one.
2. **`graft.lock` is committed and reviewed as a git diff.** Its serialization is not an
   implementation detail — a writer that reorders or repads between runs makes every sync a
   noisy diff and destroys the review story the PRD's success criteria rest on. That is why
   the headline requirement is byte equality, not semantic equality.

The module currently has zero dependencies. TOML decoding is the first one it needs.

## Goals / Non-Goals

**Goals:**

- `internal/manifest` loads and validates `graft.toml` into a value the later phases consume.
- `internal/lock` loads and validates `graft.lock`, and serializes a lock to canonical bytes.
- Round trip is byte-stable: canonical bytes → parse → serialize returns the identical bytes,
  and serializing one lock value twice returns identical bytes.
- Non-canonical but valid input normalizes to canonical bytes exactly once.
- Every error message in the two packages is fixed text a test can assert.

**Non-Goals:**

- Writing any file. No `Save`, no `os.WriteFile`, nowhere.
- Serializing `graft.toml`. `add-command` owns manifest amendment.
- Selector *matching*. Syntax only; globbing against `provides` is `catalog-and-selectors`.
- Destination computation, the prune set, and the path-escape and collision invariants over
  computed destinations — `destination-and-plan`.
- Git, network, rev resolution, the fetch cache — `git-fetch`.
- Any CLI surface: no command, flag, exit code, or stderr formatting. `command-surface` owns
  how these errors reach a terminal.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/manifest` | **new** | `Parse([]byte, filename) (*Manifest, error)` plus a thin `Load(path)`. Pure validation over decoded structs. |
| `internal/lock` | **new** | `Parse`, `Load`, `Marshal(*Lock) []byte`, and `CheckPins(manifest.Sources, *Lock) error`. Imports `internal/manifest` for the pin check; the dependency runs one way only. |
| `internal/catalog` | untouched | `catalog-and-selectors`. |
| `internal/source` | untouched | `git-fetch`. |
| `internal/plan` | untouched | This change adds **no** code to `plan` and therefore cannot compromise its purity. |
| `internal/apply` | untouched | This change adds **no write path**. `lock.Marshal` returns `[]byte`; `sync-command` is where `internal/apply` writes those bytes to `graft.lock`, last, after every file operation succeeds. A writer here would put the tool's only mutation outside the one package allowed to mutate. |
| `cmd/graft` | untouched | Nothing is wired to a command yet, and coverage is measured over `./internal/...` only — logic placed in `cmd/graft` would be invisible to the gate. |

Each package follows the same shape as `internal/buildinfo`: a small exported surface, no
package-level state, no `init()`, errors returned rather than logged.

## Contracts

**External consumers: none.** No released `graft` reads either file, so both formats are
*introduced* here rather than changed. `graft.lock`'s `version` is established at `1`; it does
not move. Nothing in this change is **BREAKING**.

**Internal consumers**, all downstream changes:

- `manifest.Manifest` — `Sources []Source`, each with `Name`, `Git`, `Rev`, `Install []string`,
  `Kinds map[string]string`. `Git` is the verbatim string; expansion to a clone URL is
  `internal/source`'s job in `git-fetch`, which also needs host/owner/repo for the cache path.
- `lock.Lock` — `Version int`, `Sources []Source`, each with `Name`, `Git`, `Rev`, `Resolved`,
  `Items []Item{ID string, Files []string}`.
- Error surface: every validation failure is a plain `error` whose `Error()` is the exact
  string the spec names, prefixed with the file it came from. TOML syntax failures are wrapped
  as `graft.toml: <decoder message>` / `graft.lock: <decoder message>`; only the prefix is a
  fixed contract, because the decoder's own text is not ours to pin. No sentinel error values
  and no error types are exported until a caller needs to branch on one.
- Absence is asymmetric and is part of the contract: `manifest.Load` on a missing file is an
  error; `lock.Load` on a missing file returns an empty lock at version 1.

## Visual Design

Not applicable. This change builds no user-facing view and no email template — it adds two
non-visual Go packages with no CLI surface — so there is no design source to import and this
section is deliberately empty rather than invented.

## Persistence and Rollout

- Migration: none. No datastore.
- Backfill: none.
- Seeding: none.
- Cache invalidation: none — the fetch cache does not exist yet and is not read here.
- Index rebuild: none.
- Authorization: none. graft has no accounts and no auth layer, and this change reads two
  files in the repo it runs in.
- Observability: none. There is no telemetry of any kind, by design.
- Deployment: none. Nothing is wired to a command, so building this changes no released
  behavior. `go.mod` gains `github.com/BurntSushi/toml v1.6.0`, the module's first dependency;
  Dependabot already covers Go modules.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem | no acceptance test in this change | **real**, via `t.TempDir()`, and only for `Load` — every parse, validate, and serialize test passes `[]byte` and touches no disk |
| `git` binary | no acceptance test in this change | **not involved** — this change runs no git command and needs no fixture repository, so the `user.name`/`user.email`-on-the-repo hazard does not arise here |
| Network (runtime) | no acceptance test in this change | **not involved** — no code path resolves, clones, or fetches; no test opens a socket |
| Go module proxy (build time) | no acceptance test in this change | **real, once** — tasks.md group 1 runs `go get` and `go mod tidy` against it. After that, `go.sum` pins the dependency and the suite needs no network |
| Fetch cache (`~/.cache/graft/`) | no acceptance test in this change | **not involved** — never read, never created; no test may reference `$HOME` |
| TOML decoder (`github.com/BurntSushi/toml`) | no acceptance test in this change | **real** — the library is the unit under test as much as our code is, and its `MetaData.Undecoded()` is what makes unknown-key rejection work |
| Lock serializer ↔ lock parser | no acceptance test in this change | **both real, against each other** — the round-trip scenarios are the only check that the hand-written writer and the library-backed parser agree |
| Golden fixtures (`internal/lock/testdata/`) | no acceptance test in this change | **real files, read only** — `canonical.lock` and `scrambled.lock` are read with `os.ReadFile`; they are the only fixtures on disk and nothing writes them at test time |
| `internal/plan`, `internal/apply` | no acceptance test in this change | **not involved** — neither is imported |
| Clock, environment variables, working directory | no acceptance test in this change | **not involved** — no output depends on time, `$HOME`, `NO_COLOR`, or the process CWD; every path in a test is absolute or explicitly relative to a `t.TempDir()` |

## Test Strategy

Two tiers only.

- **unit** — `[]byte` in, value or error out. No filesystem. Run with
  `go test ./internal/manifest/... ./internal/lock/...`.
- **fs** — a `t.TempDir()` with real files, exercising `Load` and absence. Still fast, still
  under `./internal/...`, still no git and no network. Same command.

Full gate is `task test` (race detector) and `task cover` (80% floor over `./internal/...`).

**No outer-loop acceptance test.** This change alters no end-to-end command behavior: it adds
no command, no flag, and no output. There is nothing to drive from the outside, so `tasks.md`
deletes the outer-loop acceptance group rather than inventing a CLI to justify it. The first
acceptance test in this project belongs to `sync-command`.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| manifest-format · Minimal valid manifest loads | `TestParse_Minimal` asserts every field and selector order | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Manifest with no sources is valid | `TestParse_Empty` asserts zero sources, nil error | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Missing manifest is an error | `TestLoad_Missing` in an empty `t.TempDir()`, asserts exact text | fs | filesystem real | `go test ./internal/manifest/...` |
| manifest-format · Malformed TOML is an error | `TestParse_BadTOML` asserts the `graft.toml: ` prefix | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Missing git is an error | table case in `TestParse_Errors`, exact text | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Missing rev is an error | table case in `TestParse_Errors`, exact text | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Empty install list is an error | table case, both `install = []` and absent | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Empty source name is an error | table case, `[sources.""]` | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · Plain and glob selectors are accepted | `TestParse_Selectors` asserts verbatim, unexpanded, unreordered | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · A selector with no kind separator is an error | table case in `TestParse_SelectorErrors` | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · A selector with an empty half is an error | table case, three inputs, exact text each | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · A duplicate selector is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · An override is carried verbatim | `TestParse_KindOverride` asserts trailing slash preserved | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · No kinds table means no overrides | same test, asserts zero overrides | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · An empty override destination is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · A misspelled source field is an error | `TestParse_UnknownKey`, exact text | unit | TOML decoder real (`Undecoded()`) | `go test ./internal/manifest/...` |
| manifest-format · An unknown top-level key is an error | same test, `version = 1` case | unit | TOML decoder real (`Undecoded()`) | `go test ./internal/manifest/...` |
| manifest-format · Shorthand is not expanded | `TestParse_GitVerbatim` compares to the input string | unit | TOML decoder real | `go test ./internal/manifest/...` |
| manifest-format · A full URL is not rewritten | same test, scp-style URL case | unit | TOML decoder real | `go test ./internal/manifest/...` |
| lock-format · An absent lock loads as an empty lock | `TestLoad_Absent` in `t.TempDir()`; asserts version 1, zero sources, and that no `graft.lock` appeared | fs | filesystem real | `go test ./internal/lock/...` |
| lock-format · A lock with zero sources loads | `TestParse_HeaderOnly` | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A populated lock loads | `TestParse_Populated` asserts sources, items, file lists | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · Malformed TOML is an error | `TestParse_BadTOML` asserts the `graft.lock: ` prefix | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A missing version is an error | table case in `TestParse_VersionErrors`, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A newer version fails and says to upgrade | table case, exact text, and asserts no sources returned | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A version below 1 is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A missing resolved sha is an error | table case in `TestParse_SourceErrors` | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A malformed resolved sha is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A duplicate source name is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A malformed item id is an error | table case in `TestParse_ItemErrors` | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A duplicate item id within a source is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · An item with no files is valid | `TestParse_ItemNoFiles` asserts success and zero files | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · An escaping file path is an error | table case, both `../outside.md` and `/etc/passwd`, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · A duplicate file within an item is an error | table case, exact text | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · An unknown key is an error | `TestParse_UnknownKey`, exact text | unit | TOML decoder real (`Undecoded()`) | `go test ./internal/lock/...` |
| lock-format · A populated lock serializes to the documented layout | `TestMarshal_Golden` compares against `testdata/canonical.lock` byte for byte | unit | golden fixture real, read only | `go test ./internal/lock/...` |
| lock-format · A files list of one is inline and a list of many is exploded | `TestMarshal_FilesLayout` with 0, 1, and 2 files | unit | none | `go test ./internal/lock/...` |
| lock-format · An empty lock serializes to header and version only | `TestMarshal_Empty` compares exact bytes | unit | none | `go test ./internal/lock/...` |
| lock-format · A path needing escaping round-trips | `TestMarshal_Escaping` asserts the quoted form, then re-parses and compares the path | unit | TOML decoder real | `go test ./internal/lock/...` |
| lock-format · Input order does not change output | `TestMarshal_OrderIndependent` builds ascending and reversed, compares bytes | unit | none | `go test ./internal/lock/...` |
| lock-format · Serializing the same lock twice is byte-identical | `TestMarshal_Twice` uses `bytes.Equal` | unit | none | `go test ./internal/lock/...` |
| lock-format · Canonical bytes survive a parse and serialize | `TestRoundTrip_Canonical` against `testdata/canonical.lock` | unit | golden fixture real, TOML decoder real | `go test ./internal/lock/...` |
| lock-format · Non-canonical input is normalized, then stable | `TestRoundTrip_Normalizes` against `testdata/scrambled.lock`, then re-marshals the result | unit | golden fixtures real, TOML decoder real | `go test ./internal/lock/...` |
| lock-format · Agreeing pins pass | table case in `TestCheckPins` | unit | `internal/manifest` real | `go test ./internal/lock/...` |
| lock-format · A moved manifest pin is an error | table case, exact text | unit | `internal/manifest` real | `go test ./internal/lock/...` |
| lock-format · A source only in the manifest is not an error | table case | unit | `internal/manifest` real | `go test ./internal/lock/...` |
| lock-format · A source only in the lock is not an error | table case | unit | `internal/manifest` real | `go test ./internal/lock/...` |
| lock-format · Two empty files agree | table case, both empty | unit | `internal/manifest` real | `go test ./internal/lock/...` |

## Decisions

**Decode with `github.com/BurntSushi/toml` v1.6.0; write the lock by hand.** Hand-rolling a
TOML *parser* would be reckless for a format with quoted keys, escapes, and array-of-table
nesting. Writing the lock by hand is the opposite: SPEC.md documents a specific layout —
`=` aligned per table, `[[source.item]]` indented two spaces, single-element `files` inline
and multi-element exploded with a trailing comma — and no general encoder emits that. A
hand-written writer also makes determinism structural rather than something we hope the
library preserves. *Alternatives:* `pelletier/go-toml/v2` (equally capable decoder, no
advantage here, and its `Marshal` no more matches the documented layout); encoder output
plus a documentation edit to SPEC.md (rejected — the layout is deliberate and the diff
readability it buys is the point); a full hand-written parser (rejected, disproportionate
risk). `MetaData.Undecoded()` is the deciding feature: it is what turns "unknown key" into
an error instead of a silent typo.

**Split `Parse([]byte, filename)` from `Load(path)`.** Every validation rule is then a pure
function of bytes, so the whole error table is unit-testable without a temp directory, and
the filesystem appears in exactly two tests per package. `filename` exists only to build the
`graft.toml: ` / `graft.lock: ` error prefix, so the prefix does not depend on where the file
lives.

**`Marshal` returns `[]byte`; nothing writes.** `internal/apply` is the only writer, and the
lock is written last, after every file operation succeeds — a `lock.Save` would make that
ordering unenforceable from `apply`. This change adds no write path at all.

**No `graft.toml` serialization in this change.** IMPLEMENTATION-ORDER's row says "parsing,
validation, and deterministic serialization" for both files, but SPEC.md states ordering and
determinism rules only for the lock, and `graft.toml` is human-authored with comments and
formatting worth preserving. `add-command` explicitly delivers "manifest amendment" and is
the first caller that has to answer "rewrite or edit in place". Writing a manifest encoder
now would be speculative code that the coverage floor then forces tests for.

**The pin-drift check lives in `internal/lock`, not `internal/plan`.** It needs only a
manifest and a lock, both owned by this change, and no later change claims it —
`destination-and-plan` lists destinations, the prune set, path escape, and collisions, and
this is none of those. Putting it in `plan` would mean a stub package existing only to hold
one comparison. `lock` importing `manifest` keeps the dependency pointing the way the domain
does: the lock records what the manifest requested.

**Items sort by id, even though SPEC.md's example does not.** SPEC.md's `graft.lock` example
prints `schema:tdd` before `agent:apply-orchestrator` while the bullet beneath it says
"Sources sorted by name, items by id, files by path". The explicit rule wins over the
illustration: the rule is the invariant the determinism requirement rests on, the example is
prose. See Open Questions.

**Selector syntax is validated here, matching is not.** Rejecting `"tdd"` needs no catalog
and is a property of `graft.toml` itself. Rejecting `"agent:nonexistent"` needs `provides`
and is `catalog-and-selectors`. Duplicate selectors within one source are an error for the
same reason a non-matching selector is: SPEC.md treats the install list as typo-protected,
and silently deduplicating hides the typo.

**`resolved` must be 40 lowercase hex characters.** SPEC.md records it as "the SHA it became"
and shows a full one; a rev string leaking into `resolved` is a corruption worth catching at
the door.

**`files` may be empty, but each path must be relative and free of `..`.** An item that
produced no files is representable rather than an error. A path that escapes, though, is a
deletion aimed outside the repo — the lock's `files` list is precisely what authorises
removal, so a hand-edited or corrupt lock must not be able to point it anywhere. This is
validation of the lock file's own contents and is distinct from `destination-and-plan`'s
invariant over *computed* destinations, which still needs its own check.

**`graft.toml` carries no `version`.** ENGINEERING.md names `catalog.yaml` and `graft.lock`
as the versioned formats and pointedly omits `graft.toml`. Strict decoding therefore rejects
a top-level `version` key rather than accepting and ignoring it.

## Risks / Trade-offs

- **The hand-written writer and the library-backed parser could disagree**, producing a lock
  graft cannot read back → the two round-trip scenarios exist for exactly this, and the
  golden-file test pins the bytes so a writer change cannot pass unnoticed.
- **Asserted error strings are brittle on purpose**, so a wording tweak breaks tests → that is
  the intended cost; tasks.md carries a contract gate so a message change is a deliberate act
  and SPEC.md is re-read alongside it.
- **Alignment and blank-line rules are invisible to semantic assertions** → the golden file is
  compared as bytes, not parsed and compared as a value.
- **The first module dependency arrives here** → a mature, single-purpose, widely used
  decoder with no transitive dependencies, pinned in `go.mod` and covered by Dependabot.
- **The strictness could be wrong in a direction we only learn later** — for example
  `resolved` if git repositories move to SHA-256 object ids → the rule is one function and one
  error string, and the failure is loud rather than silent.

## Migration Plan

None needed. There is no released `graft`, no existing `graft.lock` in any repository, no
datastore, and no deploy order — the change adds two packages and one module dependency and
wires neither to a command. Rollback is reverting the commits.

## Open Questions

- **SPEC.md contradicts itself on lock item order.** The example lists `schema:tdd` before
  `agent:apply-orchestrator`; the rule directly beneath says items sort by id, which would put
  `agent:apply-orchestrator` first. Resolved here in favour of the explicit rule, because
  determinism is the invariant and the example is illustrative. tasks.md's documentation group
  reorders the two items in that example so SPEC.md stops contradicting itself; no rule text
  changes.
- **IMPLEMENTATION-ORDER's row reads "parsing, validation, and deterministic serialization"
  for both files.** Read here as: parsing and validation for both, serialization for the lock
  only. Recorded rather than silently narrowed, and revisited by `add-command`.
- **Should `resolved` accept a 64-character SHA-256 object id?** Deferred: git's SHA-256 mode
  is not something graft has met, `git-fetch` produces the value, and widening the rule later
  is a one-line change with a test.
- **Is a lock item with zero `files` ever legitimate?** Allowed here on the grounds that
  rejecting it would be a rule SPEC.md does not state. If `destination-and-plan` finds it
  cannot arise, tightening it there is cheap.
- **Should `manifest` reject a `kinds` override whose destination escapes the repo root?**
  Left to `destination-and-plan`, which owns that invariant over resolved destinations and
  must check it anyway for catalog-supplied destinations.
