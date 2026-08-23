package plan

import (
	"slices"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

const resolvedSHA = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"

var (
	tdd          = item("schema:tdd", "schema", "tdd", "extras/openspec-schemas/tdd")
	orchestrator = item("agent:apply-orchestrator", "agent", "apply-orchestrator", "extras/agents/apply-orchestrator.md")

	schemaKind = catalog.Kind{To: []string{"openspec/schemas/{name}"}}
	agentKind  = catalog.Kind{To: []string{".claude/agents/"}}
)

// withInstall replaces the selectors a source asks for, leaving its catalog alone —
// which is how a selector comes to match nothing.
func withInstall(in Input, selectors ...string) Input {
	in.Source.Install = selectors
	return in
}

// emptyLock is the state of a repo that has never synced: graft.lock is absent, and
// internal/lock loads it as an empty lock at the current format version.
func emptyLock() *lock.Lock { return &lock.Lock{Version: lock.Version} }

func schemaSource(listing Listing) Input {
	return sourceInput(
		"shared",
		map[string]catalog.Kind{"schema": schemaKind},
		[]catalog.Item{tdd},
		map[string]Listing{tdd.ID: listing},
		nil,
	)
}

func TestBuild_AFirstPlanAgainstNoLock(t *testing.T) {
	in := schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})
	in.Source.Rev = "v1.2.0"

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	want := []Write{{
		Source: "shared",
		Item:   "schema:tdd",
		From:   "extras/openspec-schemas/tdd/schema.yaml",
		Dest:   "openspec/schemas/tdd/schema.yaml",
	}}
	if !slices.Equal(p.Writes, want) {
		t.Errorf("Writes:\n got %+v\nwant %+v", p.Writes, want)
	}
	if len(p.Prune) != 0 {
		t.Errorf("Prune: want nothing pruned on a first plan, got %q", p.Prune)
	}
	assertLockRecords(t, p, "shared", "schema:tdd", []string{"openspec/schemas/tdd/schema.yaml"})
}

func TestBuild_AManifestWithNoSources(t *testing.T) {
	p, err := Build(nil, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	if len(p.Writes) != 0 || len(p.Prune) != 0 || len(p.Lock.Sources) != 0 {
		t.Errorf("Build: want an empty plan, got %d writes, %d prunes, %d sources",
			len(p.Writes), len(p.Prune), len(p.Lock.Sources))
	}
}

func TestBuild_AWriteCarriesTheSourcePathAndTheDestination(t *testing.T) {
	p, err := Build([]Input{schemaSource(Listing{Dir: true, Files: []string{"templates/design.md"}})}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	want := []Write{{
		Source: "shared",
		Item:   "schema:tdd",
		From:   "extras/openspec-schemas/tdd/templates/design.md",
		Dest:   "openspec/schemas/tdd/templates/design.md",
	}}
	if !slices.Equal(p.Writes, want) {
		t.Errorf("Writes:\n got %+v\nwant %+v", p.Writes, want)
	}
}

// TestBuild_AFileItemsWriteNamesTheFileItself: a file item's one listed path is its
// own base name, so joining it to From would yield
// extras/agents/apply-orchestrator.md/apply-orchestrator.md and every copy would fail
// at apply time. The listing entry names the destination's leaf; From names the file
// as the catalog wrote it.
func TestBuild_AFileItemsWriteNamesTheFileItself(t *testing.T) {
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"agent": agentKind},
		[]catalog.Item{orchestrator},
		map[string]Listing{orchestrator.ID: {Files: []string{"apply-orchestrator.md"}}},
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	want := []Write{{
		Source: "shared",
		Item:   "agent:apply-orchestrator",
		From:   "extras/agents/apply-orchestrator.md",
		Dest:   ".claude/agents/apply-orchestrator.md",
	}}
	if !slices.Equal(p.Writes, want) {
		t.Errorf("Writes:\n got %+v\nwant %+v", p.Writes, want)
	}
}

// TestSelector_FailuresSurfaceFromPlanning: typo protection has to reach the user
// through planning rather than being swallowed by it, so the expansion error is
// returned unchanged.
func TestSelector_FailuresSurfaceFromPlanning(t *testing.T) {
	tests := []struct {
		name string
		in   Input
		want string
	}{
		{
			name: "a selector matching nothing",
			in: withInstall(sourceInput(
				"shared",
				map[string]catalog.Kind{"schema": schemaKind, "agent": agentKind},
				[]catalog.Item{orchestrator, tdd},
				nil, nil,
			), "schema:tdd-workflwo"),
			want: `source "shared": selector "schema:tdd-workflwo" matches no item; catalog provides agent:apply-orchestrator, schema:tdd`,
		},
		{
			name: "a catalog providing zero items",
			in: withInstall(sourceInput(
				"shared",
				map[string]catalog.Kind{"agent": agentKind},
				nil, nil, nil,
			), "agent:*"),
			want: `source "shared": selector "agent:*" matches no item; catalog provides no items`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Build([]Input{tc.in}, emptyLock())
			assertNoPlan(t, p, err, tc.want)
		})
	}
}

// assertNoPlan pins the half of "a failing plan is returned as no plan at all" that
// every error case in this package shares: the error is exact, and the plan is nil, so
// nothing downstream can begin writing from a build that failed validation.
func assertNoPlan(t *testing.T, p *Plan, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("Build: want error %q, got a plan with %d writes", want, len(p.Writes))
	}
	if err.Error() != want {
		t.Errorf("Build:\n got %q\nwant %q", err.Error(), want)
	}
	if p != nil {
		t.Errorf("Build: want a nil plan on error, got %+v", p)
	}
}

func assertLockRecords(t *testing.T, p *Plan, source, id string, files []string) {
	t.Helper()

	for _, s := range p.Lock.Sources {
		if s.Name != source {
			continue
		}
		for _, it := range s.Items {
			if it.ID != id {
				continue
			}
			if !slices.Equal(it.Files, files) {
				t.Errorf("next lock: source %q item %q files:\n got %q\nwant %q", source, id, it.Files, files)
			}
			return
		}
		t.Fatalf("next lock: source %q records no item %q", source, id)
	}
	t.Fatalf("next lock: no source %q", source)
}

// TestBuild_ANilLockIsAnEmptyOne: Build is total over its inputs, and a caller that has
// no lock to hand it is the repo that has never synced.
func TestBuild_ANilLockIsAnEmptyOne(t *testing.T) {
	p, err := Build([]Input{schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})}, nil)
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	if len(p.Prune) != 0 {
		t.Errorf("Prune: want nothing pruned against no lock, got %q", p.Prune)
	}
	assertWrites(t, p, "openspec/schemas/tdd/schema.yaml")
}
