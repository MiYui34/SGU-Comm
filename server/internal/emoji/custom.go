package emoji

import (
	"regexp"
	"strings"
)

// Custom server emoji have names: lower case letters, digits and _, 2 to
// 32 of them. As a reaction (and in messages) they are ":name:".
var customName = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)

// The web app checks names with the same rule (lib/customEmoji.ts).
func ValidName(name string) bool { return customName.MatchString(name) }

// CustomName is the name in ":name:" (a custom emoji reaction).
func CustomName(s string) (string, bool) {
	name, ok := strings.CutPrefix(s, ":")
	if name, ok2 := strings.CutSuffix(name, ":"); ok && ok2 && ValidName(name) {
		return name, true
	}
	return "", false
}
