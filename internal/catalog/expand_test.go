package catalog_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// fixture builds a catalog in memory from item ids. Expansion is a pure function of a
// catalog, a source name, and a selector list, so no test here needs a file: one that
// did would mean the boundary moved.
func fixture(ids ...string) *catalog.Catalog {
	c := &catalog.Catalog{Version: 1, Kinds: map[string]catalog.Kind{}}
	for _, id := range ids {
		kind, name, _ := strings.Cut(id, ":")
		c.Kinds[kind] = catalog.Kind{To: []string{kind + "s/"}}
		c.Items = append(c.Items, catalog.Item{ID: id, Kind: kind, Name: name, From: name + ".md"})
	}
	// Parse guarantees this order; a hand-built catalog has to earn it, or a test
	// could pass for the wrong reason.
	sort.Slice(c.Items, func(i, j int) bool { return c.Items[i].ID < c.Items[j].ID })
	return c
}

func ids(items []catalog.Item) []string {
	got := make([]string, 0, len(items))
	for _, it := range items {
		got = append(got, it.ID)
	}
	return got
}

const source = "openspec-schemas"

func TestExpand(t *testing.T) {
	t.Parallel()

	three := []string{"agent:apply-orchestrator", "agent:tdd-reviewer", "schema:tdd"}

	tests := []struct {
		name      string
		selectors []string
		want      []string
	}{
		{
			name:      "a plain selector selects exactly one item",
			selectors: []string{"schema:tdd"},
			want:      []string{"schema:tdd"},
		},
		{
			// Declared in reverse id order: nothing downstream may depend on how a
			// consumer wrote its install list.
			name:      "several selectors produce the union ordered by id",
			selectors: []string{"schema:tdd", "agent:tdd-reviewer"},
			want:      []string{"agent:tdd-reviewer", "schema:tdd"},
		},
		{
			name:      "an empty selector list expands to nothing",
			selectors: nil,
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := catalog.Expand(fixture(three...), source, tt.selectors)
			if err != nil {
				t.Fatalf("Expand() error = %v, want nil", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Expand() = %q, want %q", ids(got), tt.want)
			}
			if !sameStrings(ids(got), tt.want) {
				t.Errorf("Expand() = %q, want %q", ids(got), tt.want)
			}
		})
	}
}

func TestExpand_Errors(t *testing.T) {
	t.Parallel()

	three := []string{"agent:apply-orchestrator", "agent:tdd-reviewer", "schema:tdd"}

	tests := []struct {
		name      string
		catalog   *catalog.Catalog
		source    string
		selectors []string
		want      string
	}{
		{
			name:      "a misspelled selector lists what the catalog provides",
			catalog:   fixture(three...),
			source:    source,
			selectors: []string{"schema:tdd-workflow"},
			want: `source "openspec-schemas": selector "schema:tdd-workflow" matches no item; ` +
				`catalog provides agent:apply-orchestrator, agent:tdd-reviewer, schema:tdd`,
		},
		{
			name:      "a catalog providing zero items",
			catalog:   fixture(),
			source:    "empty-source",
			selectors: []string{"agent:*"},
			want:      `source "empty-source": selector "agent:*" matches no item; catalog provides no items`,
		},
		{
			name:      "a selector with no colon matches nothing",
			catalog:   fixture(three...),
			source:    source,
			selectors: []string{"tdd"},
			want: `source "openspec-schemas": selector "tdd" matches no item; ` +
				`catalog provides agent:apply-orchestrator, agent:tdd-reviewer, schema:tdd`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := catalog.Expand(tt.catalog, tt.source, tt.selectors)
			if got != nil {
				t.Errorf("Expand() items = %q, want nil", ids(got))
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Expand() error = %v, want %q", err, tt.want)
			}
		})
	}
}
