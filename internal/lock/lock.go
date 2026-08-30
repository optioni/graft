// Package lock parses, validates, and serializes graft.lock — the record of what
// graft actually installed. It reads and returns bytes; internal/apply is the only
// package permitted to write to the working tree.
package lock

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/optioni/graft/internal/itemid"
	"github.com/optioni/graft/internal/rev"
)

// Version is the graft.lock format version this binary understands. A lock carrying
// a higher version is refused outright rather than half-read.
const Version = 1

// Filename is the one spelling of graft's own record. It lives here rather than at each
// call site because two packages now name the file — this one reads it, and internal/apply
// writes it — and a string literal in two places is a rename that goes half done.
const Filename = "graft.lock"

// Lock is a parsed graft.lock.
type Lock struct {
	Version int
	Sources []Source
}

// Source is one [[source]] block. Rev records what graft.toml asked for, Resolved the
// sha it became. Matched is the tag a range resolved to, exactly as the remote spelled
// it — empty for a ref, which names itself and needs no further record.
type Source struct {
	Name     string
	Git      string
	Rev      string
	Matched  string
	Resolved string
	Items    []Item
}

// Item is one [[source.item]] block. Files is the list graft wrote for this item, and
// is the only thing that authorises graft to delete a file — nothing outside it may
// ever be removed.
type Item struct {
	ID    string
	Files []string
}

// file mirrors the on-disk shape of graft.lock.
type file struct {
	Version int      `toml:"version"`
	Sources []source `toml:"source"`
}

type source struct {
	Name     string `toml:"name"`
	Git      string `toml:"git"`
	Rev      string `toml:"rev"`
	Matched  string `toml:"matched"`
	Resolved string `toml:"resolved"`
	Items    []item `toml:"item"`
}

type item struct {
	ID    string   `toml:"id"`
	Files []string `toml:"files"`
}

// Load reads graft.lock from path. An absent file is not an error: it loads as an
// empty lock at the current format version, because a repo that has never synced is a
// legitimate starting state. Load creates, modifies, and deletes nothing.
func Load(path string) (*Lock, error) {
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return &Lock{Version: Version}, nil
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return Parse(data, name)
}

// Parse decodes and validates graft.lock bytes. filename appears only as the error
// prefix. On any error the returned lock is nil, so a caller can never derive a prune
// set from a lock that failed validation.
func Parse(data []byte, filename string) (*Lock, error) {
	// The generic decode runs first: it can only fail on a syntax problem, and its
	// tables carry the source and item names that MetaData.Undecoded drops for arrays
	// of tables.
	var raw map[string]any
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}

	// Version gating precedes every other check. A lock from a newer graft is expected
	// to carry keys this binary does not know, so rejecting it for an unknown key
	// instead of for its version would bury the one message that helps.
	if err := checkVersion(raw, filename); err != nil {
		return nil, err
	}

	var f file
	if _, err := toml.Decode(string(data), &f); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if err := rejectUnknown(raw, filename); err != nil {
		return nil, err
	}

	// rawSources is walked alongside f.Sources by index so validate can tell an absent
	// matched key from one declared empty — a distinction the typed decode alone
	// collapses, since both leave Matched as "".
	rawSources := tables(raw["source"])

	l := &Lock{Version: Version}
	seen := make(map[string]struct{}, len(f.Sources))
	// claimed spans the whole lock, not one item or one source: SPEC.md's invariants
	// say no two items share a destination path, within a source or across sources.
	claimed := map[string]struct{}{}
	for i, s := range f.Sources {
		var rawSrc map[string]any
		if i < len(rawSources) {
			rawSrc = rawSources[i]
		}
		src, err := validate(filename, s, claimed, rawSrc)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[src.Name]; dup {
			return nil, fmt.Errorf("%s: duplicate source %q", filename, src.Name)
		}
		seen[src.Name] = struct{}{}
		l.Sources = append(l.Sources, src)
	}
	return l, nil
}

func checkVersion(raw map[string]any, filename string) error {
	v, ok := raw["version"]
	if !ok {
		return fmt.Errorf("%s: version is required", filename)
	}
	// A version that is not an integer falls through to the typed decode, whose own
	// message describes the type problem better than anything invented here.
	n, ok := v.(int64)
	if !ok {
		return nil
	}
	switch {
	case n > Version:
		return fmt.Errorf("%s: version %d is not supported by this graft; upgrade graft", filename, n)
	case n < Version:
		return fmt.Errorf("%s: version %d is not a known lock version", filename, n)
	}
	return nil
}

// rejectUnknown walks the generically decoded document so an unknown key can name the
// source and item it appeared in.
func rejectUnknown(raw map[string]any, filename string) error {
	if k, ok := unknownKey(raw, "version", "source"); ok {
		return fmt.Errorf("%s: unknown key %q", filename, k)
	}
	for _, s := range tables(raw["source"]) {
		name, _ := s["name"].(string)
		if k, ok := unknownKey(s, "name", "git", "rev", "matched", "resolved", "item"); ok {
			return fmt.Errorf("%s: source %q: unknown key %q", filename, name, k)
		}
		for _, it := range tables(s["item"]) {
			id, _ := it["id"].(string)
			if k, ok := unknownKey(it, "id", "files"); ok {
				return fmt.Errorf("%s: source %q: item %q: unknown key %q", filename, name, id, k)
			}
		}
	}
	return nil
}

// unknownKey returns the lowest-sorting key of m that is not in allowed, so a table
// carrying several unknown keys always reports the same one.
func unknownKey(m map[string]any, allowed ...string) (string, bool) {
	var found []string
	for k := range m {
		if !slices.Contains(allowed, k) {
			found = append(found, k)
		}
	}
	if len(found) == 0 {
		return "", false
	}
	slices.Sort(found)
	return found[0], true
}

// tables reads an array of tables out of the generically decoded document. The two
// TOML spellings decode to different Go types — [[source]] gives []map[string]any and
// source = [{...}] gives []any — and they are the same document to a reader, so both
// have to be walked. Handling only the first silently accepted an unknown key written
// as an inline table, which is precisely the misspelling strict decoding exists for.
func tables(v any) []map[string]any {
	switch ts := v.(type) {
	case []map[string]any:
		return ts
	case []any:
		out := make([]map[string]any, 0, len(ts))
		for _, e := range ts {
			if m, ok := e.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	return nil
}

// validate checks one [[source]] block. claimed carries every file path already taken
// by an earlier item anywhere in the lock, and validate adds this source's to it. raw is
// this source's generically decoded table, needed only to tell an absent matched key
// from one declared empty — the typed decode above leaves Matched as "" for both.
func validate(filename string, s source, claimed map[string]struct{}, raw map[string]any) (Source, error) {
	if s.Name == "" {
		return Source{}, fmt.Errorf("%s: source name is empty", filename)
	}
	fail := func(msg string) error {
		return fmt.Errorf("%s: source %q: %s", filename, s.Name, msg)
	}

	switch {
	case s.Git == "":
		return Source{}, fail("git is required")
	case s.Rev == "":
		return Source{}, fail("rev is required")
	case s.Resolved == "":
		return Source{}, fail("resolved is required")
	case !isSHA(s.Resolved):
		return Source{}, fail(fmt.Sprintf("resolved %q is not a 40-character hex sha", s.Resolved))
	}

	// The range test is the same one internal/source asks before resolving: a second
	// definition would let a lock demand a matched for a pin resolution says has none.
	_, hasMatched := raw["matched"]
	switch {
	case !rev.IsRange(s.Rev) && hasMatched:
		return Source{}, fail("matched is only valid when rev is a range")
	case rev.IsRange(s.Rev) && !hasMatched:
		return Source{}, fail("matched is required when rev is a range")
	case rev.IsRange(s.Rev) && s.Matched == "":
		return Source{}, fail("matched is empty")
	}

	out := Source{Name: s.Name, Git: s.Git, Rev: s.Rev, Matched: s.Matched, Resolved: s.Resolved}
	seen := make(map[string]struct{}, len(s.Items))
	for _, it := range s.Items {
		if !itemid.Valid(it.ID) {
			return Source{}, fail(fmt.Sprintf("invalid item id %q: want kind:name", it.ID))
		}
		if _, dup := seen[it.ID]; dup {
			return Source{}, fail(fmt.Sprintf("duplicate item %q", it.ID))
		}
		seen[it.ID] = struct{}{}

		files := make([]string, 0, len(it.Files))
		for _, p := range it.Files {
			if !isRepoRelative(p) {
				return Source{}, fail(fmt.Sprintf("item %q: file %q is not a relative path inside the repo", it.ID, p))
			}
			if _, dup := claimed[p]; dup {
				return Source{}, fail(fmt.Sprintf("item %q: duplicate file %q", it.ID, p))
			}
			claimed[p] = struct{}{}
			files = append(files, p)
		}
		out.Items = append(out.Items, Item{ID: it.ID, Files: files})
	}
	return out, nil
}

func isSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

// isRepoRelative reports whether p is a path graft could have written inside the repo.
// The lock's files list is what authorises deletion, so a hand-edited or corrupt lock
// must not be able to aim a removal anywhere graft did not put a file.
//
// Escaping the tree is the obvious danger and ".." and a leading "/" cover it. Two
// quieter ones need the same rule: "." is the repo root, so a lock claiming it would
// authorise deleting the whole worktree, and an uncleaned alias like "./a.md" names the
// same file as "a.md" while slipping past the duplicate check. graft only ever writes
// cleaned paths, so requiring cleaned form rejects no lock graft produced.
func isRepoRelative(p string) bool {
	if p == "" || p == "." || strings.HasPrefix(p, "/") {
		return false
	}
	if slices.Contains(strings.Split(p, "/"), "..") {
		return false
	}
	return path.Clean(p) == p
}
