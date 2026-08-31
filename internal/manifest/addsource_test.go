package manifest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/manifest"
)

// The append's headline property: a manifest graft did not write comes back with every
// byte it had, in order, and one table after them. Asserting the prefix rather than the
// whole result is what makes "the consumer's formatting survives" a property rather than
// a fixture comparison — a re-serializing implementation fails it whatever it produces.
func TestAddSourceLeavesTheOriginalBytesAsAPrefix(t *testing.T) {
	t.Parallel()

	original := `# the schemas we vendor
[sources.other]
git      =  "github.com/optioni/other"    # the fork, not upstream
rev      =  "v0.9.0"
install  =  [ "agent:*" ]
`
	got, err := manifest.AddSource([]byte(original), "shared", "github.com/optioni/shared", "v1.0.0", []string{"agent:reviewer"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if !strings.HasPrefix(string(got), original) {
		t.Fatalf("the original bytes are not a prefix of the result:\n%s", got)
	}

	m, err := manifest.Parse(got, manifest.Filename)
	if err != nil {
		t.Fatalf("the appended manifest does not parse: %v\n%s", err, got)
	}
	if len(m.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(m.Sources))
	}
	want := `
[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.0.0"
install = ["agent:reviewer"]
`
	if block := strings.TrimPrefix(string(got), original); block != want {
		t.Errorf("appended block = %q, want %q", block, want)
	}
}

func TestAddSourceOnAnEmptyFileWritesTheTableAlone(t *testing.T) {
	t.Parallel()

	got, err := manifest.AddSource(nil, "shared", "github.com/optioni/shared", "v1.0.0", []string{"agent:reviewer"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	want := `[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.0.0"
install = ["agent:reviewer"]
`
	if string(got) != want {
		t.Errorf("result = %q, want %q", got, want)
	}
}

// A table appended onto a truncated final line would corrupt that line, so the newline
// is the one byte the append may add ahead of its own block.
func TestAddSourceGivesAFileWithNoFinalNewlineOne(t *testing.T) {
	t.Parallel()

	original := "[sources.other]\ngit     = \"a/b\"\nrev     = \"v1\"\ninstall = [\"agent:*\"]"
	got, err := manifest.AddSource([]byte(original), "shared", "a/shared", "v1.0.0", []string{"agent:reviewer"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if !strings.HasPrefix(string(got), original+"\n\n[sources.shared]\n") {
		t.Errorf("the truncated last line did not gain exactly one newline:\n%q", got)
	}
	if _, err := manifest.Parse(got, manifest.Filename); err != nil {
		t.Fatalf("the appended manifest does not parse: %v", err)
	}
}

func TestAddSourceRendersEverySelectorOnOneLine(t *testing.T) {
	t.Parallel()

	got, err := manifest.AddSource(nil, "shared", "a/shared", "v1.0.0", []string{"schema:tdd", "agent:*"})
	if err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	if !strings.Contains(string(got), `install = ["schema:tdd", "agent:*"]`+"\n") {
		t.Errorf("selectors are not on one line in order:\n%s", got)
	}
}

func TestAddSourceRefusesANameAlreadyDeclared(t *testing.T) {
	t.Parallel()

	original := "[sources.shared]\ngit     = \"a/b\"\nrev     = \"v1\"\ninstall = [\"agent:*\"]\n"
	got, err := manifest.AddSource([]byte(original), "shared", "a/shared", "v1.0.0", []string{"agent:reviewer"})
	if err == nil {
		t.Fatalf("AddSource succeeded, want a refusal:\n%s", got)
	}
	if want := `graft.toml: source "shared": already declared`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if got != nil {
		t.Errorf("bytes returned beside an error: %q", got)
	}
}

func TestAddSourceRefusesANameThatIsNotABareKey(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"my.repo", "my repo", "", `my"repo`, "[shared]"} {
		got, err := manifest.AddSource(nil, name, "a/shared", "v1.0.0", []string{"agent:reviewer"})
		if err == nil {
			t.Fatalf("AddSource(%q) succeeded, want a refusal:\n%s", name, got)
		}
		if want := fmt.Sprintf("graft.toml: source name %q is not a bare key", name); err.Error() != want {
			t.Errorf("error for %q = %q, want %q", name, err, want)
		}
		if got != nil {
			t.Errorf("bytes returned beside an error for %q: %q", name, got)
		}
	}
}

// A quotation mark would close the string it is written into and turn everything after it
// into manifest syntax the consumer never wrote. The three keys are refused alike, and
// each names itself so the message says which value to fix.
func TestAddSourceRefusesAValueThatCannotBeWrittenLiterally(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		git, rev string
		install  []string
		want     string
	}{
		{
			name:    "git",
			git:     `a/b" bad`,
			rev:     "v1.0.0",
			install: []string{"agent:reviewer"},
			want:    `graft.toml: git "a/b\" bad" contains a quote, a backslash, or a control character`,
		},
		{
			name:    "rev",
			git:     "a/b",
			rev:     `v1\0`,
			install: []string{"agent:reviewer"},
			want:    `graft.toml: rev "v1\\0" contains a quote, a backslash, or a control character`,
		},
		{
			name:    "selector",
			git:     "a/b",
			rev:     "v1.0.0",
			install: []string{`agent:"x"`},
			want:    `graft.toml: selector "agent:\"x\"" contains a quote, a backslash, or a control character`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := manifest.AddSource(nil, "shared", tt.git, tt.rev, tt.install)
			if err == nil {
				t.Fatalf("AddSource succeeded, want a refusal:\n%s", got)
			}
			if err.Error() != tt.want {
				t.Errorf("error = %q, want %q", err, tt.want)
			}
			if got != nil {
				t.Errorf("bytes returned beside an error: %q", got)
			}
		})
	}
}

// The wording the pin move already produces may not move: two spellings of one refusal is
// two contracts, and SPEC.md's failure-mode table carries this one verbatim.
func TestSetRevKeepsItsOwnRefusalWording(t *testing.T) {
	t.Parallel()

	_, err := manifest.SetRev([]byte("[sources.shared]\nrev = \"v1\"\n"), "shared", `v2"x`)
	if err == nil {
		t.Fatal("SetRev succeeded, want a refusal")
	}
	if want := `graft.toml: rev "v2\"x" contains a quote, a backslash, or a control character`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
