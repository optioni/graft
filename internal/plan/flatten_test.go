package plan

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

var pack = item("agent:pack", "agent", "pack", "extras/agents/pack")

// TestFlatten_DiscardsNestedStructure and its sibling below are the same item and the
// same listing against the same destination, differing only in the flag. Reading them
// together is what says what flatten does.
func TestFlatten_DiscardsNestedStructure(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
		pack,
		Listing{Dir: true, Files: []string{"review/outside-in.md", "apply/orchestrator.md"}},
		nil,
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	want := []string{".claude/agents/orchestrator.md", ".claude/agents/outside-in.md"}
	if !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

func TestFlatten_WithoutItTheStructureSurvives(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{".claude/agents/"}},
		pack,
		Listing{Dir: true, Files: []string{"review/outside-in.md", "apply/orchestrator.md"}},
		nil,
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	want := []string{".claude/agents/apply/orchestrator.md", ".claude/agents/review/outside-in.md"}
	if !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

// TestFlatten_TwoFilesOntoOnePathIsAnError pins the within-item message. It names the
// two from-relative paths, which is the information needed to fix the catalog; the
// cross-item collision message would print one item id twice and name no cause.
func TestFlatten_TwoFilesOntoOnePathIsAnError(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
		pack,
		Listing{Dir: true, Files: []string{"b/dup.md", "a/dup.md"}},
		nil,
	)
	want := `source "shared": item "agent:pack": flatten maps "a/dup.md" and "b/dup.md" to the same destination ".claude/agents/dup.md"`
	assertWithinItemError(t, got, err, want)
}

func TestDestination_AListValuedToPlacesTheItemTwice(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"openspec/schemas/{name}", "vendor/schemas/{name}"}},
		item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		nil,
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	want := []string{"openspec/schemas/tdd/schema.yaml", "vendor/schemas/tdd/schema.yaml"}
	if !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

func TestDestination_TwoEntriesInterpolatingAlikeIsAnError(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"openspec/schemas/{name}", "openspec/schemas/tdd"}},
		item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		nil,
	)
	want := `source "shared": item "schema:tdd": destinations "openspec/schemas/{name}" and "openspec/schemas/tdd" both interpolate to "openspec/schemas/tdd"`
	assertWithinItemError(t, got, err, want)
}

// assertWithinItemError checks the exact message and, separately, that it is not the
// cross-item collision message. One item colliding with itself is not two items
// sharing a path, and reporting it as one would name the item as its own partner.
func assertWithinItemError(t *testing.T, got []placement, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("destinations: want error %q, got %q", want, dests(got))
	}
	if err.Error() != want {
		t.Errorf("destinations:\n got %q\nwant %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), "both resolve to") {
		t.Errorf("destinations: reported a within-item collision with the cross-item message: %q", err.Error())
	}
	if got != nil {
		t.Errorf("destinations: want no placements on error, got %q", dests(got))
	}
}
