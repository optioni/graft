package cli

import (
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

	// Stdout carries machine-readable output; Stderr carries everything else.
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

	root := newRoot(u, o)
	root.SetArgs(o.Args)

	err := root.Execute()
	if err == nil {
		err = u.WriteError()
	}
	if err != nil {
		u.Fail(err)
		return 1
	}
	return 0
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
	root.CompletionOptions.DisableDefaultCmd = true

	// The UI's recording writers, not the process's streams: cobra.Command.Help() returns
	// nil unconditionally and discards its renderer's write error, so a raw stdout would
	// let `graft --help` onto a full disk exit 0 having printed nothing anywhere.
	root.SetOut(u.Out())
	root.SetErr(u.Err())

	return root
}
