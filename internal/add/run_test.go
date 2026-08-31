package add_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/add"
)

func TestRunAmendsADeclaredSourceRatherThanDuplicatingIt(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	before := manifestFor(r, "v1.0.0", "agent:reviewer")
	c.file("graft.toml", before)

	got, err := add.Run(c.request(r, "", "schema:tdd"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	manifest := c.read("graft.toml")
	if strings.Count(manifest, "[sources.shared]") != 1 {
		t.Errorf("the source was declared twice:\n%s", manifest)
	}
	if !strings.Contains(manifest, `install = ["agent:reviewer", "schema:tdd"]`) {
		t.Errorf("the selector was not added:\n%s", manifest)
	}
	// An add naming no rev moves no pin, so that line is byte-identical.
	if !strings.Contains(manifest, `rev     = "v1.0.0"`) {
		t.Errorf("the pin line moved:\n%s", manifest)
	}
	if want := []string{`graft.toml: added schema:tdd to source "shared"`}; !slices.Equal(got.Edits, want) {
		t.Errorf("edits = %q, want %q", got.Edits, want)
	}
	if !c.exists("openspec/schemas/tdd/schema.yaml") {
		t.Error("the added item was not installed")
	}
}

func TestRunLeavesAManifestThatAlreadySaysItAlone(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	before := manifestFor(r, "v1.0.0", "agent:reviewer")
	c.file("graft.toml", before)

	got, err := add.Run(c.request(r, "", "agent:reviewer"))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if after := c.read("graft.toml"); after != before {
		t.Errorf("graft.toml changed:\n%s", after)
	}
	if want := []string{"graft.toml: unchanged"}; !slices.Equal(got.Edits, want) {
		t.Errorf("edits = %q, want %q", got.Edits, want)
	}
	// It still syncs: the manifest said what was wanted, and the tree did not yet match.
	if !c.exists(".claude/agents/reviewer.md") {
		t.Error("an unchanged manifest skipped the sync")
	}
}

func TestRunWritesASelectorGivenTwiceOnce(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	if _, err := add.Run(c.request(r, "v1.0.0", "agent:reviewer", "agent:reviewer")); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := c.read("graft.toml"); !strings.Contains(got, `install = ["agent:reviewer"]`) {
		t.Errorf("the duplicate was written:\n%s", got)
	}
}

func TestRunRefusesADifferentRepositoryUnderATakenName(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	before := "[sources.shared]\ngit     = \"optioni/shared\"\nrev     = \"v1.0.0\"\ninstall = [\"agent:reviewer\"]\n"
	c.file("graft.toml", before)

	_, err := add.Run(c.request(r, "v1.0.0", "schema:tdd"))
	if err == nil {
		t.Fatal("Run succeeded against a name declared for another repository")
	}
	if want := `graft.toml: source "shared": already declared with git "optioni/shared"`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if after := c.read("graft.toml"); after != before {
		t.Errorf("graft.toml was written by a refused run:\n%s", after)
	}
}

func TestRunRefusesAnUnamendableManifestInTheAmendersWords(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	// An inline table parses perfectly and no [sources.shared] header covers it, so the
	// amendment has no array to find. This is the shape the refusal exists for: a manifest
	// graft can read and cannot edit exactly.
	before := "sources = { shared = { git = " + quote(r.dir) + ", rev = \"v1.0.0\", install = [\"agent:reviewer\"] } }\n"
	c.file("graft.toml", before)

	_, err := add.Run(c.request(r, "", "schema:tdd"))
	if err == nil {
		t.Fatal("Run succeeded against an install it cannot amend")
	}
	want := `graft.toml: source "shared": cannot amend install: install is not a plain array of strings under [sources.shared]`
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if after := c.read("graft.toml"); after != before {
		t.Errorf("graft.toml was written by a refused run:\n%s", after)
	}
}

// A graft.toml that does not parse is refused rather than treated as absent. Treating it
// as absent would overwrite a broken manifest with a fresh one holding a single source,
// destroying the consumer's own file.
func TestRunRefusesAManifestThatDoesNotParse(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	before := "[sources.shared]\ngit = \"a/b\"\nrev = \"v1\"\ninstall = [\"agent:reviewer\"]\nnonsense = true\n"
	c.file("graft.toml", before)

	_, err := add.Run(c.request(r, "v1.0.0", "schema:tdd"))
	if err == nil {
		t.Fatal("Run succeeded against a manifest that does not parse")
	}
	if want := `graft.toml: source "shared": unknown key "nonsense"`; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if after := c.read("graft.toml"); after != before {
		t.Errorf("graft.toml was written by a refused run:\n%s", after)
	}
}

// The rule `sync` is built on: only an explicit act moves a pin. Adding a selector to a
// source pinned at a branch installs from the sha the lock records, whatever the branch
// points at now.
func TestRunDoesNotMoveABranchPinItWasNotAskedToMove(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	first := r.commit("v1")

	c := newConsumer(t)
	c.file("graft.toml", manifestFor(r, "main", "agent:reviewer"))
	if _, err := add.Run(c.request(r, "", "agent:reviewer")); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if got := c.read("graft.lock"); !strings.Contains(got, first) {
		t.Fatalf("the first run did not record the first sha:\n%s", got)
	}

	// The branch advances, and the item the second add installs gains a file.
	r.write("extras/schemas/tdd/templates/tasks.md", "# tasks\n")
	second := r.commit("v2")

	if _, err := add.Run(c.request(r, "", "schema:tdd")); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	lock := c.read("graft.lock")
	if !strings.Contains(lock, first) {
		t.Errorf("the lock no longer records the pinned sha:\n%s", lock)
	}
	if strings.Contains(lock, second) {
		t.Errorf("adding a selector moved the pin to %s:\n%s", second, lock)
	}
	if c.exists("openspec/schemas/tdd/templates/tasks.md") {
		t.Error("a file only the advanced branch holds was installed")
	}
}

// quote renders a value into a TOML string for a hand-written fixture manifest.
func quote(s string) string { return `"` + s + `"` }
