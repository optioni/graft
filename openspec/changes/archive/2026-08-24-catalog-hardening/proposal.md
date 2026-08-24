## Why

`catalog-and-selectors` shipped `internal/catalog` and deferred four findings, all of them
recorded in its `tasks.md` with the same reason: nothing fetches a source tree yet, so
nothing reads a real `catalog.yaml`. That reason expires at `sync-command`, the change that
first calls `internal/source.ReadCatalog` from a command and so first hands a source's
catalog to `catalog.Parse`. Two of the four are silent — they produce a wrong result and
report success — and a silent failure that reaches a command is a failure a user has no way
to see.

- **A second YAML document is dropped.** `yaml.Unmarshal` decodes the first document and
  discards the rest. The install is short by whatever the later documents provided, and no
  guard fires: a `kind:*` selector still matches the survivors, so the no-match error — the
  one thing that would have caught it — never triggers.
- **`catalog.yaml` may be a symlink.** `os.ReadFile` follows it to any file the invoking
  user can read, and a decoder error quotes the offending lines back verbatim. `Load`'s own
  doc comment says it "reads no path other than the one it was given", and today that is
  not true. `internal/source.ReadCatalog`, its only caller, contains the read inside an
  `os.Root` and so already refuses an escaping link — but it delegates to `Load` on the
  absence path, `Load` is exported, and a guarantee that holds only because of what a
  caller happens to do is not the guarantee the package documents.
- **A version literal wider than `uint64` is reported as a non-integer.** The decoder hands
  such a literal back as a `string`, so the reader falls through to "version must be an
  integer" — telling the reader their catalog is malformed when it is simply newer than
  this binary. ENGINEERING.md's compatibility rule says the answer is "upgrade graft".
- **A repeated destination is compared as a string.** `.claude/agents/` and
  `.claude/agents//` both survive the duplicate check meant to stop a kind declaring one
  destination twice. The catalog is then refused two layers later and for the wrong reason:
  `destination-computation` requires a cleaned destination, so the reader is told
  `destination ".claude/agents//" escapes the repo root` about a file whose actual fault is
  the duplicate — and only if that kind has an item at all.

## What Changes

- **BREAKING (format):** `Parse` refuses a `catalog.yaml` holding more than one YAML
  document, rather than reading the first and discarding the rest. The count is taken from
  the file's tokens before decoding, because the decoder cannot report what it declined to
  read — it returns `io.EOF` after the first document when two markers are adjacent, so a
  guard built on it still drops real content in silence.
- `Load` refuses a `catalog.yaml` that is not a regular file — a symlink, a directory, a
  device — with an `os.Lstat` check ahead of the read, so the file it reads is the path it
  was given.
- The version reader tells a too-wide integer literal from a quoted string by the literal's
  *shape*, not by its Go type: a decimal literal too wide for the decoder's integer types
  reports the existing "upgrade graft" message, while `version: "1"` keeps reporting
  "version must be an integer".
- The duplicate-destination guard compares destinations as paths rather than as strings,
  while `Kind.To` keeps every string exactly as written. Two entries that differ only in
  redundant separators or a `./` prefix are one destination; two that differ in a trailing
  slash are **not**, because `destination-computation` already requires that pair to be one
  destination for a directory item and two for a file item.

Marked **BREAKING** because `catalog.yaml`'s accepted format narrows: a file that parses
today can be refused after this change. The scope of that break is small and worth stating
plainly — no `graft` is released, no command reads a catalog yet, and every file newly
refused was already broken in one of the four ways above. `catalog.yaml`'s `version` stays
at `1` rather than moving, because a version bump would mean a catalog written for this
graft is unreadable by an older one, and there is no older one. `graft.lock`'s format is
untouched, and no command's observable output changes.

Each item is also a contract change in this repository's narrower sense: it adds or
re-routes an asserted error message, and `openspec/config.yaml` makes error text part of the
specification.

## Non-Goals

- **No new capability.** Every requirement here belongs to `catalog-format`;
  `selector-expansion` is unchanged.
- **No re-opening of what `catalog-and-selectors` rejected.** Item-name validation stays in
  `internal/catalog` rather than moving to `internal/itemid` — an allowlist there rejects
  the legal selector `agent:*` and fails six `internal/manifest` tests — and `Expand` gains
  no nil-receiver guard.
- **No symlink resolution inside a source tree.** A `from` that is a committed symlink is
  still checked as a string. That belongs to whichever change first opens a `from` path;
  this one hardens the single path `Load` is handed.
- **No destination computation.** `{name}` interpolation, trailing-slash semantics, and
  `flatten` remain `internal/plan`'s. This change compares two `to` entries with each
  other and interprets neither.
- **No fetching, no writing, no CLI surface.** No command, flag, exit code, or stderr
  formatting changes; `internal/apply` gains no write path and `internal/plan` gains no
  code.
- Clear of the PRD non-goals: no dependency resolution, no registry, no merge behavior, no
  auth layer, no runtime dependency on graft.

## Capabilities

### New Capabilities
- None. Every requirement lands in the existing `catalog-format` capability.

### Modified Capabilities
- `catalog-format`: loading gains a regular-file rule; a new requirement makes a catalog a
  single YAML document; version gating gains the shape rule that separates a too-wide
  literal from a non-integer; kind declarations gain the path-wise duplicate-destination
  comparison.

## Impact

- `internal/catalog` only — `catalog.go` (loading, the document count, version), `kinds.go`
  (duplicate comparison), and their tests. One message reachable through
  `internal/source.ReadCatalog` moves as a consequence: a dangling relative link named
  `catalog.yaml` inside a fetched entry now reports `not a regular file` instead of the
  not-graftable message. `internal/source` itself is unchanged and no `source-listing`
  scenario pins the old wording.
- `internal/manifest`, `internal/lock`, `internal/source`, `internal/plan`,
  `internal/apply`, `internal/ui`, and `cmd/graft` are untouched. No new module dependency:
  the document count uses the `lexer` and `token` subpackages of the YAML library already in
  `go.mod`.
- `openspec/specs/catalog-format/spec.md` gains one requirement and has three rewritten.
- `openspec/IMPLEMENTATION-ORDER.md` gains a row: this change exists and is a precondition
  of `sync-command`, which is where a source's real `catalog.yaml` first reaches `Parse`
  from a command.
- No data model, background job, external service, sibling repository, or deployment
  manifest.
