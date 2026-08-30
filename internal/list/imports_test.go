package list_test

import (
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// unreachable lists the packages a read-only command may not reach. internal/apply is the
// only package that writes to the working tree, internal/source is the only one that
// fetches, internal/plan computes file operations, and internal/manifest reads the file
// list deliberately does not.
var unreachable = []string{
	"github.com/optioni/graft/internal/apply",
	"github.com/optioni/graft/internal/plan",
	"github.com/optioni/graft/internal/source",
	"github.com/optioni/graft/internal/manifest",
}

// TestPackageReachesNothingThatWritesOrFetches is a lint written as a test. "list writes
// nothing and reaches no network" is the change's central promise, and no ordinary
// red-green cycle can express it: a listing that also wrote a file would pass every
// behavioural test here. Parsing the package's own source fails the build instead of
// relying on a reviewer noticing.
//
// It is over direct imports because that is the claim: internal/manifest is reachable
// transitively — lock.CheckPins takes []manifest.Source — and is never called.
func TestPackageReachesNothingThatWritesOrFetches(t *testing.T) {
	t.Parallel()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package directory: %v", err)
	}

	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		for _, path := range importsOf(t, name) {
			if slices.Contains(unreachable, path) {
				t.Errorf("%s imports %q: internal/list writes nothing and fetches nothing", name, path)
			}
		}
	}
	if checked == 0 {
		// Otherwise the guard passes vacuously the day the package is renamed or the test
		// is run from somewhere else.
		t.Fatal("no non-test Go file was checked; the guard would pass vacuously")
	}
}

// importsOf returns the import paths of one file in the package directory.
func importsOf(t *testing.T, filename string) []string {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", filename, err)
	}

	out := make([]string, 0, len(f.Imports))
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s: unquoting import %s: %v", filename, spec.Path.Value, err)
		}
		out = append(out, p)
	}
	return out
}
