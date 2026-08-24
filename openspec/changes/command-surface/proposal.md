## Why

Four changes have landed and none of them is reachable. `cmd/graft` still answers
`--version` and then says `not implemented yet — see SPEC.md`; every parser, planner, and
fetcher behind it is exercised only by `go test`. The next change, `sync-command`, is the
first with something to tell a user — and if it lands first, then the `graft: ` prefix, the
choice of stream, the exit code, and the colour rule are all invented inside it and
retrofitted across four more commands afterwards. SPEC.md's failure-mode table is the
product for a CLI this small; it needs a place to land before the first message is written.

## What Changes

- **New `internal/ui`** — graft's output surface. It owns the two streams (machine-readable
  output to stdout, everything else to stderr), the one-line error format `graft: <message>`,
  and the colour decision from `NO_COLOR` and terminal detection. It is the only package
  that writes to either stream.
- **New `internal/cli`** — the cobra root command, the `--version` flag, help, argument and
  flag errors, and the mapping from an error to an exit code. `Main` returns an `int`; it
  never calls `os.Exit`.
- **`cmd/graft/main.go` shrinks to flag wiring and one exit call**: it reads `os.Args`, the
  linker-injected build strings, and the two real streams, and hands them to `cli.Main`.
  This is the design constraint restated as code — coverage is measured over
  `./internal/...` only, so a decision made in `cmd/graft` is a decision no gate can see.
- **Exit codes become a contract**: `0` on success, `1` on any error, including a usage
  error. SPEC.md admits exactly two, and this change adds none.
- **Two new module dependencies**, both argued rather than assumed:
  `github.com/spf13/cobra` (the roadmap names it) and `golang.org/x/term` (terminal
  detection; an `os.Stat` mode check reports `/dev/null` as a terminal, which is precisely
  the redirection the colour rule exists for). Both are pinned at the newest stable version
  at the time of writing — cobra `v1.10.2`, x/term `v0.45.0` — checked against the module
  proxy rather than recalled.
- **The placeholder's bare `version` word goes away.** `graft --version` stays; `graft
  version` becomes an unknown command, because SPEC.md's command table does not name it.
  Not **BREAKING**: no release has been cut, and the surface it changes is a placeholder
  whose other answer is `not implemented yet`.
- Not **BREAKING** in any other sense either: no change to `graft.toml`, `graft.lock`, or
  `catalog.yaml`, and no existing package's error string is touched. `graft.lock` stays at
  `version = 1`.

## Non-Goals

- **No command that does anything.** `sync`, `update`, `add`, and `list` are later changes.
  This change registers no subcommand, and it deliberately does not register empty stubs —
  a stub that exits 0 having done nothing is worse than an unknown-command error.
- **No `internal/apply`, and no write of any kind to the working tree.** Nothing in this
  change opens a file for writing.
- **No report vocabulary.** `added`, `updated`, `removed`, the per-item lines, and
  `up to date` describe a sync's effect and belong to `sync-command`.
- **No `--dry-run`, no `--json`.** Both are per-command flags on commands that do not exist.
- **No signal handling, no timeouts, and no `context` plumbing.** `git-fetch`'s planning
  review deferred a subprocess timeout to "the layer that has a user to tell", meaning this
  one; there is still no long-running operation to bound, and a timeout policy with no
  caller is a number nobody can justify. The resolution point moves to `sync-command` and is
  recorded in design.md → Open Questions. A cancel signal would additionally want an exit
  code SPEC.md does not admit.
- **No shell completion.** Cobra offers a `completion` command for free; it is disabled,
  because SPEC.md's command table is the contract and an undocumented command widens the
  surface a consumer can come to depend on. Cobra's hidden `__complete` protocol survives
  that flag and is refused separately — verified, not assumed.
- **No logging, no verbosity flag, no telemetry.** SPEC.md forbids the last outright.
- **No rewording of any existing error.** `git-fetch` left a note that `rev "47f73fc" not
  found` explains the outcome without explaining the rule. That message is
  `internal/source`'s own contract, asserted by its tests and pinned by an archived spec;
  changing it here would be an incidental edit to another change's contract. Recorded in
  design.md → Open Questions instead.
- Nothing near the PRD's non-goals: no dependency resolution, no registry, no merge
  behavior, no auth layer, and no runtime dependency on graft.

## Capabilities

### New Capabilities
- `command-invocation`: what `graft` is as a program — the root command, `--version`, help,
  how an unknown command or flag is refused, and the exit code every outcome produces.
- `command-output`: which stream carries what, the shape of an error report, and when colour
  is emitted and when it is dropped.

### Modified Capabilities
None. `manifest-format`, `lock-format`, `catalog-format`, `selector-expansion`,
`destination-computation`, `sync-plan`, `rev-resolution`, `fetch-cache`, and
`source-listing` keep every requirement they have.

## Impact

- New packages `internal/ui` and `internal/cli`; new specs `command-invocation` and
  `command-output`. `internal/buildinfo` gains a caller and keeps its API.
- `cmd/graft/main.go` is rewritten and loses its `run` function.
- `go.mod` gains `github.com/spf13/cobra` and `golang.org/x/term` directly, and
  `github.com/spf13/pflag`, `github.com/inconshreveable/mousetrap`, and `golang.org/x/sys`
  indirectly.
- `SPEC.md` gains a short paragraph in its **Output** section pinning the error format and
  the `NO_COLOR` rule, which it states nowhere today; `AGENTS.md`'s existing coverage rule is
  **rewritten** rather than appended to, because the sharper fact is that `task ci` never
  runs `go test` outside `./internal/...` at all.
- No change to `internal/{manifest,lock,catalog,itemid,plan,source}`, to any file format, to
  `Taskfile.yml`, or to CI. Nothing outside this repository is affected.
