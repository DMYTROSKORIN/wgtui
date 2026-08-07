package tui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerops"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

func (m *model) rebuildPeerTable() {
	cols := []table.Column{
		{Title: "Name", Width: 18},
		{Title: "Interface", Width: 10},
		{Title: "VPN IP", Width: 15},
		{Title: "State", Width: 16},
		{Title: "Handshake", Width: 12},
		{Title: "RX/TX", Width: 16},
	}
	rows := make([]table.Row, 0, len(m.peers))
	for _, p := range m.peers {
		handshake := "never"
		if p.LatestHandshakeSeconds != nil {
			handshake = fmt.Sprintf("%ds ago", *p.LatestHandshakeSeconds)
		}
		rows = append(rows, table.Row{
			p.Name, orDefault(p.Interface, "-"), orDefault(p.VPNIP, "-"),
			p.State, handshake, fmt.Sprintf("%d/%d", p.RxBytes, p.TxBytes),
		})
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.Faint(true)
	styles.Selected = styles.Selected.Foreground(lipgloss.Color(colorHighlight)).Bold(true)

	m.peerTable = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(len(rows)+1, 12)),
		table.WithStyles(styles),
	)
}

func (m *model) viewPeersTab() string {
	switch {
	case !m.peersLoaded:
		return "Connecting and listing peers...\n"
	case m.peersErr != "":
		return dangerStyle.Render("Error: "+m.peersErr) + "\n"
	case len(m.peers) == 0:
		return dimStyle.Render("No peers yet. Press [a] to add one.") + "\n"
	default:
		return m.peerTable.View() + "\n" + dimStyle.Render("\"online\" is a heuristic based on handshake recency, not proof of an active connection.")
	}
}

// loadPeersCmd snapshots everything it needs from the vault synchronously
// (before the goroutine starts) rather than reading m.vault from inside
// it: the async closure runs concurrently with the Bubble Tea event loop,
// which could otherwise be mutating the same vault from an unrelated
// screen at the same time.
func (m *model) loadPeersCmd() tea.Cmd {
	s := m.selected
	localPeers := m.vault.ListPeers(s.ID)
	profileSet := make(map[string]bool, len(localPeers))
	for _, p := range localPeers {
		if _, ok := m.vault.GetProfile(s.ID, p.PublicKey); ok {
			profileSet[p.PublicKey] = true
		}
	}

	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		prompts := newTerminalPrompts(os.Stdout)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return peerListMsg{err: fmt.Sprintf("connect: %v", err)}
		}
		defer sess.close()

		if err := sess.client.SudoValidate(ctx, prompts); err != nil {
			return peerListMsg{err: fmt.Sprintf("sudo: %v", err)}
		}
		res, err := sess.client.RunSudo(ctx, "wg show all dump", prompts)
		if err != nil {
			return peerListMsg{err: fmt.Sprintf("wg show all dump: %v", err)}
		}
		if res.ExitCode != 0 {
			return peerListMsg{err: fmt.Sprintf("wg show all dump exited %d: %s", res.ExitCode, res.Stderr)}
		}
		dump, err := wgconfig.ParseDump(string(res.Stdout))
		if err != nil {
			return peerListMsg{err: fmt.Sprintf("parse wg dump: %v", err)}
		}

		peers := peerview.Reconcile(dump.Peers, localPeers, func(pk string) bool { return profileSet[pk] }, time.Now())
		return peerListMsg{peers: peers}
	})
}

// resolvePeerInterface prefers the interface recorded in a peer's own
// metadata over guessing, exactly like internal/cli/peer_mutate.go's
// helper of the same name (duplicated for the same reason tui doesn't
// import cli - see docs/ARCHITECTURE.md).
func resolvePeerInterface(ctx context.Context, sess *session, metaInterface, flagInterface string) (string, error) {
	if metaInterface != "" {
		return metaInterface, nil
	}
	return peerops.ResolveInterface(ctx, sess.client, flagInterface)
}

func (m *model) openActionMenu() (tea.Model, tea.Cmd) {
	row := m.peerTable.Cursor()
	if row < 0 || row >= len(m.peers) {
		return m, nil
	}
	m.selectedPeer = m.peers[row]
	m.clearStatus()

	if !m.selectedPeer.Managed {
		m.setStatus("Unmanaged peer (not created by wgtui) - no actions available.")
		return m, nil
	}

	var options []huh.Option[string]
	if m.selectedPeer.State == "disabled" {
		options = append(options, huh.NewOption("Enable", "enable"))
	} else {
		options = append(options, huh.NewOption("Disable", "disable"))
		options = append(options, huh.NewOption("Rotate keys", "rotate"))
	}
	options = append(options, huh.NewOption("Revoke", "revoke"))
	if m.selectedPeer.ProfileAvailable {
		options = append(options, huh.NewOption("Export profile", "export"))
	}

	var choice string
	m.actionMenu = huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().Title("Actions for " + m.selectedPeer.Name).Height(len(options) + 2).Options(options...).Value(&choice),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.actionMenuChoice = &choice
	m.screen = screenPeerActionMenu
	return m, m.actionMenu.Init()
}

func (m *model) updateActionMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.actionMenu.Update(msg)
	m.actionMenu = form.(*huh.Form)

	switch m.actionMenu.State {
	case huh.StateCompleted:
		m.screen = screenServerDetail
		switch *m.actionMenuChoice {
		case "disable":
			return m, m.startPeerAction(m.selectedPeer, "disable")
		case "enable":
			return m, m.startPeerAction(m.selectedPeer, "enable")
		case "rotate":
			return m, m.startPeerAction(m.selectedPeer, "rotate")
		case "revoke":
			return m, m.startPeerAction(m.selectedPeer, "revoke")
		case "export":
			return m.startExportForm()
		}
		return m, nil
	case huh.StateAborted:
		m.screen = screenServerDetail
		return m, nil
	}
	return m, cmd
}

func (m *model) viewActionMenu() string {
	return titleStyle.Render("wgtui") + "\n\n" + m.actionMenu.View()
}

// startPeerAction covers disable/enable/rotate/revoke, which all share the
// same connect -> fetch config -> show plan -> confirm -> apply shape
// internal/cli/peer_mutate.go uses (see that file for the CLI's version of
// each of these plan strings and peerops calls - kept identical here).
func (m *model) startPeerAction(peer peerview.Peer, action string) tea.Cmd {
	s := m.selected
	serverID := s.ID
	meta, ok := m.vault.GetPeerByName(serverID, peer.Name)
	if !ok {
		return func() tea.Msg { return errMsg{err: fmt.Errorf("peer %q not found", peer.Name)} }
	}

	if action == "revoke" {
		return m.startRevoke(s, meta)
	}

	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		prompts := newTerminalPrompts(os.Stdout)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return planReadyMsg{err: fmt.Errorf("connect: %w", err), cancelScreen: screenServerDetail}
		}

		iface, err := resolvePeerInterface(ctx, sess, meta.Interface, "")
		if err != nil {
			sess.close()
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		cc, err := peerops.FetchCurrentConfig(ctx, sess.client, prompts, iface)
		if err != nil {
			sess.close()
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}

		switch action {
		case "disable":
			return planReadyMsg{
				sess: sess, cancelScreen: screenServerDetail,
				title: "Disable " + meta.Name + "?",
				plan:  fmt.Sprintf("- peer %s (%s)\n  Removed from runtime and persistent config.\n  Local profile and metadata kept, so it can be resumed later.", meta.Name, peerview.ShortenKey(meta.PublicKey)),
				apply: m.applyDisable(serverID, meta, cc, iface),
			}
		case "enable":
			return planReadyMsg{
				sess: sess, cancelScreen: screenServerDetail,
				title: "Resume " + meta.Name + "?",
				plan:  fmt.Sprintf("+ peer %s (%s)\n  Restored to persistent config exactly as it was before disabling.", meta.Name, peerview.ShortenKey(meta.PublicKey)),
				apply: m.applyEnable(serverID, meta, cc, iface),
			}
		case "rotate":
			idx := cc.Doc.FindPeerByPublicKey(meta.PublicKey)
			if idx == -1 {
				sess.close()
				return planReadyMsg{err: fmt.Errorf("peer %s not found in the current config (is it disabled?)", peerview.ShortenKey(meta.PublicKey)), cancelScreen: screenServerDetail}
			}
			block := cc.Doc.Blocks[idx]
			allowedIPs := peerops.SplitTrim(block.Value("AllowedIPs"), ",")
			keepalive, _ := strconv.Atoi(block.Value("PersistentKeepalive"))
			withPSK := block.Value("PresharedKey") != ""
			return planReadyMsg{
				sess: sess, cancelScreen: screenServerDetail,
				title: "Rotate keys for " + meta.Name + "?",
				plan:  fmt.Sprintf("Rotate %s: a fresh keypair replaces %s on the server.\nThe old local profile is removed.", meta.Name, peerview.ShortenKey(meta.PublicKey)),
				apply: m.applyRotate(serverID, meta, cc, iface, allowedIPs, keepalive, withPSK),
			}
		}
		sess.close()
		return errMsg{err: fmt.Errorf("unknown action %q", action)}
	})
}

func (m *model) applyDisable(serverID string, meta profilevault.PeerMetadata, cc peerops.CurrentConfig, iface string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		return m.releaseTerminalAndRun(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			outcome, rawBlock, err := peerops.DisablePeer(ctx, sess.client, sess.prompts, cc, meta.PublicKey)
			if err != nil {
				return actionDoneMsg{err: err}
			}
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				meta.Status = profilevault.PeerStatusDisabled
				meta.DisabledBlockRaw = rawBlock
				meta.Interface = iface
				m.vault.UpdatePeer(meta)
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("applied on the server (backup: %s) but failed to save locally: %w", outcome.BackupPath, err))
					return nil
				}
				m.setStatus("Disabled " + meta.Name + ". Backup: " + outcome.BackupPath)
				m.peersLoaded = false
				return nil
			}}
		})
	}
}

func (m *model) applyEnable(serverID string, meta profilevault.PeerMetadata, cc peerops.CurrentConfig, iface string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		return m.releaseTerminalAndRun(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			outcome, err := peerops.EnablePeer(ctx, sess.client, sess.prompts, cc, meta.DisabledBlockRaw)
			if err != nil {
				return actionDoneMsg{err: err}
			}
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				meta.Status = profilevault.PeerStatusActive
				meta.DisabledBlockRaw = ""
				meta.Interface = iface
				m.vault.UpdatePeer(meta)
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("applied on the server (backup: %s) but failed to save locally: %w", outcome.BackupPath, err))
					return nil
				}
				m.setStatus("Resumed " + meta.Name + ". Backup: " + outcome.BackupPath)
				m.peersLoaded = false
				return nil
			}}
		})
	}
}

func (m *model) applyRotate(serverID string, meta profilevault.PeerMetadata, cc peerops.CurrentConfig, iface string, allowedIPs []string, keepalive int, withPSK bool) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		oldProfile, hadProfile := m.vault.GetProfile(serverID, meta.PublicKey)
		host := m.selected.SSHHostOrAlias
		return m.releaseTerminalAndRun(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			result, err := peerops.RotatePeer(ctx, sess.client, sess.prompts, cc, meta.PublicKey, meta.Name, allowedIPs, keepalive, withPSK)
			if err != nil {
				return actionDoneMsg{err: err}
			}
			clientAddr, endpointHost, endpointPort, clientAllowedIPs, dns := peerops.RotateProfileParams(oldProfile, hadProfile, allowedIPs, host, cc.ListenPort)
			newConf := peerops.BuildClientProfile(peerops.ClientProfileParams{
				ClientPrivateKey: result.KeyPair.PrivateKey, ClientAddress: clientAddr, DNS: dns,
				ServerPublicKey: cc.ServerPublicKey, PresharedKey: result.PresharedKey,
				EndpointHost: endpointHost, EndpointPort: endpointPort, AllowedIPs: clientAllowedIPs, PersistentKeepalive: keepalive,
			})
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				oldKey := meta.PublicKey
				m.vault.DeleteProfile(serverID, oldKey)
				m.vault.SetProfile(serverID, result.KeyPair.PublicKey, newConf)
				m.vault.RemovePeer(serverID, oldKey)
				meta.PublicKey = result.KeyPair.PublicKey
				meta.Interface = iface
				_ = m.vault.AddPeer(meta)
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("applied on the server (backup: %s) but failed to save locally: %w", result.Outcome.BackupPath, err))
					return nil
				}
				m.setStatus(fmt.Sprintf("Rotated %s: %s -> %s. Backup: %s", meta.Name, peerview.ShortenKey(oldKey), peerview.ShortenKey(result.KeyPair.PublicKey), result.Outcome.BackupPath))
				m.peersLoaded = false
				return nil
			}}
		})
	}
}

// startRevoke keeps the CLI's exact three-question shape (SPEC.md §17.5):
// revoke server access, delete local profile, delete local metadata are
// three independent choices. It only connects if the peer isn't already
// disabled, exactly like internal/cli/peer_mutate.go's runPeerRevoke.
func (m *model) startRevoke(s domain.Server, meta profilevault.PeerMetadata) tea.Cmd {
	if meta.Status == profilevault.PeerStatusDisabled {
		return func() tea.Msg {
			return planReadyMsg{
				cancelScreen: screenServerDetail,
				title:        "Revoke " + meta.Name,
				multiStep:    true,
				apply:        m.applyRevoke(s.ID, meta, nil, peerops.CurrentConfig{}),
			}
		}
	}
	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		prompts := newTerminalPrompts(os.Stdout)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return planReadyMsg{err: fmt.Errorf("connect: %w", err), cancelScreen: screenServerDetail}
		}
		iface, err := resolvePeerInterface(ctx, sess, meta.Interface, "")
		if err != nil {
			sess.close()
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		cc, err := peerops.FetchCurrentConfig(ctx, sess.client, prompts, iface)
		if err != nil {
			sess.close()
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		return planReadyMsg{
			sess: sess, cancelScreen: screenServerDetail,
			title:     "Revoke " + meta.Name,
			multiStep: true,
			apply:     m.applyRevoke(s.ID, meta, sess, cc),
		}
	})
}

func (m *model) applyRevoke(serverID string, meta profilevault.PeerMetadata, sess *session, cc peerops.CurrentConfig) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		deleteProfile := m.revokeDeleteProfile
		deleteMetadata := m.revokeDeleteMetadata
		revokeAccess := m.revokeYes
		return m.releaseTerminalAndRun(func() tea.Msg {
			var backupPath string
			if revokeAccess && sess != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				outcome, err := peerops.RevokePeer(ctx, sess.client, sess.prompts, cc, meta.PublicKey)
				if err != nil {
					return actionDoneMsg{err: err}
				}
				backupPath = outcome.BackupPath
			}
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				if deleteProfile {
					m.vault.DeleteProfile(serverID, meta.PublicKey)
				}
				if deleteMetadata {
					m.vault.RemovePeer(serverID, meta.PublicKey)
				} else {
					meta.Status = profilevault.PeerStatusRevoked
					meta.DisabledBlockRaw = ""
					m.vault.UpdatePeer(meta)
				}
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("change applied but failed to save locally: %w", err))
					return nil
				}
				m.setStatus(fmt.Sprintf("Revoked %s. profile_deleted=%v metadata_deleted=%v. Backup: %s", meta.Name, deleteProfile, deleteMetadata, backupPath))
				m.peersLoaded = false
				return nil
			}}
		})
	}
}

func newRevokeForm(m *model) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(huh.NewConfirm().
			Title("1/3 Revoke server access for "+m.selectedPeer.Name+"?").
			Affirmative("Revoke").Negative("Skip").Value(&m.revokeYes)),
		huh.NewGroup(huh.NewConfirm().
			Title("2/3 Also delete the local client profile?").
			Description("This contains the private key.").
			Affirmative("Delete").Negative("Keep").Value(&m.revokeDeleteProfile)),
		huh.NewGroup(huh.NewConfirm().
			Title("3/3 Also delete the local metadata entry?").
			Affirmative("Delete").Negative("Keep").Value(&m.revokeDeleteMetadata)),
	).WithTheme(formTheme()).WithKeyMap(formKeyMap())
}

func (m *model) updateRevokeConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.revokeForm.Update(msg)
	m.revokeForm = form.(*huh.Form)

	switch m.revokeForm.State {
	case huh.StateCompleted:
		// See updateConfirm's identical comment: must leave
		// screenRevokeConfirm before running onConfirmed, or the next
		// stray tea.Msg re-enters this branch with an already-closed
		// session and panics.
		m.screen = screenApplying
		return m, m.onConfirmed(m)
	case huh.StateAborted:
		m.activeSession.close()
		m.activeSession = nil
		m.screen = m.onCancelled
		return m, nil
	}
	return m, cmd
}

func (m *model) viewRevokeConfirm() string {
	return titleStyle.Render("wgtui") + "  " + subtitleStyle.Render(m.confirmTitle) + "\n\n" + m.revokeForm.View()
}

func (m *model) startExportForm() (tea.Model, tea.Cmd) {
	m.exportPath = ""
	m.exportForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Export " + m.selectedPeer.Name + " to").
			Description("File path to write the client profile to.").
			Value(&m.exportPath).
			Validate(func(s string) error {
				if s == "" {
					return errRequired
				}
				return nil
			}),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenPeerExportForm
	return m, m.exportForm.Init()
}

func (m *model) updateExportForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.exportForm.Update(msg)
	m.exportForm = form.(*huh.Form)

	switch m.exportForm.State {
	case huh.StateCompleted:
		m.screen = screenServerDetail
		m.exportPeer(m.exportPath)
		return m, nil
	case huh.StateAborted:
		m.screen = screenServerDetail
		return m, nil
	}
	return m, cmd
}

func (m *model) viewExportForm() string {
	return titleStyle.Render("wgtui") + "\n\n" + m.exportForm.View()
}

func (m *model) exportPeer(path string) {
	meta, ok := m.vault.GetPeerByName(m.selected.ID, m.selectedPeer.Name)
	if !ok {
		m.setError(fmt.Errorf("peer %q not found", m.selectedPeer.Name))
		return
	}
	profile, ok := m.vault.GetProfile(m.selected.ID, meta.PublicKey)
	if !ok {
		m.setError(fmt.Errorf("no local profile stored for %q", meta.Name))
		return
	}
	if _, err := os.Stat(path); err == nil {
		m.setError(fmt.Errorf("%s already exists", path))
		return
	}
	if err := os.WriteFile(path, []byte(profile), 0o600); err != nil {
		m.setError(err)
		return
	}
	m.setStatus("Exported " + meta.Name + " to " + path)
}
