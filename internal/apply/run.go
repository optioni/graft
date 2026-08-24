package apply

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"

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

	if err := preflight(repo, src, p); err != nil {
		return err
	}

	for _, w := range p.Writes {
		data, err := src.read(w.Source, w.From)
		if err != nil {
			return err
		}
		if err := writeFile(repo, w.Dest, data); err != nil {
			return err
		}
	}

	for _, dest := range p.Prune {
		if err := removeFile(repo, dest); err != nil {
			return err
		}
	}

	removeEmptyDirs(repo, p.Prune)

	return writeLock(repo, p.Lock)
}

// removeEmptyDirs removes the directories the prune set left empty, deepest first.
//
// Only the ancestry of a pruned path is ever a candidate. A walk of the tree looking for
// empty directories would be the same mistake as scanning a destination directory for
// files to prune: a directory that was already empty before the sync was not emptied by
// graft, and removing it would be graft deleting something it has no record of.
//
// A candidate is examined without following it and removed only if it is a directory.
// "A non-empty directory fails harmlessly" is true of directories and false of symlinks —
// unlinking one succeeds however full its target is — so a bare Remove would delete a
// user's vendor -> shared convenience link, a path absent from graft.lock.
//
// Every failure is ignored, including the ordinary one of a directory that still holds
// something. This runs after the prunes and before the lock is written, so failing here
// would strand the sync in the state it is least able to explain, over a tidying step.
func removeEmptyDirs(repo *os.Root, prune []string) {
	seen := map[string]struct{}{}
	var candidates []string
	for _, dest := range prune {
		for _, dir := range ancestors(dest) {
			if _, dup := seen[dir]; dup {
				continue
			}
			seen[dir] = struct{}{}
			candidates = append(candidates, dir)
		}
	}
	// Descending order tries a child before its parent: a parent is a prefix of its
	// child, so it sorts first ascending. The repository root is not among them —
	// ancestors stops before ".".
	slices.Sort(candidates)
	slices.Reverse(candidates)

	for _, dir := range candidates {
		fi, err := repo.Lstat(dir)
		if err != nil || !fi.IsDir() {
			continue
		}
		_ = repo.Remove(dir)
	}
}

// removeFile deletes one pruned path, or refuses it.
//
// The only deletion this package performs is a Remove on a path it has just confirmed to
// be a regular file whose ancestry is all directories. There is no RemoveAll anywhere, and
// there is no directory listing: the prune set handed in is the only source of deletions,
// which is what makes a file absent from graft.lock invisible here rather than merely
// spared.
//
// A path that does not exist is skipped without complaint. The lock still claims a file a
// user deleted by hand, so it is still in the prune set, and there is nothing to do about
// that.
func removeFile(repo *os.Root, dest string) error {
	if err := checkPrune(repo, dest); err != nil {
		return err
	}
	if err := repo.Remove(dest); err != nil && !isNotExist(err) {
		return removeErrf(dest, "%v", err)
	}
	return nil
}

// checkPrune refuses a prune path graft could not have written. A directory, a symlink, or
// a device at a path the lock claims means the tree is not what the lock says it is:
// removing a link would succeed however full its target is, and removing a directory tree
// is the one mistake this design exists to prevent.
//
// A path that does not exist is not a refusal — it is a file the user deleted by hand, and
// there is nothing to do about that.
func checkPrune(repo *os.Root, dest string) error {
	if bad, err := badAncestor(repo, dest); err != nil {
		return removeErrf(dest, "%v", err)
	} else if bad != "" {
		return removeErrf(dest, "%q is not a directory", bad)
	}

	fi, err := repo.Lstat(dest)
	switch {
	case isNotExist(err):
		return nil
	case err != nil:
		return removeErrf(dest, "%v", err)
	case !fi.Mode().IsRegular():
		return removeErrf(dest, "it is not a regular file")
	}
	return nil
}

// preflight decides every refusal in this package before the first byte is written.
//
// All of them come down to an Lstat: a reserved path, a source file that is not a regular
// file, a destination that is not one, an ancestor that is not a directory. Discovering one
// halfway through would guarantee a partial apply — and, because the lock is written last
// and so never gets written at all, the identical failure would repeat on every subsequent
// sync, leaving the user a stuck command and a tree graft cannot describe.
//
// It is not a lock on the filesystem. A condition checked here can change before the write
// that depends on it, and that case is allowed to fail mid-flight; the checks stay at their
// point of use for exactly that reason. What this pass removes is the failures graft can see
// coming, which is all a pre-flight pass can ever do.
func preflight(repo *os.Root, src *sources, p *plan.Plan) error {
	for _, w := range p.Writes {
		if err := checkReservedWrite(w.Dest); err != nil {
			return err
		}
		if err := src.check(w.Source, w.From); err != nil {
			return err
		}
		if err := checkDestination(repo, w.Dest); err != nil {
			return err
		}
	}
	for _, dest := range p.Prune {
		if err := checkReservedRemove(dest); err != nil {
			return err
		}
		if err := checkPrune(repo, dest); err != nil {
			return err
		}
	}
	return nil
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
//
// The ancestors come first, because an Lstat of the destination itself under a bad ancestor
// fails with the operating system's own "not a directory" — a message naming neither the
// destination nor the path the reader has to go and fix.
func checkDestination(repo *os.Root, dest string) error {
	if bad, err := badAncestor(repo, dest); err != nil {
		return writeErrf(dest, "%v", err)
	} else if bad != "" {
		return writeErrf(dest, "%q is not a directory", bad)
	}

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

// badAncestor returns the shallowest existing ancestor of p that is not a directory, or
// the empty string when every one of them is.
//
// This is the half of containment an os.Root does not provide. A root refuses a path that
// leaves it and follows a symlink that stays inside it, so a link in the ancestry silently
// relocates whatever is done at the end of the path — a write lands where graft.lock does
// not say, and a deletion reaches a file the lock never claimed. A symlink to a directory
// is not a directory here, which is the whole point, and Lstat is what keeps it that way.
//
// The walk stops at the first ancestor that does not exist. Everything below such an
// ancestor is created fresh by this package, so there is nothing there to be surprised by.
func badAncestor(repo *os.Root, p string) (string, error) {
	for _, dir := range ancestors(p) {
		fi, err := repo.Lstat(dir)
		switch {
		case isNotExist(err):
			return "", nil
		case err != nil:
			return "", err
		case !fi.IsDir():
			return dir, nil
		}
	}
	return "", nil
}

// ancestors lists p's parent directories, shallowest first, so the offender reported is
// always the shallowest one and a path with two of them names the same one every time.
func ancestors(p string) []string {
	var out []string
	for dir := path.Dir(p); dir != "." && dir != "/"; dir = path.Dir(dir) {
		out = append(out, dir)
	}
	slices.Reverse(out)
	return out
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

// root returns the named source's fetched tree, opening it on first use. from appears only
// in the error, which is the read the caller was about to attempt.
func (s *sources) root(name, from string) (*os.Root, error) {
	if root, ok := s.open[name]; ok {
		return root, nil
	}
	fail := sourceErrf(name)
	dir, registered := s.dirs[name]
	if !registered {
		return nil, fail("no fetched tree")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fail("cannot read %q: %v", from, err)
	}
	s.open[name] = root
	return root, nil
}

// check reports whether from names a regular file in the source's tree, without reading it
// and without following a link at its last component.
func (s *sources) check(name, from string) error {
	root, err := s.root(name, from)
	if err != nil {
		return err
	}
	fail := sourceErrf(name)
	fi, err := root.Lstat(from)
	switch {
	case err != nil:
		return fail("cannot read %q: %v", from, err)
	case !fi.Mode().IsRegular():
		return fail("cannot read %q: not a regular file", from)
	}
	return nil
}

// read returns the bytes of from within the named source's fetched tree. Every read goes
// through that tree's own os.Root, so a source path cannot reach whatever sits beside the
// entry in the fetch cache.
func (s *sources) read(name, from string) ([]byte, error) {
	root, err := s.root(name, from)
	if err != nil {
		return nil, err
	}
	data, err := root.ReadFile(from)
	if err != nil {
		return nil, sourceErrf(name)("cannot read %q: %v", from, err)
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

// removeErrf words every failure to delete one pruned path, mirroring writeErrf.
func removeErrf(dest, format string, args ...any) error {
	return fmt.Errorf("cannot remove %q: %s", dest, fmt.Sprintf(format, args...))
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
