## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

`graft list --json` is a new client-visible command and its end-to-end wiring is the risk: a
document on the wrong stream, a trailing newline written twice, a listing built against the
wrong root, a success exit code while the document went nowhere. design.md → Test Strategy
records why this group is kept.

- [x] 0.1 Reuse `internal/cli/sync_acceptance_test.go`'s harness — `buildGraft`, `runGraftIn`
      with an absolute `XDG_CACHE_HOME`, `newSourceRepo` with `user.name` / `user.email` set
      **on the repository**, and `newConsumer`. No `t.Chdir` and no `t.Setenv`: the working
      directory and the environment belong to the child process
- [x] 0.2 RED: Write the failing end-to-end test for the headline scenario — sync a fixture
      source, then `graft list --json` exits `0`, writes the exact expected document to the
      standard output stream with exactly one trailing newline, writes nothing to the error
      stream, and leaves every path and every byte in the working tree unchanged
- [x] 0.3 Confirm it fails because `list` is not a command — expect
      `graft: unknown command "list"` — and not because the fixture repository, the temp cache,
      or the tree snapshot helper is misconfigured

## 1. itemid: the two halves of an id
<!-- kind: behavior -->

- [x] 1.1 RED: Write failing unit tests for `itemid.Split`: `schema:tdd` splits into `schema`
      and `tdd` with `ok` true; `agent:*` splits with the glob left as written; an empty half on
      either side, a missing colon, and a second colon each report `ok` false and return empty
      halves
- [x] 1.2 RED: Write a failing test asserting `Valid` and `Split` agree on every input in the
      table above — `Valid(s)` is exactly `Split(s)`'s `ok`
- [x] 1.3 GREEN: Implement `Split(s string) (kind, name string, ok bool)` and re-express `Valid`
      through it, so the grammar is stated once
- [x] 1.4 Run `go test ./internal/itemid/` — green, and the existing `Valid` tests unchanged

## 2. ui: the render vocabulary two commands share
<!-- kind: refactor -->

Structure only. The three helpers move; no rendered byte changes anywhere. The new tests at the
new address assert exactly what the moved code already produced, which is what keeps this a
refactor rather than a behavior change wearing one's clothes.

- [x] 2.1 CHARACTERIZE: Run `go test ./internal/sync/` and confirm `TestReportAlignment` —
      SPEC.md's report example asserted line for line — is green before anything moves. It is the
      characterization for this group and it may not be edited by it
- [x] 2.2 REFACTOR: Move `short`, `fileCount`, and `pad` from `internal/sync/render.go` into
      `internal/ui` as `ShortSHA`, `FileCount`, and `Pad`, with the `shortSHA = 7` constant, and
      call them from `internal/sync/render.go`. Carry each function's existing doc comment across
      and keep the bodies byte-identical
- [x] 2.3 REFACTOR: Add unit tests at the new address for the five `command-output` scenarios —
      `1 file` / `6 files` / `0 files`, the seven-character sha, a sha too short to shorten, the
      three padding cases, and that padding is computed on unstyled text
- [x] 2.4 VERIFY: Run `go test ./internal/sync/ ./internal/ui/` — green, with
      `internal/sync/render_test.go` unedited

## 3. list: the listing value and the JSON document
<!-- kind: behavior -->

**Concentration point.** The document is a published contract, and the two ways a JSON contract
breaks a consumer are both invisible to a test that only decodes: an empty collection marshalled
as `null`, and non-deterministic ordering. Every test here asserts **exact bytes**, and the
determinism assertion is byte equality across two runs and across two locks with the same
content in different orders — the same rule `graft.lock`'s own serialization is held to.

- [x] 3.1 RED: Write a failing unit test holding SPEC.md's `graft.lock` example as a hand-built
      `*lock.Lock` and the expected JSON document as an exact golden string in the test file:
      *SPEC.md's own lock renders as this exact document*, trailing newline included
- [x] 3.2 RED: Write failing unit tests for the empty and boundary shapes: *A source with no
      items renders `[]` rather than null* (plus an assertion that `null` appears nowhere), *An
      item with no files renders `[]` rather than null*, and the empty document for a lock with
      no sources
- [x] 3.3 RED: Write failing unit tests for *The document is valid JSON that round-trips*
      (decode and compare every value against the source lock, the full 40-character sha
      included), *A git URL containing an ampersand is not escaped*, and *The kind and name
      halves match the id*
- [x] 3.4 GREEN: Implement `internal/list`'s `Listing`, `Source`, and `Item` with the JSON tags
      and field order design.md → Contracts fixes, and `FromLock(*lock.Lock) *Listing` sorting
      sources by name, items by id, and files by path, allocating every slice with
      `make(..., 0, n)` so an empty one marshals as `[]`
- [x] 3.5 GREEN: Implement `(*Listing).JSON() []byte` with `json.NewEncoder` into a
      `bytes.Buffer`, `SetIndent("", "  ")` and `SetEscapeHTML(false)`, returning the complete
      document including the trailing newline `Encode` appends. Discard `Encode`'s error with an
      explicit `_ =` and the comment saying why it cannot fail — a tree of strings, ints, and
      slices of those, into a `bytes.Buffer`
- [x] 3.6 GREEN: Implement `(*Listing).Empty()` and the `Version = 1` constant, documented as
      **this document's** version rather than `graft.lock`'s
- [x] 3.7 CHECK: Re-read `specs/list-execution/spec.md`'s JSON requirement against the golden
      string in the test, field name by field name and field order included, and confirm the
      published document and the spec say the same thing. This is the change's one interface a
      separate consumer depends on
- [x] 3.8 Run `go test ./internal/list/` — green, no regressions

## 4. list: the plain rendering
<!-- kind: behavior -->

- [x] 4.1 RED: Write a failing unit test for *SPEC.md's own lock renders as one block*, holding
      the expected lines exactly and asserting no line carries trailing whitespace
- [x] 4.2 RED: Write failing unit tests for *Two sources are separated by one blank line* (the
      listing does not end with a blank line), *A source with no installed items is its header
      alone*, and *A resolved sha shorter than seven characters is printed whole*
- [x] 4.3 GREEN: Implement `(*Listing).Lines() []string` — the header
      `<name>  <rev>  (<short sha>)`, a blank line, the item lines indented two spaces with the
      id padded to the block's widest id, and one blank line between blocks. Use `ui.ShortSHA`,
      `ui.FileCount`, and `ui.Pad` rather than a second copy of any of them
- [x] 4.4 REFACTOR: Confirm no helper here duplicates one in `internal/sync/render.go`; if one
      does, it belongs in `internal/ui` beside the three already there
- [x] 4.5 Run `go test ./internal/list/` — green, no regressions

## 5. list: reading the lock the repository has
<!-- kind: behavior -->

**Concentration point.** Error strings are asserted contract. `list` introduces none of its own
— every failure here is a message `internal/lock` already words — so these tests assert that the
message arrives **unaltered**, with no second layer of context wrapped around it.

- [x] 5.1 RED: Write failing integration tests over a real `t.TempDir()` holding a hand-written
      `graft.lock`: *A lock declaring no source is the same as no lock*, *A scrambled lock lists
      in the same order as a canonical one*, and *A scrambled lock produces the same document as
      a canonical one* (byte equality across the two locks and across two runs of one)
- [x] 5.2 RED: Write a failing integration test for *A manifest whose rev moved ahead of the
      lock is not reported* — both files present and disagreeing; the listing names the lock's
      rev, does not name the manifest's, and `Run` returns no error
- [x] 5.3 RED: Write a failing integration test for *A lock that is a directory is refused* —
      the error carries the `graft.lock: ` prefix and no listing is returned
- [x] 5.4 GREEN: Implement `list.Options{Root string}` and `list.Run(Options) (*Listing, error)`
      — `lock.Load(filepath.Join(root, lock.Filename))` and `FromLock`, returning every error
      exactly as `internal/lock` worded it. An absent lock is not an error: `lock.Load` already
      returns the empty lock, which is the "nothing installed" case
- [x] 5.5 REFACTOR: Confirm `internal/list` imports neither `internal/apply`, `internal/plan`,
      `internal/source`, nor `internal/manifest`, and add a test asserting that import set — the
      observable form of "this command cannot write and cannot reach the network"
- [x] 5.6 Run `go test ./internal/list/` — green, no regressions

## 6. cli: the `list` command
<!-- kind: behavior -->

- [x] 6.1 RED: Write failing acceptance tests for the argument surface: *A positional argument
      is a usage error*, *Only the first positional argument is named*, *An unknown flag is a
      usage error*, and *A read-only subcommand's help names only the flag it has*
- [x] 6.2 RED: Write failing acceptance tests for the empty repository: *A repository with no
      lock prints a note on stderr* (exact stderr, byte-empty stdout, exit `0`, and a directory
      listing proving nothing was created), *A repository with no lock still prints a JSON
      document*, and *A directory with no graft files at all is not an error*
- [x] 6.3 RED: Write failing acceptance tests for the refusals and the read-only promise: *A
      lock from a newer graft is refused* (both forms), *A malformed lock is refused before
      anything is printed* (stdout byte-empty — no opening brace), *A listing leaves the working
      tree byte-identical*, *A listing creates no cache directory*, *A lock claiming a file that
      is not there still lists it*, and *A listing runs with no source repository reachable*
- [x] 6.4 GREEN: Extract the working-directory lookup out of `internal/cli/perform.go` into a
      helper both `perform` and `list` call, so all three commands carry one
      `cannot determine the working directory: ` message
- [x] 6.5 GREEN: Implement `newList(u *ui.UI) *cobra.Command` — `Use: "list"`, graft's own
      argument validator producing `unknown argument "<arg>"`, a `--json` bool flag, and a `RunE`
      that calls `list.Run` and writes: the document through `u.Out()` for `--json`, the note
      through `u.Note` when the listing is empty, and the lines through `u.Print` otherwise.
      Register it on the root beside `sync` and `update`
- [x] 6.6 REFACTOR: Confirm the command holds no decision of its own — every string it prints
      comes from `internal/list` or `internal/ui`, so nothing this change adds sits where the
      coverage gate cannot see it
- [x] 6.7 CHECK: Re-run the existing `command-invocation` acceptance tests and extend *Help
      lists the commands graft has* to name `list`; confirm no other existing scenario's text
      changed
- [x] 6.8 Run `go test ./internal/cli/` — green, no regressions

## 7. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [x] 7.1 VERIFY: Confirm the group 0 acceptance test now passes end to end
- [x] 7.2 REFACTOR: Fold any tree-snapshot or document-fixture helper the acceptance cases
      duplicate into one place in `internal/cli`

## 8. Documentation: SPEC.md gains the command it always named
<!-- kind: operational -->

SPEC.md is the contract and it describes `graft list` in one table row. The `--json` document is
a published interface with no home in it, and the failure-mode table is where this repository
records what a command refuses.

- [ ] 8.1 CHECK: Re-read SPEC.md's Commands, Output, and Failure modes sections and confirm what
      is missing before editing: there is no `graft list` section, no JSON document, and no row
      for a positional argument to `list`
- [ ] 8.2 CHANGE: Add a `## graft list` section to SPEC.md after `## graft add`, holding the two
      forms — the plain block and the JSON document, both as the exact text the tests assert —
      and stating that `list` reads `graft.lock` alone, writes nothing, and needs no network.
      Rewrite the command table's `graft list` row to point at it
- [ ] 8.3 CHANGE: Add the failure-mode rows this change introduces: a `graft.lock` that cannot be
      parsed reported with `internal/lock`'s own message, and `graft list` given a positional
      argument. Do not restate rows that already cover `list`
- [ ] 8.4 CHANGE: Add one line to README.md's command list only if the existing `graft list`
      line no longer describes what shipped; state the expected outcome is **no edit** if it does
- [ ] 8.5 CHECK: Decide whether AGENTS.md needs a rule. The candidate is the one a future change
      would get wrong: the JSON document's shape is a contract, not a convenience, and its empty
      collections and ordering are asserted as bytes. Prefer rewriting the existing lock
      determinism rule to cover both artifacts over adding a second rule beside it
- [ ] 8.6 VERIFY: Re-read the edited sections and confirm every example matches a test fixture
      character for character — SPEC.md's examples are what the tests are written against

## 9. Change Review
<!-- kind: operational -->

- [ ] 9.1 CHECK: Dispatch an independent reviewer — not a fork of the implementing session —
      given only `proposal.md`, the three spec files, `design.md`, `tasks.md`, and the diff.
      Concentration points for this change: the JSON document's exact bytes against the spec;
      empty collections rendering as `[]` at every level; determinism across two runs and two
      lock orderings; that `list` reads no `graft.toml`, writes nothing, and reaches no network;
      that the helper move changed no rendered byte of the sync report; that every asserted error
      string is `internal/lock`'s own, unaltered; and that no test asserts a value the
      implementation also declares
- [ ] 9.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a one-line
      reason, note the SUGGESTIONs, and re-run the affected tests
- [ ] 9.3 VERIFY: Confirm no blocking or unowned finding remains, and that the artifacts still
      describe what shipped

## 10. Lint & Verify
<!-- kind: operational -->

- [ ] 10.1 CHECK: Confirm the intended commands and the tiers they cover — `task lint`,
      `task cover` over `./internal/...`, `task build`, and `openspec validate` — and that
      `export PATH="/opt/homebrew/bin:$PATH"` is set for all of them
- [ ] 10.2 VERIFY: Run `task lint` — 0 errors, `gofumpt -l` silent
- [ ] 10.3 VERIFY: Run `task test` — green, race detector clean
- [ ] 10.4 VERIFY: Run `task cover` — green and at or above the 80% floor over `./internal/...`
- [ ] 10.5 VERIFY: Run `task build` — the binary builds
- [ ] 10.6 VERIFY: Run `openspec validate list-command --strict` — clean
