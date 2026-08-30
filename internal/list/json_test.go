package list_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/list"
	"github.com/optioni/graft/internal/lock"
)

// The document is a published interface, and the two ways a JSON contract breaks a consumer
// are both invisible to a test that only decodes: an empty collection marshalled as null,
// and an ordering that follows whatever the lock happened to hold. Every assertion here is
// on the bytes.

func TestJSONRendersSpecExample(t *testing.T) {
	t.Parallel()

	got := string(list.FromLock(specLock()).JSON())
	if got != specDocument {
		t.Errorf("document:\n%s\nwant:\n%s", got, specDocument)
	}
}

func TestJSONEmptyCollectionsAreArraysNotNull(t *testing.T) {
	t.Parallel()

	t.Run("a lock with no sources", func(t *testing.T) {
		t.Parallel()

		got := string(list.FromLock(&lock.Lock{Version: lock.Version}).JSON())
		if got != emptyDocument {
			t.Errorf("document:\n%s\nwant:\n%s", got, emptyDocument)
		}
	})

	t.Run("a source with no items", func(t *testing.T) {
		t.Parallel()

		l := &lock.Lock{Version: lock.Version, Sources: []lock.Source{{
			Name: "empty", Git: "example.test/r", Rev: "main",
			Resolved: "aaaaaaa111111111111111111111111111111111",
		}}}

		want := `{
  "version": 1,
  "sources": [
    {
      "name": "empty",
      "git": "example.test/r",
      "rev": "main",
      "resolved": "aaaaaaa111111111111111111111111111111111",
      "items": []
    }
  ]
}
`
		got := string(list.FromLock(l).JSON())
		if got != want {
			t.Errorf("document:\n%s\nwant:\n%s", got, want)
		}
		if strings.Contains(got, "null") {
			t.Errorf("the document holds a null:\n%s", got)
		}
	})

	t.Run("an item with no files", func(t *testing.T) {
		t.Parallel()

		l := &lock.Lock{Version: lock.Version, Sources: []lock.Source{{
			Name: "shared", Git: "example.test/r", Rev: "main",
			Resolved: "aaaaaaa111111111111111111111111111111111",
			Items:    []lock.Item{{ID: "agent:none"}},
		}}}

		want := `{
  "version": 1,
  "sources": [
    {
      "name": "shared",
      "git": "example.test/r",
      "rev": "main",
      "resolved": "aaaaaaa111111111111111111111111111111111",
      "items": [
        {
          "id": "agent:none",
          "kind": "agent",
          "name": "none",
          "files": []
        }
      ]
    }
  ]
}
`
		got := string(list.FromLock(l).JSON())
		if got != want {
			t.Errorf("document:\n%s\nwant:\n%s", got, want)
		}
		if strings.Contains(got, "null") {
			t.Errorf("the document holds a null:\n%s", got)
		}
	})
}

// Decoding proves the document is JSON; comparing every decoded value against the lock it
// came from proves it is the lock's own content, the full forty-character sha included —
// the seven-character form exists for a person and a program comparing shas needs all of it.
func TestJSONRoundTrips(t *testing.T) {
	t.Parallel()

	l := specLock()
	var got list.Listing
	if err := json.Unmarshal(list.FromLock(l).JSON(), &got); err != nil {
		t.Fatalf("decoding the document: %v", err)
	}

	// The literal the golden pins, not list.Version: comparing a decoded value against the
	// constant that produced it cannot tell a right answer from a wrong one.
	if got.Version != 1 {
		t.Errorf("version = %d, want 1", got.Version)
	}
	if len(got.Sources) != len(l.Sources) {
		t.Fatalf("sources = %d, want %d", len(got.Sources), len(l.Sources))
	}
	for i, src := range got.Sources {
		want := l.Sources[i]
		if src.Name != want.Name || src.Git != want.Git || src.Rev != want.Rev {
			t.Errorf("source %d = %+v, want %+v", i, src, want)
		}
		if src.Resolved != want.Resolved {
			t.Errorf("resolved = %q, want the full sha %q", src.Resolved, want.Resolved)
		}
		if len(src.Items) != len(want.Items) {
			t.Fatalf("items = %d, want %d", len(src.Items), len(want.Items))
		}
		for j, it := range src.Items {
			wantItem := want.Items[j]
			if it.ID != wantItem.ID {
				t.Errorf("item %d id = %q, want %q", j, it.ID, wantItem.ID)
			}
			if strings.Join(it.Files, ",") != strings.Join(wantItem.Files, ",") {
				t.Errorf("item %q files = %v, want %v", it.ID, it.Files, wantItem.Files)
			}
		}
	}
}

// Go's encoder escapes <, > and & by default, which would return a git URL with three
// characters replaced and stop the document round-tripping what the lock holds.
func TestJSONDoesNotEscapeHTML(t *testing.T) {
	t.Parallel()

	const url = "https://example.test/r?a=1&b=2"
	l := &lock.Lock{Version: lock.Version, Sources: []lock.Source{{
		Name: "shared", Git: url, Rev: "main",
		Resolved: "aaaaaaa111111111111111111111111111111111",
	}}}

	got := string(list.FromLock(l).JSON())
	if !strings.Contains(got, url) {
		t.Errorf("the document does not hold the URL literally:\n%s", got)
	}
	if strings.Contains(got, `\u0026`) {
		t.Errorf("the document escaped the ampersand, which is what Go's encoder does by default:\n%s", got)
	}
}

// The two halves are carried beside the id because kind:name is graft's grammar and not the
// consumer's: a caller filtering by kind should not have to re-implement graft's parsing.
func TestJSONCarriesTheHalvesOfTheID(t *testing.T) {
	t.Parallel()

	got := list.FromLock(specLock()).Sources[0].Items[1]
	if got.ID != "schema:tdd" || got.Kind != "schema" || got.Name != "tdd" {
		t.Errorf("item = %+v, want id schema:tdd, kind schema, name tdd", got)
	}
}

func TestEmpty(t *testing.T) {
	t.Parallel()

	if !list.FromLock(&lock.Lock{Version: lock.Version}).Empty() {
		t.Error("a lock with no sources is not reported as empty")
	}
	if list.FromLock(specLock()).Empty() {
		t.Error("a lock with a source is reported as empty")
	}
}

// The builder imposes the order rather than inheriting it, so two locks describing one
// installation list identically however they were written.
func TestFromLockOrdersEveryCollection(t *testing.T) {
	t.Parallel()

	scrambled := &lock.Lock{Version: lock.Version, Sources: []lock.Source{
		{
			Name: "zeta", Git: "example.test/z", Rev: "main",
			Resolved: "bbbbbbb222222222222222222222222222222222",
			Items:    []lock.Item{{ID: "agent:z", Files: []string{"z.md"}}},
		},
		{
			Name: "alpha", Git: "example.test/a", Rev: "main",
			Resolved: "aaaaaaa111111111111111111111111111111111",
			Items: []lock.Item{
				{ID: "schema:s", Files: []string{"s/b.md", "s/a.md"}},
				{ID: "agent:a", Files: []string{"a.md"}},
			},
		},
	}}

	got := list.FromLock(scrambled)
	if got.Sources[0].Name != "alpha" || got.Sources[1].Name != "zeta" {
		t.Errorf("sources are not in name order: %q, %q", got.Sources[0].Name, got.Sources[1].Name)
	}
	if got.Sources[0].Items[0].ID != "agent:a" || got.Sources[0].Items[1].ID != "schema:s" {
		t.Errorf("items are not in id order: %+v", got.Sources[0].Items)
	}
	if files := got.Sources[0].Items[1].Files; files[0] != "s/a.md" || files[1] != "s/b.md" {
		t.Errorf("files are not in path order: %v", files)
	}

	// Ordering the listing must not reorder the lock its caller still holds.
	if scrambled.Sources[0].Name != "zeta" {
		t.Error("FromLock reordered the lock it was given")
	}
}
