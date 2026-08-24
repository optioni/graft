<!-- Four of openspec/config.yaml's task concentration points do not apply to this change and
     are recorded here rather than left silent: the **prune set** (nothing is deleted; there
     is no lock), **`internal/plan` purity** (untouched, and neither new package imports it),
     **lock serialization determinism** (no lock is read or written), and **fixture git
     repositories** (no test runs git). The two that do apply — error strings are asserted
     contracts, and coverage is measured over `./internal/...` only — are called out at the
     groups they govern. -->

## 0. Acceptance Test — Outer Loop RED
<!-- kind: behavior -->

The outer loop is taken here and design.md → Test Strategy says why: `cmd/graft` is the one
file `task ci` never executes, because `cover` runs `go test` over `./internal/...` only.
The headline scenario is *No arguments prints help and succeeds*, not *`--version` goes to
stdout* — the placeholder already answers `--version`, and an acceptance test that is green
before any work starts is not an outer loop.

- [x] 0.1 Set up the harness named in design.md → Test Boundaries: create
      `internal/cli/cli.go` holding only the package doc comment — `cli` turns arguments into
      an outcome and an exit code, and lives under `./internal/` so the coverage gate can see
      it — and add `internal/cli/acceptance_test.go` with **one** parent test that owns a
      `t.TempDir()`, builds `go build -o <dir>/graft github.com/optioni/graft/cmd/graft` once,
      and runs each case as a subtest. One parent test rather than a package-level
      `sync.Once`: a `t.TempDir()` is removed when *its* test ends, so a shared build would
      hand later tests a path that no longer exists. Each run captures stdout and stderr as
      **separate** buffers, reads the exit status from `*exec.ExitError` via `errors.As`, and
      sets the child's working directory to a fresh `t.TempDir()`
- [x] 0.2 RED: Write the failing end-to-end test for *No arguments prints help and
      succeeds*: run the binary with no arguments and assert stdout names `graft`, contains
      `Usage:`, and lists `--version` and `--help`; stderr is byte-empty; the exit code is
      `0`; and the child's working directory is still empty
- [x] 0.3 Confirm it fails because the behavior is missing rather than because the harness is
      misconfigured: the placeholder writes `graft: not implemented yet — see SPEC.md` to
      stderr and exits `1`, so record that the observed failure is that message and that exit
      code — not a build error, not an empty capture, and not a missing binary

## 1. Dependencies
<!-- kind: operational -->

Plumbing, so no RED: a `go.mod` entry is not a behavior, and a test asserting a dependency is
listed only proves the same string was typed twice. The check is that the toolchain resolves
and the tree still builds.

- [x] 1.1 CHECK: Ask the module proxy for the newest stable versions rather than recalling
      them — `go list -m -versions github.com/spf13/cobra` and `go list -m -versions
      golang.org/x/term` — and record what they were on the day of the change
- [x] 1.2 CHECK: Confirm `go.mod` currently requires only `github.com/BurntSushi/toml` and
      `github.com/goccy/go-yaml`, so the diff this group produces is exactly the two direct
      additions and their indirect closure
- [x] 1.3 CHECK: Persistence gate — confirm what this change requires of stored data:
      migration, backfill, seeding, cache invalidation, and index rebuild are **all none**.
      This change reads no lock, writes no cache entry, and opens no file for writing
      (design.md → Persistence and Rollout)
- [x] 1.4 CHANGE: `go get github.com/spf13/cobra@<latest>` and `go get
      golang.org/x/term@<latest>`. **Do not run `go mod tidy` here** — nothing imports either
      module yet, so tidy prunes both straight back out; they land as `// indirect` and are
      promoted to direct requirements by the `go mod tidy` at the end of group 4, once the
      first import exists. Discovered by running it: tidy reverted `go.mod` to its two
      original requires
- [x] 1.5 VERIFY: Run `task build` and `task lint` — the module resolves, the tree compiles,
      and `gofumpt -l` is silent. That the dependencies are usable is what the build proves;
      no test asserts their presence

## 2. The output surface — streams and the error format
<!-- kind: behavior -->

**Concentration point.** Error strings are asserted by tests. `graft: ` plus the message the
producing package already wrote is the whole format, and every message in SPEC.md's
failure-mode table locates its own problem — a second layer of context would say the same
thing twice.

- [x] 2.1 RED: Write failing tests in `internal/ui` for *Machine-readable output goes to
      stdout only*; *A note leaves stdout untouched*; *An error report leaves stdout
      untouched*. Each asserts the other stream is **byte-empty**, not merely "does not
      contain" — `stdout == ""` is the assertion that would go red if the split inverted
- [x] 2.2 RED: Write *A failure-mode-table message is reported verbatim after the prefix*
      using a real message this repository already produces — `source "shared": selector
      "agent:*" matches no item; catalog provides schema:tdd`, `internal/catalog`'s own
      wording — asserting the stderr buffer is byte-exactly `graft: ` + that message + `\n`.
      A message invented for the test would not catch a printer that mangles quoting
- [x] 2.3 RED: Write *A nil error reports nothing*, asserting both buffers are byte-empty
- [x] 2.4 GREEN: Add `internal/ui/ui.go` with `type UI`, `New(out, err io.Writer, color
      bool) *UI`, and `Print`, `Note`, and `Fail`, each taking a plain `string` — no `Printf`
      and no variadic `Note`, because `unparam` flags a format parameter never given
      arguments and `fmt.Sprintf` is right there. `Fail(nil)` returns without writing;
      `Fail(err)` writes `graft: ` + `err.Error()` + `\n` to the error stream and nothing
      anywhere else. Give every exported declaration its own doc comment: revive's `exported`
      rule counts a comment shared by two declarations as a comment on neither
- [x] 2.5 GREEN: Wrap each stream in an unexported recorder that keeps the first write
      failure, and expose `WriteError() error` plus `Out()` and `Err()` returning those
      recorders (design.md → D11). `Out()` and `Err()` exist so cobra writes *through* the UI
      in group 4 rather than around it; without them a `graft --help` whose stdout is full
      exits `0` having printed nothing, because `cobra.Command.Help()` returns `nil`
      unconditionally and discards its renderer's error
- [x] 2.6 REFACTOR: Confirm every write in the package goes through the recorder for its
      stream, so no report line can reach a stream directly and skip the write-error
      recording — or state that no refactor was needed
- [x] 2.7 Run `go test ./internal/ui/` — no regressions

## 3. The colour decision and terminal detection
<!-- kind: behavior -->

**Concentration point.** The colour decision is a pure function of two values and the
terminal test is the one impure call, split exactly as `source.DefaultCacheRoot` splits from
`defaultCacheRoot` (design.md → D9). Testing the rule must not require a terminal, and
testing the detector must not require a stub.

- [x] 3.1 RED: Write failing table tests for `ColorEnabled(noColor string, terminal bool)`
      covering *A terminal with NO_COLOR unset gets colour*; *NO_COLOR set to any non-empty
      value drops colour*, with the cases `1`, `0`, and `false` — `0` and `false` are the two
      a naive truthiness check gets wrong; *NO_COLOR set to the empty string does not drop
      colour*, which is the published `NO_COLOR` convention; *A redirected stdout drops
      colour*
- [x] 3.2 RED: Write *Styling with colour off is byte-identical to the input*: apply `Bold`
      and `Dim` to `removed  agent:phase-orchestrator` with colour disabled and assert `==`
      against the input, including unchanged length. Assert the colour-enabled direction too —
      the result begins with an escape sequence and still contains the input — so a helper
      that always returns its argument cannot pass
- [x] 3.3 RED: Write *A character device that is not a terminal is not a terminal* against the
      **real** `IsTerminal`: a `bytes.Buffer`, both ends of a real `os.Pipe`, and a real
      `*os.File` opened at `/dev/null`. `/dev/null` is the case that motivates the dependency
      — it has the character-device mode bit set, so `fi.Mode()&os.ModeCharDevice != 0`
      answers "terminal" for `graft sync > /dev/null` (design.md → D8). Add **no** pty case:
      it was investigated and does not work on darwin — `/dev/ptmx` opens successfully and
      `term.IsTerminal` on the master returns false, so a "skip when unavailable" test never
      skips, it fails permanently here and on the `macos-latest` CI leg. Record in the test
      file that the one unasserted step is `term.IsTerminal` returning true, which is
      `golang.org/x/term`'s own contract (design.md → Risks)
- [x] 3.4 GREEN: Add `ColorEnabled(noColor string, terminal bool) bool` — enabled when
      `noColor == ""` and `terminal` — and `IsTerminal(w io.Writer) bool` type-asserting to
      `*os.File` and calling `term.IsTerminal(int(f.Fd()))`, returning false for anything that
      is not an `*os.File`
- [x] 3.5 GREEN: Add `Bold` and `Dim` as methods on `*UI`, returning their argument unchanged
      when colour is disabled
- [x] 3.6 REFACTOR: Confirm nothing in `internal/ui` reads an environment variable or a global
      stream — `os.Getenv`, `os.Stdout`, and `os.Stderr` appear nowhere in the package — so a
      test cannot be made to pass by the developer's shell. Or state that no refactor was
      needed
- [x] 3.7 Run `go test ./internal/ui/` — no regressions

## 4. The root command, `--version`, and help
<!-- kind: behavior -->

- [ ] 4.1 RED: Write failing tests for *The version line goes to stdout*, asserting the stdout
      buffer is byte-exactly `buildinfo.Format("v1.2.0", "abc1234", "2026-08-23") + "\n"` —
      built by **calling** `buildinfo.Format`, not by restating its output, so the two cannot
      drift; *An unbuilt binary still prints a version line*, all three strings empty; *A
      successful invocation exits 0*, asserting the returned `int` and an empty stderr
- [ ] 4.2 RED: Write *No arguments prints help and succeeds* and *`--help` prints the same
      text as no arguments at all* at the unit tier: the first asserts stdout names `graft`,
      contains `Usage:`, and lists `--version` and `--help`; the second asserts `bytes.Equal`
      between the two stdout buffers. Do **not** assert that the help text omits `completion`:
      cobra omits it from root help anyway while the root has no other subcommands, so that
      assertion stays green with `DisableDefaultCmd` removed and cannot fail. Substrings
      and a relation, never a golden file — every later change adds a command, and a churning
      golden is a golden nobody reads (design.md → Test Strategy)
- [ ] 4.3 GREEN: Add `Options` and `Main(o Options) int` in `internal/cli`, defaulting
      `Getenv` to `os.Getenv` and `IsTerminal` to `ui.IsTerminal` when nil, and building the
      `*ui.UI` from `ui.ColorEnabled(o.Getenv("NO_COLOR"), o.IsTerminal(o.Stdout))`
- [ ] 4.4 GREEN: Build the cobra root command with `SilenceErrors` and `SilenceUsage` both
      true, and install the streams with `SetOut(u.Out())` / `SetErr(u.Err())` — the UI's
      **recording** writers, not the process's real streams. This is not tidiness:
      `cobra.Command.Help()` returns `nil` unconditionally and discards its renderer's write
      error, so with the raw streams a `graft --help` onto a full disk exits `0` having
      printed nothing anywhere. Routing cobra through the recorders makes that failure visible
      to `WriteError()`, and it also stops any cobra path reaching the real `os.Stdout` from
      inside a test (design.md → D2)
- [ ] 4.5 GREEN: Declare `--version` as graft's own bool flag and handle it in `RunE`,
      printing through `ui.Print`; with the flag absent, `RunE` calls `cmd.Help()`. Cobra's
      `Version` field is deliberately not used: it writes through a template straight to
      `OutOrStdout`, bypassing the package that owns machine-readable output
      (design.md → D5)
- [ ] 4.6 GREEN: Set `CompletionOptions.DisableDefaultCmd = true` (design.md → D6)
- [ ] 4.7 REFACTOR: Confirm the root command is built by one unexported constructor that
      takes the `*ui.UI` and the build strings, so a test can construct a root and register a
      command against it without going through `Main` — or state that no refactor was needed
- [ ] 4.8 Run `go test ./internal/cli/ ./internal/ui/` — the acceptance test from group 0 is
      still red at this point, because `cmd/graft` has not been rewired; confirm that is the
      only failure

## 5. Refusing what graft does not recognise, and the exit code
<!-- kind: behavior -->

**Concentration point.** Error strings are asserted by tests and every one below is a
deliberate contract. `unknown command "<arg>"` is graft's own; `unknown flag: <flag>` is
pflag's and is passed through on purpose (design.md → Contracts).

- [ ] 5.1 RED: Write failing tests for *An unknown command names the argument*; *`version` is
      not a command*; *No completion command is offered*; *An unknown flag is a usage error
      too*; *An unknown shorthand flag is a usage error*, asserting pflag's own
      `unknown shorthand flag: 'v' in -v` — a different message from the long-flag case, and
      `-v` is a realistic invocation because D5 reserves the shorthand; *Only the first
      unrecognised argument is named*, asserting `frobnicate` is present and `wibble` absent.
      Each asserts the exit code is `1`, the **stdout buffer is byte-empty**, and the exact
      stderr text
- [ ] 5.1a RED: Write *Cobra's hidden completion protocol is refused too* for `__complete` and
      `__completeNoDesc`. `CompletionOptions.DisableDefaultCmd` does not remove these: cobra
      registers them on every `Execute`, and with the flag set `graft __complete ''` still
      exits `0` and writes `:0` to stdout, bypassing both the UI and the argument validator.
      Assert the stdout buffer is byte-empty, which is what goes red if the refusal is absent
      (design.md → D6)
- [ ] 5.2 RED: Write *A usage error carries a hint on its own line*, asserting stderr is
      exactly two lines, that the second is `run "graft --help" for usage`, and that the
      second carries **no** `graft: ` prefix — it is not a second failure. Assert also that
      the full usage block does not appear, which is what `SilenceUsage` is for
- [ ] 5.3 RED: Write *A command that returns an error exits 1* and *A command that succeeds
      exits 0* as in-package tests that register a test-only command on the root — the only
      way to exercise routing before `sync` exists (design.md → Test Boundaries). The failing
      one returns the real wording `source "shared": rev "v9.9.9" not found` and asserts **no**
      hint line follows, so a usage-error marker applied to everything cannot pass
- [ ] 5.4 RED: Write *Output that cannot be written is not a silent success*: give `Main` a
      stdout that fails every write with `disk full`, run `--version`, and assert the exit code
      is `1` and stderr holds exactly `graft: cannot write output: disk full`. Assert it fails —
      an implementation that ignores write errors returns `0` here, which is the whole point
- [ ] 5.4a RED: Write *Help that cannot be written is not a silent success either* against the
      same failing stdout, for `--help` and for no arguments at all. Both must exit `1`. This
      is the case that fails without task 4.4's recording writers, and it covers the two most
      common invocations, so a green here is the evidence that `Help()`'s discarded error was
      actually closed
- [ ] 5.5 RED: Write *The colour decision follows stdout and never stderr* as an **identity**
      assertion, not an output one: the `Options.IsTerminal` stub records every writer it is
      asked about, and `TestMainColorFollowsStdout` asserts it was asked about `Options.Stdout`
      and never about `Options.Stderr`. Nothing in this change emits styled output through
      `Main` and `UI`'s colour flag is unexported, so an output-based assertion here would
      either be a no-op or force an accessor open; the identity assertion goes red if the
      wiring is inverted
- [ ] 5.6 GREEN: Add the root's `Args` validator returning `usagef("unknown command %q",
      args[0])`, and `SetFlagErrorFunc` wrapping pflag's error in the same unexported
      `usageError` type. Refuse by graft's own validator rather than by matching cobra's
      message text (design.md → D3, D4)
- [ ] 5.6a GREEN: Refuse `__complete` and `__completeNoDesc` in `Main` before `Execute` is
      reached, through the same `usagef` path. Match the two literal names as the **first**
      argument only — a `__` prefix rule could swallow a future command — and keep the two
      lines adjacent to `DisableDefaultCmd` so enabling completion later removes both together
      (design.md → D6)
- [ ] 5.7 GREEN: Map the outcome to an exit code in `Main`: `Execute`'s error, or failing
      that `ui.WriteError()`; report it with `ui.Fail`; append the hint with `ui.Note` when
      `errors.As` finds a `usageError`; return `1`, else `0`. Define no third code —
      SPEC.md admits `0` and `1` (design.md → D4)
- [ ] 5.8 CHECK: Contract gate — re-read SPEC.md's **Output** section, its **Exit codes**
      line, and its **failure-mode table**, and confirm what was built still matches: stdout
      carries machine-readable output only, progress and errors go to stderr, colour drops
      when stdout is not a TTY or `NO_COLOR` is set, and the only codes are `0` and `1`.
      Confirm no message in the failure-mode table is reworded by this change — the format
      wraps, it does not rewrite
- [ ] 5.9 REFACTOR: Confirm `usagef` is the single constructor for the class and that nothing
      compares an error's text to decide whether it is a usage error — or state that no
      refactor was needed
- [ ] 5.10 Run `go test ./internal/cli/ ./internal/ui/` — no regressions beyond the still-red
      acceptance test

## 6. Wiring `cmd/graft`
<!-- kind: behavior -->

**Concentration point.** Coverage is measured over `./internal/...` only, and `task ci` never
runs `go test` outside it — so anything left in `cmd/graft` is not merely unmeasured, it is
unexecuted. This group's job is to leave nothing there.

- [ ] 6.1 RED: Confirm group 0's acceptance test is still red and still red for the right
      reason — `main` routes to the placeholder `run`, so the binary prints
      `graft: not implemented yet — see SPEC.md` and exits `1`
- [ ] 6.2 GREEN: Rewrite `cmd/graft/main.go` as the package doc comment, the three
      linker-injected `var`s, and a `main` that calls `os.Exit(cli.Main(cli.Options{…}))`
      with `os.Args[1:]`, `os.Stdout`, `os.Stderr`, and the three build strings. Delete `run`
      and the `errors`, `fmt`, and `io` imports
- [ ] 6.3 RED: Write `TestCmdGraftImports` in `internal/cli` — a Go **test**, not a command
      run by hand — shelling out to `go list -f '{{join .Imports "\n"}}' ./cmd/graft` and
      asserting the result is exactly `github.com/optioni/graft/internal/cli` and `os`, in that
      order (`.Imports` comes back lexically sorted), and to `go list ./cmd/...` asserting
      exactly one package. `internal/buildinfo` must **not** appear: the build strings travel
      to `cli` as strings and `buildinfo.Format` is called where the coverage gate can see it,
      so an import of it here is the failure. A command run by hand would not go red when a
      later change adds an import to `main.go`, which is the regression this guards, in the one
      directory CI never runs a test over. The `go` toolchain is already a named collaborator
      (design.md → Test Boundaries)
- [ ] 6.4 VERIFY: Confirm `TestCmdGraftImports` passes against the rewritten `main.go`, and
      record the `go list` output it saw
- [ ] 6.5 Run `go test ./internal/...` — no regressions

## 7. Acceptance Test — Outer Loop GREEN
<!-- kind: behavior -->

- [ ] 7.1 VERIFY: Confirm group 0's acceptance test now passes end to end against the
      compiled binary — help on stdout, empty stderr, exit `0`, working directory untouched
- [ ] 7.2 VERIFY: Extend the acceptance run with `--version` and with `frobnicate`, asserting
      the stream each lands on and the exit code, so the split and the code are proven across
      a real process boundary and not only across two `bytes.Buffer`s
- [ ] 7.3 REFACTOR: Confirm the binary is built once per test binary rather than once per
      subtest, and that the build output and the child's working directory are both under
      `t.TempDir()` — or state that no refactor was needed

## 8. Documentation
<!-- kind: operational -->

- [ ] 8.1 CHECK: Re-read SPEC.md's **Output** and **Exit codes** sections and identify what is
      now true and undocumented. The error format `graft: <message>` is the one durable fact
      this change establishes that SPEC.md does not state, and SPEC.md is where the
      failure-mode table already lives
- [ ] 8.2 CHANGE: Add to SPEC.md's Output section a short paragraph pinning the error format —
      `graft: ` plus the message, one line, on stderr; a usage error additionally pointing at
      `graft --help`; and `NO_COLOR` disabling colour when present and non-empty. Net addition
      to SPEC.md: ~5 lines. It replaces nothing, because nothing there says it today
- [ ] 8.3 CHANGE: Rewrite — do not append to — AGENTS.md's existing coverage rule under "Rules
      that are easy to get wrong". It currently says logic in `cmd/graft` is invisible to the
      gate; the sharper and more useful fact this change found is that `task ci` runs `lint`,
      `cover`, and `build`, and `cover` runs `go test` over `./internal/...` only, so a test
      outside `./internal/` **never runs in CI at all**. Add, in the same edit, that every byte
      a user sees is written by `internal/ui` and that `cmd/graft` prints nothing. Net change:
      ~4 lines, of which ~2 replace existing text
- [ ] 8.4 VERIFY: Confirm no section of SPEC.md, PRD.md, ENGINEERING.md, or AGENTS.md now
      contradicts what was built, and that `openspec/IMPLEMENTATION-ORDER.md`'s Phase 4 row
      for `command-surface` still describes this change accurately

## 9. Change Review
<!-- kind: operational -->

- [ ] 9.1 CHECK: Dispatch an independent reviewer — a fresh subagent given only proposal.md,
      the two spec files, design.md, tasks.md, and the diff, never a fork of the implementing
      session — with these concentration points named: (a) for every spec scenario, the test
      that would go red if the behavior were deleted, and specifically whether the stdout
      assertions are byte-empty rather than "does not contain"; (b) whether the colour
      decision genuinely follows **stdout** and not stderr, and whether a test would catch it
      if it followed the wrong one; (c) whether `NO_COLOR=0` and `NO_COLOR=false` disable
      colour, which a truthiness check gets wrong; (d) whether anything was left in
      `cmd/graft`, where CI never runs a test; (e) whether every asserted error string matches
      the specs and design.md → Contracts exactly, and whether any existing package's error
      wording was changed; (f) whether the exit code is `1` for every failure and no third
      code was introduced; (g) whether any file outside `t.TempDir()` is written by any test,
      `internal/apply` still being the only future writer of the working tree; (h) whether the
      help assertions are substrings and relations rather than a golden file that will churn
- [ ] 9.1a CHECK: Name to that reviewer the three things this plan deliberately did **not**
      do, and ask whether each deferral holds or hides a defect: no subcommand stubs, no
      timeout or signal handling (Q1), and no rewording of `internal/source`'s messages (Q2).
      A reviewer given only the general shape tends to re-derive the same list; the useful
      answer is a fourth
- [ ] 9.2 CHANGE: Fix every CRITICAL, resolve or consciously accept each WARNING with a
      one-line reason, note each SUGGESTION, and re-run the affected tests. Record the
      dispositions in planning-review.md
- [ ] 9.3 VERIFY: Confirm no blocking or unowned finding remains, and that any contract
      changed while fixing findings was written back into the owning artifact and into
      planning-review.md

## 10. Lint & Verify
<!-- kind: operational -->

- [ ] 10.1 CHECK: Inspect the intended verification commands and affected tiers — two tiers
      here, unit across `./internal/ui/` and `./internal/cli/`, and one acceptance test inside
      `./internal/cli/` that compiles and runs the real binary. Confirm the acceptance tier is
      under `./internal/` and therefore visible to both `task cover` and CI, rather than beside
      `main.go` where neither would run it
- [ ] 10.2 VERIFY: Run `task lint` — golangci-lint clean and `gofumpt -l` silent, 0 errors
- [ ] 10.3 VERIFY: Run `task test` — `go test -race ./...` green
- [ ] 10.4 VERIFY: Run `task cover` — at or above the 80% floor measured over `./internal/...`,
      and report `internal/ui`'s and `internal/cli`'s own figures alongside the total
- [ ] 10.5 VERIFY: Run `task build` — the binary builds, which is Go's type check, and run it
      once by hand with `--version` and with no arguments to see the real streams
- [ ] 10.6 VERIFY: Run `openspec validate command-surface --strict` — clean
