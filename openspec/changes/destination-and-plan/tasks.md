<!-- No outer-loop acceptance group. design.md → Test Strategy records why: no command
     exists yet (`command-surface` and `sync-command` are later changes) and `internal/apply`
     — the only thing that could make a plan observable in a tree — does not exist, so an
     end-to-end test here could only drive an invented harness, which is a boundary the Test
     Boundaries table does not name. -->

## 1. Package scaffold and the purity guard
<!-- kind: operational -->

The concentration point this whole package exists to hold is that `internal/plan` never
reaches the filesystem. It is enforced by a guard, and a guard nobody has watched fail is
not a guard.

- [x] 1.1 CHECK: Create `internal/plan/plan.go` with only the package doc comment stating
      that `plan` is pure — values in, a plan value out, no file read, no command run,
      nothing created, modified, or deleted — and confirm `go build ./internal/plan/`
      succeeds
- [x] 1.2 CHANGE: Add `internal/plan/purity_test.go` with `TestPackageImportsNothingImpure`,
      parsing every non-test `.go` file in the package directory with `go/parser` and failing
      on any import of `os`, `io/fs`, `path/filepath`, `os/exec`, `net`, or `net/http`.
      Assert the failure message names the offending file and import
- [x] 1.3 VERIFY: Temporarily add `import "os"` (with a `var _ = os.Getenv`) to `plan.go`,
      run `go test ./internal/plan/ -run Impure` and confirm it FAILS naming that import,
      then remove the import and confirm it passes. Record that this is the CHECK step for
      an invariant no ordinary RED can express
- [x] 1.4 VERIFY: Run `go test ./internal/plan/` — green

*Covers spec scenario* **Computing destinations touches nothing** — the "touches nothing"
half; its destination table case lands in group 2.

## 2. Destination of one item
<!-- kind: behavior -->

`{name}` interpolation and the trailing-slash rule, for a `from` that is a directory and for
a `from` that is a file. This is the group that fixes design.md → D4.

- [x] 2.1 RED: Write failing tests for: *A directory item preserves its structure under an
      interpolated destination*; *A trailing slash places a file item inside the directory*;
      *Without a trailing slash a file item lands at the destination itself*; *A trailing
      slash is a no-op for a directory item*; *A destination with no `{name}` is used as
      written*; *An item contributing no files computes no destinations*; and the table half
      of *Computing destinations touches nothing*. Build catalogs as literal
      `catalog.Catalog` values and listings as literal `plan.Listing` values — no
      `t.TempDir()`, no fixture directory
- [x] 2.2 GREEN: Add the `Listing` type (`Dir bool`, `Files []string` relative to `from`) and
      an unexported `destinations` function mapping one item plus one interpolated `to` to
      its repo-relative paths, per design.md → D2, D3, D4
- [x] 2.3 GREEN: Interpolate `{name}` with `strings.ReplaceAll` and branch on the trailing
      `/` for file items only; join with `path.Join`, never `filepath.Join`, so the
      separator can never become platform-dependent
- [x] 2.4 CHECK: Contract gate — re-read SPEC.md's `catalog.yaml` section and confirm the
      documented `to`, `{name}`, and trailing-slash semantics still match what this group
      implements, including both worked examples and the resulting `graft.lock` paths
- [x] 2.5 REFACTOR: Collapse the file/directory branch to one code path where it reads
      better, or state that no refactor was needed
- [x] 2.6 Run `go test ./internal/plan/` — no regressions

## 3. The repo-root boundary
<!-- kind: behavior -->

SPEC.md's invariant: no destination escapes the repo root. Checked on the interpolated
destination itself as well as on every path computed under it, so an escaping `to` is
refused even for an item contributing no files.

- [x] 3.1 RED: Write failing tests for: *A `to` climbing out of the repo is refused*; *An
      absolute `to` is refused*; *A listing entry climbing out of its item is refused*; *A
      `to` escaping with no files to place is still refused*; *A destination at the repo root
      itself is accepted*. Assert the exact message
      `source "s": item "agent:x": destination "..." escapes the repo root` — error strings
      are an asserted contract, and changing one later is a deliberate contract change
- [x] 3.2 GREEN: Add the unexported `insideRepo` predicate — reject empty, absolute, `.`, any
      `..` segment, and any path not in `path.Clean` form — written in this package with its
      own wording, per design.md → D10
- [x] 3.3 GREEN: Apply the check to the interpolated destination and to every computed file
      path, returning the first offending path in destination order so the message is
      deterministic
- [x] 3.4 REFACTOR: Ensure the escape error is constructed in exactly one place, as
      `catalog.errf` and `manifest`'s `fail` closure already do, or state that no refactor
      was needed
- [x] 3.5 Run `go test ./internal/plan/` — no regressions

## 4. `flatten` and a list-valued `to`
<!-- kind: behavior -->

- [ ] 4.1 RED: Write failing tests for: *Nested files are flattened into the destination
      root*; *Without flatten the same item preserves its structure*; *Two files flattening
      onto one path is an error*; *One item lands in two destinations*; *Two entries
      interpolating to one destination is an error*; *One item producing the same path twice
      is not this error*. Assert the exact within-item messages
      `... flatten maps "a" and "b" to the same destination "d"` and
      `... destinations "a" and "b" both interpolate to "d"`, and assert that neither is the
      cross-item collision message
- [ ] 4.2 GREEN: Apply `path.Base` to each listed path when the kind declares `flatten`,
      detecting a within-item flatten collision and reporting the two `from`-relative paths
      in ascending order
- [ ] 4.3 GREEN: Iterate a kind's `to` list in declared order, refusing two entries that
      interpolate to the same destination for one item before any file is mapped
- [ ] 4.4 REFACTOR: Keep the two within-item collision messages distinct from the cross-item
      one (design.md → D9), or state that no refactor was needed
- [ ] 4.5 Run `go test ./internal/plan/` — no regressions

## 5. Consumer overrides
<!-- kind: behavior -->

SPEC.md: the destination is what a consumer actually agrees to, and a consumer override
beats the catalog.

- [ ] 5.1 RED: Write failing tests for: *An override moves a kind's items*; *An override
      replaces a list-valued destination entirely*; *An override keeps the catalog's
      flatten*; *An override applies to its own source only*; *An override for an undeclared
      kind is an error*; *An escaping consumer override is refused*. Assert the exact message
      `source "shared": kind override "agnet" names a kind the catalog does not declare`
- [ ] 5.2 GREEN: Resolve each item's destination list as `manifest.Source.Kinds[kind]` when
      present, falling back to `catalog.Kind.To`, carrying `catalog.Kind.Flatten` unchanged
      either way
- [ ] 5.3 GREEN: Refuse an override naming a kind the catalog does not declare, reporting the
      lowest-sorting such kind so the message never depends on map iteration order
- [ ] 5.4 CHECK: Contract gate — re-read SPEC.md's `graft.toml` section and confirm the
      `[sources.<name>.kinds]` override still means what this group implements: one
      destination string per kind, per source, beating the catalog, with no `flatten` of its
      own
- [ ] 5.5 REFACTOR: Ensure the override lookup happens once per item rather than once per
      destination, or state that no refactor was needed
- [ ] 5.6 Run `go test ./internal/plan/` — no regressions

## 6. `Build`: expansion and writes
<!-- kind: behavior -->

- [ ] 6.1 RED: Write failing tests for: *A first plan against no lock*; *A plan for a
      manifest with no sources*; *A write carries the source path and the destination*; *A
      file item's write names the file itself, not a path below it*; *A selector matching
      nothing fails the plan*; *A catalog providing zero items fails the plan*; *A failing
      plan is returned as no plan at all*. The last is asserted in every error case in this
      change: on error the returned `*Plan` is nil
- [ ] 6.2 GREEN: Add `Input`, `Write`, `Plan`, and `Build(inputs []Input, lk *lock.Lock)
      (*Plan, error)` with the shapes design.md → Contracts specifies
- [ ] 6.3 GREEN: Expand each source's `install` against its catalog with `catalog.Expand`,
      returning its error unchanged, and walk sources by name and items by id so every later
      error and every ordering is independent of map iteration
- [ ] 6.4 GREEN: Emit one `Write` per planned file carrying source name, item id, the source
      path, and the destination; sort writes by destination. The source path is
      `path.Join(item.From, rel)` for a **directory** item and `item.From` itself for a
      **file** item — joining in the file case yields `extras/agents/x.md/x.md`, which the
      dedicated scenario exists to catch (design.md → Contracts)
- [ ] 6.5 REFACTOR: Extract the per-source and per-item error prefix into one closure, or
      state that no refactor was needed
- [ ] 6.6 Run `go test ./internal/plan/` — no regressions

## 7. The prune set
<!-- kind: behavior -->

**Concentration point.** graft may never delete a file absent from `graft.lock`. The prune
set is derived from the lock alone and never from a directory listing — `plan` has no
directory to list, which is exactly why this is the right place for the rule.

- [ ] 7.1 RED: Write failing tests for: *A foreign file in a shared destination is never
      pruned*; *An item dropped from install has its files pruned*; *An item the source
      stopped providing has its files pruned*; *A source removed from the manifest has all
      its files pruned*; *A moved destination prunes the old path and writes the new one*; *A
      version bump that adds and removes items*; *A path moving from one source to another is
      written, not pruned*; *A file already present in the tree is still written*
- [ ] 7.2 RED: The foreign-file test asserts the repo-owned `.claude/agents/local-reviewer.md`
      appears in **no** field of the plan — not in `Writes`, not in `Prune`, not in the next
      lock — and asserts it twice: once with the synced item kept and once with it dropped.
      A test that only checks `Prune` would stay green if the path leaked into a write
- [ ] 7.3 GREEN: Derive `Prune` as the lock's file paths minus the newly produced file set,
      sorted by path
- [ ] 7.4 CHECK: Persistence gate — confirm what this change requires of stored data:
      migration, backfill, cache invalidation, and index rebuild all **none**, and
      `graft.lock` stays at `version = 1` (design.md → Persistence and Rollout)
- [ ] 7.5 REFACTOR: Build the new file set once and share it between the prune diff and the
      next lock rather than recomputing it, or state that no refactor was needed
- [ ] 7.6 Run `go test ./internal/plan/` — no regressions

## 8. The next lock
<!-- kind: behavior -->

- [ ] 8.1 RED: Write failing tests for: *An item contributing no files still appears in the
      lock*; *An item placed in two destinations records both files*; *The next lock carries
      rev and resolved separately*; *The next lock round-trips through the lock parser*.
      Assert against `lock.Marshal` output, not against struct fields, so the assertion is
      about what a consumer will actually read in a diff
- [ ] 8.2 GREEN: Build `*lock.Lock` from the plan: sources sorted by name — sorted here, not
      assumed sorted because `manifest.Parse` happens to sort, since `Build` takes a slice a
      caller assembled — `git` and `rev` verbatim from `manifest.Source`, `resolved` from
      `Input.Resolved`, one `lock.Item` per installed item with its destinations sorted by
      path, and no entry for a source the manifest no longer declares
- [ ] 8.3 GREEN: Round-trip the result: `lock.Parse(lock.Marshal(p.Lock), "graft.lock")`
      succeeds and yields the same sources, items, and files. This is what checks, without a
      second validator in this package, that everything `graft.lock` enforces on load holds
      of what `plan` built — the sha shape, unique source names and item ids, and no path
      claimed twice (design.md → Contracts, Preconditions on `Input`)
- [ ] 8.4 CHECK: Contract gate — re-read SPEC.md's `graft.lock` section and confirm the
      constructed lock still matches the documented format: `version = 1`, `rev` records the
      request and `resolved` the sha, `files` is the per-item list that authorises deletion,
      and no content hashes
- [ ] 8.5 REFACTOR: Confirm nothing here duplicates `lock.Marshal`'s normalization — `plan`
      supplies ordered values, `lock` owns the bytes — or state that no refactor was needed
- [ ] 8.6 Run `go test ./internal/plan/` — no regressions

## 9. The collision invariant
<!-- kind: behavior -->

SPEC.md: no two items share a destination path, within a source or across sources.
Collisions are an error, not last-writer-wins — the loser would be a file the lock claims
and a later sync would delete.

- [ ] 9.1 RED: Write failing tests for: *Two items of one source colliding is an error*; *Two
      sources colliding is an error*; *A path claimed by the lock and by another item is still
      a collision*. Assert the exact message
      `source "a" item "agent:x" and source "b" item "agent:y" both resolve to ".claude/agents/x.md"`
      and assert the returned plan is nil
- [ ] 9.2 GREEN: Fill a destination-to-owner map during the single deterministic walk
      (sources by name, items by id, destinations in declared order, files by path) and fail
      on the first second claimant, naming both owners in walk order (design.md → D8)
- [ ] 9.3 REFACTOR: Confirm the collision check runs before any write is appended to the
      plan, so a failed build has produced nothing, or state that no refactor was needed
- [ ] 9.4 Run `go test ./internal/plan/` — no regressions

## 10. Determinism
<!-- kind: behavior -->

**Concentration point.** Lock serialization must be deterministic — sources by name, items
by id, files by path — or every sync churns the diff. Byte equality is what is asserted, not
semantic equality.

- [ ] 10.1 RED: Write failing tests for: *Writes are ordered by destination across sources
      and items*; *Sources, items, and files are ordered independently of input order*; *An
      idempotent re-plan prunes nothing*
- [ ] 10.2 RED: The determinism test builds a plan twice from inputs supplied in reversed
      order — sources, `install` selectors, `Listing.Files`, and the map insertion order of
      `Input.Items` all shuffled — and asserts
      `bytes.Equal(lock.Marshal(a.Lock), lock.Marshal(b.Lock))`. `reflect.DeepEqual` on the
      structs is not sufficient and must not be substituted
- [ ] 10.3 GREEN: Sort every emitted slice at the point it is built; do not rely on
      `lock.Marshal`'s normalization to hide an unsorted plan
- [ ] 10.4 VERIFY: Feed a built plan's lock back in as the current lock, rebuild, and confirm
      an empty prune set and byte-identical lock output — the idempotent re-sync property, at
      the plan tier
- [ ] 10.5 Run `go test ./internal/plan/` — no regressions

## 11. Documentation
<!-- kind: operational -->

- [ ] 11.1 CHECK: Re-read SPEC.md's `catalog.yaml` bullet list and confirm the `to` bullet
      still says only "A trailing `/` means 'into this directory'" — a sentence that does not
      say whether a directory item's own leaf name is appended (design.md → Q1)
- [ ] 11.2 CHANGE: **Rewrite that bullet in place** in SPEC.md (audience: anyone writing a
      `catalog.yaml`; section: `catalog.yaml` — the source's offer) to state the rule the
      implementation now fixes: a trailing `/` places a *file* item inside the directory under
      its own base name, and is a no-op for a *directory* item, whose `to` names the
      destination directory either way. Net addition to SPEC.md: ~2 lines, replacing an
      ambiguous half-sentence rather than appending beside it. Durable reason: the ambiguity
      is what forced design.md → D4, and the next reader of SPEC.md would have to re-derive it
- [ ] 11.3 VERIFY: Confirm SPEC.md's two worked examples and its `graft.lock` example still
      read correctly under the rewritten bullet, and that no other section of SPEC.md, PRD.md,
      ENGINEERING.md, or AGENTS.md now contradicts it. Add nothing to AGENTS.md: its
      `internal/plan` purity rule and the coverage rule already cover this change, and no new
      durable pitfall was found

## 12. Change Review
<!-- kind: operational -->

- [ ] 12.1 CHECK: Dispatch an independent reviewer — a fresh subagent given only
      proposal.md, both spec files, design.md, tasks.md, and the diff, never a fork of the
      implementing session — with these concentration points named: (a) no file absent from
      `graft.lock` can enter the prune set, and the foreign-file test would go red if it
      could; (b) `internal/plan` performs no filesystem access and no test in it creates a
      directory; (c) lock determinism is asserted as byte equality across two builds; (d)
      every asserted error string matches the specs and design.md → Contracts exactly; (e)
      nothing was added to `cmd/graft`, where the coverage gate cannot see it; (f) no code
      path lets a source repository cause anything to execute or place a file outside the
      repo root
- [ ] 12.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a
      one-line reason, note each SUGGESTION, and re-run the affected tests
- [ ] 12.3 VERIFY: Confirm no blocking or unowned finding remains, and that any contract
      changed while fixing findings was written back into the owning artifact and
      planning-review.md

## 13. Lint & Verify
<!-- kind: operational -->

- [ ] 13.1 CHECK: Inspect the intended verification commands and affected tiers — one tier
      only here (unit, `./internal/plan/`), no integration tier, no fixture git repositories
- [ ] 13.2 VERIFY: Run `task lint` — golangci-lint clean and `gofumpt -l` silent, 0 errors
- [ ] 13.3 VERIFY: Run `task test` — `go test -race ./...` green
- [ ] 13.4 VERIFY: Run `task cover` — at or above the 80% floor measured over
      `./internal/...`
- [ ] 13.5 VERIFY: Run `task build` — the binary builds, which is Go's type check
- [ ] 13.6 VERIFY: Run `openspec validate destination-and-plan --strict` — clean
