package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The outer loop for `graft add`: the real binary, a real fixture repository, and a
// working directory holding neither graft.toml nor graft.lock. `add` is the one command
// that creates the manifest, so the state it starts from is the state no other
// acceptance test here can produce.

// newBareConsumer is a repository graft runs in that holds nothing at all — no
// graft.toml, no graft.lock. Every other command fails here; `add` is the answer to that.
func newBareConsumer(t *testing.T) *consumer {
	t.Helper()
	return &consumer{t: t, dir: t.TempDir(), env: []string{"XDG_CACHE_HOME=" + t.TempDir()}}
}

// exists reports whether a path is there at all, which is how "nothing was written" is
// asserted: read would fail the test, and absence is the expected outcome.
func (c *consumer) exists(path string) bool {
	c.t.Helper()
	_, err := os.Lstat(filepath.Join(c.dir, filepath.FromSlash(path)))
	return err == nil
}

func TestGraftAddDeclaresASourceAndSyncsIt(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	sha := repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)

	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:reviewer")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty: add's report is a summary, and summaries go to stderr", got.stdout)
	}

	if got := c.read(".claude/agents/reviewer.md"); got != "# reviewer\n" {
		t.Errorf(".claude/agents/reviewer.md = %q, want %q", got, "# reviewer\n")
	}

	manifest := c.read("graft.toml")
	for _, want := range []string{
		"[sources.shared]",
		`git     = "` + repo.dir + `"`,
		`rev     = "v1.0.0"`,
		`install = ["agent:reviewer"]`,
	} {
		if !strings.Contains(manifest, want) {
			t.Errorf("graft.toml does not contain %s:\n%s", want, manifest)
		}
	}

	lock := c.read("graft.lock")
	for _, want := range []string{
		`name     = "shared"`,
		`rev      = "v1.0.0"`,
		`resolved = "` + sha + `"`,
		`id    = "agent:reviewer"`,
		`".claude/agents/reviewer.md"`,
	} {
		if !strings.Contains(lock, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lock)
		}
	}

	if !strings.Contains(got.stderr, `graft.toml: added source "shared" at v1.0.0`) {
		t.Errorf("the report does not name the manifest edit:\n%s", got.stderr)
	}
}

func TestGraftAddAmendsOneSourceAndLeavesTheOtherPinned(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	other := newNamedSourceRepo(t, "other")
	other.seedCatalog()
	otherSHA := other.commit("v1")
	other.tag("v1.0.0")

	shared := newSourceRepo(t)
	shared.seedCatalog()
	shared.commit("v1")
	shared.tag("v1.0.0")

	// The declared source is keyed on the name `add` would derive for its own directory,
	// which is what makes the second add an addition rather than an amendment of it.
	c := newConsumer(t, strings.Replace(
		manifestFor(other, "v1.0.0", "agent:reviewer"), "[sources.shared]", "[sources.other]", 1))
	if got := runGraftIn(t, bin, c.dir, c.env, "sync"); got.code != 0 {
		t.Fatalf("seeding sync: exit %d\n%s", got.code, got.stderr)
	}

	got := runGraftIn(t, bin, c.dir, c.env, "add", shared.dir+"@v1.0.0", "schema:tdd")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}

	manifest := c.read("graft.toml")
	for _, want := range []string{"[sources.other]", "[sources.shared]"} {
		if !strings.Contains(manifest, want) {
			t.Errorf("graft.toml does not declare %s:\n%s", want, manifest)
		}
	}
	// The source that was already pinned keeps its sha: adding a source re-resolves no
	// other source's pin.
	if lock := c.read("graft.lock"); !strings.Contains(lock, otherSHA) {
		t.Errorf("graft.lock no longer records the other source's sha:\n%s", lock)
	}
}

func TestGraftAddWritesSelectorsInTheOrderGiven(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "schema:tdd", "agent:*")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if manifest := c.read("graft.toml"); !strings.Contains(manifest, `install = ["schema:tdd", "agent:*"]`) {
		t.Errorf("install is not in the order given:\n%s", manifest)
	}
	for _, path := range []string{"openspec/schemas/tdd/schema.yaml", ".claude/agents/reviewer.md"} {
		if !c.exists(path) {
			t.Errorf("%s was not installed", path)
		}
	}
}

// The manifest is written by internal/apply, after the plan is built and every check has
// passed. A selector the catalog cannot satisfy fails during expansion, which is before
// that — so the failed add leaves no manifest behind at all.
func TestGraftAddWithASelectorMatchingNothingWritesNoManifest(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:nonexistent")

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1\nstderr:\n%s", got.code, got.stderr)
	}
	if c.exists("graft.toml") {
		t.Errorf("graft.toml was written by a failed add:\n%s", c.read("graft.toml"))
	}
	if c.exists("graft.lock") {
		t.Error("graft.lock was written by a failed add")
	}
}

// A range is a rev like any other to `add`: it is written verbatim, and the tag it
// resolved to is recorded in the lock as matched.
func TestGraftAddWritesARangeVerbatim(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.2.0")
	repo.write("extras/agents/reviewer.md", "# reviewer, revised\n")
	repo.commit("v2")
	repo.tag("v1.3.0")

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@^1.2.0", "agent:reviewer")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if manifest := c.read("graft.toml"); !strings.Contains(manifest, `rev     = "^1.2.0"`) {
		t.Errorf("the range was not written verbatim:\n%s", manifest)
	}
	if lock := c.read("graft.lock"); !strings.Contains(lock, `matched  = "v1.3.0"`) {
		t.Errorf("graft.lock does not record the matched tag:\n%s", lock)
	}
}

func TestGraftAddWithNoRevPinsTheHighestStableTag(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.2.0")
	repo.tag("v2.0.0-rc.1")
	repo.tag("nightly")

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir, "agent:reviewer")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if manifest := c.read("graft.toml"); !strings.Contains(manifest, `rev     = "v1.2.0"`) {
		t.Errorf("the default pin is not the highest stable tag:\n%s", manifest)
	}
}

func TestGraftAddWithNoTagsPinsTheDefaultBranch(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir, "agent:reviewer")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if manifest := c.read("graft.toml"); !strings.Contains(manifest, `rev     = "main"`) {
		t.Errorf("the default pin is not the default branch:\n%s", manifest)
	}
}

// An add naming a rev the manifest does not hold moves the pin, and re-resolves that
// source alone. The manifest diff is the two lines the edit names and nothing else.
func TestGraftAddWithARevMovesThePin(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")
	repo.write("extras/agents/reviewer.md", "# reviewer, revised\n")
	second := repo.commit("v2")
	repo.tag("v2.0.0")

	c := newConsumer(t, manifestFor(repo, "v1.0.0", "agent:reviewer"))
	if got := runGraftIn(t, bin, c.dir, c.env, "sync"); got.code != 0 {
		t.Fatalf("seeding sync: exit %d\n%s", got.code, got.stderr)
	}
	before := c.read("graft.toml")

	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v2.0.0", "schema:tdd")
	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}

	after := c.read("graft.toml")
	if !strings.Contains(after, `rev     = "v2.0.0"`) {
		t.Errorf("the pin did not move:\n%s", after)
	}
	if changed := changedLineCount(before, after); changed != 2 {
		t.Errorf("changed lines = %d, want 2 (the pin and the install list)\nbefore:\n%s\nafter:\n%s",
			changed, before, after)
	}
	if lock := c.read("graft.lock"); !strings.Contains(lock, second) {
		t.Errorf("graft.lock does not record the sha the moved pin names:\n%s", lock)
	}
	for _, want := range []string{
		`graft.toml: moved source "shared" to v2.0.0`,
		`graft.toml: added schema:tdd to source "shared"`,
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("the report does not contain %q:\n%s", want, got.stderr)
		}
	}
}

// --no-sync with a rev given needs nothing from the network, and the proof is a source
// path no repository exists at: the run still succeeds.
func TestGraftAddNoSyncWritesTheManifestAndTouchesNothingElse(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")
	repo.removeDir()

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:reviewer", "--no-sync")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if !c.exists("graft.toml") {
		t.Fatal("graft.toml was not written")
	}
	if c.exists("graft.lock") {
		t.Error("graft.lock was written by --no-sync")
	}
	if c.exists(".claude") {
		t.Error("a destination directory was created by --no-sync")
	}
	if !strings.Contains(got.stderr, `graft.toml: added source "shared" at v1.0.0`) {
		t.Errorf("the report does not name the edit:\n%s", got.stderr)
	}
}

// The trade --no-sync makes: nothing is fetched, so nothing checks the selector against a
// catalog. The next sync is where that arrives.
func TestGraftAddNoSyncWritesAnUncheckedSelector(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)
	if got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:nonexistent", "--no-sync"); got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if manifest := c.read("graft.toml"); !strings.Contains(manifest, `install = ["agent:nonexistent"]`) {
		t.Errorf("the unchecked selector was not written:\n%s", manifest)
	}

	sync := runGraftIn(t, bin, c.dir, c.env, "sync")
	if sync.code != 1 {
		t.Fatalf("the sync exit code = %d, want 1\nstderr:\n%s", sync.code, sync.stderr)
	}
	if !strings.Contains(sync.stderr, "agent:nonexistent") {
		t.Errorf("the sync does not name the selector that matches nothing:\n%s", sync.stderr)
	}
}

func TestGraftAddAgainstAnUnreachableSourceLeavesNoManifest(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	c := newBareConsumer(t)

	got := runGraftIn(t, bin, c.dir, c.env, "add", filepath.Join(t.TempDir(), "no-such-repo"), "agent:reviewer")

	if got.code != 1 {
		t.Fatalf("exit code = %d, want 1\nstderr:\n%s", got.code, got.stderr)
	}
	if !strings.Contains(got.stderr, "cannot reach") {
		t.Errorf("the failure does not name what it could not reach:\n%s", got.stderr)
	}
	if c.exists("graft.toml") {
		t.Errorf("graft.toml was written by a failed add:\n%s", c.read("graft.toml"))
	}
}

// A listing is content the caller asked for, so it goes to standard output — and nothing
// reaches the repository, which is what makes --list safe to run against a source you have
// not decided about yet.
func TestGraftAddListPrintsToStdoutAndWritesNothing(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)
	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "--list")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	for _, want := range []string{
		"shared  v1.0.0  (",
		"  agent:reviewer  .claude/agents/reviewer.md",
		"  schema:tdd      openspec/schemas/tdd/",
	} {
		if !strings.Contains(got.stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, got.stdout)
		}
	}
	if got.stderr != "" {
		t.Errorf("stderr = %q, want empty", got.stderr)
	}
	if entries, _ := os.ReadDir(c.dir); len(entries) != 0 {
		t.Errorf("the repository is not empty after --list: %v", entries)
	}
}

// changedLineCount is how many lines differ between two texts, which is the form the
// manifest edits' claim takes: a pin move and an install amendment are two lines, and a
// re-serialized manifest is every line.
func changedLineCount(before, after string) int {
	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	n := 0
	for i := range max(len(b), len(a)) {
		var bl, al string
		if i < len(b) {
			bl = b[i]
		}
		if i < len(a) {
			al = a[i]
		}
		if bl != al {
			n++
		}
	}
	return n
}

// The report's verb has to be true. A repository that has been hand-editing a file at a
// destination is the ordinary way graft gets adopted, and `added` says the opposite of what
// happened to it.
func TestGraftAddReportsAFileItReplaced(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)
	c.writeFile(".claude/agents/reviewer.md", "# my own reviewer, written by hand\n")

	got := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:reviewer")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	for _, want := range []string{
		"adopted  agent:reviewer  1 file  replaced existing content",
		"1 file written (1 replaced existing content), 0 removed",
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("the report does not contain %q:\n%s", want, got.stderr)
		}
	}
	// Reported, not refused: adoption is how a repository starts using graft.
	if got := c.read(".claude/agents/reviewer.md"); got != "# reviewer\n" {
		t.Errorf("the file was not replaced: %q", got)
	}
}

// Adoption is a one-time event. Once the lock claims the path it is graft's own file, and
// rewriting it is what a sync does rather than something to report.
func TestGraftSyncAfterAnAdoptionReportsNothingAdopted(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	repo.commit("v1")
	repo.tag("v1.0.0")

	c := newBareConsumer(t)
	c.writeFile(".claude/agents/reviewer.md", "# my own reviewer, written by hand\n")
	if first := runGraftIn(t, bin, c.dir, c.env, "add", repo.dir+"@v1.0.0", "agent:reviewer"); first.code != 0 {
		t.Fatalf("the first add failed: %s", first.stderr)
	}

	second := runGraftIn(t, bin, c.dir, c.env, "sync")
	if second.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", second.code, second.stderr)
	}
	if strings.Contains(second.stderr, "replaced existing content") {
		t.Errorf("the second run still reports adoption:\n%s", second.stderr)
	}
}
