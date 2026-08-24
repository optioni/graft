package source

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// entryDir builds a fetched entry by hand. Listing walks a real directory either way, so
// a fixture repository is only needed where the *fetch* is under test.
func entryDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for path, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile %s: %v", path, err)
		}
	}
	return dir
}

func item(id, from string) catalog.Item {
	kind, name, _ := strings.Cut(id, ":")
	return catalog.Item{ID: id, Kind: kind, Name: name, From: from}
}

func TestReadCatalogParsesAFetchedTree(t *testing.T) {
	entry := entryDir(t, map[string]string{
		"catalog.yaml": "version: 1\n" +
			"kinds:\n  schema:\n    to: \"openspec/schemas/{name}\"\n" +
			"provides:\n  - { kind: schema, name: tdd, from: extras/tdd }\n",
		"extras/tdd/schema.yaml": "x\n",
	})
	before := snapshot(t, entry)

	c, err := ReadCatalog(entry)
	if err != nil {
		t.Fatalf("ReadCatalog: unexpected error: %v", err)
	}
	if c.Version != 1 {
		t.Errorf("Version: got %d, want 1", c.Version)
	}
	if len(c.Kinds) != 1 {
		t.Errorf("Kinds: got %d, want 1", len(c.Kinds))
	}
	if len(c.Items) != 1 || c.Items[0].ID != "schema:tdd" {
		t.Errorf("Items: got %+v, want one item schema:tdd", c.Items)
	}
	assertSame(t, "entry", before, snapshot(t, entry))
}

// TestReadCatalogMissing asserts internal/catalog's own wording. A second spelling of
// "not graftable" would give the failure two owners.
func TestReadCatalogMissing(t *testing.T) {
	entry := entryDir(t, map[string]string{"extras/tdd/schema.yaml": "x\n"})
	want := "catalog.yaml not found: the source is not graftable"

	c, err := ReadCatalog(entry)
	if err == nil {
		t.Fatalf("ReadCatalog: want an error, got %+v", c)
	}
	if err.Error() != want {
		t.Errorf("ReadCatalog:\n got %q\nwant %q", err.Error(), want)
	}
	if c != nil {
		t.Errorf("ReadCatalog: want a nil catalog on failure, got %+v", c)
	}
}

// TestReadCatalogSymlinkEscape: a source commits its own catalog.yaml, so it may commit
// a symlink under that name, and an ordinary os.ReadFile follows one.
func TestReadCatalogSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	const secret = "SECRET-CATALOG-CONTENT\n"
	outside := filepath.Join(base, "outside.yaml")
	if err := os.WriteFile(outside, []byte(secret), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entry := filepath.Join(base, "entry")
	if err := os.Mkdir(entry, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(entry, "catalog.yaml")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	c, err := ReadCatalog(entry)
	if err == nil {
		t.Fatalf("ReadCatalog: want an error, got %+v", c)
	}
	if c != nil {
		t.Errorf("ReadCatalog: want a nil catalog, got %+v", c)
	}
	if strings.Contains(err.Error(), strings.TrimSpace(secret)) {
		t.Errorf("ReadCatalog: the outside file's contents leaked into the error: %q", err)
	}
}

func TestListFileFromListsExactlyThatFile(t *testing.T) {
	entry := entryDir(t, map[string]string{
		"extras/agents/apply-orchestrator.md": "orchestrator\n",
	})

	got, err := List(entry, "shared", item("agent:apply-orchestrator", "extras/agents/apply-orchestrator.md"))
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if got.Dir {
		t.Errorf("Dir: got true, want false for a from naming a file")
	}
	// The base name, relative to from, as destination-computation requires.
	if want := []string{"apply-orchestrator.md"}; !slices.Equal(got.Files, want) {
		t.Errorf("Files:\n got %q\nwant %q", got.Files, want)
	}
}

func TestListDirectoryFromListsItsTree(t *testing.T) {
	entry := entryDir(t, map[string]string{
		"extras/tdd/schema.yaml":           "x\n",
		"extras/tdd/templates/proposal.md": "p\n",
		"extras/tdd/templates/design.md":   "d\n",
		"extras/other/ignored.md":          "i\n",
	})

	got, err := List(entry, "shared", item("schema:tdd", "extras/tdd"))
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if !got.Dir {
		t.Errorf("Dir: got false, want true for a from naming a directory")
	}
	// Exact equality including order: sorted, slash-separated, relative to from. A set
	// comparison would let an unsorted listing churn graft.lock's files on every sync.
	want := []string{"schema.yaml", "templates/design.md", "templates/proposal.md"}
	if !slices.Equal(got.Files, want) {
		t.Errorf("Files:\n got %q\nwant %q", got.Files, want)
	}
}

func TestListEmptyDirectories(t *testing.T) {
	entry := t.TempDir()
	for _, dir := range []string{"extras/empty", "extras/hollow/a/b", "extras/hollow/c"} {
		if err := os.MkdirAll(filepath.Join(entry, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	for _, c := range []struct{ name, from string }{
		{"an empty directory", "extras/empty"},
		{"a directory holding only empty subdirectories", "extras/hollow"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := List(entry, "shared", item("schema:empty", c.from))
			if err != nil {
				t.Fatalf("List: unexpected error: %v", err)
			}
			if !got.Dir {
				t.Errorf("Dir: got false, want true")
			}
			if len(got.Files) != 0 {
				t.Errorf("Files: got %q, want none", got.Files)
			}
		})
	}
}

// TestListSkipsSymlink: a symlink is not content, and following one is how a source aims
// a read at whatever sits beside its own tree. It is skipped rather than refused, so one
// stray link cannot make an otherwise valid source unusable.
func TestListSkipsSymlink(t *testing.T) {
	entry := entryDir(t, map[string]string{"extras/tdd/real.md": "real\n"})
	link := filepath.Join(entry, "extras", "tdd", "link.md")
	if err := os.Symlink("../../../../etc/passwd", link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	got, err := List(entry, "shared", item("schema:tdd", "extras/tdd"))
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if want := []string{"real.md"}; !slices.Equal(got.Files, want) {
		t.Errorf("Files:\n got %q\nwant %q", got.Files, want)
	}
}

func TestListErrors(t *testing.T) {
	entry := entryDir(t, map[string]string{"extras/real/a.md": "a\n"})
	if err := os.Symlink(filepath.Join(entry, "extras", "real"), filepath.Join(entry, "extras", "tdd")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	cases := []struct {
		name string
		from string
		want string
	}{
		{
			name: "a from that does not exist",
			from: "extras/gone",
			want: `source "shared": item "schema:tdd": from "extras/gone" not found in the source tree`,
		},
		{
			name: "a from naming a symlink",
			from: "extras/tdd",
			want: `source "shared": item "schema:tdd": from "extras/tdd" is not a regular file or directory`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := List(entry, "shared", item("schema:tdd", c.from))
			if err == nil {
				t.Fatalf("List: want an error, got %+v", got)
			}
			if err.Error() != c.want {
				t.Errorf("List:\n got %q\nwant %q", err.Error(), c.want)
			}
			// Empty, so no caller can plan a write from a failed listing.
			if got.Dir || len(got.Files) != 0 {
				t.Errorf("List: want an empty listing on failure, got %+v", got)
			}
		})
	}
}

// TestListSymlinkedParentIsRefused is the case that refusing the last component alone
// does not cover. os.Lstat does not follow the final element but does resolve every
// intermediate one, and catalog.inSource sees nothing wrong with "extras/tdd" as a
// string — it is relative, cleaned, and carries no "..".
func TestListSymlinkedParentIsRefused(t *testing.T) {
	base := t.TempDir()
	outside := filepath.Join(base, "outside", "tdd")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	const secret = "PRIVATE-KEY-MATERIAL\n"
	if err := os.WriteFile(filepath.Join(outside, "id_rsa"), []byte(secret), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entry := filepath.Join(base, "entry")
	if err := os.Mkdir(entry, 0o755); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if err := os.Symlink(filepath.Join(base, "outside"), filepath.Join(entry, "extras")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	got, err := List(entry, "shared", item("schema:tdd", "extras/tdd"))
	if err == nil {
		t.Fatalf("List: want an error, got %+v", got)
	}
	if slices.Contains(got.Files, "id_rsa") {
		t.Errorf("List: listed a file from outside the entry: %q", got.Files)
	}
	if strings.Contains(err.Error(), strings.TrimSpace(secret)) {
		t.Errorf("List: the outside file's contents leaked into the error: %q", err)
	}
}

// TestListSubmoduleListsNothing pins the outcome rather than leaving it incidental: a
// gitlink is checked out as an empty directory, nothing is cloned, and no second remote
// is contacted — so a submodule is not a way for a source to reach a repository the
// consumer never named.
func TestListSubmoduleListsNothing(t *testing.T) {
	inner := newRepo(t)
	inner.write("secret.md", "inner\n")
	inner.commit("inner")

	outer := newRepo(t)
	outer.write("catalog.yaml", "version: 1\n")
	outer.git("-C", outer.URL(), "-c", "protocol.file.allow=always",
		"submodule", "add", "-q", inner.URL(), "extras/tdd")
	sha := outer.commit("with submodule")

	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	entry, err := c.Fetch("shared", outer.URL(), sha)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}

	got, err := List(entry, "shared", item("schema:tdd", "extras/tdd"))
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if !got.Dir {
		t.Errorf("Dir: got false, want true — a gitlink is checked out as a directory")
	}
	if len(got.Files) != 0 {
		t.Errorf("Files: got %q, want none; the submodule must not have been cloned", got.Files)
	}
}

// TestListChangesNothing takes every listing above against one entry and asserts the
// entry is identical afterwards, and that a consumer tree beside it is untouched.
func TestListChangesNothing(t *testing.T) {
	entry := entryDir(t, map[string]string{
		"catalog.yaml":                        "version: 1\n",
		"extras/tdd/schema.yaml":              "x\n",
		"extras/tdd/templates/proposal.md":    "p\n",
		"extras/agents/apply-orchestrator.md": "o\n",
	})
	consumer := entryDir(t, map[string]string{
		"graft.toml":                       "[sources.shared]\n",
		".claude/agents/local-reviewer.md": "repo-owned\n",
	})
	beforeEntry, beforeConsumer := snapshot(t, entry), snapshot(t, consumer)

	for _, it := range []catalog.Item{
		item("schema:tdd", "extras/tdd"),
		item("agent:apply-orchestrator", "extras/agents/apply-orchestrator.md"),
		item("schema:gone", "extras/gone"),
	} {
		_, _ = List(entry, "shared", it)
	}
	if _, err := ReadCatalog(entry); err != nil {
		t.Fatalf("ReadCatalog: %v", err)
	}

	assertSame(t, "entry", beforeEntry, snapshot(t, entry))
	assertSame(t, "consumer tree", beforeConsumer, snapshot(t, consumer))
}
