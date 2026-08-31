package add

import (
	"fmt"
	"testing"
)

func TestSplitSource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		spec     string
		git, rev string
		hasRev   bool
	}{
		{name: "no rev", spec: "optioni/shared", git: "optioni/shared"},
		{name: "a tag", spec: "optioni/shared@v1.0.0", git: "optioni/shared", rev: "v1.0.0", hasRev: true},
		{name: "a range", spec: "optioni/shared@^1.2.0", git: "optioni/shared", rev: "^1.2.0", hasRev: true},
		{
			name: "an scp-style address keeps its own @",
			spec: "git@github.com:optioni/shared",
			git:  "git@github.com:optioni/shared",
		},
		{
			name:   "an scp-style address with a rev splits at the last @",
			spec:   "git@github.com:optioni/shared@v1.0.0",
			git:    "git@github.com:optioni/shared",
			rev:    "v1.0.0",
			hasRev: true,
		},
		{
			name:   "a URL with a rev",
			spec:   "https://github.com/optioni/shared.git@v1.0.0",
			git:    "https://github.com/optioni/shared.git",
			rev:    "v1.0.0",
			hasRev: true,
		},
		{
			name:   "an @ with nothing after it is a rev that was not named",
			spec:   "optioni/shared@",
			git:    "optioni/shared",
			hasRev: true,
		},
		{name: "no separator at all", spec: "shared", git: "shared"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			git, rev, hasRev := SplitSource(tt.spec)
			if git != tt.git || rev != tt.rev || hasRev != tt.hasRev {
				t.Errorf("SplitSource(%q) = (%q, %q, %t), want (%q, %q, %t)",
					tt.spec, git, rev, hasRev, tt.git, tt.rev, tt.hasRev)
			}
		})
	}
}

// Three spellings of one repository derive one name, which is what lets a consumer switch
// between them without the manifest's own key moving.
func TestDeriveNameOfOneRepositorySpelledThreeWays(t *testing.T) {
	t.Parallel()

	for _, git := range []string{
		"optioni/shared",
		"https://github.com/optioni/shared.git",
		"git@github.com:optioni/shared",
		"github.com/optioni/shared/",
		"/absolute/path/to/shared",
	} {
		got, err := DeriveName(git)
		if err != nil {
			t.Fatalf("DeriveName(%q): %v", git, err)
		}
		if got != "shared" {
			t.Errorf("DeriveName(%q) = %q, want %q", git, got, "shared")
		}
	}
}

func TestDeriveNameRefusesWhatItCannotWriteAsABareKey(t *testing.T) {
	t.Parallel()

	// "optioni/" is deliberately absent: it derives "optioni", which is a name graft can
	// write. That the value is not a repository is git's answer to give, not this rule's.
	for _, git := range []string{"optioni/sh ared", "optioni/my.repo", "", `optioni/"x"`, "optioni/.git"} {
		got, err := DeriveName(git)
		if err == nil {
			t.Fatalf("DeriveName(%q) = %q, want a refusal", git, got)
		}
		if want := fmt.Sprintf("cannot derive a source name from %q", git); err.Error() != want {
			t.Errorf("error for %q = %q, want %q", git, err, want)
		}
	}
}
