package plan_test

import (
	"fmt"
	"go/ast"
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

// impureCalls lists the filesystem entry points of the packages plan legitimately
// imports. Banning imports alone is not enough: internal/catalog, internal/manifest,
// and internal/lock each read a file on request, so plan could reach the filesystem
// through a collaborator while importing nothing forbidden — exactly the regression
// this guard exists to catch. Everything plan needs from those packages arrives as a
// parsed value in Input.
var impureCalls = []string{"catalog.Load", "manifest.Load", "lock.Load"}

// TestPackageImportsNothingImpure is a lint written as a test. The rule it enforces —
// internal/plan never reaches the filesystem — is the whole reason the package exists,
// and no ordinary red-green cycle can express it: a filesystem read would make every
// behavioural test pass just as well. Parsing the package's own source fails the build
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
		for _, bad := range impurities(t, name, string(src)) {
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
// fires, for both halves of the rule. A guard nobody has watched fail is not a guard.
func TestPackageImportsNothingImpure_ReportsTheOffender(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "a forbidden import",
			src:  "package plan\n\nimport (\n\t\"os\"\n\t\"path\"\n)\n\nvar _ = os.Getenv\nvar _ = path.Join\n",
			want: []string{`leaky.go imports "os": internal/plan is pure and may not reach the filesystem`},
		},
		{
			// Imports nothing forbidden, and still reads a file.
			name: "a collaborator's filesystem entry point",
			src: "package plan\n\nimport \"github.com/optioni/graft/internal/catalog\"\n\n" +
				"func leak(p string) { _, _ = catalog.Load(p) }\n",
			want: []string{`leaky.go calls catalog.Load: internal/plan is pure and may not reach the filesystem`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := impurities(t, "leaky.go", tc.src); !slices.Equal(got, tc.want) {
				t.Errorf("impurities:\n got %q\nwant %q", got, tc.want)
			}
		})
	}
}

// impurities parses one file and reports every forbidden import and every call to a
// collaborator's filesystem entry point, naming the file and what it found.
func impurities(t *testing.T, filename, src string) []string {
	t.Helper()

	f, err := parser.ParseFile(token.NewFileSet(), filename, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", filename, err)
	}

	found := []string{}
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatalf("%s: unquoting import %s: %v", filename, spec.Path.Value, err)
		}
		if slices.Contains(impure, p) {
			found = append(found, fmt.Sprintf("%s imports %q: %s", filename, p, why))
		}
	}

	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if call := pkg.Name + "." + sel.Sel.Name; slices.Contains(impureCalls, call) {
			found = append(found, fmt.Sprintf("%s calls %s: %s", filename, call, why))
		}
		return true
	})

	if len(found) == 0 {
		return nil
	}
	return found
}

const why = "internal/plan is pure and may not reach the filesystem"
