package sshtransport

import "testing"

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
