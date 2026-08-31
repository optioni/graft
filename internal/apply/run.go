package apply

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"

	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/manifest"
	"github.com/optioni/graft/internal/plan"
)

// File and directory modes. Both are graft's rather than the source's: graft executes
// nothing a source provides, and the cheapest way to keep that from eroding is that a
// source cannot produce an executable file in a consumer's repository at all.
const (
	fileMode = 0o644
	dirMode  = 0o755
)

// Option adjusts what an apply does beyond performing the plan. There is exactly one, and
// the shape is variadic so that every existing call site — this package's own tests, and
// internal/sync — reads and compiles unchanged.
type Option func(*options)

type options struct {
	// manifest is the graft.toml to write, or nil to leave the file alone.
	manifest []byte
}

// WithManifest makes the apply write data to graft.toml at the repository root, immediately
// before graft.lock and only after every planned write, deletion, and directory removal has
// succeeded.
//
// It is not a hole in "every path this package touches comes from the plan". graft's own two
// files are the named exception: their paths are fixed rather than derived, and their bytes
// come from the caller rather than from a source. graft.lock has always been written this
// way; this is the same class of write, for the file that records what the consumer asked
// for rather than what graft installed. A *plan* naming either is still refused outright —
// the two are told apart by where the bytes came from, never by the path string.
func WithManifest(data []byte) Option {
	return func(o *options) { o.manifest = data }
}

// Run performs a plan's file operations against the repository at root. trees maps a
// source name to the path of that source's fetched tree.
//
// The order is SPEC.md's resolution step 8: write the planned files, delete the prune set,
// remove the directories the prune set left empty, write graft.toml when the caller supplied
// it, write graft.lock. Nothing here derives a path of its own — every one comes from the
// plan, or is one of graft's own two files at the root.
func Run(root string, trees map[string]string, p *plan.Plan, opts ...Option) ([]string, error) {
	var o options
	for _, apply := range opts {
		apply(&o)
	}

	repo, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("cannot open the repository root %q: %w", root, err)
	}
	defer func() { _ = repo.Close() }()

	src := &sources{dirs: trees, open: map[string]*os.Root{}}
	defer src.close()

	if err := preflight(repo, src, p, o); err != nil {
		return nil, err
	}

	// The identity of each file written, so the prune step can decline to delete one of
	// them. See written.
	var wrote written
	// The destinations at which existing content was replaced, in the plan's own order.
	// A run that fails returns none of these, for the same reason it writes no lock: a
	// partial account describes a state that never existed.
	var replaced []string
	for _, w := range p.Writes {
		data, err := src.read(w.Source, w.From)
		if err != nil {
			return nil, err
		}
		if replaces(repo, w, data) {
			replaced = append(replaced, w.Dest)
		}
		if err := writeFile(repo, w.Dest, data); err != nil {
			return nil, err
		}
		if fi, err := repo.Lstat(w.Dest); err == nil {
			wrote = append(wrote, fi)
		}
	}

	// Only the paths this run actually unlinked become directory-removal candidates. A
	// prune path the user had already deleted by hand removes nothing, so the directory it
	// used to live in was not emptied by graft — and removing it would be graft deleting
	// something it has no record of, which is the same mistake as scanning for files to
	// prune.
	unlinked := make([]string, 0, len(p.Prune))
	for _, dest := range p.Prune {
		gone, err := removeFile(repo, dest, wrote)
		if err != nil {
			return nil, err
		}
		if gone {
			unlinked = append(unlinked, dest)
		}
	}

	removeEmptyDirs(repo, unlinked)

	if o.manifest != nil {
		if err := writeManifest(repo, o.manifest); err != nil {
			return nil, err
		}
	}
	if err := writeLock(repo, p.Lock); err != nil {
		return nil, err
	}
	return replaced, nil
}

// replaces reports whether writing this file replaces content graft does not own: the
// destination exists, the plan did not mark it claimed by the lock, and its bytes differ
// from the bytes about to be written.
//
// All three conditions matter. A claimed destination is graft's own file being rewritten,
// which is what a sync does. A destination holding exactly what is about to be written
// replaced nothing. An absent one is an ordinary write. Reporting any of them would put a
// number in the summary that a reader learns to skip.
//
// The comparison is against bytes the caller is already holding, so it costs one read of a
// file that is about to be overwritten anyway — and none at all for a claimed path, which
// is every file of a consumer that has synced before.
//
// A read that fails answers false rather than an error. Whatever is there is about to be
// overwritten either way, and the write itself is where a real failure surfaces; refusing a
// run because a note could not be produced would put reporting ahead of the work.
func replaces(repo *os.Root, w plan.Write, data []byte) bool {
	if w.Claimed {
		return false
	}
	fi, err := repo.Lstat(w.Dest)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	existing, err := repo.ReadFile(w.Dest)
	if err != nil {
		return false
	}
	return !bytes.Equal(existing, data)
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
//
// The pre-flight pass already refuses a prune path with a non-directory ancestor, so the
// skip below is unreachable through Run today and is deliberately kept anyway. It is the
// last line between this walk and a user's `vendor -> shared` link, it costs one Lstat,
// and the pass that makes it redundant is one refactor away from not doing so.
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
// The only file deletion this package performs is a Remove on a path it has just confirmed
// to be a regular file whose ancestry is all directories; removeEmptyDirs deletes
// directories, under its own rules. There is no RemoveAll anywhere, and there is no
// directory listing: the prune set handed in is the only source of deletions, which is what
// makes a file absent from graft.lock invisible here rather than merely spared.
//
// A path that does not exist is skipped without complaint. The lock still claims a file a
// user deleted by hand, so it is still in the prune set, and there is nothing to do about
// that.
// It reports whether it unlinked anything, so a path that was already gone — or one this run
// just wrote — does not make the directory it used to live in a removal candidate.
func removeFile(repo *os.Root, dest string, wrote written) (bool, error) {
	fi, err := checkPrune(repo, dest)
	switch {
	case err != nil:
		return false, err
	case fi == nil:
		return false, nil
	case wrote.is(fi):
		// internal/plan makes the write set and the prune set a difference over path
		// strings, and on a case-sensitive filesystem that is the end of it. On APFS and
		// NTFS it is not: a source renaming Foo.md to foo.md puts one path in each set
		// naming one file, and pruning it would delete the file the write just created,
		// leaving graft.lock claiming a path that is not there. The comparison is by file
		// identity rather than by folded string, so a filesystem where the two really are
		// different files still gets both operations.
		return false, nil
	}

	if err := repo.Remove(dest); err != nil {
		if isNotExist(err) {
			return false, nil
		}
		return false, removeErrf(dest, "%v", err)
	}
	return true, nil
}

// written is the identity of every file a run wrote. It is a slice rather than a set because
// os.SameFile is the only portable way to ask the question, and a plan holds tens of files
// rather than thousands.
type written []os.FileInfo

func (w written) is(fi os.FileInfo) bool {
	for _, other := range w {
		if os.SameFile(other, fi) {
			return true
		}
	}
	return false
}

// checkPrune refuses a prune path graft could not have written. A directory, a symlink, or
// a device at a path the lock claims means the tree is not what the lock says it is:
// removing a link would succeed however full its target is, and removing a directory tree
// is the one mistake this design exists to prevent.
//
// A path that does not exist is not a refusal — it is a file the user deleted by hand, and
// there is nothing to do about that.
// It returns the path's own information, or nil when there is nothing there.
func checkPrune(repo *os.Root, dest string) (os.FileInfo, error) {
	if bad, err := badAncestor(repo, dest); err != nil {
		return nil, removeErrf(dest, "%v", err)
	} else if bad != "" {
		return nil, removeErrf(dest, "%q is not a directory", bad)
	}

	fi, err := repo.Lstat(dest)
	switch {
	case isNotExist(err):
		return nil, nil
	case err != nil:
		return nil, removeErrf(dest, "%v", err)
	case !fi.Mode().IsRegular():
		return nil, removeErrf(dest, "it is not a regular file")
	}
	return fi, nil
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
func preflight(repo *os.Root, src *sources, p *plan.Plan, o options) error {
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
		if _, err := checkPrune(repo, dest); err != nil {
			return err
		}
	}
	// graft's own two files are writes like any other and are the last ones performed, so
	// leaving them out of the pass would mean a repository where graft.lock is a directory
	// applies the whole plan and then fails at the final step — files written, files
	// deleted, and no record of either. Neither is in p.Writes, so both are named
	// explicitly. graft.toml is checked only when the caller supplied bytes for it: a run
	// that is not moving a pin never touches it, so a graft.toml that is somehow not a
	// regular file is not that run's problem to report.
	if o.manifest != nil {
		if err := checkDestination(repo, manifest.Filename); err != nil {
			return err
		}
		// The staging path is checked here for the same reason the destinations are:
		// reaching it after every planned write and every deletion means failing with the
		// tree already moved. It is also the one path this package writes that no plan and
		// no lock names, so a leftover that is not a regular file has to be refused here
		// rather than removed later.
		if err := checkDestination(repo, manifestTemp); err != nil {
			return err
		}
	}
	return checkDestination(repo, lock.Filename)
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
	return writeFileAs(repo, dest, dest, data)
}

// writeFileAs is writeFile with the path it writes to and the path its failures name pulled
// apart. Only the manifest write uses the second form: it stages into a temporary file, and
// a reader told `cannot write ".graft.toml.tmp"` about a failed write would be sent to look
// at a path they have never seen and that no longer exists.
//
// The refusal of a destination that is not a regular file is the exception, and deliberately
// so: it names the staging path, because that is the path the user has to go and look at.
// Only the failures after the check are re-labelled.
func writeFileAs(repo *os.Root, at, name string, data []byte) error {
	if err := checkDestination(repo, at); err != nil {
		return err
	}
	if dir := path.Dir(at); dir != "." {
		if err := repo.MkdirAll(dir, dirMode); err != nil {
			return writeErrf(name, "%v", err)
		}
	}
	if err := repo.Remove(at); err != nil && !isNotExist(err) {
		return writeErrf(name, "%v", err)
	}

	f, err := repo.OpenFile(at, os.O_WRONLY|os.O_CREATE|os.O_EXCL, fileMode)
	if err != nil {
		return writeErrf(name, "%v", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return writeErrf(name, "%v", err)
	}
	if err := f.Close(); err != nil {
		return writeErrf(name, "%v", err)
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

// manifestTemp is where the new graft.toml is staged. It is at the repository root beside
// its destination, because a rename is only atomic within one filesystem, and it is named
// visibly rather than hidden: a leftover from a process that died mid-write should show up
// in `git status` rather than lurk.
const manifestTemp = ".graft.toml.tmp"

// writeManifest puts data at graft.toml through a temporary file and a rename.
//
// Not through writeFile, which removes an existing destination before creating it with
// O_EXCL. That removal is load-bearing for a *planned* write — the mode argument to a
// create-and-truncate open applies only on creation, so truncating would let a destination
// someone once made executable stay executable while a source replaced its contents — and
// its reason does not apply here, because no source's bytes are involved. The cost does
// apply: graft.toml is the one file in the repository graft cannot regenerate, and a failure
// between the unlink and a successful close would delete the consumer's own request. A
// rename makes a reader see either the old bytes or the new ones and never neither.
//
// The temporary file is removed on every failure, and only ever when it is a file. On
// success the rename consumed it.
func writeManifest(repo *os.Root, data []byte) error {
	if err := writeFileAs(repo, manifestTemp, manifest.Filename, data); err != nil {
		removeStaged(repo)
		return err
	}
	if err := repo.Rename(manifestTemp, manifest.Filename); err != nil {
		removeStaged(repo)
		return writeErrf(manifest.Filename, "%v", err)
	}
	return nil
}

// removeStaged deletes the staging file, and only when it is one.
//
// os.Root.Remove takes an empty directory and a symlink as readily as a regular file, and
// deleting either would be graft removing a path no lock claims — the one thing it may never
// do. The staging path is refused in the pre-flight pass when it holds anything else, so
// this is the second half of that check rather than a substitute for it: what survives to
// here is either the file this run created or nothing.
func removeStaged(repo *os.Root) {
	if fi, err := repo.Lstat(manifestTemp); err == nil && fi.Mode().IsRegular() {
		_ = repo.Remove(manifestTemp)
	}
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

// Manifest writes data to graft.toml at the repository root and does nothing else: no
// planned write, no deletion, no empty-directory removal, and no graft.lock.
//
// It exists because `graft add --no-sync` has to record what the consumer asked for
// without syncing, and it may not reach that through Run: internal/plan's prune set is
// the lock's files minus the new resolution, so an empty plan against a populated lock
// deletes everything the lock claims. The convenient path is a data-loss bug, and this is
// the entry point that cannot express one — it has no prune set at all.
//
// Everything else is the plan-carrying path's, unchanged: the same pre-flight refusals
// for graft.toml and for the staging path, the same staging file and rename, the same
// removal of a temporary file a failed rename left behind, and the same containment
// through the repository's os.Root. This widens what a caller may ask for; it does not
// add a second writer.
func Manifest(root string, data []byte) error {
	repo, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("cannot open the repository root %q: %w", root, err)
	}
	defer func() { _ = repo.Close() }()

	// Both checks before the first byte, for the reason preflight runs before Run's first
	// write: a refusal that arrives after the file is staged leaves a repository holding a
	// path nobody asked for.
	if err := checkDestination(repo, manifest.Filename); err != nil {
		return err
	}
	if err := checkDestination(repo, manifestTemp); err != nil {
		return err
	}
	return writeManifest(repo, data)
}
