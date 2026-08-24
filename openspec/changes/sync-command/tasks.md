## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

This change alters end-to-end command behavior for the first time — `graft` had one flag and
no command — and the risk it carries is wiring: a plan applied against the wrong root, a
report on the wrong stream, a cache root read from a global. design.md → Test Strategy records
why the group is kept.

- [x] 0.1 Extend `internal/cli/acceptance_test.go`'s existing harness — which builds the real
      binary and runs it as a subprocess with `cmd.Dir` — with an environment-carrying variant,
      and add a fixture source repository built in `t.TempDir()` with
      `git -c init.defaultBranch=main init -q` and `user.name` / `user.email` set **on the
      repository**. The subprocess's `XDG_CACHE_HOME` points at an **absolute** `t.TempDir()`,
      or `defaultCacheRoot` falls through to the developer's real `~/.cache`. No `t.Chdir` and
      no `t.Setenv`: the working directory and the environment belong to the child
- [x] 0.2 RED: Write the failing end-to-end test for *A first sync installs what the manifest
      asks for* — the binary run with `sync` exits `0`, every item's file exists at
      its destination with the source's bytes, `graft.lock` records the source and both
      items, the report is on the error stream, and the standard output stream is byte-empty
- [x] 0.3 Confirm it fails because `sync` is not a command — `graft: unknown command "sync"` —
      and not because the fixture repository or the temp cache is misconfigured

## 1. Filename constants and the apply package scaffold
<!-- kind: operational -->

- [x] 1.1 CHECK: Confirm `graft.toml` and `graft.lock` are currently spelled only inside
      `internal/manifest` and `internal/lock` doc comments and tests, so exporting a constant
      introduces no second spelling
- [x] 1.2 CHANGE: Add `manifest.Filename = "graft.toml"` and `lock.Filename = "graft.lock"`,
      each with a doc comment saying it is the one spelling of that file's name
- [x] 1.3 CHANGE: Create `internal/apply/apply.go` holding only the package doc comment —
      `apply` is the only package permitted to create, modify, or delete anything in the
      repository graft runs in; it derives nothing, and every path it touches comes from the
      plan it was given
- [x] 1.4 VERIFY: `go build ./...` succeeds and `go test ./internal/manifest/ ./internal/lock/`
      is green

## 2. apply: writing planned files
<!-- kind: behavior -->

- [x] 2.1 RED: Write failing tests for: *A file is copied into a directory that does not exist
      yet*, *A hand-edited synced file is overwritten*, *An executable source file lands as a
      non-executable one*, *An executable destination is made non-executable*, *A source file
      that cannot be read fails the apply*, *A write naming an unregistered source fails
      before it is attempted*, *An empty plan writes only the lock*, *Nothing outside the plan
      is touched*, *A directory at a destination is refused*, *A symlink at a destination is
      refused rather than followed* — the last two belong here rather than in group 3 because
      the safe write in 2.2 removes an existing destination, and removing one it has not
      confirmed regular is the mistake this whole package exists to avoid
- [x] 2.2 GREEN: Implement `apply.Run(root string, trees map[string]string, p *plan.Plan)
      error` — open an `os.Root` at `root` and one at each source's tree, and for each write in
      plan order create the destination's parents at `0755`, remove an existing destination,
      and create the file with `O_EXCL` at `0644`
- [x] 2.3 GREEN: Remove-then-create rather than truncate-in-place, because the permission
      argument to a create-and-truncate open applies only when the file is created — a
      destination someone once ran `chmod +x` on would otherwise stay executable while graft
      replaced its contents. The *An executable destination is made non-executable* test is
      the one that fails without it
- [x] 2.4 GREEN: Add the two per-source error strings the specification pins —
      `source "<name>": no fetched tree` and `source "<name>": cannot read "<path>": <reason>`
      — through one error helper, following `internal/source`'s `sourceErrf`
- [x] 2.5 REFACTOR: Keep the write loop free of any decision the plan did not make — no
      content comparison, no stat-based skip, no ordering of its own
- [x] 2.6 Run `go test ./internal/apply/` — green, no regressions elsewhere

## 3. apply: an ancestor that is not a directory
<!-- kind: behavior -->

**Concentration point.** `os.Root` refuses a path that leaves the root but **follows** a
symlink that stays inside it. The root alone is therefore not the floor it looks like, and
this group is where that gap is closed on the write side. Group 2 already owns the
destination's own type, because a safe write cannot be written without deciding it.

- [x] 3.1 RED: Write failing tests for: *A destination under a symlinked parent is refused* and
      *A destination whose parent is a regular file is named*
- [x] 3.2 GREEN: Walk the destination's ancestors from the top; an ancestor that exists and is
      not a directory fails with `cannot write "<path>": "<ancestor>" is not a directory`,
      naming the shallowest one. A symlink to a directory is not a directory here
- [x] 3.3 REFACTOR: Extract the two predicates — "exists and is a regular file", "exists and is
      a directory" — so group 5's prune-side checks share them and "graft only ever writes
      regular files" is stated once
- [x] 3.4 Run `go test ./internal/apply/` — green

## 4. apply: reserved paths
<!-- kind: behavior -->

- [x] 4.1 RED: Write failing tests for: *A destination inside .git is refused*, *A prune path
      inside .git is refused*, *A destination of graft.toml or graft.lock is refused*, *A path
      merely beginning with .git is not inside it* — the last asserting `.github/workflows/ci.yml`
      and `.gitignore` are both written
- [x] 4.2 GREEN: Refuse a destination or prune path whose **first path segment** is `.git`, or
      which equals `graft.toml` or `graft.lock`, with the four messages design.md → Contracts
      lists. The rule is on the segment, never a string prefix
- [x] 4.3 REFACTOR: State in a comment why this floor is here rather than in `internal/plan`:
      `.git/config` does not escape the repo root, so no planning rule catches it, and a file
      placed there is invisible to the `git diff` that is SPEC.md's whole review story
- [x] 4.4 Run `go test ./internal/apply/` — green

## 5. apply: executing the prune set
<!-- kind: behavior -->

**Concentration point.** graft may never delete a file absent from `graft.lock`. This group
owns the prune set, so it carries the tests proving a foreign file survives — beside a synced
file, in the ancestry of a pruned path, and in a directory holding files no lock records.

- [x] 5.1 RED: Write failing tests for: *A foreign file in a shared destination survives every
      operation* (all three variants in the scenario), *Unrecorded files in a destination
      directory are never enumerated*, *A prune path that is already gone is not an error*, *A
      prune path that is a directory is refused*, *A prune path that is a symlink is refused*
- [x] 5.2 RED: Write failing tests for the ancestry rule: *A prune path under a symlinked
      parent is refused* — the lock claims `vendor/x.md`, `vendor` has become a link to
      `docs/`, and `docs/x.md` must still exist afterwards — and *A prune path whose parent is
      a regular file is refused*
- [x] 5.3 GREEN: Delete each prune path through the repository's `os.Root`, skipping a path
      that does not exist, failing a path that is not a regular file with
      `cannot remove "<path>": it is not a regular file`, and failing an ancestor that is not a
      directory with `cannot remove "<path>": "<ancestor>" is not a directory`
- [x] 5.4 REFACTOR: Confirm `RemoveAll` appears nowhere in `internal/apply`, that the only
      deletion call is `Remove` on a path just confirmed regular, and that no directory listing
      is performed anywhere in the package
- [x] 5.5 Run `go test ./internal/apply/` — green

## 6. apply: removing directories the prune set left empty
<!-- kind: behavior -->

- [x] 6.1 RED: Write failing tests for: *An emptied directory chain is removed*, *A directory
      still holding a foreign file is kept*, *An unrelated empty directory is left alone*, *A
      symlinked ancestor of a pruned path is not removed*, *A pruned path at the repository
      root removes nothing*
- [x] 6.2 GREEN: After the prune set has run, walk each pruned path's ancestors upward, deepest
      first. `Lstat` each candidate and remove it **only if it is a directory** — unlinking a
      symlink succeeds however full its target is, so a bare `Remove` would delete a user's
      `vendor -> docs` link. A removal that fails for any reason is ignored
- [x] 6.3 REFACTOR: Derive the candidate set from the pruned paths only, deduplicated and
      ordered so a child is always tried before its parent, with the repository root never a
      candidate; state in a comment why a walk of the tree would be the same mistake as
      scanning for files to prune
- [x] 6.4 Run `go test ./internal/apply/` — green

## 7. apply: the pre-flight pass
<!-- kind: behavior -->

- [x] 7.1 RED: Write failing tests for: *A refused destination leaves every write unapplied*,
      *A refused prune path leaves every write unapplied*, *A missing source file is refused
      before anything is written* — each asserting that no destination exists, nothing was
      deleted, and `graft.lock` is unchanged
- [x] 7.2 GREEN: Hoist every check from groups 2 through 6 into one pass that runs before the
      first write: source registered, source path a regular file, destination not reserved,
      destination ancestors all directories, destination regular if it exists; and for each
      prune path, not reserved, ancestors all directories, regular if it exists
- [x] 7.3 REFACTOR: Leave the checks in place at their point of use where they cost nothing,
      or remove them where the pass makes them dead — but state in a comment that the pass is
      not a lock on the filesystem: a condition can change between the check and the write, and
      that case is allowed to fail mid-flight
- [x] 7.4 Run `go test ./internal/apply/` — green

## 8. apply: writing graft.lock last
<!-- kind: refactor -->

**Reclassified during implementation.** This was planned as a behavior group, and its tests
passed on first run: group 2's writer wrote the lock after the writes from the start, because
a writer that did not would have had nowhere sensible to put it. A test that goes green
without a red is characterization, not a driver, so the group takes the refactor lifecycle it
actually has. The **contract gate** is unchanged and is the point of the group.

- [x] 8.1 CHARACTERIZE: Write tests pinning the behavior already in place — *The lock that is
      written is the plan's lock* (parsed back through `lock.Parse`, because a lock this
      package writes that the next run refuses to read is the worst failure available here),
      *An unchanged sync still writes the lock* (asserting **byte** equality across two runs,
      not semantic equality), and *A write that fails mid-flight leaves the previous lock in
      place* — the last driven by a read-only destination directory, which is the residual
      failure the pre-flight pass cannot remove
- [x] 8.2 REFACTOR: None. The lock write is three lines calling `lock.Marshal` and the
      package's own `writeFile`, and extracting anything from it would add a name without
      removing a decision
- [x] 8.3 CHECK: Contract gate — re-read SPEC.md's `graft.lock` section and diff a written
      lock against the documented example's layout: header line, `version = 1`, sources by
      name, items by id, files by path, the documented key alignment
- [x] 8.4 VERIFY: Run `go test ./internal/apply/` — green

## 9. apply: containment
<!-- kind: refactor -->

**Reclassified during implementation**, like group 8 and for the same reason: the `os.Root`
containment was put in place by group 2 as the mechanism the package is built on, so these
tests characterize it rather than driving it. What they add is the evidence — plans
`plan.Build` would refuse to produce, which is the only way to reach the floor at all.

- [x] 9.1 CHARACTERIZE: Write tests for *A destination escaping the root fails rather than
      being written*, *A source path escaping the fetched tree fails rather than being read*,
      and *A missing repository root is named*, the first two driven by hand-built plans
- [x] 9.2 REFACTOR: None. The containment is two `os.OpenRoot` calls; there is nothing to
      extract that would not just be a second name for one of them
- [x] 9.3 VERIFY: Confirm no path in the package is built with `filepath.Join(root, ...)` and
      then passed to an `os` function — the containment is the `os.Root`, not a string check —
      and that the ancestor rules from groups 3 and 5 are documented as the half the `os.Root`
      does not provide
- [x] 9.4 VERIFY: Run `go test -race ./internal/apply/` — green

## 10. sync: the resolution sequence and the pin rule
<!-- kind: behavior -->

**Concentration point.** Fixture git repositories need `user.name` and `user.email` set on
**the repository**, not the machine, or every test in this group and the next four fails on a
clean CI runner.

- [x] 10.1 RED: Add `internal/sync/fixture_test.go` — a fixture source repository helper and a
      consumer-directory helper — then write failing tests for: *A first sync installs what the
      manifest asks for*, *A second sync changes nothing*, *A missing manifest is refused
      before anything else happens*, *A source with no lock entry is resolved once and
      recorded*
- [x] 10.2 RED: Write failing tests for the pin rule: *A moved branch does not move the pin*
      (the branch advances with a new file after the lock is written; the sync installs the
      recorded sha and `resolved` does not change), *A bumped rev in the manifest points at
      graft update*, *The pin check precedes the network*, *A source dropped from the manifest
      is pruned without being fetched*
- [x] 10.3 GREEN: Implement `sync.Options{Root, CacheRoot, DryRun}` and `sync.Run` performing
      SPEC.md's steps in order: load manifest, load lock, `lock.CheckPins`, then per source
      take the sha from the lock or `source.Resolve` it, `Cache.Fetch`, `source.ReadCatalog`,
      `catalog.Expand`, `source.List` per item, `plan.Build`, `apply.Run`
- [x] 10.4 REFACTOR: Confirm by reading the function that the fetch loop cannot be entered
      before `lock.CheckPins` returns, and that a source absent from the manifest reaches
      neither `Resolve` nor `Fetch`; the two tests in 10.2 are what prove it
- [x] 10.5 Run `go test ./internal/sync/` — green

## 11. sync: failure modes leave the tree untouched
<!-- kind: refactor -->

**Reclassified during implementation**, the third group to be. Group 10 built the sequence
with every collaborator's error returned unaltered, so these tests characterize SPEC.md's
failure-mode table reaching a user end to end rather than driving new behavior. That is
still the most valuable thing to pin here: the table *is* the product for a CLI this size,
and until this change none of its rows had ever been reached through a whole run.

- [x] 11.1 CHARACTERIZE: Write tests for *A rev that no longer exists fails, naming the rev
      and the source*, *A source without a catalog is not graftable*, *An invalid catalog
      fails the run* (both variants), *A selector matching nothing fails the run*, *Two items
      resolving to one path fail the run before any of it is written*
- [x] 11.2 CHARACTERIZE: Write tests for the two rows nothing else reaches: *A destination
      outside the repository root fails the run* — through a real `plan.Build`, not a
      hand-built plan — and *A source's listing climbing out of its item fails the run*
- [x] 11.3 CHARACTERIZE: Write tests for *A cache miss with no network names what it needed*
      and *A cache hit with no network succeeds*
- [x] 11.4 CHARACTERIZE: Every failure test asserts the same three things through one shared
      helper: the exact error string, that every pre-existing file is byte-identical, and
      that `graft.lock` was neither created nor modified
- [x] 11.5 REFACTOR: None. "Return the error unaltered" is the absence of code, and the
      characterization is what keeps it absent
- [x] 11.6 VERIFY: Run `go test ./internal/sync/` — green

## 12. sync: `--dry-run`
<!-- kind: behavior -->

- [x] 12.1 RED: Write failing tests for: *A dry run of a first sync creates nothing*, *A dry
      run of a removal deletes nothing*, *A dry run of a failing plan fails the same way* —
      the first asserting no destination **directory** exists, which is the half of SPEC.md's
      promise a file-existence check would miss
- [x] 12.2 GREEN: Return after `plan.Build` when `DryRun` is set, without calling `apply.Run`
- [x] 12.3 REFACTOR: State in a comment that the fetch still happens under `--dry-run` — there
      is no plan without a catalog — and that a clean dry run says the plan is valid, not that
      the sync will succeed, because none of `apply`'s refusals are reached
- [x] 12.4 Run `go test ./internal/sync/` — green

## 13. sync: the report value
<!-- kind: behavior -->

- [x] 13.1 RED: Write failing unit tests, over hand-built old and new locks with no filesystem
      at all, for: *A newly installed item is added*, *An unchanged item under a moved pin is
      still updated*, *An unchanged item under an unchanged pin produces no line*, *An item
      dropped from install says so*, *An item of a source dropped from the manifest says so*,
      *A version bump shows both revs and both shas*, *A newly added source shows one rev and
      one sha*, *A branch pin whose sha moved shows both shas and one rev*
- [x] 13.2 RED: Write failing tests for the nothing-to-do predicate: *A repeated sync reports
      nothing*, *A sync that only rewrites identical files is still nothing to do*, *A first
      sync is never nothing to do*, *A dry run with nothing to do reports nothing*
- [x] 13.3 GREEN: Build the `Report` from the old lock, the plan's lock, the prune set, and
      the per-source catalogs — verb from item presence plus sha and file-list movement, note
      from whether the catalog still provides the item or the source is gone. Sources come from
      the **union** of the two locks, in name order, so a source dropped from `graft.toml` is
      still reported
- [x] 13.4 GREEN: Implement the nothing-to-do predicate as
      `bytes.Equal(lock.Marshal(old), lock.Marshal(new)) && len(p.Prune) == 0`
- [x] 13.5 REFACTOR: Keep the report free of any filesystem access — it is built from values,
      and a test needing a directory to exercise it is a signal the boundary moved
- [x] 13.6 Run `go test ./internal/sync/` — green

## 14. sync: rendering the report
<!-- kind: behavior -->

- [x] 14.1 RED: Write failing tests for: *An updated and a removed item align in one block* —
      pinning the exact bytes of SPEC.md's own example, **including that no line carries
      trailing whitespace** — *Two sources are separated by a blank line*, *The summary counts
      every planned write*, *A sync that only removes still reports zero written*, *A single
      file is reported in the singular*, *A dry run says nothing was written*, *With colour off
      the report is plain text*, *With colour on only the verb and the note are styled*
- [x] 14.2 GREEN: Implement `(*Report).Lines(u *ui.UI) []string` — header, blank, item lines,
      blank, summary. The verb and the id are padded to the widest of their column in that
      source's block and followed by two spaces; the count is padded and followed by two spaces
      **only when a note follows it**, so a line with no note ends at the count
- [x] 14.3 GREEN: Style the verb with `u.Bold` and a removed item's note with `u.Dim`, after
      the padding is computed, so enabling colour never moves a column
- [x] 14.4 REFACTOR: Diff the rendered block against SPEC.md's Output section character by
      character, trailing whitespace included, and keep the fixture in the test rather than in
      a comment
- [x] 14.5 Run `go test ./internal/sync/` — green

## 15. cli: the `sync` subcommand and the `help` refusal
<!-- kind: behavior -->

- [x] 15.1 RED: Write failing tests for: *An argument to sync is a usage error*, *There is no
      flag to make sync re-resolve or refuse to overwrite* (`--force` and `--frozen` both
      unknown flags), *Help lists the commands graft has*, *`help` is not a command*, *`help
      sync` is not a command either*, *A failing sync leaves standard output byte-empty*
- [x] 15.2 GREEN: Register `sync` on the root with `--dry-run`, its own argument validator
      producing `unknown argument "<argument>"` as a usage error, and a `RunE` that builds
      `sync.Options` from the working directory and `source.DefaultCacheRoot()`, then writes
      each report line through `ui.Note`
- [x] 15.3 GREEN: Refuse the literal argument `help` in `Main`, beside the existing
      `__complete` and `__completeNoDesc` guards. Do **not** use `SetHelpCommand` with a
      placeholder: cobra adds whatever it is given to the root as a real command, so the
      placeholder's own name would become a working, undocumented command
- [x] 15.4 CHECK: Contract gate on the existing surface — confirm every current `internal/cli`
      test still passes **unchanged**, which is what carries the two scenarios this change
      restates without altering: *No arguments prints help and succeeds* and *`--help` prints
      the same text as no arguments at all*. Both must still write byte-identical text to
      stdout, now including the commands section
- [x] 15.5 Run `go test ./internal/cli/` — green

## 16. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [x] 16.1 VERIFY: Confirm the group 0 acceptance test now passes end to end
- [x] 16.2 RED: Write the two remaining acceptance scenarios against the same harness — *A
      successful sync leaves standard output byte-empty* and *The report never reaches standard
      output* — and confirm each fails for the reason it names before making it pass
- [x] 16.3 GREEN: Make both pass without changing the harness
- [x] 16.4 REFACTOR: Fold the fixture-repository helpers into one place per package if group 0
      and group 10 duplicated them; do not share a helper across packages through a non-test
      file

## 17. Documentation: SPEC.md and the layout blocks
<!-- kind: operational -->

- [ ] 17.1 CHECK: List the user-facing conditions this change adds against SPEC.md's
      Failure modes table and Invariants, and confirm none is already there
- [ ] 17.2 CHANGE: Add rows to SPEC.md's Failure modes table for: a destination or prune path
      that is not a regular file, an ancestor that is not a directory, a destination inside
      `.git` or naming graft's own two files, a source file that cannot be read, and a
      repository root that cannot be opened. Add the `.git` rule to Invariants, and add the
      `--dry-run` summary line to the Output section
- [ ] 17.3 CHANGE: Add `internal/sync` to the layout blocks in ENGINEERING.md and AGENTS.md,
      which both still list six packages
- [ ] 17.4 VERIFY: Re-read the amended SPEC.md sections against the error strings the tests
      assert, and confirm each row is worded exactly as the message the code produces

## 18. Change Review
<!-- kind: operational -->

- [ ] 18.1 CHECK: Dispatch an **independent reviewer subagent** — not a fork of the
      implementing session — against proposal.md, all four spec files, design.md, tasks.md,
      and the diff. Point it explicitly at the prune set and the foreign-file guarantee: that
      no code path in `internal/apply` can delete a path absent from `graft.lock`, that no
      code path enumerates a destination directory, that no path resolves through a symlink to
      reach a file the lock does not name, and that the empty-directory removal cannot reach a
      directory outside a pruned path's ancestry
- [ ] 18.2 CHECK: Have the reviewer confirm no source-provided content can cause anything to
      execute, that `internal/plan` gained no filesystem access, and that `cmd/graft` gained
      no decision
- [ ] 18.3 CHANGE: Fix every CRITICAL, resolve or accept each WARNING with the reason
      recorded, and re-run the affected tests
- [ ] 18.4 VERIFY: Confirm no blocking or unowned finding remains

## 19. Lint & Verify
<!-- kind: operational -->

- [ ] 19.1 CHECK: Inspect the verification commands `Taskfile.yml` defines and confirm the
      affected tiers are `./internal/apply`, `./internal/sync`, `./internal/cli`,
      `./internal/lock`, and `./internal/manifest`
- [ ] 19.2 VERIFY: Run `task lint` — 0 errors
- [ ] 19.3 VERIFY: Run `task cover` — green and above the 80% floor over `./internal/...`,
      with `internal/apply` and `internal/sync` reported individually
- [ ] 19.4 VERIFY: Run `task build` — succeeds
- [ ] 19.5 VERIFY: Run `openspec validate sync-command --strict` — clean
