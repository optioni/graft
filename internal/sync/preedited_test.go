package sync_test

import (
	"strings"
	"testing"

	"github.com/optioni/graft/internal/sync"
)

// `graft add` edits graft.toml before anything is resolved, and the run has to honour the
// bytes it will write rather than the ones on disk. This is the same discipline movePin
// already follows for `update --to`: what goes to disk and what the run resolves are one
// object, not two readings of it.

func TestRunHonoursTheManifestBytesItIsGiven(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	// On disk: a manifest asking for one item. In the run: bytes asking for both.
	c.manifest(r, "v1.0.0", "agent:reviewer")
	bytes := sourceBlock("shared", r, "v1.0.0", "schema:tdd", "agent:*")

	o := c.options()
	o.Manifest = []byte(bytes)
	if _, err := sync.Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !c.exists("openspec/schemas/tdd/schema.yaml") {
		t.Error("the item only the given bytes ask for was not installed")
	}
	if got := c.read("graft.toml"); got != bytes {
		t.Errorf("graft.toml = %q, want the bytes the run was given", got)
	}
}

// The strongest form of "it never re-reads the file": a graft.toml on disk that does not
// parse at all, and a run that succeeds anyway.
func TestRunGivenManifestBytesNeverReadsTheFile(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml", "this is not TOML at all [[[\n")

	o := c.options()
	o.Manifest = []byte(sourceBlock("shared", r, "v1.0.0", "agent:reviewer"))
	if _, err := sync.Run(o); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !c.exists(".claude/agents/reviewer.md") {
		t.Error("the run did not install what the given bytes asked for")
	}
}

// Two sources of manifest bytes is a run that could write one file and resolve another.
// It is unreachable from the command surface and refused here anyway, because the cost of
// the guard is a branch and the cost of the bug is a corrupted manifest.
func TestRunRefusesBothManifestBytesAndAPinMove(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.manifest(r, "v1.0.0")

	o := c.options()
	o.Manifest = []byte(sourceBlock("shared", r, "v1.0.0", "agent:reviewer"))
	o.Update = &sync.Update{Source: "shared", To: "v2.0.0"}

	_, err := sync.Run(o)
	if err == nil {
		t.Fatal("Run succeeded with both manifest bytes and a pin move")
	}
	if want := "graft.toml: a run cannot both be given manifest bytes and move a pin"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if got := c.read("graft.toml"); !strings.Contains(got, `rev     = "v1.0.0"`) {
		t.Errorf("graft.toml was written by a refused run: %q", got)
	}
}
