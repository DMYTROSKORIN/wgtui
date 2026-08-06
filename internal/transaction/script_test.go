package transaction

import (
	"strings"
	"testing"
)

func validPlan() Plan {
	return Plan{
		Interface:           "wg0",
		ConfigPath:          "/etc/wireguard/wg0.conf",
		ExpectedFingerprint: "deadbeef",
		NewContent:          "[Interface]\nPrivateKey = should-not-appear-since-this-is-just-a-test-marker\n",
	}
}

func TestBuildScript_ContainsExpectedSteps(t *testing.T) {
	script, err := buildScript(validPlan())
	if err != nil {
		t.Fatalf("buildScript: %v", err)
	}
	mustContain := []string{
		"set -e",
		"mkdir \"$LOCKDIR\"",
		"trap 'rmdir \"$LOCKDIR\" 2>/dev/null' EXIT",
		"sha256sum \"$CONF\"",
		"WGTUI_FINGERPRINT_MISMATCH",
		"cp -p \"$CONF\" \"$BACKUP\"",
		"wg-quick strip \"$TMP\"",
		"WGTUI_VALIDATE_FAILED",
		"mv \"$TMP\" \"$CONF\"",
		"wg syncconf \"$IFACE\"",
		"WGTUI_APPLY_FAILED_ROLLED_BACK",
		"WGTUI_OK",
		"should-not-appear-since-this-is-just-a-test-marker", // the actual payload, via the heredoc
	}
	for _, want := range mustContain {
		if !strings.Contains(script, want) {
			t.Errorf("script missing expected fragment %q", want)
		}
	}
}

func TestBuildScript_RejectsUnsafeInterfaceName(t *testing.T) {
	p := validPlan()
	p.Interface = "wg0; rm -rf /"
	if _, err := buildScript(p); err == nil {
		t.Fatal("expected an error for an unsafe interface name, got nil")
	}
}

func TestBuildScript_RejectsIncompletePlan(t *testing.T) {
	cases := []Plan{
		{ConfigPath: "/etc/wireguard/wg0.conf", ExpectedFingerprint: "x"},
		{Interface: "wg0", ExpectedFingerprint: "x"},
		{Interface: "wg0", ConfigPath: "/etc/wireguard/wg0.conf"},
	}
	for _, p := range cases {
		if _, err := buildScript(p); err == nil {
			t.Errorf("expected an error for incomplete plan %+v, got nil", p)
		}
	}
}

func TestBuildScript_ConfigPathIsSafelyQuoted(t *testing.T) {
	p := validPlan()
	p.ConfigPath = "/etc/wireguard/wg'0.conf" // a single quote in the path
	script, err := buildScript(p)
	if err != nil {
		t.Fatalf("buildScript: %v", err)
	}
	if !strings.Contains(script, `CONF='/etc/wireguard/wg'\''0.conf'`) {
		t.Errorf("expected the single quote to be shell-escaped in the CONF assignment, got script:\n%s", script)
	}
}

func TestBuildScript_TwoCallsUseDifferentDelimiters(t *testing.T) {
	s1, err := buildScript(validPlan())
	if err != nil {
		t.Fatalf("buildScript: %v", err)
	}
	s2, err := buildScript(validPlan())
	if err != nil {
		t.Fatalf("buildScript: %v", err)
	}
	if s1 == s2 {
		t.Fatal("two calls with the same Plan produced byte-identical scripts - delimiter isn't actually random")
	}
}

func TestShellSingleQuote(t *testing.T) {
	cases := map[string]string{
		"simple":    `'simple'`,
		"":          `''`,
		"a'b":       `'a'\''b'`,
		"a'b'c":     `'a'\''b'\''c'`,
		"$(rm -rf)": `'$(rm -rf)'`,
	}
	for in, want := range cases {
		if got := shellSingleQuote(in); got != want {
			t.Errorf("shellSingleQuote(%q) = %q, want %q", in, got, want)
		}
	}
}
