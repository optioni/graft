package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// TestLoad_NotARegularFile: Load's contract is that it reads no path other than the one
// it was given, and os.ReadFile follows a symlink to whatever the invoking user can read.
//
// The link's target is a *valid* catalog carrying a distinctive string on purpose. With
// an invalid target the test would pass against an implementation that follows the link,
// because the parse would fail either way — and the leak is that the target's lines come
// back in the decoder's error, so the message is asserted not to hold them.
func TestLoad_NotARegularFile(t *testing.T) {
	t.Parallel()

	const secret = "version: 1\nkinds:\n  smuggled:\n    to: \"outside/{name}\"\n"

	tests := []struct {
		name  string
		build func(t *testing.T, dir, path string)
	}{
		{
			name: "a symlink to a readable file",
			build: func(t *testing.T, dir, path string) {
				target := filepath.Join(dir, "target.yaml")
				if err := os.WriteFile(target, []byte(secret), 0o600); err != nil {
					t.Fatalf("writing target: %v", err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatalf("Symlink: %v", err)
				}
			},
		},
		{
			name: "a dangling symlink",
			build: func(t *testing.T, dir, path string) {
				if err := os.Symlink(filepath.Join(dir, "gone.yaml"), path); err != nil {
					t.Fatalf("Symlink: %v", err)
				}
			},
		},
		{
			name: "a directory",
			build: func(t *testing.T, dir, path string) {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatalf("Mkdir: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			path := filepath.Join(dir, "catalog.yaml")
			tt.build(t, dir, path)

			c, err := catalog.Load(path)
			if c != nil {
				t.Errorf("Load() catalog = %+v, want nil", c)
			}
			const want = "catalog.yaml: not a regular file"
			if err == nil || err.Error() != want {
				t.Fatalf("Load() error = %v, want %q", err, want)
			}
			if strings.Contains(err.Error(), "smuggled") {
				t.Errorf("Load() error = %q: the target's contents leaked into the message", err)
			}
			if strings.Contains(err.Error(), "not graftable") {
				t.Errorf("Load() error = %q, want a read error rather than the not-graftable one", err)
			}
		})
	}
}
