package cli

import (
	"github.com/spf13/cobra"

	"github.com/optioni/graft/internal/sync"
	"github.com/optioni/graft/internal/ui"
)

// newSync builds the `graft sync` command.
//
// It is wiring and nothing else: the working directory and the default cache root become
// sync.Options, the report's lines go to the error stream, and every error is returned
// exactly as internal/sync worded it. Every decision the command could make is one another
// package already made, which is what keeps this file out of the way of the coverage gate's
// blind spot in cmd/graft.
func newSync(u *ui.UI) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Make the tree match graft.lock",
		Long: `Fetch each source at the sha graft.lock records, write the files its items
place, delete the files the lock claims that the new resolution no longer
produces, and rewrite graft.lock.

sync never re-resolves a pin: it installs what the lock says, so rev = "main"
cannot drift between syncs. Moving a pin is ` + "`graft update`" + `.`,

		// graft's own wording rather than cobra.NoArgs, which would say
		// `unknown command "x" for "graft sync"` — a second error format inside a tool
		// whose error format is the contract.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown argument %q", args[0])
			}
			return nil
		},

		// The zero Update is what makes this a sync: no pin is re-resolved. `update` is the
		// same tail with that field set, which is the whole difference between the two.
		RunE: func(_ *cobra.Command, _ []string) error {
			return perform(u, sync.Options{DryRun: dryRun})
		},
	}

	// The only flag. There is no --force and no --frozen: sync always overwrites and sync
	// is always frozen, and a flag that made it do its job would be the bug it prevents.
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and change nothing")

	return cmd
}

// newHelpCommand replaces cobra's built-in help command with one that refuses.
//
// Registering any subcommand makes cobra install `help`, and SPEC.md's command table names
// none — the same trade `--version` already makes, where one spelling is enough, and the
// same trade the completion command gets.
//
// The refusal has to be this command's own RunE rather than a guard before Execute, and the
// command cannot simply be suppressed: cobra adds whatever it is given to the root, so a
// placeholder named anything would become a working, undocumented command under that name.
// Hidden keeps it out of the help listing; the RunE is what makes `graft help` behave like
// every other unrecognised argument.
func newHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "help",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Named literally, not from args: `graft help sync` names the first
			// unrecognised argument, which is `help`.
			return usagef("unknown command %q", "help")
		},
	}
}
