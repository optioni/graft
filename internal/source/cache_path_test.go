package source

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEntryPath covers the derivation. Every case asserts the cache root was not
// created: deriving a path is a pure function a caller may ask speculatively, and one
// that made a directory could not be.
func TestEntryPath(t *testing.T) {
	cases := []struct {
		name string
		git  string
		want string
	}{
		{
			name: "the cache path mirrors the remote and the sha",
			git:  "https://github.com/optioni/openspec-schemas",
			want: "github.com/optioni/openspec-schemas/" + testSHA,
		},
		{
			name: "an ssh address derives the same path as HTTPS",
			git:  "git@github.com:optioni/openspec-schemas.git",
			want: "github.com/optioni/openspec-schemas/" + testSHA,
		},
		{
			name: "a .git suffix and a trailing slash together are the same entry",
			git:  "https://github.com/optioni/openspec-schemas.git/",
			want: "github.com/optioni/openspec-schemas/" + testSHA,
		},
		{
			name: "a user and a port do not change the path",
			git:  "ssh://git@github.com:2222/optioni/openspec-schemas",
			want: "github.com/optioni/openspec-schemas/" + testSHA,
		},
		{
			name: "a filesystem remote gets an entry under local",
			git:  "/srv/mirrors/assets",
			want: "local/srv/mirrors/assets/" + testSHA,
		},
		{
			name: "a relative filesystem remote also lands under local",
			git:  "../sibling-repo",
			want: "local/_../sibling-repo/" + testSHA,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "cache")
			got, err := Cache{Root: root}.Entry("shared", c.git, testSHA)
			if err != nil {
				t.Fatalf("Entry(%q): unexpected error: %v", c.git, err)
			}
			want := filepath.Join(root, filepath.FromSlash(c.want))
			if got != want {
				t.Errorf("Entry(%q):\n got %q\nwant %q", c.git, got, want)
			}
			if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("Entry(%q): created the cache root; deriving a path must create nothing", c.git)
			}
		})
	}
}

// TestEntryPathSameRepositoryOverSSHAndHTTPS is the identity claim stated directly: the
// same repository addressed two ways is fetched once rather than twice.
func TestEntryPathSameRepositoryOverSSHAndHTTPS(t *testing.T) {
	c := Cache{Root: filepath.Join(t.TempDir(), "cache")}
	urls := []string{
		"https://github.com/optioni/openspec-schemas",
		"git@github.com:optioni/openspec-schemas.git",
		"https://github.com/optioni/openspec-schemas.git/",
	}
	var first string
	for _, u := range urls {
		got, err := c.Entry("shared", u, testSHA)
		if err != nil {
			t.Fatalf("Entry(%q): unexpected error: %v", u, err)
		}
		if first == "" {
			first = got
			continue
		}
		if got != first {
			t.Errorf("Entry(%q):\n got %q\nwant the same entry as the first form, %q", u, got, first)
		}
	}
}

// TestEntryPathCannotEscapeTheCacheRoot is the containment guarantee. It is asserted
// with filepath.Rel rather than by inspecting the string, because filepath.Rel is what
// actually answers the question.
func TestEntryPathCannotEscapeTheCacheRoot(t *testing.T) {
	hostile := []string{
		"https://example.com/../../etc/passwd",
		"https://example.com/..%2f..%2fetc",
		"https://example.com/%2e%2e/%2e%2e/etc",
		"https://../a",
		"https://./a",
		"git@..:../../etc/x",
		"/../../etc/passwd",
		"../../../../../../etc",
		"https://example.com/a/../../../b",
		"https://example.com//////a",
		"https:///onlypath",
		"example.com/a\x00b/c",
		"/",
		".",
		"..",
		"",
	}
	root := filepath.Join(t.TempDir(), "cache")
	for _, u := range hostile {
		entry, err := Cache{Root: root}.Entry("shared", u, testSHA)
		if err != nil {
			// Refusing outright is a fine answer; escaping is not.
			continue
		}
		rel, err := filepath.Rel(root, entry)
		if err != nil {
			t.Errorf("Entry(%q) = %q: not relative to the cache root: %v", u, entry, err)
			continue
		}
		for _, seg := range strings.Split(rel, string(filepath.Separator)) {
			if seg == ".." {
				t.Errorf("Entry(%q) = %q: escapes the cache root (relative path %q)", u, entry, rel)
				break
			}
		}
		if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Entry(%q): created the cache root", u)
		}
	}
}

// TestEntryPathRefusesANonSHA pins the wording internal/plan already produces, and which
// internal/lock carries as the tail of its own. Three packages must not disagree about
// what a valid resolved is.
func TestEntryPathRefusesANonSHA(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	want := `source "shared": resolved "not-a-sha" is not a 40-character hex sha`

	entry, err := Cache{Root: root}.Entry("shared", "https://example.com/a/b", "not-a-sha")
	if err == nil {
		t.Fatalf("Entry: want an error, got %q", entry)
	}
	if err.Error() != want {
		t.Errorf("Entry:\n got %q\nwant %q", err.Error(), want)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Entry: created the cache root while refusing a bad sha")
	}
}

// TestDefaultCacheRoot drives the seam directly with stub lookups. A test that had to
// set HOME is one edit away from a test that writes to the developer's real cache.
func TestDefaultCacheRoot(t *testing.T) {
	base := t.TempDir()
	home := filepath.Join(base, "home", "dev")

	cases := []struct {
		name    string
		xdg     string
		home    string
		homeErr error
		want    string
		wantErr string
	}{
		{
			name: "the default root under a home directory",
			home: home,
			want: filepath.Join(home, ".cache", "graft"),
		},
		{
			name: "XDG_CACHE_HOME moves the default root",
			xdg:  filepath.Join(base, "var", "cache"),
			home: home,
			want: filepath.Join(base, "var", "cache", "graft"),
		},
		{
			// A cache root that moves with the working directory would give the same
			// source a different entry per directory, which a content-addressed cache
			// may not do.
			name: "a relative XDG_CACHE_HOME is ignored",
			xdg:  "relative/cache",
			home: home,
			want: filepath.Join(home, ".cache", "graft"),
		},
		{
			name:    "no home directory and no XDG_CACHE_HOME is an error",
			homeErr: errors.New("$HOME is not defined"),
			wantErr: "cannot determine the cache root: ",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			getenv := func(k string) string {
				if k == "XDG_CACHE_HOME" {
					return c.xdg
				}
				return ""
			}
			homeFn := func() (string, error) { return c.home, c.homeErr }

			got, err := defaultCacheRoot(getenv, homeFn)
			switch {
			case c.wantErr != "":
				if err == nil {
					t.Fatalf("defaultCacheRoot: want an error, got %q", got)
				}
				if !strings.HasPrefix(err.Error(), c.wantErr) {
					t.Errorf("defaultCacheRoot:\n got %q\nwant a message beginning %q", err, c.wantErr)
				}
				if got != "" {
					t.Errorf("defaultCacheRoot: want an empty root on failure, got %q", got)
				}
			default:
				if err != nil {
					t.Fatalf("defaultCacheRoot: unexpected error: %v", err)
				}
				if got != c.want {
					t.Errorf("defaultCacheRoot:\n got %q\nwant %q", got, c.want)
				}
			}
			// Nothing is made until the first entry is written.
			for _, p := range []string{filepath.Join(home, ".cache"), filepath.Join(home, ".cache", "graft"), got} {
				if p == "" {
					continue
				}
				if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("defaultCacheRoot: created %q; computing the default must create nothing", p)
				}
			}
		})
	}
}

// TestDefaultCacheRootReadsTheRealEnvironment covers the exported wrapper, which is the
// only place os.Getenv and os.UserHomeDir are consulted.
func TestDefaultCacheRootReadsTheRealEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", dir)

	got, err := DefaultCacheRoot()
	if err != nil {
		t.Fatalf("DefaultCacheRoot: unexpected error: %v", err)
	}
	if want := filepath.Join(dir, "graft"); got != want {
		t.Errorf("DefaultCacheRoot:\n got %q\nwant %q", got, want)
	}
}
