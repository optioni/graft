package rev_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestPackageIsALeaf is a lint written as a test. internal/rev exists only because both
// internal/lock and internal/source must ask the same question and may never disagree,
// and that only works if this package depends on neither of them, nor on anything that
// depends on either — a leaf, observably. No ordinary red-green cycle can express "this
// package imports nothing of graft's own"; parsing its own source fails the build instead
// of relying on a reviewer noticing the cycle it would otherwise reopen.
func TestPackageIsALeaf(t *testing.T) {
	t.Parallel()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	entries, err := os.ReadDir(dir)
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
		for _, path := range importsOf(t, filepath.Join(dir, name)) {
			if strings.HasPrefix(path, "github.com/optioni/graft/") {
				t.Errorf("%s imports %q: internal/rev is a leaf and may depend on no package of graft's own", name, path)
			}
		}
	}
	if checked == 0 {
		// Otherwise the guard passes vacuously the day the package is renamed or the
		// test is run from somewhere else.
		t.Fatal("no non-test Go file was checked; the guard would pass vacuously")
	}
}

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
