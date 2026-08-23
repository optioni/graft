<!-- No outer-loop acceptance group. This change adds no command, flag, or output, so there is
     no end-to-end entry point to drive. Recorded in design.md → Test Strategy. -->

## 1. TOML decoder dependency
<!-- kind: operational -->

- [x] 1.1 CHECK: Confirm the module currently has zero dependencies (`go list -m all` shows only `github.com/optioni/graft`) and confirm the latest published version with `go list -m -versions github.com/BurntSushi/toml`
- [x] 1.2 CHANGE: `go get github.com/BurntSushi/toml@v1.6.0` (or the newer latest found in 1.1), then `go mod tidy` — this is the one step in the change that reaches the Go module proxy, as design.md → Test Boundaries records
- [x] 1.3 VERIFY: `task build` succeeds and `go.sum` is committed alongside `go.mod` — the real build is the check; do not write a test asserting the dependency is listed

## 2. `internal/manifest` — parse and validate `graft.toml`
<!-- kind: behavior -->

- [x] 2.1 RED: Write failing tests for the loading scenarios — `Minimal valid manifest loads`, `Manifest with no sources is valid`, `Malformed TOML is an error` (prefix `graft.toml: ` only), and `Missing manifest is an error` in a `t.TempDir()` asserting the exact text `graft.toml not found`
- [x] 2.2 RED: Write failing table tests for the required-field scenarios — `Missing git is an error`, `Missing rev is an error`, `Empty install list is an error` (both `install = []` and absent), `Empty source name is an error` — each asserting the exact message from specs/manifest-format/spec.md
- [x] 2.3 RED: Write failing tests for the selector scenarios — `Plain and glob selectors are accepted` (verbatim, unexpanded, unreordered), `A selector with no kind separator is an error`, `A selector with an empty half is an error` (`schema:`, `:tdd`, `schema:tdd:extra`), `A duplicate selector is an error`
- [x] 2.4 RED: Write failing tests for `An override is carried verbatim`, `No kinds table means no overrides`, `An empty override destination is an error`, `A misspelled source field is an error`, `An unknown top-level key is an error`, `Shorthand is not expanded`, `A full URL is not rewritten`
- [x] 2.5 Confirm every test above fails because `internal/manifest` does not exist, not because a fixture is malformed
- [x] 2.6 GREEN: Add `Manifest`, `Source`, and `Parse([]byte, filename string) (*Manifest, error)` decoding with `toml.Decode` and rejecting `MetaData.Undecoded()` keys
- [x] 2.7 GREEN: Add field validation — non-empty source name, `git`, `rev`, and at least one selector — producing the exact error strings the spec names
- [x] 2.8 GREEN: Add selector syntax validation (`kind:name`, exactly one colon, both halves non-empty, `*`/`?` allowed in the name) and duplicate-selector detection, preserving declared order
- [x] 2.9 GREEN: Add `kinds` override decoding with verbatim destinations and the empty-destination error
- [x] 2.10 GREEN: Add `Load(path string) (*Manifest, error)` — read the file, return `graft.toml not found` when absent, otherwise delegate to `Parse`. It reads only; it creates, modifies, and deletes nothing
- [x] 2.11 REFACTOR: Collapse the repeated `graft.toml: source "%s": ...` formatting into one helper while tests stay green, or record that no refactor was warranted
- [x] 2.12 CHECK: Contract gate — re-read SPEC.md's `graft.toml` section and confirm every field, its optionality, and the selector grammar still match what `Parse` accepts, and that no consumer-visible field was added or dropped
- [x] 2.13 Run `go test -race ./internal/manifest/...` — green, no regressions

## 3. `internal/lock` — parse and validate `graft.lock`
<!-- kind: behavior -->
<!-- parallel-after: 1 -->

- [x] 3.1 RED: Write failing tests for `An absent lock loads as an empty lock` in a `t.TempDir()` — version 1, zero sources, nil error, and assert no `graft.lock` was created — plus `A lock with zero sources loads`, `A populated lock loads`, and `Malformed TOML is an error` (prefix `graft.lock: ` only)
- [x] 3.2 RED: Write failing table tests for `A missing version is an error`, `A newer version fails and says to upgrade` (also asserting no source is returned), `A version below 1 is an error`
- [x] 3.3 RED: Write failing table tests for `A missing resolved sha is an error`, `A malformed resolved sha is an error`, `A duplicate source name is an error`, `A malformed item id is an error`, `A duplicate item id within a source is an error`, `An unknown key is an error`
- [x] 3.4 RED: Write failing tests for `An item with no files is valid`, `A duplicate file within an item is an error`, and `An escaping file path is an error` for both `../outside.md` and `/etc/passwd`
- [x] 3.5 Confirm every test above fails because `internal/lock` does not exist, not because a fixture is malformed
- [x] 3.6 GREEN: Add `Lock`, `Source`, `Item`, and `Parse([]byte, filename string) (*Lock, error)` with strict decoding
- [x] 3.7 GREEN: Add version gating — required, `0` rejected, `> 1` rejected with the upgrade message, and no partially decoded lock returned on any of them
- [x] 3.8 GREEN: Add source validation — non-empty `name`, `git`, `rev`, uniqueness by name, and `resolved` as 40 lowercase hex characters
- [x] 3.9 GREEN: Add item validation — `id` in `kind:name` form, unique within its source, `files` entries non-empty, relative, free of any `..` segment, and unique within the item; an empty `files` list is accepted
- [x] 3.10 GREEN: Add `Load(path string) (*Lock, error)` returning an empty lock at version 1 when the file is absent, reading only
- [x] 3.11 REFACTOR: Collapse the repeated `graft.lock: source "%s": ...` formatting inside this package while tests stay green, or record that no refactor was warranted — cross-package extraction waits for group 4, so this group stays independent of group 2
- [x] 3.12 CHECK: Contract gate — re-read SPEC.md's `graft.lock` section and confirm the accepted fields, the `version = 1` value, and the meaning of `files` still match; confirm the escaping-path rule protects the prune mechanism without duplicating `destination-and-plan`'s invariant over computed destinations
- [x] 3.13 Run `go test -race ./internal/lock/...` — green, no regressions

## 4. Shared `kind:name` grammar
<!-- kind: refactor -->

- [ ] 4.1 CHARACTERIZE: Confirm groups 2 and 3 are both green and that the selector-syntax and item-id tests already pin the grammar's accepted and rejected forms in each package
- [ ] 4.2 REFACTOR: Move the duplicated `kind:name` check to one place both packages call, without editing a single characterization test — the two error strings differ and must stay differing, so only the predicate moves, not the message
- [ ] 4.3 VERIFY: Run `go test -race ./internal/manifest/... ./internal/lock/...` — the unchanged tests stay green

## 5. Canonical lock serialization
<!-- kind: behavior -->

- [ ] 5.1 RED: Add `internal/lock/testdata/canonical.lock` holding the exact bytes from `A populated lock serializes to the documented layout`, and write `TestMarshal_Golden` comparing `Marshal` output to that file byte for byte
- [ ] 5.2 RED: Write failing tests for `A files list of one is inline and a list of many is exploded` (0, 1, and 2 files), `An empty lock serializes to header and version only`, and `A path needing escaping round-trips`
- [ ] 5.3 Confirm the failures come from the missing serializer, not from a stale or mis-encoded golden file (check the golden file is UTF-8, LF-only, and ends in exactly one newline)
- [ ] 5.4 GREEN: Implement `Marshal(*Lock) []byte` — header comment, `version = 1`, one `[[source]]` block per source with `name`/`git`/`rev`/`resolved` padded to align on `=`, each `[[source.item]]` indented two spaces with `id`/`files` aligned, exactly one blank line between blocks, LF endings, one trailing newline
- [ ] 5.5 GREEN: Implement the `files` layout rule — `[]` when empty, inline when one, exploded with four-space-indented entries and a trailing comma after the last when two or more
- [ ] 5.6 GREEN: Implement TOML basic-string quoting for every emitted value so a path containing `"` or `\` is escaped
- [ ] 5.7 CHECK: Confirm `Marshal` returns bytes and writes nothing — no `os.Create`, `os.WriteFile`, or `io.Writer` sink anywhere in `internal/lock`; `internal/apply` is the only package permitted to write, and it does so in `sync-command`
- [ ] 5.8 REFACTOR: Reduce the writer to one `strings.Builder` pass with no per-line intermediate allocation, or record that no refactor was warranted
- [ ] 5.9 CHECK: Contract gate — re-read SPEC.md's `graft.lock` example and confirm the emitted layout matches it field for field, including key alignment, the two-space `[[source.item]]` indent, and the trailing comma in an exploded `files` array
- [ ] 5.10 Run `go test -race ./internal/lock/...` — green, no regressions

## 6. Determinism and byte-stable round trip
<!-- kind: behavior -->

- [ ] 6.1 RED: Write `TestMarshal_OrderIndependent` for `Input order does not change output` — build two locks with identical content, one ascending and one with sources, items, and files all reversed, and assert the marshalled bytes are equal and ordered name-, id-, and path-ascending
- [ ] 6.2 RED: Write `TestMarshal_Twice` for `Serializing the same lock twice is byte-identical`, asserting with `bytes.Equal` rather than comparing parsed values
- [ ] 6.3 RED: Write `TestRoundTrip_Canonical` for `Canonical bytes survive a parse and serialize` against `testdata/canonical.lock`
- [ ] 6.4 RED: Add `internal/lock/testdata/scrambled.lock` — same content, reversed order, single-space padding, no header comment — and write `TestRoundTrip_Normalizes` for `Non-canonical input is normalized, then stable`, asserting the first marshal equals `canonical.lock` and a second parse-and-marshal returns the identical bytes
- [ ] 6.5 Confirm these fail on ordering and normalization, not on a golden file that disagrees with itself
- [ ] 6.6 GREEN: Sort sources by name, items by id, and files by path with byte-wise string comparison inside `Marshal`, so ordering cannot depend on map iteration, locale, or platform
- [ ] 6.7 REFACTOR: Hoist the three sorts into one normalization step so a future field cannot be sorted in one code path and not another, or record that no refactor was warranted
- [ ] 6.8 CHECK: Concentration point — confirm the determinism assertions compare bytes, not parsed values; a semantic comparison here would pass while every sync churned the git diff
- [ ] 6.9 CHECK: Contract gate — re-read SPEC.md's ordering bullet and confirm `Marshal` implements "sources by name, items by id, files by path", and that the SPEC example's item order is reconciled in the documentation group below
- [ ] 6.10 Run `go test -race ./internal/lock/...` — green, no regressions

## 7. Manifest and lock pin agreement
<!-- kind: behavior -->

- [ ] 7.1 RED: Write a failing table test covering `Agreeing pins pass`, `A moved manifest pin is an error` with the exact message, `A source only in the manifest is not an error`, `A source only in the lock is not an error`, and `Two empty files agree`
- [ ] 7.2 Confirm the failures come from the missing check, not from a manifest fixture that fails validation first
- [ ] 7.3 GREEN: Implement `CheckPins` in `internal/lock`, comparing `rev` only for sources present in both and returning the exact drift message pointing at `graft update`
- [ ] 7.4 REFACTOR: None expected for a single comparison — state explicitly that no refactor was needed, or make it and keep the tests unchanged
- [ ] 7.5 CHECK: Confirm the dependency direction is `lock` → `manifest` and that `internal/plan` gained no code, so plan purity is untouched by this change
- [ ] 7.6 Run `go test -race ./internal/lock/... ./internal/manifest/...` — green, no regressions

## 8. Documentation
<!-- kind: operational -->

- [ ] 8.1 CHECK: Re-read SPEC.md's `graft.lock` section and confirm the example contradicts the ordering bullet directly beneath it — the example prints `schema:tdd` before `agent:apply-orchestrator`, while the rule says items sort by id
- [ ] 8.2 CHECK: Confirm no AGENTS.md change is warranted — lock determinism, asserted error strings, the `apply`-is-sole-writer rule, and the `./internal/...` coverage scope are already written there, and this change adds no new recurring pitfall
- [ ] 8.3 CHANGE: Rewrite that example in SPEC.md so its two `[[source.item]]` blocks appear id-ascending. Document: SPEC.md; audience: anyone implementing or reviewing the lock; section: `graft.lock — what was actually installed`; durable reason: the example is the format contract a future change will copy, and an example that violates its own rule will be copied instead of the rule. This reorders existing lines and adds no net content
- [ ] 8.4 VERIFY: Re-read the edited SPEC.md section against `internal/lock/testdata/canonical.lock` and confirm the two agree on order

## 9. Change Review
<!-- kind: operational -->

- [ ] 9.1 CHECK: Dispatch an independent reviewer — one that did not write the implementation and is given only proposal.md, both spec files, design.md, tasks.md, and the diff
- [ ] 9.2 CHECK: Point the reviewer at this repository's concentration points — (a) lock serialization determinism is asserted as byte equality across two runs, not semantic equality; (b) every asserted error string matches the spec text character for character, and any deviation is a deliberate contract change rather than a typo; (c) `internal/plan` gained no code and no test needs a real directory to exercise parse logic; (d) this change computes no prune set and deletes nothing, and the lock's relative-path rule is what keeps a corrupt lock from aiming a future deletion outside the repo; (e) no fixture git repository was introduced, so no `user.name`/`user.email` hazard exists here; (f) no logic landed in `cmd/graft`, where the coverage gate cannot see it
- [ ] 9.3 CHECK: Confirm no test asserts a `go.mod` key or restates a literal the implementation also declares — the build and the round-trip tests are the checks
- [ ] 9.4 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a one-line reason, note SUGGESTIONs, and re-run affected tests
- [ ] 9.5 VERIFY: Confirm no blocking or unowned finding remains

## 10. Lint & Verify
<!-- kind: operational -->

- [ ] 10.1 CHECK: Inspect the intended verification commands and affected tiers — both new packages are under `./internal/...`, so the coverage gate applies; there is no acceptance tier in this change
- [ ] 10.2 VERIFY: Run `task lint` — golangci-lint reports 0 issues and `gofumpt -l .` prints nothing
- [ ] 10.3 VERIFY: Run `task cover` — suite green under `-race` and total coverage over `./internal/...` at or above the 80% floor
- [ ] 10.4 VERIFY: Run `task build` — the binary compiles with the version ldflags
- [ ] 10.5 VERIFY: Run `openspec validate manifest-and-lock --strict` — valid
