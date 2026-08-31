package add

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/optioni/graft/internal/apply"
	"github.com/optioni/graft/internal/manifest"
	"github.com/optioni/graft/internal/source"
	"github.com/optioni/graft/internal/sync"
)

// Request is one `graft add`, already split into its parts by the command surface.
type Request struct {
	// Root is the repository graft runs in, and CacheRoot the content-addressed fetch
	// cache. Both are values rather than things this package looks up, so no test can
	// write into the developer's real cache.
	Root      string
	CacheRoot string

	// Git is the source's git value, written into graft.toml exactly as given.
	Git string

	// Rev is the rev given after `@`, or "" to resolve the source's default pin. An
	// existing source keeps the rev it has when this is empty: an add that quietly bumped
	// a pin whenever it added a selector would be a second `update`.
	Rev string

	// Install is the selectors to declare, in the order given.
	Install []string

	// NoSync writes graft.toml and stops: nothing is fetched for a plan, nothing is
	// written to a destination, and graft.lock is neither written nor created.
	NoSync bool
}

// Report is what an add did to graft.toml, and what the sync that followed it reported.
type Report struct {
	// Edits are the manifest-edit lines, in the order they are printed.
	Edits []string

	// Sync is the report of the sync that followed, or nil under --no-sync.
	Sync *sync.Report
}

// Run performs `graft add`: amend graft.toml so it declares what the invocation asked for,
// then sync the result — or, under --no-sync, write the manifest and stop.
//
// The order is the contract, and it is the same shape internal/sync's is. Everything up to
// the amendment creates, modifies, and deletes nothing: the name is derived, the selectors
// are already checked, the manifest is read, the edit is computed in memory, and the result
// is re-parsed and checked against what was asked for. Only then does anything reach disk,
// through internal/apply either way.
//
// Every error is returned exactly as the package that raised it worded it. SPEC.md's
// failure-mode table is written as those messages, and a second layer of context here would
// say the same thing twice.
func Run(r Request) (*Report, error) {
	name, err := DeriveName(r.Git)
	if err != nil {
		return nil, err
	}

	data, m, err := readManifest(r.Root)
	if err != nil {
		return nil, err
	}

	amended, edits, err := amend(r, name, data, m)
	if err != nil {
		return nil, err
	}

	changed := !slices.Equal(amended, data)
	if !changed {
		edits = []string{unchangedLine}
	}

	if r.NoSync {
		if changed {
			if err := apply.Manifest(r.Root, amended); err != nil {
				return nil, err
			}
		}
		return &Report{Edits: edits}, nil
	}

	o := sync.Options{Root: r.Root, CacheRoot: r.CacheRoot}
	if changed {
		// The bytes the run resolves are the bytes it writes. An unchanged manifest is
		// left to internal/sync to read for itself, which is the same file.
		o.Manifest = amended
	}
	if movedPin(edits) {
		// The one source whose pin moved is re-resolved, exactly as `update <source>`
		// re-resolves one. Every other source's sha still comes from graft.lock, and this
		// source's would too — against a pin that no longer matches it.
		o.Update = &sync.Update{Source: name}
	}

	report, err := sync.Run(o)
	if err != nil {
		return nil, err
	}
	return &Report{Edits: edits, Sync: report}, nil
}

// readManifest reads graft.toml, treating its absence as an empty manifest.
//
// `add` is the only command permitted to create the file, which is why absence is not an
// error here and is one everywhere else: every other command failing on it says "you are
// not in a graft repository", and add is the answer to that. A file that exists and does
// not parse is a different thing entirely and is refused in internal/manifest's own words —
// treating it as absent would overwrite a broken manifest with a fresh one holding a single
// source, destroying the consumer's file.
func readManifest(root string) ([]byte, *manifest.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(root, manifest.Filename))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &manifest.Manifest{}, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", manifest.Filename, err)
	}
	m, err := manifest.Parse(data, manifest.Filename)
	if err != nil {
		return nil, nil, err
	}
	return data, m, nil
}

// amend computes the graft.toml bytes this add will write, and the lines describing what it
// did. It reads no file and writes none.
//
// The result is re-parsed and checked against the request before it is returned. That check
// is not belt and braces: both edits are text edits, and a text edit's real failure is
// landing somewhere other than the line it named — which produces a file that parses
// perfectly while the run installs something else.
func amend(r Request, name string, data []byte, m *manifest.Manifest) ([]byte, []string, error) {
	var (
		edits []string
		err   error
	)

	existing := declared(m, name)
	amended := slices.Clone(data)
	rev := r.Rev

	if existing == nil {
		if rev == "" {
			// The one network call an add with no @rev makes before it writes anything.
			if rev, err = source.DefaultRev(name, r.Git); err != nil {
				return nil, nil, err
			}
		}
		if amended, err = manifest.AddSource(amended, name, r.Git, rev, dedupe(r.Install)); err != nil {
			return nil, nil, err
		}
		edits = append(edits, fmt.Sprintf("%s: added source %q at %s", manifest.Filename, name, rev))
	} else {
		if existing.Git != r.Git {
			// Two repositories cannot share a source name. Retargeting a declared source
			// would move every file it owns, so the collision is named rather than
			// resolved: the message carries the value the manifest holds, which is the
			// fact the consumer needs to pick a way out.
			return nil, nil, fmt.Errorf("%s: source %q: already declared with git %q",
				manifest.Filename, name, existing.Git)
		}
		if rev == "" {
			rev = existing.Rev
		}
		if rev != existing.Rev {
			if amended, err = manifest.SetRev(amended, name, rev); err != nil {
				return nil, nil, err
			}
			edits = append(edits, fmt.Sprintf("%s: moved source %q to %s", manifest.Filename, name, rev))
		}

		added := missing(existing.Install, dedupe(r.Install))
		if len(added) > 0 {
			if amended, err = manifest.AddInstall(amended, name, added); err != nil {
				return nil, nil, err
			}
			edits = append(edits, fmt.Sprintf("%s: added %s to source %q",
				manifest.Filename, strings.Join(added, ", "), name))
		}
	}

	if err := check(amended, name, r.Git, rev, r.Install); err != nil {
		return nil, nil, err
	}
	return amended, edits, nil
}

// check re-parses the amended bytes and confirms they say what was asked for.
func check(amended []byte, name, git, rev string, install []string) error {
	fail := fmt.Errorf("%s: source %q: the amendment did not take effect", manifest.Filename, name)

	m, err := manifest.Parse(amended, manifest.Filename)
	if err != nil {
		return err
	}
	s := declared(m, name)
	if s == nil || s.Git != git || s.Rev != rev {
		return fail
	}
	for _, sel := range install {
		if !slices.Contains(s.Install, sel) {
			return fail
		}
	}
	return nil
}

// declared returns the manifest's source of this name, or nil.
func declared(m *manifest.Manifest, name string) *manifest.Source {
	for i := range m.Sources {
		if m.Sources[i].Name == name {
			return &m.Sources[i]
		}
	}
	return nil
}

// dedupe keeps the first occurrence of each selector. A manifest declaring one selector
// twice is one manifest.Parse refuses, so a command line naming one twice may not become
// one.
func dedupe(selectors []string) []string {
	seen := make(map[string]struct{}, len(selectors))
	out := make([]string, 0, len(selectors))
	for _, sel := range selectors {
		if _, dup := seen[sel]; dup {
			continue
		}
		seen[sel] = struct{}{}
		out = append(out, sel)
	}
	return out
}

// missing is the selectors not already in have, in the order given.
func missing(have, want []string) []string {
	out := make([]string, 0, len(want))
	for _, sel := range want {
		if !slices.Contains(have, sel) {
			out = append(out, sel)
		}
	}
	return out
}

// unchangedLine is what an add that asked for nothing new reports. It still syncs: the
// manifest already said what was wanted, and the tree may not yet match it.
const unchangedLine = manifest.Filename + ": unchanged"

// movedPin reports whether the edits moved a pin, which is what decides whether this run
// re-resolves the source it touched.
func movedPin(edits []string) bool {
	return slices.ContainsFunc(edits, func(line string) bool {
		return strings.HasPrefix(line, manifest.Filename+": moved source ")
	})
}
