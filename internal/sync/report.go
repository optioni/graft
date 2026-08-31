package sync

import (
	"bytes"
	"slices"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/plan"
)

// The three verbs SPEC.md's report uses. Words, not symbols.
const (
	verbAdded   = "added"
	verbAdopted = "adopted"
	verbUpdated = "updated"
	verbRemoved = "removed"
)

// The three notes a removed item can carry. Each is distinguishable only from something
// the report is handed — the source's catalog, or the absence of the source from the new
// lock — which is why they are decided here rather than anywhere downstream.
const (
	noteNotProvided  = "no longer provided"
	noteNotInstalled = "no longer installed"
	noteSourceGone   = "source removed"

	// noteReplaced is the one note an added or updated item can carry: this write went
	// over content graft did not own.
	noteReplaced = "replaced existing content"
)

// Report is what a sync changed, as a value: which sources moved, which items were added,
// updated, or removed, and how many files were written and deleted.
//
// It is built from the lock that was on disk, the lock the plan produced, and the plan's
// own counts — never from the tree. Every planned file is written on every sync, so
// "updated" cannot mean "the bytes changed", and no comparison this package could make
// would let it mean that.
type Report struct {
	// Sources holds only the sources with something to say, in name order over the union
	// of the two locks.
	Sources []SourceReport

	// Written is every file the plan writes, and Removed the size of its prune set.
	Written int
	Removed int

	// Replaced is how many of those writes went over content the lock did not claim. It is
	// filled in after the apply, because it is a fact about the filesystem rather than
	// about the plan — which is why a dry run always reports none.
	Replaced int

	// DryRun changes what the summary says, not what any of the rest means.
	DryRun bool

	// upToDate is byte equality of the two serialized locks plus an empty prune set. It is
	// held rather than derived on demand because the locks are not.
	upToDate bool
}

// SourceReport is one source's block. A previous half is empty when there was no previous
// one — a source the lock had never seen, and a source that is being reported only because
// it is going away.
type SourceReport struct {
	Name         string
	Rev          string
	PrevRev      string
	Matched      string
	PrevMatched  string
	Resolved     string
	PrevResolved string
	Items        []ItemReport
}

// ItemReport is one line: the verb, the item, how many files, and — for a removed item —
// why it went.
type ItemReport struct {
	Verb  string
	ID    string
	Files int
	Note  string
}

// UpToDate reports whether this sync had nothing to do.
//
// The test is byte equality of the two serialized locks plus an empty prune set: one
// predicate over the two artifacts a reader would diff, rather than a conjunction of the
// conditions that produce report lines. It also covers the case those conditions miss — a
// source whose git value changed in graft.toml with the same rev produces a different lock
// and no item lines, and must not be reported as nothing to do.
//
// Its known cost is the reverse case: a user who deleted installed files by hand and
// re-syncs gets them back and is told "up to date", because the lock did not move and
// nothing was pruned. Narrowing the predicate to cover that would mean checking every
// destination's presence, which is the tree-scanning this design does not do — and the
// restored files still appear where SPEC.md says a sync's effect appears, in git status.
func (r *Report) UpToDate() bool { return r.upToDate }

// newReport derives the report from the two locks, the plan, and the catalog of each source
// the manifest still declares.
//
// catalogs is keyed by source name and holds an entry only for a source that was fetched.
// A source graft.toml no longer declares has none, which is exactly what tells its removed
// items apart from ones a selector stopped matching.
func newReport(before *lock.Lock, p *plan.Plan, catalogs map[string]*catalog.Catalog, dryRun bool) *Report {
	after := p.Lock
	r := &Report{
		Written:  len(p.Writes),
		Removed:  len(p.Prune),
		DryRun:   dryRun,
		upToDate: len(p.Prune) == 0 && bytes.Equal(lock.Marshal(before), lock.Marshal(after)),
	}

	old := byName(before)
	current := byName(after)
	for _, name := range unionOfNames(old, current) {
		b, hadBefore := old[name]
		a, hasAfter := current[name]

		items := itemReports(b, a, hasAfter, catalogs[name])
		// A source with no item lines is still worth a header when its pin moved; one
		// whose pin held and whose items all held has nothing to say at all. Matched is
		// checked alongside Rev and Resolved: a retag onto the same commit changes
		// nothing else, and a report that skipped it would print a summary describing a
		// lock diff it never explained.
		if len(items) == 0 && hadBefore && hasAfter &&
			b.Rev == a.Rev && b.Resolved == a.Resolved && b.Matched == a.Matched {
			continue
		}

		s := SourceReport{Name: name}
		switch {
		case !hasAfter:
			// Reported only because it is going away: there is no new half to move to.
			s.Rev, s.Matched, s.Resolved = b.Rev, b.Matched, b.Resolved
		case !hadBefore:
			s.Rev, s.Matched, s.Resolved = a.Rev, a.Matched, a.Resolved
		default:
			s.PrevRev, s.PrevMatched, s.PrevResolved = b.Rev, b.Matched, b.Resolved
			s.Rev, s.Matched, s.Resolved = a.Rev, a.Matched, a.Resolved
		}
		s.Items = items
		r.Sources = append(r.Sources, s)
	}
	return r
}

// itemReports is one source's lines, in item-id order.
//
// An item present in both locks earns a line when the source's sha moved or its own file
// list did. The sha half is what makes a version bump report every item it installs: the
// bytes behind an unchanged path may well have changed, and there is no content comparison
// here to say otherwise.
func itemReports(b, a lock.Source, hasAfter bool, cat *catalog.Catalog) []ItemReport {
	oldItems := make(map[string][]string, len(b.Items))
	for _, it := range b.Items {
		oldItems[it.ID] = it.Files
	}
	newItems := make(map[string][]string, len(a.Items))
	for _, it := range a.Items {
		newItems[it.ID] = it.Files
	}

	ids := make([]string, 0, len(oldItems)+len(newItems))
	for id := range oldItems {
		ids = append(ids, id)
	}
	for id := range newItems {
		if _, dup := oldItems[id]; !dup {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)

	var out []ItemReport
	for _, id := range ids {
		oldFiles, had := oldItems[id]
		newFiles, has := newItems[id]
		switch {
		case has && !had:
			out = append(out, ItemReport{Verb: verbAdded, ID: id, Files: len(newFiles)})
		case !has:
			out = append(out, ItemReport{
				Verb: verbRemoved, ID: id, Files: len(oldFiles),
				Note: removalNote(id, hasAfter, cat),
			})
		case a.Resolved != b.Resolved || !slices.Equal(oldFiles, newFiles):
			out = append(out, ItemReport{Verb: verbUpdated, ID: id, Files: len(newFiles)})
		}
	}
	return out
}

// removalNote says why an item went. The three cases are genuinely different to a reader:
// a source dropped from graft.toml, a selector that stopped matching, and a source that
// stopped offering the item at the resolved sha.
func removalNote(id string, hasAfter bool, cat *catalog.Catalog) string {
	if !hasAfter {
		// No catalog was read for this source, because nothing was fetched for it.
		return noteSourceGone
	}
	if cat != nil {
		for _, it := range cat.Items {
			if it.ID == id {
				return noteNotInstalled
			}
		}
	}
	return noteNotProvided
}

func byName(l *lock.Lock) map[string]lock.Source {
	out := make(map[string]lock.Source, len(l.Sources))
	for _, s := range l.Sources {
		out[s.Name] = s
	}
	return out
}

// unionOfNames is every source in either lock, sorted. A source dropped from graft.toml is
// in the old lock only and is still reported; taking the new lock's order alone would drop
// exactly the sources whose files are being deleted.
func unionOfNames(old, current map[string]lock.Source) []string {
	names := make([]string, 0, len(old)+len(current))
	for name := range old {
		names = append(names, name)
	}
	for name := range current {
		if _, dup := old[name]; !dup {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// adopt folds the applier's account of what it replaced into the report: the note on every
// item that replaced something, the verb where `added` would otherwise be false, and the
// count the summary carries.
//
// The verb is corrected only for an item the lock had never seen. An item already in the
// lock that gains a file at an occupied path has genuinely been updated, and a fourth verb
// for the combination would name an intersection rather than an event — the note says the
// rest.
//
// It runs after the apply because nothing before the apply can know. A dry run never calls
// it, which is exactly why a dry run reports no adoption.
func (r *Report) adopt(p *plan.Plan, replaced []string) {
	if len(replaced) == 0 {
		return
	}
	r.Replaced = len(replaced)

	dests := make(map[string]struct{}, len(replaced))
	for _, d := range replaced {
		dests[d] = struct{}{}
	}

	type owner struct{ source, item string }
	adopted := map[owner]struct{}{}
	for _, w := range p.Writes {
		if _, ok := dests[w.Dest]; ok {
			adopted[owner{w.Source, w.Item}] = struct{}{}
		}
	}

	for i := range r.Sources {
		for j := range r.Sources[i].Items {
			it := &r.Sources[i].Items[j]
			if _, ok := adopted[owner{r.Sources[i].Name, it.ID}]; !ok {
				continue
			}
			if it.Verb == verbAdded {
				it.Verb = verbAdopted
			}
			it.Note = noteReplaced
		}
	}
}
