package sync

import (
	"slices"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/plan"
)

// The report is built from the lock that was on disk, the lock the plan produced, and the
// plan's own counts — never from the tree. These tests supply all three directly, with no
// filesystem anywhere: a test here that needed a directory would mean the boundary moved.

const (
	shaA = "aaaaaaa2a30c1d4b8e9f0a2b3c4d5e6f708192a3"
	shaB = "bbbbbbb2a30c1d4b8e9f0a2b3c4d5e6f708192a3"
)

func item(id string, files ...string) lock.Item {
	return lock.Item{ID: id, Files: files}
}

func lockOf(sources ...lock.Source) *lock.Lock {
	return &lock.Lock{Version: lock.Version, Sources: sources}
}

func sourceOf(name, rev, resolved string, items ...lock.Item) lock.Source {
	return lock.Source{Name: name, Git: "example.com/o/" + name, Rev: rev, Resolved: resolved, Items: items}
}

// provides is a catalog offering exactly these item ids, which is all the report reads one
// for: whether a removed item is still on offer decides which note it carries.
func provides(ids ...string) *catalog.Catalog {
	c := &catalog.Catalog{Version: catalog.Version, Kinds: map[string]catalog.Kind{}}
	for _, id := range ids {
		c.Items = append(c.Items, catalog.Item{ID: id})
	}
	return c
}

// report is newReport with the arguments a test usually does not care about defaulted.
func report(before, after *lock.Lock, prune []string, cats map[string]*catalog.Catalog) *Report {
	return newReport(before, &plan.Plan{Prune: prune, Lock: after}, cats, false)
}

func onlySource(t *testing.T, r *Report) SourceReport {
	t.Helper()
	if len(r.Sources) != 1 {
		t.Fatalf("sources = %+v, want exactly one", r.Sources)
	}
	return r.Sources[0]
}

func TestReportVerbAdded(t *testing.T) {
	t.Parallel()

	r := report(
		lockOf(sourceOf("shared", "v1.0.0", shaA, item("schema:tdd", "a.md"))),
		lockOf(sourceOf("shared", "v1.0.0", shaA,
			item("agent:reviewer", "b.md"), item("schema:tdd", "a.md"))),
		nil,
		map[string]*catalog.Catalog{"shared": provides("agent:reviewer", "schema:tdd")},
	)

	got := onlySource(t, r).Items
	want := []ItemReport{{Verb: "added", ID: "agent:reviewer", Files: 1}}
	if !slices.Equal(got, want) {
		t.Errorf("items = %+v, want %+v", got, want)
	}
}

// The bytes behind an unchanged path may well have changed, and graft has no content
// comparison to say otherwise. Reporting the item as untouched would be a claim it cannot
// support.
func TestReportVerbUpdatedOnMovedPin(t *testing.T) {
	t.Parallel()

	r := report(
		lockOf(sourceOf("shared", "v1.2.0", shaA, item("schema:tdd", "a.md"))),
		lockOf(sourceOf("shared", "v1.3.0", shaB, item("schema:tdd", "a.md"))),
		nil,
		map[string]*catalog.Catalog{"shared": provides("schema:tdd")},
	)

	got := onlySource(t, r).Items
	want := []ItemReport{{Verb: "updated", ID: "schema:tdd", Files: 1}}
	if !slices.Equal(got, want) {
		t.Errorf("items = %+v, want %+v", got, want)
	}
}

func TestReportSilentItem(t *testing.T) {
	t.Parallel()

	r := report(
		lockOf(sourceOf("shared", "v1.0.0", shaA,
			item("agent:reviewer", "b.md"), item("schema:tdd", "a.md"))),
		lockOf(sourceOf("shared", "v1.0.0", shaA,
			item("agent:reviewer", "b.md", "c.md"), item("schema:tdd", "a.md"))),
		nil,
		map[string]*catalog.Catalog{"shared": provides("agent:reviewer", "schema:tdd")},
	)

	// The source has something to report, so the block is emitted — but only the item
	// whose file list moved appears in it.
	got := onlySource(t, r).Items
	want := []ItemReport{{Verb: "updated", ID: "agent:reviewer", Files: 2}}
	if !slices.Equal(got, want) {
		t.Errorf("items = %+v, want %+v", got, want)
	}
}

// The three notes a removed item can carry, each distinguishable only from something the
// report is handed: the catalog, or the absence of a source in the new lock.
func TestReportRemovalNotes(t *testing.T) {
	t.Parallel()

	t.Run("no longer installed", func(t *testing.T) {
		t.Parallel()

		r := report(
			lockOf(sourceOf("shared", "v1.0.0", shaA,
				item("agent:reviewer", "b.md"), item("schema:tdd", "a.md"))),
			lockOf(sourceOf("shared", "v1.0.0", shaA, item("schema:tdd", "a.md"))),
			[]string{"b.md"},
			map[string]*catalog.Catalog{"shared": provides("agent:reviewer", "schema:tdd")},
		)

		got := onlySource(t, r).Items
		want := []ItemReport{{Verb: "removed", ID: "agent:reviewer", Files: 1, Note: "no longer installed"}}
		if !slices.Equal(got, want) {
			t.Errorf("items = %+v, want %+v", got, want)
		}
	})

	t.Run("no longer provided", func(t *testing.T) {
		t.Parallel()

		r := report(
			lockOf(sourceOf("shared", "v1.2.0", shaA,
				item("agent:phase-orchestrator", "b.md"), item("schema:tdd", "a.md"))),
			lockOf(sourceOf("shared", "v1.3.0", shaB, item("schema:tdd", "a.md"))),
			[]string{"b.md"},
			map[string]*catalog.Catalog{"shared": provides("schema:tdd")},
		)

		got := onlySource(t, r).Items
		want := []ItemReport{
			{Verb: "removed", ID: "agent:phase-orchestrator", Files: 1, Note: "no longer provided"},
			{Verb: "updated", ID: "schema:tdd", Files: 1},
		}
		if !slices.Equal(got, want) {
			t.Errorf("items = %+v, want %+v", got, want)
		}
	})

	t.Run("source removed", func(t *testing.T) {
		t.Parallel()

		// No catalog for the source: graft.toml no longer declares it, so nothing was
		// fetched for it and there is nothing to ask. That is why the note cannot be one
		// of the other two.
		r := report(
			lockOf(sourceOf("retired", "v1.0.0", shaA,
				item("agent:a", "a.md"), item("agent:b", "b.md", "c.md"))),
			lockOf(),
			[]string{"a.md", "b.md", "c.md"},
			map[string]*catalog.Catalog{},
		)

		got := onlySource(t, r).Items
		want := []ItemReport{
			{Verb: "removed", ID: "agent:a", Files: 1, Note: "source removed"},
			{Verb: "removed", ID: "agent:b", Files: 2, Note: "source removed"},
		}
		if !slices.Equal(got, want) {
			t.Errorf("items = %+v, want %+v", got, want)
		}
	})
}

func TestReportHeaders(t *testing.T) {
	t.Parallel()

	t.Run("both moved", func(t *testing.T) {
		t.Parallel()

		r := report(
			lockOf(sourceOf("shared", "v1.2.0", shaA, item("schema:tdd", "a.md"))),
			lockOf(sourceOf("shared", "v1.3.0", shaB, item("schema:tdd", "a.md"))),
			nil,
			map[string]*catalog.Catalog{"shared": provides("schema:tdd")},
		)

		s := onlySource(t, r)
		if s.PrevRev != "v1.2.0" || s.Rev != "v1.3.0" || s.PrevResolved != shaA || s.Resolved != shaB {
			t.Errorf("header = %+v", s)
		}
	})

	t.Run("a new source", func(t *testing.T) {
		t.Parallel()

		r := report(
			lockOf(),
			lockOf(sourceOf("extra", "v2.0.0", shaA, item("agent:x", "x.md"))),
			nil,
			map[string]*catalog.Catalog{"extra": provides("agent:x")},
		)

		s := onlySource(t, r)
		if s.PrevRev != "" || s.PrevResolved != "" || s.Rev != "v2.0.0" || s.Resolved != shaA {
			t.Errorf("header = %+v, want no previous halves", s)
		}
	})

	t.Run("the sha moved and the rev did not", func(t *testing.T) {
		t.Parallel()

		r := report(
			lockOf(sourceOf("shared", "main", shaA, item("schema:tdd", "a.md"))),
			lockOf(sourceOf("shared", "main", shaB, item("schema:tdd", "a.md"))),
			nil,
			map[string]*catalog.Catalog{"shared": provides("schema:tdd")},
		)

		s := onlySource(t, r)
		if s.PrevRev != "main" || s.Rev != "main" || s.PrevResolved != shaA || s.Resolved != shaB {
			t.Errorf("header = %+v", s)
		}
	})
}

// Sources come from the union of the two locks, in name order. A source dropped from
// graft.toml appears in the old lock only and must still be reported.
func TestReportSourceUnionInNameOrder(t *testing.T) {
	t.Parallel()

	r := report(
		lockOf(sourceOf("zeta", "v1.0.0", shaA, item("agent:z", "z.md"))),
		lockOf(sourceOf("alpha", "v1.0.0", shaB, item("agent:a", "a.md"))),
		[]string{"z.md"},
		map[string]*catalog.Catalog{"alpha": provides("agent:a")},
	)

	if len(r.Sources) != 2 || r.Sources[0].Name != "alpha" || r.Sources[1].Name != "zeta" {
		t.Fatalf("sources = %+v, want alpha then zeta", r.Sources)
	}
	if r.Sources[1].Items[0].Note != "source removed" {
		t.Errorf("zeta's item = %+v", r.Sources[1].Items[0])
	}
}

func TestReportCounts(t *testing.T) {
	t.Parallel()

	r := newReport(
		lockOf(),
		&plan.Plan{
			Writes: []plan.Write{{Dest: "a"}, {Dest: "b"}, {Dest: "c"}},
			Prune:  []string{"x", "y"},
			Lock:   lockOf(sourceOf("shared", "v1", shaA, item("schema:tdd", "a", "b", "c"))),
		},
		map[string]*catalog.Catalog{"shared": provides("schema:tdd")},
		false,
	)

	if r.Written != 3 || r.Removed != 2 {
		t.Errorf("Written, Removed = %d, %d, want 3, 2", r.Written, r.Removed)
	}
}

// Nothing to do is byte equality of the two serialized locks plus an empty prune set: one
// predicate over the two artifacts a reader would diff, rather than a conjunction of the
// conditions that produce report lines.
func TestReportUpToDatePredicate(t *testing.T) {
	t.Parallel()

	same := func() *lock.Lock {
		return lockOf(sourceOf("shared", "v1.0.0", shaA, item("schema:tdd", "a.md")))
	}

	for name, tc := range map[string]struct {
		before, after *lock.Lock
		prune         []string
		want          bool
	}{
		"identical locks, nothing pruned": {same(), same(), nil, true},
		"identical locks, something pruned": {
			same(), same(), []string{"stale.md"}, false,
		},
		"a first sync": {lockOf(), same(), nil, false},
		"a moved pin": {
			same(),
			lockOf(sourceOf("shared", "v1.0.0", shaB, item("schema:tdd", "a.md"))),
			nil, false,
		},
		// The case the report-line conditions miss entirely: same rev, same sha, same
		// files, and a graft.toml whose git value was rewritten. The lock changes and the
		// run is not nothing to do.
		"only the git value changed": {
			same(),
			lockOf(lock.Source{
				Name: "shared", Git: "https://example.com/o/shared", Rev: "v1.0.0",
				Resolved: shaA, Items: []lock.Item{item("schema:tdd", "a.md")},
			}),
			nil, false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := report(tc.before, tc.after, tc.prune,
				map[string]*catalog.Catalog{"shared": provides("schema:tdd")})
			if got := r.UpToDate(); got != tc.want {
				t.Errorf("UpToDate() = %v, want %v", got, tc.want)
			}
		})
	}
}
