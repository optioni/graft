package cli

import (
	"github.com/spf13/cobra"

	"github.com/optioni/graft/internal/sync"
	"github.com/optioni/graft/internal/ui"
)

// toFlag is the name of the flag that moves a pin. It is spelled once because the argument
// validator reads it back through Flags().Changed.
const toFlag = "to"

// newUpdate builds the `graft update` command.
//
// update is the one command allowed to move a pin, which is what lets `sync` promise it never
// re-resolves one. Everything downstream is the same code: internal/sync takes re-resolution
// as a parameter, so this file is the parameter and nothing else.
func newUpdate(u *ui.UI) *cobra.Command {
	var (
		to     string
		dryRun bool
	)

	cmd := &cobra.Command{
		Use:   "update [source]",
		Short: "Re-resolve pins to their current shas and sync",
		Long: `Re-resolve each source's rev in graft.toml to the sha it names today, then do
everything sync does with the result: fetch, write, prune, and rewrite
graft.lock. The report says what moved.

Named with a source, only that source is re-resolved; every other pin still
comes from the lock. With --to, the pin is first moved in graft.toml — the one
path in update that writes it.`,

		// graft's own wording rather than cobra.MaximumNArgs, which would say
		// `accepts at most 1 arg(s), received 2` — a second error format inside a tool whose
		// error format is the contract.
		//
		// The --to shapes are decided here rather than in internal/sync because a malformed
		// invocation earns the hint line, and usageError is this package's. Cobra parses
		// flags before it calls this, which is what makes Flags().Changed readable from it.
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return usagef("unknown argument %q", args[1])
			}
			// An empty positional is not "no source": internal/sync reads an empty Source
			// as every source, so accepting one here would turn `graft update "$SOURCE"`
			// with an unset variable into a silent update of everything — and, with --to,
			// into a run that discards the flag and exits 0 having moved no pin. The
			// sentinel is safe against a manifest, which cannot name a source "", and not
			// against a shell.
			if len(args) == 1 && args[0] == "" {
				return usagef("source name may not be empty")
			}
			if !cmd.Flags().Changed(toFlag) {
				return nil
			}
			// Changed rather than the value: "" is what an unset flag holds too, and
			// `graft update --to= shared` asks for a pin that is not a rev rather than for
			// no pin move at all.
			if len(args) == 0 {
				return usagef("--to requires a source")
			}
			if to == "" {
				return usagef("--to requires a rev")
			}
			return nil
		},

		RunE: func(_ *cobra.Command, args []string) error {
			// An empty Source means every source the manifest declares. It cannot collide
			// with a real name: manifest.Parse refuses a source whose name is empty.
			update := sync.Update{To: to}
			if len(args) == 1 {
				update.Source = args[0]
			}
			return perform(u, sync.Options{DryRun: dryRun, Update: &update})
		},
	}

	cmd.Flags().StringVar(&to, toFlag, "", "the rev to write into graft.toml for the named source")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print the plan and change nothing")

	return cmd
}
