package sshtransport

import (
	"strings"
	"testing"
)

// hostKeyPromptLines simulates what lineBuf actually contains right
// before each newline reset, for every line of a real captured host-key
// prompt against a disposable VM. See isHostKeyQuestionLine's doc comment
// and client.go's case comment for the full story: the previous condition
// needed text from two different lines at once and could never fire.
var hostKeyPromptLines = []struct {
	line      string
	wantMatch bool
}{
	{"The authenticity of host '[localhost]:2223 ([127.0.0.1]:2223)' can't be established.", false},
	{"ED25519 key fingerprint is: SHA256:Q4kAzTKwIBswYmz4IfO5SWIoqR5a+f4yTKsGBvJ7AqY", false},
	{"This key is not known by any other names.", false},
	{"", false},
	{"Are you sure you want to continue connecting (yes/no/[fingerprint])?", true},
	{"Are you sure you want to continue connecting (yes/no/[fingerprint])? ", true},
}

func TestIsHostKeyQuestionLine(t *testing.T) {
	for _, tc := range hostKeyPromptLines {
		if got := isHostKeyQuestionLine(tc.line); got != tc.wantMatch {
			t.Errorf("isHostKeyQuestionLine(%q) = %v, want %v", tc.line, got, tc.wantMatch)
		}
	}
}

// TestHostKeyPromptMatch_OldConditionWasUnreachable documents why the
// previous condition never fired in real use: it required both
// "authenticity of host" and "?" in the same lineBuf snapshot, but they
// never coexist once every line resets lineBuf on '\n'.
func TestHostKeyPromptMatch_OldConditionWasUnreachable(t *testing.T) {
	oldCondition := func(text string) bool {
		return strings.Contains(text, "authenticity of host") && strings.Contains(text, "?")
	}
	for _, tc := range hostKeyPromptLines {
		if oldCondition(tc.line) {
			t.Fatalf("old condition unexpectedly matched a single line %q - the bug this documents should make every line fail", tc.line)
		}
	}
}
