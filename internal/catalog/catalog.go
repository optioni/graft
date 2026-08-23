// Package catalog parses and validates catalog.yaml — a source repository's
// declaration of what it offers and where each class of thing belongs — and expands a
// consumer's selectors against it. It reads; it never writes.
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/goccy/go-yaml"
)

// Version is the catalog.yaml format version this binary understands. A catalog
// carrying a higher version is refused outright rather than half-read.
const Version = 1

// Catalog is a parsed catalog.yaml. Items are ordered by ID so that nothing
// downstream depends on the order a source happened to write its provides list.
type Catalog struct {
	Version int
	Kinds   map[string]Kind
	Items   []Item
}

// Kind is one kinds.<kind> declaration. To is always a list — a string-valued `to`
// becomes a one-element one — and every destination is carried exactly as written:
// {name} uninterpolated, trailing slashes intact, nothing cleaned. Computing a
// destination from these belongs to internal/plan, not here.
type Kind struct {
	To      []string
	Flatten bool
}

// Item is one provides entry. ID is Kind + ":" + Name and is the identity the lock
// records; From is a path within the source tree.
type Item struct {
	ID   string
	Kind string
	Name string
	From string
}

// Load reads catalog.yaml from path and parses it. A file that does not exist is the
// failure-mode table's "not graftable" error: graft never falls back to guessing a
// source's layout. Load creates, modifies, and deletes nothing, and reads no path
// other than the one it was given.
func Load(path string) (*Catalog, error) {
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s not found: the source is not graftable", name)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return Parse(data, name)
}

// Parse decodes and validates catalog.yaml bytes. filename appears only as the error
// prefix, so a message does not depend on where the file happened to live. On any
// error the returned catalog is nil — never a partially decoded one.
func Parse(data []byte, filename string) (*Catalog, error) {
	// The document is decoded generically rather than into a struct: unknown keys can
	// then be attributed to the kind or the provides index they appeared in, and the
	// string-or-list `to` is a type switch instead of a custom unmarshaller.
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	raw, err := document(doc, filename)
	if err != nil {
		return nil, err
	}

	// Version gating precedes every other check. A catalog written for a newer graft
	// is expected to carry keys this binary does not know, so answering it with an
	// unknown-key complaint would bury the one message that helps.
	version, err := checkVersion(raw, filename)
	if err != nil {
		return nil, err
	}

	kinds, err := parseKinds(raw, filename)
	if err != nil {
		return nil, err
	}

	c := &Catalog{Version: version, Kinds: kinds}
	for _, e := range sequence(raw["provides"]) {
		p, _ := e.(map[string]any)
		kind, _ := p["kind"].(string)
		itemName, _ := p["name"].(string)
		from, _ := p["from"].(string)
		c.Items = append(c.Items, Item{
			ID:   kind + ":" + itemName,
			Kind: kind,
			Name: itemName,
			From: from,
		})
	}
	sort.Slice(c.Items, func(i, j int) bool { return c.Items[i].ID < c.Items[j].ID })
	return c, nil
}

// errf builds an error prefixed with the file it came from. Every message this
// package returns for a catalog carries that prefix, and it is asserted by tests, so
// it is formatted in exactly one place.
func errf(filename, format string, args ...any) error {
	return fmt.Errorf("%s: %s", filename, fmt.Sprintf(format, args...))
}

// document narrows the decoded document to a mapping. An empty file decodes to nil
// and is treated as an empty mapping, so its failure is reported as the missing
// version it actually is rather than as a syntax error.
func document(doc any, filename string) (map[string]any, error) {
	switch d := doc.(type) {
	case nil:
		return map[string]any{}, nil
	case map[string]any:
		return d, nil
	}
	return nil, errf(filename, "top level must be a mapping")
}

func checkVersion(raw map[string]any, filename string) (int, error) {
	v, ok := raw["version"]
	if !ok || v == nil {
		return 0, errf(filename, "version is required")
	}
	n, ok := asInt(v)
	if !ok {
		return 0, errf(filename, "version must be an integer")
	}
	switch {
	case n > Version:
		return 0, errf(filename, "version %d is not supported by this graft; upgrade graft", n)
	case n < Version:
		return 0, errf(filename, "version %d is not a known catalog version", n)
	}
	return n, nil
}
