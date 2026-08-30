package sync

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

// rangeSourceOf is sourceOf with a matched tag, for the report tests that need one.
func rangeSourceOf(name, rev, matched, resolved string, items ...lock.Item) lock.Source {
	s := sourceOf(name, rev, resolved, items...)
	s.Matched = matched
	return s
}

// TestReportMatchedTagMovedOntoSameCommitStillGetsHeader: report.go's per-source skip
// compares only rev and resolved, so a retag onto the same commit changes the lock but,
// left unfixed, would print a summary with no block explaining it.
func TestReportMatchedTagMovedOntoSameCommitStillGetsHeader(t *testing.T) {
	t.Parallel()

	before := lockOf(rangeSourceOf("shared", "^1.2.0", "v1.2.0", shaA, item("schema:tdd", "a.md")))
	after := lockOf(rangeSourceOf("shared", "^1.2.0", "v1.3.0", shaA, item("schema:tdd", "a.md")))

	r := report(before, after, nil, map[string]*catalog.Catalog{"shared": provides("schema:tdd")})
	if len(r.Sources) != 1 {
		t.Fatalf("sources = %+v, want exactly one: a matched-tag move must not be skipped as nothing to report", r.Sources)
	}
}
