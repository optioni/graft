package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// example is SPEC.md's own catalog.yaml, verbatim apart from the two agents it lists.
const example = `version: 1

kinds:
  schema:
    to: "openspec/schemas/{name}"
  agent:
    to: ".claude/agents/"
    flatten: true

provides:
  - { kind: schema, name: tdd, from: extras/openspec-schemas/tdd }
  - { kind: agent, name: apply-orchestrator, from: extras/agents/apply-orchestrator.md }
`

// writeCatalog drops content into a fresh temp dir and returns the path it wrote.
func writeCatalog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
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

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestLoad_Valid(t *testing.T) {
	t.Parallel()

	path := writeCatalog(t, example)
	dir := filepath.Dir(path)
	before := snapshot(t, dir)

	c, err := catalog.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if c.Version != 1 {
		t.Errorf("Version = %d, want 1", c.Version)
	}
	if len(c.Kinds) != 2 {
		t.Fatalf("len(Kinds) = %d, want 2", len(c.Kinds))
	}
	if len(c.Items) != 2 {
		t.Fatalf("len(Items) = %d, want 2", len(c.Items))
	}

	if after := snapshot(t, dir); !sameStrings(before, after) {
		t.Errorf("tree changed: before %v, after %v", before, after)
	}
}

func TestLoad_Missing(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	before := snapshot(t, dir)

	c, err := catalog.Load(filepath.Join(dir, "catalog.yaml"))
	if c != nil {
		t.Errorf("Load() catalog = %+v, want nil", c)
	}
	const want = "catalog.yaml not found: the source is not graftable"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}

	if after := snapshot(t, dir); !sameStrings(before, after) {
		t.Errorf("tree changed: before %v, after %v", before, after)
	}
}

// A read that fails for any reason other than absence is not the not-graftable
// error: the source may well be graftable and the read is what went wrong.
func TestLoad_Unreadable(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "catalog.yaml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("creating fixture: %v", err)
	}

	c, err := catalog.Load(path)
	if c != nil {
		t.Errorf("Load() catalog = %+v, want nil", c)
	}
	if err == nil {
		t.Fatal("Load() error = nil, want a read error")
	}
	if !strings.HasPrefix(err.Error(), "catalog.yaml: ") {
		t.Errorf("Load() error = %q, want the %q prefix", err, "catalog.yaml: ")
	}
	if strings.Contains(err.Error(), "not graftable") {
		t.Errorf("Load() error = %q, want a read error rather than the not-graftable one", err)
	}
}

func TestParse_BadYAML(t *testing.T) {
	t.Parallel()

	c, err := catalog.Parse([]byte("version: 1\nkinds: [unclosed\n"), "catalog.yaml")
	if c != nil {
		t.Errorf("Parse() catalog = %+v, want nil", c)
	}
	if err == nil {
		t.Fatal("Parse() error = nil, want a decoder error")
	}
	if !strings.HasPrefix(err.Error(), "catalog.yaml: ") {
		t.Errorf("Parse() error = %q, want the %q prefix", err, "catalog.yaml: ")
	}
}

func TestParse_Shape(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "a sequence is not a mapping",
			in:   "- schema:tdd\n",
			want: "catalog.yaml: top level must be a mapping",
		},
		{
			name: "a scalar is not a mapping",
			in:   "7\n",
			want: "catalog.yaml: top level must be a mapping",
		},
		{
			name: "an empty file is a missing version",
			in:   "",
			want: "catalog.yaml: version is required",
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

func TestParse_Empty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
	}{
		{
			name: "kinds but no provides",
			in:   "version: 1\nkinds:\n  agent:\n    to: \".claude/agents/\"\n",
		},
		{
			name: "neither kinds nor provides",
			in:   "version: 1\n",
		},
		{
			name: "explicitly null kinds and provides",
			in:   "version: 1\nkinds:\nprovides:\n",
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
			if len(c.Items) != 0 {
				t.Errorf("len(Items) = %d, want 0", len(c.Items))
			}
		})
	}
}

func TestParse_NoKindsNoProvides(t *testing.T) {
	t.Parallel()

	c, err := catalog.Parse([]byte("version: 1\n"), "catalog.yaml")
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(c.Kinds) != 0 {
		t.Errorf("len(Kinds) = %d, want 0", len(c.Kinds))
	}
	if len(c.Items) != 0 {
		t.Errorf("len(Items) = %d, want 0", len(c.Items))
	}
}

func TestParse_VersionErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "missing version",
			in: "kinds:\n  agent:\n    to: \".claude/agents/\"\n" +
				"provides:\n  - { kind: agent, name: tdd, from: a.md }\n",
			want: "catalog.yaml: version is required",
		},
		{
			name: "a newer version beats an unknown key",
			in:   "version: 2\nrequires: []\n",
			want: "catalog.yaml: version 2 is not supported by this graft; upgrade graft",
		},
		{
			name: "version zero",
			in:   "version: 0\n",
			want: "catalog.yaml: version 0 is not a known catalog version",
		},
		{
			name: "a negative version",
			in:   "version: -1\n",
			want: "catalog.yaml: version -1 is not a known catalog version",
		},
		{
			name: "a non-integer version",
			in:   "version: \"1\"\n",
			want: "catalog.yaml: version must be an integer",
		},
		{
			name: "a fractional version",
			in:   "version: 1.5\n",
			want: "catalog.yaml: version must be an integer",
		},
		{
			name: "an explicitly null version",
			in:   "version:\n",
			want: "catalog.yaml: version is required",
		},
		{
			// Past what an int64 holds. It is still unambiguously newer than the one
			// version this graft knows, and it is printed as written, not clamped.
			name: "a version past the machine integer range",
			in:   "version: 9223372036854775808\n",
			want: "catalog.yaml: version 9223372036854775808 is not supported by this graft; upgrade graft",
		},
		{
			// Past what uint64 holds, so the decoder hands it back as a string —
			// the same Go type a quoted version arrives as. It is a version, and
			// the answer is the same one a narrower future version gets.
			name: "a version wider than any integer type",
			in:   "version: 99999999999999999999999999\n",
			want: "catalog.yaml: version 99999999999999999999999999 is not supported by this graft; upgrade graft",
		},
		{
			name: "a version wider than any integer type, with separators",
			in:   "version: 99_999_999_999_999_999_999_999_999\n",
			want: "catalog.yaml: version 99_999_999_999_999_999_999_999_999 is not supported by this graft; upgrade graft",
		},
		{
			name: "a signed version wider than any integer type",
			in:   "version: +99999999999999999999999999\n",
			want: "catalog.yaml: version +99999999999999999999999999 is not supported by this graft; upgrade graft",
		},
		{
			// Quoted, and the rule reads shape rather than the source token, so it
			// gets the same answer as the bare spelling. Both are refused; only
			// which refusal differs.
			name: "a quoted version wider than any integer type",
			in:   "version: \"99999999999999999999999999\"\n",
			want: "catalog.yaml: version 99999999999999999999999999 is not supported by this graft; upgrade graft",
		},
		{
			// Below 1 however it is written, so it is not a future format.
			name: "a hugely negative version",
			in:   "version: -99999999999999999999999999\n",
			want: "catalog.yaml: version -99999999999999999999999999 is not a known catalog version",
		},
		{
			name: "a boolean version",
			in:   "version: true\n",
			want: "catalog.yaml: version must be an integer",
		},
		{
			// Integer-shaped up to the last character, which is what stops the
			// shape rule from treating any long string as a version.
			name: "a wide literal with a trailing letter",
			in:   "version: \"99999999999999999999999999x\"\n",
			want: "catalog.yaml: version must be an integer",
		},
		{
			// -1 is a value the decoder could hold, so a string carrying it was
			// quoted deliberately: a string, not a version.
			name: "a quoted negative version that fits",
			in:   "version: \"-1\"\n",
			want: "catalog.yaml: version must be an integer",
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
