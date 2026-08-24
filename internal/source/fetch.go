package source

import (
	"errors"
	"os"
	"path/filepath"
)

// emptyTree is git's hash of the empty tree object. Pointing attr.tree at it disables
// the source's own committed .gitattributes for the checkout — see Fetch.
const emptyTree = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

// Fetch returns the path to the source's tree at sha, fetching it if the cache lacks it.
//
// On a cache hit it runs no git command at all. That is not an optimization: SPEC.md's
// "network unavailable, cache hit: proceeds" means graft does not try, so a sync against
// a lock whose shas are cached does no network I/O and works on a plane.
//
// The corollary is that an entry may never exist unless it is complete. A commit's tree
// is immutable, so nothing will ever re-fetch an entry that exists, and an incomplete one
// would be wrong forever. Fetch therefore builds in a temporary directory beside the
// entry and publishes it by renaming.
//
// Fetch writes only under the cache root. It creates, modifies, and deletes nothing in
// the repository graft is running in.
func (c Cache) Fetch(name, git, sha string) (string, error) {
	fail := sourceErrf(name)
	entry, err := c.Entry(name, git, sha)
	if err != nil {
		return "", err
	}
	// Before exec.LookPath and before any subprocess. This one line is the offline
	// guarantee.
	if isDir(entry) {
		return entry, nil
	}
	url, err := CloneURL(name, git)
	if err != nil {
		return "", err
	}

	parent := filepath.Dir(entry)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", fail("cannot create cache entry for %q: %v", sha, err)
	}
	// Made in the entry's own parent so the publishing rename stays within one
	// filesystem.
	scaffold, err := os.MkdirTemp(parent, ".graft-fetch-")
	if err != nil {
		return "", fail("cannot create cache entry for %q: %v", sha, err)
	}
	// On every path, including success: after the rename the scaffold holds only the
	// now-empty directory the tree was moved out of. Its removal is best-effort by
	// design — a leftover scaffold is not an entry, so it cannot be mistaken for one,
	// and failing a good fetch over it would be the worse outcome.
	defer func() { _ = os.RemoveAll(scaffold) }()

	tree := filepath.Join(scaffold, "tree")
	if err := os.Mkdir(tree, 0o755); err != nil {
		return "", fail("cannot create cache entry for %q: %v", sha, err)
	}
	if err := fetchTree(filepath.Join(scaffold, "git"), tree, url, sha); err != nil {
		if errors.Is(err, errNoGit) {
			return "", err
		}
		return "", fail("cannot fetch %q from %q: %v", sha, url, err)
	}

	if err := os.Rename(tree, entry); err != nil {
		// Another process published the same entry first. Two runs racing on one sha
		// both want the same immutable tree, so this is a hit rather than a failure —
		// turning it into an error would make concurrent syncs fail for no reason.
		if isDir(entry) {
			return entry, nil
		}
		return "", fail("cannot create cache entry for %q: %v", sha, err)
	}
	return entry, nil
}

// fetchTree puts the tree at sha into work, using gitDir as a bare repository beside it.
//
// The git directory sits beside the work tree rather than inside it, so no .git ever
// exists within the tree that gets published — a stronger guarantee than deleting one
// afterwards, where an interrupted run could leave the repository behind.
func fetchTree(gitDir, work, url, sha string) error {
	if detail, err := git("init", "-q", "--bare", gitDir); err != nil {
		return gitErr(detail, err)
	}
	// "--" separates options from operands: a URL is not trusted to be a URL, because
	// git decides what is a flag by its spelling and not by its position.
	if detail, err := git("--git-dir="+gitDir, "remote", "add", "origin", "--", url); err != nil {
		return gitErr(detail, err)
	}
	// One commit's history, and no tag objects the tree does not need.
	if detail, err := git("--git-dir="+gitDir, "fetch", "--depth", "1", "--no-tags", "-q", "origin", sha); err != nil {
		return gitErr(detail, err)
	}
	// attr.tree at the empty tree disables the source's own committed .gitattributes.
	// Without it a source rewrites the bytes it is cached as — `* text eol=crlf` turns
	// LF into CRLF, `ident` expands $Id$ — and, the reason this is not cosmetic,
	// `filter=` selects a driver whose command comes from the consumer's git config.
	// That would be a source-controlled file causing a program to run, which is exactly
	// what graft does not do. core.autocrlf and core.eol do not close it: the in-tree
	// attributes, not the config, are what select the filter.
	detail, err := git(
		"--git-dir="+gitDir,
		"-c", "attr.tree="+emptyTree,
		"-c", "core.bare=false",
		"--work-tree="+work,
		"checkout", "-q", "--detach", "FETCH_HEAD",
	)
	if err != nil {
		return gitErr(detail, err)
	}
	return nil
}

// gitErr prefers git's own first stderr line to exec's exit status, which says only that
// something failed.
func gitErr(detail string, err error) error {
	if errors.Is(err, errNoGit) {
		return err
	}
	if detail != "" {
		return errors.New(detail)
	}
	return err
}

func isDir(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
