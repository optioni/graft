## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

- [x] 0.1 Extend the `add` acceptance harness with a consumer that already holds a file at a
  destination the source will write, committed, with content of its own
- [x] 0.2 RED: Write the failing end-to-end test for *An item that replaced a hand-written
  file is adopted* — `graft add` reports `adopted … replaced existing content` and the
  summary counts it
- [x] 0.3 Confirm it fails because the report says `added`, not because the fixture is wrong

## 1. The plan marks what the lock already claimed
<!-- kind: behavior -->
<!-- parallel-after: 0 -->

- [x] 1.1 RED: Write failing tests for: *A path the lock claims is marked claimed*, *A path
  claimed under a different source or item still counts as claimed*, *A path no lock claims is
  not marked*, *An empty lock claims nothing*
- [x] 1.2 GREEN: Add `Write.Claimed`, set from the lock `plan.Build` is already given
- [x] 1.3 VERIFY: `internal/plan`'s purity test still passes and no new test needs a directory
- [x] 1.4 Run `go test ./internal/plan/` — no regressions

## 2. The applier observes what it replaced
<!-- kind: behavior -->
<!-- parallel-after: 0 -->

- [x] 2.1 RED: Write failing tests for: *A hand-written file at a destination is reported*, *A
  claimed destination is not reported*, *Identical bytes are not a replacement*, *An absent
  destination is not a replacement*, *A failed apply reports nothing*
- [x] 2.2 GREEN: Compare against the bytes already held before writing, for unclaimed
  destinations only, and return the paths from `apply.Run`
- [x] 2.3 CHECK: Confirm nothing about what is written changed — the same writes, the same
  order, the same containment, and the prune set untouched
- [x] 2.4 Run `go test ./internal/apply/` — no regressions

## 3. The report says so
<!-- kind: behavior -->

- [x] 3.1 RED: Write failing tests for: *An item that replaced a hand-written file is
  adopted*, *An updated item that replaced a hand-written file keeps its verb*, *A destination
  the lock already claimed is not a replacement*, *Identical bytes replace nothing*, *A dry run
  reports no adoption*
- [x] 3.2 RED: Write failing tests for: *The summary names how many files replaced something*
  and *A sync that replaced nothing carries no parenthetical*
- [x] 3.3 GREEN: Fold the applier's paths into the report — the verb where it would be false,
  the note, and the count — and render the summary's parenthetical
- [x] 3.4 VERIFY: Assert a report with no replacement is byte-identical to what it was before
  this change, including its column alignment
- [x] 3.5 Run `go test ./internal/sync/` — no regressions

## 4. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [x] 4.1 VERIFY: Confirm the group 0 acceptance test passes end to end
- [x] 4.2 RED then GREEN: A second `graft sync` over the same repository reports nothing
  adopted — the file is graft's now, and adoption is a one-time event

## 5. Documentation
<!-- kind: operational -->

- [ ] 5.1 CHECK: Re-read SPEC.md's Output section, which lists the report's verbs, and its
  `catalog.yaml` section, which claims `add` shows every destination before writing
- [ ] 5.2 CHANGE: Add `adopted` to the verb list with its note and the summary's
  parenthetical, and correct the destination sentence to describe `--list` and the picker —
  which do show destinations first — rather than claiming the non-interactive form does
- [ ] 5.3 VERIFY: Confirm no other document repeats either claim

## 6. Change Review
<!-- kind: operational -->

- [ ] 6.1 CHECK: Review the implementation against proposal.md, every spec scenario,
  design.md, and tasks.md
- [ ] 6.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING, note
  SUGGESTIONs, re-run affected tests
- [ ] 6.3 VERIFY: Confirm no blocking or unowned finding remains

## 7. Lint & Verify
<!-- kind: operational -->

- [ ] 7.1 CHECK: Inspect the intended verification commands and the tiers they cover
- [ ] 7.2 VERIFY: Run `task lint` — 0 errors
- [ ] 7.3 VERIFY: Run `task test` — green
- [ ] 7.4 VERIFY: Run `task cover` — the 80% floor over `./internal/...` holds
- [ ] 7.5 VERIFY: Run `task build` — the binary builds
- [ ] 7.6 VERIFY: Run `openspec validate adoption-report --strict`
