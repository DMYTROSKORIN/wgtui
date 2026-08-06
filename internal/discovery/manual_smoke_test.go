//go:build manual

// Run against a real server you own, read-only:
//
//	WGTUI_MANUAL_HOST=nas.paynet.one \
//	WGTUI_MANUAL_PORT=2626 \
//	WGTUI_MANUAL_USER=skorin \
//	WGTUI_MANUAL_SUDO_PASS_FILE=/path/to/private/0600/file \
//	go test -tags manual ./internal/discovery/... -run TestManualFullDiscovery -v
package discovery

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

func TestManualFullDiscovery(t *testing.T) {
	host := os.Getenv("WGTUI_MANUAL_HOST")
	if host == "" {
		t.Skip("WGTUI_MANUAL_HOST not set; skipping manual integration test")
	}
	port, _ := strconv.Atoi(os.Getenv("WGTUI_MANUAL_PORT"))
	user := os.Getenv("WGTUI_MANUAL_USER")
	sudoPassFile := os.Getenv("WGTUI_MANUAL_SUDO_PASS_FILE")
	if sudoPassFile == "" {
		t.Skip("WGTUI_MANUAL_SUDO_PASS_FILE not set; skipping manual integration test")
	}
	sudoPassRaw, err := os.ReadFile(sudoPassFile)
	if err != nil {
		t.Fatalf("read sudo password file: %v", err)
	}
	sudoPass := strings.TrimRight(string(sudoPassRaw), "\r\n")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	controlDir := t.TempDir()
	client := sshtransport.NewClient(
		sshtransport.Target{HostOrAlias: host, Port: port, User: user},
		filepath.Join(controlDir, "cm.sock"),
	)
	prompts := sshtransport.Prompts{
		HostKey: func(prompt string) bool {
			t.Errorf("unexpected host key prompt (host should already be known): %s", prompt)
			return false
		},
		Password: func() (string, error) {
			t.Errorf("unexpected SSH password prompt; this target is expected to use key auth")
			return "", context.Canceled
		},
		SudoPassword: func() (string, error) { return sudoPass, nil },
	}

	if err := client.Dial(ctx, prompts); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close(ctx)

	report := Discover(ctx, client, prompts)
	state := Classify(report)

	t.Logf("state: %s", state)
	t.Logf("system: kernel=%q os=%s/%s pkgmgr=%s sudo=%v", report.System.Kernel, report.System.OSID, report.System.OSVersionID, report.System.PackageManager, report.System.SudoAvailable)
	t.Logf("wireguard: present=%v wg-quick=%v version=%q interfaces=%v", report.WireGuard.WgPresent, report.WireGuard.WgQuickPresent, report.WireGuard.WgVersion, report.WireGuard.ActiveInterfaces)
	for name, d := range report.WireGuard.Interfaces {
		t.Logf("  interface %s: active=%v config=%v saveconfig=%v drift=%v runtime_peers=%d persistent_peers=%d",
			name, d.Active, d.ConfigExists, d.SaveConfig, d.Drift, len(d.RuntimePeerPublicKeys), len(d.PersistentPeerPublicKeys))
	}
	t.Logf("network: outbound=%q ipv4_fwd=%v ipv6_fwd=%v firewall=%q udp_ports=%v",
		report.Network.DefaultOutboundInterface, report.Network.IPv4Forwarding, report.Network.IPv6Forwarding, report.Network.FirewallBackend, report.Network.ListeningUDPPorts)
	if len(report.Errors) > 0 {
		t.Logf("non-fatal errors: %v", report.Errors)
	}

	if !report.System.SudoAvailable {
		t.Fatal("expected sudo to be available with a correct password")
	}
	if !report.WireGuard.WgPresent {
		t.Fatal("expected WireGuard to be present on this server")
	}
	if state != StateActiveConsistent && state != StateDrift {
		t.Errorf("expected an active interface (consistent or drifted) on this server, got state %s", state)
	}
	if report.Network.DefaultOutboundInterface == "" {
		t.Error("expected a default outbound interface to be detected")
	}
}
