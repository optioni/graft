## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

- [x] 0.1 Build the `add` acceptance harness beside `internal/cli`'s existing ones: a fixture
  source repo with `user.name` and `user.email` set on the repo, a catalog offering
  `agent:reviewer`, a tag, and a temp working tree with neither `graft.toml` nor `graft.lock`
- [x] 0.2 RED: Write the failing end-to-end test for *A first source is added to a repository
  with no graft.toml* — `graft add <fixture>@v1.0.0 agent:reviewer` writes the file, the
  manifest, and the lock, and exits 0
- [x] 0.3 Confirm it fails because `add` is not a command yet, not because the fixture or the
  cache root is misconfigured

## 1. Appending a source table to graft.toml
<!-- kind: behavior -->

- [x] 1.1 RED: Write failing tests for: *A table is appended to a manifest holding one
  source*, *An empty file gets the table alone*, *A file with no final newline gains one*,
  *Several selectors render on one line, in order*, *Appending a name already declared is
  refused*, *A name that is not a bare key is refused*, *A selector carrying a quote is
  refused*
- [x] 1.2 GREEN: Implement `manifest.AddSource`, rendering the block with SPEC.md's alignment
  and asserting the original bytes are a prefix of the result
- [x] 1.3 GREEN: Generalize `checkRev`'s refusal to `git`, `rev`, and `selector` values,
  leaving the existing `rev` message byte-identical
- [x] 1.4 CHECK: Contract gate — re-read SPEC.md's `graft.toml` section and confirm the
  rendered block matches the documented format key for key
- [x] 1.5 REFACTOR: Extract whatever `AddSource` and `SetRev` genuinely share, or record that
  the scanner is not yet duplicated
- [x] 1.6 Run `go test ./internal/manifest/` — no regressions

## 2. Amending an existing install list
<!-- kind: behavior -->

- [x] 2.1 RED: Write failing tests for: *A one-line array gains a selector on its own line*,
  *A multi-line array gains a line matching its indentation*, *A multi-line array with no
  trailing comma keeps that style*, *A selector already present is not added twice*, *A
  comment after the last element survives*
- [x] 2.2 RED: Write failing tests for the refusals: *An install that is not an array is
  refused*, *A source written as an inline table is refused*, *An unterminated array is
  refused*, *An element carrying an escape is refused*
- [x] 2.3 GREEN: Implement the array scanner — locate `install` under the source's own
  standard table, span the array across lines with quote and comment awareness, and refuse
  anything but single-line quoted strings between the brackets
- [x] 2.4 GREEN: Implement `manifest.AddInstall`, inserting after the last element and its
  trailing comma, preserving indentation and comma style
- [x] 2.5 REFACTOR: Share the table-tracking scan with `SetRev` rather than keeping a second
  copy, while both packages' tests stay green
- [x] 2.6 Run `go test ./internal/manifest/` — no regressions

## 3. A source's default rev
<!-- kind: behavior -->
<!-- parallel-after: 0 -->

- [x] 3.1 RED: Write failing tests for: *The highest stable tag wins*, *A source with no
  semver tags falls back to its branch*, *An empty repository is an error, not an empty rev*,
  *An unreachable source is not reported as an empty one*, *A git value beginning with a dash
  is refused before the call*
- [x] 3.2 GREEN: Implement `source.DefaultRev` — one `git ls-remote --symref` call through
  `CloneURL` and `gitOutput`, tags selected by `MatchRange(name, "*", tags)`, the symref's
  branch as the fallback
- [x] 3.3 REFACTOR: Fold the tag-parsing loop shared with `resolveRange` into one helper, or
  record why the two shapes differ enough to stay apart
- [x] 3.4 Run `go test ./internal/source/` — no regressions

## 4. A manifest-only apply
<!-- kind: behavior -->
<!-- parallel-after: 0 -->

- [x] 4.1 RED: Write failing tests for: *Only graft.toml appears*, *An existing lock is left
  alone*, *A graft.toml that is not a regular file is refused*, *A staging leftover that is
  not a regular file is refused*
- [x] 4.2 GREEN: Implement `apply.Manifest`, reusing the existing staging, rename, pre-flight,
  and ancestor checks rather than a second write path
- [x] 4.3 CHECK: Confirm no prune set exists on this path — assert a populated lock's files
  all survive, which is the rule graft may never break
- [x] 4.4 REFACTOR: Reuse the plan-carrying path's manifest writer verbatim if it factors out
  cleanly
- [x] 4.5 Run `go test ./internal/apply/` — no regressions

## 5. Destinations for a catalog listing
<!-- kind: behavior -->
<!-- parallel-after: 0 -->

- [x] 5.1 RED: Write failing tests in `internal/plan` for `ItemDestinations` — an item's own
  destinations from a catalog, a consumer override, and the listing's `Dir` flag, with no
  filesystem access: a file item names its file, a directory item its directory, a
  list-valued `to` names several, and an escaping destination is refused
- [x] 5.2 GREEN: Export that computation, factoring the entry interpolation, the override
  lookup, and the repo-root check out of `destinations` so both callers share one copy — a
  directory item's destination rendered with a trailing `/`
- [x] 5.3 VERIFY: Confirm `internal/plan`'s purity test still passes, and that no new test
  needs a real directory
- [x] 5.4 Run `go test ./internal/plan/` — no regressions

## 6. Parsing the add invocation
<!-- kind: behavior -->

- [x] 6.1 RED: Write failing tests for: *Three spellings of one repository derive one name*,
  *A git value with no usable last segment is refused*, and the `@rev` split, including that
  `git@github.com:optioni/shared` keeps its `@`
- [x] 6.2 GREEN: Implement the derivation and the `@rev` split in `internal/add`, pure over
  strings
- [x] 6.3 REFACTOR: Name the two rules where they are readable, or record that none is needed
- [x] 6.4 Run `go test ./internal/add/` — no regressions

## 7. A sync that honours pre-edited manifest bytes
<!-- kind: behavior -->

- [x] 7.1 RED: Write failing tests for a run given manifest bytes: the run resolves what the
  bytes say, writes exactly those bytes, and never re-reads `graft.toml` from disk
- [x] 7.2 RED: Write a failing test asserting `Options.Manifest` and `Update.To` are never
  both set — the two-sources-of-bytes risk design.md names
- [x] 7.3 GREEN: Add `sync.Options.Manifest`, threading it through `Run` beside the existing
  `movePin` path
- [x] 7.4 CHECK: Contract gate — confirm `graft.lock`'s format and its `version` are untouched
  by this change, and that a lock written before it still loads
- [x] 7.5 Run `go test ./internal/sync/` — no regressions

## 8. The add sequence
<!-- kind: behavior -->

- [x] 8.1 RED: Write failing tests for: *An unparsable graft.toml is refused before anything
  is resolved*, *A new selector joins an existing source*, *A selector
  already declared is not written twice*, *The same selector given twice is written once*, *A
  different repository under a taken name is refused*, *An unamendable manifest is refused in
  the amender's words*
- [x] 8.2 RED: Write failing tests for the amendment's read-back check — bytes that parse but
  do not declare what was asked for fail with `the amendment did not take effect`
- [x] 8.3 GREEN: Implement `add.Run` — read the manifest or treat its absence as empty bytes,
  derive, resolve the default rev only when no `@rev` was given, amend, re-parse, check
- [x] 8.4 GREEN: Implement the two tails: `--no-sync` through `apply.Manifest`, and the
  syncing path through `sync.Run` with `Options.Manifest` set, plus `Update{Source: name}`
  only when the pin actually moved
- [x] 8.5 RED then GREEN: *Adding a selector to a branch pin does not move it* — the lock's
  sha survives an `add` that names no rev
- [x] 8.6 REFACTOR: Keep the sequence readable as one ordered function, as `sync.Run` is
- [x] 8.7 Run `go test ./internal/add/ ./internal/sync/` — no regressions

## 9. Reporting the manifest edit
<!-- kind: behavior -->

- [ ] 9.1 RED: Write failing tests for: *Adding a source reports one line*, *Moving a pin and
  adding a selector reports both, in order*, *An invocation that changes nothing says so*
- [ ] 9.2 GREEN: Return the edit summary from `add.Run` and render it through `internal/ui`
  on the error stream, before the sync report
- [ ] 9.3 VERIFY: Assert standard output stays byte-empty on every syncing `add`
- [ ] 9.4 Run `go test ./internal/add/ ./internal/cli/` — no regressions

## 10. `--list`
<!-- kind: behavior -->

- [ ] 10.1 RED: Write failing tests for: *A catalog is listed with its destinations*, *A
  consumer override is reflected in the listing*, *A source offering no items lists none*, *An
  ungraftable source is refused under --list*
- [ ] 10.2 GREEN: Implement the listing — resolve, fetch, read the catalog, compute
  destinations, pad ids to a common width, sort ascending, print to standard output
- [ ] 10.3 VERIFY: Assert two runs against one catalog print byte-identical text, and that the
  repository holds no `graft.toml`, no `graft.lock`, and no written file afterwards
- [ ] 10.4 Run `go test ./internal/add/ ./internal/cli/` — no regressions

## 11. The command surface
<!-- kind: behavior -->

- [ ] 11.1 RED: Write failing tests for: *No arguments*, *An empty source argument*, *--list
  with selectors*, *--list with --no-sync*, *An unknown flag*, *An empty rev is a usage
  error*, *A malformed selector is refused before the network*, *No selectors, no TTY*, *No
  selectors on a terminal is the same refusal*
- [ ] 11.2 GREEN: Implement `internal/cli/add.go` — the argument validator, the two flags, and
  the call into `internal/add`; wiring only, no decision of its own
- [ ] 11.3 GREEN: Register `add` on the root command
- [ ] 11.4 VERIFY: Confirm every refusal above happens before any `git` process runs
- [ ] 11.5 Run `go test ./internal/cli/` — no regressions

## 12. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [ ] 12.1 VERIFY: Confirm the group 0 acceptance test passes end to end
- [ ] 12.2 RED then GREEN: The remaining end-to-end scenarios — *A second source is added
  beside an existing one*, *Several selectors are written in the order given*, *A selector
  matching nothing leaves graft.toml unwritten*, *An explicit rev is written verbatim*, *A
  source with tags gets its highest tag as the default pin*, *A source with no tags gets its
  default branch*, *An explicit rev on a declared source moves the pin*, *The manifest is
  written and nothing else is*, *An unverified selector is written*, *An unreachable source
  leaves no manifest behind*
- [ ] 12.3 VERIFY: Assert `--no-sync` with `@rev` succeeds against a source path no repository
  exists at — the only honest proof that it makes no network call
- [ ] 12.4 REFACTOR: Fold the acceptance harness into the shape the existing `sync` and
  `update` acceptance tests use

## 13. Documentation
<!-- kind: operational -->

- [ ] 13.1 CHECK: Re-read CLAUDE.md's rule *`graft.toml` is a human's file* and confirm it now
  describes only half of what graft writes — it speaks of moving a pin, not of appending a
  table or amending an array
- [ ] 13.2 CHANGE: Rewrite that rule in place to cover all three edits and the insertion
  anchor the amendment uses, deleting what is now too narrow rather than appending beside it
- [ ] 13.3 VERIFY: Confirm the rule is still under five lines and that no other document
  repeats it

## 14. Change Review
<!-- kind: operational -->

- [ ] 14.1 CHECK: Dispatch an independent `outside-in-tdd-reviewer` — never a fork of this
  session — against proposal.md, every spec scenario, design.md, and tasks.md
- [ ] 14.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a
  one-line reason, note SUGGESTIONs, and re-run affected tests
- [ ] 14.3 VERIFY: Confirm no blocking or unowned finding remains

## 15. Lint & Verify
<!-- kind: operational -->

- [ ] 15.1 CHECK: Inspect the intended verification commands and the tiers they cover
- [ ] 15.2 VERIFY: Run `task lint` — 0 errors
- [ ] 15.3 VERIFY: Run `task test` — green
- [ ] 15.4 VERIFY: Run `task cover` — the 80% floor over `./internal/...` holds
- [ ] 15.5 VERIFY: Run `task build` — the binary builds
- [ ] 15.6 VERIFY: Run `openspec validate add-command --strict`
