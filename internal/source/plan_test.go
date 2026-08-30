package source

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/optioni/graft/internal/catalog"
	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/manifest"
	"github.com/optioni/graft/internal/plan"
)

const fixtureCatalog = `version: 1
kinds:
  schema:
    to: "openspec/schemas/{name}"
  agent:
    to: ".claude/agents/"
provides:
  - { kind: schema, name: tdd, from: extras/tdd }
  - { kind: agent, name: apply-orchestrator, from: extras/agents/apply-orchestrator.md }
`

// newPlanFixture builds a source repository and tags it, returning the repo.
func newPlanFixture(t *testing.T) *repo {
	t.Helper()
	r := newRepo(t)
	r.write("catalog.yaml", fixtureCatalog)
	r.write("extras/tdd/schema.yaml", "schema\n")
	r.write("extras/tdd/templates/proposal.md", "proposal\n")
	r.write("extras/agents/apply-orchestrator.md", "orchestrator\n")
	r.commit("one")
	r.tag("v1.0.0")
	return r
}

// buildFromFixture runs the whole chain a sync will run: resolve, fetch, read the
// catalog, expand the selectors, list each item, and plan.
func buildFromFixture(t *testing.T, r *repo, root string) (*plan.Plan, string) {
	t.Helper()

	sha, _, err := Resolve("shared", r.URL(), "v1.0.0")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	entry, err := (Cache{Root: root}).Fetch("shared", r.URL(), sha)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	cat, err := ReadCatalog(entry)
	if err != nil {
		t.Fatalf("ReadCatalog: %v", err)
	}
	src := manifest.Source{
		Name:    "shared",
		Git:     r.URL(),
		Rev:     "v1.0.0",
		Install: []string{"schema:tdd", "agent:*"},
	}
	items, err := catalog.Expand(cat, src.Name, src.Install)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	listings := map[string]plan.Listing{}
	for _, it := range items {
		l, err := List(entry, src.Name, it)
		if err != nil {
			t.Fatalf("List(%s): %v", it.ID, err)
		}
		listings[it.ID] = l
	}
	in := plan.Input{Source: src, Resolved: sha, Catalog: cat, Items: listings}

	p, err := plan.Build([]plan.Input{in}, &lock.Lock{Version: lock.Version})
	if err != nil {
		t.Fatalf("plan.Build: %v", err)
	}
	return p, entry
}

// TestFetchedSourcePlansItsTree discharges destination-and-plan's deferred note: whether
// a Listing faithfully describes a real fetched tree is this change's contract, and it is
// only answerable against a real one.
func TestFetchedSourcePlansItsTree(t *testing.T) {
	r := newPlanFixture(t)
	p, entry := buildFromFixture(t, r, filepath.Join(t.TempDir(), "cache"))

	var dests []string
	for _, w := range p.Writes {
		dests = append(dests, w.Dest)
	}
	want := []string{
		".claude/agents/apply-orchestrator.md",
		"openspec/schemas/tdd/schema.yaml",
		"openspec/schemas/tdd/templates/proposal.md",
	}
	if !slices.Equal(dests, want) {
		t.Errorf("Writes:\n got %q\nwant %q", dests, want)
	}

	// Every source path names a file that actually exists in the fetched entry. This is
	// the property a hand-written Listing in a plan unit test cannot check, and the one
	// that catches the file-item asymmetry: Write.From is item.From itself for a file
	// item, not a path below it, so joining unconditionally would yield
	// extras/agents/apply-orchestrator.md/apply-orchestrator.md.
	for _, w := range p.Writes {
		full := filepath.Join(entry, filepath.FromSlash(w.From))
		fi, err := os.Lstat(full)
		if err != nil {
			t.Errorf("Write %q: From %q names nothing in the entry: %v", w.Dest, w.From, err)
			continue
		}
		if !fi.Mode().IsRegular() {
			t.Errorf("Write %q: From %q is not a regular file", w.Dest, w.From)
		}
	}
}

// TestFetchedSourcePlansDeterministically asserts byte equality of the serialized lock
// across two independent runs of the whole chain — over values a real directory walk
// produced, rather than over literals. reflect.DeepEqual on the structs is not
// sufficient: a lock that differs only in ordering churns every consumer's diff.
func TestFetchedSourcePlansDeterministically(t *testing.T) {
	r := newPlanFixture(t)

	first, _ := buildFromFixture(t, r, filepath.Join(t.TempDir(), "cache"))
	second, _ := buildFromFixture(t, r, filepath.Join(t.TempDir(), "cache"))

	a, b := lock.Marshal(first.Lock), lock.Marshal(second.Lock)
	if !bytes.Equal(a, b) {
		t.Errorf("two runs of the chain serialize differently:\n%s\n---\n%s", a, b)
	}
}
