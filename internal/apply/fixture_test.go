package apply_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/optioni/graft/internal/lock"
	"github.com/optioni/graft/internal/plan"
)

// The filesystem is real in every test here, under t.TempDir(). internal/apply is a
// filesystem package: a fake would test the fake, and the two conditions this package
// exists to refuse — a symlink where a regular file is expected, an ancestor that is not
// a directory — are exactly the ones a fake gets wrong.
//
// No git anywhere. A "fetched tree" is a directory a test wrote with os.WriteFile, which
// is all internal/apply ever sees of one.

// tree is a directory a test owns: a repository root, or a source's fetched tree.
type tree struct {
	t   *testing.T
	dir string
}

func newTree(t *testing.T) *tree {
	t.Helper()
	return &tree{t: t, dir: t.TempDir()}
}

// file writes a regular file at mode 0644, creating parents.
func (x *tree) file(path, content string) { x.fileMode(path, content, 0o644) }

// fileMode writes a regular file at an explicit mode, creating parents. os.WriteFile
// applies its perm only when it creates, so the mode is set afterwards — which is the
// same trap internal/apply has to avoid on the destination side.
func (x *tree) fileMode(path, content string, mode fs.FileMode) {
	x.t.Helper()
	full := x.path(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		x.t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.WriteFile(full, []byte(content), mode); err != nil {
		x.t.Fatalf("WriteFile %s: %v", full, err)
	}
	if err := os.Chmod(full, mode); err != nil {
		x.t.Fatalf("Chmod %s: %v", full, err)
	}
}

// mkdir creates a directory and its parents.
func (x *tree) mkdir(path string) {
	x.t.Helper()
	if err := os.MkdirAll(x.path(path), 0o755); err != nil {
		x.t.Fatalf("MkdirAll %s: %v", path, err)
	}
}

// symlink creates a link at path pointing at target, which is interpreted relative to
// the link's own directory exactly as the operating system does.
func (x *tree) symlink(target, path string) {
	x.t.Helper()
	full := x.path(path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		x.t.Fatalf("MkdirAll %s: %v", filepath.Dir(full), err)
	}
	if err := os.Symlink(filepath.FromSlash(target), full); err != nil {
		x.t.Fatalf("Symlink %s -> %s: %v", path, target, err)
	}
}

func (x *tree) path(p string) string { return filepath.Join(x.dir, filepath.FromSlash(p)) }

// read returns a file's contents, failing the test if it cannot be read.
func (x *tree) read(path string) string {
	x.t.Helper()
	data, err := os.ReadFile(x.path(path))
	if err != nil {
		x.t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

// exists reports whether anything is at path, without following a link at its last
// component.
func (x *tree) exists(path string) bool {
	x.t.Helper()
	_, err := os.Lstat(x.path(path))
	return err == nil
}

// mode returns a path's permission bits.
func (x *tree) mode(path string) fs.FileMode {
	x.t.Helper()
	fi, err := os.Lstat(x.path(path))
	if err != nil {
		x.t.Fatalf("Lstat %s: %v", path, err)
	}
	return fi.Mode().Perm()
}

// entries lists everything below the tree, each path slash-separated and relative, with
// a trailing slash on a directory. It is what makes "nothing outside the plan is
// touched" assertable as one comparison rather than a list of guesses.
func (x *tree) entries() []string {
	x.t.Helper()
	var out []string
	err := filepath.WalkDir(x.dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(x.dir, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		name := filepath.ToSlash(rel)
		if d.IsDir() {
			name += "/"
		}
		out = append(out, name)
		return nil
	})
	if err != nil {
		x.t.Fatalf("walking %s: %v", x.dir, err)
	}
	slices.Sort(out)
	return out
}

// assertEntries fails unless the tree holds exactly these paths.
func (x *tree) assertEntries(want ...string) {
	x.t.Helper()
	slices.Sort(want)
	got := x.entries()
	if !slices.Equal(got, want) {
		x.t.Errorf("tree contents:\n got %v\nwant %v", got, want)
	}
}

// sha is a plausible resolved sha. internal/plan refuses anything else, and a lock built
// in a test has to satisfy the same rule the real one does.
const sha = "fae2a30c1d4b8e9f0a2b3c4d5e6f708192a3b4c5"

// write is one plan.Write with this package's single fixture source name.
func write(from, dest string) plan.Write {
	return plan.Write{Source: "shared", Item: "schema:tdd", From: from, Dest: dest}
}

// lockOf builds a lock recording one item's files under the fixture source, which is
// what apply writes at the end of a run.
func lockOf(files ...string) *lock.Lock {
	return &lock.Lock{
		Version: lock.Version,
		Sources: []lock.Source{{
			Name:     "shared",
			Git:      "example.com/o/r",
			Rev:      "v1.0.0",
			Resolved: sha,
			Items:    []lock.Item{{ID: "schema:tdd", Files: files}},
		}},
	}
}

// emptyLock is the lock a plan with no sources produces.
func emptyLock() *lock.Lock { return &lock.Lock{Version: lock.Version} }

// assertError fails unless err's message is exactly want.
func assertError(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want %q", want)
	}
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err.Error(), want)
	}
}

// assertErrorPrefix fails unless err's message begins with want. It is for the messages
// that end in an operating-system reason, which is not graft's to pin.
func assertErrorPrefix(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("no error, want one beginning %q", want)
	}
	if !strings.HasPrefix(err.Error(), want) {
		t.Errorf("error = %q, want one beginning %q", err.Error(), want)
	}
}
