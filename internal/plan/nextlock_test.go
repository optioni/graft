package plan

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
)

// rendered is what a consumer will actually read in a diff. Assertions about the next
// lock are made against these bytes rather than against struct fields, because the
// bytes are the artifact.
func rendered(t *testing.T, p *Plan) string {
	t.Helper()
	return string(lock.Marshal(p.Lock))
}

// TestBuild_AnItemContributingNoFilesStillAppearsInTheLock: an empty directory in the
// source is a legitimate state, and lock-format already declares an empty files list
// valid. Dropping the item from the lock instead would make the next sync unable to
// tell "installed, empty" from "never installed".
func TestBuild_AnItemContributingNoFilesStillAppearsInTheLock(t *testing.T) {
	empty := item("schema:empty", "schema", "empty", "extras/openspec-schemas/empty")
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"schema": schemaKind},
		[]catalog.Item{empty},
		nil,
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	if len(p.Writes) != 0 {
		t.Errorf("Writes: want none, got %+v", p.Writes)
	}
	if len(p.Prune) != 0 {
		t.Errorf("Prune: want none, got %q", p.Prune)
	}
	want := "  [[source.item]]\n  id    = \"schema:empty\"\n  files = []\n"
	if got := rendered(t, p); !strings.Contains(got, want) {
		t.Errorf("next lock:\n%s\ndoes not record the item with an empty files list", got)
	}
}

func TestBuild_AnItemInTwoDestinationsRecordsBothFiles(t *testing.T) {
	in := sourceInput(
		"shared",
		map[string]catalog.Kind{"schema": {To: []string{"vendor/schemas/{name}", "openspec/schemas/{name}"}}},
		[]catalog.Item{tdd},
		map[string]Listing{tdd.ID: {Dir: true, Files: []string{"schema.yaml"}}},
		nil,
	)

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	assertLockRecords(t, p, "shared", "schema:tdd", []string{
		"openspec/schemas/tdd/schema.yaml",
		"vendor/schemas/tdd/schema.yaml",
	})

	// Both belong to the one item, so dropping it later removes both.
	p = prune(t, nil, p.Lock)
	want := []string{"openspec/schemas/tdd/schema.yaml", "vendor/schemas/tdd/schema.yaml"}
	if !slices.Equal(p.Prune, want) {
		t.Errorf("Prune after dropping the item:\n got %q\nwant %q", p.Prune, want)
	}
}

// TestBuild_TheNextLockCarriesRevAndResolvedSeparately: rev records the request and
// resolved the sha it became, and git is recorded exactly as graft.toml wrote it —
// shorthand unexpanded, because expanding it belongs to the package that talks to git.
func TestBuild_TheNextLockCarriesRevAndResolvedSeparately(t *testing.T) {
	in := schemaSource(Listing{Dir: true, Files: []string{"schema.yaml"}})
	in.Source.Git = "github.com/optioni/openspec-schemas"
	in.Source.Rev = "v1.2.0"
	in.Resolved = resolvedSHA

	p, err := Build([]Input{in}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	for _, want := range []string{
		`git      = "github.com/optioni/openspec-schemas"`,
		`rev      = "v1.2.0"`,
		`resolved = "` + resolvedSHA + `"`,
	} {
		if got := rendered(t, p); !strings.Contains(got, want) {
			t.Errorf("next lock:\n%s\ndoes not contain %q", got, want)
		}
	}
}

// TestBuild_TheNextLockRoundTripsThroughTheLockParser is what checks, without a second
// validator in this package, that everything graft.lock enforces on load holds of what
// plan built: a 40-character hex resolved, unique source names, unique item ids, and no
// path claimed twice. A plan may never build a lock a later sync would refuse to read.
func TestBuild_TheNextLockRoundTripsThroughTheLockParser(t *testing.T) {
	empty := item("schema:empty", "schema", "empty", "extras/openspec-schemas/empty")
	shared := sourceInput(
		"shared",
		map[string]catalog.Kind{"schema": schemaKind},
		[]catalog.Item{empty, tdd},
		map[string]Listing{tdd.ID: {Dir: true, Files: []string{"schema.yaml", "templates/design.md"}}},
		nil,
	)
	other := agentSource("other", nil, agentX)

	p, err := Build([]Input{shared, other}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}

	data := lock.Marshal(p.Lock)
	parsed, err := lock.Parse(data, "graft.lock")
	if err != nil {
		t.Fatalf("lock.Parse of the plan's own lock failed: %v\n%s", err, data)
	}
	if !bytes.Equal(data, lock.Marshal(parsed)) {
		t.Errorf("round trip changed the lock:\n before %s\n after %s", data, lock.Marshal(parsed))
	}
	if len(parsed.Sources) != 2 {
		t.Fatalf("round trip: want 2 sources, got %d", len(parsed.Sources))
	}
	for i, s := range parsed.Sources {
		want := p.Lock.Sources[i]
		if s.Name != want.Name || s.Git != want.Git || s.Rev != want.Rev || s.Resolved != want.Resolved {
			t.Errorf("round trip: source %d:\n got %+v\nwant %+v", i, s, want)
		}
		if len(s.Items) != len(want.Items) {
			t.Fatalf("round trip: source %q: want %d items, got %d", s.Name, len(want.Items), len(s.Items))
		}
		for j, it := range s.Items {
			if it.ID != want.Items[j].ID || !slices.Equal(it.Files, want.Items[j].Files) {
				t.Errorf("round trip: source %q item %d:\n got %+v\nwant %+v", s.Name, j, it, want.Items[j])
			}
		}
	}
}

// TestBuild_SourcesAreSortedByNameInThePlan asserts the plan's own slice, not the
// serialized bytes. lock.Marshal normalizes order on its way out, so asserting only on
// bytes would let an unsorted plan hide behind it — and internal/apply reads the plan,
// not the bytes.
func TestBuild_SourcesAreSortedByNameInThePlan(t *testing.T) {
	zeta := agentSource("zeta", nil, agentX)
	alpha := sourceInput(
		"alpha",
		map[string]catalog.Kind{"schema": schemaKind},
		[]catalog.Item{tdd},
		map[string]Listing{tdd.ID: {Dir: true, Files: []string{"schema.yaml"}}},
		nil,
	)

	p, err := Build([]Input{zeta, alpha}, emptyLock())
	if err != nil {
		t.Fatalf("Build: unexpected error: %v", err)
	}
	got := make([]string, 0, len(p.Lock.Sources))
	for _, s := range p.Lock.Sources {
		got = append(got, s.Name)
	}
	if want := []string{"alpha", "zeta"}; !slices.Equal(got, want) {
		t.Errorf("next lock source order:\n got %q\nwant %q", got, want)
	}
}
