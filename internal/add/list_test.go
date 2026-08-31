package add_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/add"
)

// `graft add <source> --list` answers "what does this source offer, and where would it
// put it" before anything is written. Its whole value is that the answer arrives without
// a commitment, so "nothing was written" is asserted in every case here.

func TestListNamesEveryItemAndItsDestination(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	got, err := add.List(c.request(r, "v1.0.0"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []string{
		"shared  v1.0.0  (" + shortSHA(r.head()) + ")",
		"",
		"  agent:reviewer  .claude/agents/reviewer.md",
		"  schema:tdd      openspec/schemas/tdd/",
	}
	if !slices.Equal(got, want) {
		t.Errorf("listing:\n got %q\nwant %q", got, want)
	}
	if entries, _ := os.ReadDir(c.dir); len(entries) != 0 {
		t.Errorf("the repository is not empty after a listing: %v", entries)
	}
}

// Two runs against one catalog print the same bytes: a listing that churned would be a
// listing nobody could diff.
func TestListIsByteIdenticalAcrossTwoRuns(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	req := c.request(r, "v1.0.0")

	first, err := add.List(req)
	if err != nil {
		t.Fatalf("first List: %v", err)
	}
	second, err := add.List(req)
	if err != nil {
		t.Fatalf("second List: %v", err)
	}
	if !slices.Equal(first, second) {
		t.Errorf("two listings differ:\n%q\n%q", first, second)
	}
}

// The destination is what a consumer actually agrees to, so an override the consumer
// already declared is what the listing shows.
func TestListShowsTheConsumersOwnDestination(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	c.file("graft.toml", manifestFor(r, "v1.0.0", "agent:reviewer")+
		"\n[sources.shared.kinds]\nagent = \".codex/agents/\"\n")

	got, err := add.List(c.request(r, "v1.0.0"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !slices.ContainsFunc(got, func(l string) bool { return strings.Contains(l, ".codex/agents/reviewer.md") }) {
		t.Errorf("the override is not reflected:\n%q", got)
	}
	for _, l := range got {
		if strings.Contains(l, ".claude/agents/") {
			t.Errorf("the catalog's own destination survived the override: %q", l)
		}
	}
}

func TestListOfASourceOfferingNothingSaysSo(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("catalog.yaml", "version: 1\n\nkinds:\n  agent:\n    to: \".claude/agents/\"\n\nprovides: []\n")
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	got, err := add.List(c.request(r, "v1.0.0"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[1] != "  (no items)" {
		t.Errorf("listing = %q, want a header and a no-items line", got)
	}
}

func TestListRefusesASourceThatIsNotGraftable(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.write("readme.md", "no catalog here\n")
	r.commit("v1")
	r.tag("v1.0.0")

	c := newConsumer(t)
	got, err := add.List(c.request(r, "v1.0.0"))
	if err == nil {
		t.Fatalf("List = %q, want a refusal", got)
	}
	if !strings.Contains(err.Error(), "not graftable") {
		t.Errorf("error = %q, want the not-graftable message", err)
	}
	if got != nil {
		t.Errorf("lines returned beside an error: %q", got)
	}
}

// A listing needs a rev like anything else, and with none given it resolves the same
// default pin an add would write.
func TestListWithNoRevUsesTheDefaultPin(t *testing.T) {
	t.Parallel()

	r := newRepo(t)
	r.seed()
	r.commit("v1")
	r.tag("v1.2.0")

	c := newConsumer(t)
	got, err := add.List(c.request(r, ""))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if !strings.HasPrefix(got[0], "shared  v1.2.0  (") {
		t.Errorf("header = %q, want the default pin", got[0])
	}
}
