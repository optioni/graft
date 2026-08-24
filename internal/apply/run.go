package apply

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"

	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/plan"
)

// File and directory modes. Both are graft's rather than the source's: graft executes
// nothing a source provides, and the cheapest way to keep that from eroding is that a
// source cannot produce an executable file in a consumer's repository at all.
const (
	fileMode = 0o644
	dirMode  = 0o755
)

// Run performs a plan's file operations against the repository at root. trees maps a
// source name to the path of that source's fetched tree.
//
// The order is SPEC.md's resolution step 8: write the planned files, delete the prune set,
// remove the directories the prune set left empty, write graft.lock. Nothing here derives
// a path of its own — every one comes from the plan.
func Run(root string, trees map[string]string, p *plan.Plan) error {
	repo, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("cannot open the repository root %q: %w", root, err)
	}
	defer func() { _ = repo.Close() }()

	src := &sources{dirs: trees, open: map[string]*os.Root{}}
	defer src.close()

	for _, w := range p.Writes {
		data, err := src.read(w.Source, w.From)
		if err != nil {
			return err
		}
		if err := writeFile(repo, w.Dest, data); err != nil {
			return err
		}
	}

	return writeLock(repo, p.Lock)
}

// writeFile puts data at dest, creating the parent directories it needs.
//
// An existing destination is removed and recreated rather than truncated. The permission
// argument to a create-and-truncate open applies only when the file is created, so a
// destination someone once made executable would keep the bit while graft replaced its
// contents with source-controlled bytes — the one hole in "a source cannot produce an
// executable file here". O_EXCL on the create is what makes the removal load-bearing
// rather than decorative.
//
// Removing is only safe because the destination has been confirmed to be a regular file.
// An empty directory would be removed outright, and a symlink would be removed while its
// target stayed — a deletion of something graft never wrote.
func writeFile(repo *os.Root, dest string, data []byte) error {
	if err := checkDestination(repo, dest); err != nil {
		return err
	}
	if dir := path.Dir(dest); dir != "." {
		if err := repo.MkdirAll(dir, dirMode); err != nil {
			return writeErrf(dest, "%v", err)
		}
	}
	if err := repo.Remove(dest); err != nil && !isNotExist(err) {
		return writeErrf(dest, "%v", err)
	}

	f, err := repo.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return writeErrf(dest, "%v", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return writeErrf(dest, "%v", err)
	}
	if err := f.Close(); err != nil {
		return writeErrf(dest, "%v", err)
	}
	return nil
}

// checkDestination refuses a destination graft did not put there. It looks without
// following: an os.Root stops a path leaving the repository but happily follows a symlink
// that stays inside it, so a repo-owned link at a destination would redirect the write to
// a path no lock claims and no prune could ever reach.
func checkDestination(repo *os.Root, dest string) error {
	fi, err := repo.Lstat(dest)
	switch {
	case isNotExist(err):
		return nil
	case err != nil:
		return writeErrf(dest, "%v", err)
	case !fi.Mode().IsRegular():
		return writeErrf(dest, "it exists and is not a regular file")
	}
	return nil
}

// writeLock serializes the plan's lock into the repository. It is called last, after every
// file operation has succeeded, so a run that failed partway leaves the previous lock —
// describing the previous state rather than one that never existed.
func writeLock(repo *os.Root, l *lock.Lock) error {
	if err := writeFile(repo, lock.Filename, lock.Marshal(l)); err != nil {
		return err
	}
	return nil
}

// sources holds one os.Root per fetched tree, opened on first use. Opening lazily means a
// tree registered for a source the plan never writes from is never touched.
type sources struct {
	dirs map[string]string
	open map[string]*os.Root
}

// read returns the bytes of from within the named source's fetched tree. Every read goes
// through that tree's own os.Root, so a source path cannot reach whatever sits beside the
// entry in the fetch cache.
func (s *sources) read(name, from string) ([]byte, error) {
	fail := sourceErrf(name)
	root, ok := s.open[name]
	if !ok {
		dir, registered := s.dirs[name]
		if !registered {
			return nil, fail("no fetched tree")
		}
		var err error
		if root, err = os.OpenRoot(dir); err != nil {
			return nil, fail("cannot read %q: %v", from, err)
		}
		s.open[name] = root
	}

	data, err := root.ReadFile(from)
	if err != nil {
		return nil, fail("cannot read %q: %v", from, err)
	}
	return data, nil
}

func (s *sources) close() {
	for _, root := range s.open {
		_ = root.Close()
	}
}

// writeErrf words every failure to write one destination. The path comes first because it
// is what the reader has to go and look at.
func writeErrf(dest, format string, args ...any) error {
	return fmt.Errorf("cannot write %q: %s", dest, fmt.Sprintf(format, args...))
}

// sourceErrf carries the per-source prefix every other package in graft uses, so a run
// with several sources always says which one failed.
func sourceErrf(name string) func(string, ...any) error {
	return func(format string, args ...any) error {
		return fmt.Errorf("source %q: %s", name, fmt.Sprintf(format, args...))
	}
}

func isNotExist(err error) bool {
	return err != nil && errors.Is(err, fs.ErrNotExist)
}
