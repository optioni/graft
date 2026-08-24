package apply_test

import (
	"os"
	"testing"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/manifest"
	"github.com/optioni/graft/internal/plan"
)

// graft.toml is the one file in the repository graft cannot regenerate, and until now
// internal/apply refused to touch it at all. It still refuses every path that arrives in a
// *plan* — the two are told apart by where the bytes came from, never by the path string.

const movedManifest = `[sources.shared]
git     = "example.com/o/r"
rev     = "v1.1.0"
install = ["schema:tdd"]
`

func TestRunWritesManifestBytesJustBeforeTheLock(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(manifest.Filename, "[sources.shared]\nrev = \"v1.0.0\"\n")
	src := newTree(t)
	src.file("extras/a.md", "a\n")
	src.file("extras/b.md", "b\n")

	p := &plan.Plan{
		Writes: []plan.Write{write("extras/a.md", "docs/a.md"), write("extras/b.md", "docs/b.md")},
		Lock:   lockOf("docs/a.md", "docs/b.md"),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p, apply.WithManifest([]byte(movedManifest)))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries("docs/", "docs/a.md", "docs/b.md", lock.Filename, manifest.Filename)
	if got := repo.read(manifest.Filename); got != movedManifest {
		t.Errorf("graft.toml = %q, want %q", got, movedManifest)
	}
	// A manifest this package writes that the next run refuses to read is the worst
	// failure available here, so the assertion goes through the parser rather than
	// comparing strings alone.
	if _, err := manifest.Parse([]byte(repo.read(manifest.Filename)), manifest.Filename); err != nil {
		t.Errorf("the manifest this run wrote does not parse: %v", err)
	}
	if got := repo.read(lock.Filename); got == "" {
		t.Error("graft.lock was not written")
	}
}

func TestRunWithoutManifestBytesLeavesTheManifestAlone(t *testing.T) {
	t.Parallel()

	const original = "# hand written\n[sources.shared]\nrev = \"v1.0.0\"\n"

	repo := newTree(t)
	repo.file(manifest.Filename, original)
	src := newTree(t)
	src.file("extras/a.md", "a\n")

	p := &plan.Plan{Writes: []plan.Write{write("extras/a.md", "docs/a.md")}, Lock: lockOf("docs/a.md")}
	if err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := repo.read(manifest.Filename); got != original {
		t.Errorf("graft.toml = %q, want %q — an apply given no manifest bytes never touches it", got, original)
	}
}

// The empty plan is the boundary case: with no manifest bytes it writes one file, with them
// it writes two, and neither writes a third.
func TestRunEmptyPlanWithManifestBytes(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file("README.md", "mine\n")

	err := apply.Run(repo.dir, nil, &plan.Plan{Lock: emptyLock()}, apply.WithManifest([]byte(movedManifest)))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	repo.assertEntries("README.md", lock.Filename, manifest.Filename)
	if got := repo.read(manifest.Filename); got != movedManifest {
		t.Errorf("graft.toml = %q, want the given bytes", got)
	}
	if got := repo.read("README.md"); got != "mine\n" {
		t.Errorf("README.md = %q, want %q", got, "mine\n")
	}
}

// The provenance rule, tested at its sharpest: the run rewriting graft.toml still refuses a
// plan that names graft.toml as a destination.
func TestRunStillRefusesAPlannedWriteOfTheManifest(t *testing.T) {
	t.Parallel()

	const original = "[sources.shared]\nrev = \"v1.0.0\"\n"

	repo := newTree(t)
	repo.file(manifest.Filename, original)
	src := newTree(t)
	src.file("extras/a.md", "a\n")
	src.file("extras/evil.toml", "[sources.attacker]\n")

	p := &plan.Plan{
		Writes: []plan.Write{
			write("extras/a.md", "docs/a.md"),
			write("extras/evil.toml", manifest.Filename),
		},
		Lock: lockOf("docs/a.md", manifest.Filename),
	}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p, apply.WithManifest([]byte(movedManifest)))
	assertError(t, err, `cannot write "graft.toml": graft never writes over "graft.toml"`)

	if got := repo.read(manifest.Filename); got != original {
		t.Errorf("graft.toml = %q, want the original: the refusal is in the pre-flight pass", got)
	}
	repo.assertEntries(manifest.Filename)
}

func TestRunRefusesAManifestThatIsNotARegularFile(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.mkdir(manifest.Filename)
	src := newTree(t)
	src.file("extras/a.md", "a\n")

	p := &plan.Plan{Writes: []plan.Write{write("extras/a.md", "docs/a.md")}, Lock: lockOf("docs/a.md")}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p, apply.WithManifest([]byte(movedManifest)))
	assertError(t, err, `cannot write "graft.toml": it exists and is not a regular file`)

	repo.assertEntries(manifest.Filename + "/")
}

// The residual failure the pre-flight pass cannot remove. graft.toml stays where it was, so
// the manifest and the lock still agree — the state a re-run recovers from.
func TestRunFailedApplyLeavesTheManifestUnmoved(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root: permission bits do not deny anything")
	}

	const original = "[sources.shared]\nrev = \"v1.0.0\"\n"

	repo := newTree(t)
	repo.file(manifest.Filename, original)
	repo.file(lock.Filename, "the previous lock\n")
	repo.file("locked/keep", "x\n")
	if err := os.Chmod(repo.path("locked"), 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(repo.path("locked"), 0o755) })

	src := newTree(t)
	src.file("extras/b.md", "b\n")

	p := &plan.Plan{Writes: []plan.Write{write("extras/b.md", "locked/b.md")}, Lock: lockOf("locked/b.md")}
	err := apply.Run(repo.dir, map[string]string{"shared": src.dir}, p, apply.WithManifest([]byte(movedManifest)))
	assertErrorPrefix(t, err, `cannot write "locked/b.md": `)

	if got := repo.read(manifest.Filename); got != original {
		t.Errorf("graft.toml = %q, want the original", got)
	}
	if got := repo.read(lock.Filename); got != "the previous lock\n" {
		t.Errorf("graft.lock = %q, want the previous one", got)
	}
}

// The manifest is written through a temporary file and a rename, so it is never absent and
// never half-written. Neither outcome may leave the temporary behind.
func TestRunLeavesNoTemporaryManifestBehind(t *testing.T) {
	t.Parallel()

	repo := newTree(t)
	repo.file(manifest.Filename, "[sources.shared]\nrev = \"v1.0.0\"\n")

	err := apply.Run(repo.dir, nil, &plan.Plan{Lock: emptyLock()}, apply.WithManifest([]byte(movedManifest)))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	repo.assertEntries(lock.Filename, manifest.Filename)

	// And on a failed run: the plan's write fails, so the manifest write is never reached
	// and the tree is exactly what it was.
	failing := newTree(t)
	failing.file(manifest.Filename, "[sources.shared]\nrev = \"v1.0.0\"\n")
	failing.mkdir("docs/a.md")
	src := newTree(t)
	src.file("extras/a.md", "a\n")

	p := &plan.Plan{Writes: []plan.Write{write("extras/a.md", "docs/a.md")}, Lock: lockOf("docs/a.md")}
	err = apply.Run(failing.dir, map[string]string{"shared": src.dir}, p, apply.WithManifest([]byte(movedManifest)))
	assertError(t, err, `cannot write "docs/a.md": it exists and is not a regular file`)
	failing.assertEntries("docs/", "docs/a.md/", manifest.Filename)
}

// A leftover temporary from a process that died mid-write, which is not a regular file. It
// is refused rather than removed blindly, and the message names the path the user has to go
// and look at rather than the destination they asked for.
func TestRunRefusesALeftoverTemporaryThatIsNotARegularFile(t *testing.T) {
	t.Parallel()

	const original = "[sources.shared]\nrev = \"v1.0.0\"\n"

	repo := newTree(t)
	repo.file(manifest.Filename, original)
	repo.mkdir(".graft.toml.tmp/occupied")

	err := apply.Run(repo.dir, nil, &plan.Plan{Lock: emptyLock()}, apply.WithManifest([]byte(movedManifest)))
	assertError(t, err, `cannot write ".graft.toml.tmp": it exists and is not a regular file`)

	if got := repo.read(manifest.Filename); got != original {
		t.Errorf("graft.toml = %q, want the original", got)
	}
	if !repo.exists(".graft.toml.tmp/occupied") {
		t.Error("the leftover was removed: graft only ever removes a path it confirmed regular")
	}
}
