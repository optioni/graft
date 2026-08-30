package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/optioni/graft/internal/buildinfo"
	"github.com/optioni/graft/internal/ui"
)

// Options is everything the command surface reads from its process.
//
// Nothing here is read from a global. The streams, the environment, and the terminal test
// all arrive as values, which is what lets every behavior below be asserted without
// touching os.Args, os.Stdout, or the developer's shell.
type Options struct {
	// Args is the argument list without the program name — os.Args[1:].
	Args []string

	// Stdout carries machine-readable output; Stderr carries everything else. Both are
	// required — there is no nil default, because a run whose output silently went nowhere
	// is the outcome this package exists to make impossible.
	Stdout io.Writer
	Stderr io.Writer

	// Getenv reads the environment. A nil value means os.Getenv.
	Getenv func(string) string

	// IsTerminal reports whether a stream is a terminal. A nil value means ui.IsTerminal.
	IsTerminal func(io.Writer) bool

	// Version, Commit, and Date are the strings the linker injects into cmd/graft.
	Version string
	Commit  string
	Date    string
}

// Main runs graft and returns the process exit code: 0 on success, 1 on any error.
//
// It never calls os.Exit. Returning the code is what makes every exit-status behavior a
// unit test over a value, and leaves cmd/graft with exactly one thing to get wrong.
func Main(o Options) int {
	getenv := o.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	isTerminal := o.IsTerminal
	if isTerminal == nil {
		isTerminal = ui.IsTerminal
	}

	// The colour decision is taken from stdout, and from stdout only — SPEC.md's rule, kept
	// literally, so the two streams cannot disagree about it.
	u := ui.New(o.Stdout, o.Stderr, ui.ColorEnabled(getenv("NO_COLOR"), isTerminal(o.Stdout)))

	// Refused before Execute, because cobra registers these two on every run regardless of
	// CompletionOptions.DisableDefaultCmd, and they never reach the argument validator.
	// Paired with DisableDefaultCmd in newRoot: enabling completion means removing both.
	if len(o.Args) > 0 && (o.Args[0] == completeCmd || o.Args[0] == completeNoDescCmd) {
		return report(u, usagef("unknown command %q", o.Args[0]))
	}

	root := newRoot(u, o)

	// Never nil: cobra falls back to os.Args[1:] whenever the slice it was given is nil,
	// which would let the process's own arguments reach a caller that passed none.
	args := o.Args
	if args == nil {
		args = []string{}
	}
	root.SetArgs(args)

	return execute(u, root)
}

// execute runs a prepared root and turns its outcome into an exit code.
func execute(u *ui.UI, root *cobra.Command) int {
	err := root.Execute()
	if err == nil {
		// Checked only when the command itself succeeded: an error from the command is the
		// more useful thing to report, and a write failure usually caused it anyway.
		if werr := u.WriteError(); werr != nil {
			err = fmt.Errorf("cannot write output: %w", werr)
		}
	}
	return report(u, err)
}

// report renders an outcome and returns the exit code for it. There is no third code:
// SPEC.md admits 0 for success and 1 for error, and a code invented for a class of failure
// is a contract a script starts depending on the moment it exists.
func report(u *ui.UI, err error) int {
	if err == nil {
		return 0
	}
	u.Fail(err)

	// One line rather than the whole usage block: the block for a tool with six commands
	// buries the sentence that says what went wrong.
	var usage usageError
	if errors.As(err, &usage) {
		u.Note(hintLine)
	}
	return 1
}

// newRoot builds graft's root command. It is separate from Main so a test can register a
// command against a real root without going through the process-shaped Options.
func newRoot(u *ui.UI, o Options) *cobra.Command {
	var showVersion bool

	root := &cobra.Command{
		Use:   "graft",
		Short: "Vendor files from a source git repository into this one",
		Long: `graft vendors files from a source git repository into the repository it runs
in — agent definitions, schemas, skills — pinned by graft.lock and committed.

Sources and the items to install are declared in graft.toml.`,

		// graft owns every byte on stderr. Left to itself cobra prints "Error: <err>"
		// followed by the whole usage block, which is a second error format inside a tool
		// whose error format is the contract.
		SilenceErrors: true,
		SilenceUsage:  true,

		// graft's own validator, not a match against cobra's message text: the wording is
		// graft's contract, and cobra names the parent command in a way that is noise at the
		// root. It is consulted only when no subcommand matched, so it keeps working
		// unchanged once `sync` is registered.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown command %q", args[0])
			}
			return nil
		},

		RunE: func(cmd *cobra.Command, _ []string) error {
			if showVersion {
				u.Print(buildinfo.Format(o.Version, o.Commit, o.Date))
				return nil
			}
			return cmd.Help()
		},
	}

	// Cobra's version support renders through a template straight to OutOrStdout, which
	// would put the one piece of machine-readable output this change ships outside the
	// package that owns machine-readable output.
	root.Flags().BoolVar(&showVersion, "version", false, "print the version, commit, and build date")

	// SPEC.md's command table is the contract and it does not list a completion command.
	// This removes the visible one; Main refuses the hidden __complete protocol, which cobra
	// registers regardless of this field. The two belong together.
	root.CompletionOptions.DisableDefaultCmd = true

	// The UI's recording writers, not the process's streams: cobra.Command.Help() returns
	// nil unconditionally and discards its renderer's write error, so a raw stdout would
	// let `graft --help` onto a full disk exit 0 having printed nothing anywhere.
	root.SetOut(u.Out())
	root.SetErr(u.Err())

	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })

	// SPEC.md's command table is the contract. `sync` is on it; `help` is not, and cobra
	// installs one the moment a subcommand exists — see newHelpCommand.
	root.AddCommand(newSync(u))
	root.AddCommand(newUpdate(u))
	root.AddCommand(newList(u))
	root.SetHelpCommand(newHelpCommand())

	return root
}

// The names of cobra's hidden completion protocol commands. They are matched literally
// rather than by a "__" prefix, so a future command cannot be swallowed by the rule.
const (
	completeCmd       = "__complete"
	completeNoDescCmd = "__completeNoDesc"
)

// hintLine follows a usage error. It is not a second failure and carries no "graft: ".
const hintLine = `run "graft --help" for usage`

// usageError marks the class of failure that earns the hint line: graft was asked for
// something it does not have, rather than failing at something it does.
type usageError struct{ err error }

func (e usageError) Error() string { return e.err.Error() }
func (e usageError) Unwrap() error { return e.err }

// usagef is the single constructor for the class. Nothing compares an error's text to
// decide whether it is a usage error.
func usagef(format string, a ...any) error {
	return usageError{fmt.Errorf(format, a...)}
}
