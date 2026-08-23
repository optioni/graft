// Package buildinfo formats the build details injected by the linker.
package buildinfo

import "fmt"

// Format renders the line printed by `graft --version`. Empty fields render as
// "unknown" so a binary built without ldflags still prints something meaningful.
func Format(version, commit, date string) string {
	return fmt.Sprintf("graft %s (%s, built %s)", orUnknown(version), orUnknown(commit), orUnknown(date))
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
