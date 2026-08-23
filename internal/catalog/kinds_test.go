package catalog_test

import (
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

func TestParse_Kinds(t *testing.T) {
	t.Parallel()

	const in = `version: 1
kinds:
  schema:
    to: "openspec/schemas/{name}"
  agent:
    to: ".claude/agents/"
    flatten: true
  both:
    to: [".claude/agents/", ".codex/agents/"]
`

	c, err := catalog.Parse([]byte(in), "catalog.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	tests := []struct {
		kind    string
		to      []string
		flatten bool
	}{
		// {name} is carried uninterpolated: interpolation belongs to the package that
		// computes a destination, not to the one that reads the file.
		{kind: "schema", to: []string{"openspec/schemas/{name}"}, flatten: false},
		// The trailing slash survives, because the slash is what later means
		// "into this directory".
		{kind: "agent", to: []string{".claude/agents/"}, flatten: true},
		{kind: "both", to: []string{".claude/agents/", ".codex/agents/"}, flatten: false},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			t.Parallel()

			k, ok := c.Kinds[tt.kind]
			if !ok {
				t.Fatalf("Kinds[%q] missing, got %v", tt.kind, c.Kinds)
			}
			if !sameStrings(k.To, tt.to) {
				t.Errorf("To = %q, want %q", k.To, tt.to)
			}
			if k.Flatten != tt.flatten {
				t.Errorf("Flatten = %v, want %v", k.Flatten, tt.flatten)
			}
		})
	}
}

func TestParse_KindErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "an empty kind name",
			in:   "version: 1\nkinds:\n  \"\":\n    to: \"x/\"\n",
			want: "catalog.yaml: kind name is empty",
		},
		{
			name: "to is absent",
			in:   "version: 1\nkinds:\n  agent:\n    flatten: true\n",
			want: `catalog.yaml: kind "agent": to is required`,
		},
		{
			name: "to is the empty string",
			in:   "version: 1\nkinds:\n  agent:\n    to: \"\"\n",
			want: `catalog.yaml: kind "agent": to is required`,
		},
		{
			name: "to is an empty list",
			in:   "version: 1\nkinds:\n  agent:\n    to: []\n",
			want: `catalog.yaml: kind "agent": to is required`,
		},
		{
			name: "to is null",
			in:   "version: 1\nkinds:\n  agent:\n    to:\n",
			want: `catalog.yaml: kind "agent": to is required`,
		},
		{
			name: "an empty destination inside a list",
			in:   "version: 1\nkinds:\n  agent:\n    to: [\".claude/agents/\", \"\"]\n",
			want: `catalog.yaml: kind "agent": to contains an empty destination`,
		},
		{
			name: "to is a mapping",
			in:   "version: 1\nkinds:\n  agent:\n    to: { dir: \".claude/agents/\" }\n",
			want: `catalog.yaml: kind "agent": to must be a string or a list of strings`,
		},
		{
			name: "to is a number",
			in:   "version: 1\nkinds:\n  agent:\n    to: 7\n",
			want: `catalog.yaml: kind "agent": to must be a string or a list of strings`,
		},
		{
			name: "a list element is not a string",
			in:   "version: 1\nkinds:\n  agent:\n    to: [\".claude/agents/\", 7]\n",
			want: `catalog.yaml: kind "agent": to must be a string or a list of strings`,
		},
		{
			name: "a repeated destination within one kind",
			in:   "version: 1\nkinds:\n  agent:\n    to: [\".claude/agents/\", \".claude/agents/\"]\n",
			want: `catalog.yaml: kind "agent": duplicate destination ".claude/agents/"`,
		},
		{
			name: "flatten is not a boolean",
			in:   "version: 1\nkinds:\n  agent:\n    to: \"x/\"\n    flatten: \"yes\"\n",
			want: `catalog.yaml: kind "agent": flatten must be a boolean`,
		},
		{
			name: "kinds is not a mapping",
			in:   "version: 1\nkinds: [agent]\n",
			want: "catalog.yaml: kinds must be a mapping",
		},
		{
			name: "a kind is not a mapping",
			in:   "version: 1\nkinds:\n  agent: \".claude/agents/\"\n",
			want: `catalog.yaml: kind "agent": must be a mapping`,
		},
		// Kinds are walked in sorted order, so a catalog with two faults always
		// reports the same one rather than whichever Go's map iteration reached first.
		{
			name: "two faulty kinds report the lowest-sorting one",
			in:   "version: 1\nkinds:\n  zebra:\n    to: []\n  agent:\n    to: []\n",
			want: `catalog.yaml: kind "agent": to is required`,
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

// A kind a catalog declares but never provides is legitimate: a source may declare
// hook and ship no hooks yet.
func TestParse_KindWithoutItems(t *testing.T) {
	t.Parallel()

	const in = `version: 1
kinds:
  agent:
    to: ".claude/agents/"
  hook:
    to: ".claude/hooks/"
provides:
  - { kind: agent, name: tdd, from: a.md }
`

	c, err := catalog.Parse([]byte(in), "catalog.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(c.Kinds) != 2 {
		t.Errorf("len(Kinds) = %d, want 2", len(c.Kinds))
	}
	if len(c.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1", len(c.Items))
	}
}
