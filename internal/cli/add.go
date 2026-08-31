package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/optioni/graft/internal/add"
	"github.com/optioni/graft/internal/itemid"
	"github.com/optioni/graft/internal/picker"
	"github.com/optioni/graft/internal/ui"
)

// The two flags `graft add` has. They are spelled once because the argument validator
// reads them back while deciding whether an invocation is well formed.
const (
	listFlag   = "list"
	noSyncFlag = "no-sync"
)

// newAdd builds the `graft add` command.
//
// It is wiring: the argument shapes SPEC.md's failure-mode table names, the two flags, and
// the call into internal/add. Every refusal here is a usage error, because the invocation
// is malformed rather than the run having failed at something — and each one is decided
// before internal/add is reached, so a mistyped selector never starts a git process.
func newAdd(u *ui.UI, o Options) *cobra.Command {
	var list, noSync bool

	cmd := &cobra.Command{
		Use:   "add <source>[@rev] [selector...]",
		Short: "Declare a source in graft.toml and sync it",
		Long: `Add a source to graft.toml with the selectors given, then do everything sync
does with the result. A source already declared is amended rather than
duplicated: the new selectors join its install list.

The source's name is its git value's last path segment. Without @rev, the pin
written is the source's highest semver tag, or its default branch when it
publishes none. --list prints what the source offers and writes nothing;
--no-sync writes graft.toml and stops.`,

		// graft's own wording rather than cobra's argument validators, which would say
		// `accepts between 1 and N arg(s)` — a second error format inside a tool whose
		// error format is the contract. Cobra parses flags before calling this, which is
		// what lets the two flags be read from it.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return usagef("add requires a source")
			}
			// An empty positional is not "no source": `graft add "$SOURCE" agent:x` with
			// an unset variable would otherwise derive a name from nothing at all.
			if args[0] == "" {
				return usagef("source may not be empty")
			}
			if _, rev, hasRev := add.SplitSource(args[0]); hasRev && rev == "" {
				// An @ with nothing after it asks for a pin it did not name, which is a
				// different mistake from asking for no pin.
				return usagef("rev may not be empty")
			}

			selectors := args[1:]
			if list {
				if len(selectors) > 0 {
					return usagef("--list takes no selectors")
				}
				if noSync {
					// --list writes nothing, so --no-sync has nothing to suppress.
					return usagef("--list and --no-sync cannot be combined")
				}
				return nil
			}
			if len(selectors) == 0 && !o.Interactive() {
				// Off a terminal there is nobody to ask, and graft never guesses a set of
				// items to install. On one, the picker asks.
				return usagef("add requires at least one selector, or --list to see what the source offers")
			}
			for _, sel := range selectors {
				// internal/manifest's predicate, so a selector the command accepts is one
				// the manifest it writes accepts too.
				if !itemid.Valid(sel) {
					return usagef("invalid selector %q: want kind:name", sel)
				}
			}
			return nil
		},

		RunE: func(_ *cobra.Command, args []string) error {
			root, cacheRoot, err := roots()
			if err != nil {
				return err
			}
			git, rev, _ := add.SplitSource(args[0])
			req := add.Request{
				Root:      root,
				CacheRoot: cacheRoot,
				Git:       git,
				Rev:       rev,
				Install:   args[1:],
				NoSync:    noSync,
			}
			if len(req.Install) == 0 {
				// Reached only when the validator above found a terminal to draw on.
				req.Choose = pick(u, o)
			}

			if list {
				// A listing is the content the caller asked for, so it goes to standard
				// output — the stream `--version` and `graft list` already use.
				lines, err := add.List(req)
				if err != nil {
					return err
				}
				for _, line := range lines {
					u.Print(line)
				}
				return nil
			}

			report, err := add.Run(req)
			if err != nil {
				return err
			}
			// The report is a summary a human reads, so it goes to the error stream and
			// stdout stays byte-empty for whatever a script is piping.
			for _, line := range report.Lines(u) {
				u.Note(line)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&list, listFlag, false, "print the source's catalog and exit, writing nothing")
	cmd.Flags().BoolVar(&noSync, noSyncFlag, false, "write graft.toml and stop, syncing nothing")

	return cmd
}

// pick is the chooser `add` hands to internal/add: the picker, drawn on the error stream,
// reading keys from the process's own input.
//
// The error stream rather than standard output, because a picker is interaction and not
// content a caller asked for — stdout stays byte-empty for a syncing add, picker or no
// picker.
func pick(u *ui.UI, o Options) func(string, []picker.Item) ([]string, error) {
	return func(title string, items []picker.Item) ([]string, error) {
		m := picker.New(items).WithTitle(title).WithHeight(listHeight(o.Stderr))
		return picker.Run(picker.Terminal{In: o.Stdin, Out: u.Err(), MakeRaw: o.MakeRaw}, m)
	}
}

// listHeight is how many item lines the picker may use: the terminal's height less the
// lines the view spends on its title, its help, and the room a shell prompt wants back. A
// stream whose size cannot be read returns 0, which leaves the picker's own default.
func listHeight(w io.Writer) int {
	f, ok := w.(*os.File)
	if !ok {
		return 0
	}
	_, rows, err := term.GetSize(int(f.Fd()))
	if err != nil {
		return 0
	}
	return rows - reservedRows
}

// reservedRows is what the view spends around the list: a title and a blank line, a blank
// line and the help line, and one row left for the cursor to sit on.
const reservedRows = 5
