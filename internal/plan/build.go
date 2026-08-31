package plan

import (
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

// Plan is what a sync will do, held as a value: the files to write, the files to
// delete, and the lock to record afterwards. It carries no notion of "unchanged" and
// no report — synced files are derived artifacts, always overwritten, and a plan has
// no file content to compare.
type Plan struct {
	// Writes is ordered by Dest, in ascending byte order.
	Writes []Write
	// Prune is ordered by path, in ascending byte order.
	Prune []string
	// Lock is what internal/apply serializes last, after every file operation has
	// succeeded. Building it here rather than in apply keeps it a record of what was
	// planned rather than of what happened to get written.
	Lock *lock.Lock
}

// Write is one file to copy: which source's fetched tree it comes from, which item it
// belongs to, its path within that tree, and where it lands in the consumer's repo.
type Write struct {
	Source string
	Item   string
	From   string
	Dest   string

	// Claimed reports whether the lock this plan was built against already claims Dest,
	// for any source and any item. A claimed path is a file graft wrote and is rewriting;
	// an unclaimed one is either absent or somebody else's, and only a filesystem can say
	// which — that is internal/apply's question, and this is what tells it which paths are
	// worth asking about.
	//
	// It is a set operation over the lock this package is already given, so it costs no
	// filesystem access and this package stays pure.
	Claimed bool
}

// claim records which item took a destination, so the second item to reach it can name
// the first. SPEC.md makes a collision an error rather than last-writer-wins: the loser
// would be a file the lock claims and a later sync would delete.
//
// The walk is deterministic — sources by name, items by id, destinations in declared
// order, files by ascending path — so the two items are named in a stable order and the
// message is assertable. Only the first collision is reported; one message is the shape
// of SPEC.md's failure-mode table.
type claim struct {
	source string
	item   string
	dest   string
}

// claims records which item took which path, and every directory those paths imply.
// Two items may not share a path, and — the quieter half — one item's file may not be
// the directory another item's file needs: docs/api and docs/api/index.md cannot both
// exist. Left to apply, that fails partway through, and because the lock is written
// last the file already written is absent from graft.lock, outside anything a later
// prune could reach. Refusing it here is what keeps that state unreachable.
type claims struct {
	files map[string]claim
	dirs  map[string]claim
}

func newClaims() *claims {
	return &claims{files: map[string]claim{}, dirs: map[string]claim{}}
}

// take records one destination, or returns the collision it makes with an earlier one.
func (c *claims) take(next claim) error {
	if first, taken := c.files[next.dest]; taken {
		return fmt.Errorf(
			"source %q item %q and source %q item %q both resolve to %q",
			first.source, first.item, next.source, next.item, next.dest,
		)
	}
	// next is a directory some earlier item's file already lives in.
	if first, taken := c.dirs[next.dest]; taken {
		return nesting(first, next)
	}
	// Some earlier item's file is a directory next needs.
	for dir := path.Dir(next.dest); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if first, taken := c.files[dir]; taken {
			return nesting(first, next)
		}
	}

	c.files[next.dest] = next
	for dir := path.Dir(next.dest); dir != "." && dir != "/"; dir = path.Dir(dir) {
		if _, known := c.dirs[dir]; !known {
			c.dirs[dir] = next
		}
	}
	return nil
}

// nesting names both paths, in the order the deterministic walk reached them, because
// neither path on its own says what the clash is.
func nesting(first, next claim) error {
	return fmt.Errorf(
		"source %q item %q writes %q and source %q item %q writes %q: one cannot contain the other",
		first.source, first.item, first.dest, next.source, next.item, next.dest,
	)
}

// isSHA reports whether s is the 40-character lowercase hex sha graft.lock requires.
//
// This is the one load-time constraint on the lock that a caller can violate silently:
// unique source names, unique item ids, and no path claimed twice all follow from what
// planning already guarantees, and the round-trip test is what verifies them. A bad
// resolved sha follows from nothing, and lock.Marshal validates nothing, so leaving it
// to the caller would let plan build a lock lock.Parse refuses — two packages
// disagreeing about what a valid graft.lock is, surfacing one run later, in a different
// package, against a file the user is told not to edit.
func isSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// Build turns each source's resolved inputs and the current lock into a plan. It reads
// no file, stats no path, runs no command, opens no network connection, and creates,
// modifies, or deletes nothing.
//
// It is total over well-formed inputs: no source at all, no lock at all, and an item
// contributing no files are legitimate states rather than failures. On any error the
// returned plan is nil, so no caller can act on a half-computed one — which is what
// makes "nothing touches the tree until every check passes" a property of the type.
//
// Build does not re-validate what its collaborators guarantee. It relies on every
// installed item's kind being declared in the catalog (catalog.Parse refuses an item
// whose kind is not), on source names being unique (manifest.Parse's sources come from
// a TOML table), on Resolved being a 40-character hex sha (internal/source's job), and
// on a file item's listing holding exactly its own base name. Nor does it check pins:
// lock.CheckPins belongs to the caller, and must fire before anything is fetched
// rather than after.
func Build(inputs []Input, lk *lock.Lock) (*Plan, error) {
	p := &Plan{Lock: &lock.Lock{Version: lock.Version}}
	// The one file set this build produces, shared between the prune diff and the
	// next lock rather than derived twice from two walks that could disagree.
	produced := map[string]struct{}{}
	// Filled as the walk proceeds; the first clash is the error.
	claimed := newClaims()

	// Sorted here rather than assumed sorted because manifest.Parse happens to sort:
	// Build takes a slice a caller assembled, and an unsorted one would churn every
	// consumer's lock diff on every sync. The copy keeps the caller's slice its own.
	sorted := slices.Clone(inputs)
	slices.SortFunc(sorted, func(a, b Input) int {
		return strings.Compare(a.Source.Name, b.Source.Name)
	})

	for _, in := range sorted {
		// Before anything is planned for this source, so a lock lock.Parse would
		// refuse is never built even partway.
		if !isSHA(in.Resolved) {
			return nil, fmt.Errorf(
				"source %q: resolved %q is not a 40-character hex sha",
				in.Source.Name, in.Resolved,
			)
		}

		// Returned unchanged: typo protection has to reach the user through planning
		// rather than being swallowed by it.
		items, err := catalog.Expand(in.Catalog, in.Source.Name, in.Source.Install)
		if err != nil {
			return nil, err
		}
		if err := checkOverrides(in); err != nil {
			return nil, err
		}

		src := lock.Source{
			Name:     in.Source.Name,
			Git:      in.Source.Git,
			Rev:      in.Source.Rev,
			Matched:  in.Matched,
			Resolved: in.Resolved,
		}
		// catalog.Expand returns items ordered by id, so every error raised below and
		// every slice built below is independent of map iteration order.
		for _, it := range items {
			places, err := destinations(in, it)
			if err != nil {
				return nil, err
			}

			files := make([]string, 0, len(places))
			for _, pl := range places {
				// Checked before the write is appended, so a build that fails has
				// produced nothing a caller could act on.
				if err := claimed.take(claim{
					source: in.Source.Name,
					item:   it.ID,
					dest:   pl.Dest,
				}); err != nil {
					return nil, err
				}

				p.Writes = append(p.Writes, Write{
					Source: in.Source.Name,
					Item:   it.ID,
					From:   pl.From,
					Dest:   pl.Dest,
				})
				produced[pl.Dest] = struct{}{}
				files = append(files, pl.Dest)
			}
			slices.Sort(files)
			src.Items = append(src.Items, lock.Item{ID: it.ID, Files: files})
		}
		// catalog.Expand documents this order, but the plan's own ordering is asserted
		// against and read by internal/apply, so it is established here rather than
		// inherited. lock.Marshal normalizes on its way out; relying on that would let
		// an unsorted plan hide behind the bytes.
		slices.SortFunc(src.Items, func(a, b lock.Item) int { return strings.Compare(a.ID, b.ID) })
		p.Lock.Sources = append(p.Lock.Sources, src)
	}

	slices.SortFunc(p.Writes, func(a, b Write) int { return strings.Compare(a.Dest, b.Dest) })
	markClaimed(p.Writes, lockedPaths(lk))
	p.Prune = pruneSet(lk, produced)
	return p, nil
}

// lockedPaths is every path the lock claims, across every source and item. The question a
// caller asks of it is whether graft owned a path, not which item owned it: a path moving
// between items is still graft rewriting its own file.
func lockedPaths(lk *lock.Lock) map[string]struct{} {
	out := map[string]struct{}{}
	if lk == nil {
		return out
	}
	for _, s := range lk.Sources {
		for _, it := range s.Items {
			for _, f := range it.Files {
				out[f] = struct{}{}
			}
		}
	}
	return out
}

// markClaimed records, per write, whether the lock already claimed its destination.
func markClaimed(writes []Write, locked map[string]struct{}) {
	for i := range writes {
		_, writes[i].Claimed = locked[writes[i].Dest]
	}
}

// pruneSet is exactly those paths the lock claims that the new resolution no longer
// produces, ordered by path. A path enters it only by being in the lock — never by
// being found in a destination directory, which this package could not look in even if
// it wanted to.
//
// That is what lets synced files share a directory with files graft does not own: a
// file absent from graft.lock is invisible to graft and can never be deleted by it.
// The diff is over paths rather than per source, so a path one source stops producing
// and another starts producing is written rather than deleted and re-created.
func pruneSet(lk *lock.Lock, produced map[string]struct{}) []string {
	if lk == nil {
		return nil
	}

	// Collected as a set, so a lock that claimed one path from two items — which
	// lock.Parse refuses, but a lock built in code could hold — cannot ask for the
	// same deletion twice.
	drop := map[string]struct{}{}
	for _, s := range lk.Sources {
		for _, it := range s.Items {
			for _, f := range it.Files {
				if _, kept := produced[f]; !kept {
					drop[f] = struct{}{}
				}
			}
		}
	}

	out := make([]string, 0, len(drop))
	for f := range drop {
		out = append(out, f)
	}
	slices.Sort(out)
	return out
}
