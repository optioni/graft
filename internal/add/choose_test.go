package add_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/add"
	"github.com/optioni/graft/internal/picker"
)

// The picker chooses selectors and has no other powers: what it returns enters the
// sequence exactly where a command line's selectors would. These tests hand the sequence a
// scripted chooser, which is the whole interactive path minus the terminal.

// chooser returns a Choose that answers with fixed selectors and records what it was
// offered, so a test can assert both what the picker would have shown and that it was
// shown at all.
func chooser(answer []string, seen *[]picker.Item, title *string) func(string, []picker.Item) ([]string, error) {
	return func(t string, items []picker.Item) ([]string, error) {
		if seen != nil {
			*seen = items
		}
		if title != nil {
			*title = t
		}
		return answer, nil
	}
}

func TestRunTakesItsSelectorsFromTheChooser(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	var seen []picker.Item
	var title string
	req := c.request(r, "v1.0.0")
	req.Choose = chooser([]string{"agent:reviewer"}, &seen, &title)

	if _, err := add.Run(req); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got := c.read("graft.toml"); !strings.Contains(got, `install = ["agent:reviewer"]`) {
		t.Errorf("the chosen selector was not written:\n%s", got)
	}
	if !c.exists(".claude/agents/reviewer.md") {
		t.Error("the chosen item was not installed")
	}

	// What the picker was offered is the catalog, with the destinations --list prints.
	ids := make([]string, 0, len(seen))
	for _, it := range seen {
		ids = append(ids, it.ID)
	}
	if !slices.Equal(ids, []string{"agent:reviewer", "schema:tdd"}) {
		t.Errorf("offered ids = %q, want the whole catalog in id order", ids)
	}
	if len(seen) > 0 && !slices.Equal(seen[0].Destinations, []string{".claude/agents/reviewer.md"}) {
		t.Errorf("offered destinations = %q, want the ones --list shows", seen[0].Destinations)
	}
	if !strings.HasPrefix(title, "shared  v1.0.0  (") {
		t.Errorf("title = %q, want the source header --list prints", title)
	}
}

// The claim that keeps the interactive layer thin: what the picker chooses and what a user
// types produce the same file, byte for byte.
func TestAChosenSelectorWritesTheSameManifestATypedOneDoes(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	typed := newConsumer(t)
	if _, err := add.Run(typed.request(r, "v1.0.0", "agent:reviewer")); err != nil {
		t.Fatalf("typed Run: %v", err)
	}

	chosen := newConsumer(t)
	req := chosen.request(r, "v1.0.0")
	req.Choose = chooser([]string{"agent:reviewer"}, nil, nil)
	if _, err := add.Run(req); err != nil {
		t.Fatalf("chosen Run: %v", err)
	}

	if a, b := typed.read("graft.toml"), chosen.read("graft.toml"); a != b {
		t.Errorf("the two manifests differ:\n%s\n---\n%s", a, b)
	}
}

func TestACancelledChooserWritesNothing(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	req := c.request(r, "v1.0.0")
	req.Choose = func(string, []picker.Item) ([]string, error) { return nil, nil }

	_, err := add.Run(req)
	if !errors.Is(err, add.ErrCancelled) {
		t.Fatalf("error = %v, want ErrCancelled", err)
	}
	if want := "add cancelled"; err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
	}
	if c.exists("graft.toml") {
		t.Errorf("graft.toml was written by a cancelled add:\n%s", c.read("graft.toml"))
	}
	if c.exists("graft.lock") {
		t.Error("graft.lock was written by a cancelled add")
	}
	if c.exists(".claude") {
		t.Error("a destination directory was created by a cancelled add")
	}
}

// The list a picker shows is the source's catalog, so a source that has none fails with
// that failure rather than with an empty list the user is asked to choose from.
func TestAnUngraftableSourceFailsBeforeTheChooser(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("readme.md", "no catalog here\n")
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	called := false
	req := c.request(r, "v1.0.0")
	req.Choose = func(string, []picker.Item) ([]string, error) {
		called = true
		return []string{"agent:reviewer"}, nil
	}

	_, err := add.Run(req)
	if err == nil {
		t.Fatal("Run succeeded against a source with no catalog")
	}
	if !strings.Contains(err.Error(), "not graftable") {
		t.Errorf("error = %q, want the not-graftable message", err)
	}
	if called {
		t.Error("the chooser was called for a source with no catalog")
	}
	if c.exists("graft.toml") {
		t.Error("graft.toml was written")
	}
}

// A chooser's own failure is the run's failure, unaltered: the terminal broke, and that is
// not something to report as a cancellation.
func TestAChooserFailureStopsTheRun(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	req := c.request(r, "v1.0.0")
	req.Choose = func(string, []picker.Item) ([]string, error) {
		return nil, errors.New("the terminal went away")
	}

	_, err := add.Run(req)
	if err == nil || err.Error() != "the terminal went away" {
		t.Fatalf("error = %v, want the chooser's own", err)
	}
	if c.exists("graft.toml") {
		t.Error("graft.toml was written")
	}
}
