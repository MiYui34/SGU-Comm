package emoji

import (
	"regexp"
	"strconv"
	"strings"
)

// Custom server emoji have names: lower case letters, digits and _, 2 to
// 32 of them. As a reaction (and in messages) they are ":name:".
var customName = regexp.MustCompile(`^[a-z0-9_]{2,32}$`)

// The web app checks names with the same rule (lib/customEmoji.ts).
func ValidName(name string) bool { return customName.MatchString(name) }

// A personal emoji in messages and as a reaction: "<:name:id>", the id of
// its image (as lib/personalEmoji.ts in the web app). The name is the
// sender's; Mumble clients see ":name:".
var refPattern = regexp.MustCompile(`<:([a-z0-9_]{2,32}):([1-9][0-9]{0,17})>`)

func Ref(name string, id int64) string { return "<:" + name + ":" + strconv.FormatInt(id, 10) + ">" }

// ParseRef reads a whole "<:name:id>".
func ParseRef(s string) (name string, id int64, ok bool) {
	m := refPattern.FindStringSubmatchIndex(s)
	if m == nil || m[0] != 0 || m[1] != len(s) {
		return "", 0, false
	}
	id, err := strconv.ParseInt(s[m[4]:m[5]], 10, 64)
	return s[m[2]:m[3]], id, err == nil
}

// RefsAsNames writes personal emoji as ":name:" (for Mumble clients).
func RefsAsNames(text string) string { return refPattern.ReplaceAllString(text, ":$1:") }

// CustomName is the name in ":name:" (a custom emoji reaction).
func CustomName(s string) (string, bool) {
	name, ok := strings.CutPrefix(s, ":")
	if name, ok2 := strings.CutSuffix(name, ":"); ok && ok2 && ValidName(name) {
		return name, true
	}
	return "", false
}
