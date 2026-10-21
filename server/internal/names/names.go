// Package names holds the user name rules shared by accounts and the bridge.
package names

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Key is the form names are compared in: trimmed (Murmur trims too), NFC
// (composed and decomposed characters are the same name), lower-case.
func Key(name string) string {
	return strings.ToLower(norm.NFC.String(strings.TrimSpace(name)))
}

// Murmur's default "username" rule: letters, digits and -=_[]{}()@|.
var allowed = regexp.MustCompile(`^[-=\p{L}\p{N}_\[\]{}()@|.]+$`)

// Check returns why a name cannot be used for an account, or "".
func Check(name string) string {
	n := utf8.RuneCountInString(name)
	switch {
	case n < 2 || n > 32:
		return "the name needs 2 to 32 characters"
	case !allowed.MatchString(name):
		return "use letters, digits and - = _ [ ] { } ( ) @ | . only (no spaces)"
	case Key(name) == "superuser":
		return "this name is reserved"
	}
	return ""
}

// AllowedRune: the character may be in a name.
func AllowedRune(r rune) bool { return allowed.MatchString(string(r)) }
