package plan_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// impure lists the imports a pure package may not have. os and io/fs read the tree,
// path/filepath makes a path platform-dependent as well as reachable, os/exec runs a
// command, and net and net/http open a connection. plan does none of those, and this
// is where that stops being a comment.
var impure = []string{"os", "io/fs", "path/filepath", "os/exec", "net", "net/http"}

// TestPackageImportsNothingImpure is a lint written as a test. The rule it enforces —
// internal/plan never reaches the filesystem — is the whole reason the package exists,
// and no ordinary red-green cycle can express it: a filesystem read would make every
// behavioural test pass just as well. Parsing the package's own imports fails the build
// instead of relying on a reviewer noticing.
func TestPackageImportsNothingImpure(t *testing.T) {
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
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		checked++
		for _, bad := range impureImports(t, name, string(src)) {
			t.Error(bad)
		}
	}
	if checked == 0 {
		// Otherwise the guard passes vacuously the day the package is renamed or the
		// test is run from somewhere else.
		t.Fatal("no non-test Go file was checked; the purity guard would pass vacuously")
	}
}

// TestPackageImportsNothingImpure_ReportsTheOffender pins what the guard says when it
// fires. A guard nobody has watched fail is not a guard.
func TestPackageImportsNothingImpure_ReportsTheOffender(t *testing.T) {
	src := "package plan\n\nimport (\n\t\"os\"\n\t\"path\"\n)\n\nvar _ = os.Getenv\nvar _ = path.Join\n"

	got := impureImports(t, "leaky.go", src)
	want := []string{`leaky.go imports "os": internal/plan is pure and may not reach the filesystem`}
	if !slices.Equal(got, want) {
		t.Errorf("impureImports:\n got %q\nwant %q", got, want)
	}
}

// impureImports parses one file's import block and reports every forbidden import,
// naming the file and the import path. Only imports are parsed: the guard has no
// business type-checking the package it protects.
func impureImports(t *testing.T, filename, src string) []string {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", filename, err)
	}

	var found []string
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s: unquoting import %s: %v", filename, spec.Path.Value, err)
		}
		if slices.Contains(impure, p) {
			found = append(found, fmt.Sprintf(
				"%s imports %q: internal/plan is pure and may not reach the filesystem", filename, p))
		}
	}
	return found
}
