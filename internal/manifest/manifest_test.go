package manifest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/manifest"
)

const minimal = `[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
rev     = "v1.2.0"
install = ["schema:tdd", "agent:*"]
`

// writeManifest drops content into a fresh temp dir and returns the path it wrote.
func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "graft.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	return path
}

// snapshot lists every path under dir, so a test can prove loading touched nothing.
func snapshot(t *testing.T, dir string) []string {
	t.Helper()
	var got []string
	err := filepath.Walk(dir, func(path string, _ os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		got = append(got, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return got
}

func TestLoad_Minimal(t *testing.T) {
	t.Parallel()

	path := writeManifest(t, minimal)
	dir := filepath.Dir(path)
	before := snapshot(t, dir)

	m, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if len(m.Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1", len(m.Sources))
	}
	s := m.Sources[0]
	if s.Name != "openspec-schemas" {
		t.Errorf("Name = %q, want %q", s.Name, "openspec-schemas")
	}
	if s.Git != "github.com/optioni/openspec-schemas" {
		t.Errorf("Git = %q, want %q", s.Git, "github.com/optioni/openspec-schemas")
	}
	if s.Rev != "v1.2.0" {
		t.Errorf("Rev = %q, want %q", s.Rev, "v1.2.0")
	}
	want := []string{"schema:tdd", "agent:*"}
	if len(s.Install) != len(want) {
		t.Fatalf("Install = %v, want %v", s.Install, want)
	}
	for i := range want {
		if s.Install[i] != want[i] {
			t.Errorf("Install[%d] = %q, want %q", i, s.Install[i], want[i])
		}
	}

	if after := snapshot(t, dir); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("tree changed: before %v, after %v", before, after)
	}
}

func TestParse_Empty(t *testing.T) {
	t.Parallel()

	m, err := manifest.Parse(nil, "graft.toml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(m.Sources) != 0 {
		t.Errorf("len(Sources) = %d, want 0", len(m.Sources))
	}
}

func TestLoad_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	_, err := manifest.Load(filepath.Join(dir, "graft.toml"))
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
	if got, want := err.Error(), "graft.toml not found"; got != want {
		t.Errorf("Load() error = %q, want %q", got, want)
	}
	if entries := snapshot(t, dir); len(entries) != 1 {
		t.Errorf("tree changed: %v", entries)
	}
}

func TestParse_BadTOML(t *testing.T) {
	t.Parallel()

	m, err := manifest.Parse([]byte("[sources.a\n"), "graft.toml")
	if err == nil {
		t.Fatal("Parse() error = nil, want an error")
	}
	if !strings.HasPrefix(err.Error(), "graft.toml: ") {
		t.Errorf("Parse() error = %q, want prefix %q", err.Error(), "graft.toml: ")
	}
	if m != nil {
		t.Errorf("Parse() manifest = %+v, want nil", m)
	}
}

func TestParse_Errors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   string
		want string
	}{
		"missing git": {
			in: `[sources.openspec-schemas]
rev     = "v1.2.0"
install = ["schema:tdd"]
`,
			want: `graft.toml: source "openspec-schemas": git is required`,
		},
		"empty git": {
			in: `[sources.openspec-schemas]
git     = ""
rev     = "v1.2.0"
install = ["schema:tdd"]
`,
			want: `graft.toml: source "openspec-schemas": git is required`,
		},
		"missing rev": {
			in: `[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
install = ["schema:tdd"]
`,
			want: `graft.toml: source "openspec-schemas": rev is required`,
		},
		"empty install list": {
			in: `[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
rev     = "v1.2.0"
install = []
`,
			want: `graft.toml: source "openspec-schemas": install must list at least one selector`,
		},
		"absent install list": {
			in: `[sources.openspec-schemas]
git = "github.com/optioni/openspec-schemas"
rev = "v1.2.0"
`,
			want: `graft.toml: source "openspec-schemas": install must list at least one selector`,
		},
		"empty source name": {
			in: `[sources.""]
git     = "github.com/optioni/openspec-schemas"
rev     = "v1.2.0"
install = ["schema:tdd"]
`,
			want: `graft.toml: source name is empty`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, err := manifest.Parse([]byte(tc.in), "graft.toml")
			if err == nil {
				t.Fatalf("Parse() error = nil, want %q", tc.want)
			}
			if got := err.Error(); got != tc.want {
				t.Errorf("Parse() error = %q, want %q", got, tc.want)
			}
			if m != nil {
				t.Errorf("Parse() manifest = %+v, want nil", m)
			}
		})
	}
}

func TestParse_Selectors(t *testing.T) {
	t.Parallel()

	in := `[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
rev     = "v1.2.0"
install = ["schema:tdd", "agent:*", "agent:outside-in-*"]
`
	m, err := manifest.Parse([]byte(in), "graft.toml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	want := []string{"schema:tdd", "agent:*", "agent:outside-in-*"}
	got := m.Sources[0].Install
	if len(got) != len(want) {
		t.Fatalf("Install = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Install[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParse_SelectorErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		install string
		want    string
	}{
		"no separator": {
			`["tdd"]`,
			`graft.toml: source "openspec-schemas": invalid selector "tdd": want kind:name`,
		},
		"empty name": {
			`["schema:"]`,
			`graft.toml: source "openspec-schemas": invalid selector "schema:": want kind:name`,
		},
		"empty kind": {
			`[":tdd"]`,
			`graft.toml: source "openspec-schemas": invalid selector ":tdd": want kind:name`,
		},
		"two separators": {
			`["schema:tdd:extra"]`,
			`graft.toml: source "openspec-schemas": invalid selector "schema:tdd:extra": want kind:name`,
		},
		"duplicate": {
			`["agent:*", "agent:*"]`,
			`graft.toml: source "openspec-schemas": duplicate selector "agent:*"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := `[sources.openspec-schemas]
git     = "github.com/optioni/openspec-schemas"
rev     = "v1.2.0"
install = ` + tc.install + "\n"
			m, err := manifest.Parse([]byte(in), "graft.toml")
			if err == nil {
				t.Fatalf("Parse() error = nil, want %q", tc.want)
			}
			if got := err.Error(); got != tc.want {
				t.Errorf("Parse() error = %q, want %q", got, tc.want)
			}
			if m != nil {
				t.Errorf("Parse() manifest = %+v, want nil", m)
			}
		})
	}
}

func TestParse_KindOverride(t *testing.T) {
	t.Parallel()

	in := minimal + `
[sources.openspec-schemas.kinds]
agent = ".codex/agents/"
`
	m, err := manifest.Parse([]byte(in), "graft.toml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	kinds := m.Sources[0].Kinds
	if len(kinds) != 1 {
		t.Fatalf("Kinds = %v, want exactly one override", kinds)
	}
	if got, want := kinds["agent"], ".codex/agents/"; got != want {
		t.Errorf("Kinds[agent] = %q, want %q (trailing slash preserved)", got, want)
	}

	plain, err := manifest.Parse([]byte(minimal), "graft.toml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if n := len(plain.Sources[0].Kinds); n != 0 {
		t.Errorf("Kinds = %v, want zero overrides", plain.Sources[0].Kinds)
	}
}

func TestParse_KindOverrideEmpty(t *testing.T) {
	t.Parallel()

	in := minimal + `
[sources.openspec-schemas.kinds]
agent = ""
`
	want := `graft.toml: source "openspec-schemas": kind "agent": destination is required`
	m, err := manifest.Parse([]byte(in), "graft.toml")
	if err == nil {
		t.Fatalf("Parse() error = nil, want %q", want)
	}
	if got := err.Error(); got != want {
		t.Errorf("Parse() error = %q, want %q", got, want)
	}
	if m != nil {
		t.Errorf("Parse() manifest = %+v, want nil", m)
	}
}

func TestParse_UnknownKey(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   string
		want string
	}{
		"misspelled source field": {
			in: `[sources.openspec-schemas]
git      = "github.com/optioni/openspec-schemas"
revision = "v1.2.0"
install  = ["schema:tdd"]
`,
			want: `graft.toml: source "openspec-schemas": unknown key "revision"`,
		},
		"unknown top-level key": {
			in:   "version = 1\n" + minimal,
			want: `graft.toml: unknown key "version"`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			m, err := manifest.Parse([]byte(tc.in), "graft.toml")
			if err == nil {
				t.Fatalf("Parse() error = nil, want %q", tc.want)
			}
			if got := err.Error(); got != tc.want {
				t.Errorf("Parse() error = %q, want %q", got, tc.want)
			}
			if m != nil {
				t.Errorf("Parse() manifest = %+v, want nil", m)
			}
		})
	}
}

func TestParse_GitVerbatim(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]string{
		"shorthand": "github.com/optioni/openspec-schemas",
		"scp url":   "git@github.com:optioni/openspec-schemas.git",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := `[sources.openspec-schemas]
git     = "` + want + `"
rev     = "v1.2.0"
install = ["schema:tdd"]
`
			m, err := manifest.Parse([]byte(in), "graft.toml")
			if err != nil {
				t.Fatalf("Parse() error = %v, want nil", err)
			}
			if got := m.Sources[0].Git; got != want {
				t.Errorf("Git = %q, want %q verbatim", got, want)
			}
		})
	}
}

func TestLoad_Unreadable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "graft.toml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("creating directory fixture: %v", err)
	}

	_, err := manifest.Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}
	if !strings.HasPrefix(err.Error(), "graft.toml: ") {
		t.Errorf("Load() error = %q, want prefix %q", err.Error(), "graft.toml: ")
	}
	if err.Error() == "graft.toml not found" {
		t.Error("Load() reported a present-but-unreadable file as not found")
	}
}

// TestParse_UnknownKeyIsLowest pins which key is reported when a file carries several,
// so the message does not depend on the decoder's key order.
func TestParse_UnknownKeyIsLowest(t *testing.T) {
	t.Parallel()

	in := "zzz = 1\naaa = 2\n" + minimal
	want := `graft.toml: unknown key "aaa"`
	_, err := manifest.Parse([]byte(in), "graft.toml")
	if err == nil {
		t.Fatalf("Parse() error = nil, want %q", want)
	}
	if got := err.Error(); got != want {
		t.Errorf("Parse() error = %q, want %q", got, want)
	}
}
