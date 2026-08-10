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

// TestNormalizePTYCapture_StripsSudoPwfeedbackEcho reproduces byte-for-byte
// what a real Ubuntu 26.04 target with `Defaults pwfeedback` sent back
// after answering its sudo password prompt (captured live via a debug
// hook on runInteractive): a leading space, one '*' per password
// character, then sudo's own backspace/space erasure of that masked line,
// directly followed by the real command output. Uncaught, this corrupted
// prefix was silently embedded into the next config content a caller
// built from the capture (transaction.Plan.NewContent), which then failed
// `wg-quick strip` validation on the remote side.
func TestNormalizePTYCapture_StripsSudoPwfeedbackEcho(t *testing.T) {
	var b strings.Builder
	b.WriteString(" ") // sudo's leading space before the masked line
	stars := 23        // matches a real captured password length
	for i := 0; i < stars; i++ {
		b.WriteByte('*')
	}
	b.WriteByte('\b')
	for i := 0; i < stars; i++ {
		b.WriteString(" \b\b")
	}
	b.WriteString("\n[Interface]\nPrivateKey = abc\n")

	got := normalizePTYCapture([]byte(b.String()))
	want := "[Interface]\nPrivateKey = abc\n"
	if string(got) != want {
		t.Fatalf("normalizePTYCapture did not strip pwfeedback echo:\ngot  %q\nwant %q", got, want)
	}
}

// TestStripPasswordFeedbackEcho_LeavesRealAsteriskContentAlone guards
// against the stripper eating legitimate output that happens to start
// with literal asterisks but isn't followed by backspace/space erasure
// bytes - the one thing that never happens in real command output.
func TestStripPasswordFeedbackEcho_LeavesRealAsteriskContentAlone(t *testing.T) {
	in := "*** WARNING: this is a real banner ***\nsome output\n"
	if got := stripPasswordFeedbackEcho(in); got != in {
		t.Fatalf("stripPasswordFeedbackEcho altered non-artifact content:\ngot  %q\nwant %q", got, in)
	}
}
