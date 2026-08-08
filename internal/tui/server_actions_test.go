package tui

import (
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

// TestAfterInstallContinuesStraightToConfigure locks in that installing
// WireGuard on an empty server walks the user directly into the configure
// wizard instead of stopping at Overview with a separate manual step -
// requested because treating install and configure as two disconnected
// actions on a server that has nothing configured yet isn't a real choice.
func TestAfterInstallContinuesStraightToConfigure(t *testing.T) {
	server := domain.Server{ID: "office", DisplayName: "Office"}
	m := testModel(t, server)
	m.screen, m.tab, m.selected = screenServerDetail, tabOverview, server
	m.report.WireGuard.WgPresent = false

	cmd := m.afterInstall("wireguard-tools v1.0.20210914")

	if !m.report.WireGuard.WgPresent {
		t.Fatal("afterInstall did not mark WireGuard as present")
	}
	if m.report.WireGuard.WgVersion != "wireguard-tools v1.0.20210914" {
		t.Fatalf("afterInstall did not record the installed version: %q", m.report.WireGuard.WgVersion)
	}
	if m.state != discovery.StateToolsNoInterfaces {
		t.Fatalf("afterInstall left state=%v, want tools_no_interfaces", m.state)
	}
	if cmd == nil {
		t.Fatal("afterInstall did not continue into configure-prep")
	}
	if m.statusMsg == "" || m.statusErr {
		t.Fatalf("afterInstall did not leave a success status: msg=%q err=%v", m.statusMsg, m.statusErr)
	}
}
