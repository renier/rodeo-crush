package shellutil

import "strings"

// ShellEscape wraps a string for safe inclusion in shell commands.
// It single-quotes the value when it contains any shell metacharacters.
func ShellEscape(s string) string {
	if !strings.ContainsAny(s, " \t\n'\"\\$`!#&|;(){}[]<>?*~") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
