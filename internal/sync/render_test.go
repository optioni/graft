package sync

import (
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/ui"
)

func plainUI() *ui.UI  { return ui.New(io.Discard, io.Discard, false) }
func colourUI() *ui.UI { return ui.New(io.Discard, io.Discard, true) }

// specExample is SPEC.md's Output section, character for character. It is the fixture
// rather than a paraphrase of it: the report layout is a contract, and the only way to keep
// a contract is to hold the bytes.
var specExample = []string{
	"openspec-schemas  v1.2.0 -> v1.3.0  (fae2a30 -> 9c1e77a)",
	"",
	"  updated  schema:tdd                6 files",
	"  removed  agent:phase-orchestrator  1 file   no longer provided",
	"",
	"6 files written, 1 removed - review with `git diff`",
}

func specReport() *Report {
	return &Report{
		Written: 6,
		Removed: 1,
		Sources: []SourceReport{{
			Name:         "openspec-schemas",
			PrevRev:      "v1.2.0",
			Rev:          "v1.3.0",
			PrevResolved: "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5",
			Resolved:     "9c1e77accccccccccccccccccccccccccccccccc",
			Items: []ItemReport{
				{Verb: verbUpdated, ID: "schema:tdd", Files: 6},
				{Verb: verbRemoved, ID: "agent:phase-orchestrator", Files: 1, Note: noteNotProvided},
			},
		}},
	}
}

func assertLines(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("lines:\n got %q\nwant %q", got, want)
	}
	for i, line := range got {
		if strings.TrimRight(line, " ") != line {
			t.Errorf("line %d carries trailing whitespace: %q", i, line)
		}
	}
}

func TestReportAlignment(t *testing.T) {
	t.Parallel()

	assertLines(t, specReport().Lines(plainUI()), specExample)
}

func TestReportUpToDateLine(t *testing.T) {
	t.Parallel()

	r := &Report{upToDate: true, Written: 6}
	assertLines(t, r.Lines(plainUI()), []string{"up to date"})

	// A dry run with nothing to do says the same thing: --dry-run changes what the summary
	// says, not what nothing to do means.
	r.DryRun = true
	assertLines(t, r.Lines(plainUI()), []string{"up to date"})
}

func TestReportSourceSeparation(t *testing.T) {
	t.Parallel()

	r := &Report{
		Written: 2,
		Sources: []SourceReport{
			{
				Name: "alpha", Rev: "v1.0.0", Resolved: "aaaaaaa1111111111111111111111111111111111",
				Items: []ItemReport{{Verb: verbAdded, ID: "agent:a", Files: 1}},
			},
			{
				Name: "beta", Rev: "v2.0.0", Resolved: "bbbbbbb2222222222222222222222222222222222",
				Items: []ItemReport{{Verb: verbAdded, ID: "agent:b", Files: 1}},
			},
		},
	}

	assertLines(t, r.Lines(plainUI()), []string{
		"alpha  v1.0.0  (aaaaaaa)",
		"",
		"  added  agent:a  1 file",
		"",
		"beta  v2.0.0  (bbbbbbb)",
		"",
		"  added  agent:b  1 file",
		"",
		"2 files written, 0 removed - review with `git diff`",
	})
}

// The rev is rendered once when it did not move, even though the sha did — which is what a
// branch pin moved by `graft update` looks like.
func TestReportHeaderShaOnly(t *testing.T) {
	t.Parallel()

	r := &Report{
		Written: 1,
		Sources: []SourceReport{{
			Name: "shared", PrevRev: "main", Rev: "main",
			PrevResolved: "aaaaaaa1111111111111111111111111111111111",
			Resolved:     "bbbbbbb2222222222222222222222222222222222",
			Items:        []ItemReport{{Verb: verbUpdated, ID: "schema:tdd", Files: 1}},
		}},
	}

	if got := r.Lines(plainUI())[0]; got != "shared  main  (aaaaaaa -> bbbbbbb)" {
		t.Errorf("header = %q", got)
	}
}

func TestReportSummary(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		written, removed int
		dryRun           bool
		want             string
	}{
		"several written and one removed": {6, 1, false, "6 files written, 1 removed - review with `git diff`"},
		"only removals":                   {0, 3, false, "0 files written, 3 removed - review with `git diff`"},
		"a single file":                   {1, 0, false, "1 file written, 0 removed - review with `git diff`"},
		"a dry run":                       {6, 1, true, "6 files to write, 1 to remove - nothing written"},
		"a dry run of a single file":      {1, 0, true, "1 file to write, 0 to remove - nothing written"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			r := &Report{Written: tc.written, Removed: tc.removed, DryRun: tc.dryRun}
			lines := r.Lines(plainUI())
			if got := lines[len(lines)-1]; got != tc.want {
				t.Errorf("summary = %q, want %q", got, tc.want)
			}
		})
	}
}

// With colour on, the verb is bold and a removed item's note is dim. Nothing else is
// styled, and stripping every escape sequence yields exactly the plain rendering — which is
// what proves the padding was computed on the unstyled text.
func TestReportStyledWithColorOn(t *testing.T) {
	t.Parallel()

	got := specReport().Lines(colourUI())

	if !strings.Contains(got[2], "\x1b[1mupdated\x1b[0m") {
		t.Errorf("the verb is not bold: %q", got[2])
	}
	if !strings.Contains(got[3], "\x1b[2mno longer provided\x1b[0m") {
		t.Errorf("the note is not dim: %q", got[3])
	}

	stripped := make([]string, 0, len(got))
	for _, line := range got {
		stripped = append(stripped, stripEscapes(line))
	}
	assertLines(t, stripped, specExample)
}

func TestReportPlainWithColorOff(t *testing.T) {
	t.Parallel()

	for _, line := range specReport().Lines(plainUI()) {
		if strings.ContainsRune(line, '\x1b') {
			t.Errorf("line carries an escape sequence with colour off: %q", line)
		}
	}
}

// stripEscapes removes every ANSI select-graphic-rendition sequence, so the styled and
// plain renderings can be compared as text.
func stripEscapes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\x1b' {
			b.WriteByte(s[i])
			continue
		}
		for i < len(s) && s[i] != 'm' {
			i++
		}
	}
	return b.String()
}
