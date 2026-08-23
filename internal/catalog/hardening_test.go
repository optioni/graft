package catalog_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/catalog"
)

// A stray document separator must not silently discard everything below it. A glob
// selector still matches the surviving items, so the no-match guard never fires and the
// install is short without saying so — the same silent under-install the name
// restriction exists to prevent.
func TestParse_RejectsMultipleDocuments(t *testing.T) {
	t.Parallel()

	data := []byte("version: 1\nkinds:\n  agent:\n    to: \"a/\"\n---\nversion: 2\n")
	_, err := catalog.Parse(data, "catalog.yaml")
	if err == nil {
		t.Fatal("Parse() = nil error, want an error naming the extra document")
	}
	if !strings.Contains(err.Error(), "document") {
		t.Errorf("Parse() error = %q, want it to name the extra document", err)
	}
}

// dcb020e added the uint64 branch so an out-of-range version says "upgrade graft", but
// goccy returns a string for a literal wider than uint64 — landing on the one message
// that does not help.
func TestParse_VersionBeyondUint64SaysUpgrade(t *testing.T) {
	t.Parallel()

	data := []byte("version: 99999999999999999999999999\n")
	_, err := catalog.Parse(data, "catalog.yaml")
	if err == nil {
		t.Fatal("Parse() = nil error, want an unsupported-version error")
	}
	if !strings.Contains(err.Error(), "upgrade graft") {
		t.Errorf("Parse() error = %q, want it to say to upgrade graft", err)
	}
}

// The duplicate-destination guard exists so a kind cannot write every item twice. Two
// spellings of one directory defeat a byte comparison but not filepath.Join.
func TestParse_RejectsAliasedDuplicateDestinations(t *testing.T) {
	t.Parallel()

	for name, to := range map[string]string{
		"trailing slash": `[".claude/agents/", ".claude/agents//"]`,
		"dot prefix":     `["a/b", "./a/b"]`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			data := []byte("version: 1\nkinds:\n  agent:\n    to: " + to + "\n")
			if _, err := catalog.Parse(data, "catalog.yaml"); err == nil {
				t.Errorf("Parse() = nil error for to: %s, want a duplicate-destination error", to)
			}
		})
	}
}

// catalog.yaml in a source repository is attacker-controlled content, and git can
// materialise it as a symlink. Load's own contract says it reads no path other than the
// one it was given; os.ReadFile resolving a link breaks that, and goccy quotes offending
// source lines verbatim in parse errors.
func TestLoad_RefusesSymlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	secret := filepath.Join(dir, "secret.txt")
	if err := os.WriteFile(secret, []byte("PRIVATE KEY MATERIAL\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "catalog.yaml")
	if err := os.Symlink(secret, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, err := catalog.Load(link)
	if err == nil {
		t.Fatal("Load() = nil error, want a refusal to follow a symlink")
	}
	// A parse error is not good enough: it means the link was followed and the file
	// read. The refusal must happen before any content is touched.
	if !strings.Contains(err.Error(), "regular file") {
		t.Errorf("Load() error = %q, want a refusal naming that it is not a regular file", err)
	}
	if strings.Contains(err.Error(), "PRIVATE KEY MATERIAL") {
		t.Errorf("Load() error leaks linked file contents: %q", err)
	}
}
