// Package catalog parses and validates catalog.yaml — a source repository's
// declaration of what it offers and where each class of thing belongs — and expands a
// consumer's selectors against it. It reads; it never writes.
package catalog

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

	// Lstat, never Stat, and before the read rather than after it: Stat would answer
	// about a symlink's target, and os.ReadFile would then read it. A source commits its
	// own catalog.yaml, so it may commit a link under that name — and a decoder error
	// quotes the offending lines back, which would hand a source the contents of any
	// file the invoking user can read. A rule enforced after the read is enforced too
	// late, because the read is the thing being prevented.
	//
	// internal/source refuses a non-regular `from` the same way, for the same reason.
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s not found: the source is not graftable", name)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		// A symlink, a directory, a device, or a fifo. One message for all of them: the
		// reader's next move is the same, and naming a link's target would be the leak.
		return nil, errf(name, "not a regular file")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		// Absence is Lstat's answer above, so this is a read that failed for another
		// reason — a permission denial, or the file changing underneath us.
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return Parse(data, name)
}

// Parse decodes and validates catalog.yaml bytes. filename appears only as the error
// prefix, so a message does not depend on where the file happened to live. On any
// error the returned catalog is nil — never a partially decoded one.
func Parse(data []byte, filename string) (*Catalog, error) {
	// Before the decode, because the decoder reads the first document and discards the
	// rest in silence — and because a fault after a marker is the extra document, not
	// the syntax error the decoder would report about it. See documents.
	if documents(data) > 1 {
		return nil, errf(filename, "multiple YAML documents; a catalog is a single document")
	}

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

	if err := rejectUnknown(raw, filename); err != nil {
		return nil, err
	}

	kinds, err := parseKinds(raw, filename)
	if err != nil {
		return nil, err
	}

	items, err := parseItems(raw, kinds, filename)
	if err != nil {
		return nil, err
	}
	return &Catalog{Version: version, Kinds: kinds, Items: items}, nil
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

	// The decoder picks uint64 for a non-negative literal and int64 for a negative
	// one. int is accepted as well, so a later change in that choice cannot make a
	// catalog this graft should read suddenly unreadable.
	var n int64
	switch t := v.(type) {
	case uint64:
		if t > math.MaxInt64 {
			// Not representable as int64, and unambiguously newer than the one
			// version this binary knows. Printed as written rather than clamped.
			return 0, errf(filename, "version %d is not supported by this graft; upgrade graft", t)
		}
		n = int64(t)
	case int64:
		n = t
	case int:
		n = int64(t)
	case string:
		// A literal too wide for any of those comes back as a string — the same Go
		// type a quoted version arrives as, so the two are told apart by the
		// literal's shape rather than by its type. A value that would fit never
		// arrives as a string, so a string carrying "1" was quoted deliberately.
		if negative, wide := wideLiteral(t); wide {
			if negative {
				// Below 1 however it is written, so it is not a future format.
				return 0, errf(filename, "version %s is not a known catalog version", t)
			}
			return 0, errf(filename, "version %s is not supported by this graft; upgrade graft", t)
		}
		return 0, errf(filename, "version must be an integer")
	default:
		return 0, errf(filename, "version must be an integer")
	}

	switch {
	case n > Version:
		return 0, errf(filename, "version %d is not supported by this graft; upgrade graft", n)
	case n < Version:
		return 0, errf(filename, "version %d is not a known catalog version", n)
	}
	return int(n), nil
}

// wideLiteral reports whether s is a decimal integer literal too wide for any 64-bit
// integer type, and whether it is negative. It exists for one input class and is not a
// general number parser: the decoder converts every integer literal it can hold, so a
// string reaching here is either a quoted value or a literal that overflowed.
//
// Only a range failure counts. ParseUint rejects "99...9_0" with ErrSyntax rather than
// ErrRange, which is why the separators are stripped first and why the two failures are
// not treated alike — conflating them would route a separated wide literal back to the
// non-integer message this case exists to avoid.
func wideLiteral(s string) (negative, wide bool) {
	digits := s
	switch {
	case strings.HasPrefix(digits, "-"):
		negative, digits = true, digits[1:]
	case strings.HasPrefix(digits, "+"):
		digits = digits[1:]
	}
	if digits == "" || digits[0] < '0' || digits[0] > '9' {
		return false, false
	}
	digits = strings.ReplaceAll(digits, "_", "")
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false, false
		}
	}

	var err error
	if negative {
		_, err = strconv.ParseInt("-"+digits, 10, 64)
	} else {
		_, err = strconv.ParseUint(digits, 10, 64)
	}
	return negative, errors.Is(err, strconv.ErrRange)
}
