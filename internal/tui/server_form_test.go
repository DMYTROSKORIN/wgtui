package tui

import (
	"testing"

	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

func TestAddKeyOpensServerForm(t *testing.T) {
	m := testModel(t)
	res, _ := m.Update(key("a"))
	rm := res.(*model)
	if rm.screen != screenServerForm {
		t.Errorf("screen = %v, want screenServerForm", rm.screen)
	}
	if rm.serverEdit {
		t.Error("expected serverEdit=false for 'a'")
	}
}

func TestEditKeyPrefillsServerForm(t *testing.T) {
	m := testModel(t, domain.Server{
		ID: "office", DisplayName: "Office", SSHHostOrAlias: "1.2.3.4",
		SSHUser: "admin", SSHPort: 2222, SSHIdentityFile: "/home/x/.ssh/id_ed25519",
	})
	res, _ := m.Update(key("e"))
	rm := res.(*model)
	if rm.screen != screenServerForm || !rm.serverEdit {
		t.Fatalf("screen=%v serverEdit=%v, want screenServerForm/true", rm.screen, rm.serverEdit)
	}
	if rm.fID != "office" || rm.fName != "Office" || rm.fHost != "1.2.3.4" ||
		rm.fUser != "admin" || rm.fPort != "2222" || rm.fIdentity != "/home/x/.ssh/id_ed25519" {
		t.Errorf("form not correctly pre-filled: %+v", rm)
	}
}

func TestEditKeyWithNoServersIsSafe(t *testing.T) {
	m := testModel(t)
	res, _ := m.Update(key("e"))
	if res.(*model).screen != screenServerList {
		t.Error("'e' with no servers should not change screen")
	}
}

func TestRemoveKeyOpensConfirm(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	res, _ := m.Update(key("x"))
	rm := res.(*model)
	if rm.screen != screenServerRemoveConfirm {
		t.Errorf("screen = %v, want screenServerRemoveConfirm", rm.screen)
	}
	if rm.selected.ID != "office" {
		t.Errorf("selected.ID = %q, want office", rm.selected.ID)
	}
}

func TestRemoveKeyWithNoServersIsSafe(t *testing.T) {
	m := testModel(t)
	res, _ := m.Update(key("x"))
	if res.(*model).screen != screenServerList {
		t.Error("'x' with no servers should not change screen")
	}
}

func TestRemoveConfirm_YesRemovesServer(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.startRemoveConfirm(m.vault.Registry.Servers[0])
	m.confirmYes = true
	m.removeForm.State = huh.StateCompleted

	res, _ := m.updateRemoveConfirm(nil)
	rm := res.(*model)
	if rm.screen != screenServerList {
		t.Errorf("screen = %v, want screenServerList", rm.screen)
	}
	if len(rm.vault.Registry.Servers) != 0 {
		t.Errorf("expected server removed, still have %d", len(rm.vault.Registry.Servers))
	}
}

func TestRemoveConfirm_NoKeepsServer(t *testing.T) {
	m := testModel(t, domain.Server{ID: "office", DisplayName: "Office"})
	m.startRemoveConfirm(m.vault.Registry.Servers[0])
	m.confirmYes = false
	m.removeForm.State = huh.StateCompleted

	res, _ := m.updateRemoveConfirm(nil)
	rm := res.(*model)
	if len(rm.vault.Registry.Servers) != 1 {
		t.Errorf("expected server kept, have %d", len(rm.vault.Registry.Servers))
	}
}
