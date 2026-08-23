<!-- No outer-loop acceptance group. This change adds no command, flag, or output, so there is
     no end-to-end entry point to drive. Recorded in design.md → Test Strategy. -->

## 1. YAML decoder dependency
<!-- kind: operational -->

- [x] 1.1 CHECK: Confirm the module's current dependency set (`go list -m all` shows only `github.com/optioni/graft` and `github.com/BurntSushi/toml`) and confirm the latest published version with `go list -m -versions github.com/goccy/go-yaml`
- [x] 1.2 CHECK: Confirm the chosen decoder has no transitive dependencies — `go mod graph` after the install must add exactly one line — because design.md → Decisions rests on that
- [x] 1.3 CHANGE: `go get github.com/goccy/go-yaml@v1.19.2` (the latest published version, confirmed in 1.1), this is the one step in the change that reaches the Go module proxy, as design.md → Test Boundaries records. `go mod tidy` is deferred to group 2: with no importer yet it would drop the dependency again, so the require line lands marked `// indirect` and tidy promotes it once `internal/catalog` imports it
- [x] 1.4 VERIFY: `task build` succeeds and `go.sum` is committed alongside `go.mod` — the real build is the check; do not write a test asserting the dependency is listed

## 2. `internal/catalog` — document shape, version gating, and loading
<!-- kind: behavior -->

- [x] 2.1 RED: Write failing tests for the loading scenarios — `A valid catalog loads` (SPEC.md's own example, written into a `t.TempDir()`, asserting the tree is unchanged), `A missing catalog is the not-graftable error` asserting the exact text `catalog.yaml not found: the source is not graftable`, and `Malformed YAML is an error` asserting the `catalog.yaml: ` prefix only
- [x] 2.2 RED: Write failing table tests for the document-shape scenarios — `A catalog that is not a mapping is an error`, `An empty catalog file is a missing-version error`, `A catalog with zero provides loads`, `A catalog with neither kinds nor provides loads` — each asserting the exact message or the empty result from specs/catalog-format/spec.md
- [x] 2.3 RED: Write failing table tests for `A missing version is an error`, `A newer version fails and says to upgrade` (input also carries an unknown top-level key, and the test asserts the upgrade message wins and no catalog is returned), `A version below 1 is an error`
- [x] 2.4 Confirm every test above fails because `internal/catalog` does not exist, not because a YAML literal is malformed in a way the test did not intend
- [x] 2.5 GREEN: Add `Catalog`, `Kind`, `Item`, and `Parse([]byte, filename string) (*Catalog, error)` decoding the document into `map[string]any` with `yaml.Unmarshal`, treating an empty document as an empty mapping and any non-mapping document as an error
- [x] 2.6 GREEN: Add version gating ahead of every other check — required, `0` and below rejected, `> 1` rejected with the upgrade message, and no partially decoded catalog returned on any of them; accept the integer kinds the decoder actually produces (`uint64`, `int64`, `int`) and reject non-integers
- [x] 2.7 GREEN: Add `Load(path string) (*Catalog, error)` — read the file, return `catalog.yaml not found: the source is not graftable` when absent, otherwise delegate to `Parse`. It reads only; it creates, modifies, and deletes nothing
- [x] 2.8 REFACTOR: Collapse the repeated `catalog.yaml: ...` formatting into one helper while tests stay green, or record that no refactor was warranted — done: `errf(filename, format, args...)` is the single place the prefix is built; only the two `%w` wraps around a decoder error stay separate, because they must wrap rather than format
- [x] 2.9 CHECK: Contract gate — re-read SPEC.md's `catalog.yaml` section and ENGINEERING.md's Compatibility section, and confirm the accepted `version` value and the never-half-read-a-newer-file rule still match what `Parse` does
- [x] 2.10 Run `go test -race ./internal/catalog/...` — green, no regressions

## 3. Kind declarations
<!-- kind: behavior -->

- [x] 3.1 RED: Write failing tests for `A string-valued to is carried verbatim` (one-element `To`, `{name}` uninterpolated, `Flatten` false), `A trailing slash is preserved`, `A list-valued to is carried in declared order`
- [x] 3.2 RED: Write failing table tests for `An empty kind name is an error`, `A missing or empty to is an error` (absent, `""`, and `[]` — one message), `An empty destination inside a list is an error`, `A to of the wrong type is an error` (a mapping and a number — one message), `A repeated destination within one kind is an error` — each asserting the exact text from specs/catalog-format/spec.md
- [x] 3.3 Confirm the failures come from missing kind parsing, not from group 2's version gate rejecting the fixtures first
- [x] 3.4 GREEN: Add kind extraction — every key of the `kinds` mapping with a non-empty name, walked in sorted key order so a catalog with two invalid kinds always reports the same one, `to` normalised to `[]string` through a type switch over `string` and `[]any`, and `flatten` read as a bool defaulting to `false`
- [x] 3.5 GREEN: Add the `to` validation rules — required, no empty element, no repeated destination — producing the exact error strings the spec names, and carry every destination verbatim with no cleaning, no interpolation, and no trailing-slash handling
- [x] 3.6 REFACTOR: Reduce the value-extraction helpers (`string`, `bool`, `[]string`) to one place so a future field cannot be read one way here and another way in group 4, or record that no refactor was warranted — done: `values.go` now holds every read of a value out of the decoded document, and group 4 reads its strings through it
- [x] 3.7 CHECK (confirmed: nothing in the package calls `path.Join`, `filepath.*`, `strings.Replace` on a destination, or reads `Flatten` — `to` is stored exactly as decoded): Concentration point — confirm no destination semantics leaked in: nothing in this package interpolates `{name}`, resolves a trailing `/`, applies `flatten`, or joins a destination to a path. Those belong to `destination-and-plan`, and splitting them across two packages is how the invariant "no destination escapes the repo root" gets checked in only one of them
- [x] 3.8 CHECK: Contract gate — re-read SPEC.md's `kinds` bullets and confirm the accepted shapes (`to` as string or list, `flatten`) and their optionality still match what `Parse` accepts
- [x] 3.9 Run `go test -race ./internal/catalog/...` — green, no regressions

## 4. Provided items
<!-- kind: behavior -->

- [x] 4.1 RED: Write failing tests for `Items are carried with kind, name, and from` and `Items are ordered by id` — the latter building the `provides` list in a deliberately wrong order and asserting the sorted ids
- [x] 4.2 RED: Write failing table tests for `A missing field is an error` (three inputs, three exact messages, each naming the 0-based `provides[0]`), `A kind or name containing a colon is an error`, `An item naming an undeclared kind is an error`, `A duplicate item is an error`
- [x] 4.3 RED: Write a failing table test for `A from outside the source tree is an error` covering `../outside`, `/etc/passwd`, `.`, and `./extras/tdd`, each asserting the exact message with the value as written
- [x] 4.4 Confirm the failures come from missing item parsing, not from a fixture whose kind is undeclared when the case is about something else
- [x] 4.5 GREEN: Add item extraction — `kind`, `name`, and `from` required, the id built as `kind:name` and checked with `internal/itemid.Valid` rather than a second grammar, and the undeclared-kind and duplicate-id checks
- [x] 4.6 GREEN: Add the `from` containment rule — non-empty, relative, cleaned, and free of any `..` segment, with `.` rejected because it names the whole source tree
- [x] 4.7 GREEN: Sort the parsed items by id with byte-wise string comparison, so ordering cannot depend on map iteration, locale, or platform
- [x] 4.8 REFACTOR: Collapse the repeated `catalog.yaml: item %q: ...` and `catalog.yaml: provides[%d]: ...` formatting into helpers while tests stay green, or record that no refactor was warranted — done: the two prefixes are built by the `at` and `item` closures, one per entry, and the required-string read moved to `values.go`'s `field` so kinds and items read a value the same way
- [x] 4.9 CHECK (confirmed: `internal/catalog` calls `itemid.Valid` and contains no colon-splitting of its own outside the selector matcher): Concentration point — confirm the item id grammar is `internal/itemid`'s and is not restated here. `graft.toml` selectors, `graft.lock` item ids, and catalog items must agree on the grammar or a selector will silently fail to match an id the lock happily records
- [x] 4.10 CHECK: Contract gate — re-read SPEC.md's `provides` bullets and the "Item identity is `kind:name`" claim, and confirm the accepted fields and the containment rule on `from` still match
- [x] 4.11 Run `go test -race ./internal/catalog/...` — green, no regressions

## 5. Unknown-key rejection
<!-- kind: behavior -->

- [x] 5.1 RED: Write failing table tests for `An unknown top-level key is an error`, `An unknown key inside a kind is an error`, `An unknown key inside a provides entry is an error` — the last asserting the 0-based index in `provides[1]` — each with the exact text from specs/catalog-format/spec.md
- [x] 5.2 Confirm the failures come from the missing walk, not from the value extraction rejecting the fixture for an unrelated reason, and confirm the group 2 case asserting the upgrade message beats an unknown key is still green
- [x] 5.3 GREEN: Add the unknown-key walk over the decoded `map[string]any` — top level, each `kinds.<kind>` mapping, and each `provides` entry by index — reporting one key deterministically when several are unknown
- [x] 5.4 REFACTOR: Compare this walk with `internal/lock`'s and either extract the shared part or record why they stay separate — kept separate. Read side by side, the two walks share only `unknownKey`, a ten-line lowest-sorting-key loop. Everything else differs: lock nests source then item and attributes by name, catalog nests kind then provides index and attributes by position; lock needs `tables()` because TOML spells an array of tables two ways, which YAML does not. A third package holding the ten lines would tie two separately asserted error contracts together for no reduction in either
- [x] 5.5 CHECK (confirmed: the allowed sets are exactly SPEC.md's documented keys — `version`, `kinds`, `provides`; `to`, `flatten`; `kind`, `name`, `from` — and `requires` is rejected rather than accepted-and-ignored, which is what keeps SPEC.md's open question askable at version 2): Contract gate — re-read SPEC.md's `catalog.yaml` section and confirm no documented key is rejected by the walk, and that `requires` — SPEC.md's own open question — is rejected rather than accepted-and-ignored
- [x] 5.6 Run `go test -race ./internal/catalog/...` — green, no regressions

## 6. Selector expansion and the no-match error
<!-- kind: behavior -->

- [ ] 6.1 RED: Write failing table tests for `A plain selector selects exactly one item`, `Several selectors produce the union ordered by id` (selectors given in reverse id order), `Overlapping selectors yield each item once` (asserting length as well as ids), `An empty selector list expands to nothing`
- [ ] 6.2 RED: Write failing table tests for `A misspelled selector is an error listing what the catalog provides`, `One selector matching does not excuse another that does not` (asserting no items are returned even though the first selector matched), `Any selector against a catalog providing zero items is an error` — each asserting the exact message including the ordered provides list
- [ ] 6.3 Confirm the failures come from the missing `Expand`, not from a hand-built `*Catalog` whose items are out of order
- [ ] 6.4 GREEN: Implement `Expand(c *Catalog, source string, selectors []string) ([]Item, error)` — match each selector, union the results, deduplicate by id, and return sorted by id
- [ ] 6.5 GREEN: Implement the no-match error — checked per selector so one match cannot excuse another, naming the source and the selector, and listing every provided id in ascending order, or `no items` when the catalog provides none
- [ ] 6.6 CHECK: Concentration point — confirm `Expand` reads no file, runs no command, and touches no global state; every test builds its `*Catalog` in memory. A test that needs a real directory to exercise expansion is a signal the boundary moved
- [ ] 6.7 REFACTOR: Reduce the provides listing to one formatting helper shared by every no-match path, or record that no refactor was warranted
- [ ] 6.8 CHECK: Contract gate — re-read SPEC.md's `install` bullet and the failure-mode row "A selector matches no item", and confirm the error lists what the catalog provides rather than only naming the selector
- [ ] 6.9 Run `go test -race ./internal/catalog/...` — green, no regressions

## 7. Globs in the name position
<!-- kind: behavior -->

- [ ] 7.1 RED: Write failing table tests for `A trailing star selects every item of a kind`, `A prefix glob selects a subset`, `A question mark matches exactly one character` (the catalog holds both `schema:td` and `schema:tdd`)
- [ ] 7.2 RED: Write failing table tests for `A glob matching nothing is an error` (`hook:*` against a catalog with no `hook` items), `The kind position is matched literally` (asserting the exact no-match message for `*:tdd`), and `A malformed glob pattern is an error` (asserting the exact message for `agent:[tdd` and that no items are returned)
- [ ] 7.3 Confirm the glob cases fail because matching is still literal, and the malformed case fails because the pattern error is still swallowed into a no-match
- [ ] 7.4 GREEN: Split each selector at its single colon, compare the kind literally, and match the name with `path.Match`
- [ ] 7.5 GREEN: Surface `path.ErrBadPattern` as the exact invalid-pattern message instead of treating the selector as a literal name — a bad pattern is a typo, and typo protection is the whole point of the no-match rule
- [ ] 7.6 REFACTOR: Fold the plain-selector path into the glob path if a literal name is now just a pattern with no metacharacters, keeping group 6's tests unchanged, or record that no refactor was warranted
- [ ] 7.7 CHECK: Contract gate — re-read SPEC.md's `install` bullet and confirm the glob lives in the name position only, and that the resolved reading recorded in design.md → Open Questions still matches the rest of SPEC.md
- [ ] 7.8 Run `go test -race ./internal/catalog/...` — green, no regressions

## 8. Documentation
<!-- kind: operational -->

- [ ] 8.1 CHECK: Re-read `openspec/config.yaml` → `context` and confirm the clause "Cobra is the intended command surface but is not added yet — the module has no dependencies" is now false: `manifest-and-lock` added `github.com/BurntSushi/toml` and this change adds a second dependency
- [ ] 8.2 CHECK: Confirm no AGENTS.md or ENGINEERING.md change is warranted — the architecture tables already name `internal/catalog` as "catalog.yaml, selector expansion", and the concentration points this change touches (asserted error strings, `plan` purity, the `./internal/...` coverage scope) are already written there. This change adds no new recurring pitfall, and a rule about one package would not belong in the root file anyway
- [ ] 8.3 CHANGE: Rewrite that clause in `openspec/config.yaml` so it says Cobra is not added yet and names dependencies as something to check rather than asserting there are none. Document: `openspec/config.yaml`; audience: every future planning session, which loads this text as project context; section: `context`; durable reason: a stale "no dependencies" claim gets read as a constraint and will argue against a dependency a change legitimately needs. This rewrites one clause and adds no net content
- [ ] 8.4 VERIFY: Run `openspec instructions proposal --change catalog-and-selectors --json` and confirm the corrected `context` is what the tool now serves

## 9. Change Review
<!-- kind: operational -->

- [ ] 9.1 CHECK: Dispatch an independent reviewer — one that did not write the implementation and is given only proposal.md, both spec files, design.md, tasks.md, and the diff
- [ ] 9.2 CHECK: Point the reviewer at this repository's concentration points — (a) every asserted error string matches the spec text character for character, and any deviation is a deliberate contract change rather than a typo; (b) `internal/plan` gained no code and no test needs a real directory to exercise parse or expansion logic; (c) this change computes no prune set and deletes nothing, so no foreign-file test applies — confirm that is still true rather than assuming it; (d) item order is by id and is asserted, because the lock's determinism depends on the ids arriving in that order; (e) no fixture git repository was introduced, so no `user.name`/`user.email` hazard exists here; (f) no logic landed in `cmd/graft`, where the coverage gate cannot see it; (g) no code path lets a source repository cause anything to execute — the catalog is read and validated, never run
- [ ] 9.3 CHECK: Confirm no test asserts a `go.mod` key or restates a literal the implementation also declares — the build is the check for the dependency
- [ ] 9.4 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a one-line reason, note SUGGESTIONs, and re-run affected tests
- [ ] 9.5 VERIFY: Confirm no blocking or unowned finding remains

## 10. Lint & Verify
<!-- kind: operational -->

- [ ] 10.1 CHECK: Inspect the intended verification commands and affected tiers — the new package is under `./internal/...`, so the coverage gate applies; there is no acceptance tier in this change
- [ ] 10.2 VERIFY: Run `task lint` — golangci-lint reports 0 issues and `gofumpt -l .` prints nothing
- [ ] 10.3 VERIFY: Run `task cover` — suite green under `-race` and total coverage over `./internal/...` at or above the 80% floor
- [ ] 10.4 VERIFY: Run `task build` — the binary compiles with the version ldflags
- [ ] 10.5 VERIFY: Run `openspec validate catalog-and-selectors --strict` — valid
