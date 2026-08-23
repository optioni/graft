package plan

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/optioni/graft/internal/catalog"
)

// Listing is what one item's From contributes: the paths below it, each relative to
// From. For an item whose From names a file rather than a directory, Dir is false and
// Files holds exactly that file's base name.
//
// The listing is an input because enumerating a fetched tree is a filesystem question
// and this package may not ask one. Dir is carried explicitly rather than inferred
// from the listing, because a directory holding one identically named file is
// indistinguishable from a file, and the two place their contents differently.
type Listing struct {
	Dir   bool
	Files []string
}

// placement is one file of one item: where it comes from within the source's fetched
// tree, and the repo-relative path it lands at.
type placement struct {
	From string
	Dest string
}

// destinations maps one item's listed files to repo-relative paths. It walks the
// kind's `to` entries in declared order and, within each, the item's files in
// ascending path order, so the result — and the first error raised along the way —
// is independent of how the inputs were assembled.
//
// On any error it returns no placements at all. A caller that could act on a partially
// computed destination set would be acting on a plan that failed validation.
func destinations(in Input, it catalog.Item) ([]placement, error) {
	fail := itemErrf(in.Source.Name, it.ID)
	// One condition, one construction: the escape is raised for the interpolated
	// destination and for every path computed under it, and both must read alike.
	escape := func(dest string) error {
		return fail("destination %q escapes the repo root", dest)
	}
	kind := in.Catalog.Kinds[it.Kind]

	// Every `to` entry is interpolated and checked before any file is mapped, so a
	// destination that may not be used is refused even when the item contributes
	// nothing to place under it.
	to := make([]string, 0, len(kind.To))
	declared := make(map[string]string, len(kind.To))
	for _, entry := range kind.To {
		dest := strings.ReplaceAll(entry, "{name}", it.Name)
		if !insideRepo(strings.TrimSuffix(dest, "/")) {
			return nil, escape(dest)
		}
		if first, dup := declared[dest]; dup {
			// The catalog already refuses two identical `to` entries; these are two
			// different ones that collapse onto the same path once {name} is filled
			// in, which only shows up per item.
			return nil, fail("destinations %q and %q both interpolate to %q", first, entry, dest)
		}
		declared[dest] = entry
		to = append(to, dest)
	}

	listing := in.Items[it.ID]
	files := slices.Clone(listing.Files)
	slices.Sort(files)

	var out []placement
	for _, dest := range to {
		// Scoped to one destination entry: the same base name under two entries of a
		// list-valued `to` is two different paths, not a collision.
		flattened := make(map[string]string, len(files))
		for _, rel := range files {
			final := place(dest, listing.Dir, kind.Flatten, rel)
			if !insideRepo(final) {
				return nil, escape(final)
			}
			if kind.Flatten {
				if first, dup := flattened[final]; dup {
					// files is sorted, so the two paths are named in ascending order.
					return nil, fail("flatten maps %q and %q to the same destination %q", first, rel, final)
				}
				flattened[final] = rel
			}
			out = append(out, placement{From: sourcePath(it, listing.Dir, rel), Dest: final})
		}
	}
	return out, nil
}

// itemErrf builds the per-item error prefix every message in this package shares, so
// each condition is worded in exactly one place and a test can pin it.
func itemErrf(source, id string) func(string, ...any) error {
	return func(format string, args ...any) error {
		return fmt.Errorf("source %q: item %q: %s", source, id, fmt.Sprintf(format, args...))
	}
}

// place computes one file's destination.
//
// A trailing "/" on the destination means "into this directory", which only a file
// item can be asked to obey: for a directory item, `to` names the destination
// directory whether or not it is written with a slash, and appending the item's own
// leaf would both repeat it — openspec/schemas/tdd/tdd — and put the source's From
// into every consumer's tree, which is exactly the coupling an item id exists to
// avoid. So a slashless destination names the file for a file item, and everything
// else joins below the destination directory.
func place(dest string, dir, flatten bool, rel string) string {
	if !dir && !strings.HasSuffix(dest, "/") {
		return path.Clean(dest)
	}
	// path.Clean is what makes the trailing slash a no-op for a directory item.
	// Taking the base name is flatten's whole effect; a file item takes it too, and
	// harmlessly, since its one listed path is already its own base name.
	if flatten || !dir {
		rel = path.Base(rel)
	}
	return path.Join(path.Clean(dest), rel)
}

// sourcePath names the file to copy, within the source's fetched tree. For a directory
// item it is From joined with the listed path; for a file item it is From itself.
// Joining in the file case would produce extras/agents/x.md/x.md, since a file item's
// one listed path is its own base name.
func sourcePath(it catalog.Item, dir bool, rel string) string {
	if !dir {
		return it.From
	}
	return path.Join(it.From, rel)
}

// insideRepo reports whether p is a path graft may write inside the consumer's
// repository. SPEC.md makes "no destination escapes the repo root" an invariant, and a
// destination is the one thing in this computation a source repository controls, so
// the check is applied to the interpolated destination itself as well as to every file
// path computed under it.
//
// ".." and a leading "/" are the obvious escapes. Two quieter cases need the same
// answer: "." is the repo root itself, which names the whole worktree rather than a
// file, and an uncleaned alias like "./a.md" names the same file as "a.md" while
// looking different to the collision check and to the lock's own file list.
//
// This is the third copy of a small path rule — internal/catalog constrains a source's
// `from`, internal/lock constrains what a lock may authorise deleting — and it is
// written here on purpose. Each has a different subject and its own wording, and
// hoisting them into a shared package would put one predicate between three invariants
// that are allowed to diverge.
func insideRepo(p string) bool {
	if p == "" || p == "." || strings.HasPrefix(p, "/") {
		return false
	}
	if slices.Contains(strings.Split(p, "/"), "..") {
		return false
	}
	return path.Clean(p) == p
}
