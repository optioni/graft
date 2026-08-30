package list_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/list"
	"github.com/optioni/graft/internal/lock"
)

// rangeLock is specLock with its one source pinning a range instead of a ref.
func rangeLock() *lock.Lock {
	l := specLock()
	l.Sources[0].Rev = "^1.2.0"
	l.Sources[0].Matched = "v1.2.0"
	return l
}

// TestLinesRangeSourceNamesTheTagItMatched: the header line gains a column between the
// rev and the sha, present only for a source whose rev is a range.
func TestLinesRangeSourceNamesTheTagItMatched(t *testing.T) {
	t.Parallel()

	got := list.FromLock(rangeLock()).Lines()
	want := "openspec-schemas  ^1.2.0  v1.2.0  (fae2a30)"
	if len(got) == 0 || got[0] != want {
		t.Errorf("header = %q, want %q", got, want)
	}
}

// TestLinesRefSourceHeaderGainsNoColumn: the two headers in one listing are not padded
// into a shared layout — each block is independent, exactly as it is today.
func TestLinesRefSourceHeaderGainsNoColumn(t *testing.T) {
	t.Parallel()

	l := &lock.Lock{Version: lock.Version, Sources: []lock.Source{
		{
			Name: "ranged", Git: "example.test/r", Rev: "^1.2.0", Matched: "v1.2.0",
			Resolved: "aaaaaaa111111111111111111111111111111111",
		},
		{
			Name: "tagged", Git: "example.test/t", Rev: "v1.0.0",
			Resolved: "bbbbbbb222222222222222222222222222222222",
		},
	}}

	// One blank line separates the two header-only blocks; neither header is padded to
	// match the other's width.
	want := []string{
		"ranged  ^1.2.0  v1.2.0  (aaaaaaa)",
		"",
		"tagged  v1.0.0  (bbbbbbb)",
	}
	if got := list.FromLock(l).Lines(); len(got) != 3 || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

// TestJSONRangeSourceCarriesTheTagItMatched: matched sits between rev and resolved on
// every source object, present unconditionally.
func TestJSONRangeSourceCarriesTheTagItMatched(t *testing.T) {
	t.Parallel()

	got := string(list.FromLock(rangeLock()).JSON())
	i := strings.Index(got, `"rev": "^1.2.0"`)
	j := strings.Index(got, `"matched": "v1.2.0"`)
	k := strings.Index(got, `"resolved"`)
	if i < 0 || j < 0 || k < 0 || !(i < j && j < k) {
		t.Errorf("document does not order rev, matched, resolved in that position:\n%s", got)
	}
}

// TestJSONVersionIsTwo: the document's own version moved from 1 to 2, because every
// source object gained a member — a consumer pinned to 1 learns that from the number
// rather than from a decode failure.
func TestJSONVersionIsTwo(t *testing.T) {
	t.Parallel()

	var got list.Listing
	if err := json.Unmarshal(list.FromLock(specLock()).JSON(), &got); err != nil {
		t.Fatalf("decoding the document: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("Version = %d, want 2", got.Version)
	}
}

// TestJSONRefSourceMatchedIsEmptyString: matched is present unconditionally and empty
// for a ref, never omitted — an omitted member would make every consumer branch on
// presence.
func TestJSONRefSourceMatchedIsEmptyString(t *testing.T) {
	t.Parallel()

	got := string(list.FromLock(specLock()).JSON())
	if !strings.Contains(got, `"matched": ""`) {
		t.Errorf("document does not carry an empty matched member for a ref source:\n%s", got)
	}
}
