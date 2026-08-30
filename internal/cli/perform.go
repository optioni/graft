package cli

import (
	"fmt"
	"os"

	"github.com/optioni/graft/internal/source"
	"github.com/optioni/graft/internal/sync"
	"github.com/optioni/graft/internal/ui"
)

// perform is the tail `sync` and `update` share: the working directory, the default cache
// root, the run, and the report's lines on the error stream.
//
// It exists so neither command holds a decision of its own. `sync` and `update` differ in
// exactly one field of sync.Options — the one internal/sync was given so that re-resolution
// could be a parameter rather than a second sequence — and a second copy of this tail is a
// second place for the report to reach the wrong stream.
//
// Every error is returned exactly as the package that raised it worded it. SPEC.md's
// failure-mode table is written as those messages, and a layer of context here would say the
// same thing twice.
func perform(u *ui.UI, o sync.Options) error {
	root, err := workingDirectory()
	if err != nil {
		return err
	}
	cacheRoot, err := source.DefaultCacheRoot()
	if err != nil {
		return err
	}
	o.Root, o.CacheRoot = root, cacheRoot

	report, err := sync.Run(o)
	if err != nil {
		return err
	}
	// The report is a summary a human reads, so it goes to the error stream and stdout stays
	// byte-empty for whatever a script is piping.
	for _, line := range report.Lines(u) {
		u.Note(line)
	}
	return nil
}

// workingDirectory is the repository graft runs in. Every command starts here, so the
// message is worded once: three commands saying the same failure three ways is three
// contracts to keep.
func workingDirectory() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("cannot determine the working directory: %w", err)
	}
	return root, nil
}
