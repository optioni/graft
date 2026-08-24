## Context

Four changes have landed — `manifest-and-lock`, `catalog-and-selectors`,
`destination-and-plan`, and `git-fetch` — and every one of them is a library. `cmd/graft` is
still the scaffold it was on day one:

```go
func run(args []string, out io.Writer) error {
	if len(args) == 1 && (args[0] == "--version" || args[0] == "version") {
		_, err := fmt.Fprintln(out, buildinfo.Format(version, commit, date))
		return err
	}
	return errors.New("not implemented yet — see SPEC.md")
}
```

`openspec/IMPLEMENTATION-ORDER.md` puts this change before `sync-command` for one stated
reason: "so the first real command lands into a settled error format rather than
retrofitting one across four commands later." The three things that get settled here are the
error format, the stream split, and the exit code, and none of them is a large piece of
code. What makes the change worth its own proposal is **where the code goes**.

The constraint that shapes everything below is ENGINEERING.md's, restated in AGENTS.md:

> Coverage is measured over `./internal/...` only. `cmd/graft` is excluded deliberately —
> the way to hit a coverage number honestly is to keep `main()` trivial and put every
> decision in a package.

It is stronger than a coverage rule. `Taskfile.yml`'s `ci` task is `lint`, `cover`, and
`build`; `cover` runs `go test … ./internal/...`. So a test file under `cmd/graft` would
**never run in CI at all** — not merely be invisible to the gate. A command surface built
inside `main()` would therefore be both untested and unmeasured, and the first time anyone
noticed would be a user-visible regression. Everything except `os.Args`, the two real
streams, the three linker-injected strings, and `os.Exit` goes into `./internal/`.

## Goals / Non-Goals

**Goals:**

- One place that writes to graft's output streams, holding the stdout/stderr split, the
  `graft: ` error format, and the colour decision.
- One place that turns arguments into an outcome: a cobra root command with `--version`,
  help, refusal of anything it does not recognise, and an exit code.
- `cmd/graft` reduced to wiring: it reads the process, calls one function, and exits with
  the `int` that function returned.
- Error text a user sees pinned by tests, as every existing package's already is.
- A colour decision that is a pure function of two inputs, so `NO_COLOR` and non-terminal
  behavior can be asserted without a terminal.
- An end-to-end check that the **built binary** behaves, since `cmd/graft` is the one file
  the unit suite cannot reach.

**Non-Goals:**

- **No subcommand.** `sync`, `update`, `add`, and `list` are later changes, and no stub is
  registered for them — a stub that exits 0 having done nothing is worse than
  `unknown command "sync"`.
- **No working-tree write.** `internal/apply` still does not exist, and nothing here opens a
  file for writing. The only file this change's tests create is a compiled binary under
  `t.TempDir()`.
- **No report vocabulary.** `added`, `updated`, `removed`, the per-item lines, the summary
  line, and `up to date` belong to `sync-command`.
- **No `--dry-run`, no `--json`, no verbosity flag, no logging, no telemetry.**
- **No signals, no timeouts, no `context` plumbing.** See Open Questions → Q1.
- **No shell completion.** See D6.
- **No third exit code.** SPEC.md admits `0` and `1`.
- **No change to any existing package's error strings, or to any file format.**
  `graft.lock` stays at `version = 1`.

## Boundaries

| Package | Touched | How |
|---|---|---|
| `internal/ui` | **new** | The output surface: the two streams, the `graft: ` error format, the colour decision, and the styling helpers. The only package in the module that writes to stdout or stderr. |
| `internal/cli` | **new** | The cobra root command, `--version`, help, argument and flag refusal, and the mapping from an error to an exit code. Depends on `ui` and `buildinfo`; nothing depends on it but `cmd/graft`. |
| `internal/buildinfo` | read-only | `cli` calls `buildinfo.Format` for the `--version` line. Its API and its tests are unchanged; it gains its first non-test caller — and that caller is `cli`, **not** `cmd/graft`, which loses the import it has today. |
| `cmd/graft` | **rewritten** | Loses `run`, `errors`, `fmt`, `io`, and `internal/buildinfo`, keeping only `os` and `internal/cli`. Keeps the three `var`s the linker writes into, and becomes `os.Exit(cli.Main(…))`. |
| `internal/{manifest,lock,catalog,itemid,plan,source}` | untouched | No import, no error-string edit, no signature change. `cli` registers no command, so it does not yet call any of them. |
| `internal/apply` | does not exist | **This change adds no write path.** Nothing here opens a file for writing; the acceptance test's compiled binary lands under `t.TempDir()`. |

New pieces follow patterns already in the tree:

- **An impure edge is split from the decision it feeds.** `source.DefaultCacheRoot` calls
  `defaultCacheRoot(getenv, home)` so the rule can be tested without touching the
  environment (`git-fetch` → D7). `ui` does the same shape one step further:
  `ColorEnabled(noColor string, terminal bool) bool` is a pure two-argument function, and
  `IsTerminal(io.Writer) bool` is the one impure call, tested separately against real files.
- **Errors are built in one place per condition and asserted by tests**, as `catalog.errf`,
  `manifest`'s `fail` closure, `plan.itemErrf`, and `source`'s per-source closure already
  are. `cli` gets `usagef`, which is both the constructor and the marker for the class of
  error that earns a hint line.
- **A caller passes the root in; nothing reads a global.** `Cache.Root` is a field so no
  test can reach the developer's `~/.cache/graft`; `cli.Options` carries the streams, the
  environment reader, and the terminal test for exactly the same reason.

## Contracts

Both packages are internal; nothing outside this module can depend on them, so nothing here
is breaking. The surface `sync-command`, `update-command`, `list-command`, and `add-command`
will all code against:

```go
package ui

// UI is graft's output surface: the two streams and one colour decision.
type UI struct { /* out, err *recorder; color bool */ }

// New builds a UI writing machine-readable output to out and everything else to err.
func New(out, err io.Writer, color bool) *UI

// ColorEnabled reports whether styled output is allowed. Pure.
func ColorEnabled(noColor string, terminal bool) bool

// IsTerminal reports whether w is a terminal. False for anything that is not an *os.File.
func IsTerminal(w io.Writer) bool

// Out returns the machine-readable stream as a writer whose failures the UI records.
// It exists so a dependency that renders its own output writes through the UI rather
// than around it.
func (u *UI) Out() io.Writer

// Err returns the human-facing stream as a writer whose failures the UI records.
func (u *UI) Err() io.Writer

// Print writes one line of machine-readable output to the stdout stream.
func (u *UI) Print(s string)

// Note writes one line of human-facing text to the stderr stream.
func (u *UI) Note(s string)

// Fail writes "graft: <err>" to the stderr stream. A nil error writes nothing.
func (u *UI) Fail(err error)

// WriteError returns the first write failure on either stream, or nil.
func (u *UI) WriteError() error

// Bold styles s when colour is enabled, and returns s unchanged when it is not.
func (u *UI) Bold(s string) string

// Dim styles s when colour is enabled, and returns s unchanged when it is not.
func (u *UI) Dim(s string) string
```

Every declaration carries its own doc comment because `.golangci.yml` enables revive's
`exported` rule, which counts a comment shared by two declarations as a comment on neither.
There is no `Printf` and no variadic `Note`: `unparam` flags a format parameter that is
never given arguments, and a caller that needs formatting has `fmt.Sprintf`.

```go
package cli

// Options is everything the command surface reads from its process.
type Options struct {
	Args                    []string  // os.Args[1:]
	Stdout, Stderr          io.Writer
	Getenv                  func(string) string // nil means os.Getenv
	IsTerminal              func(io.Writer) bool // nil means ui.IsTerminal
	Version, Commit, Date   string    // the linker-injected build strings
}

// Main runs graft and returns the process exit code: 0 on success, 1 on any error.
// It never calls os.Exit and never touches os.Stdout, os.Stderr, or os.Args itself.
func Main(o Options) int
```

`cmd/graft/main.go` becomes, in full:

```go
func main() {
	os.Exit(cli.Main(cli.Options{
		Args: os.Args[1:], Stdout: os.Stdout, Stderr: os.Stderr,
		Version: version, Commit: commit, Date: date,
	}))
}
```

**Error surface** — every message below is asserted by a test and is a deliberate contract:

| Condition | Stream | Text |
|---|---|---|
| Any error from a command | stderr | `graft: ` + the error's own message, one line |
| An unrecognised argument | stderr | `graft: unknown command "frobnicate"`, then `run "graft --help" for usage` |
| `__complete` or `__completeNoDesc` | stderr | the same, naming the argument (D6) |
| An unrecognised long flag | stderr | `graft: unknown flag: --nope`, then `run "graft --help" for usage` |
| An unrecognised shorthand flag | stderr | `graft: unknown shorthand flag: 'v' in -v`, then the same hint |
| A write to either stream that fails | stderr | `graft: cannot write output: ` + the OS error |

`unknown flag: <flag>` and `unknown shorthand flag: 'v' in -v` are **pflag's** wording, not
graft's, and are deliberately passed through: they are accurate, they are stable across
cobra's own compatibility promise, and restating them would create a second thing to keep in
step. `unknown command "<arg>"` is graft's own and is produced by the root's argument
validator (D3).

## Persistence and Rollout

- **Migration**: none. No file format changes; `graft.lock` stays at `version = 1`.
- **Backfill**: none.
- **Seeding**: none.
- **Cache invalidation**: none. This change reads no cache and writes no cache entry.
- **Index rebuild**: none.
- **Authorization**: none. No command exists that could need any.
- **Observability**: none, deliberately. No logging, no metrics, no telemetry — SPEC.md
  forbids telemetry outright, and a verbosity flag is scope this change does not need.
- **Deployment**: none. No release is cut for this change. `Taskfile.yml` and the GitHub
  Actions workflow are unchanged; `task build` already injects the three linker variables
  this change keeps reading.

**Effect on `graft.lock`'s format: none.** `version` stays `1`. This change neither reads
nor writes a lock.

**Visual design source**: not applicable. This change builds no user-facing view and no
email template; the only "design" is terminal text, which is specified in the specs
themselves.

## Test Boundaries

| Dependency | In acceptance test | In unit tests |
|---|---|---|
| The two output streams | **real `*os.File` pipes**, captured from the compiled binary's stdout and stderr as separate buffers, so the split is asserted against real file descriptors rather than against two `bytes.Buffer`s a test wired up itself | **replaced** by `bytes.Buffer`, and by a writer that fails every write for the write-failure scenario |
| Process arguments | **real** — passed on the compiled binary's command line | **replaced** by `Options.Args`; no test reads `os.Args` |
| Exit code | **real** — read from the child process's exit status via `exec.ExitError` | **replaced** — `Main` returns an `int`, which is the whole reason it does |
| Environment (`NO_COLOR`) | **not set** — the acceptance test asserts nothing about colour | **replaced** by an `Options.Getenv` stub and by direct calls to the pure `ColorEnabled`; no test calls `t.Setenv` for `NO_COLOR` |
| Terminal detection | **real, and never a terminal** — a piped child process is not one, which is the whole point of the split being safe under a pipe | **replaced** by an `Options.IsTerminal` stub for the decision tests; **real** in `TestIsTerminal`, which asks the real `ui.IsTerminal` about a `bytes.Buffer`, both ends of a real `os.Pipe`, and a real `*os.File` opened at `/dev/null`. No test opens a pty — see Risks |
| Filesystem (the repository graft runs in) | **not used.** The compiled binary runs with its working directory set to `t.TempDir()`, and the test asserts that directory is still empty afterwards | **not used.** No unit test writes anywhere; the only files any unit test *opens* are `/dev/null` and the two ends of an `os.Pipe`, both read-only in effect and both named in the terminal-detection row |
| Filesystem (build output) | **real**, under a directory the acceptance test owns for the whole test binary and removes afterwards | **not used** |
| `go` toolchain | **real.** The acceptance test shells out to `go build` to produce the binary under test, and `TestCmdGraftImports` shells out to `go list`; both are the only way to reach `cmd/graft`, which no other test can | **not used** |
| Go module proxy | **not used** — the module cache is already warm by the time any test runs | **not used.** Group 1 contacts it, but that is `go get` run by a human, not by a test |
| `git` binary | **not used** | **not used** |
| Network | **not used** | **not used** |
| `github.com/spf13/cobra` | **real** — it is the thing under test | **real.** A fake cobra would test the fake, and cobra's own parsing is precisely where an unknown flag is detected |
| `golang.org/x/term` | **real**, transitively | **real** in `TestIsTerminal`; the decision it feeds is tested through the stub instead |
| `internal/buildinfo` | **real** | **real.** `Format` is called, not restated: the expected version line is built by calling it |
| A registered subcommand | **not used** — the shipped binary has none | **replaced** by a test-only command added to the root inside an in-package test, which is the only way to exercise routing, a command's error, and a command's stdout before `sync` exists |
| Clock, randomness | **not used** | **not used.** The build directory's name comes from `t.TempDir()`, which no assertion depends on |

## Test Strategy

Two tiers, both under `./internal/...` so `task cover` runs both:

- **unit** — `./internal/ui/` and most of `./internal/cli/`. Buffers, stubs, and pure
  functions; no subprocess, no file, no environment variable.
- **acceptance** — one file in `./internal/cli/`, which compiles `./cmd/graft` with `go
  build` into `t.TempDir()` and runs the resulting binary with real arguments, real pipes,
  and a real exit status.

**This change takes the outer-loop acceptance test**, and it is the first change that can.
The client-visible interface *is* the change, and the end-to-end wiring is exactly the risk:
`cmd/graft` is the one file `task ci` never executes, and a `Main` that works perfectly
under `bytes.Buffer` proves nothing about a `main()` that forgot to pass `os.Stderr`. The
headline scenario is *No arguments prints help and succeeds*, chosen because it is red today
in a way `--version` is not — the placeholder already answers `--version`, and an acceptance
test that is green before any work starts is not an outer loop.

**The acceptance test lives in `./internal/cli/`, not in `./cmd/graft/`.** A test file beside
`main.go` would never run: `task ci` is `lint`, `cover`, and `build`, and `cover` runs
`go test` over `./internal/...` only. Placing it under `internal/cli` costs one `go build`
per run and is the difference between a check that runs in CI and one that does not.

**Help text is asserted by substring, never as a golden file.** Every later change adds a
command and would churn a golden, and a churning golden is a golden nobody reads. The
assertions are: the text names `graft`, carries a `Usage:` section, and lists `--version`
and `--help`. The one byte-exact help assertion is a comparison of `graft --help` against
`graft` with no arguments — a relation between two outputs, which does not churn.

Deliberately **not** asserted: that the help text omits `completion`. Cobra leaves the
completion command out of root help while the root has no other subcommands, so that
assertion stays green with `DisableDefaultCmd` removed — it cannot fail, which makes it
worse than nothing beside an assertion that can. `graft completion` returning
`unknown command` is the discriminating half and is the one that is kept.

| Spec Scenario | Verification | Tier | Collaborators | Command |
|---|---|---|---|---|
| A successful invocation exits 0 | `TestMainVersion`, asserting the returned `int` is `0` and stderr is byte-empty | unit | buffers, real `buildinfo` | `go test ./internal/cli/ -run Version` |
| cmd/graft holds no decision of its own | `TestCmdGraftImports`, a Go test in `internal/cli` shelling out to `go list -f '{{join .Imports "\n"}}' ./cmd/graft` and `go list ./cmd/...`, asserting exactly `github.com/optioni/graft/internal/cli` and `os`, and exactly one package. A test rather than a hand-run command: this is the one directory CI never runs a test over, and the regression it guards against is a later change adding an import | unit | real `go` toolchain | `go test ./internal/cli/ -run CmdGraftImports` |
| The version line goes to stdout | `TestMainVersion`, asserting stdout is byte-exactly `buildinfo.Format("v1.2.0","abc1234","2026-08-23")+"\n"` and stderr is `""` | unit | buffers, real `buildinfo` | `go test ./internal/cli/ -run Version` |
| An unbuilt binary still prints a version line | `TestMainVersion` case with all three strings empty | unit | buffers, real `buildinfo` | `go test ./internal/cli/ -run Version` |
| `version` is not a command | `TestMainUnknown` case, exact stderr | unit | buffers | `go test ./internal/cli/ -run Unknown` |
| No arguments prints help and succeeds | `TestGraftBinary/no_arguments` — the compiled binary, real pipes, real exit status — plus `TestMainHelp` at the unit tier | acceptance + unit | real `go build`, real process, real pipes | `go test ./internal/cli/ -run 'Binary|Help'` |
| `--help` prints the same text as no arguments at all | `TestMainHelp`, comparing the two stdout buffers with `bytes.Equal` | unit | buffers | `go test ./internal/cli/ -run Help` |
| An unknown command names the argument | `TestMainUnknown` case, exact two-line stderr, byte-empty stdout, code `1`; also `TestGraftBinary/frobnicate` across a real process | unit + acceptance | buffers; real process | `go test ./internal/cli/ -run 'Unknown|Binary'` |
| An unknown flag is a usage error too | `TestMainUnknown` case, exact two-line stderr | unit | buffers, real cobra/pflag | `go test ./internal/cli/ -run Unknown` |
| An unknown shorthand flag is a usage error | `TestMainUnknown` case for `-v`, asserting pflag's `unknown shorthand flag: 'v' in -v` | unit | buffers, real cobra/pflag | `go test ./internal/cli/ -run Unknown` |
| Only the first unrecognised argument is named | `TestMainUnknown` case asserting `frobnicate` is present and `wibble` absent | unit | buffers | `go test ./internal/cli/ -run Unknown` |
| No completion command is offered | `TestMainUnknown` case for `completion`. Mutation-checked: with `DisableDefaultCmd` removed, `graft completion` exits `0` and prints completion help, so the assertion is discriminating | unit | buffers, real cobra | `go test ./internal/cli/ -run Unknown` |
| Cobra's hidden completion protocol is refused too | `TestMainUnknown` cases for `__complete` and `__completeNoDesc`, asserting stdout is byte-empty — cobra writes `:0` there when the refusal is absent | unit | buffers, real cobra | `go test ./internal/cli/ -run Unknown` |
| A command that returns an error exits 1 | `TestMainCommandError`, in-package, registering a test-only command returning the real `source` wording, asserting no hint line follows | unit | buffers, test-only command | `go test ./internal/cli/ -run CommandError` |
| A command that succeeds exits 0 | `TestMainCommandOutput`, in-package, the test-only command writing through `ui.Print` | unit | buffers, test-only command | `go test ./internal/cli/ -run CommandOutput` |
| Output that cannot be written is not a silent success | `TestMainWriteFailure/version`, stdout a writer failing every write with `disk full`, asserting code `1` and `graft: cannot write output: disk full` on stderr | unit | failing writer | `go test ./internal/cli/ -run WriteFailure` |
| Help that cannot be written is not a silent success either | `TestMainWriteFailure/help` and `/no arguments`, same failing writer. Without the recording writers of D2 both exit `0` with nothing on either stream, which is what makes this row load-bearing | unit | failing writer, real cobra | `go test ./internal/cli/ -run WriteFailure` |
| A note leaves stdout untouched | `TestNote`, asserting stdout is byte-empty | unit | buffers | `go test ./internal/ui/ -run Note` |
| An error report leaves stdout untouched | `TestFail` case asserting stdout `""` | unit | buffers | `go test ./internal/ui/ -run Fail` |
| Machine-readable output goes to stdout only | `TestPrint` | unit | buffers | `go test ./internal/ui/ -run Print` |
| A failure-mode-table message is reported verbatim after the prefix | `TestFail` case using the exact `catalog.Expand` wording | unit | buffers | `go test ./internal/ui/ -run Fail` |
| A nil error reports nothing | `TestFail` nil case, both buffers `""` | unit | buffers | `go test ./internal/ui/ -run Fail` |
| A usage error carries a hint on its own line | `TestMainUnknown`, asserting exactly two stderr lines and that the second has no `graft: ` prefix | unit | buffers | `go test ./internal/cli/ -run Unknown` |
| A terminal with NO_COLOR unset gets colour | `TestColorEnabled` table case, plus `TestStyle` asserting the escape sequence | unit | none | `go test ./internal/ui/ -run 'ColorEnabled|Style'` |
| NO_COLOR set to any non-empty value drops colour | `TestColorEnabled` cases `1`, `0`, `false` — the last two are what a truthiness check gets wrong | unit | none | `go test ./internal/ui/ -run ColorEnabled` |
| NO_COLOR set to the empty string does not drop colour | `TestColorEnabled` case, which is by construction the same input as the unset case | unit | none | `go test ./internal/ui/ -run ColorEnabled` |
| The colour decision follows stdout and never stderr | `TestMainColorFollowsStdout`: the `IsTerminal` stub records every writer it is asked about, and the test asserts it was asked about `Options.Stdout` and never about `Options.Stderr` — an identity assertion, which goes red if the wiring is inverted | unit | stub `IsTerminal` | `go test ./internal/cli/ -run Color` |
| A redirected stdout drops colour | `TestColorEnabled` case | unit | none | `go test ./internal/ui/ -run ColorEnabled` |
| A character device that is not a terminal is not a terminal | `TestIsTerminal` against a `bytes.Buffer`, both ends of a real `os.Pipe`, and a real `*os.File` opened at `/dev/null` | unit | real filesystem, real `x/term` | `go test ./internal/ui/ -run IsTerminal` |
| Styling with colour off is byte-identical to the input | `TestStyle` asserting `==` against the input for both helpers, and asserting the enabled direction too so an identity function cannot pass | unit | none | `go test ./internal/ui/ -run Style` |

## Decisions

**D1 — Two packages, not one.** `ui` is *how graft speaks*; `cli` is *what graft does with
its arguments*. Splitting them keeps the colour decision, the stream split, and the error
format testable without constructing a cobra command, and it gives `sync-command` a
dependency it can take without dragging in the root command. The alternative — one
`internal/cli` holding both — was rejected because a package that both parses flags and
formats bytes has no seam where the report format can be tested on its own, and the report
is the part `sync-command`, `update-command`, and `list-command` all extend.

Rejected also: putting the colour decision in `internal/term`. It would be a package holding
two functions, one of which is one line.

**D2 — Cobra runs with `SilenceErrors` and `SilenceUsage`, and every stream it is given is
one `ui` records.** By default cobra writes `Error: <err>` followed by the whole usage block
on any failure, which is a second error format inside a tool whose error format is the
contract. Both are silenced, `Execute` returns the error, and `Main` reports it through `ui`.

`SetOut`/`SetErr` are given `u.Out()` and `u.Err()` rather than the process's real streams.
That is not tidiness: **`cobra.Command.Help()` returns `nil` unconditionally and discards
its renderer's write error**, so a `graft --help` whose stdout is a full disk would exit `0`
having printed nothing anywhere — the exact silent success this change's exit-code
requirement forbids, on the two most common invocations. Routing cobra's writes through
`ui`'s recorders makes the failure visible to `WriteError()` without graft having to
reimplement help rendering. It also means no cobra path can reach the real `os.Stdout` from
inside a test.

**D3 — An unrecognised argument is refused by the root's own `Args` validator, not by
matching cobra's message.** The root sets

```go
Args: func(_ *cobra.Command, args []string) error {
	if len(args) > 0 { return usagef("unknown command %q", args[0]) }
	return nil
},
```

so the wording is graft's and is asserted by a graft test. The alternative — letting cobra
produce `unknown command "frobnicate" for "graft"` and rewriting it — means either shipping
cobra's phrasing (it names the parent command, which is noise at the root) or string-matching
a dependency's message, which breaks silently on an upgrade. The validator is consulted only
when no subcommand matched, so it keeps working unchanged once `sync` is registered.

`args[0]` is the first unrecognised argument specifically: naming all of them invites the
reader to fix the last one first.

**D4 — A usage error is a distinct type carrying the same exit code.** `usagef` returns an
error wrapped in an unexported `usageError`; `Main` detects it with `errors.As` and follows
the report with `run "graft --help" for usage`. `SetFlagErrorFunc` wraps pflag's errors in
the same type, so an unknown flag and an unknown command are one class. The exit code stays
`1` — SPEC.md admits two codes, and a separate usage code is the kind of contract a script
starts depending on the moment it exists.

The hint is one line rather than the full usage block: the usage block for a tool with six
commands is thirty lines, and it buries the sentence that says what went wrong.

**D5 — `--version` is graft's own bool flag handled in `RunE`, not cobra's `Version`
field.** Cobra's built-in version support writes through a template to `OutOrStdout`,
bypassing `ui` — which would mean the one piece of machine-readable output this change ships
does not go through the package that owns machine-readable output, and the write-failure
scenario would test cobra's error propagation instead of graft's. A declared
`--version` bool flag appears in help exactly the same way, and the handler is three lines.

No `-v` shorthand: `-v` means verbose in most tools, and a future `--verbose` should not have
to fight for it.

**D6 — Cobra's completion surface is removed, and that takes two lines rather than one.**
SPEC.md's command table is the contract, and it lists six commands. `graft completion` would
be a seventh, undocumented, that a user's shell configuration could come to depend on before
anyone decided it was supported. `CompletionOptions.DisableDefaultCmd = true` removes it,
and a scenario asserts it is gone rather than trusting the field name.

**The flag does not remove the hidden `__complete` command.** Cobra registers
`initCompleteCmd` on every `Execute`, independently of `DisableDefaultCmd`; with the flag
set, `graft __complete ''` still exits `0` and writes `:0` to stdout — bypassing `ui`,
bypassing the argument validator, and producing exactly the undocumented surface the flag
exists to prevent. `Main` therefore refuses `__complete` and `__completeNoDesc` as
unrecognised arguments before `Execute` is reached. This is a first-argument check against
two literal names, not a prefix rule, so it cannot swallow a future command. Enabling
completion later means deleting both lines together, which is the point of them being
adjacent.

**D7 — One colour decision, taken from stdout, exactly as SPEC.md words it.** SPEC.md says
"Color is dropped when stdout is not a TTY or `NO_COLOR` is set." Read literally, that drops
colour on **stderr** too when stdout is redirected, which is where all of graft's coloured
output actually goes. That reading was kept deliberately over the obvious refinement of
asking each stream about itself:

- One decision cannot make two streams disagree, and a run whose two streams are merged
  (`graft sync > log 2>&1`) is the common case where per-stream detection produces escape
  sequences in a file.
- The literal rule fails safe. Its cost is a colourless terminal for
  `graft sync > items.json`; the refinement's cost is escape sequences in a captured log.
- It is the documented contract. Changing it is a SPEC.md edit, which is a decision to argue
  on its own rather than a side effect of implementing it.

`NO_COLOR` follows the published convention — present and non-empty disables, present and
empty does not — so `NO_COLOR=` behaves as an unset variable, which is what a user who
cleared it means.

**D8 — Terminal detection uses `golang.org/x/term`, not an `os.Stat` mode check.** The
dependency-free idiom is `fi.Mode()&os.ModeCharDevice != 0`, and it is wrong in the exact
case the colour rule exists for: `/dev/null` is a character device, so
`graft sync > /dev/null` would be reported as a terminal and emit colour. `term.IsTerminal`
asks the kernel (`tcgetattr`), which is the actual question. The alternatives were a
hand-rolled `ioctl` — two build-tagged files and `unsafe` in a repository that has neither —
or accepting the false positive, which would make one of this change's own scenarios a lie.
`golang.org/x/term` is the Go team's module, is one package, and brings only
`golang.org/x/sys`.

`IsTerminal` takes an `io.Writer` and type-asserts to `*os.File`, returning false for
anything else. An interface with an `Fd()` method was rejected: a type that reports a file
descriptor it does not own is a lie the production path would believe.

**D9 — The environment and the terminal test reach `cli` through `Options`, never through a
global.** `Options.Getenv` and `Options.IsTerminal` default to `os.Getenv` and
`ui.IsTerminal` when nil, and every unit test passes stubs. This is `source`'s `getenv`/`home`
seam (D7 there) applied one layer up, and for the same reason: `t.Setenv` forbids
`t.Parallel`, and a test that has to set a real environment variable is one edit away from
depending on the developer's shell.

**D10 — `Main` returns an `int` and never calls `os.Exit`.** `os.Exit` skips deferred
functions and cannot be observed from a test in the same process. Returning the code makes
every exit-status scenario a unit test over a returned value, and leaves `cmd/graft` with
exactly one thing to get wrong.

**D11 — Write failures are recorded, not returned.** Threading an `error` back from every
`Print` call would put error handling into every future report line for a failure mode that
is almost always fatal anyway. `UI` wraps each stream in a recorder that keeps the first
write error, `Main` checks `WriteError()` after `Execute` returns, and a failed write becomes
exit `1` with `graft: cannot write output: <err>` on stderr. This is `bufio.Writer`'s shape,
and — because cobra writes through those same recorders (D2) — it is why "reported success
while the output went nowhere" cannot happen on any path, help included.

A failed write to **stderr** fails the run too, with the same message. The wording says
"output" for both rather than distinguishing them: a user whose error stream is broken
cannot read a more precise message anyway, and the exit code is the part that reaches them.

**D12 — Styling is exactly `Bold` and `Dim`, and both are identity functions when colour is
off.** Two helpers are the minimum that makes "colour is dropped" an observable behavior
rather than a boolean nobody reads; without any styled output at all, the `NO_COLOR`
requirement would be a field with no consumer, which is the "test that cannot fail" trap.
They are also the two the report format in SPEC.md actually needs — a bold item verb, a
dimmed trailing note. A palette of named semantic colours was rejected as designing
`sync-command`'s report a change early.

## Risks / Trade-offs

**[Help and usage text is cobra's, not graft's]** → The wording of `Usage:`, `Flags:`, and
the flag-alignment columns comes from cobra's template and can shift on a dependency
upgrade. Mitigated by asserting substrings and relations rather than a golden file, and by
pinning the two strings that *are* graft's — the error line and the hint — exactly.

**[Five modules enter `go.mod` where there were two]** → cobra is named by
`openspec/IMPLEMENTATION-ORDER.md`, so its cost is already accepted; `x/term` is argued in
D8. The indirect additions (`pflag`, `mousetrap`, `x/sys`) are cobra's and x/term's own and
are not separately chosen. Dependabot already covers Go modules, so a security advisory
surfaces as a pull request rather than as nothing.

**[The acceptance test shells out to `go build`]** → It needs a `go` toolchain, which is
trivially present since the test is itself run by `go test`, and it is cheap: measured at
~150 ms warm on this machine. The alternative is that `cmd/graft` is executed by nothing in
CI, and `main()` is precisely the code no other test can reach. The binary is built **once**,
by a single parent test that owns the `t.TempDir()` it lands in and runs every case as a
subtest — not by a package-level `sync.Once`, which would hand later tests a path inside a
`t.TempDir()` the first test already removed.

**[The positive direction of real terminal detection is not asserted anywhere]** → `go test`
runs with stdout on a pipe, so no test can point the real `ui.IsTerminal` at a terminal and
see `true`. A pty was investigated and rejected on evidence: on darwin, opening `/dev/ptmx`
**succeeds** and `term.IsTerminal` on the resulting master returns **false**, and stays
false through `TIOCPTYGRANT`/`TIOCPTYUNLK` — it only answers true once the slave side
(`/dev/ttysNNN`, reached through `TIOCPTYGNAME`) is opened too. So a "best-effort, skip when
unavailable" test would not skip: it would fail permanently on this machine and on the
`macos-latest` CI leg, and getting it to pass needs either darwin ioctl magic numbers or
`github.com/creack/pty` — a third dependency, for one assertion, contradicting D8's own
argument about what a dependency has to earn.

What stands instead is stated rather than hidden: the negative direction is asserted against
four real objects, including the `/dev/null` case that motivates D8 and that a mode-bit
check gets wrong; the decision that consumes it is a pure function tested in both
directions; and the wiring between them is asserted by identity — the stub records which
writer it was asked about. The single unasserted step is `term.IsTerminal` returning `true`,
which is `golang.org/x/term`'s own contract and its own test suite's job.

**[SPEC.md's colour rule surprises someone]** → `graft sync > items.json` in a terminal
prints no colour. Accepted in D7, and it is the documented behavior rather than a bug: the
alternative writes escape sequences into files.

## Migration Plan

None. No stored state, no format change, no deploy order, no rollback step. The change is
observable only as a different set of messages from a binary that has never been released.
`task build` and `.goreleaser.yaml` already pass the three `-X main.<var>` flags this change
keeps reading, and neither is edited.

## Open Questions

**Q1 — Where does a subprocess timeout belong?** `git-fetch`'s planning review deferred one
to "the layer that has a user to tell", meaning this change. It is not taken here, because a
timeout policy needs an operation to bound and there is none: no command runs `git`, and a
number chosen with no caller is a number nobody can defend. Nothing in `internal/source`
has to change when it arrives — `exec.CommandContext` is a one-line substitution at the
single site that starts a subprocess — and the surface it needs (a `context.Context` reaching
a command's `RunE`) is `cmd.Context()`, which cobra provides without any work here.
**Resolution point: `sync-command`**, the first change with a fetch to bound.

**Q2 — Should the command layer explain the rules behind another package's error?**
`git-fetch` recorded that `rev "47f73fc" not found` describes the outcome without explaining
that an abbreviated sha is not an accepted rev. This change deliberately adds no wrapping:
`ui.Fail` passes the message through, and every message SPEC.md's failure-mode table names
already locates its own problem. Improving that particular sentence means changing
`internal/source`'s error string, which is an archived capability's contract and needs its
own delta spec — not an incidental edit made from a different change. **Resolution point:
`sync-command`**, where that message first reaches a real user and the cost of the wording is
observable.

**Q3 — Should shell completion be offered?** Disabled in D6 because SPEC.md's command table
is the contract. It becomes worth revisiting once the table is complete and stable — enabling
it is one line, and the argument for it gets stronger as the number of commands and selectors
a user has to type grows. **Resolution point: after `add-command`**, when the surface a
completion script would describe stops changing.

**Q4 — Should colour be decided per stream?** Rejected in D7 in favour of SPEC.md's literal
rule. Recorded because the first user report of "why is my terminal output not coloured when
I redirect stdout" is the evidence that would settle it, and settling it means editing
SPEC.md's Output section, not just the code. **Resolution point: a user report, or the first
change that gives stderr enough coloured output for the loss to be felt.**
