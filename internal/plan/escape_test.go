package plan

import (
	"slices"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// TestEscape_RefusesADestinationOutsideTheRepo pins SPEC.md's invariant and its exact
// wording. The message is a contract: changing it is a deliberate contract change, not
// a rewording.
func TestEscape_RefusesADestinationOutsideTheRepo(t *testing.T) {
	tests := []struct {
		name    string
		kind    catalog.Kind
		item    catalog.Item
		listing Listing
		want    string
	}{
		{
			name:    "a to climbing out of the repo",
			kind:    catalog.Kind{To: []string{"../outside/agents/"}},
			item:    item("agent:x", "agent", "x", "extras/agents/x.md"),
			listing: Listing{Files: []string{"x.md"}},
			want:    `source "shared": item "agent:x": destination "../outside/agents/" escapes the repo root`,
		},
		{
			name:    "an absolute to",
			kind:    catalog.Kind{To: []string{"/etc/agents/"}},
			item:    item("agent:x", "agent", "x", "extras/agents/x.md"),
			listing: Listing{Files: []string{"x.md"}},
			want:    `source "shared": item "agent:x": destination "/etc/agents/" escapes the repo root`,
		},
		{
			// The check does not depend on the item happening to contribute a file:
			// an unusable destination is refused before anything is mapped under it.
			name:    "a to escaping with no files to place",
			kind:    catalog.Kind{To: []string{"../outside/agents/"}},
			item:    item("agent:x", "agent", "x", "extras/agents/x.md"),
			listing: Listing{},
			want:    `source "shared": item "agent:x": destination "../outside/agents/" escapes the repo root`,
		},
		{
			// A malformed listing must not be able to aim a write outside the tree by
			// climbing out of its own item's destination.
			name:    "a listing entry climbing out of its item",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
			listing: Listing{Dir: true, Files: []string{"../../../../../etc/passwd"}},
			want:    `source "shared": item "schema:tdd": destination "../../etc/passwd" escapes the repo root`,
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

// TestEscape_AcceptsTheRepoRootItself keeps the boundary from being drawn one level in.
// The top of the repository is inside it.
func TestEscape_AcceptsTheRepoRootItself(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"{name}.md"}},
		item("doc:README", "doc", "README", "extras/docs/readme.md"),
		Listing{Files: []string{"readme.md"}},
		nil,
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	if want := []string{"README.md"}; !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}
