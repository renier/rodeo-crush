package shellutil

import "testing"

func TestShellEscape(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"simple", "simple"},
		{"/path/to/file", "/path/to/file"},
		{"has space", "'has space'"},
		{"has\ttab", "'has\ttab'"},
		{"it's", "'it'\"'\"'s'"},
		{"back`tick", "'back`tick'"},
		{"semi;colon", "'semi;colon'"},
		{"dollar$var", "'dollar$var'"},
		{"", ""},
	}
	for _, tt := range tests {
		got := ShellEscape(tt.in)
		if got != tt.want {
			t.Errorf("ShellEscape(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
