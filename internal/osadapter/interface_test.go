package osadapter

import (
	"net/netip"
	"strings"
	"testing"
)

func validSpec() NewInterfaceSpec {
	return NewInterfaceSpec{
		Name:       "wg0",
		Address:    netip.MustParsePrefix("10.10.0.1/24"),
		ListenPort: 51820,
	}
}

func TestNewInterfaceSpec_Validate(t *testing.T) {
	if err := validSpec().validate(); err != nil {
		t.Errorf("valid spec rejected: %v", err)
	}

	bad := validSpec()
	bad.Name = "wg0; rm -rf /"
	if err := bad.validate(); err == nil {
		t.Error("expected error for unsafe interface name")
	}

	bad = validSpec()
	bad.Address = netip.Prefix{}
	if err := bad.validate(); err == nil {
		t.Error("expected error for invalid address")
	}

	bad = validSpec()
	bad.ListenPort = 0
	if err := bad.validate(); err == nil {
		t.Error("expected error for out-of-range port")
	}

	bad = validSpec()
	bad.NAT = true
	bad.OutboundInterface = "eth0; malicious"
	if err := bad.validate(); err == nil {
		t.Error("expected error for unsafe outbound interface name")
	}
}

func TestBuildCreateInterfaceScript(t *testing.T) {
	spec := validSpec()
	spec.NAT = true
	spec.OutboundInterface = "eth0"
	script := buildCreateInterfaceScript(spec)

	mustContain := []string{
		"set -e",
		"if [ -e \"$CONF\" ]; then echo WGTUI_ALREADY_EXISTS; exit 1; fi",
		"umask 077",
		"PRIVKEY=$(wg genkey)",
		"PUBKEY=$(printf '%s' \"$PRIVKEY\" | wg pubkey)",
		"echo \"PrivateKey = $PRIVKEY\"",
		"Address = 10.10.0.1/24",
		"ListenPort = 51820",
		"PostUp = iptables",
		"MASQUERADE",
		"chmod 600",
		"WGTUI_PUBKEY",
	}
	for _, want := range mustContain {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q\nfull script:\n%s", want, script)
		}
	}

	// The private key must never be a literal in the script - it only
	// ever exists as the $PRIVKEY shell variable, generated server-side.
	if strings.Contains(script, "PRIVKEY=") && strings.Count(script, "$PRIVKEY") == 0 {
		t.Error("script does not reference $PRIVKEY as a variable")
	}
}

func TestBuildCreateInterfaceScript_NoNAT(t *testing.T) {
	script := buildCreateInterfaceScript(validSpec())
	if strings.Contains(script, "PostUp") {
		t.Error("script should not contain PostUp when NAT is false")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("shellQuote(a'b) = %q", got)
	}
}
