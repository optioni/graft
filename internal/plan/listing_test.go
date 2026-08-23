package plan

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// TestListing_RefusesAnEntryOutsideItsItem: the repo-root boundary is not enough on its
// own. Joining absorbs leading ".." segments, so an entry with fewer of them than its
// destination has segments lands somewhere else *inside* the repo — every repo-root
// check still passing — and the same entry is joined to `from` to name the file to read.
func TestListing_RefusesAnEntryOutsideItsItem(t *testing.T) {
	tests := []struct {
		name    string
		kind    catalog.Kind
		item    catalog.Item
		listing Listing
		want    string
	}{
		{
			name:    "an entry climbing out of its item",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    tdd,
			listing: Listing{Dir: true, Files: []string{"../../../etc/passwd"}},
			want:    `source "shared": item "schema:tdd": file "../../../etc/passwd" is not a relative path inside the item`,
		},
		{
			// The catalog's `to` never names .git, and does not have to: the entry
			// reaches it on its own. Whether a destination may *name* .git is a
			// separate question this change deliberately leaves open.
			name:    "an entry reaching .git under an unrelated destination",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    tdd,
			listing: Listing{Dir: true, Files: []string{"../../../.git/hooks/pre-commit"}},
			want:    `source "shared": item "schema:tdd": file "../../../.git/hooks/pre-commit" is not a relative path inside the item`,
		},
		{
			// Never leaves the repo, and still refused: the consumer agreed to
			// .claude/agents/, not to .claude/hooks/. SPEC.md's mitigation for an
			// untrusted source is that the destination is shown before install, and a
			// file landing somewhere else defeats exactly that.
			name:    "an entry climbing into a sibling directory",
			kind:    catalog.Kind{To: []string{".claude/agents/"}},
			item:    pack,
			listing: Listing{Dir: true, Files: []string{"../hooks/x.md"}},
			want:    `source "shared": item "agent:pack": file "../hooks/x.md" is not a relative path inside the item`,
		},
		{
			// flatten reduces every path to its base name, so the destination is
			// contained no matter what the listing said. The file graft would read is
			// not, and that is the half a destination check cannot see.
			name:    "a flattened entry whose source path leaves the item",
			kind:    catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
			item:    pack,
			listing: Listing{Dir: true, Files: []string{"../../secret.md"}},
			want:    `source "shared": item "agent:pack": file "../../secret.md" is not a relative path inside the item`,
		},
		{
			name:    "an absolute entry",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    tdd,
			listing: Listing{Dir: true, Files: []string{"/etc/passwd"}},
			want:    `source "shared": item "schema:tdd": file "/etc/passwd" is not a relative path inside the item`,
		},
		{
			name:    "an uncleaned entry",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    tdd,
			listing: Listing{Dir: true, Files: []string{"./schema.yaml"}},
			want:    `source "shared": item "schema:tdd": file "./schema.yaml" is not a relative path inside the item`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := compute(tc.kind, tc.item, tc.listing, nil)
			if err == nil {
				t.Fatalf("destinations: want error %q, got %q", tc.want, dests(got))
			}
			if err.Error() != tc.want {
				t.Errorf("destinations:\n got %q\nwant %q", err.Error(), tc.want)
			}
			if got != nil {
				t.Errorf("destinations: want no placements on error, got %q", dests(got))
			}
		})
	}
}

// TestListing_AnEntryClimbingOutOfFromNamesNoFileToRead is the read side. The
// destination stays inside the repo, so every repo-root check passes; what escapes is
// the path internal/apply would copy *from*, which nothing downstream re-checks.
func TestListing_AnEntryClimbingOutOfFromNamesNoFileToRead(t *testing.T) {
	leaky := item("schema:leaky", "schema", "leaky", "a")
	got, err := compute(
		catalog.Kind{To: []string{"w/x/y/z"}},
		leaky,
		Listing{Dir: true, Files: []string{"../../secrets/id_rsa"}},
		nil,
	)
	want := `source "shared": item "schema:leaky": file "../../secrets/id_rsa" is not a relative path inside the item`
	if err == nil {
		t.Fatalf("destinations: want error %q, got %+v", want, got)
	}
	if err.Error() != want {
		t.Errorf("destinations:\n got %q\nwant %q", err.Error(), want)
	}
	for _, p := range got {
		if strings.Contains(p.From, "..") {
			t.Errorf("destinations: planned a read outside the source tree: %q", p.From)
		}
	}
}

// TestListing_ARepeatedEntryIsOneWrite: a duplicated listing entry names one file, so
// it is one write rather than an item colliding with itself.
func TestListing_ARepeatedEntryIsOneWrite(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"openspec/schemas/{name}"}},
		tdd,
		Listing{Dir: true, Files: []string{"schema.yaml", "schema.yaml"}},
		nil,
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	if want := []string{"openspec/schemas/tdd/schema.yaml"}; !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

// TestDestination_TrailingSlashAliasesAreOneDestination: by D4 a trailing slash is a
// no-op for a directory item, so these two entries are the same destination and must be
// refused by the rule that exists for it — not by the cross-item collision check, which
// would name the item as its own partner.
func TestDestination_TrailingSlashAliasesAreOneDestination(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"a/{name}", "a/{name}/"}},
		tdd,
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		nil,
	)
	assertWithinItemError(t, got, err,
		`source "shared": item "schema:tdd": destinations "a/{name}" and "a/{name}/" both interpolate to "a/tdd"`)
}

// TestDestination_TrailingSlashAliasesAreTwoDestinationsForAFileItem: the same pair is
// genuinely two destinations here — one names the file, the other a directory to put it
// in — so it must not be refused.
func TestDestination_TrailingSlashAliasesAreTwoDestinationsForAFileItem(t *testing.T) {
	notes := item("doc:notes", "doc", "notes", "extras/notes.md")
	got, err := compute(
		catalog.Kind{To: []string{"docs/{name}", "docs/{name}/"}},
		notes,
		Listing{Files: []string{"notes.md"}},
		nil,
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	if want := []string{"docs/notes", "docs/notes/notes.md"}; !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

// TestDestination_OneItemPlacingAFileTwiceIsNotACrossItemCollision closes the last route
// by which an item could become its own collision partner: two `to` entries that are
// different destinations, one nested in the other, meeting on a file.
func TestDestination_OneItemPlacingAFileTwiceIsNotACrossItemCollision(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"a", "a/b"}},
		tdd,
		Listing{Dir: true, Files: []string{"b/x.md", "x.md"}},
		nil,
	)
	assertWithinItemError(t, got, err,
		`source "shared": item "schema:tdd": destinations "a" and "a/b" both place a file at "a/b/x.md"`)
}
