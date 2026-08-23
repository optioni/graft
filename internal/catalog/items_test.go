package catalog_test

import (
	"fmt"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// declared is a kinds block covering every kind the item fixtures below name, so a
// case about a missing field never fails on an undeclared kind instead.
const declared = `version: 1
kinds:
  agent:
    to: ".claude/agents/"
  schema:
    to: "openspec/schemas/{name}"
`

func TestParse_Items(t *testing.T) {
	t.Parallel()

	in := declared + "provides:\n  - { kind: schema, name: tdd, from: extras/openspec-schemas/tdd }\n"

	c, err := catalog.Parse([]byte(in), "catalog.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(c.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1", len(c.Items))
	}
	got := c.Items[0]
	want := catalog.Item{
		ID:   "schema:tdd",
		Kind: "schema",
		Name: "tdd",
		From: "extras/openspec-schemas/tdd",
	}
	if got != want {
		t.Errorf("Items[0] = %+v, want %+v", got, want)
	}
}

func TestParse_ItemOrder(t *testing.T) {
	t.Parallel()

	// Deliberately the wrong order: nothing downstream may depend on how a source
	// happened to write its list, because the lock records items by id.
	in := declared + `provides:
  - { kind: schema, name: tdd, from: extras/tdd }
  - { kind: agent, name: apply-orchestrator, from: extras/apply.md }
  - { kind: agent, name: tdd-reviewer, from: extras/reviewer.md }
`

	c, err := catalog.Parse([]byte(in), "catalog.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	want := []string{"agent:apply-orchestrator", "agent:tdd-reviewer", "schema:tdd"}
	got := make([]string, 0, len(c.Items))
	for _, it := range c.Items {
		got = append(got, it.ID)
	}
	if !sameStrings(got, want) {
		t.Errorf("item ids = %q, want %q", got, want)
	}
}

func TestParse_ItemErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "kind is missing",
			in:   declared + "provides:\n  - { name: tdd, from: a.md }\n",
			want: "catalog.yaml: provides[0]: kind is required",
		},
		{
			name: "name is missing",
			in:   declared + "provides:\n  - { kind: agent, from: a.md }\n",
			want: "catalog.yaml: provides[0]: name is required",
		},
		{
			name: "from is missing",
			in:   declared + "provides:\n  - { kind: agent, name: tdd }\n",
			want: "catalog.yaml: provides[0]: from is required",
		},
		{
			name: "from is empty",
			in:   declared + "provides:\n  - { kind: agent, name: tdd, from: \"\" }\n",
			want: "catalog.yaml: provides[0]: from is required",
		},
		{
			name: "a field is not a string",
			in:   declared + "provides:\n  - { kind: agent, name: 7, from: a.md }\n",
			want: "catalog.yaml: provides[0]: name must be a string",
		},
		{
			name: "a name containing a colon",
			in:   declared + "provides:\n  - { kind: agent, name: \"a:b\", from: x.md }\n",
			want: `catalog.yaml: provides[0]: invalid item id "agent:a:b": want kind:name`,
		},
		{
			// path.Match's * does not cross "/", so an item named nested/thing would
			// be invisible to agent:* while its siblings matched — a silent
			// under-install, not an error.
			name: "a name holding a path separator",
			in:   declared + "provides:\n  - { kind: agent, name: \"nested/thing\", from: a.md }\n",
			want: `catalog.yaml: provides[0]: invalid name "nested/thing": want letters, digits, dot, dash, or underscore`,
		},
		{
			// A source could otherwise name an item so that a consumer's exact
			// selector quietly pulls in a neighbour.
			name: "a name holding a glob metacharacter",
			in:   declared + "provides:\n  - { kind: agent, name: \"a*b\", from: a.md }\n",
			want: `catalog.yaml: provides[0]: invalid name "a*b": want letters, digits, dot, dash, or underscore`,
		},
		{
			name: "a name holding a character class",
			in:   declared + "provides:\n  - { kind: agent, name: \"[x]\", from: a.md }\n",
			want: `catalog.yaml: provides[0]: invalid name "[x]": want letters, digits, dot, dash, or underscore`,
		},
		{
			name: "a name holding a backslash",
			in:   declared + "provides:\n  - { kind: agent, name: \"a\\\\b\", from: a.md }\n",
			want: `catalog.yaml: provides[0]: invalid name "a\\b": want letters, digits, dot, dash, or underscore`,
		},
		{
			// {name} is interpolated into a destination by a later package, and this
			// is the check that must not depend on that package repeating it.
			name: "a name of dot-dot",
			in:   declared + "provides:\n  - { kind: agent, name: \"..\", from: a.md }\n",
			want: `catalog.yaml: provides[0]: invalid name "..": a name may not be "." or ".."`,
		},
		{
			name: "a name of dot",
			in:   declared + "provides:\n  - { kind: agent, name: \".\", from: a.md }\n",
			want: `catalog.yaml: provides[0]: invalid name ".": a name may not be "." or ".."`,
		},
		{
			name: "an undeclared kind",
			in: "version: 1\nkinds:\n  agent:\n    to: \".claude/agents/\"\n" +
				"provides:\n  - { kind: schema, name: tdd, from: extras/tdd }\n",
			want: `catalog.yaml: item "schema:tdd": kind "schema" is not declared`,
		},
		{
			name: "a duplicate item",
			in: declared + "provides:\n  - { kind: agent, name: tdd, from: a.md }\n" +
				"  - { kind: agent, name: tdd, from: b.md }\n",
			want: `catalog.yaml: duplicate item "agent:tdd"`,
		},
		{
			name: "provides is not a list",
			in:   declared + "provides:\n  agent: tdd\n",
			want: "catalog.yaml: provides must be a list",
		},
		{
			name: "an entry is not a mapping",
			in:   declared + "provides:\n  - agent:tdd\n",
			want: "catalog.yaml: provides[0]: must be a mapping",
		},
		{
			name: "the reported index is the entry's own",
			in: declared + "provides:\n  - { kind: agent, name: tdd, from: a.md }\n" +
				"  - { kind: agent, from: b.md }\n",
			want: "catalog.yaml: provides[1]: name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := catalog.Parse([]byte(tt.in), "catalog.yaml")
			if c != nil {
				t.Errorf("Parse() catalog = %+v, want nil", c)
			}
			if err == nil || err.Error() != tt.want {
				t.Fatalf("Parse() error = %v, want %q", err, tt.want)
			}
		})
	}
}

// The names a source is expected to use all pass, so the rule that closes the holes
// above does not also reject ordinary vocabulary.
func TestParse_OrdinaryNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"tdd", "apply-orchestrator", "outside_in", "v1.2", "TDD9"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in := declared + fmt.Sprintf("provides:\n  - { kind: agent, name: %q, from: a.md }\n", name)
			c, err := catalog.Parse([]byte(in), "catalog.yaml")
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if len(c.Items) != 1 || c.Items[0].Name != name {
				t.Errorf("Items = %+v, want one item named %q", c.Items, name)
			}
		})
	}
}

// from staying inside the source tree is one of SPEC.md's invariants. Requiring
// cleaned form as well as rejecting ".." removes the aliases that would otherwise
// name the same path while slipping past any later comparison.
func TestParse_FromContainment(t *testing.T) {
	t.Parallel()

	for _, from := range []string{
		"../outside",
		"extras/../../outside",
		"/etc/passwd",
		".",
		"./extras/tdd",
		"extras/tdd/",
	} {
		t.Run(from, func(t *testing.T) {
			t.Parallel()

			in := declared + fmt.Sprintf("provides:\n  - { kind: schema, name: tdd, from: %q }\n", from)
			want := fmt.Sprintf(
				"catalog.yaml: item %q: from %q is not a relative path inside the source",
				"schema:tdd", from,
			)

			c, err := catalog.Parse([]byte(in), "catalog.yaml")
			if c != nil {
				t.Errorf("Parse() catalog = %+v, want nil", c)
			}
			if err == nil || err.Error() != want {
				t.Fatalf("Parse() error = %v, want %q", err, want)
			}
		})
	}
}
