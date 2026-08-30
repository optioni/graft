package cli

import (
	"github.com/spf13/cobra"

	"github.com/optioni/graft/internal/list"
	"github.com/optioni/graft/internal/ui"
)

// jsonFlag is the one flag `graft list` has. There is no --format and no second output
// form: one rendering for a person, one for a program, and the second is versioned so it
// can be extended without a third.
const jsonFlag = "json"

// nothingInstalledNote is what a repository with nothing installed says. It is a note about
// the absence of content rather than content, so it goes where notes go — and a caller
// piping the plain form gets zero bytes rather than a sentence that parses as an item.
const nothingInstalledNote = "nothing installed"

// newList builds the `graft list` command.
//
// It is wiring and nothing else: the working directory becomes list.Options, the document
// or the lines go to standard output, and every error is returned exactly as internal/lock
// worded it. Every string it prints comes from internal/list or from here as a constant, so
// nothing this command decides sits where the coverage gate cannot see it.
func newList(u *ui.UI) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show what is installed here, and at which sha",
		Long: `Print what graft.lock records: each source, the rev it was pinned to, its
resolved sha, and the items installed from it with a file count each.

list reads graft.lock and nothing else. It does not read graft.toml, does not
look at the working tree, resolves nothing, fetches nothing, and writes
nothing — so a repository with no lock is not an error, it is a repository
with nothing installed. Use --json for the same information as a document a
program can consume.`,

		// graft's own wording rather than cobra.NoArgs, which would say
		// `unknown command "shared" for "graft list"` — a second error format inside a tool
		// whose error format is the contract.
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usagef("unknown argument %q", args[0])
			}
			return nil
		},

		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingDirectory()
			if err != nil {
				return err
			}
			listing, err := list.Run(list.Options{Root: root})
			if err != nil {
				return err
			}

			// The document is written whole, the trailing newline internal/list already put
			// on it included. Printing it a line at a time would add a second one.
			//
			// The write error is dropped here for the same reason u.Print drops one: the
			// recorder behind the stream has kept it, and execute collects it once at the
			// end as `cannot write output`, so a failed write has one wording rather than
			// two.
			if asJSON {
				_, _ = u.Out().Write(listing.JSON())
				return nil
			}
			if listing.Empty() {
				u.Note(nothingInstalledNote)
				return nil
			}
			for _, line := range listing.Lines() {
				u.Print(line)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&asJSON, jsonFlag, false, "print the listing as a JSON document")

	return cmd
}
