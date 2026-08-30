package list

import (
	"path/filepath"

	"github.com/optioni/graft/internal/lock"
)

// Options is everything a listing is computed from: one directory.
//
// There is no cache root and no network timeout because there is nothing to fetch, and no
// manifest path because list never reads graft.toml — the lock is the record of what was
// installed, and that is the question the command answers.
type Options struct {
	// Root is the repository graft runs in.
	Root string
}

// Run reads the repository's graft.lock and returns what it records.
//
// An absent lock is not an error: lock.Load returns the empty lock for one, which is
// exactly the "nothing installed" case, already decided in the right place. A repository
// that has never synced is a legitimate starting state.
//
// Every error is returned exactly as internal/lock worded it. Error strings are this
// tool's contract, each message already locates its own problem, and list adds no failure
// mode of its own to a file it only reads.
func Run(o Options) (*Listing, error) {
	l, err := lock.Load(filepath.Join(o.Root, lock.Filename))
	if err != nil {
		return nil, err
	}
	return FromLock(l), nil
}
