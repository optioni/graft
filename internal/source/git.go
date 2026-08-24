package source

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// errNoGit is the failure-mode table's one runtime dependency, worded for a user rather
// than surfacing an exec.Error naming a Go package.
var errNoGit = errors.New("git not found on PATH")

// minGitMajor and minGitMinor are the oldest git that honours attr.tree, which is what
// keeps a source's own .gitattributes from touching a cache entry. git ignores an
// unknown -c key in silence, so on an older git that defence would fail open with
// nothing said — see checkVersion.
const (
	minGitMajor = 2
	minGitMinor = 40
)

// gitState names the environment variables that point git at a repository. graft may be
// running inside one: a post-merge hook, `git rebase --exec`, or `git bisect run` all
// set these, and an inherited GIT_INDEX_FILE makes the internal checkout write the
// consumer's index — verified, along with GIT_OBJECT_DIRECTORY depositing the source's
// objects in the consumer's .git/objects.
//
// That would break this package's central claim, so they are removed from the child's
// environment. Nothing else is: GIT_ASKPASS, SSH_AUTH_SOCK, and the user's
// credential.helper are how a private source works at all, and SPEC.md promises graft
// does not interfere with them.
var gitState = []string{
	"GIT_DIR",
	"GIT_WORK_TREE",
	"GIT_INDEX_FILE",
	"GIT_INDEX_VERSION",
	"GIT_OBJECT_DIRECTORY",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES",
	"GIT_COMMON_DIR",
	"GIT_NAMESPACE",
	"GIT_PREFIX",
}

// gitOutput runs one git command and returns its stdout. It is the only place in this
// package that starts a subprocess.
//
// The argv is explicit and there is no shell, so there are no quoting rules to get wrong.
// That is necessary and not sufficient: a value can still become a flag, which is why
// every caller passing a URL puts "--" before it and why CloneURL refuses a value
// beginning with "-".
//
// On failure the returned error carries git's first stderr line only. Everything after it
// is advice for a human at a terminal, and a multi-line error inside a per-source failure
// buries the line that identifies the problem.
func gitOutput(args ...string) (string, string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", errNoGit
	}
	cmd := exec.Command("git", args...)
	cmd.Env = gitEnv(os.Environ())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), firstLine(stderr.String()), err
}

// git runs a command for its exit status alone.
func git(args ...string) (string, error) {
	_, detail, err := gitOutput(args...)
	return detail, err
}

// gitEnv strips the repo-state variables from an environment and disables terminal
// prompting. Prompting is disabled because the failure it prevents is a hang: a private
// source with no usable credentials would otherwise block on a password prompt forever
// inside what a caller believes is a function call.
func gitEnv(environ []string) []string {
	out := make([]string, 0, len(environ)+1)
	for _, kv := range environ {
		name, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(gitState, name) {
			out = append(out, kv)
		}
	}
	return append(out, "GIT_TERMINAL_PROMPT=0")
}

// requireVersion fails when git is too old to honour attr.tree.
func requireVersion() error {
	out, _, err := gitOutput("--version")
	if err != nil {
		return err
	}
	return checkVersion(out)
}

// checkVersion parses `git --version` output and refuses a git older than the minimum.
// The check exists because git ignores an unknown -c key silently: without it, attr.tree
// would be dropped on an old git, a source's .gitattributes would take effect again, and
// nothing would say so.
//
// Unparseable output is accepted. A git that does not report its version the usual way is
// more likely a wrapper than an ancient binary, and refusing to run at all on a guess
// would be worse than the risk it hedges.
func checkVersion(out string) error {
	fields := strings.Fields(out)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return nil
	}
	parts := strings.Split(fields[2], ".")
	if len(parts) < 2 {
		return nil
	}
	major, okMajor := atoi(parts[0])
	minor, okMinor := atoi(parts[1])
	switch {
	case !okMajor || !okMinor:
		return nil
	case major > minGitMajor, major == minGitMajor && minor >= minGitMinor:
		return nil
	}
	return fmt.Errorf("git %s is too old: graft needs git %d.%d or newer", fields[2], minGitMajor, minGitMinor)
}

func atoi(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// firstLine reduces captured stderr to its first non-empty trimmed line.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
