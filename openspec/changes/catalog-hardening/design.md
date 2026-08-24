## Context

`internal/catalog` landed in `catalog-and-selectors` and is complete against its own specs:
`Parse` decodes `catalog.yaml` generically and validates it, `Load` reads a path and
delegates, `Expand` matches selectors. Its review pass produced four findings that were
recorded, argued, and deliberately deferred — all four with the same reason, written into
that change's `tasks.md` §9.4: nothing in the shipped binary fetches a source tree, so
`Load` is unreachable and none of the four can be triggered by running `graft`.

That reason has an expiry date written into it: **"all four to be closed before
`sync-command` makes `Load` reachable."** `git-fetch` has since landed, so the fetched tree
exists and `internal/source.ReadCatalog` already reads a source's `catalog.yaml` — through an
`os.Root`, handing the bytes to `catalog.Parse` and delegating to `Load` only on the absence
path. `sync-command` is the change that first calls `ReadCatalog` from a command.

That detail matters for what each finding is worth, and the four are not equal:

- Findings 1, 3, and 4 live in `Parse` and become **user-reachable** the moment
  `sync-command` runs, whatever path the bytes arrived by.
- Finding 2 lives in `Load`, and the `os.Root` in `ReadCatalog` already refuses an escaping
  link — verified: `openat catalog.yaml: path escapes from parent`. Closing it is hardening
  an exported entry point whose doc comment claims something untrue, not repairing a
  reachable leak. It is worth doing for that reason and is not dressed up as more.

Three constraints shape everything below, all inherited rather than chosen here.

1. **`catalog.yaml` is written by a *source* repository.** It is the least trusted of
   graft's three files: a human on the other side of a git URL writes it and graft acts on
   it. Two of the four findings are silent failures, and a silent failure on untrusted input
   is the worst shape a fault can take.
2. **`internal/apply` is the only package permitted to write, and `internal/plan` is pure.**
   This change adds no write path and no code to either.
3. **Error strings are the contract.** `openspec/config.yaml` makes error text part of the
   specification, so re-routing a message is a spec change — which is why four findings
   inside one package are a change proposal rather than a patch.

`internal/plan` already exists and already consumes `catalog.Kind`. That is not incidental:
its `destination-computation` spec settles what a trailing slash means, and finding 4's
repair has to agree with it (D7).

## Goals / Non-Goals

**Goals:**

- A `catalog.yaml` holding more than one YAML document is refused instead of half-read.
- `Load` reads the path it was given, and nothing else.
- A catalog written for a future graft is answered with "upgrade graft" however wide its
  `version` literal is.
- A kind cannot declare one destination twice under two spellings.
- Every message above is fixed text a test asserts, and every message this change does not
  deliberately move is unchanged.

**Non-Goals:**

- **No new capability and no new package.** Every requirement is `catalog-format`'s.
- **No symlink resolution inside a source tree.** `from` is still checked as a string;
  `catalog-and-selectors` → design.md → Risks records that as belonging to whichever change
  first opens a `from` path. This change hardens the one path `Load` opens.
- **No destination computation.** `{name}`, trailing-slash semantics, and `flatten` stay in
  `internal/plan`. This change compares two `to` entries with each other and interprets
  neither.
- **No re-opening of the two rejections in `catalog-and-selectors` → tasks.md §9.4**: item
  name validation does not move to `internal/itemid` (an allowlist there rejects the legal
  selector `agent:*` and fails six `internal/manifest` tests), and `Expand` gains no
  nil-receiver guard.
- **No fetching, no writing, no CLI surface.**

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/catalog` | **modified** | `catalog.go`: `Load` gains an `os.Lstat` regular-file check; `Parse` counts documents over the file's tokens before decoding and refuses more than one, keeping `yaml.Unmarshal` for the decode itself; `checkVersion` gains a `string` case reading the literal's shape. `kinds.go`: the duplicate-destination guard keys on a path identity instead of the raw string. No exported signature changes and no new exported symbol. Two subpackages of the existing dependency are imported (`goccy/go-yaml/lexer`, `.../token`); `go.mod` is unchanged. |
| `internal/plan` | untouched | Adds no code. Its `destKey` already distinguishes a trailing slash for a file item from one for a directory item, and D7 makes the catalog-level comparison agree with it rather than pre-empt it. No test in this change needs a real directory to exercise plan logic — no test in this change touches `plan` at all. |
| `internal/apply` | untouched | **No new write path.** Every change here is a read or a comparison: `Load` gains a stat, `Parse` gains a token scan, `checkVersion` and `destinations` gain branches. There is nothing to write — a catalog is an input, never an output. |
| `internal/source` | **untouched, one behavior moves** | `ReadCatalog` reads `catalog.yaml` through an `os.Root` over the fetched entry, so an escaping link is already refused there (`path escapes from parent`, verified) and an in-tree link is followed harmlessly — the source's own content either way. It delegates to `catalog.Load` on the absence path, and that delegation is where one observable message moves: a **dangling relative link** inside an entry makes `os.Root.ReadFile` return `fs.ErrNotExist`, so `ReadCatalog` delegates, and `Load` now answers `catalog.yaml: not a regular file` where it previously answered `catalog.yaml not found: the source is not graftable`. That is the better answer — the source did publish a `catalog.yaml` — and `source-listing`'s spec pins no message for it: its not-graftable scenario is a tree with *no* `catalog.yaml`, and its own requirement says an invalid catalog surfaces `internal/catalog`'s message. So no `source-listing` delta is needed; task 2.6 checks the case rather than leaving it to be discovered. The two rules stay siblings and are deliberately not collapsed — `source` polices what is inside a fetched tree, `catalog` polices the single path `Load` is handed, and `Load` is exported to callers `source` does not control. |
| `internal/manifest`, `internal/lock`, `internal/itemid`, `internal/ui` | untouched | Not imported by anything this change edits, beyond `itemid`'s existing use. |
| `cmd/graft` | untouched | No logic lands there, where the coverage gate cannot see it. |

## Contracts

**External consumers: none.** No `graft` is released and no repository publishes a
`catalog.yaml` for graft to read. `catalog.yaml`'s `version` stays at `1`: every catalog
that parses today parses unchanged unless it is one of the four faults, and each of those is
a file that was already broken — a dropped document, a link, an unreadable version, a
doubled destination.

**Internal consumers.** `internal/plan` is the only caller of `catalog.Catalog`, `Kind`, and
`Item`. None of the three types changes: `Kind.To` still carries every destination verbatim,
`Items` is still sorted by id, `Version` is still an `int`. `Load`, `Parse`, and `Expand`
keep their signatures. What changes is which inputs are refused and which message a refusal
carries.

**Error surface.** Two new messages and one re-route:

| Message | Site | Status |
|---|---|---|
| `catalog.yaml: multiple YAML documents; a catalog is a single document` | `Parse` | new |
| `catalog.yaml: not a regular file` | `Load` | new |
| `catalog.yaml: version <literal> is not supported by this graft; upgrade graft` | `checkVersion` | existing text, reached by a new input |
| `catalog.yaml: version <literal> is not a known catalog version` | `checkVersion` | existing text, reached by a new input |
| `catalog.yaml: kind "<kind>": duplicate destination "<b>": same path as "<a>"` | `destinations` | new, alongside the existing single-string form |

Every other message in the package is unchanged, including
`catalog.yaml: version must be an integer`, which keeps exactly the inputs it has today plus
the shapes D5 declines to treat as versions.

**Effect on `graft.lock`'s format: none, and `version` does not move.** This change writes
no lock, reads no lock, and adds no field to one. It refuses some catalogs that previously
parsed; a refused catalog produces no items, so no lock this change influences can hold an
id or a path the current lock parser would reject.

## Persistence and Rollout

- Migration: none. No datastore.
- Backfill: none.
- Seeding: none.
- Cache invalidation: none. The fetch cache is content-addressed by SHA and holds source
  trees, not parsed catalogs; nothing this change touches is cached.
- Index rebuild: none.
- Authorization: none. graft has no accounts and no auth layer.
- Observability: none. There is no telemetry of any kind, by design.
- Deployment: none. No command reads a catalog yet, so building this changes no released
  behavior. `go.mod` is untouched — the multi-document check uses the decoder already
  present.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| Filesystem | no acceptance test in this change | **real**, via `t.TempDir()`, and only for `Load` — including `os.Symlink` for the two link cases and `os.Mkdir` for the directory case. Every parse, version, and kind test passes `[]byte` and touches no disk |
| Symlink support in the test filesystem | no acceptance test in this change | **real** — `os.Symlink` in a `t.TempDir()`. darwin and linux are the only supported platforms (ENGINEERING.md → Compatibility), and both provide it unprivileged; no test skips on its absence |
| `git` binary | no acceptance test in this change | **not involved in any test this change writes** — no test here runs a git command or builds a fixture repository, so the `user.name`/`user.email`-on-the-repo hazard does not arise in new code. Tasks 2.6, 4.9 and 7.3 run the existing `internal/source` suite, which does build fixture repositories; running a neighbouring suite unchanged is a command, not a collaborator this change adopts |
| Network | no acceptance test in this change | **not involved** — no code path resolves, clones, or fetches; no test this change writes opens a socket; `go.mod` gains nothing, so not even the module proxy is contacted |
| Fetch cache (`~/.cache/graft/`) | no acceptance test in this change | **not involved in any test this change writes** — never read, never created, and no new test references `$HOME`. The existing `internal/source` suite does read it, under the same carve-out as the `git` row |
| YAML lexer and decoder (`github.com/goccy/go-yaml`, and its `lexer` and `token` subpackages) | no acceptance test in this change | **real** — the token stream's document markers and the decoder's choice of Go type per literal are both under test here as much as our code is. Both subpackages ship inside the module already pinned in `go.mod` |
| `internal/itemid` | no acceptance test in this change | **real** — unchanged, still called by item parsing |
| `internal/plan`, `internal/apply`, `internal/manifest`, `internal/lock`, `internal/ui` | no acceptance test in this change | **not involved** — none imported by `internal/catalog`, none edited, and no test in this change constructs one. `internal/plan`'s existing suite is run unchanged as a regression check (task 4.6), which is a command, not a collaborator |
| `internal/source` | no acceptance test in this change | **not involved as a collaborator, exercised as a regression** — not imported and not edited, but it calls `catalog.Load` on its absence path, so tasks 2.6 and 4.9 run its existing suite unchanged. That suite owns the `git` and `$HOME` rows above |
| `cmd/graft` binary | no acceptance test in this change | **not involved** — nothing is built or executed |
| Golden fixtures / `testdata` | no acceptance test in this change | **not involved** — every input is an inline YAML literal, as in `catalog-and-selectors` |
| Clock, environment variables, working directory, process umask | no acceptance test in this change | **not involved** — no output depends on any of them; every path in a test is inside a `t.TempDir()` |
| Running as root | no acceptance test in this change | **not involved by construction** — no test depends on a permission being *denied*, which is the one filesystem assertion a root CI runner would invert. The unreadable-file path is left as it is for exactly this reason (see Risks) |

## Test Strategy

Two tiers, as in `catalog-and-selectors`.

- **unit** — `[]byte` in, value or error out. No filesystem.
- **fs** — a `t.TempDir()` with real files and links, exercising `Load`.

Both run under `go test ./internal/catalog/...`; the full gate is `task lint`, `task cover`
(80% floor over `./internal/...`), and `task build`.

**No outer-loop acceptance test.** This change alters no end-to-end command behavior: no
command reads a catalog yet, so there is no entry point to drive from the outside and no
observable CLI change to assert. `tasks.md` therefore deletes the outer-loop acceptance
group rather than inventing a command to justify it. The first acceptance test that reaches
`Load` belongs to `sync-command`, which is also where these four failures would first have
become user-visible.

Rows marked **regression** are scenarios restated unchanged by a MODIFIED requirement; they
already have tests, and the tasks that touch their requirement re-run them rather than
rewriting them. There are 37 scenarios in the delta: 18 regressions and 19 new.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| A second document is an error | `TestParse_MultipleDocuments` case `two documents`, exact text | unit | YAML lexer real | `go test ./internal/catalog/ -run TestParse_MultipleDocuments` |
| Content after a separator is reported even when it is malformed | same test, case `malformed second document`, exact text — goes red against a decoder-based count, which reports the syntax error instead | unit | YAML lexer real | same |
| An empty document between two separators is still a document | same test, case `adjacent separators then content`, exact text — the case a decoder-based count returns `io.EOF` for, silently dropping the trailing mapping | unit | YAML lexer real | same |
| A file opening with two separators is more than one document | same test, case `two leading separators`, exact text — asserts specifically that it is not `version is required` | unit | YAML lexer real | same |
| A separator inside a scalar is not a separator | same test, cases `quoted marker` and `block scalar marker`, asserting the catalog parses | unit | YAML lexer real | same |
| A trailing separator with nothing after it is accepted | same test, cases `trailing separator` and `trailing separator then comment`, asserting a parsed catalog and nil error | unit | YAML lexer real | same |
| A leading separator is accepted, with or without a trailing one | same test, cases `leading separator` and `leading and trailing separator`, asserting a parsed catalog and nil error | unit | YAML lexer real | same |
| A valid catalog loads | `TestLoad_Valid` (regression, unchanged) | fs | filesystem real, YAML decoder real | `go test ./internal/catalog/ -run TestLoad` |
| A catalog with zero provides loads | `TestParse_Empty` (regression, unchanged) | unit | YAML decoder real | `go test ./internal/catalog/ -run TestParse_Empty` |
| A catalog with neither kinds nor provides loads | `TestParse_Empty`, `TestParse_NoKindsNoProvides` (regression, unchanged) | unit | YAML decoder real | `go test ./internal/catalog/ -run 'TestParse_Empty|TestParse_NoKindsNoProvides'` |
| A missing catalog is the not-graftable error | `TestLoad_Missing` (regression, unchanged) — the `Lstat` guard must keep returning this exact text for an absent file | fs | filesystem real | `go test ./internal/catalog/ -run TestLoad_Missing` |
| A symlinked catalog is refused without being read | `TestLoad_NotARegularFile` case `symlink`, exact text, and asserts the link target's distinctive content appears nowhere in the message | fs | filesystem real (`os.Symlink`) | `go test ./internal/catalog/ -run TestLoad_NotARegularFile` |
| A dangling symlink is refused as a link, not as an absence | same test, case `dangling symlink`, exact text, asserts it is not the not-graftable message | fs | filesystem real (`os.Symlink`) | same |
| A directory named catalog.yaml is refused | same test, case `directory`, exact text; `TestLoad_Unreadable` is kept and stays green on its prefix-and-not-graftable assertions | fs | filesystem real | same |
| Malformed YAML is an error | `TestParse_BadYAML` (regression, unchanged) — asserts the prefix only, and must keep reporting the decoder's message rather than the multiple-documents one, since a single malformed document is one document | unit | YAML lexer and decoder real | `go test ./internal/catalog/ -run TestParse_BadYAML` |
| A catalog that is not a mapping is an error | `TestParse_Shape` (regression, unchanged) | unit | YAML decoder real | `go test ./internal/catalog/ -run TestParse_Shape` |
| An empty catalog file is a missing-version error | `TestParse_Shape` case `an empty file is a missing version` (regression) — a file with no content tokens is zero documents, so the count must not turn it into a multiple-documents error | unit | YAML lexer and decoder real | same |
| A missing version is an error | `TestParse_VersionErrors` (regression, unchanged) | unit | YAML decoder real | `go test ./internal/catalog/ -run TestParse_Version` |
| A newer version fails and says to upgrade | `TestParse_VersionErrors` (regression, unchanged) | unit | YAML decoder real | same |
| A version below 1 is an error | `TestParse_VersionErrors` (regression, unchanged) | unit | YAML decoder real | same |
| A version literal wider than any integer type says to upgrade | new case in `TestParse_VersionErrors`, exact text with the literal as written | unit | YAML decoder real | same |
| A sign or separators do not change the answer | new cases in `TestParse_VersionErrors` for `+99…` and the quoted `"99…"`, exact text each — they also cover the sign-stripping branch the coverage note names | unit | YAML decoder real | same |
| A hugely negative version literal is not a known version | new case in `TestParse_VersionErrors`, exact text | unit | YAML decoder real | same |
| A quoted version is not an integer | existing case in `TestParse_VersionErrors` — kept, and it is the assertion that pins the shape rule against over-reach | unit | YAML decoder real | same |
| A version that is neither an integer nor an integer literal is an error | existing `1.5` case plus new `true`, `"…x"`, and quoted `"-1"` cases, one message — the last covers the negative-that-fits branch | unit | YAML decoder real | same |
| A string-valued to is carried verbatim | `TestParse_Kinds` (regression, unchanged) | unit | YAML decoder real | `go test ./internal/catalog/ -run TestParse_Kind` |
| A trailing slash is preserved | `TestParse_Kinds` (regression, unchanged) | unit | YAML decoder real | same |
| An uncleaned destination is carried verbatim | new case in `TestParse_Kinds`, asserts `To[0]` byte for byte | unit | YAML decoder real | same |
| A list-valued to is carried in declared order | `TestParse_Kinds` (regression, unchanged) | unit | YAML decoder real | same |
| An empty kind name is an error | `TestParse_KindErrors` (regression, unchanged) | unit | YAML decoder real | same |
| A missing or empty to is an error | `TestParse_KindErrors` (regression, unchanged) | unit | YAML decoder real | same |
| An empty destination inside a list is an error | `TestParse_KindErrors` (regression, unchanged) | unit | YAML decoder real | same |
| A to of the wrong type is an error | `TestParse_KindErrors` (regression, unchanged) | unit | YAML decoder real | same |
| A repeated destination within one kind is an error | `TestParse_KindErrors` (regression, unchanged) — pins the one-string form the new form must not replace | unit | YAML decoder real | same |
| Two spellings of one destination are an error naming both | new case in `TestParse_KindErrors`, exact text naming both spellings | unit | YAML decoder real | same |
| A dot-slash prefix is the same destination | new case in `TestParse_KindErrors`, exact text | unit | YAML decoder real | same |
| A trailing slash makes two destinations, not one | `TestParse_KindTrailingSlashIsNotADuplicate`, asserts both entries survive in declared order | unit | YAML decoder real | same |

## Decisions

**D1 — A second document is refused, not merged and not preferred.** The alternatives are
merging every document into one catalog and taking the last. Both invent semantics
`catalog.yaml` does not have: SPEC.md describes one document with three top-level keys, and
a source author who writes two has made a mistake in one direction or the other. Refusing
says so once; either alternative would install something nobody wrote.

**D2 — The count is taken from the token stream, before decoding, and not from the
decoder.** `lexer.Tokenize` gives the file's tokens; document markers (`DocumentHeaderType`
for `---`, `DocumentEndType` for `...`) split it into regions; a region holding anything but
comments is a document, and so is an empty region *between* two markers. An empty region
before the first marker or after the last is not, which is what makes a leading separator, a
trailing separator, and both together all one document. This is the load-bearing decision in
the group, and it was reached by measurement rather than by reading:

- **The obvious implementation does not close the finding.** Decoding in a loop until
  `io.EOF` looks sufficient and is not: goccy returns `io.EOF` after the first document when
  two markers are adjacent. `version: 1\n---\n---\nkinds:\n  b:\n    to: "y/"\n` decodes to
  the first document with a **nil error**, `kinds.b` silently gone — the exact failure this
  requirement exists to prevent, reproduced against the pinned version. The mirror case
  `---\n---\nversion: 1\n` decodes to nothing at all and would report `version is required`
  for a file that plainly declares one.
- **A decoder cannot report what it declined to read.** The token count is right in every
  case measured, including both of the above, `---` inside a block scalar and inside a
  quoted string (neither is a marker), a trailing `---` followed only by a comment, and
  `...` used as an end marker.
- **It works on a file the decoder cannot parse.** The lexer tolerates
  `version: 1\n---\nkinds: [unclosed` and still reports two documents, so the fault is named
  as the extra document it is rather than as a syntax error inside a document graft is about
  to say should not exist. The decode is only reached when the count is one, so the
  multiple-documents message never competes with a decoder message.

**D3 — `yaml.Unmarshal` stays.** Since D2 does not need the decoder to count, the decode
path is untouched: no swap to `yaml.NewDecoder`, and therefore no risk that an existing
message, an empty-file result, or a type choice moves underneath the existing suite.
*Alternatives rejected:* the decoder loop (D2, wrong answers); and `parser.ParseBytes` with
`len(f.Docs)`, which reports **2** for a file ending in a bare `---` — refusing a file that
discards nothing — and errors outright on a malformed second document, so it fails both
scenarios the token count passes.

**D4 — `os.Lstat` plus `Mode().IsRegular()`, before the read.** `Load`'s doc comment already
claims it "reads no path other than the one it was given"; the check makes the claim true.
The shape is not invented here: `internal/source`'s `List` refuses a non-regular `from` with
`Lstat` and `Mode().IsRegular()` for the same reason, so the two rules read alike. The
motivating leak is not reachable through today's only caller — `source.ReadCatalog` reads
through an `os.Root`, which refuses an escaping link outright — and the check is made anyway,
because `Load` is exported, is delegated to directly on the absence path, and is what the
next caller will reach for. A property that holds only because of what a caller happens to do
is not a property of this package.
It sits before `os.ReadFile` rather than after, because a rule enforced after the read is a
rule enforced too late — the read is the thing being prevented. *Alternative considered:*
`os.OpenFile` with `syscall.O_NOFOLLOW`, which is atomic and closes the TOCTOU window `Lstat`
leaves. Rejected: it buys nothing against the threat that motivates the rule (a committed
symlink in a fetched tree, which is fixed at fetch time and not racing anything), costs a
`syscall` import in a package that has none, and would make `Load` the only place in the
codebase reaching for a raw syscall constant. The residual race is recorded under Risks.

**D5 — One message for every non-regular file.** A symlink, a directory, a fifo, and a
device all get `catalog.yaml: not a regular file`. Splitting them would multiply asserted
strings for a distinction the reader cannot act on differently, and the directory case is
already covered by `TestLoad_Unreadable`, whose assertions (the `catalog.yaml: ` prefix, and
*not* the not-graftable text) this message satisfies unchanged. The message deliberately does
not name the link target: quoting it back is the information leak the rule exists to close.

**D6 — The version literal is read by shape, and the shape is a decimal integer literal the
decoder could not hold.** The decoder hands back a `string` for both `version: "1"` and
`version: 99999999999999999999999999`, so the Go type cannot separate them and the spec
requires the first to keep its existing message. The rule is therefore: an optional `+` or
`-`, then decimal digits with optional `_` separators, and — the part that makes it exact —
the value must **overflow** 64 bits. Separators are stripped before the range test and only
`strconv.ErrRange` counts: `ParseUint` fails on `99…9_0` with `ErrSyntax`, not `ErrRange`,
so conflating the two failures would route an underscore-separated wide literal straight back
to the non-integer message this rule exists to avoid. A literal that *would* fit never
arrives as a string in the first place, so a string carrying `1` was quoted deliberately and
is not an integer. Sign decides which existing message applies: a
negative literal is below `1`, so it is "not a known catalog version" rather than a future
format. Non-decimal spellings (`0x…`, `0o…`) are not treated as versions: goccy converts the
ones it can hold, and one too wide to hold is far enough outside SPEC.md's `version = 1` that
"must be an integer" is a fair answer. *Alternative rejected:* `math/big` to parse the value
and compare it against `1`. It answers a question nobody asks — the exact magnitude of a
version this binary will refuse either way — and adds a parse path for the one input class
that is guaranteed to fail.

**D7 — The duplicate-destination key is the cleaned path plus the trailing-slash bit, not the
cleaned path alone.** This is the one place the finding as originally written had to be
narrowed — and the one whose stated harm had to be corrected. The finding said the kind
"would be written to one directory twice"; it would not. `destination-computation` requires
a destination in cleaned form (`insideRepo` demands `path.Clean(p) == p`), so
`.claude/agents//` is already refused at plan time — with
`destination ".claude/agents//" escapes the repo root`, a message about the wrong fault, at
the wrong layer, for a catalog whose actual mistake is a duplicate. What this repair buys is
the accurate message at the layer that can see the duplicate, one step earlier and
independent of whether the kind has any items. `path.Clean` alone makes `a/{name}` and `a/{name}/` one destination — and
`destination-computation` (a live, archived requirement, *The same pair is two destinations
for a file item*) says they are **two** for an item whose `from` names a file, with
`internal/plan` already implementing exactly that in `destKey`. Cleaning them together here
would make a catalog that spec requires to work unparseable, breaking a capability this
change does not even claim to touch. So the key keeps the bit that carries meaning:
`path.Clean(d)`, with `/` re-appended when `d` ended in one. `.claude/agents/` and
`.claude/agents//` collapse; `a/b` and `./a/b` collapse; `docs/{name}` and `docs/{name}/`
do not. `internal/plan` keeps its per-item rule, which decides the case this one deliberately
leaves open — and which also catches what a string comparison here never could, two entries
that collapse only once `{name}` is filled in.

**D8 — The comparison cleans; the value does not.** `Kind.To` still carries every destination
exactly as written, `{name}` uninterpolated and separators untouched, because
`destination-computation` reads the raw string and a cleaned one would change what a trailing
slash means. The cleaned form exists inside `destinations()` as a map key and nowhere else,
which is asserted by its own scenario rather than left to review.

**D9 — Two message forms for a duplicate destination.** Identical spellings keep
`duplicate destination "<d>"`; differing spellings get
`duplicate destination "<b>": same path as "<a>"`. One form for both would either break an
asserted message this change has no reason to move, or print one string twice and send the
reader hunting for two identical entries that are not in the file. Naming both is what makes
the second case actionable, and saying "same path as" twice about one string would be noise.

## Risks / Trade-offs

- **TOCTOU between `Lstat` and `ReadFile`** → accepted, and named rather than papered over.
  Closing it needs `O_NOFOLLOW` (D4). The window matters only if something can swap the path
  between two syscalls, which for a fetched, content-addressed tree means an attacker who
  already has write access to the consumer's cache — at which point the catalog's *contents*
  are theirs to choose and a symlink is the least of it.
- **The `os.ReadFile` error branch becomes nearly unreachable** once `Lstat` has passed →
  accepted, uncovered, and deliberately not tested. The remaining way in is a permission
  denial, and a test that depends on a read being *refused* inverts under a root CI runner —
  the one filesystem assertion this repository cannot make portably. Measured rather than
  guessed: `internal/catalog` is at **99.5%** today, and a prototype of these four repairs
  lands at **96.9%** with eight uncovered statements — the `Lstat` non-`ErrNotExist` wrap and
  the whole `ReadFile` error branch, which no portable test reaches. The version scenarios
  for `+99…`, the quoted wide literal, and the quoted `"-1"` exist partly to cover the shape
  helper's branches, and task 2.4's refactor removes the now-dead second absence path. The
  floor is 80% over `./internal/...`; the residue is named here so nobody has to rediscover
  it.
- **The document count reaches into the decoder's `lexer` and `token` subpackages**, which are
  a lower-level API than `yaml.Unmarshal` and could change shape across a dependency bump →
  accepted, and cheap to hold: the marker token types are a stable part of the YAML grammar,
  the whole use is one function, and eight scenarios pin its answers. The decode path itself
  is untouched (D3), so the 18 regression scenarios are a control on everything else.
- **The shape rule treats a *quoted* 26-digit string as a version** → accepted. Both
  spellings are refused, both name the literal, and the only difference is which of two
  refusals the reader is told. Distinguishing them would need the source token's quoting,
  which means holding the AST rather than a decoded value — a structural change to `Parse`
  for a distinction with no consequence.
- **A narrower duplicate rule than the finding proposed** (D7) → the finding, applied
  literally, would have contradicted an archived requirement. The narrowing is recorded here
  and in the spec text so it reads as a decision rather than as an incomplete repair, and the
  case it declines to judge is the one `internal/plan` already judges per item.
- **Four unrelated repairs in one change** → they share a package, a capability, a reason for
  having been deferred, and a deadline. Splitting them into four proposals would multiply the
  planning artifacts by four for a diff of roughly forty lines, and would leave three of them
  still open at `sync-command`.

## Migration Plan

None needed. There is no released `graft`, no `catalog.yaml` in any repository graft reads,
no datastore, and no deploy order. Rollback is reverting the commits.

## Open Questions

None remain. Three were resolved while writing this:

- **Should a bare trailing `---` be an error?** No — it introduces no document and discards
  nothing, and the decoder agrees (D2). Pinned by a scenario so a future "tidy up" cannot
  quietly make it one.
- **Should the non-regular-file message name what it found?** No (D5). One string, because
  the reader's next action is the same for a link and a directory: publish a real file.
- **Should the duplicate comparison also catch `{name}` collapses**, such as
  `["a/{name}", "a/tdd"]`? No. Those are per-item — they depend on which item is being placed
  — and `internal/plan` already reports them with a message naming the item. Catching them
  here would mean interpolating a name at parse time, which is the boundary
  `catalog-and-selectors` drew deliberately.
