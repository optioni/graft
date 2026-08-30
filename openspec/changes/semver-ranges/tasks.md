## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

A range crossing six packages is the risk: `internal/rev` classifies it, `internal/source`
resolves it, `internal/plan` carries it, `internal/lock` records it, `internal/sync` reports
it, and `internal/list` publishes it.
The two promises no single-package test can hold — that a lock with no range is byte-identical,
and that `sync` never re-evaluates — are both end-to-end. design.md → Test Strategy records why
this group is taken.

- [x] 0.1 Reuse `internal/cli/sync_acceptance_test.go`'s harness — `buildGraft`, `runGraftIn`
      with an absolute `XDG_CACHE_HOME`, `newSourceRepo` with `user.name` / `user.email` set
      **on the repository**, and `newConsumer`. Tags come from the existing
      `(*sourceRepo).tag` helper, which already exists and can be called repeatedly — nothing
      needs extending
- [x] 0.2 RED: Write the failing end-to-end test for the headline scenario — a consumer pinning
      `rev = "^1.2.0"` against a source publishing `v1.2.0` and `v1.3.0`: `graft update` exits
      `0`, `graft.lock` records `rev = "^1.2.0"`, `matched = "v1.3.0"`, and `v1.3.0`'s sha, the
      report header names `^1.2.0  v1.2.0 -> v1.3.0`, and `graft list --json` returns the
      version-`2` document with `"matched": "v1.3.0"`
- [x] 0.3 RED: Write the failing end-to-end test for the other half — `graft sync` against that
      same repository after `v1.4.0` is published keeps `matched = "v1.3.0"`, writes a
      byte-identical `graft.lock`, and runs with the source repository **deleted**, proving no
      tag listing happened
- [x] 0.4 Confirm both fail because a range does not resolve — expect
      `source "shared": rev "^1.2.0" not found` — and not because the fixture repository, its
      tags, the temp cache, or the tree snapshot helper is misconfigured

## 1. The semver dependency
<!-- kind: operational -->

Plumbing, and its evidence is that the build resolves — not a test asserting a string appears in
`go.mod`, which would only prove the same string was typed twice.

- [x] 1.1 CHECK: Confirm the current latest is `github.com/Masterminds/semver/v3` v3.5.0 via
      `go list -m -versions`, rather than trusting a remembered version
- [x] 1.2 CHANGE: `go get github.com/Masterminds/semver/v3@v3.5.0` and `go mod tidy`
- [x] 1.3 VERIFY: Run `task build` — the module resolves and the binary builds
- [x] 1.4 VERIFY: Confirm the dependency is pure — `go doc` its surface and confirm the calls
      this change uses (`NewVersion`, `NewConstraint`, `Constraint.Check`, `Version.Compare`)
      touch no network, no filesystem, and start no process

## 2. rev: a rev is a range or a ref
<!-- kind: behavior -->

**Concentration point.** This predicate is asked by `internal/lock` and `internal/source` and
they may never disagree. It lives in a **new leaf package `internal/rev`** because the obvious
home does not compile: `internal/source/listing.go` imports `internal/plan`, which imports
`internal/lock`, so `lock -> source` closes a cycle. It is pure and decidable offline, which is
the whole reason it is syntactic — so every test here runs without git.

- [x] 2.1 RED: Write failing unit tests for `rev.IsRange` from the `rev-ranges` scenarios:
      *A caret rev is a range*, *A plain tag is a ref*, *A branch name containing a dash is a
      ref*, *A compound range with a space is a range*, *An alternation is a range*, *A bare
      x-range is a ref, not a range*, and *A full sha is a ref and never a range*
- [x] 2.2 RED: Write a failing test for the observable claim rather than the predicate's own
      return: `Resolve` on an empty rev fails with `source "shared": rev is empty` and starts no
      git process. Asserting `IsRange("") == false` would pass under nearly any implementation,
      and the existing *An empty rev* test already covers the message
- [x] 2.3 GREEN: Create `internal/rev` and implement `IsRange(rev string) bool` — leading `^`,
      `~`, `>`, `<`, or `=`, or containing a space, or containing `||`, or exactly `*`. Document
      why each character is claimed and what claiming `>`/`<`/`=` costs
- [x] 2.4 REFACTOR: Add an import-set test in the style of `internal/list/imports_test.go`
      asserting `internal/rev` imports no package of graft's own — the observable form of "this
      is a leaf, and the cycle stays closed"
- [x] 2.5 CHECK: Confirm `go build ./...` succeeds with `internal/lock` importing
      `internal/rev`, which is the compile the original single-package design would have failed
- [x] 2.6 Run `go test ./internal/rev/` — green, and no test starts a git process

## 3. source: parsing a range and selecting a tag
<!-- kind: behavior -->

Selection takes a `[]string` of tag names and returns one, so every case here is a unit test
over a slice. The remote's job is producing that slice, and that is group 4.

- [x] 3.1 RED: Write failing unit tests for the parse surface: *A caret range parses* and
      *A malformed range is refused without a network call*, asserting the exact message
      `source "shared": rev "^^1" is not a valid semver range`
- [x] 3.2 RED: Write failing unit tests for selection over a fixed tag slice: *The highest
      satisfying tag wins*, *A tag without the v prefix is accepted*, *Unparseable tags are
      ignored rather than refused*, *A range matching exactly one tag*, and *An exact-version
      range selects that version*
- [x] 3.3 RED: Write failing unit tests for prereleases: *A prerelease is not selected by a
      plain range*, *A range naming a prerelease admits it*, and *Only prereleases exist and the
      range names none*
- [x] 3.4 RED: Write failing unit tests for the empty and unsatisfiable shapes, asserting both
      messages exactly: *No tag satisfies the range*, *The source publishes no semver tags*, and
      *The source publishes no tags at all*
- [x] 3.5 RED: Write failing unit tests for determinism: *Tag order from the remote does not
      affect the result* (the same slice shuffled) and *Two tags naming the same version*,
      resolving the tie by tag name
- [x] 3.6 GREEN: Implement the range parse and `MatchRange(rev string, tags []string) (string,
      error)` — parse each tag with `semver.NewVersion`, discard what does not parse, filter by
      the constraint, exclude prereleases unless the range names one, and take the maximum,
      breaking a tie on the tag name so the result never depends on input order
- [x] 3.7 REFACTOR: Confirm the two unsatisfiable messages are built in one place, so the
      distinction between "no semver tags" and "none match" cannot drift between them
- [x] 3.8 Run `go test ./internal/source/` — green, no regressions

## 4. source: resolving a range against the remote
<!-- kind: behavior -->

**Concentration point.** `Resolve` gains a second return value, so every existing caller
changes. The refusals that keep `internal/source` from executing anything must hold on the new
path too — a second `ls-remote` call site is a second place to get the `--` and the leading-dash
guard wrong.

- [ ] 4.1 RED: Write failing integration tests over a fixture repository with tags:
      *The highest satisfying tag wins* end to end, *An annotated tag resolves to its commit*,
      and *A range resolves through the range path and reports its tag*
- [ ] 4.2 RED: Write failing integration tests for the failure paths: *An unreachable remote
      under a range reports the network failure*, *A malformed range does not fall back to a ref
      lookup* (a branch of that exact name exists and is not returned), and *An unreachable
      remote under a range* is distinguishable from an unsatisfiable one
- [ ] 4.3 RED: Write failing unit tests proving the guards hold on the range path with no git
      process started: *An option-shaped remote is refused under a range too* and
      *A malformed range is refused without a network call*
- [ ] 4.4 RED: Write a failing integration test for *Resolving a range writes nothing* —
      snapshot the working tree and the cache root before and after
- [ ] 4.5 GREEN: Implement tag listing with `git ls-remote --tags -- <url>` through the existing
      `gitOutput` helper, reusing `CloneURL`'s leading-dash refusal and the same
      prompt-disabling environment. Strip `refs/tags/` and the `^{}` suffix, preferring the
      peeled ref for an annotated tag. Preserve the `errors.Is(err, errNoGit)` passthrough the
      ref path has: without it a missing git reports `cannot reach "<url>"` instead of
      `git not found on PATH`, which is an asserted error string
- [ ] 4.6 GREEN: Change `Resolve` to `Resolve(name, git, rev) (sha, matched string, err error)`
      — `rev.IsRange` routes to the range path, everything else to the existing ref path unchanged,
      and a ref returns an empty `matched`. Update every caller — one in production
      (`internal/sync/run.go`) and the test call sites. Assert the empty-matched clause the
      `rev-resolution` scenarios now carry: a branch and a lightweight tag both report an empty
      matched tag
- [ ] 4.7 CHECK: Re-read SPEC.md's `rev` line and `rev-resolution`'s existing scenarios, and
      confirm every ref behavior is untouched: the peeled-tag precedence, the tag-beats-branch
      precedence, the sha passthrough with no network, the uppercase-sha case, and all three ref
      failure messages verbatim — `rev "<rev>" not found`, `cannot reach "<url>": `, and the
      option-shaped refusal
- [ ] 4.8 Run `go test ./internal/source/` — green, and the pre-existing resolution tests
      unedited apart from the new return value

## 5. lock: the matched tag
<!-- kind: behavior -->

**Concentration point.** Determinism and byte-compatibility. A lock with no range must
re-serialize to bytes identical to the file on disk, or every consumer's next sync churns the
diff for a feature they do not use.

- [ ] 5.1 CHARACTERIZE: Write the guard for *A lock with no ranges is byte-identical after this
      change* — green before and after, which is the point: it is the compatibility promise, not
      a behavior being added, so it is scheduled first but never goes red — parse a fixture lock written before this change and assert the re-serialized
      bytes equal the input exactly
- [ ] 5.2 RED: Write failing unit tests for serialization: *A range's lock carries the matched
      tag*, *A ref's lock carries no matched key*, and *A range source serializes matched
      between rev and resolved*, asserting the four header lines exactly and that the alignment
      column is unchanged
- [ ] 5.3 RED: Write failing unit tests for validation, asserting each message exactly: *A
      matched key on a ref pin is refused*, *A range pin without a matched key is refused*, *An
      empty matched value is refused*, and *The range test is the same one resolution uses*
      (`rev = "1.x"` with no `matched` parses)
- [ ] 5.4 GREEN: Add `matched` in all **five** places it must appear: the `lock.Source` struct,
      the `source` TOML struct, `rejectUnknown`'s allow-list in `internal/lock/lock.go`, the
      `validate` rules against `rev.IsRange`, and `Marshal`'s slot between `rev` and `resolved`,
      written only when non-empty. The allow-list is the one that fails silently if missed:
      `rejectUnknown` runs **before** `validate`, so without it every range lock fails as
      `unknown key "matched"` and all three new validation messages are unreachable
- [ ] 5.5 CHECK: Confirm the migration, backfill, and cache-invalidation steps this change
      requires — expected outcome is **none apply**: `matched` is absent from every lock that
      exists, no file moves, and the fetch cache is keyed by sha
- [ ] 5.6 CHECK: Re-read SPEC.md's `graft.lock` section against the serializer and confirm the
      documented format matches, including that `version` stays `1` and why
- [ ] 5.7 Run `go test ./internal/lock/` — green, and the existing byte-equality and ordering
      tests unedited

## 6. plan: carrying the tag into the next lock
<!-- kind: behavior -->

**Concentration point.** `internal/plan/build.go` is the only place a `lock.Source` is
constructed, and `plan.Input` has no field for the matched tag. Without this group every range
source produces `matched = ""` in the next lock, which group 5's own validation then refuses —
so groups 0, 7 and 9 all fail with nothing owning the cause. `plan` stays pure: it carries the
value and forms no opinion about it.

- [ ] 6.1 RED: Write a failing unit test for *A range source's next lock carries the matched tag
      and round-trips* — build a plan for a source with `rev = "^1.2.0"` and matched `v1.3.0`,
      serialize the next lock, and parse it back through `lock`'s own parser
- [ ] 6.2 RED: Write a failing unit test for *A ref source's next lock carries no matched tag* —
      the serialized bytes contain no `matched` line and parse back without error, which they
      would not if an empty `matched` were written
- [ ] 6.3 GREEN: Add `Matched string` to `plan.Input` and set it on the `lock.Source` that
      `Build` constructs, carried verbatim
- [ ] 6.4 REFACTOR: Confirm `internal/plan` calls `rev.IsRange` nowhere and reads no file — a
      third opinion about what a rev means is a third thing to keep in agreement, and a
      filesystem read here breaks the purity invariant
- [ ] 6.5 CHECK: Re-read `sync-plan`'s round-trip requirement and confirm the plan cannot build
      a lock `lock.Parse` would refuse, for a range source and a ref source alike
- [ ] 6.6 Run `go test ./internal/plan/ ./internal/lock/` — green, no regressions

## 7. sync: carrying the tag, and the report that names it
<!-- kind: behavior -->

**Concentration point.** `run.go`'s `resolve` deliberately skips `source.Resolve` when the lock
already holds a sha — that skip *is* the difference between sync and update. So `matched` has
two sources depending on the branch taken, and `newReport` derives from the two **locks**, not
from resolution. Carrying it on only the resolving path would make every `graft sync` over a
range write a lock its own validation refuses.

- [ ] 7.1 RED: Write a failing integration test for the non-resolving path — *A newer tag
      satisfying a range does not move the pin*: `graft sync` over a range whose lock entry
      exists writes a byte-identical lock, `matched` intact, with no tag listing
- [ ] 7.2 RED: Write a failing integration test for the first-resolution path — *A range source
      with no lock entry is resolved once and recorded*: `graft sync` over a hand-added range
      source does list tags, selects the highest, and records `matched`
- [ ] 7.3 RED: Write a failing integration test for *A range with no lock entry that no tag
      satisfies fails the run*, asserting the exact message and that nothing is written
- [ ] 7.4 RED: Write failing unit tests over a built report value: *A range whose matched tag
      moved shows the range once and the tag twice*, *A newly added range source shows the range
      and its tag once each*, and *A range whose tag did not move renders every half once*
- [ ] 7.5 CHARACTERIZE: Confirm `TestReportAlignment` is green before anything in this group
      moves, and keep it unedited throughout — it already passes and must keep passing, so it is
      a guard rather than a RED step
- [ ] 7.6 RED: Write failing integration tests for *A new tag satisfying a range moves the pin*,
      *A new tag outside the range does not move the pin*, and *A range that stops matching is an
      update failure, not a sync failure*
- [ ] 7.7 GREEN: Carry `matched` into the `pinned` map alongside the sha, so the non-resolving
      path reads it from `current.Sources`, and take it from `source.Resolve` on the branch that
      resolves. Feed both into `plan.Input.Matched`
- [ ] 7.8 GREEN: Render the optional matched column in the report. Reuse `ui.ShortSHA` and the
      existing half rendering rather than a second copy of either
- [ ] 7.9 RED: Write a failing test for *A matched tag that moved onto the same commit still
      gets a header* — `report.go`'s per-source skip is
      `len(items)==0 && hadBefore && hasAfter && b.Rev==a.Rev && b.Resolved==a.Resolved`, which
      swallows a retag onto one commit: the lock changes and the report shows a summary with no
      block explaining it
- [ ] 7.10 GREEN: Add `b.Matched == a.Matched` to that skip condition
- [ ] 7.11 REFACTOR: Render the matched half with the existing `transition(before, after)`
      helper in `internal/sync/render.go`, which already does exactly this for rev and sha —
      do not write a second copy
- [ ] 7.12 Run `go test ./internal/sync/` — green, with `TestReportAlignment` unedited

## 8. list: matched in both forms, and document version 2
<!-- kind: behavior -->
<!-- parallel-after: 5 -->

**Concentration point.** The `--json` document is a published contract asserted as exact bytes.
Its version moves, which means every golden string in `internal/list` and every acceptance
document changes together — a stale one is a test that passes against the old contract.

- [ ] 8.1 RED: Write failing unit tests for the plain form: *A range source names the tag it
      matched* and *A ref source's header gains no column*, asserting the header lines exactly
      and that no line carries trailing whitespace
- [ ] 8.2 RED: Write failing unit tests for the document over a built `*list.Listing`:
      *SPEC.md's own lock renders as this exact document* with `"version": 2` and
      `"matched": ""`, and *A range source carries the tag it matched*. **Not** *A repository
      with no lock still prints a JSON document* — a repository with no lock cannot be expressed
      as a built value, so it belongs at the acceptance tier in 8.6
- [ ] 8.3 RED: Write failing unit tests confirming the existing contract rules still hold at
      version 2: *A source with no items renders `[]` rather than null*, *An item with no files
      renders `[]` rather than null*, *A git URL containing an ampersand is not escaped*, *A
      scrambled lock produces the same document as a canonical one*, and *The document is valid
      JSON that round-trips*
- [ ] 8.4 GREEN: Add `Matched` to `list.Source` between `Rev` and `Resolved`, always marshalled;
      move `Version` to `2`; add the optional column to `Lines()`
- [ ] 8.5 GREEN: Update the two `--json` goldens in `internal/cli/list_acceptance_test.go` —
      `fixtureDocument` and the empty-document literal — to `"version": 2` with `"matched"`
      between `rev` and `resolved`. They are the acceptance tier design.md assigns these
      scenarios to, and a stale golden asserts the contract this change replaced
- [ ] 8.6 GREEN: Add the acceptance cases design.md's matrix names at that tier: *SPEC.md's own
      lock renders as this exact document*, *A range source carries the tag it matched*, *A
      repository with no lock still prints a JSON document* at version 2, *SPEC.md's own lock
      renders as one block*, and *A range source names the tag it matched*
- [ ] 8.7 CHECK: Re-read `specs/list-execution/spec.md`'s JSON requirement against the golden
      string field name by field name and field order included, and confirm the published
      document and the spec say the same thing. This is the change's one interface a separate
      consumer depends on
- [ ] 8.8 CHECK: Grep the repository for every remaining `"version": 1` and every JSON golden,
      and confirm none still asserts the version-1 document — the bump changes every one at once
- [ ] 8.9 Run `go test ./internal/list/ ./internal/cli/` — green, no regressions

## 9. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [ ] 9.1 VERIFY: Confirm both group 0 acceptance tests now pass end to end
- [ ] 9.2 VERIFY: Confirm *`graft sync` does not re-evaluate a range* passes with the source
      repository deleted, which is the only proof that no tag listing happened
- [ ] 9.3 REFACTOR: Confirm no acceptance case grew its own tag-creating helper beside the
      existing `(*sourceRepo).tag`; fold one back if it did

## 10. Documentation
<!-- kind: operational -->

SPEC.md and PRD.md are the contract and the motivation, and this change makes one sentence in
each false. CLAUDE.md carries the rules a future change would get wrong.

- [ ] 10.1 CHECK: Re-read PRD.md's non-goals, SPEC.md's `rev` line, `graft.lock` section,
      `graft update` section, and Failure modes table, and CLAUDE.md's pin rule — and list what
      is now false before editing anything
- [ ] 10.2 CHANGE: **Rewrite** PRD.md's "no version solving, no semver ranges" non-goal in place
      rather than deleting it, so it says what is still true: one source at a time, no
      transitive dependencies, no cross-source constraint solving. Do not add a second bullet
      beside the stale one
- [ ] 10.3 CHANGE: **Rewrite** SPEC.md's `rev` line to admit a range, add the range forms and the
      `matched` key to the `graft.lock` example and its notes, and state under `graft update`
      that a range is re-evaluated there and nowhere else
- [ ] 10.4 CHANGE: Update the three SPEC.md examples this change falsifies, each of which a test
      asserts against character for character: the `--json` document in `## graft list`
      (`"version": 1`, no `matched` member), the plain-listing header beside it, and the
      sync-report header example in the Output section. Show the range form of each
- [ ] 10.5 CHANGE: Add the failure-mode rows this change introduces — an unsatisfiable range, a
      source publishing no semver tags, and a malformed range. Do not restate rows that already
      cover resolution
- [ ] 10.6 CHANGE: **Rewrite** CLAUDE.md's `sync` never re-resolves a pin rule in place to cover
      the range case, and rewrite the JSON-document determinism rule to name version 2. Expected
      net addition is under ten lines; if it is more, something existing is stale and should be
      cut instead
- [ ] 10.7 CHECK: Decide whether README.md's command list still describes what shipped. Expected
      outcome is **no edit** — no command was added or removed
- [ ] 10.8 VERIFY: Re-read every edited section and confirm each example matches a test fixture
      character for character

## 11. Change Review
<!-- kind: operational -->

- [ ] 11.1 CHECK: Dispatch an independent reviewer — not a fork of the implementing session —
      given only `proposal.md`, the six spec files, `design.md`, `tasks.md`, and the diff.
      Concentration points: that `internal/lock` and `internal/source` ask **one** function
      whether a rev is a range; that a lock with no range re-serializes byte-identically; that
      the JSON document's version-2 bytes match the spec exactly; that `sync` starts no
      `ls-remote` for a range, proven with the remote gone; that selection is deterministic
      across input orderings; that the leading-dash and `--` guards hold on the new git call
      site; that every asserted error string is exactly as specified; and that no test asserts a
      value the implementation also declares — the `go.mod` version in particular
- [ ] 11.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a
      one-line reason, note the SUGGESTIONs, and re-run the affected tests
- [ ] 11.3 VERIFY: Confirm no blocking or unowned finding remains, and that the artifacts still
      describe what shipped

## 12. Lint & Verify
<!-- kind: operational -->

- [ ] 12.1 CHECK: Confirm the intended commands and the tiers they cover — `task lint`,
      `task test`, `task cover` over `./internal/...`, `task build`, and `openspec validate` —
      and that `export PATH="$HOME/.nvm/versions/node/v24.18.0/bin:/opt/homebrew/bin:$PATH"` is
      set, since `openspec` is an npm global and Homebrew's `node` is currently broken
- [ ] 12.2 VERIFY: Run `task lint` — 0 errors, `gofumpt -l` silent
- [ ] 12.3 VERIFY: Run `task test` — green, race detector clean
- [ ] 12.4 VERIFY: Run `task cover` — green and at or above the 80% floor over `./internal/...`
- [ ] 12.5 VERIFY: Run `task build` — the binary builds
- [ ] 12.6 VERIFY: Run `openspec validate semver-ranges --strict` — clean
