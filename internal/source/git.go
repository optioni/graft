package source

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
)

// errNoGit is the failure-mode table's one runtime dependency, worded for a user rather
// than surfacing an exec.Error naming a Go package.
var errNoGit = errors.New("git not found on PATH")

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
func gitOutput(dir string, args ...string) (string, string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", "", errNoGit
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Prompting disabled, and nothing else scrubbed. The failure this prevents is a
	// hang: a private source with no usable credentials would otherwise block on a
	// password prompt forever inside what a caller believes is a function call.
	// GIT_ASKPASS, SSH_AUTH_SOCK, and credential.helper are how a private source works
	// at all, and SPEC.md promises graft does not interfere with them.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), firstLine(stderr.String()), err
}

// git runs a command for its exit status alone.
func git(dir string, args ...string) (string, error) {
	_, detail, err := gitOutput(dir, args...)
	return detail, err
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
