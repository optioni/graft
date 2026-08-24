## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

This change adds a client-visible command whose end-to-end wiring is the risk: a `--to` value
that never reaches `SetRev`, a report on the wrong stream, an update run against the wrong root,
a cache root read from a global. design.md → Test Strategy records why the group is kept.

- [x] 0.1 Reuse `internal/cli/sync_acceptance_test.go`'s existing harness — `runGraftIn` with an
      absolute `XDG_CACHE_HOME`, and `newSourceRepo` with `user.name` / `user.email` set **on the
      repository**. No `t.Chdir` and no `t.Setenv`: the working directory and the environment
      belong to the child process
- [x] 0.2 RED: Write the failing end-to-end test for the headline scenario — a consumer synced at
      `v1.0.0`, the source repository tagged `v1.1.0` with a file added to an installed item, then
      `graft update --to v1.1.0 shared` exits `0`, `graft.toml` says `rev = "v1.1.0"` and differs
      from before in exactly one line, `graft.lock` records the new sha, the new file exists at its
      destination, the report is on the error stream, and the standard output stream is byte-empty
- [x] 0.3 Confirm it fails because `update` is not a command — observed as
      `graft: unknown flag: --to`, cobra reaching the flag before the root's argument
      validator reaches `update` — and not because the fixture repository or the temp cache is
      misconfigured

## 1. manifest: reading the bytes, and moving one rev in them
<!-- kind: behavior -->

**Concentration point.** `SetRev` is text editing over a file a human wrote. Its real failure is
not "it did not work" but "it worked and landed on the wrong line", so every test here asserts
the **exact** expected bytes rather than a re-parse alone, and three boundaries get their own
tests because the obvious implementation gets each wrong: the extent of the source's table, the
extent of the value, and comment lines.

- [x] 1.1 RED: Write failing unit tests, over in-memory `[]byte` with no filesystem at all, for
      the preserving half: *The aligned value is replaced and nothing else moves*, *A comment
      trailing the rev line survives* (including a rev value containing `#`), *Other sources are
      untouched*, *Line endings and a missing final newline are preserved* (a CRLF input and an
      input whose last line has no newline)
- [x] 1.2 RED: Write failing unit tests for the locating half: *A rev key in a kinds sub-table is
      not the one edited*, *A commented-out rev above the real one is skipped*, *A quoted table
      key is recognised* (bare, quoted, and `[ sources . "x" ]`), *A source written as an inline
      table is refused*, *A source the file does not declare is refused the same way* (absent, and
      `[[sources.x]]`), *A multi-line rev value is refused rather than half-rewritten*
- [x] 1.3 RED: Write failing unit tests for the character rule and the round trip: *A rev that
      would have to be escaped is refused* (`"`, `\n`, `\`, DEL), *A rev that would inject a second
      key is refused*, *The result round-trips through the parser*
- [x] 1.4 GREEN: Implement `manifest.SetRev(data []byte, name, rev string) ([]byte, error)`.
      Refuse the rev first — a quote, a backslash, or any rune `unicode.IsControl` reports true
      for — before scanning anything. Then scan lines, tracking the current table by its **exact**
      key path: a header is `sources`, `<name>` with whitespace tolerated and a quoted name
      unquoted, the source's table ends at the **next header of any kind** so
      `[sources.<name>.kinds]` is a different table, and `[[sources.<name>]]` does not match a
      standard table
- [x] 1.5 GREEN: Inside that table, skip comment lines, match `rev` with optional leading
      whitespace followed by optional whitespace and `=`, find the value's opening and closing
      quotation marks **on the same line**, and replace only the span between them — so the
      alignment before `=` and any comment after the value both survive, and a value whose closing
      quote is not on that line is refused rather than half-rewritten. Preserve each line's
      original ending and do not add a trailing newline the input did not have
- [x] 1.6 GREEN: Pin the two error strings design.md → Contracts lists, both prefixed
      `graft.toml: ` literally. A table that is not found and a `rev` that is not a plain key
      produce the **same** message — `rev is not a plain key under [sources.<name>]` is true of
      both, and one message is the shape of SPEC.md's failure-mode table
- [x] 1.7 GREEN: Add `manifest.Read(path) (*Manifest, []byte, error)` and make `Load` delegate to
      it, so `graft.toml not found` keeps its single spelling and no existing caller or test moves
- [x] 1.8 REFACTOR: Confirm `SetRev` returns `nil` bytes on every error path, so no caller can
      write a half-edited file, and that nothing in the function touches the filesystem
- [x] 1.9 CHECK: Contract gate — re-read SPEC.md's `graft.toml` section and confirm the shape
      `SetRev` recognises is exactly the shape SPEC.md documents, that the refusal names the shapes
      it does not, and that `manifest.Parse` still accepts every file `SetRev` can produce
- [x] 1.10 Run `go test ./internal/manifest/` — green, and `go test ./internal/sync/` unchanged

## 2. apply: writing manifest bytes, immediately before the lock
<!-- kind: behavior -->

**Concentration point.** `internal/apply` refuses `graft.toml` as a *plan* destination, and must
keep refusing it on the very run that rewrites `graft.toml`. The two are told apart by where the
bytes came from — the plan, or the caller — never by the path string.

- [x] 2.1 RED: Write failing tests, driven by hand-built plans against a real `t.TempDir()`, for:
      *Manifest bytes are written just before the lock*, *An apply with no manifest bytes leaves
      graft.toml alone*, *An empty plan with manifest bytes writes graft's two files and nothing
      else*, *A plan naming graft.toml as a destination is still refused* (with `WithManifest`
      also given), *A graft.toml that is not a regular file fails before the first write*, *A
      failed apply leaves graft.toml unmoved*, *No temporary file survives a successful or a
      failed apply*
- [x] 2.2 GREEN: Add `apply.Option` and `apply.WithManifest(data []byte)`, and make
      `Run(root, trees, p, opts ...Option)` variadic so all 56 existing call sites compile and
      assert unchanged
- [x] 2.3 GREEN: Write the bytes after the prune set and the empty-directory walk and immediately
      before `writeLock`, through a **temporary file at the repository root plus
      `(*os.Root).Rename`** rather than through `writeFile`'s remove-then-create: `graft.toml` is
      the one file graft cannot regenerate, and the removal exists so a *source* cannot preserve
      an exec bit, which is a reason that does not apply here. Remove the temporary file if
      anything fails
- [x] 2.4 GREEN: Add `graft.toml` to the pre-flight pass beside the existing
      `checkDestination(repo, lock.Filename)` — **only on a run that was given manifest bytes**,
      so `graft sync` neither reads nor writes it and its behavior is unchanged
- [x] 2.5 REFACTOR: State in a comment why this write does **not** go through
      `checkReservedWrite` — it is graft's own file, the same class as `graft.lock`, which this
      package has always written — and that the reserved refusal still governs every path in
      `p.Writes` and `p.Prune`
- [x] 2.6 CHECK: Contract gate — confirm `graft.lock` is still the last write on every path, that
      a run given manifest bytes that fails before them writes neither file, and that every
      existing `internal/apply` test still passes **unmodified** — the empty-plan test already
      asserts the whole tree listing, which is what proves no `graft.toml` was created
- [x] 2.7 Run `go test ./internal/apply/` — green, no regressions elsewhere

## 3. sync: re-resolution as a parameter, and the narrowed pin check
<!-- kind: behavior -->

**Concentration point.** graft may never delete a file absent from `graft.lock`, and an update is
a new way to reach the prune set — a moved pin whose new rev stops providing an installed item.
This group therefore carries the test proving a repo-owned file in a shared destination survives.

**Concentration point.** Fixture git repositories need `user.name` and `user.email` set on **the
repository**, not the machine, or every test in this group and the next two fails on a clean CI
runner. `internal/sync/fixture_test.go` already does this; extend it rather than writing a second
harness.

- [x] 3.1 RED: Extend `internal/sync/fixture_test.go` with an `update` helper beside `run` and
      `dryRun`, then write failing tests for: *A moved branch moves the pin* (asserting the paired
      `sync` on the same fixture does **not** move it), *An update that finds nothing new reports
      nothing*, *An update in a repository with no lock installs everything*, *A source dropped
      from the manifest is pruned without being re-resolved*, *A rev that no longer exists fails
      without touching the tree*, *A manifest declaring no sources updates nothing*
- [x] 3.2 RED: Write the failing prune test — *An item the new rev no longer provides is removed,
      and a repo-owned file beside it survives*: the destination holds a `local-reviewer.md` that
      no lock claims, the newer rev stops providing one item, and after the run the item's file is
      gone, `local-reviewer.md` is byte-identical, and the directory still exists
- [x] 3.3 RED: Write failing tests for the source argument: *Only the named source is
      re-resolved*, *A source name the manifest does not declare is refused*, *A source in the
      lock but not the manifest cannot be updated*
- [x] 3.4 RED: Write failing tests for the narrowed pin check: *An update repairs a manifest whose
      rev was moved by hand*, *Updating one source still refuses another source's disagreement* —
      the second asserting the cache root is still empty, so the refusal precedes the network
- [x] 3.5 GREEN: Add `sync.Update{Source, To}` and `sync.Options.Update *Update`, with a doc
      comment recording that `To` is honoured only together with `Source` and that the command
      surface refuses the other combination. The zero `Options` must remain exactly today's
      `graft sync`, so no existing test in the package changes
- [x] 3.6 GREEN: Refuse an `Update.Source` the manifest does not declare with
      `graft.toml has no source "<name>"`, **before** any resolution, any fetch, and any attempt
      to edit the manifest — so a mistyped name never produces `SetRev`'s shape refusal instead
- [x] 3.7 GREEN: Hand `lock.CheckPins` only the sources this run does not re-resolve, and take a
      re-resolved source's sha from `source.Resolve` rather than from the lock
- [x] 3.8 REFACTOR: Confirm by reading the function that `plan`, `catalog`, and `apply` see no
      difference between a sync and an update, and that the only branch is which sha a source gets
- [x] 3.9 Run `go test ./internal/sync/` — green, every pre-existing sync test unchanged

## 4. sync: `--to` moves the pin, and the report says so
<!-- kind: behavior -->

- [x] 4.1 RED: Write failing tests for: *The pin moves and the rest of the file survives* (a
      manifest with a leading comment, aligned keys, and a second source; assert the result differs
      in exactly one line), *An update without `--to` never writes the manifest*, *A `--to` rev
      that does not exist leaves the manifest where it was*, *A `--to` naming a source the manifest
      does not declare is refused*, *A `--to` against a manifest shape the editor cannot rewrite is
      refused* — the last asserting the cache root stays empty and that `graft update shared`
      succeeds on the same fixture
- [x] 4.2 RED: Write failing tests for the report: *A bumped tag shows both revs and both shas*
      and *A branch whose sha moved shows one rev and both shas*, both asserting the rendered
      header exactly. Assert the report is the one `sync-report` specifies rather than a second
      rendering
- [x] 4.3 GREEN: When `Update.To` is set, run `manifest.SetRev` on the bytes `manifest.Read`
      returned, and hand the result to `apply.Run` through `WithManifest`
- [x] 4.4 GREEN: Re-parse the edited bytes and **assert the named source's rev is `Update.To`**
      before the run uses them, failing without writing anything if the parse fails or the value is
      not what was asked for. This is what turns every wrong-target text edit — a commented-out
      key, a sub-table key — from a silent success into a failed run
- [x] 4.5 REFACTOR: State in a comment why the edited bytes are re-parsed rather than the parsed
      manifest being mutated: what goes to disk and what the run resolves must be the same thing,
      and re-parsing is the only way to prove it
- [x] 4.6 CHECK: Contract gate — this group is what causes `graft.toml` to be rewritten in a real
      run. Confirm a `graft.toml` written by an update still parses through `manifest.Load`, that
      `lock.CheckPins` agrees with it on the next `graft sync`, and that its bytes differ from the
      original in exactly one line
- [x] 4.7 Run `go test ./internal/sync/` — green

## 5. sync: `--dry-run` under an update
<!-- kind: behavior -->

- [ ] 5.1 RED: Write failing tests for: *A dry run of an update writes neither of graft's files*,
      *A dry run of `--to` leaves the manifest where it was*, *A dry run of a first update creates
      no directory* — the last asserting the tree holds only `graft.toml`, which is the half of
      SPEC.md's promise a file-existence check would miss
- [ ] 5.2 GREEN: Confirm the existing early return after `plan.Build` already covers the manifest
      write, and add nothing if it does; if the `SetRev` result reaches disk under `--dry-run`,
      move the write behind the same return
- [ ] 5.3 REFACTOR: None expected — `--dry-run` returns before `apply.Run` and the manifest write
      lives inside it. State that in the verification task rather than leaving a checked box that
      overstates what was done
- [ ] 5.4 Run `go test ./internal/sync/` — green

## 6. cli: the `update` command
<!-- kind: behavior -->

- [ ] 6.1 RED: Write failing tests for: *A second argument is a usage error*, *An unknown flag is
      a usage error* (`graft: unknown flag: --force` plus the hint line), *`--to` with no source is
      a usage error*, *`--to` with an empty rev is a usage error*, *An update leaves standard
      output byte-empty*, *A subcommand's own help goes to standard output* (`graft update --help`)
- [ ] 6.2 RED: Extend the existing *Help lists the commands graft has* test to name `update` as
      well as `sync`, and confirm it fails before the command is registered
- [ ] 6.3 GREEN: Register `update` on the root with `--to` and `--dry-run`, and an argument
      validator producing `unknown argument "<argument>"`, `--to requires a source`, and
      `--to requires a rev` as usage errors — `--to`'s presence read through `Flags().Changed`, so
      an explicitly empty value is told apart from an absent flag. Cobra parses flags before it
      calls the argument validator, which is what makes that reachable there
- [ ] 6.4 REFACTOR: Extract the tail `sync` and `update` now share — working directory,
      `source.DefaultCacheRoot()`, `sync.Run`, and the report's lines through `ui.Note` — into one
      helper, so neither command holds a decision of its own
- [ ] 6.5 CHECK: Contract gate on the existing surface — confirm every other current
      `internal/cli` test still passes **unchanged**, including *`--help` prints the same text as
      no arguments at all*, *`help` is not a command*, and *`help sync` is not a command either*
- [ ] 6.6 Run `go test ./internal/cli/` — green

## 7. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [ ] 7.1 VERIFY: Confirm the group 0 acceptance test now passes end to end
- [ ] 7.2 REFACTOR: Fold any fixture helper this change duplicated into the package's existing
      one; do not share a helper across packages through a non-test file

## 8. Documentation: SPEC.md's invariant, its failure-mode table, and one AGENTS.md rule
<!-- kind: operational -->

- [ ] 8.1 CHECK: List the user-visible conditions this change adds against SPEC.md's Failure modes
      table and confirm none is already there — `graft.toml has no source`, the two `SetRev`
      refusals, and the `--to` usage errors. Confirm `rev` not found needs **no** new row, being
      already covered
- [ ] 8.2 CHANGE: Amend SPEC.md → Invariants. The bullet reading "graft never writes inside
      `.git`, and never over `graft.toml` or `graft.lock`" is false as written once `--to` exists;
      scope the refusal to paths arriving in a plan and name graft's own manifest write as the
      exception, matching the argument design.md makes. Confirm the sentence is not duplicated in
      AGENTS.md or ENGINEERING.md before editing only SPEC.md
- [ ] 8.3 CHANGE: Add rows to SPEC.md's Failure modes table for `graft.toml has no source` and the
      two `SetRev` refusals, worded exactly as the messages the code produces. Collapse the two
      `--to` usage errors into **one** row — SPEC.md's table holds no row for `sync`'s existing
      `unknown argument`, and the Output section already generalises usage errors
- [ ] 8.4 CHANGE: Add one rule to AGENTS.md's "Rules that are easy to get wrong": `graft.toml` is
      a human's file, so the pin is moved by replacing one value in place rather than by
      re-serializing a parsed manifest, a shape that cannot be edited exactly is refused rather
      than guessed at, and any change writing `graft.toml` needs a test asserting the result
      differs in exactly one line. Two to five lines, with the evidence clause its neighbours have.
      Leave the existing "`sync` never re-resolves a pin" rule **unedited** — this change is the
      command it points at, so it becomes more load-bearing rather than stale
- [ ] 8.5 VERIFY: Re-read the amended SPEC.md rows against the error strings the tests assert and
      confirm each is character-for-character the message the code produces

## 9. Change Review
<!-- kind: operational -->

- [ ] 9.1 CHECK: Dispatch an **independent reviewer subagent** — not a fork of the implementing
      session — against proposal.md, all four spec files, design.md, tasks.md, and the diff.
      Point it explicitly at: that `graft sync`'s behavior is byte-for-byte unchanged and every
      pre-existing test still passes unmodified; that no code path in `internal/apply` can delete
      a path absent from `graft.lock`; that `internal/plan` gained no filesystem access and no
      knowledge of `update`; that `cmd/graft` gained no decision; and that `SetRev` cannot edit a
      line it did not mean to — the sub-table, the trailing comment, the commented-out key, and
      the multi-line value each get looked at in the code, not just in the tests
- [ ] 9.2 CHECK: Have the reviewer confirm no source-provided content can cause anything to
      execute, that a source can influence neither which rev is resolved nor what `SetRev` writes,
      that a `--to` value cannot reach git as an operand, and that no test reaches this
      repository's own tree, its own `.claude/agents/`, or the developer's real `~/.cache/graft`
- [ ] 9.3 CHANGE: Fix every CRITICAL, resolve or accept each WARNING with the reason recorded, and
      re-run the affected tests
- [ ] 9.4 VERIFY: Confirm no blocking or unowned finding remains

## 10. Lint & Verify
<!-- kind: operational -->

- [ ] 10.1 CHECK: Inspect the verification commands `Taskfile.yml` defines and confirm the affected
      tiers are `./internal/manifest`, `./internal/apply`, `./internal/sync`, and `./internal/cli`
- [ ] 10.2 VERIFY: Run `task lint` — 0 errors, `gofumpt -l` included
- [ ] 10.3 VERIFY: Run `task cover` — green and above the 80% floor over `./internal/...`
- [ ] 10.4 VERIFY: Run `task build` — succeeds
- [ ] 10.5 VERIFY: Run `openspec validate update-command --strict` — clean
