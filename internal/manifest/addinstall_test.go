package manifest_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/manifest"
)

// changedLines names the lines that differ between two texts, by index. The amendment's
// whole claim is about how few of them there are, so counting them is the assertion
// rather than a whole-file comparison that would pass for a re-rendered file too.
func changedLines(before, after string) []int {
	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	var out []int
	for i := range max(len(b), len(a)) {
		var bl, al string
		if i < len(b) {
			bl = b[i]
		}
		if i < len(a) {
			al = a[i]
		}
		if bl != al {
			out = append(out, i)
		}
	}
	return out
}

func TestAddInstallAmendsAOneLineArrayOnItsOwnLine(t *testing.T) {
	t.Parallel()

	before := `# vendored
[sources.shared]
git     = "a/shared"
rev     = "v1.0.0"
install = ["agent:reviewer"]
`
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"schema:tdd"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	if !strings.Contains(string(got), `install = ["agent:reviewer", "schema:tdd"]`+"\n") {
		t.Errorf("the amended line is not what was wanted:\n%s", got)
	}
	if lines := changedLines(before, string(got)); len(lines) != 1 {
		t.Errorf("changed lines = %v, want exactly one\n%s", lines, got)
	}
}

func TestAddInstallAmendsAMultiLineArrayKeepingItsIndentAndComma(t *testing.T) {
	t.Parallel()

	before := `[sources.shared]
git     = "a/shared"
rev     = "v1.0.0"
install = [
    "agent:reviewer",
    "agent:planner",
]
`
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"schema:tdd"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	want := `install = [
    "agent:reviewer",
    "agent:planner",
    "schema:tdd",
]
`
	if !strings.Contains(string(got), want) {
		t.Errorf("the amended array is not what was wanted:\n%s", got)
	}
	if lines := changedLines(before, string(got)); len(lines) != 2 {
		// One inserted line shifts the closing bracket, so two indices differ; the
		// assertion that matters is that no existing line's *text* was rewritten.
		t.Errorf("changed lines = %v, want two (the insertion and the shift)\n%s", lines, got)
	}
}

// The one existing byte an amendment may add: an array whose last element carries no
// comma cannot gain a sibling without one.
func TestAddInstallAddsTheMissingCommaAndNoMore(t *testing.T) {
	t.Parallel()

	before := `[sources.shared]
git     = "a/shared"
rev     = "v1.0.0"
install = [
    "agent:reviewer"
]
`
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"schema:tdd"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	want := `install = [
    "agent:reviewer",
    "schema:tdd"
]
`
	if !strings.Contains(string(got), want) {
		t.Errorf("the amended array is not what was wanted:\n%s", got)
	}
	if _, err := manifest.Parse(got, manifest.Filename); err != nil {
		t.Fatalf("the amended manifest does not parse: %v\n%s", err, got)
	}
}

func TestAddInstallLeavesASelectorItAlreadyHolds(t *testing.T) {
	t.Parallel()

	before := "[sources.shared]\ngit     = \"a/shared\"\nrev     = \"v1.0.0\"\ninstall = [\"agent:reviewer\"]\n"
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"agent:reviewer"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	if string(got) != before {
		t.Errorf("the bytes changed for a selector already present:\n%s", got)
	}
}

func TestAddInstallInsertsBeforeATrailingComment(t *testing.T) {
	t.Parallel()

	before := `[sources.shared]
git     = "a/shared"
rev     = "v1.0.0"
install = [
  "agent:reviewer",
  # everything below is experimental
]
`
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"schema:tdd"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	want := `  "agent:reviewer",
  "schema:tdd",
  # everything below is experimental
]
`
	if !strings.Contains(string(got), want) {
		t.Errorf("the comment did not survive in place:\n%s", got)
	}
}

func TestAddInstallAddsSeveralSelectorsInOrder(t *testing.T) {
	t.Parallel()

	before := "[sources.shared]\ngit = \"a/shared\"\nrev = \"v1\"\ninstall = [\"agent:reviewer\"]\n"
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"schema:tdd", "agent:planner", "agent:reviewer"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	if !strings.Contains(string(got), `install = ["agent:reviewer", "schema:tdd", "agent:planner"]`) {
		t.Errorf("selectors are not appended in order, once each:\n%s", got)
	}
}

func TestAddInstallRefusesEveryShapeItCannotRewriteExactly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		before string
	}{
		{
			name:   "install is not an array",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = \"agent:reviewer\"\n",
		},
		{
			name:   "the source is an inline table",
			before: "sources = { shared = { git = \"a/b\", rev = \"v1\", install = [\"agent:reviewer\"] } }\n",
		},
		{
			name:   "the array is never closed",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = [\n  \"agent:reviewer\",\n",
		},
		{
			name:   "an element is not a string",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = [\"agent:reviewer\", 3]\n",
		},
		{
			name:   "an element carries an escape",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = [\"agent:\\u0072eviewer\"]\n",
		},
		{
			name:   "an element is a multi-line string",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = [\"\"\"agent:reviewer\"\"\"]\n",
		},
		{
			name:   "there is no install key at all",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\n",
		},
		{
			name:   "the install key is in a sub-table",
			before: "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\n\n[sources.shared.kinds]\ninstall = \"x/\"\n",
		},
	}
	want := `graft.toml: source "shared": cannot amend install: install is not a plain array of strings under [sources.shared]`
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := manifest.AddInstall([]byte(tt.before), "shared", []string{"schema:tdd"})
			if err == nil {
				t.Fatalf("AddInstall succeeded, want a refusal:\n%s", got)
			}
			if err.Error() != want {
				t.Errorf("error = %q, want %q", err, want)
			}
			if got != nil {
				t.Errorf("bytes returned beside an error: %q", got)
			}
		})
	}
}

func TestAddInstallRefusesASelectorThatCannotBeWrittenLiterally(t *testing.T) {
	t.Parallel()

	before := "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = [\"agent:reviewer\"]\n"
	got, err := manifest.AddInstall([]byte(before), "shared", []string{`agent:"x"`})
	if err == nil {
		t.Fatalf("AddInstall succeeded, want a refusal:\n%s", got)
	}
	if want := `graft.toml: selector "agent:\"x\"" contains a quote, a backslash, or a control character`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}

// A commented-out install above the real one is not the key, exactly as a commented-out
// rev is not the key the pin move finds.
func TestAddInstallSkipsACommentedOutInstall(t *testing.T) {
	t.Parallel()

	before := `[sources.shared]
git     = "a/shared"
rev     = "v1.0.0"
# install = ["agent:old"]
install = ["agent:reviewer"]
`
	got, err := manifest.AddInstall([]byte(before), "shared", []string{"schema:tdd"})
	if err != nil {
		t.Fatalf("AddInstall: %v", err)
	}
	if !strings.Contains(string(got), `# install = ["agent:old"]`+"\n"+`install = ["agent:reviewer", "schema:tdd"]`) {
		t.Errorf("the commented-out key was not skipped:\n%s", got)
	}
}
