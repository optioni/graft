package manifest

import (
	"fmt"
	"strings"
)

// keyWidth is the column the appended block's `=` signs line up in: the width of the
// longest key it writes. SPEC.md's own graft.toml example is aligned this way, and a
// block that arrives misaligned beside it is a diff the consumer has to fix by hand.
const keyWidth = len(installKey)

const (
	installKey = "install"
	gitKey     = "git"
)

// AddSource returns data with one new [sources.<name>] table appended and every existing
// byte left exactly as it was.
//
// It is the append half of the same bargain SetRev makes for the pin move: a block graft
// writes gets graft's formatting, because it has none of the consumer's to preserve, and
// nothing the consumer wrote is ever re-rendered. The original bytes are a prefix of the
// result, with one exception — a file whose last byte is not a newline gains one, because
// a table appended onto a truncated final line would corrupt that line.
//
// AddSource creates, modifies, and deletes nothing. It returns bytes; internal/apply is
// the only package that puts them anywhere. On any error it returns no bytes at all.
func AddSource(data []byte, name, git, rev string, install []string) ([]byte, error) {
	if !bareKey(name) {
		return nil, fmt.Errorf("%s: source name %q is not a bare key", Filename, name)
	}
	if err := checkLiteral(gitKey, git); err != nil {
		return nil, err
	}
	if err := checkLiteral(revKey, rev); err != nil {
		return nil, err
	}
	for _, sel := range install {
		if err := checkLiteral("selector", sel); err != nil {
			return nil, err
		}
	}
	if len(install) == 0 {
		// The wording validate already produces for the same condition, rather than a
		// second sentence saying it: a manifest this package writes that the next run
		// refuses to read is the worst failure available here.
		return nil, fmt.Errorf("%s: source %q: install must list at least one selector", Filename, name)
	}

	text := string(data)
	if _, _, found := sourceTableSpan(text, name); found {
		return nil, fmt.Errorf("%s: source %q: already declared", Filename, name)
	}

	var b strings.Builder
	b.WriteString(text)
	if text != "" && !strings.HasSuffix(text, "\n") {
		b.WriteString("\n")
	}
	// A blank line separates the block from content, and only from content: a file that
	// is empty or holds only whitespace receives the table with nothing ahead of it.
	if strings.TrimSpace(text) != "" {
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "[sources.%s]\n", name)
	writeKey(&b, gitKey, quoted(git))
	writeKey(&b, revKey, quoted(rev))

	selectors := make([]string, 0, len(install))
	for _, sel := range install {
		selectors = append(selectors, quoted(sel))
	}
	writeKey(&b, installKey, "["+strings.Join(selectors, ", ")+"]")

	return []byte(b.String()), nil
}

// writeKey renders one aligned `key = value` line.
func writeKey(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "%-*s = %s\n", keyWidth, key, value)
}

// quoted wraps a value that checkLiteral has already cleared, so there is nothing left to
// escape. Anything needing an escape was refused rather than rewritten.
func quoted(s string) string { return `"` + s + `"` }

// bareKey reports whether name may be written as a TOML bare key.
//
// A dot is excluded along with everything else outside the set: [sources.my.repo] is a
// sub-table of `my`, not the source named `my.repo`, so a dotted name would write a file
// that parses, declares a source nobody asked for, and whose install array the amender
// can never find again. A quoted key would parse too, and would leave a shape the next
// in-place edit has to guess at — which is the thing this package refuses to do.
func bareKey(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}
