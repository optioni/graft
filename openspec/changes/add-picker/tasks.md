## 1. The picker model
<!-- kind: behavior -->

- [x] 1.1 RED: Write failing tests for: *Pressing keys moves a cursor and selects*, *The
  cursor stops at both ends*, *`a` selects all and then clears all*, *An unbound key changes
  nothing*, *`enter` confirms and stops*
- [x] 1.2 GREEN: Implement `picker.Item`, `picker.Key`, and `picker.Model` with `Update`,
  `Done`, `Cancelled`, and `Selectors` — pure over values, no filesystem and no clock
- [x] 1.3 RED then GREEN: *Cancelling with a selection made discards it* and *Confirming
  nothing is a cancellation*
- [x] 1.4 REFACTOR: Keep the key table one readable mapping, or record that none is needed
- [x] 1.5 Run `go test ./internal/picker/` — no regressions

## 2. Rendering
<!-- kind: behavior -->

- [x] 2.1 RED: Write failing tests for: *Rendering names every item's destination*, *An item
  with several destinations names all of them*
- [x] 2.2 GREEN: Implement `View`, padding ids to a common width with `ui.Pad` so the picker
  and `--list` align text the same way, marking the cursor and the selected items
- [x] 2.3 GREEN: Window the list to the cursor so a catalog longer than the terminal still
  renders, falling back to a fixed height when none is known
- [x] 2.4 Run `go test ./internal/picker/` — no regressions

## 3. The collapse offer
<!-- kind: behavior -->

- [x] 3.1 RED: Write failing tests for: *Selecting both agents offers the glob*, *A kind with
  one item is never offered*, *Two wholly selected kinds are offered separately*, *Cancelling
  at the offer cancels the whole picker*
- [x] 3.2 GREEN: Implement the second screen — one offer per wholly selected kind of more
  than one item, each accepted or declined, the text stating what `kind:*` means
- [x] 3.3 GREEN: Order the resulting selectors by kind as the catalog orders them and by id
  within a kind, so two runs of one selection produce one manifest
- [x] 3.4 Run `go test ./internal/picker/` — no regressions

## 4. The terminal driver
<!-- kind: behavior -->

- [x] 4.1 RED: Write failing tests for: *A scripted key stream chooses a selector*, *An arrow
  key is one event, not an escape and two letters*, *Raw mode is restored on every path*, *A
  closed input cancels*
- [x] 4.2 GREEN: Implement `picker.Terminal` and `picker.Run` — read, decode, update, render,
  restore; raw mode entered through the caller's function and restored on every return
- [x] 4.3 CHECK: Confirm no test in the package needs a pseudo-terminal, and that
  `term.MakeRaw` is the only line the suite does not reach
- [x] 4.4 Run `go test ./internal/picker/` — no regressions

## 5. The chooser in the add sequence
<!-- kind: behavior -->

- [ ] 5.1 RED: Write failing tests in `internal/add` for: *No selectors on a terminal opens
  the picker*, *A cancelled picker writes nothing*, *An ungraftable source fails before any
  list is shown* — each with a scripted chooser against a fixture repository
- [ ] 5.2 GREEN: Add `Request.Choose`, and restructure `Run` so the rev is resolved and the
  catalog read once, before the chooser is called and before the amendment
- [ ] 5.3 GREEN: Build the picker's items from the catalog and `plan.ItemDestinations`, so
  the picker and `--list` show the same destinations by construction
- [ ] 5.4 RED then GREEN: The manifest a chosen selector produces is byte-identical to the
  one the same selector typed on the command line produces
- [ ] 5.5 CHECK: Confirm nothing is written before the chooser returns — a cancelled run
  leaves the repository byte-identical, `graft.toml` included
- [ ] 5.6 Run `go test ./internal/add/` — no regressions

## 6. The command surface
<!-- kind: behavior -->

- [ ] 6.1 RED: Write failing tests for: *No selectors, no TTY*, *A malformed selector is
  refused before the network*, and a cancelled run exiting `1` with `add cancelled`
- [ ] 6.2 GREEN: Add `Options.Stdin`, `Options.Interactive`, and `Options.MakeRaw`, each
  defaulting to the real process when nil
- [ ] 6.3 GREEN: Wire `add` — the no-selector refusal only when not interactive, the chooser
  built from the picker otherwise, and `add cancelled` when it comes back empty
- [ ] 6.4 CHECK: Confirm standard output is still byte-empty for a syncing `add`, including
  one that went through the picker
- [ ] 6.5 Run `go test ./internal/cli/` — no regressions

## 7. Documentation
<!-- kind: operational -->

- [ ] 7.1 CHECK: Re-read SPEC.md's `## graft add` section and its failure-mode row for an
  add with no selectors, and confirm both now describe behavior that exists
- [ ] 7.2 CHANGE: Rewrite that row in place — it currently says "until the picker exists
  this is the answer on a terminal too", which this change makes false — and add the
  cancellation row
- [ ] 7.3 VERIFY: Confirm no other document repeats the narrowed rule

## 8. Change Review
<!-- kind: operational -->

- [ ] 8.1 CHECK: Review the implementation against proposal.md, every spec scenario,
  design.md, and tasks.md, with the concentration points the reviewer instructions name
- [ ] 8.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a
  one-line reason, note SUGGESTIONs, and re-run affected tests
- [ ] 8.3 VERIFY: Confirm no blocking or unowned finding remains

## 9. Lint & Verify
<!-- kind: operational -->

- [ ] 9.1 CHECK: Inspect the intended verification commands and the tiers they cover
- [ ] 9.2 VERIFY: Run `task lint` — 0 errors
- [ ] 9.3 VERIFY: Run `task test` — green
- [ ] 9.4 VERIFY: Run `task cover` — the 80% floor over `./internal/...` holds
- [ ] 9.5 VERIFY: Run `task build` — the binary builds
- [ ] 9.6 VERIFY: Run `openspec validate add-picker --strict`
