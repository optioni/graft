package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/manifest"
)

// SetRev is text editing over a file a human wrote, so every test here asserts the exact
// expected bytes. A re-parse alone would pass an edit that landed on the wrong line and
// happened to leave valid TOML behind, which is the failure that actually matters.

const aligned = `# pinned deliberately
[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.0.0"
install = ["schema:tdd"]
`

func setRev(t *testing.T, in, name, rev string) string {
	t.Helper()
	out, err := manifest.SetRev([]byte(in), name, rev)
	if err != nil {
		t.Fatalf("SetRev(%q, %q): %v", name, rev, err)
	}
	return string(out)
}

func TestSetRevReplacesTheValueAndNothingElse(t *testing.T) {
	t.Parallel()

	want := `# pinned deliberately
[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.1.0"
install = ["schema:tdd"]
`
	if got := setRev(t, aligned, "shared", "v1.1.0"); got != want {
		t.Errorf("SetRev =\n%q\nwant\n%q", got, want)
	}
}

// The value ends at its closing quotation mark, never at the first "#" on the line: one
// reading deletes a trailing comment, the other corrupts a rev that contains a "#", and git
// permits one.
func TestSetRevKeepsATrailingComment(t *testing.T) {
	t.Parallel()

	in := "[sources.shared]\nrev     = \"v1.0.0\"  # do not bump without reading the changelog\n"
	want := "[sources.shared]\nrev     = \"v1.1.0\"  # do not bump without reading the changelog\n"
	if got := setRev(t, in, "shared", "v1.1.0"); got != want {
		t.Errorf("SetRev =\n%q\nwant\n%q", got, want)
	}

	hash := "[sources.shared]\nrev = \"release#1\"\n"
	wantHash := "[sources.shared]\nrev = \"release#2\"\n"
	if got := setRev(t, hash, "shared", "release#2"); got != wantHash {
		t.Errorf("SetRev with a # in the value =\n%q\nwant\n%q", got, wantHash)
	}
}

func TestSetRevLeavesOtherSourcesAlone(t *testing.T) {
	t.Parallel()

	in := `[sources.extra]
git     = "github.com/optioni/extra"
rev     = "v2.0.0"
install = ["agent:*"]

# the one that moves
[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.0.0"
install = ["schema:tdd"]
`
	got := setRev(t, in, "shared", "v1.1.0")

	if !strings.Contains(got, "[sources.extra]\ngit     = \"github.com/optioni/extra\"\nrev     = \"v2.0.0\"\n") {
		t.Errorf("the other source moved:\n%s", got)
	}
	if !strings.Contains(got, "# the one that moves\n") {
		t.Errorf("the comment was lost:\n%s", got)
	}

	m, err := manifest.Parse([]byte(got), manifest.Filename)
	if err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
	revs := map[string]string{}
	for _, s := range m.Sources {
		revs[s.Name] = s.Rev
	}
	if revs["extra"] != "v2.0.0" || revs["shared"] != "v1.1.0" {
		t.Errorf("revs = %v, want extra v2.0.0 and shared v1.1.0", revs)
	}
}

func TestSetRevPreservesLineEndingsAndAMissingFinalNewline(t *testing.T) {
	t.Parallel()

	crlf := "[sources.shared]\r\ngit     = \"x\"\r\nrev     = \"v1.0.0\"\r\n"
	wantCRLF := "[sources.shared]\r\ngit     = \"x\"\r\nrev     = \"v1.1.0\"\r\n"
	if got := setRev(t, crlf, "shared", "v1.1.0"); got != wantCRLF {
		t.Errorf("CRLF: SetRev =\n%q\nwant\n%q", got, wantCRLF)
	}

	bare := "[sources.shared]\nrev = \"v1.0.0\""
	wantBare := "[sources.shared]\nrev = \"v1.1.0\""
	if got := setRev(t, bare, "shared", "v1.1.0"); got != wantBare {
		t.Errorf("no final newline: SetRev =\n%q\nwant\n%q", got, wantBare)
	}
}

// kinds is a map with no constraint on its key names, so a kind named "rev" is legal. A
// scanner that leaves the source's table only at the next [sources. header walks straight
// into the sub-table and rewrites it.
func TestSetRevDoesNotReachIntoASubTable(t *testing.T) {
	t.Parallel()

	in := `[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.0.0"
install = ["schema:tdd"]

[sources.shared.kinds]
rev = ".codex/revs/"
`
	want := `[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.1.0"
install = ["schema:tdd"]

[sources.shared.kinds]
rev = ".codex/revs/"
`
	got := setRev(t, in, "shared", "v1.1.0")
	if got != want {
		t.Errorf("SetRev =\n%q\nwant\n%q", got, want)
	}

	m, err := manifest.Parse([]byte(got), manifest.Filename)
	if err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
	if m.Sources[0].Kinds["rev"] != ".codex/revs/" {
		t.Errorf("the kinds override = %v, want rev -> .codex/revs/", m.Sources[0].Kinds)
	}
}

func TestSetRevSkipsACommentedOutKey(t *testing.T) {
	t.Parallel()

	in := "[sources.shared]\n# rev     = \"v0.9.0\"\nrev     = \"v1.0.0\"\n"
	want := "[sources.shared]\n# rev     = \"v0.9.0\"\nrev     = \"v1.1.0\"\n"
	if got := setRev(t, in, "shared", "v1.1.0"); got != want {
		t.Errorf("SetRev =\n%q\nwant\n%q", got, want)
	}
}

func TestSetRevRecognisesAQuotedTableKey(t *testing.T) {
	t.Parallel()

	for _, header := range []string{`[sources."my-source"]`, `[ sources . "my-source" ]`, `[sources.'my-source']`} {
		in := header + "\nrev = \"v1.0.0\"\n"
		want := header + "\nrev = \"v1.1.0\"\n"
		if got := setRev(t, in, "my-source", "v1.1.0"); got != want {
			t.Errorf("header %s: SetRev =\n%q\nwant\n%q", header, got, want)
		}
	}
}

// A header line may carry a trailing comment, and the comment may itself contain a "]".
func TestSetRevRecognisesAHeaderWithATrailingComment(t *testing.T) {
	t.Parallel()

	in := "[sources.shared] # pinned [by hand]\nrev = \"v1.0.0\"\n"
	want := "[sources.shared] # pinned [by hand]\nrev = \"v1.1.0\"\n"
	if got := setRev(t, in, "shared", "v1.1.0"); got != want {
		t.Errorf("SetRev =\n%q\nwant\n%q", got, want)
	}
}

func TestSetRevRefusesAShapeItCannotRewriteExactly(t *testing.T) {
	t.Parallel()

	const wantErr = `graft.toml: source "shared": cannot move the pin: rev is not a plain key under [sources.shared]`

	cases := map[string]string{
		"an inline table": `[sources]
shared = { git = "x", rev = "v1.0.0", install = ["schema:tdd"] }
`,
		"a source the file does not declare": `[sources.extra]
rev = "v2.0.0"
`,
		"an array of tables": `[[sources.shared]]
rev = "v1.0.0"
`,
		"a multi-line value":                       "[sources.shared]\nrev = \"\"\"\nv1.0.0\"\"\"\n",
		"a value that is not a quoted string":      "[sources.shared]\nrev = 1\n",
		"a rev key with nothing after the =":       "[sources.shared]\nrev =\n",
		"a value whose closing quote is elsewhere": "[sources.shared]\nrev = \"v1.0.0\n",
		"no rev key at all": `[sources.shared]
git = "x"
`,
		"a dotted key at the top level": `sources.shared.rev = "v1.0.0"
`,
	}

	for name, in := range cases {
		out, err := manifest.SetRev([]byte(in), "shared", "v1.1.0")
		if err == nil {
			t.Errorf("%s: no error, want %q", name, wantErr)
			continue
		}
		if err.Error() != wantErr {
			t.Errorf("%s: error = %q, want %q", name, err, wantErr)
		}
		if out != nil {
			t.Errorf("%s: returned %q, want no bytes at all", name, out)
		}
	}
}

func TestSetRevRefusesARevItWouldHaveToEscape(t *testing.T) {
	t.Parallel()

	for _, rev := range []string{`v1"`, "v1\nrev = \"v2", `v1\`, "v1\x7f", "v1\x00", `v1'`} {
		out, err := manifest.SetRev([]byte(aligned), "shared", rev)
		want := "graft.toml: rev " + quote(rev) + " contains a quote, a backslash, or a control character"
		if err == nil {
			t.Errorf("rev %q: no error, want %q", rev, want)
			continue
		}
		if err.Error() != want {
			t.Errorf("rev %q: error = %q, want %q", rev, err, want)
		}
		if out != nil {
			t.Errorf("rev %q: returned %q, want no bytes at all", rev, out)
		}
	}
}

// The rev that would append a whole second key if it were written literally. It is refused
// by the character rule before any scanning, which is the whole reason the rule exists.
func TestSetRevRefusesARevThatWouldInjectAKey(t *testing.T) {
	t.Parallel()

	out, err := manifest.SetRev([]byte(aligned), "shared", "v1.0.0\"\ninstall = []")
	if err == nil {
		t.Fatal("no error: a rev may not close its own string")
	}
	if !strings.Contains(err.Error(), "contains a quote, a backslash, or a control character") {
		t.Errorf("error = %q, want the character rule", err)
	}
	if out != nil {
		t.Errorf("returned %q, want no bytes at all", out)
	}
}

func TestSetRevResultRoundTrips(t *testing.T) {
	t.Parallel()

	in := `[sources.extra]
git     = "github.com/optioni/extra"
rev     = "v2.0.0"
install = ["agent:*"]

[sources.shared]
git     = "github.com/optioni/shared"
rev     = "v1.0.0"
install = ["schema:tdd", "agent:reviewer"]

[sources.shared.kinds]
agent = ".codex/agents/"
`
	m, err := manifest.Parse([]byte(setRev(t, in, "shared", "v1.1.0")), manifest.Filename)
	if err != nil {
		t.Fatalf("the result does not parse: %v", err)
	}
	if len(m.Sources) != 2 {
		t.Fatalf("sources = %d, want 2", len(m.Sources))
	}
	extra, shared := m.Sources[0], m.Sources[1]
	if extra.Name != "extra" || extra.Rev != "v2.0.0" || len(extra.Install) != 1 {
		t.Errorf("extra = %+v", extra)
	}
	if shared.Rev != "v1.1.0" {
		t.Errorf("shared rev = %q, want v1.1.0", shared.Rev)
	}
	if len(shared.Install) != 2 || shared.Install[0] != "schema:tdd" {
		t.Errorf("shared install = %v", shared.Install)
	}
	if shared.Kinds["agent"] != ".codex/agents/" {
		t.Errorf("shared kinds = %v", shared.Kinds)
	}
}

// quote renders a rev the way the error message does, so the expectation and the message
// cannot drift apart through two spellings of the same escaping.
func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		default:
			if r < 0x20 || r == 0x7f {
				b.WriteString(controlEscape(r))
				continue
			}
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func controlEscape(r rune) string {
	const hex = "0123456789abcdef"
	return `\x` + string([]byte{hex[byte(r)>>4], hex[byte(r)&0xf]})
}

// Read is Load plus the bytes, and both spell "not found" once.
func TestReadReturnsTheBytesItParsed(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, manifest.Filename)
	if err := os.WriteFile(path, []byte(aligned), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m, data, err := manifest.Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(data) != aligned {
		t.Errorf("bytes = %q, want the file's own", data)
	}
	if len(m.Sources) != 1 || m.Sources[0].Rev != "v1.0.0" {
		t.Errorf("manifest = %+v", m.Sources)
	}

	_, _, err = manifest.Read(filepath.Join(dir, "absent.toml"))
	if err == nil || err.Error() != "absent.toml not found" {
		t.Errorf("error = %v, want %q", err, "absent.toml not found")
	}
}
