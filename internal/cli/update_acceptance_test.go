package cli_test

import (
	"fmt"
	"strings"
	"testing"
)

// The outer loop for `graft update`: the real binary, a real fixture repository, a real
// working directory, and a real fetch cache, with nothing replaced. It shares
// sync_acceptance_test.go's harness rather than building a second one — the collaborators
// are the same and the risk is the same, which is wiring: a --to value that never reaches
// the manifest editor, a report on the wrong stream, an update run against the wrong root,
// a cache root read from a global.

// TestGraftUpdateMovesThePin is this change's headline scenario, end to end: the pin moves
// in graft.toml, the pin moves in graft.lock, the new rev's files land, and the report says
// what happened on the error stream.
func TestGraftUpdateMovesThePin(t *testing.T) {
	t.Parallel()

	bin := buildGraft(t)
	repo := newSourceRepo(t)
	repo.seedCatalog()
	first := repo.commit("v1")
	repo.tag("v1.0.0")

	// The manifest a human would have written: a comment, and the aligned keys SPEC.md's
	// own example uses. Every byte of it but one line has to survive the update.
	manifest := "# pinned deliberately\n" + manifestFor(repo, "v1.0.0", "schema:tdd", "agent:*")
	c := newConsumer(t, manifest)

	if got := runGraftIn(t, bin, c.dir, c.env, "sync"); got.code != 0 {
		t.Fatalf("first sync: exit %d\n%s", got.code, got.stderr)
	}

	repo.write("extras/schemas/tdd/templates/spec.md", "# spec\n")
	second := repo.commit("v2")
	repo.tag("v1.1.0")

	got := runGraftIn(t, bin, c.dir, c.env, "update", "--to", "v1.1.0", "shared")

	if got.code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr:\n%s", got.code, got.stderr)
	}
	if got.stdout != "" {
		t.Errorf("stdout = %q, want empty: an update's report is a summary, and summaries go to stderr", got.stdout)
	}

	// graft.toml moved, and moved by exactly one line.
	after := c.read("graft.toml")
	if diff := changedLines(manifest, after); len(diff) != 1 || diff[0] != `rev     = "v1.1.0"` {
		t.Errorf("graft.toml changed in %d lines %q, want exactly one reading %q",
			len(diff), diff, `rev     = "v1.1.0"`)
	}

	// graft.lock followed it, and the new rev's file is on disk.
	lock := c.read("graft.lock")
	for _, want := range []string{
		`rev      = "v1.1.0"`,
		fmt.Sprintf(`resolved = %q`, second),
		`"openspec/schemas/tdd/templates/spec.md"`,
	} {
		if !strings.Contains(lock, want) {
			t.Errorf("graft.lock does not contain %s:\n%s", want, lock)
		}
	}
	if strings.Contains(lock, first) {
		t.Errorf("graft.lock still records the old sha %s:\n%s", first, lock)
	}
	if got := c.read("openspec/schemas/tdd/templates/spec.md"); got != "# spec\n" {
		t.Errorf("the new rev's file = %q, want %q", got, "# spec\n")
	}

	// The report names what moved, on the error stream.
	for _, want := range []string{
		fmt.Sprintf("shared  v1.0.0 -> v1.1.0  (%s -> %s)", first[:7], second[:7]),
		"updated  schema:tdd",
		"review with `git diff`",
	} {
		if !strings.Contains(got.stderr, want) {
			t.Errorf("the report does not contain %q:\n%s", want, got.stderr)
		}
	}
}

// changedLines returns the lines of after that differ from before, positionally. It is
// deliberately naive: the assertion it serves is that exactly one line moved, and a real
// diff would let a two-line change look like a one-line one plus an insertion.
func changedLines(before, after string) []string {
	old := strings.Split(before, "\n")
	current := strings.Split(after, "\n")

	var out []string
	for i, line := range current {
		if i >= len(old) || old[i] != line {
			out = append(out, line)
		}
	}
	for i := len(current); i < len(old); i++ {
		out = append(out, "(removed) "+old[i])
	}
	return out
}
