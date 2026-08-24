package manifest_test

import (
	"testing"

	"github.com/optioni/graft/internal/manifest"
)

// A multi-line string is a value, and everything inside it is text a human quoted. A header
// or a `rev` written in one belongs to whichever key opened the string, so a scanner that
// read them as syntax would edit a line in a completely different key's value — the failure
// this function exists to make impossible, and the one the re-parse in internal/sync cannot
// always catch, because a manifest corrupted this way still parses.
func TestSetRevDoesNotReadInsideAMultiLineString(t *testing.T) {
	t.Parallel()

	const wantErr = `graft.toml: source "b": cannot move the pin: rev is not a plain key under [sources.b]`

	for name, in := range map[string]string{
		"a basic multi-line string":   "[sources.a]\ngit = \"\"\"\n[sources.b]\nrev = \"zzz\"\n\"\"\"\nrev = \"v1\"\n",
		"a literal multi-line string": "[sources.a]\ngit = '''\n[sources.b]\nrev = \"zzz\"\n'''\nrev = \"v1\"\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out, err := manifest.SetRev([]byte(in), "b", "v2")
			if err == nil {
				t.Fatalf("SetRev rewrote a line inside a quoted value:\n%q", out)
			}
			if err.Error() != wantErr {
				t.Errorf("error = %q, want %q", err, wantErr)
			}
			if out != nil {
				t.Errorf("returned %q, want no bytes at all", out)
			}
		})
	}
}

// The source whose own table holds the string still moves, and the string is untouched.
func TestSetRevMovesThePinPastAMultiLineString(t *testing.T) {
	t.Parallel()

	in := "[sources.shared]\nnote = \"\"\"\nrev = \"decoy\"\n\"\"\"\nrev     = \"v1.0.0\"\n"
	want := "[sources.shared]\nnote = \"\"\"\nrev = \"decoy\"\n\"\"\"\nrev     = \"v1.1.0\"\n"
	if got := setRev(t, in, "shared", "v1.1.0"); got != want {
		t.Errorf("SetRev =\n%q\nwant\n%q", got, want)
	}
}

// A one-line value carrying an escaped quote, and the shapes that look like the key without
// being it. Each is one branch of the scanner, and each would otherwise be asserted by
// nothing.
func TestSetRevReadsTheLineExactly(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct{ in, want string }{
		"an escaped quote inside an earlier value": {
			in:   "[sources.shared]\ngit  = \"a\\\"b\"\nrev  = \"v1.0.0\"\n",
			want: "[sources.shared]\ngit  = \"a\\\"b\"\nrev  = \"v1.1.0\"\n",
		},
		"a key that merely starts with rev": {
			in:   "[sources.shared]\nrevision = \"no\"\nrev = \"v1.0.0\"\n",
			want: "[sources.shared]\nrevision = \"no\"\nrev = \"v1.1.0\"\n",
		},
		"a header whose key path never closes its quote": {
			in:   "[sources.\"shared]\n[sources.shared]\nrev = \"v1.0.0\"\n",
			want: "[sources.\"shared]\n[sources.shared]\nrev = \"v1.1.0\"\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := setRev(t, tc.in, "shared", "v1.1.0"); got != tc.want {
				t.Errorf("SetRev =\n%q\nwant\n%q", got, tc.want)
			}
		})
	}
}
