package peerops

import (
	"context"
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// TestResolveInterfaceRejectsUnsafeHint guards F2: an operator-supplied
// --interface value with shell metacharacters must be refused before it can
// ever be concatenated into a remote command, not passed through verbatim.
func TestResolveInterfaceRejectsUnsafeHint(t *testing.T) {
	unsafe := []string{
		"wg0; reboot",
		"wg0 && id",
		"$(touch /tmp/x)",
		"../../etc/passwd",
		"wg0`whoami`",
		"toolonginterfacename", // > 15 chars
	}
	for _, name := range unsafe {
		// A non-empty hint is validated and returns before any client call,
		// so a nil client is safe here: every unsafe value must be rejected
		// at that validation step.
		got, err := ResolveInterface(context.Background(), nil, name)
		if err == nil {
			t.Errorf("ResolveInterface(%q) = %q, nil; want an error", name, got)
		}
	}
}

// TestResolveInterfaceAcceptsSafeHint keeps the happy path working: a plain
// interface name is returned unchanged without touching the client.
func TestResolveInterfaceAcceptsSafeHint(t *testing.T) {
	for _, name := range []string{"wg0", "wg-office", "wg_0", "eth0"} {
		got, err := ResolveInterface(context.Background(), nil, name)
		if err != nil || got != name {
			t.Errorf("ResolveInterface(%q) = %q, %v; want %q, nil", name, got, err, name)
		}
	}
}

// TestFetchCurrentConfigRejectsUnsafeInterface guards F2 defense-in-depth:
// even a caller that bypasses ResolveInterface must not be able to smuggle a
// shell-unsafe interface name into the "cat /etc/wireguard/<iface>.conf"
// command.
func TestFetchCurrentConfigRejectsUnsafeInterface(t *testing.T) {
	_, err := FetchCurrentConfig(context.Background(), nil, sshtransport.Prompts{}, "wg0; rm -rf /")
	if err == nil {
		t.Fatal("FetchCurrentConfig accepted an unsafe interface name")
	}
}
