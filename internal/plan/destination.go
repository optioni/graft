package plan

import (
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
// ascending path order, so the result — and every error raised along the way — is
// independent of how the inputs were assembled.
func destinations(in Input, it catalog.Item) ([]placement, error) {
	kind := in.Catalog.Kinds[it.Kind]

	// Every `to` entry is interpolated and checked before any file is mapped, so a
	// destination that cannot be used is refused even when the item contributes
	// nothing to place under it.
	to := make([]string, 0, len(kind.To))
	for _, entry := range kind.To {
		to = append(to, strings.ReplaceAll(entry, "{name}", it.Name))
	}

	files := slices.Clone(in.Items[it.ID].Files)
	slices.Sort(files)
	dir := in.Items[it.ID].Dir

	var out []placement
	for _, dest := range to {
		for _, rel := range files {
			out = append(out, placement{
				From: sourcePath(it, dir, rel),
				Dest: place(dest, dir, kind.Flatten, rel),
			})
		}
	}
	return out, nil
}

// place computes one file's destination.
//
// A trailing "/" on the destination means "into this directory", which only a file
// item can be asked to obey: for a directory item, `to` names the destination
// directory whether or not it is written with a slash, and appending the item's own
// leaf would both repeat it — openspec/schemas/tdd/tdd — and put the source's From
// into every consumer's tree, which is exactly the coupling SPEC.md says an item id
// exists to avoid. So a slashless destination names the file for a file item, and
// everything else joins below the destination directory.
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
