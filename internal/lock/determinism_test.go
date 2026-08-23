package lock_test

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"github.com/optioni/graft/internal/lock"
)

// twoSources builds the same content twice: once with every collection ascending and
// once with every collection reversed. Marshal must not be able to tell them apart.
func twoSources(reversed bool) *lock.Lock {
	order := func(s []string) []string {
		out := slices.Clone(s)
		if reversed {
			slices.Reverse(out)
		}
		return out
	}
	items := func(its []lock.Item) []lock.Item {
		out := slices.Clone(its)
		for i := range out {
			out[i].Files = order(out[i].Files)
		}
		if reversed {
			slices.Reverse(out)
		}
		return out
	}

	sources := []lock.Source{
		{
			Name: "alpha", Git: "github.com/optioni/alpha", Rev: "v1.0.0", Resolved: sha,
			Items: items([]lock.Item{
				{ID: "agent:one", Files: []string{".claude/agents/a.md", ".claude/agents/b.md"}},
				{ID: "schema:tdd", Files: []string{"openspec/schemas/tdd/a.yaml", "openspec/schemas/tdd/b.yaml"}},
			}),
		},
		{
			Name: "beta", Git: "github.com/optioni/beta", Rev: "v2.0.0", Resolved: sha,
			Items: items([]lock.Item{
				{ID: "hook:pre", Files: []string{"hooks/pre.sh"}},
				{ID: "skill:x", Files: []string{"skills/x/SKILL.md", "skills/x/y.md"}},
			}),
		},
	}
	if reversed {
		slices.Reverse(sources)
	}
	return &lock.Lock{Version: 1, Sources: sources}
}

func TestMarshal_OrderIndependent(t *testing.T) {
	t.Parallel()

	asc := lock.Marshal(twoSources(false))
	desc := lock.Marshal(twoSources(true))
	if !bytes.Equal(asc, desc) {
		t.Fatalf("Marshal() depends on input order\nascending:\n%s\nreversed:\n%s", asc, desc)
	}

	parsed, err := lock.Parse(asc, "graft.lock")
	if err != nil {
		t.Fatalf("Parse() of marshalled bytes error = %v, want nil", err)
	}
	names := make([]string, 0, len(parsed.Sources))
	for _, s := range parsed.Sources {
		names = append(names, s.Name)
		ids := make([]string, 0, len(s.Items))
		for _, it := range s.Items {
			ids = append(ids, it.ID)
			if !slices.IsSorted(it.Files) {
				t.Errorf("source %q item %q files = %v, want path-ascending", s.Name, it.ID, it.Files)
			}
		}
		if !slices.IsSorted(ids) {
			t.Errorf("source %q items = %v, want id-ascending", s.Name, ids)
		}
	}
	if !slices.IsSorted(names) {
		t.Errorf("sources = %v, want name-ascending", names)
	}
}

func TestMarshal_Twice(t *testing.T) {
	t.Parallel()

	l := canonicalLock()
	first := lock.Marshal(l)
	second := lock.Marshal(l)
	if !bytes.Equal(first, second) {
		t.Errorf("Marshal() is not byte-stable across two calls\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestMarshal_DoesNotMutate proves Marshal sorts a copy. A caller holding a lock must
// not find its slices reordered underneath it.
func TestMarshal_DoesNotMutate(t *testing.T) {
	t.Parallel()

	l := twoSources(true)
	before := l.Sources[0].Name
	beforeFiles := slices.Clone(l.Sources[0].Items[0].Files)
	lock.Marshal(l)
	if l.Sources[0].Name != before {
		t.Errorf("Marshal() reordered the caller's sources: %q became %q", before, l.Sources[0].Name)
	}
	if !slices.Equal(l.Sources[0].Items[0].Files, beforeFiles) {
		t.Errorf("Marshal() reordered the caller's files: %v became %v", beforeFiles, l.Sources[0].Items[0].Files)
	}
}

func TestRoundTrip_Canonical(t *testing.T) {
	t.Parallel()

	want := golden(t)
	parsed, err := lock.Parse(want, "graft.lock")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if got := lock.Marshal(parsed); !bytes.Equal(got, want) {
		t.Errorf("canonical bytes did not survive a round trip\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRoundTrip_Normalizes(t *testing.T) {
	t.Parallel()

	in, err := os.ReadFile("testdata/scrambled.lock")
	if err != nil {
		t.Fatalf("reading scrambled fixture: %v", err)
	}
	parsed, err := lock.Parse(in, "graft.lock")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	want := golden(t)
	first := lock.Marshal(parsed)
	if !bytes.Equal(first, want) {
		t.Fatalf("non-canonical input did not normalize\n got:\n%s\nwant:\n%s", first, want)
	}

	again, err := lock.Parse(first, "graft.lock")
	if err != nil {
		t.Fatalf("Parse() of normalized bytes error = %v, want nil", err)
	}
	if second := lock.Marshal(again); !bytes.Equal(second, first) {
		t.Errorf("normalization did not settle after one pass\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}
