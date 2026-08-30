package cli_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// The outer loop for `graft list`: the real binary, a real repository synced by a real
// `graft sync`, and a real fetch cache. It is the tier that can fail for a reason no unit
// or integration test can — a document on the wrong stream, a trailing newline written
// twice, a listing built against the wrong root, an exit code that says success while the
// document went nowhere.
//
// The subprocess carries its own working directory and its own environment, so nothing here
// calls t.Chdir or t.Setenv: both are process-global, and the process running the tests is
// not the process under test.

// snapshotTree records every path under dir and the bytes of every regular file, so a
// command's promise to change nothing can be asserted as a comparison of two values rather
// than as a spot check of the files a test happened to think of.
func snapshotTree(t *testing.T, dir string) map[string]string {
	t.Helper()

	out := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			out[filepath.ToSlash(rel)+"/"] = ""
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", dir, err)
	}
	return out
}

// fixtureDocument is the document the seeded fixture lists as, built from the values only
// the running test knows. The bytes either side of them are the contract, written out in
// full rather than assembled from the same code the implementation uses.
func fixtureDocument(repoDir, sha string) string {
	return fmt.Sprintf(`{
  "version": 1,
  "sources": [
    {
      "name": "shared",
      "git": %q,
      "rev": "v1.0.0",
      "resolved": %q,
      "items": [
        {
          "id": "agent:reviewer",
          "kind": "agent",
          "name": "reviewer",
          "files": [
            ".claude/agents/reviewer.md"
          ]
        },
        {
          "id": "schema:tdd",
          "kind": "schema",
          "name": "tdd",
          "files": [
            "openspec/schemas/tdd/schema.yaml",
            "openspec/schemas/tdd/templates/design.md"
          ]
        }
      ]
    }
  ]
}
`, repoDir, sha)
}

// The headline scenario, end to end: a repository synced by a real `graft sync` lists as
// exactly the document the contract names, on standard output, with the error stream empty
// and the tree untouched.
func TestGraftListJSONReportsWhatWasInstalled(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	sha := repo.commit("v1")
	repo.tag("v1.0.0")

	c := newConsumer(t, manifestFor(repo, "v1.0.0", "schema:tdd", "agent:*"))
	if synced := runGraftIn(t, bin, c.dir, c.env, "sync"); synced.code != 0 {
		t.Fatalf("seeding sync: exit %d\nstderr:\n%s", synced.code, synced.stderr)
	}

	before := snapshotTree(t, c.dir)

	got := runGraftIn(t, bin, c.dir, c.env, "list", "--json")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if want := fixtureDocument(repo.dir, sha); got.stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", got.stdout, want)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty: the document is the whole of what list says", got.stderr)
	}

	after := snapshotTree(t, c.dir)
	if len(after) != len(before) {
		t.Errorf("the tree gained or lost paths: %d before, %d after", len(before), len(after))
	}
	for path, want := range before {
		if got, ok := after[path]; !ok {
			t.Errorf("%s is gone after a listing", path)
		} else if got != want {
			t.Errorf("%s changed: %q, want %q", path, got, want)
		}
	}
}
