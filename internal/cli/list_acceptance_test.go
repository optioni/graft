package cli_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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

// `graft list` accepts --json and nothing else. The wording is graft's own rather than
// cobra's, so the refusal is the contract rather than a detail of how a subcommand reports
// an argument it did not want.
func TestGraftListArgumentSurface(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)

	t.Run("a positional argument is a usage error", func(t *testing.T) {
		t.Parallel()

		got := runGraft(t, bin, t.TempDir(), "list", "shared")

		if want := "graft: unknown argument \"shared\"\n" + hint + "\n"; got.stderr != want {
			t.Errorf("stderr = %q, want %q", got.stderr, want)
		}
		if got.stdout != "" {
			t.Errorf("stdout = %q, want empty", got.stdout)
		}
		if got.code != 1 {
			t.Errorf("exit code = %d, want 1", got.code)
		}
	})

	t.Run("only the first positional argument is named", func(t *testing.T) {
		t.Parallel()

		got := runGraft(t, bin, t.TempDir(), "list", "shared", "other")

		if !strings.Contains(got.stderr, `"shared"`) {
			t.Errorf("stderr does not name the first argument: %q", got.stderr)
		}
		if strings.Contains(got.stderr, "other") {
			t.Errorf("stderr names the second argument too: %q", got.stderr)
		}
		if got.code != 1 {
			t.Errorf("exit code = %d, want 1", got.code)
		}
	})

	t.Run("an unknown flag is a usage error", func(t *testing.T) {
		t.Parallel()

		got := runGraft(t, bin, t.TempDir(), "list", "--format=yaml")

		if want := "graft: unknown flag: --format\n" + hint + "\n"; got.stderr != want {
			t.Errorf("stderr = %q, want %q", got.stderr, want)
		}
		if got.stdout != "" {
			t.Errorf("stdout = %q, want empty", got.stdout)
		}
		if got.code != 1 {
			t.Errorf("exit code = %d, want 1", got.code)
		}
	})

	// A command that writes nothing has nothing to offer a dry run.
	t.Run("its help names only the flag it has", func(t *testing.T) {
		t.Parallel()

		got := runGraft(t, bin, t.TempDir(), "list", "--help")

		for _, want := range []string{"list", "--json"} {
			if !strings.Contains(got.stdout, want) {
				t.Errorf("help does not name %q:\n%s", want, got.stdout)
			}
		}
		if strings.Contains(got.stdout, "--dry-run") {
			t.Errorf("help offers --dry-run on a command that writes nothing:\n%s", got.stdout)
		}
		if got.stderr != "" || got.code != 0 {
			t.Errorf("stderr = %q, code = %d", got.stderr, got.code)
		}
	})
}

// A repository with nothing installed is a normal outcome, not an error and not an empty
// screen. The plain form says so where notes go and leaves stdout byte-empty, so a caller
// piping the listing receives zero bytes rather than a sentence that parses as an item.
func TestGraftListWithNothingInstalled(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)

	// The three repositories that hold nothing: never synced, synced away to nothing, and
	// a directory with no graft file at all — the last of which would fail if list read
	// graft.toml.
	for name, seed := range map[string]map[string]string{
		"a repository with no lock":      {"graft.toml": "[sources.shared]\ngit = \"example.test/r\"\nrev = \"main\"\n"},
		"a lock declaring no source":     {"graft.lock": "version = 1\n"},
		"a directory with no graft file": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			for path, content := range seed {
				if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o644); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
			}
			before := snapshotTree(t, dir)

			plain := runGraft(t, bin, dir, "list")
			if plain.stderr != "nothing installed\n" {
				t.Errorf("stderr = %q, want %q", plain.stderr, "nothing installed\n")
			}
			if plain.stdout != "" {
				t.Errorf("stdout = %q, want byte-empty", plain.stdout)
			}
			if plain.code != 0 {
				t.Errorf("exit code = %d, want 0", plain.code)
			}

			// The machine-readable form answers "nothing" in the same shape as "something":
			// a form that printed nothing would make it indistinguishable from a command
			// that did not run.
			doc := runGraft(t, bin, dir, "list", "--json")
			if want := "{\n  \"version\": 1,\n  \"sources\": []\n}\n"; doc.stdout != want {
				t.Errorf("stdout = %q, want %q", doc.stdout, want)
			}
			if doc.stderr != "" || doc.code != 0 {
				t.Errorf("stderr = %q, code = %d", doc.stderr, doc.code)
			}

			if after := snapshotTree(t, dir); len(after) != len(before) {
				t.Errorf("listing created something: %v, was %v", after, before)
			}
		})
	}
}

// A graft.lock that exists but cannot be read fails the run with internal/lock's own
// message, and stdout stays byte-empty in both forms: a --json invocation that failed emits
// no partial document, not even an opening brace.
func TestGraftListRefusesALockItCannotRead(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)

	for name, tc := range map[string]struct{ lock, want string }{
		"a lock from a newer graft": {
			"version = 2\n",
			"graft: graft.lock: version 2 is not supported by this graft; upgrade graft\n",
		},
		"a malformed lock": {
			"version = 1\n\n[[source]]\nname = \"shared\"\n",
			"graft: graft.lock: source \"shared\": git is required\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "graft.lock"), []byte(tc.lock), 0o644); err != nil {
				t.Fatalf("writing graft.lock: %v", err)
			}

			for _, args := range [][]string{{"list"}, {"list", "--json"}} {
				got := runGraft(t, bin, dir, args...)
				if got.stderr != tc.want {
					t.Errorf("%v: stderr = %q, want %q", args, got.stderr, tc.want)
				}
				if got.stdout != "" {
					t.Errorf("%v: stdout = %q, want byte-empty", args, got.stdout)
				}
				if got.code != 1 {
					t.Errorf("%v: exit code = %d, want 1", args, got.code)
				}
			}
		})
	}
}

// list reports what the lock claims, not what the tree holds: SPEC.md admits no
// verification command because git status is one, and a list that stat-ed its way to an
// answer would be that command under another name. The same run proves list needs neither
// the source repository nor the cache.
func TestGraftListNeedsNeitherTheTreeNorTheSource(t *testing.T) {
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

	before := runGraftIn(t, bin, c.dir, c.env, "list")
	if before.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", before.code, before.stderr)
	}

	// A file deleted by hand, the source repository gone, and the fetch cache emptied.
	if err := os.Remove(filepath.Join(c.dir, ".claude", "agents", "reviewer.md")); err != nil {
		t.Fatalf("deleting the installed file: %v", err)
	}
	if err := os.RemoveAll(repo.dir); err != nil {
		t.Fatalf("deleting the source repository: %v", err)
	}
	cacheRoot := filepath.Join(t.TempDir(), "cache")
	env := []string{"XDG_CACHE_HOME=" + cacheRoot}

	after := runGraftIn(t, bin, c.dir, env, "list")
	if after.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", after.code, after.stderr)
	}
	if after.stdout != before.stdout {
		t.Errorf("the listing changed:\n%s\nwas:\n%s", after.stdout, before.stdout)
	}

	doc := runGraftIn(t, bin, c.dir, env, "list", "--json")
	if want := fixtureDocument(repo.dir, sha); doc.stdout != want {
		t.Errorf("stdout:\n%s\nwant:\n%s", doc.stdout, want)
	}
	if !strings.Contains(doc.stdout, ".claude/agents/reviewer.md") {
		t.Error("a file the lock claims and the tree no longer holds is missing from the document")
	}

	// Nothing was fetched, so nothing was cached — and the cache root graft was pointed at
	// was never created.
	if _, err := os.Stat(cacheRoot); !os.IsNotExist(err) {
		t.Errorf("the cache root exists after a listing: %v", err)
	}
}

// The plain form goes to standard output, because a listing is the content the caller asked
// for rather than a summary of something that happened. `graft list | grep agent:` is the
// reason the command exists at all.
func TestGraftListPlainFormGoesToStdout(t *testing.T) {
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

	got := runGraftIn(t, bin, c.dir, c.env, "list")

	want := "shared  v1.0.0  (" + sha[:7] + ")\n" + `
  agent:reviewer  1 file
  schema:tdd      2 files
`
	if got.stdout != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", got.stdout, want)
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
	if got.code != 0 {
		t.Errorf("exit code = %d, want 0", got.code)
	}

	after := snapshotTree(t, c.dir)
	if len(after) != len(before) {
		t.Errorf("the tree gained or lost paths: %d before, %d after", len(before), len(after))
	}
	for path, wantBytes := range before {
		if gotBytes, ok := after[path]; !ok || gotBytes != wantBytes {
			t.Errorf("%s changed or is gone", path)
		}
	}
}
