package cli

import (
	"bytes"
	"testing"
)

// These lock in a real bug found by an end-to-end run against
// nas.paynet.one: the standard library's flag.FlagSet stops parsing at the
// first non-flag token, so "wgtui server add office308 --ssh-host ..." (the
// exact form SPEC.md §21 documents, ID before flags) made every flag after
// the ID get swallowed as an unexpected extra positional argument instead
// of being parsed. Each command now peels the ID off args[0] before
// building its FlagSet.
//
// None of these reach the network: with no controlling terminal (as in a
// normal `go test` run), the master-password prompt fails fast with a
// clear error instead of hanging, so a command that gets *past* argument
// parsing is expected to fail with exit code 1 (runtime error), not 2
// (usage error) - that distinction is what these tests check.

func TestServerAdd_IDBeforeFlags_ParsesPastUsage(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	code := runServerAdd([]string{"office308", "--ssh-host", "nas.paynet.one", "--ssh-port", "2626"}, &out, &errOut)
	if code == 2 {
		t.Fatalf("got usage error (exit 2) for a validly-ordered command; stderr=%s", errOut.String())
	}
}

func TestServerInspect_IDBeforeJSONFlag_ParsesPastUsage(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	code := runServerInspect([]string{"office308", "--json"}, &out, &errOut)
	if code == 2 {
		t.Fatalf("got usage error (exit 2) for 'office308 --json'; stderr=%s", errOut.String())
	}
}

func TestPeerList_IDBeforeJSONFlag_ParsesPastUsage(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	code := runPeerList([]string{"office308", "--json"}, &out, &errOut)
	if code == 2 {
		t.Fatalf("got usage error (exit 2) for 'office308 --json'; stderr=%s", errOut.String())
	}
}

func TestDiagnose_IDBeforeJSONFlag_ParsesPastUsage(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	var out, errOut bytes.Buffer
	code := runDiagnose([]string{"office308", "--json"}, &out, &errOut)
	if code == 2 {
		t.Fatalf("got usage error (exit 2) for 'office308 --json'; stderr=%s", errOut.String())
	}
}

func TestServerInspect_MissingID_IsUsageError(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := runServerInspect(nil, &out, &errOut); code != 2 {
		t.Errorf("missing ID: exit code = %d, want 2", code)
	}
}
