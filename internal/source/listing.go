package source

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/plan"
)

// catalogFile is the one name a source's offer may live under. SPEC.md admits no
// alternative and no fallback to guessing a layout.
const catalogFile = "catalog.yaml"

// ReadCatalog reads catalog.yaml from the root of a fetched tree. It lives here because
// this package is the only one that knows where a fetched tree is.
//
// It adds no error wording of its own: a missing catalog surfaces internal/catalog's
// "not graftable" message, so that failure keeps exactly one owner.
//
// The read is contained to the entry. A source commits its own catalog.yaml, so it may
// commit a symlink under that name, and an ordinary read follows one — a link to an
// absolute path would make graft parse a file the source never contained.
func ReadCatalog(entry string) (*catalog.Catalog, error) {
	root, err := os.OpenRoot(entry)
	if err != nil {
		return nil, errf(catalogFile, err)
	}
	defer func() { _ = root.Close() }()

	data, err := root.ReadFile(catalogFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			// Delegated only here, where there is by definition no link to follow, so
			// catalog.Load owns the not-graftable wording without ever being handed a
			// path that could resolve outside the entry.
			return catalog.Load(filepath.Join(entry, catalogFile))
		}
		// catalog.Load's own format for a read that fails for a reason other than
		// absence, not a second wording of it.
		return nil, errf(catalogFile, err)
	}
	return catalog.Parse(data, catalogFile)
}

// List turns one item's From, resolved inside a fetched tree, into the plan.Listing that
// plan.Build consumes. Returning plan's own type rather than a convertible twin is what
// makes "usable as plan.Input.Items with no adaptation" a property of the type system
// rather than a claim a test has to keep checking.
//
// Every path operation goes through the entry's os.Root, which refuses any name whose
// components leave it. Checking only the last component of From would close nothing: a
// source committing `extras` as a symlink out of the tree and declaring
// `from: extras/secrets` reads a directory outside the entry entirely, and catalog's own
// rule — relative, cleaned, no ".." segment — finds nothing wrong with the string.
//
// List creates, modifies, and deletes nothing.
func List(entry, name string, it catalog.Item) (plan.Listing, error) {
	fail := itemErrf(name, it.ID)

	root, err := os.OpenRoot(entry)
	if err != nil {
		return plan.Listing{}, fail("from %q not found in the source tree", it.From)
	}
	defer func() { _ = root.Close() }()

	// Lstat, never Stat: Stat would follow a symlink and answer about its target, which
	// is exactly the case that must be refused.
	fi, err := root.Lstat(it.From)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return plan.Listing{}, fail("from %q not found in the source tree", it.From)
		}
		// A component of From resolved outside the entry, or could not be read at all.
		// Either way From does not name something in this source's tree.
		return plan.Listing{}, fail("from %q is not a regular file or directory", it.From)
	}

	switch {
	case fi.Mode().IsRegular():
		// The base name, relative to From, as destination-computation requires for a
		// file item.
		return plan.Listing{Files: []string{filepath.Base(it.From)}}, nil
	case fi.IsDir():
		files, err := walk(root, it.From)
		if err != nil {
			return plan.Listing{}, fail("from %q not found in the source tree", it.From)
		}
		return plan.Listing{Dir: true, Files: files}, nil
	}
	// A symlink, socket, device, or fifo. Refused rather than skipped: From is a claim
	// about where an entire item lives, and listing zero files for it would install
	// nothing while reporting success.
	return plan.Listing{}, fail("from %q is not a regular file or directory", it.From)
}

// walk enumerates the regular files at or below from, each path relative to from,
// slash-separated and sorted ascending — byte-stable across platforms and across two
// runs, which is what keeps graft.lock's files from churning.
//
// The walk is re-rooted at from, so it is contained to the item's own subtree and not
// merely to the entry. fs.WalkDir never follows a symlink, so a link below from is
// skipped rather than traversed: graft copies file content, and a symlink is not content.
func walk(root *os.Root, from string) ([]string, error) {
	sub, err := root.OpenRoot(from)
	if err != nil {
		return nil, err
	}
	defer func() { _ = sub.Close() }()

	var files []string
	err = fs.WalkDir(sub.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	return files, nil
}
