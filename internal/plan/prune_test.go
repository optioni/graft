package plan

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

// lockOf builds the lock a previous sync would have left behind.
func lockOf(sources ...lock.Source) *lock.Lock {
	return &lock.Lock{Version: lock.Version, Sources: sources}
}

func lockSource(name string, items ...lock.Item) lock.Source {
	return lock.Source{
		Name:     name,
		Git:      "github.com/optioni/" + name,
		Rev:      "v1.0.0",
		Resolved: resolvedSHA,
		Items:    items,
	}
}

func lockItem(id string, files ...string) lock.Item {
	return lock.Item{ID: id, Files: files}
}

var (
	agentX = item("agent:x", "agent", "x", "extras/agents/x.md")
	agentY = item("agent:y", "agent", "y", "extras/agents/y.md")
)

// agentSource is one source publishing file items under .claude/agents/.
func agentSource(name string, overrides map[string]string, items ...catalog.Item) Input {
	listings := make(map[string]Listing, len(items))
	for _, it := range items {
		listings[it.ID] = Listing{Files: []string{it.Name + ".md"}}
	}
	return sourceInput(name, map[string]catalog.Kind{"agent": agentKind}, items, listings, overrides)
}

func prune(t *testing.T, inputs []Input, lk *lock.Lock) *Plan {
	t.Helper()

	p, err := Build(inputs, lk)
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	return p
}

// TestPrune_AForeignFileIsNeverPruned is the concentration point of this change.
// .claude/agents/ holds repo-owned agents beside synced ones, and the lock's per-item
// file list exists solely to make removal safe: a path absent from the lock is
// invisible to graft and can never be deleted by it.
//
// The assertion is that the foreign path appears in NO field of the plan — a test
// checking only Prune would stay green if the path leaked into a write or into the
// next lock — and it is made twice: with the synced item kept, and with it dropped.
func TestPrune_AForeignFileIsNeverPruned(t *testing.T) {
	const foreign = ".claude/agents/local-reviewer.md"

	lk := lockOf(lockSource("shared", lockItem("agent:apply-orchestrator", ".claude/agents/apply-orchestrator.md")))
	installed := sourceInput(
		"shared",
		map[string]catalog.Kind{"agent": agentKind},
		[]catalog.Item{orchestrator},
		map[string]Listing{orchestrator.ID: {Files: []string{"apply-orchestrator.md"}}},
		nil,
	)

	t.Run("with the item kept", func(t *testing.T) {
		p := prune(t, []Input{installed}, lk)
		if len(p.Prune) != 0 {
			t.Errorf("Prune: want nothing pruned, got %q", p.Prune)
		}
		assertAbsentFromPlan(t, p, foreign)
	})

	t.Run("with the item dropped", func(t *testing.T) {
		p := prune(t, nil, lk)
		want := []string{".claude/agents/apply-orchestrator.md"}
		if !slices.Equal(p.Prune, want) {
			t.Errorf("Prune:\n got %q\nwant %q", p.Prune, want)
		}
		assertAbsentFromPlan(t, p, foreign)
	})
}

func TestPrune_AnItemDroppedFromInstallHasItsFilesPruned(t *testing.T) {
	lk := lockOf(lockSource("shared", lockItem("schema:tdd",
		"openspec/schemas/tdd/schema.yaml",
		"openspec/schemas/tdd/templates/design.md",
	)))
	in := withInstall(sourceInput(
		"shared",
		map[string]catalog.Kind{"schema": schemaKind, "agent": agentKind},
		[]catalog.Item{agentX, tdd},
		map[string]Listing{agentX.ID: {Files: []string{"x.md"}}},
		nil,
	), "agent:x")

	p := prune(t, []Input{in}, lk)
	want := []string{"openspec/schemas/tdd/schema.yaml", "openspec/schemas/tdd/templates/design.md"}
	if !slices.Equal(p.Prune, want) {
		t.Errorf("Prune:\n got %q\nwant %q", p.Prune, want)
	}
	for _, s := range p.Lock.Sources {
		for _, it := range s.Items {
			if it.ID == "schema:tdd" {
				t.Errorf("next lock: still records the dropped item %q", it.ID)
			}
		}
	}
}

// TestPrune_AnItemTheSourceStoppedProvidingIsPruned: agent:* still matches another
// item, so no no-match error is raised, and the item that vanished from the catalog is
// removed rather than silently left behind.
func TestPrune_AnItemTheSourceStoppedProvidingIsPruned(t *testing.T) {
	lk := lockOf(lockSource("shared", lockItem("agent:phase-orchestrator", ".claude/agents/phase-orchestrator.md")))
	in := withInstall(agentSource("shared", nil, orchestrator), "agent:*")

	p := prune(t, []Input{in}, lk)
	if want := []string{".claude/agents/phase-orchestrator.md"}; !slices.Equal(p.Prune, want) {
		t.Errorf("Prune:\n got %q\nwant %q", p.Prune, want)
	}
}

func TestPrune_ASourceRemovedFromTheManifestIsPrunedEntirely(t *testing.T) {
	lk := lockOf(lockSource("retired",
		lockItem("agent:x", ".claude/agents/x.md"),
		lockItem("schema:tdd", "openspec/schemas/tdd/schema.yaml", "openspec/schemas/tdd/templates/design.md"),
	))

	p := prune(t, nil, lk)
	want := []string{
		".claude/agents/x.md",
		"openspec/schemas/tdd/schema.yaml",
		"openspec/schemas/tdd/templates/design.md",
	}
	if !slices.Equal(p.Prune, want) {
		t.Errorf("Prune:\n got %q\nwant %q", p.Prune, want)
	}
	if len(p.Lock.Sources) != 0 {
		t.Errorf("next lock: want no sources, got %d", len(p.Lock.Sources))
	}
}

func TestPrune_AMovedDestinationPrunesTheOldPath(t *testing.T) {
	lk := lockOf(lockSource("shared", lockItem("agent:x", ".claude/agents/x.md")))
	in := agentSource("shared", map[string]string{"agent": ".codex/agents/"}, agentX)

	p := prune(t, []Input{in}, lk)
	if want := []string{".claude/agents/x.md"}; !slices.Equal(p.Prune, want) {
		t.Errorf("Prune:\n got %q\nwant %q", p.Prune, want)
	}
	assertWrites(t, p, ".codex/agents/x.md")
	assertLockRecords(t, p, "shared", "agent:x", []string{".codex/agents/x.md"})
}

func TestPrune_AVersionBumpAddsAndRemoves(t *testing.T) {
	lk := lockOf(lockSource("shared", lockItem("schema:tdd",
		"openspec/schemas/tdd/schema.yaml",
		"openspec/schemas/tdd/templates/old.md",
	)))
	in := schemaSource(Listing{Dir: true, Files: []string{"schema.yaml", "templates/new.md"}})

	p := prune(t, []Input{in}, lk)
	if want := []string{"openspec/schemas/tdd/templates/old.md"}; !slices.Equal(p.Prune, want) {
		t.Errorf("Prune:\n got %q\nwant %q", p.Prune, want)
	}
	assertWrites(t, p, "openspec/schemas/tdd/schema.yaml", "openspec/schemas/tdd/templates/new.md")
}

// TestPrune_APathMovingBetweenSourcesIsWrittenNotPruned: the prune set is a set
// difference over paths, not per-source bookkeeping. The obvious per-source
// implementation — diff each source's lock entry against its own new files — deletes a
// file it has just written.
func TestPrune_APathMovingBetweenSourcesIsWrittenNotPruned(t *testing.T) {
	lk := lockOf(lockSource("alpha", lockItem("agent:x", ".claude/agents/x.md")))
	alpha := withInstall(agentSource("alpha", nil, agentX, agentY), "agent:y")
	beta := agentSource("beta", nil, agentX)

	p := prune(t, []Input{alpha, beta}, lk)
	if len(p.Prune) != 0 {
		t.Errorf("Prune: want nothing pruned, got %q", p.Prune)
	}
	assertWrites(t, p, ".claude/agents/x.md", ".claude/agents/y.md")
	for _, w := range p.Writes {
		if w.Dest == ".claude/agents/x.md" && w.Source != "beta" {
			t.Errorf("Writes: %q should now come from beta, got %q", w.Dest, w.Source)
		}
	}
}

// TestPrune_AFileAlreadyInTheTreeIsStillWritten: a plan carries no notion of
// "unchanged". Synced files are derived artifacts and a plan has no file content to
// compare, so a hand-edit is silently overwritten and git diff is the report. The tree
// is deliberately absent from this test — that it cannot matter is the property.
func TestPrune_AFileAlreadyInTheTreeIsStillWritten(t *testing.T) {
	lk := lockOf(lockSource("shared", lockItem("schema:tdd", "openspec/schemas/tdd/schema.yaml")))

	p := prune(t, []Input{schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})}, lk)
	assertWrites(t, p, "openspec/schemas/tdd/schema.yaml")
	if len(p.Prune) != 0 {
		t.Errorf("Prune: want nothing pruned, got %q", p.Prune)
	}
}

func assertWrites(t *testing.T, p *Plan, want ...string) {
	t.Helper()

	got := make([]string, 0, len(p.Writes))
	for _, w := range p.Writes {
		got = append(got, w.Dest)
	}
	if !slices.Equal(got, want) {
		t.Errorf("Writes:\n got %q\nwant %q", got, want)
	}
}

// assertAbsentFromPlan checks a path appears nowhere in a plan — not as a write, not
// as a prune, not in the next lock.
func assertAbsentFromPlan(t *testing.T, p *Plan, path string) {
	t.Helper()

	for _, w := range p.Writes {
		if w.Dest == path {
			t.Errorf("Writes: names the repo-owned file %q", path)
		}
	}
	if slices.Contains(p.Prune, path) {
		t.Errorf("Prune: names the repo-owned file %q", path)
	}
	for _, s := range p.Lock.Sources {
		for _, it := range s.Items {
			if slices.Contains(it.Files, path) {
				t.Errorf("next lock: source %q item %q names the repo-owned file %q", s.Name, it.ID, path)
			}
		}
	}
	if rendered := string(lock.Marshal(p.Lock)); strings.Contains(rendered, path) {
		t.Errorf("next lock bytes name the repo-owned file %q:\n%s", path, rendered)
	}
}
