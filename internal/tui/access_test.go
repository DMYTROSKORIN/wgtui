package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func TestVaultUnlockHappensInsideBrandedTUI(t *testing.T) {
	path := t.TempDir() + "/vault.json"
	vault, err := profilevault.New("correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := vault.Save(path); err != nil {
		t.Fatal(err)
	}
	m := newLockedModel(path, false)
	view := ansi.Strip(m.View())
	for _, want := range []string{"wgtui", "Unlock local vault", "Master password", "never sent to a", "server"} {
		if !strings.Contains(view, want) {
			t.Errorf("vault screen lacks %q:\n%s", want, view)
		}
	}
	m.vaultPassword = "correct-password"
	m.vaultForm.State = huh.StateCompleted
	m.updateVaultAccess(nil)
	if m.screen != screenServerList || m.vault == nil || m.vaultPassword != "" {
		t.Fatalf("unlock did not enter app and clear secret: screen=%v vault=%v password=%q", m.screen, m.vault != nil, m.vaultPassword)
	}
}

func TestAccessAndCredentialScreensFitEveryLayout(t *testing.T) {
	for _, size := range [][2]int{{64, 18}, {90, 24}, {135, 24}, {204, 40}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			locked := newLockedModel(t.TempDir()+"/vault.json", true)
			locked.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			assertViewFits(t, locked.View(), size[0], size[1])

			m := testModel(t)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			m.beginCredentialPrompt(credentialRequestMsg{kind: credentialSSHPassword, target: "Lab server (skorin@192.168.122.245)", response: make(chan credentialResponse, 1)})
			assertViewFits(t, m.View(), size[0], size[1])
		})
	}
}

func TestWrongVaultPasswordStaysInTUIAndClearsSecret(t *testing.T) {
	path := t.TempDir() + "/vault.json"
	vault, _ := profilevault.New("correct-password")
	if err := vault.Save(path); err != nil {
		t.Fatal(err)
	}
	m := newLockedModel(path, false)
	m.vaultPassword = "wrong-password"
	m.vaultForm.State = huh.StateCompleted
	m.updateVaultAccess(nil)
	if m.screen != screenVaultAccess || m.vault != nil || m.vaultPassword != "" {
		t.Fatalf("wrong password escaped access screen or remained in memory: screen=%v vault=%v password=%q", m.screen, m.vault != nil, m.vaultPassword)
	}
	if !strings.Contains(ansi.Strip(m.View()), "did not unlock") {
		t.Fatal("wrong password lacks an in-app explanation")
	}
}

func TestRemotePasswordPromptIsModalNamedAndClearsValue(t *testing.T) {
	m := testModel(t)
	m.screen = screenServerDetail
	response := make(chan credentialResponse, 1)
	m.beginCredentialPrompt(credentialRequestMsg{kind: credentialSudoPassword, target: "Lab (skorin@192.168.122.245)", response: response})
	view := ansi.Strip(m.View())
	for _, want := range []string{"Privilege required", "sudo password", "Lab (skorin@192.168.122.245)", "remote server", "only in memory"} {
		if !strings.Contains(view, want) {
			t.Errorf("credential modal lacks %q:\n%s", want, view)
		}
	}
	m.credentialValue = "temporary-secret"
	m.credentialForm.State = huh.StateCompleted
	m.updateCredentialPrompt(nil)
	answer := <-response
	if answer.value != "temporary-secret" || answer.err != nil {
		t.Fatalf("credential response=%+v", answer)
	}
	if m.screen != screenServerDetail || m.credentialValue != "" || m.credentialRequest != nil {
		t.Fatalf("credential modal did not restore/clear: screen=%v value=%q request=%v", m.screen, m.credentialValue, m.credentialRequest)
	}
}

func TestCredentialCancelUnblocksRemoteWork(t *testing.T) {
	m := testModel(t)
	response := make(chan credentialResponse, 1)
	m.beginCredentialPrompt(credentialRequestMsg{kind: credentialSSHPassword, target: "Lab", response: response})
	m.credentialForm.State = huh.StateAborted
	m.updateCredentialPrompt(nil)
	if answer := <-response; answer.err == nil || answer.value != "" {
		t.Fatalf("cancel response=%+v, want error and no secret", answer)
	}
}

func TestHostKeyPromptRequiresExplicitTrustAndRestoresScreen(t *testing.T) {
	for _, accepted := range []bool{false, true} {
		m := testModel(t)
		m.screen = screenServerDetail
		response := make(chan credentialResponse, 1)
		m.beginCredentialPrompt(credentialRequestMsg{
			kind: credentialHostKey, target: "Lab server", detail: "SHA256:test-fingerprint", response: response,
		})
		view := ansi.Strip(m.View())
		for _, want := range []string{"First connection", "SSH host key", "SHA256:test-fingerprint", "Trust and connect"} {
			if !strings.Contains(view, want) {
				t.Errorf("host-key modal lacks %q:\n%s", want, view)
			}
		}
		m.credentialAccepted = accepted
		m.credentialForm.State = huh.StateCompleted
		m.updateCredentialPrompt(nil)
		answer := <-response
		if answer.accepted != accepted || (accepted && answer.err != nil) || (!accepted && answer.err == nil) {
			t.Fatalf("accepted=%v response=%+v", accepted, answer)
		}
		if m.screen != screenServerDetail || m.credentialRequest != nil {
			t.Fatalf("host-key modal did not restore screen: screen=%v request=%v", m.screen, m.credentialRequest)
		}
	}
}
