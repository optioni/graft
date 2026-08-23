package plan

import (
	"slices"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/manifest"
)

// item is one provides entry, written the way a catalog would.
func item(id, kind, name, from string) catalog.Item {
	return catalog.Item{ID: id, Kind: kind, Name: name, From: from}
}

// compute runs destination computation for a single item against a single kind. Every
// input is an in-memory value: a catalog literal, a manifest source literal, and a
// listing literal. Nothing here reads a directory, because reading one would mean the
// boundary this package exists to hold has moved.
func compute(kind catalog.Kind, it catalog.Item, l Listing, overrides map[string]string) ([]placement, error) {
	in := Input{
		Source: manifest.Source{
			Name:    "shared",
			Git:     "github.com/optioni/shared",
			Rev:     "v1.0.0",
			Install: []string{it.ID},
			Kinds:   overrides,
		},
		Resolved: "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
		Catalog: &catalog.Catalog{
			Version: catalog.Version,
			Kinds:   map[string]catalog.Kind{it.Kind: kind},
			Items:   []catalog.Item{it},
		},
		Items: map[string]Listing{it.ID: l},
	}
	return destinations(in, it)
}

func dests(places []placement) []string {
	out := make([]string, 0, len(places))
	for _, p := range places {
		out = append(out, p.Dest)
	}
	return out
}

func TestDestination(t *testing.T) {
	tests := []struct {
		name    string
		kind    catalog.Kind
		item    catalog.Item
		listing Listing
		want    []string
	}{
		{
			// "Computing destinations touches nothing" — the table half. The other
			// half is TestPackageImportsNothingImpure.
			name:    "an interpolated destination places a directory item's files",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
			listing: Listing{Dir: true, Files: []string{"schema.yaml", "templates/design.md"}},
			want:    []string{"openspec/schemas/tdd/schema.yaml", "openspec/schemas/tdd/templates/design.md"},
		},
		{
			name:    "a directory item preserves its structure",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
			listing: Listing{Dir: true, Files: []string{"schema.yaml", "templates/design.md", "templates/proposal.md"}},
			want: []string{
				"openspec/schemas/tdd/schema.yaml",
				"openspec/schemas/tdd/templates/design.md",
				"openspec/schemas/tdd/templates/proposal.md",
			},
		},
		{
			name:    "a trailing slash places a file item inside the directory",
			kind:    catalog.Kind{To: []string{".claude/agents/"}},
			item:    item("agent:apply-orchestrator", "agent", "apply-orchestrator", "extras/agents/apply-orchestrator.md"),
			listing: Listing{Files: []string{"apply-orchestrator.md"}},
			want:    []string{".claude/agents/apply-orchestrator.md"},
		},
		{
			name:    "without a trailing slash a file item lands at the destination itself",
			kind:    catalog.Kind{To: []string{"docs/{name}.md"}},
			item:    item("command:release", "command", "release", "extras/commands/release-notes.md"),
			listing: Listing{Files: []string{"release-notes.md"}},
			want:    []string{"docs/release.md"},
		},
		{
			name:    "a trailing slash is a no-op for a directory item",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}/"}},
			item:    item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
			listing: Listing{Dir: true, Files: []string{"schema.yaml"}},
			want:    []string{"openspec/schemas/tdd/schema.yaml"},
		},
		{
			name:    "a destination with no {name} takes the file's own base name",
			kind:    catalog.Kind{To: []string{".claude/agents/"}},
			item:    item("agent:one", "agent", "one", "extras/agents/one.md"),
			listing: Listing{Files: []string{"one.md"}},
			want:    []string{".claude/agents/one.md"},
		},
		{
			name:    "a second item of the same kind takes its own base name",
			kind:    catalog.Kind{To: []string{".claude/agents/"}},
			item:    item("agent:two", "agent", "two", "extras/agents/two.md"),
			listing: Listing{Files: []string{"two.md"}},
			want:    []string{".claude/agents/two.md"},
		},
		{
			name:    "an item contributing no files computes no destinations",
			kind:    catalog.Kind{To: []string{"openspec/schemas/{name}"}},
			item:    item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
			listing: Listing{Dir: true},
			want:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := compute(tc.kind, tc.item, tc.listing, nil)
			if err != nil {
				t.Fatalf("destinations: unexpected error: %v", err)
			}
			if !slices.Equal(dests(got), tc.want) {
				t.Errorf("destinations:\n got %q\nwant %q", dests(got), tc.want)
			}
		})
	}
}

// TestDestination_TrailingSlashIsANoOpForADirectoryItem states design.md D4 as an
// equality rather than as two independent expectations: the slashed and slashless
// spellings must agree, or SPEC.md's own example grows a repeated tdd/tdd segment.
func TestDestination_TrailingSlashIsANoOpForADirectoryItem(t *testing.T) {
	it := item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd")
	l := Listing{Dir: true, Files: []string{"schema.yaml", "templates/design.md"}}

	with, err := compute(catalog.Kind{To: []string{"openspec/schemas/{name}/"}}, it, l, nil)
	if err != nil {
		t.Fatalf("with a trailing slash: %v", err)
	}
	without, err := compute(catalog.Kind{To: []string{"openspec/schemas/{name}"}}, it, l, nil)
	if err != nil {
		t.Fatalf("without a trailing slash: %v", err)
	}
	if !slices.Equal(dests(with), dests(without)) {
		t.Errorf("a trailing slash changed a directory item's destinations:\n with %q\nwithout %q",
			dests(with), dests(without))
	}
}

// TestDestination_NoDestinationMentionsFrom pins SPEC.md's from-mobility claim: a
// source may move an item's from without moving any consumer's files.
func TestDestination_NoDestinationMentionsFrom(t *testing.T) {
	here := item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd")
	moved := item("schema:tdd", "schema", "tdd", "packages/schemas/tdd")
	l := Listing{Dir: true, Files: []string{"schema.yaml", "templates/design.md"}}
	kind := catalog.Kind{To: []string{"openspec/schemas/{name}"}}

	before, err := compute(kind, here, l, nil)
	if err != nil {
		t.Fatalf("before the move: %v", err)
	}
	after, err := compute(kind, moved, l, nil)
	if err != nil {
		t.Fatalf("after the move: %v", err)
	}
	if !slices.Equal(dests(before), dests(after)) {
		t.Errorf("moving from changed the destinations:\nbefore %q\n after %q", dests(before), dests(after))
	}
}
