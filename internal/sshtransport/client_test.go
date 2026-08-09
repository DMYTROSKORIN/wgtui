package sshtransport

import (
	"errors"
	"strings"
	"testing"
)

func TestBaseArgs_IncludesTargetAndControlPath(t *testing.T) {
	c := NewClient(Target{HostOrAlias: "nas.paynet.one", Port: 2626, User: "skorin", IdentityFile: "/home/x/.ssh/id_ed25519"}, "/tmp/wgtui-test.sock")
	args := c.baseArgs()

	wantPairs := [][2]string{
		{"-p", "2626"},
		{"-l", "skorin"},
		{"-i", "/home/x/.ssh/id_ed25519"},
		{"-o", "ControlPath=/tmp/wgtui-test.sock"},
	}
	for _, pair := range wantPairs {
		if !containsPair(args, pair[0], pair[1]) {
			t.Errorf("baseArgs() = %v, missing %q %q", args, pair[0], pair[1])
		}
	}
}

func TestBaseArgs_NeverStrictHostKeyCheckingNo(t *testing.T) {
	// SPEC.md §10.2, §23.1: never disable host key checking by default.
	c := NewClient(Target{HostOrAlias: "x"}, "/tmp/sock")
	args := c.baseArgs()
	if containsPair(args, "-o", "StrictHostKeyChecking=no") {
		t.Fatal("baseArgs() must never set StrictHostKeyChecking=no")
	}
	if !containsPair(args, "-o", "StrictHostKeyChecking=ask") {
		t.Fatal("baseArgs() must set StrictHostKeyChecking=ask")
	}
}

func containsPair(args []string, flag, val string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == val {
			return true
		}
	}
	return false
}

func TestSudoPasswordWasRejected(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{"lowercase incorrect password", "Sorry, try again.\nsudo: 1 incorrect password attempt", true},
		{"sorry try again only", "[sudo] password for skorin: Sorry, try again.\n[sudo] password for skorin: ", true},
		{"unrelated command failure", "bash: apt-get: command not found", false},
		{"not in sudoers", "skorin is not in the sudoers file.  This incident will be reported.", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sudoPasswordWasRejected([]byte(tc.out)); got != tc.want {
				t.Fatalf("sudoPasswordWasRejected(%q) = %v, want %v", tc.out, got, tc.want)
			}
		})
	}
}

func TestInteractiveOutputReturnsFullBufferOnErrorNotJustTheStrippedTail(t *testing.T) {
	full := "[sudo] password for skorin: Sorry, try again.\n[sudo] password for skorin: sudo: 2 incorrect password attempts"
	// outputStart simulates the second "password for" match advancing past
	// the "Sorry, try again." explanation - the exact scenario that used to
	// silently discard it.
	outputStart := len("[sudo] password for skorin: Sorry, try again.\n[sudo] password for skorin: ")

	onError := interactiveOutput(full, outputStart, errTestSentinel)
	if !strings.Contains(string(onError), "Sorry, try again") {
		t.Fatalf("on error, interactiveOutput dropped the rejection explanation: %q", onError)
	}

	onSuccess := interactiveOutput(full, outputStart, nil)
	if strings.Contains(string(onSuccess), "Sorry, try again") {
		t.Fatalf("on success, interactiveOutput kept prompt boilerplate it should have stripped: %q", onSuccess)
	}
}

var errTestSentinel = errors.New("test failure")
