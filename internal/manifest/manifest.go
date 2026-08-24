// Package manifest parses and validates graft.toml, the consumer's declaration of
// which items it wants from which sources. It reads; it never writes.
package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/optioni/graft/internal/itemid"
)

// Filename is the one spelling of the consumer's manifest. It lives here rather than at
// each call site because two packages now name the file — this one reads it, and the
// command that loads it locates it — and a string literal in two places is a rename that
// goes half done.
const Filename = "graft.toml"

// Manifest is a parsed graft.toml. Sources are ordered by name so that every error
// and every downstream walk is independent of TOML map iteration order.
type Manifest struct {
	Sources []Source
}

// Source is one [sources.<name>] block. Git is stored exactly as written — expanding
// shorthand to a clone URL belongs to the package that talks to git.
type Source struct {
	Name    string
	Git     string
	Rev     string
	Install []string
	Kinds   map[string]string
}

// file mirrors the on-disk shape of graft.toml. Decoding is strict: any key absent
// from this struct lands in MetaData.Undecoded and becomes an error.
type file struct {
	Sources map[string]source `toml:"sources"`
}

type source struct {
	Git     string            `toml:"git"`
	Rev     string            `toml:"rev"`
	Install []string          `toml:"install"`
	Kinds   map[string]string `toml:"kinds"`
}

// Load reads graft.toml from path and parses it. A file that does not exist is an
// error: the consumer's request is the one input graft cannot infer. Load creates,
// modifies, and deletes nothing.
func Load(path string) (*Manifest, error) {
	name := filepath.Base(path)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("%s not found", name)
		}
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return Parse(data, name)
}

// Parse decodes and validates graft.toml bytes. filename appears only as the error
// prefix, so a message does not depend on where the file happened to live. On any
// error the returned manifest is nil — never a partially populated one.
func Parse(data []byte, filename string) (*Manifest, error) {
	var f file
	md, err := toml.Decode(string(data), &f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filename, err)
	}
	if err := rejectUnknown(md, filename); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(f.Sources))
	for name := range f.Sources {
		names = append(names, name)
	}
	sort.Strings(names)

	m := &Manifest{}
	for _, name := range names {
		s, err := validate(filename, name, f.Sources[name])
		if err != nil {
			return nil, err
		}
		m.Sources = append(m.Sources, s)
	}
	return m, nil
}

// rejectUnknown turns the decoder's undecoded-key list into an error. Keys are sorted
// so a file with several unknown keys always reports the same one.
func rejectUnknown(md toml.MetaData, filename string) error {
	keys := md.Undecoded()
	if len(keys) == 0 {
		return nil
	}
	paths := make([][]string, 0, len(keys))
	for _, k := range keys {
		paths = append(paths, []string(k))
	}
	sort.Slice(paths, func(i, j int) bool {
		return strings.Join(paths[i], "\x00") < strings.Join(paths[j], "\x00")
	})

	k := paths[0]
	if len(k) >= 3 && k[0] == "sources" {
		return fmt.Errorf("%s: source %q: unknown key %q", filename, k[1], k[2])
	}
	return fmt.Errorf("%s: unknown key %q", filename, strings.Join(k, "."))
}

func validate(filename, name string, s source) (Source, error) {
	if name == "" {
		return Source{}, fmt.Errorf("%s: source name is empty", filename)
	}
	fail := func(msg string) error {
		return fmt.Errorf("%s: source %q: %s", filename, name, msg)
	}

	switch {
	case s.Git == "":
		return Source{}, fail("git is required")
	case s.Rev == "":
		return Source{}, fail("rev is required")
	case len(s.Install) == 0:
		return Source{}, fail("install must list at least one selector")
	}

	seen := make(map[string]struct{}, len(s.Install))
	for _, sel := range s.Install {
		if !itemid.Valid(sel) {
			return Source{}, fail(fmt.Sprintf("invalid selector %q: want kind:name", sel))
		}
		if _, dup := seen[sel]; dup {
			return Source{}, fail(fmt.Sprintf("duplicate selector %q", sel))
		}
		seen[sel] = struct{}{}
	}

	kinds := make([]string, 0, len(s.Kinds))
	for kind := range s.Kinds {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		if s.Kinds[kind] == "" {
			return Source{}, fail(fmt.Sprintf("kind %q: destination is required", kind))
		}
	}

	out := Source{
		Name:    name,
		Git:     s.Git,
		Rev:     s.Rev,
		Install: append([]string(nil), s.Install...),
	}
	if len(kinds) > 0 {
		out.Kinds = make(map[string]string, len(kinds))
		for _, kind := range kinds {
			out.Kinds[kind] = s.Kinds[kind]
		}
	}
	return out, nil
}
