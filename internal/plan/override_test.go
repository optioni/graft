package plan

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/manifest"
)

// sourceInput assembles one source's inputs under a chosen name, so a test can hold
// two sources side by side and see that an override belongs to exactly one of them.
func sourceInput(name string, kinds map[string]catalog.Kind, items []catalog.Item, listings map[string]Listing, overrides map[string]string) Input {
	install := make([]string, 0, len(items))
	for _, it := range items {
		install = append(install, it.ID)
	}
	return Input{
		Source: manifest.Source{
			Name:    name,
			Git:     "github.com/optioni/" + name,
			Rev:     "v1.0.0",
			Install: install,
			Kinds:   overrides,
		},
		Resolved: "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
		Catalog:  &catalog.Catalog{Version: catalog.Version, Kinds: kinds, Items: items},
		Items:    listings,
	}
}

func TestOverride_MovesAKindsItems(t *testing.T) {
	orchestrator := item("agent:apply-orchestrator", "agent", "apply-orchestrator", "extras/agents/apply-orchestrator.md")

	got, err := compute(
		catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
		orchestrator,
		Listing{Files: []string{"apply-orchestrator.md"}},
		map[string]string{"agent": ".codex/agents/"},
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	want := []string{".codex/agents/apply-orchestrator.md"}
	if !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
	// The destination is what a consumer actually agreed to, so the catalog's own
	// proposal must not survive alongside the override.
	for _, d := range dests(got) {
		if strings.HasPrefix(d, ".claude/") {
			t.Errorf("destinations: the catalog's destination survived the override: %q", d)
		}
	}
}

func TestOverride_ReplacesAListValuedDestinationEntirely(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{"openspec/schemas/{name}", "vendor/schemas/{name}"}},
		item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd"),
		Listing{Dir: true, Files: []string{"schema.yaml"}},
		map[string]string{"schema": "openspec/schemas/{name}"},
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	want := []string{"openspec/schemas/tdd/schema.yaml"}
	if !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

// TestOverride_KeepsTheCatalogsFlatten: graft.toml declares a destination and nothing
// else, so flatten stays the catalog's to state.
func TestOverride_KeepsTheCatalogsFlatten(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{".claude/agents/"}, Flatten: true},
		pack,
		Listing{Dir: true, Files: []string{"review/outside-in.md"}},
		map[string]string{"agent": ".codex/agents/"},
	)
	if err != nil {
		t.Fatalf("destinations: unexpected error: %v", err)
	}
	want := []string{".codex/agents/outside-in.md"}
	if !slices.Equal(dests(got), want) {
		t.Errorf("destinations:\n got %q\nwant %q", dests(got), want)
	}
}

// TestOverride_AppliesToItsOwnSourceOnly holds two sources that provide the same item
// from identical catalogs, and overrides one of them.
func TestOverride_AppliesToItsOwnSourceOnly(t *testing.T) {
	x := item("agent:x", "agent", "x", "extras/agents/x.md")
	kinds := map[string]catalog.Kind{"agent": {To: []string{".claude/agents/"}}}
	listings := map[string]Listing{x.ID: {Files: []string{"x.md"}}}

	shared := sourceInput("shared", kinds, []catalog.Item{x}, listings, map[string]string{"agent": ".codex/agents/"})
	other := sourceInput("other", kinds, []catalog.Item{x}, listings, nil)

	got, err := destinations(shared, x)
	if err != nil {
		t.Fatalf("shared: unexpected error: %v", err)
	}
	if want := []string{".codex/agents/x.md"}; !slices.Equal(dests(got), want) {
		t.Errorf("shared:\n got %q\nwant %q", dests(got), want)
	}

	got, err = destinations(other, x)
	if err != nil {
		t.Fatalf("other: unexpected error: %v", err)
	}
	if want := []string{".claude/agents/x.md"}; !slices.Equal(dests(got), want) {
		t.Errorf("other:\n got %q\nwant %q", dests(got), want)
	}
}

// TestOverride_AnEscapingOverrideIsRefused: the override being the consumer's own
// words does not exempt it. The repo root is the boundary, not the source.
func TestOverride_AnEscapingOverrideIsRefused(t *testing.T) {
	got, err := compute(
		catalog.Kind{To: []string{".claude/agents/"}},
		item("agent:x", "agent", "x", "extras/agents/x.md"),
		Listing{Files: []string{"x.md"}},
		map[string]string{"agent": "../../elsewhere/"},
	)
	want := `source "shared": item "agent:x": destination "../../elsewhere/" escapes the repo root`
	if err == nil {
		t.Fatalf("destinations: want error %q, got %q", want, dests(got))
	}
	if err.Error() != want {
		t.Errorf("destinations:\n got %q\nwant %q", err.Error(), want)
	}
	if got != nil {
		t.Errorf("destinations: want no placements on error, got %q", dests(got))
	}
}

// TestOverride_AnUndeclaredKindIsAnError is typo protection: an override that silently
// did nothing would leave files at the catalog's destination while the manifest claims
// they moved. The lowest-sorting offender is reported, so the message never depends on
// map iteration order.
func TestOverride_AnUndeclaredKindIsAnError(t *testing.T) {
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{
			"agent":  {To: []string{".claude/agents/"}},
			"schema": {To: []string{"openspec/schemas/{name}"}},
		},
		nil, nil,
		map[string]string{"agnet": ".codex/agents/", "shcema": "vendor/{name}"},
	)

	err := checkOverrides(in)
	want := `source "shared": kind override "agnet" names a kind the catalog does not declare`
	if err == nil {
		t.Fatalf("checkOverrides: want error %q, got nil", want)
	}
	if err.Error() != want {
		t.Errorf("checkOverrides:\n got %q\nwant %q", err.Error(), want)
	}
}

// TestOverride_ADeclaredKindWithNoItemsIsFine: the kind exists, so it is not a typo.
// Overriding `hook` before installing any hook is legitimate (design.md Q3).
func TestOverride_ADeclaredKindWithNoItemsIsFine(t *testing.T) {
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"hook": {To: []string{".githooks/"}}},
		nil, nil,
		map[string]string{"hook": ".git/hooks/"},
	)
	if err := checkOverrides(in); err != nil {
		t.Errorf("checkOverrides: unexpected error: %v", err)
	}
}
