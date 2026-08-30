package list_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/list"
)

// The integration tier: list.Run against a real directory holding a hand-written
// graft.lock. No git repository is involved, because list has no source — a lock is the
// whole of its input, and a hand-written one is the input it actually has.

// canonicalLock is SPEC.md's example written the way graft writes it: sources by name,
// items by id, files by path.
const canonicalLock = `version = 1

[[source]]
name     = "alpha"
git      = "example.test/a"
rev      = "v1.0.0"
resolved = "aaaaaaa111111111111111111111111111111111"

  [[source.item]]
  id    = "agent:one"
  files = ["a/one.md"]

  [[source.item]]
  id    = "schema:two"
  files = [
    "b/x.md",
    "b/y.md",
  ]

[[source]]
name     = "zeta"
git      = "example.test/z"
rev      = "main"
resolved = "bbbbbbb222222222222222222222222222222222"

  [[source.item]]
  id    = "agent:z"
  files = ["z.md"]
`

// scrambledLock is the same content with sources, items, and files each in the other order.
// A lock is a file a human can edit and a merge can reorder, and two locks describing one
// installation must list identically.
const scrambledLock = `version = 1

[[source]]
name     = "zeta"
git      = "example.test/z"
rev      = "main"
resolved = "bbbbbbb222222222222222222222222222222222"

  [[source.item]]
  id    = "agent:z"
  files = ["z.md"]

[[source]]
name     = "alpha"
git      = "example.test/a"
rev      = "v1.0.0"
resolved = "aaaaaaa111111111111111111111111111111111"

  [[source.item]]
  id    = "schema:two"
  files = [
    "b/y.md",
    "b/x.md",
  ]

  [[source.item]]
  id    = "agent:one"
  files = ["a/one.md"]
`

// repoWith returns a directory holding the named files, and is the whole of the fixture:
// list reads one file and creates nothing, so there is nothing else to set up.
func repoWith(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	return dir
}

func runIn(t *testing.T, root string) *list.Listing {
	t.Helper()

	got, err := list.Run(list.Options{Root: root})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

// A repository that has never synced and one whose lock declares no source are the same
// answer: nothing is installed. Neither is an error.
func TestRunTreatsNoSourcesAsNothingInstalled(t *testing.T) {
	t.Parallel()

	absent := runIn(t, repoWith(t, nil))
	declared := runIn(t, repoWith(t, map[string]string{"graft.lock": "version = 1\n"}))

	for name, got := range map[string]*list.Listing{"no lock": absent, "a lock with no source": declared} {
		if !got.Empty() {
			t.Errorf("%s: the listing is not empty", name)
		}
		if lines := got.Lines(); len(lines) != 0 {
			t.Errorf("%s: lines = %q, want none", name, lines)
		}
		if doc := string(got.JSON()); doc != emptyDocument {
			t.Errorf("%s: document:\n%s\nwant:\n%s", name, doc, emptyDocument)
		}
	}
}

// Ordering is imposed by the listing rather than inherited from the file, in both forms and
// across repeated runs. Byte equality, not semantic equality, or every consumer diffing two
// documents sees churn that means nothing.
func TestRunIsDeterministicWhateverOrderTheLockIsIn(t *testing.T) {
	t.Parallel()

	canonical := repoWith(t, map[string]string{"graft.lock": canonicalLock})
	scrambled := repoWith(t, map[string]string{"graft.lock": scrambledLock})

	first := runIn(t, canonical)
	second := runIn(t, scrambled)

	if a, b := strings.Join(first.Lines(), "\n"), strings.Join(second.Lines(), "\n"); a != b {
		t.Errorf("the two locks list differently:\n%s\n---\n%s", a, b)
	}
	if a, b := string(first.JSON()), string(second.JSON()); a != b {
		t.Errorf("the two locks produce different documents:\n%s\n---\n%s", a, b)
	}
	if a, b := string(first.JSON()), string(runIn(t, canonical).JSON()); a != b {
		t.Errorf("two runs of one lock produce different documents:\n%s\n---\n%s", a, b)
	}
}

// list reads graft.lock and not graft.toml. A manifest whose rev has moved ahead is the
// disagreement sync raises and update repairs, and reporting it from an informational read
// would be a fourth place keeping one rule.
func TestRunIgnoresTheManifest(t *testing.T) {
	t.Parallel()

	root := repoWith(t, map[string]string{
		"graft.lock": `version = 1

[[source]]
name     = "shared"
git      = "example.test/r"
rev      = "v1.0.0"
resolved = "aaaaaaa111111111111111111111111111111111"
`,
		"graft.toml": `[sources.shared]
git = "example.test/r"
rev = "v2.0.0"
`,
	})

	got := strings.Join(runIn(t, root).Lines(), "\n")
	if !strings.Contains(got, "v1.0.0") {
		t.Errorf("the listing does not name the lock's rev:\n%s", got)
	}
	if strings.Contains(got, "v2.0.0") {
		t.Errorf("the listing names the manifest's rev:\n%s", got)
	}
}

// Every failure list can have is one internal/lock already words. The message arrives
// unaltered, with no second layer of context wrapped around it.
func TestRunRefusesALockThatIsADirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "graft.lock"), 0o755); err != nil {
		t.Fatalf("creating the directory: %v", err)
	}

	got, err := list.Run(list.Options{Root: root})
	if err == nil {
		t.Fatal("a lock that is a directory was accepted")
	}
	if got != nil {
		t.Errorf("a listing was returned beside the error: %+v", got)
	}
	if !strings.HasPrefix(err.Error(), "graft.lock: ") {
		t.Errorf("error = %q, want the lock's own graft.lock: prefix", err)
	}
}

// A lock from a newer graft is refused with the lock's own sentence, not a rewording of it.
func TestRunReturnsTheLocksOwnMessage(t *testing.T) {
	t.Parallel()

	root := repoWith(t, map[string]string{"graft.lock": "version = 2\n"})

	_, err := list.Run(list.Options{Root: root})
	if err == nil {
		t.Fatal("a lock from a newer graft was accepted")
	}
	want := "graft.lock: version 2 is not supported by this graft; upgrade graft"
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
}
