package source

import "testing"

// TestCloneURL covers expansion. Every case is a pure function of a string, so the test
// creates no directory and runs no command — it is given no temp dir at all.
func TestCloneURL(t *testing.T) {
	cases := []struct {
		name string
		git  string
		want string
	}{
		{
			name: "shorthand expands to HTTPS",
			git:  "github.com/optioni/openspec-schemas",
			want: "https://github.com/optioni/openspec-schemas",
		},
		{
			name: "a URL carrying a scheme is passed through",
			git:  "https://example.com/team/assets.git",
			want: "https://example.com/team/assets.git",
		},
		{
			name: "an ssh scheme is passed through",
			git:  "ssh://git@example.com/team/assets.git",
			want: "ssh://git@example.com/team/assets.git",
		},
		{
			name: "an scp-style address is passed through",
			git:  "git@github.com:optioni/openspec-schemas.git",
			want: "git@github.com:optioni/openspec-schemas.git",
		},
		{
			name: "an absolute filesystem path is passed through",
			git:  "/srv/mirrors/openspec-schemas",
			want: "/srv/mirrors/openspec-schemas",
		},
		{
			name: "a relative filesystem path is passed through",
			git:  "../sibling-repo",
			want: "../sibling-repo",
		},
		{
			name: "a dot-relative filesystem path is passed through",
			git:  "./local-fixture",
			want: "./local-fixture",
		},
		{
			// The first segment has no dot, so it is not a hostname and the value is
			// not shorthand. Expanding it would invent a remote the user never named.
			name: "a two-segment path with no hostname is passed through",
			git:  "mirrors/assets",
			want: "mirrors/assets",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := CloneURL("shared", c.git)
			if err != nil {
				t.Fatalf("CloneURL(%q): unexpected error: %v", c.git, err)
			}
			if got != c.want {
				t.Errorf("CloneURL(%q):\n got %q\nwant %q", c.git, got, c.want)
			}
		})
	}
}

// TestCloneURLRefusesOption pins the refusal that keeps a manifest from choosing what
// runs. `git ls-remote --upload-pack=./pwn.sh refs/tags/v1` parses the first word as an
// option and executes the script, promoting the refspec to the repository operand — an
// explicit argv does not prevent it, because argv position is not what git uses to tell
// an option from an operand.
func TestCloneURLRefusesOption(t *testing.T) {
	const value = "--upload-pack=./pwn.sh"
	want := `source "shared": git "--upload-pack=./pwn.sh" may not begin with "-"`

	got, err := CloneURL("shared", value)
	if err == nil {
		t.Fatalf("CloneURL(%q): want an error, got %q", value, got)
	}
	if err.Error() != want {
		t.Errorf("CloneURL(%q):\n got %q\nwant %q", value, err.Error(), want)
	}
	if got != "" {
		t.Errorf("CloneURL(%q): want an empty URL on refusal, got %q", value, got)
	}
}
