package catalog_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// one is the smallest catalog that parses, used as the document these fixtures add to.
const one = "version: 1\nkinds:\n  agent:\n    to: \".claude/agents/\"\n"

const multiDoc = "catalog.yaml: multiple YAML documents; a catalog is a single document"

// TestParse_MultipleDocuments covers the refusing half of the rule. Each case fails
// differently against a count taken from the decoder rather than from the tokens, which
// is why they are listed separately rather than collapsed into one fixture.
func TestParse_MultipleDocuments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{
			// The plain case: the second document's kind is dropped in silence.
			name: "two documents",
			in:   one + "---\nversion: 1\nkinds:\n  schema:\n    to: \"openspec/schemas/{name}\"\n",
		},
		{
			// The decoder reports its own syntax error here, quoting the offending
			// lines back. The fault is that the content is there at all.
			name: "malformed second document",
			in:   one + "---\nkinds: [unclosed\n",
		},
		{
			// Two adjacent markers: the decoder returns io.EOF after the first
			// document and the trailing kind vanishes with a nil error.
			name: "adjacent separators then content",
			in:   one + "---\n---\nkinds:\n  schema:\n    to: \"x/\"\n",
		},
		{
			// The decoder reads nothing at all here and reports a missing version
			// for a file that declares one.
			name: "two leading separators",
			in:   "---\n---\n" + one,
		},
		{
			// The prologue rule suppresses a region; it must not suppress a
			// document. The second "---" here separates rather than opens.
			name: "a directives prologue before two documents",
			in:   "%YAML 1.2\n---\n" + one + "---\nkinds:\n  schema:\n    to: \"x/\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := catalog.Parse([]byte(tt.in), "catalog.yaml")
			if c != nil {
				t.Errorf("Parse() catalog = %+v, want nil", c)
			}
			if err == nil || err.Error() != multiDoc {
				t.Fatalf("Parse() error = %v, want %q", err, multiDoc)
			}
		})
	}
}

// TestParse_OneDocument is the other half: a marker that introduces no document, and a
// marker that is not a marker at all. Every case here parses today and must keep
// parsing — the rule above must refuse two documents without refusing these.
func TestParse_OneDocument(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{name: "trailing separator", in: one + "---\n"},
		{name: "trailing separator then comment", in: one + "---\n# nothing here\n"},
		{name: "leading separator", in: "---\n" + one},
		{name: "leading and trailing separator", in: "---\n" + one + "---\n"},
		{
			name: "a marker inside a quoted string",
			in:   "version: 1\nkinds:\n  agent:\n    to: \"---\"\n",
		},
		{
			name: "a marker inside a block scalar",
			in:   "version: 1\nkinds:\n  agent:\n    to: |\n      ---\n      still one document\n",
		},
		{
			// A directives prologue is closed by ---, which is the same marker that
			// separates two documents. It opens the one document rather than ending
			// a document before it, so it must not be counted.
			name: "a YAML directive before the document",
			in:   "%YAML 1.2\n---\n" + one,
		},
		{
			name: "a tag directive before the document",
			in:   "%TAG ! tag:example.com,2026:\n---\n" + one,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			c, err := catalog.Parse([]byte(tt.in), "catalog.yaml")
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if c.Version != 1 {
				t.Errorf("Version = %d, want 1", c.Version)
			}
			if len(c.Kinds) != 1 {
				t.Errorf("len(Kinds) = %d, want 1", len(c.Kinds))
			}
		})
	}
}

// TestParse_MalformedSingleDocumentIsNotAMultiDocumentError guards the boundary between
// the two messages: one malformed document is one document, and its report belongs to
// the decoder.
func TestParse_MalformedSingleDocumentIsNotAMultiDocumentError(t *testing.T) {
	t.Parallel()

	_, err := catalog.Parse([]byte("version: 1\nkinds: [unclosed\n"), "catalog.yaml")
	if err == nil {
		t.Fatal("Parse() error = nil, want a decoder error")
	}
	if strings.Contains(err.Error(), "multiple YAML documents") {
		t.Errorf("Parse() error = %q, want the decoder's own message", err)
	}
}
