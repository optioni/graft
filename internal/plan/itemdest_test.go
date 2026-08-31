package plan

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/manifest"
)

// `graft add --list` shows what a source offers and where each item would land, before
// anything is written. The destination is what a consumer actually agrees to, so it is
// computed by the same rules a sync computes it by — one item at a time, and still with
// no filesystem access anywhere in this package.

// computeItem runs item-level destination computation for a single item against a single
// kind, with everything in memory exactly as compute does.
func computeItem(kind catalog.Kind, it catalog.Item, l Listing, overrides map[string]string) ([]string, error) {
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
	return ItemDestinations(in, it)
}

func TestItemDestinationsNamesAFileItemsFile(t *testing.T) {
	t.Parallel()

	got, err := computeItem(
		catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
		item("agent:reviewer", "agent", "reviewer", "extras/agents/reviewer.md"),
		Listing{Files: []string{"reviewer.md"}},
		nil,
	)
	if err != nil {
		t.Fatalf("ItemDestinations: %v", err)
	}
	if want := []string{".claude/agents/reviewer.md"}; !slices.Equal(got, want) {
		t.Errorf("destinations = %q, want %q", got, want)
	}
}

// A directory item fills a directory, and printing the files inside it would be printing
// the source's own layout — the thing an item id exists to keep out of a consumer's view.
func TestItemDestinationsNamesADirectoryItemsDirectory(t *testing.T) {
	t.Parallel()

	got, err := computeItem(
		catalog.Kind{To: []string{"openspec/schemas/{name}"}},
		item("schema:tdd", "schema", "tdd", "extras/schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml", "templates/design.md"}},
		nil,
	)
	if err != nil {
		t.Fatalf("ItemDestinations: %v", err)
	}
	if want := []string{"openspec/schemas/tdd/"}; !slices.Equal(got, want) {
		t.Errorf("destinations = %q, want %q", got, want)
	}
}

func TestItemDestinationsHonoursAConsumerOverride(t *testing.T) {
	t.Parallel()

	got, err := computeItem(
		catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
		item("agent:reviewer", "agent", "reviewer", "extras/agents/reviewer.md"),
		Listing{Files: []string{"reviewer.md"}},
		map[string]string{"agent": ".codex/agents/"},
	)
	if err != nil {
		t.Fatalf("ItemDestinations: %v", err)
	}
	if want := []string{".codex/agents/reviewer.md"}; !slices.Equal(got, want) {
		t.Errorf("destinations = %q, want %q", got, want)
	}
}

func TestItemDestinationsNamesEveryDestinationOfAListValuedTo(t *testing.T) {
	t.Parallel()

	got, err := computeItem(
		catalog.Kind{To: []string{"openspec/schemas/{name}", "vendor/schemas/{name}"}},
		item("schema:tdd", "schema", "tdd", "extras/schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		nil,
	)
	if err != nil {
		t.Fatalf("ItemDestinations: %v", err)
	}
	want := []string{"openspec/schemas/tdd/", "vendor/schemas/tdd/"}
	if !slices.Equal(got, want) {
		t.Errorf("destinations = %q, want %q", got, want)
	}
}

func TestItemDestinationsRefusesADestinationOutsideTheRepo(t *testing.T) {
	t.Parallel()

	got, err := computeItem(
		catalog.Kind{To: []string{"../outside/{name}"}},
		item("schema:tdd", "schema", "tdd", "extras/schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		nil,
	)
	if err == nil {
		t.Fatalf("ItemDestinations = %q, want a refusal", got)
	}
	if !strings.Contains(err.Error(), "escapes the repo root") {
		t.Errorf("error = %q, want it to name the escape", err)
	}
	if got != nil {
		t.Errorf("destinations returned beside an error: %q", got)
	}
}

// The two entries of a list-valued `to` that interpolate to one path are refused here for
// the same reason they are refused in a plan: a listing that names one destination twice
// would show a consumer one item landing in two places that are one place.
func TestItemDestinationsRefusesTwoEntriesThatCollapse(t *testing.T) {
	t.Parallel()

	_, err := computeItem(
		catalog.Kind{To: []string{"openspec/schemas/{name}", "openspec/schemas/tdd"}},
		item("schema:tdd", "schema", "tdd", "extras/schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		nil,
	)
	if err == nil {
		t.Fatal("ItemDestinations succeeded on two entries interpolating to one path")
	}
	if !strings.Contains(err.Error(), "both interpolate to") {
		t.Errorf("error = %q, want it to name the collapse", err)
	}
}
