## Context

`sync-command` landed the whole machine: `internal/sync` walks SPEC.md's resolution sequence,
`internal/apply` performs it, `internal/plan` decides what it may perform, and a `Report`
renders what changed. One line in that machine is what this change is about —

```go
sha, known := pinned[s.Name]
if !known {
    sha, err = source.Resolve(s.Name, s.Git, s.Rev)
}
```

— a source the lock knows is never re-resolved. That is the point of `sync`, and it is also why
graft currently has no way to move a pin at all.

`graft update` is that line with a different answer. Everything downstream of it is identical:
the same fetch, the same catalog read, the same expansion, the same plan, the same applier, the
same prune rule, the same lock written last, the same report. So the design question is not
"how do I build update" but "how do I make update a parameter of the sequence rather than a
second copy of it", plus one genuinely new capability: `--to` writes `graft.toml`, and until now
nothing did.

Three constraints from AGENTS.md shape everything below and are worth restating because they
pull against each other here:

1. **`internal/apply` is the only package that writes to the working tree.** `graft.toml` is in
   the working tree.
2. **graft never writes over `graft.toml` or `graft.lock`** — a floor under the writer, because
   `kinds` are arbitrary and a catalog could otherwise name either as a destination.
3. **Error strings are asserted by tests.** Every message below is a contract, not a phrasing.

(1) and (2) are not in conflict once the question is asked correctly: (2) is about a path that
arrived *in a plan*, from a source. A rev the user typed into `--to` is graft's own write, the
same class as `graft.lock`, which apply has always written. The mechanism keeps them apart by
provenance rather than by path string.

## Goals / Non-Goals

**Goals**

- One resolution sequence, one apply path, one report, with re-resolution as a parameter.
- `graft.toml` edited surgically: one value replaced, every other byte preserved.
- A failed update leaves the manifest and the lock still agreeing with each other.
- Every new error message is decided in a package the coverage gate can see.

**Non-Goals**

- No second orchestration package, and no `update` variant of `apply.Run`'s core.
- No TOML round-trip encoder. `SetRev` rewrites one value and refuses shapes it cannot rewrite
  exactly; it never re-serializes a parsed manifest, because that would delete the consumer's
  comments to move a pin.
- No semver comparison, no "is an update available" exit code, no interactive confirmation.
- Nothing in `cmd/graft`.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/manifest` | yes | new `SetRev` (pure, bytes in / bytes out) and `Read` returning the parsed manifest **and** the bytes it parsed. `Load` stays, delegating to `Read`. |
| `internal/lock` | no | `CheckPins` is unchanged; `internal/sync` narrows *which sources* it hands over. |
| `internal/catalog` | no | — |
| `internal/source` | no | `Resolve` already does exactly what an update needs. |
| `internal/plan` | no | still pure, still no filesystem access, still unaware that `update` exists. |
| `internal/apply` | yes | one `Option`, one write, one pre-flight entry. |
| `internal/sync` | yes | `Options.Update`, a refresh predicate in `resolve`, the narrowed pin check, and the manifest edit. |
| `internal/cli` | yes | the `update` command, plus the tail `sync` and `update` now share. |
| `internal/ui` | no | the report already renders through it. |
| `cmd/graft` | no | one call, one exit. |

**Which existing pattern each new piece follows.** `Options.Update` follows `sync.Options` and
`cli.Options`: everything a run reads from its surroundings arrives as a value, never from a
global. `manifest.SetRev` follows `manifest.Parse` and `lock.Marshal`: bytes in, bytes out, no
filesystem. `apply.Option` is the one new shape in the repository, argued for under Decisions.
The `update` command follows `newSync` exactly — an argument validator producing graft's own
wording, a `RunE` that is wiring, and every decision it could make already made elsewhere.

## Contracts

**`internal/sync` — additive.**

```go
type Options struct {
    Root      string
    CacheRoot string
    DryRun    bool
    Update    *Update // nil for `graft sync`: no pin is re-resolved
}

// Update names what a `graft update` run moves.
type Update struct {
    Source string // "" for every source the manifest declares
    To     string // "" to leave graft.toml alone; otherwise the rev to write for Source
}
```

The zero `Options` is exactly today's `graft sync`, so every existing caller and every existing
test is unchanged.

`To` is honoured **only** together with `Source`, and that is a documented precondition on the
type rather than a checked error. `graft update --to <rev>` with no source is refused by the
command surface as a usage error, which is where it belongs — it earns the hint line, and
`usageError` is `internal/cli`'s. A guard in `sync.Run` would be a branch no user can reach, and
an unreachable branch is a hole in the coverage gate rather than a safety net. The doc comment on
the field says so.

**`internal/manifest` — additive.**

```go
func Read(path string) (*Manifest, []byte, error)          // Load, plus the bytes it parsed
func SetRev(data []byte, name, rev string) ([]byte, error) // one value replaced
```

`Load` keeps its signature and its `graft.toml not found` message, delegating to `Read`, so no
existing caller or test moves. `SetRev` takes no filename: every message it produces is prefixed
`graft.toml: ` literally, because that is the only file it is ever handed and inventing a
parameter to say so would be a second spelling of a constant this package already owns.

**`internal/apply` — additive, by design.**

```go
func Run(root string, trees map[string]string, p *plan.Plan, opts ...Option) error
func WithManifest(data []byte) Option
```

Variadic so that all 56 existing `apply.Run(root, trees, p)` call sites compile and assert
unchanged. See Decisions for why this rather than a fourth parameter or a field on `plan.Plan`.

**New error strings.** Each is a contract; each is asserted by a test.

| Message | Raised by | Class |
|---|---|---|
| `graft.toml has no source "<name>"` | `internal/sync` | domain |
| `graft.toml: source "<name>": cannot move the pin: rev is not a plain key under [sources.<name>]` | `internal/manifest` | domain |
| `graft.toml: rev "<rev>" contains a quote, a backslash, or a control character` | `internal/manifest` | domain |
| `--to requires a source` | `internal/cli` | usage — carries the hint line |
| `--to requires a rev` | `internal/cli` | usage — carries the hint line |
| `unknown argument "<arg>"` | `internal/cli` | usage — the wording `sync` already uses |

`graft.toml has no source` follows `lock.CheckPins`' existing `graft.toml has rev …` rather than
the `graft.toml: …` prefix the parsers use, because it is a statement about the file's content
rather than a parse failure located within it.

**Reused unchanged, and asserted to be:** `source "<name>": rev "<rev>" not found`, the pin
disagreement message, every apply refusal, and every line of the report.

**Consumers affected.** graft is a single binary with no library consumers. The observable
surface that moves is `graft --help`, which gains one line; SPEC.md's command table already
lists `update`, so the help is catching up to the contract rather than changing it. That change
is recorded as a MODIFIED requirement on `command-invocation`, generalised to "every subcommand"
so that the next command added does not need a third amendment there.

## Persistence and Rollout

- **Migration**: none. No format changes.
- **Backfill**: none.
- **Seeding**: none.
- **Cache invalidation**: none. The fetch cache is content-addressed by commit sha; an update
  resolves to a *different* sha and therefore a different entry. No entry is ever rewritten.
- **Index rebuild**: none.
- **Authorization**: none. graft has no auth layer; a private source works exactly as far as the
  user's existing git credentials reach.
- **Observability**: none. There is no telemetry and this change adds none.
- **Deployment**: none. A binary release.
- **`graft.lock` format**: unchanged, `version` stays `1`. An update writes the same lock a sync
  writes, with a different `resolved`.
- **`graft.toml` format**: unchanged. `SetRev` replaces one value inside the format already
  specified.

## Test Boundaries

| Dependency | In acceptance test | In integration tests | In unit tests |
|---|---|---|---|
| The compiled `graft` binary | real, run as a subprocess with its own `cmd.Dir` and `cmd.Env` | not used — `sync.Run` is called in-process | not used |
| The consumer's working tree | real `t.TempDir()` | real `t.TempDir()` | real `t.TempDir()` for `internal/apply`, whose subject *is* the filesystem; **not used** for `manifest.SetRev`, the `Report`, and the renderer, which are values in and values out |
| The source repository | real git repository in `t.TempDir()`, `user.name`/`user.email` set **on the repo** | same | not used |
| `git` on `PATH` | real | real | not used |
| The network | never reached — a fixture's clone URL is a filesystem path, which exercises the same git code | same | not used |
| The fetch cache | real, rooted at an **absolute** `t.TempDir()` passed as `XDG_CACHE_HOME` | real, rooted at a `t.TempDir()` named by `sync.Options.CacheRoot` | not used |
| `internal/apply` | real | real | real, driven by hand-built plans |
| `internal/plan` | real | real | not used |
| `graft.toml` bytes | real file | real file | in-memory `[]byte` for `SetRev`; a real file for `internal/apply` |
| Output streams | real OS pipes across a process boundary | `*Report` values, and `ui.UI` over `bytes.Buffer` | `ui.UI` over `bytes.Buffer` |
| The developer's `~/.cache/graft` | unreachable — `XDG_CACHE_HOME` is set on the child | unreachable — the cache root is a value | unreachable |
| The developer's git identity | unreachable — identity is set on the fixture repository | same | n/a |
| This repository's own tree, `.claude/agents/`, `openspec/schemas/` | unreachable — every root is a `t.TempDir()` | unreachable | unreachable |

Nothing is mocked anywhere. That is not thoroughness for its own sake: the thing this change can
get wrong is which sha a run resolves to, and a replaced `git` would be the one collaborator
that cannot demonstrate it.

## Test Strategy

Three tiers, matching what `sync-command` established:

- **unit** — `go test ./internal/manifest/`, `./internal/apply/`, `./internal/sync/` for the
  report; values only, no git, no network, and for `SetRev` no filesystem at all.
- **integration** — `go test ./internal/sync/`; real fixture git repositories in `t.TempDir()`,
  real cache, `sync.Run` called in-process.
- **acceptance** — `go test ./internal/cli/`; the compiled binary as a subprocess.

**This change takes the outer-loop acceptance test.** `graft update` is a new client-visible
command, and its end-to-end wiring is exactly the risk: a `--to` value that never reaches
`SetRev`, a report on the wrong stream, an update run against the wrong root, a cache root read
from a global. The existing `sync_acceptance_test.go` harness already builds the binary and
carries an environment, so the cost is one more subtest rather than a new tier.

### Verification matrix

One row per scenario in this change's three spec files, plus one row for the outer loop, which
has no spec scenario of its own. Rows marked *existing* restate a scenario this change carries
into a MODIFIED requirement without altering it; its test already exists and must still pass
unchanged, which is the verification.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| **outer loop** (no spec scenario) | real binary, real fixture repo, real cache; `graft update --to <tag> shared` moves `graft.toml`, moves the lock, writes the files, reports on stderr, exits 0, stdout byte-empty | acceptance | real binary, real git, real cache | `go test ./internal/cli/` |
| update-execution: A moved branch moves the pin | branch advances after the lock is written; assert the new file exists and `resolved` moved; assert the same fixture under `sync` does neither | integration | real git, real cache, real tree | `go test ./internal/sync/` |
| update-execution: An update that finds nothing new reports nothing | `Report.UpToDate()` and the rendered lines | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: An update in a repository with no lock installs everything | assert the full tree listing and the lock | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: A source dropped from the manifest is pruned without being re-resolved | remove the source's git directory before the run so any fetch would fail | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: A rev that no longer exists fails without touching the tree | exact error string plus a whole-tree byte comparison | integration | real git | `go test ./internal/sync/` |
| update-execution: An item the new rev no longer provides is removed, and a repo-owned file beside it survives | **prune-set concentration point**: a repo-owned file in the shared destination, asserted byte-identical after the run | integration | real git, real cache, real tree | `go test ./internal/sync/` |
| update-execution: A manifest declaring no sources updates nothing | `up to date`, and a tree listing holding only graft's two files | integration | real tree | `go test ./internal/sync/` |
| update-execution: Only the named source is re-resolved | two sources, both branches advanced; assert one `resolved` moved and one did not | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: A source name the manifest does not declare is refused | exact error string; assert the cache root is empty and the tree unchanged | integration | real tree, real cache | `go test ./internal/sync/` |
| update-execution: A source in the lock but not the manifest cannot be updated | same, with the source present in the lock | integration | real tree | `go test ./internal/sync/` |
| update-execution: The pin moves and the rest of the file survives | line-by-line diff of `graft.toml` before and after; exactly one line differs | integration | real git, real tree | `go test ./internal/sync/` |
| update-execution: An update without `--to` never writes the manifest | byte comparison of `graft.toml` | integration | real git, real tree | `go test ./internal/sync/` |
| update-execution: `--to` with no source is a usage error | exact stderr, hint line, exit code, empty stdout | acceptance | real binary | `go test ./internal/cli/` |
| update-execution: `--to` with an empty rev is a usage error | same | acceptance | real binary | `go test ./internal/cli/` |
| update-execution: A `--to` rev that does not exist leaves the manifest where it was | exact error plus byte comparison of `graft.toml` and `graft.lock` | integration | real git, real tree | `go test ./internal/sync/` |
| update-execution: A `--to` naming a source the manifest does not declare is refused | exact error, manifest byte-identical, and the message is the membership one | integration | real tree | `go test ./internal/sync/` |
| update-execution: A `--to` against a manifest shape the editor cannot rewrite is refused | exact error; assert the cache root is empty; assert `graft update shared` succeeds on the same fixture | integration | real git, real tree, real cache | `go test ./internal/sync/` |
| update-execution: An update repairs a manifest whose rev was moved by hand | the run succeeds and the lock's rev follows; the paired `sync` on the same fixture fails with the pin message | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: Updating one source still refuses another source's disagreement | exact pin error naming the other source; cache root empty | integration | real tree, real cache | `go test ./internal/sync/` |
| update-execution: A bumped tag shows both revs and both shas | rendered lines against an exact fixture | integration | real git, `ui` over a buffer | `go test ./internal/sync/` |
| update-execution: A branch whose sha moved shows one rev and both shas | rendered header, exact | integration | real git, `ui` over a buffer | `go test ./internal/sync/` |
| update-execution: An update leaves standard output byte-empty | both streams captured across a process boundary, on success and on failure | acceptance | real binary | `go test ./internal/cli/` |
| update-execution: A dry run of an update writes neither of graft's files | tree entry list plus byte comparison of both files | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: A dry run of `--to` leaves the manifest where it was | rendered header plus byte comparison of `graft.toml` | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: A dry run of a first update creates no directory | assert the tree holds only `graft.toml` — the half a file-existence check would miss | integration | real git, real cache | `go test ./internal/sync/` |
| update-execution: A second argument is a usage error | exact stderr, hint line, exit code | acceptance | real binary | `go test ./internal/cli/` |
| update-execution: An unknown flag is a usage error | exact stderr `graft: unknown flag: --force`, hint line, exit code | acceptance | real binary | `go test ./internal/cli/` |
| manifest-format: The aligned value is replaced and nothing else moves | exact expected bytes | unit | none | `go test ./internal/manifest/` |
| manifest-format: A comment trailing the rev line survives | exact expected bytes, plus a rev value containing `#` | unit | none | `go test ./internal/manifest/` |
| manifest-format: Other sources are untouched | exact expected bytes plus a re-parse | unit | none | `go test ./internal/manifest/` |
| manifest-format: Line endings and a missing final newline are preserved | exact expected bytes for a CRLF input and for an input with no final newline | unit | none | `go test ./internal/manifest/` |
| manifest-format: A rev key in a kinds sub-table is not the one edited | exact expected bytes plus a re-parse asserting the `kinds` override | unit | none | `go test ./internal/manifest/` |
| manifest-format: A commented-out rev above the real one is skipped | exact expected bytes | unit | none | `go test ./internal/manifest/` |
| manifest-format: A quoted table key is recognised | exact expected bytes, bare and whitespace-decorated header | unit | none | `go test ./internal/manifest/` |
| manifest-format: A source written as an inline table is refused | exact error string, nil bytes | unit | none | `go test ./internal/manifest/` |
| manifest-format: A source the file does not declare is refused the same way | exact error string, for an absent source and for `[[sources.x]]` | unit | none | `go test ./internal/manifest/` |
| manifest-format: A multi-line rev value is refused rather than half-rewritten | exact error string, nil bytes | unit | none | `go test ./internal/manifest/` |
| manifest-format: A rev that would have to be escaped is refused | exact error string for `"`, `\n`, `\`, and DEL | unit | none | `go test ./internal/manifest/` |
| manifest-format: A rev that would inject a second key is refused | exact error string; assert no bytes | unit | none | `go test ./internal/manifest/` |
| manifest-format: The result round-trips through the parser | `Parse` the result and compare every field | unit | none | `go test ./internal/manifest/` |
| file-application: A plan's operations happen in the documented order | *existing* test must still pass unchanged | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: An empty plan writes only the lock | *existing* test, unchanged — its whole-tree assertion is already what proves no `graft.toml` was created | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: An empty plan with manifest bytes writes graft's two files and nothing else | new test over the same empty plan | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: Nothing outside the plan is touched | *existing* test must still pass unchanged | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: Manifest bytes are written just before the lock | apply a hand-built plan; read both files back, the manifest through `manifest.Parse` | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: An apply with no manifest bytes leaves graft.toml alone | byte comparison | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: A plan naming graft.toml as a destination is still refused | exact error with `WithManifest` also given; assert no planned file exists | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: A graft.toml that is not a regular file fails before the first write | `graft.toml` created as a directory; exact error; assert no planned file exists | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: A failed apply leaves graft.toml unmoved | read-only destination directory, the residual failure the pre-flight pass cannot remove | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| file-application: No temporary file survives a successful or a failed apply | full tree listing after each | unit | real `t.TempDir()` | `go test ./internal/apply/` |
| command-invocation: No arguments prints help and succeeds | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: Help lists the commands graft has | *existing* test, extended to name `update` | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: `--help` prints the same text as no arguments at all | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: `help` is not a command | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: `help sync` is not a command either | *existing* test must still pass unchanged | acceptance | real binary | `go test ./internal/cli/` |
| command-invocation: A subcommand's own help goes to standard output | `graft update --help` stdout names `update`, `--to`, `--dry-run`; stderr empty; exit 0 | acceptance | real binary | `go test ./internal/cli/` |

Every scenario appears. No task may reach for a collaborator the Test Boundaries table does not
name.

**Visual design source.** Not applicable: this change ships no user-facing view and no email
template. Its entire output surface is the existing report, rendered by `internal/sync` through
`internal/ui`, and is specified as exact text in `sync-report`.

## Decisions

**Re-resolution is a parameter of `sync.Run`, not a second orchestration.**
Alternatives considered: a `sync.Update(Options, ...)` entry point beside `sync.Run`, and an
`internal/update` package. Both duplicate the eight-step sequence, and the sequence is the part
whose *order* is the contract — the pin check before the first fetch, nothing written before the
plan. Two copies means two places for that order to drift, and the second copy is the one no
reviewer re-reads. The parameter form makes "update is sync with the pins re-resolved" a
property of the code rather than a claim in a comment.

**`Update` is a pointer, and `nil` is `sync`.**
Alternative: a `Refresh []string` slice where empty means none. It cannot express "every source"
without a sentinel, and a sentinel string in a list of source names is a name a source could
have. `*Update` with `Source == ""` meaning all is unambiguous because `""` is not a valid source
name — `manifest.Parse` refuses an empty one.

**`SetRev` edits text; it does not re-serialize a parsed manifest.**
Alternative: decode `graft.toml`, set the field, re-encode with `toml.Encoder`. It is far less
code and it is wrong: it deletes every comment, normalizes the alignment SPEC.md's own example
uses, and reorders keys — turning a one-line pin move into a whole-file diff that a reviewer
cannot read. `graft.toml` is a human's file. Another alternative, a comment-preserving TOML
library, is a new dependency for one function, and `go.mod`'s dependency count is a decision to
argue rather than a rule to obey — but not for this.

The cost is that `SetRev` recognises only the shape SPEC.md documents: `rev` as a plain key under
`[sources.<name>]`. Everything else is refused with a message that says exactly what it could not
do. Guessing is the one option not on the table: a wrong guess corrupts the consumer's request.

**A text edit's real failure is landing on the wrong line, so the boundaries are exact and the
result is checked.** Three of them are worth naming because the obvious implementation gets each
wrong:

- **The table's extent.** A header is matched as the exact key path `sources`, `<name>` and the
  table ends at the *next header of any kind*. A prefix match, or one that only stops at the next
  `[sources.` header, walks straight into `[sources.<name>.kinds]` — and `Kinds` is a
  `map[string]string` with no constraint on its keys, so a kind named `rev` is legal and would be
  rewritten by a scanner that never left the source's table.
- **The value's extent.** The value ends at its closing quotation mark, not at the first `#` and
  not at end of line. Cutting at `#` corrupts a rev containing one (git permits it); rebuilding
  the line from everything before `=` deletes a trailing comment the requirement promises to
  preserve. A value whose closing quote is not on the same line — a TOML multi-line string, which
  `manifest.Parse` happily accepts — is refused rather than half-rewritten.
- **Comment lines are skipped**, so `# rev = "v1.0.0"` above the real key is not the target.

And because none of that is provable by inspection, `internal/sync` **re-parses the edited bytes
and asserts the named source's rev is what was asked for** before using them. That single check
turns every wrong-target bug in the class above from a silent success into a failed run. It is
cheap, and it is the difference between "the editor is careful" and "a careless editor cannot get
past this point".

**A rev can never become a git operand, and that is why the character rule is enough.**
`source.Resolve` interpolates the rev into `refs/tags/<rev>`, `refs/tags/<rev>^{}`, and
`refs/heads/<rev>`, and passes `--` before the URL, so a `--to` value beginning with `-` cannot
become a git option. The character rule — no quote, no backslash, nothing `unicode.IsControl`
reports true for — is therefore not doing double duty as an argument sanitiser; its one job is
that a rev cannot close the TOML string it is written into and append a key, a table, or a whole
second source to the consumer's manifest. `unicode.IsControl` rather than `< 0x20` because DEL
and the C1 range are invalid inside a TOML basic string too.

**The membership check runs before the edit.** `graft update --to v1.1.0 sharde` has two true
things to say — the manifest has no such source, and the pin cannot be moved in a table that does
not exist — and only the first helps. `internal/sync` checks membership against the parsed
manifest first, so `SetRev`'s refusal is only ever about a *shape*, never about a name.

**The manifest write goes through `internal/apply`, as an option rather than a fourth argument.**
Alternatives:
- *A fourth parameter on `Run`.* Every one of the existing `apply.Run(root, trees, p)` call sites
  — thirty-odd, across the package's own tests and `internal/sync` — would grow a `nil` that
  means nothing to a reader.
- *A `Manifest []byte` field on `plan.Plan`.* `plan.Build` would never set it, so the plan would
  carry a field its own producer does not produce, and `sync-plan`'s specification would have to
  describe a field that is always absent from a built plan.
- *A separate `apply.WriteManifest` called before `apply.Run`.* The manifest would then move
  before the pre-flight pass ran, so a refused apply would leave a manifest pointing where the
  lock does not.

The option keeps one writer, one pre-flight pass, and one ordering, and `add-command` will reuse
it verbatim when it writes a new source into `graft.toml`.

**`graft.toml` is written through a temporary file and a rename, not through the
remove-then-create the planned writes use.** `writeFile` removes an existing destination before
creating it with `O_EXCL`, and that removal is load-bearing for planned writes: the mode argument
to a create-and-truncate open applies only on creation, so truncating would let a destination
someone once made executable stay executable while a *source* replaced its contents. No source's
bytes are involved in the manifest write, so the reason does not apply — and the cost does.
`graft.toml` is the one file in the repository graft cannot regenerate; a failure between the
unlink and a successful close would delete the consumer's own request. `os.Root.Rename` (Go 1.25)
makes the alternative one call: write a sibling temporary file, rename it over the destination,
and remove the temporary if anything fails. A reader then sees either the old bytes or the new
ones, never neither.

The cost, stated rather than hidden: the file's mode becomes `0644` like any file this package
creates, and a custom mode on `graft.toml` does not survive. That is true of the remove-then-create
path as well, so it is not a regression — and a mode-preserving write would mean stat-then-chmod,
which is a decision this package is otherwise built not to make.

**`graft.toml` is written immediately before `graft.lock`, not first.**
Writing it last means a failure anywhere in the apply leaves the manifest and the lock still
agreeing — the state a plain re-run of `graft update --to …` recovers from. Writing it first
would leave them disagreeing, which is recoverable too, but only by knowing which command to run
next. The residual window is the two consecutive writes, and it is not closed: closing it would
mean a two-file atomic commit, which on a POSIX filesystem means a temporary directory and a
rename dance for a case that leaves a `git diff` a human reads anyway.

**The pin check narrows at the call site rather than inside `lock.CheckPins`.**
`CheckPins` answers "do these sources' pins agree", and *which* sources to ask about is a decision
about this run — which `internal/sync` owns. Changing its signature would push a policy into a
package whose only job is the lock's format, and would churn its existing tests for no gain.

**Argument-shape errors are decided in `internal/cli`, domain errors in `internal/sync`.**
`--to requires a source` earns the hint line, and `usageError` is `internal/cli`'s type; a
message constructed anywhere else could not carry it without exporting the class. `graft.toml has
no source` earns no hint — the invocation was well formed and the file disagreed — and belongs
where the manifest is loaded. Both packages are under `./internal/`, so both are inside the
coverage gate.

**`--dry-run` is offered on `update` as well.**
SPEC.md states `--dry-run` in its Commands section, above the table, not inside `sync`'s row. It
costs one flag and one already-implemented early return, and an update is precisely the run a
user most wants to preview.

## Risks / Trade-offs

- **`SetRev` refuses a manifest shape a user legitimately wrote** (an inline table, a dotted key)
  → the refusal names the shape and the source, `graft update` without `--to` still works, and
  hand-editing the rev then running `graft update <source>` reaches the same end state. The
  alternative — guessing — corrupts a file graft did not write.
- **An update and a lock write are two operations, and a crash between them leaves the manifest
  moved with the lock behind** → the next `graft sync` fails with the pin message naming both
  revs and pointing at `graft update`, which is exactly the right instruction. No silent state.
- **`graft update` on a manifest with many sources makes one `git ls-remote` per source** →
  accepted; `graft update <source>` exists for the narrow case, and a full sha still short-circuits
  before `exec.LookPath`.
- **`update` and `sync` sharing one function means a bug in the shared part is a bug in both** →
  that is the trade being made deliberately, and it is the right one: the shared part is the part
  with the invariants, and one implementation is one thing to keep correct.
- **A test that runs `graft update` against this repository's own tree would rewrite its
  hand-copied `.claude/agents/`** → every test names its own `t.TempDir()` root and its own cache
  root, and the acceptance tier sets `XDG_CACHE_HOME` on the child process. No test calls
  `t.Chdir` or `t.Setenv`.
- **A `SetRev` that lands on the wrong line looks like success** → the re-parse-and-assert in
  `internal/sync` turns it into a failed run, and the spec pins the three boundaries the obvious
  implementation gets wrong (the sub-table, the trailing comment, the multi-line value). This is
  the risk with the widest blast radius in the change, and the reason it carries the most
  scenarios.
- **A rename-based manifest write can leave a temporary file behind if the process dies between
  create and rename** → the file is at a fixed name at the repository root, so it is visible in
  `git status` rather than hidden, and the next successful run overwrites it. The alternative — no
  temporary file — loses the manifest instead, which git can restore but only if the user knows to.
- **`update` and `sync` sharing one function means a change to `sync.Run` can now break `update`
  silently** → task group 3 requires every pre-existing `internal/sync` test to pass unmodified,
  and the zero `Options` to remain byte-for-byte today's behavior.

## Migration Plan

None required. The change is additive: `graft sync` behaves identically, both file formats are
unchanged at `version = 1`, and no state written by an earlier graft needs converting. Rollback
is reverting the commits; a `graft.lock` written by this version is byte-for-byte one an earlier
version reads.

Deploy order: none — one binary, one repository.

## Open Questions

None blocking. One noted for a later change: `add-command` will also write `graft.toml`, to add a
source rather than to move a rev, and will need an insertion function beside `SetRev`. Whether
those two become one text-editing helper is a decision for that change, when there are two callers
to generalize over rather than one to speculate about.
