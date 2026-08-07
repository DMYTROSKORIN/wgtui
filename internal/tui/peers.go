package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerops"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

func (m *model) rebuildPeerTable() {
	available := m.peerTableAvailableWidth()
	wideColumns := available >= 95
	var cols []table.Column
	if wideColumns {
		budget := available - 6
		nameWidth := max(14, budget*16/100)
		vpnWidth := max(14, budget*15/100)
		stateWidth := max(12, budget*13/100)
		seenWidth := max(12, budget*14/100)
		endpointWidth := max(18, budget*22/100)
		trafficWidth := max(19, budget-nameWidth-vpnWidth-stateWidth-seenWidth-endpointWidth)
		cols = []table.Column{
			{Title: "Peer", Width: nameWidth},
			{Title: "VPN IP", Width: vpnWidth},
			{Title: "State", Width: stateWidth},
			{Title: "Last seen", Width: seenWidth},
			{Title: "Endpoint", Width: endpointWidth},
			{Title: "Traffic ↓ / ↑", Width: trafficWidth},
		}
	} else {
		budget := max(60, available-5)
		nameWidth := max(12, budget*19/100)
		vpnWidth := max(14, budget*20/100)
		stateWidth := max(12, budget*17/100)
		seenWidth := max(12, budget*16/100)
		trafficWidth := max(19, budget-nameWidth-vpnWidth-stateWidth-seenWidth)
		cols = []table.Column{
			{Title: "Peer", Width: nameWidth},
			{Title: "VPN IP", Width: vpnWidth},
			{Title: "State", Width: stateWidth},
			{Title: "Last seen", Width: seenWidth},
			{Title: "Traffic ↓ / ↑", Width: trafficWidth},
		}
	}
	rows := make([]table.Row, 0, len(m.peers))
	for _, p := range m.peers {
		row := table.Row{
			terminalText(p.Name), orDefault(terminalText(p.VPNIP), "-"), stateLabel(p), formatHandshakeAge(p.LatestHandshakeSeconds),
		}
		if wideColumns {
			row = append(row, orDefault(terminalText(p.Endpoint), "-"))
		}
		row = append(row, fmt.Sprintf("%s / %s", formatBytes(p.RxBytes), formatBytes(p.TxBytes)))
		rows = append(rows, row)
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.Foreground(colorMuted).Bold(true)
	styles.Selected = styles.Selected.Foreground(colorHighlight).Bold(true)

	tableHeight := min(max(3, len(rows)+1), max(5, m.height-16))
	if m.width >= 160 {
		tableHeight = max(5, m.height-12)
	}
	m.peerTable = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(tableHeight),
		table.WithStyles(styles),
	)
}

// terminalText removes control characters from server- and user-derived
// values before rendering. In particular, an endpoint or imported peer name
// containing ESC must not be able to inject terminal commands or recolor the
// surrounding UI.
func terminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

func (m *model) peerTableAvailableWidth() int {
	if m.width >= 160 {
		_, peersWidth, _ := widePeerPanelWidths(m.width)
		return max(60, peersWidth-4)
	}
	if m.width >= 100 {
		return max(60, m.width-clamp(m.width/4, 22, 30)-9)
	}
	return max(60, m.width-8)
}

func widePeerPanelWidths(total int) (servers, peers, inspector int) {
	servers = clamp(total/5, 24, 30)
	inspector = clamp(total/4, 40, 54)
	peers = max(60, total-servers-inspector-8)
	return
}

func (m *model) viewPeersTab() string {
	list := m.viewPeerList()
	if m.peersLoaded && m.peersErr == "" && len(m.peers) > 0 {
		list += "\n\n" + m.viewSelectedPeerCard()
	}
	return list
}

func (m *model) viewPeerList() string {
	switch {
	case !m.peersLoaded:
		return warningStyle.Render("↻ Loading peers from the server…") + "\n"
	case m.peersErr != "":
		return dangerStyle.Render("Error: "+m.peersErr) + "\n"
	case len(m.peers) == 0:
		return dimStyle.Render("No peers yet. Press n to add one.") + "\n"
	default:
		activity := ""
		if m.peersRefreshing {
			activity = warningStyle.Render("↻ Refreshing peer state…") + "\n\n"
		}
		return activity + m.peerTable.View()
	}
}

func formatHandshakeAge(seconds *int64) string {
	if seconds == nil {
		return "never"
	}
	s := max(int(*seconds), 0)
	switch {
	case s < 60:
		return fmt.Sprintf("%ds ago", s)
	case s < 3600:
		return fmt.Sprintf("%dm ago", s/60)
	case s < 48*3600:
		return fmt.Sprintf("%dh %dm ago", s/3600, (s%3600)/60)
	default:
		return fmt.Sprintf("%dd %dh ago", s/(24*3600), (s%(24*3600))/3600)
	}
}

func stateLabel(p peerview.Peer) string {
	switch p.State {
	case "online":
		return "● online"
	case "disabled":
		return "◼ disabled"
	case "drifted", "error":
		return "! " + p.State
	default:
		return "○ " + strings.ReplaceAll(p.State, "_", " ")
	}
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit && exp < 3; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}

func (m *model) viewSelectedPeerCard() string {
	row := m.peerTable.Cursor()
	if row < 0 || row >= len(m.peers) {
		return ""
	}
	p := m.peers[row]
	managed := "unmanaged (read-only)"
	if p.Managed {
		managed = "managed by wgtui"
	}
	profile := "not available"
	if p.ProfileAvailable {
		profile = "stored securely in local vault"
	}
	return titleStyle.Render("Selected peer") + "\n" +
		fmt.Sprintf("%s  %s\nInterface: %s   Public key: %s\nProfile: %s",
			selectedStyle.Render(terminalText(p.Name)), peerStateBadge(p.State), orDefault(terminalText(p.Interface), "-"), terminalText(peerview.ShortenKey(p.PublicKey)), profile) +
		"\n" + dimStyle.Render(managed+" · Enter opens available actions")
}

func (m *model) viewSelectedPeerInspector() string {
	row := m.peerTable.Cursor()
	if !m.peersLoaded {
		return accentHeadingStyle.Render("Selected peer") + "\n\n" + dimStyle.Render("Waiting for peer data…")
	}
	if m.peersErr != "" {
		return accentHeadingStyle.Render("Selected peer") + "\n\n" + dangerStyle.Render("Peer data unavailable")
	}
	if row < 0 || row >= len(m.peers) {
		return accentHeadingStyle.Render("Selected peer") + "\n\n" + dimStyle.Render("No peer selected")
	}
	p := m.peers[row]
	profile := "Not available"
	if p.ProfileAvailable {
		profile = "Encrypted in local vault"
	}
	ownership := "Unmanaged · read-only"
	if p.Managed {
		ownership = "Managed by wgtui"
	}
	psk := "No"
	if p.State == "disabled" {
		psk = "Not active"
	} else if p.PresharedKeyPresent {
		psk = "Yes · secret hidden"
	}
	return accentHeadingStyle.Render("Selected peer") + "\n\n" +
		selectedStyle.Render(terminalText(p.Name)) + "  " + peerStateBadge(p.State) + "\n\n" +
		fmt.Sprintf("VPN address    %s\nInterface      %s\nEndpoint       %s\nLast handshake %s\nTraffic ↓ / ↑  %s / %s\n\nPublic key\n%s\nPreshared key  %s\nClient profile %s\n%s\n\n",
			orDefault(terminalText(p.VPNIP), "-"), orDefault(terminalText(p.Interface), "-"), orDefault(terminalText(p.Endpoint), "Not observed"),
			formatHandshakeAge(p.LatestHandshakeSeconds), formatBytes(p.RxBytes), formatBytes(p.TxBytes),
			terminalText(p.PublicKey), psk, profile, ownership) +
		dimStyle.Render("Enter opens actions · r refreshes")
}

// loadPeersFromSessionCmd refreshes the peer table over the connection
// opened for the server detail screen. Discovery has already collected
// and cached any SSH/sudo credentials in this session, so changing tabs
// never drops out of the TUI to ask for another password.
func (m *model) loadPeersFromSessionCmd() tea.Cmd {
	m.peersRefreshing = true
	s := m.selected
	sess := m.detailSession
	if sess == nil || sess.closed {
		return func() tea.Msg {
			return peerListMsg{err: "server session is not connected; press r on Overview to reconnect"}
		}
	}
	localPeers := m.vault.ListPeers(s.ID)
	profileSet := make(map[string]bool, len(localPeers))
	for _, p := range localPeers {
		if _, ok := m.vault.GetProfile(s.ID, p.PublicKey); ok {
			profileSet[p.PublicKey] = true
		}
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		res, err := sess.client.RunSudo(ctx, "wg show all dump", sess.prompts)
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
		return peerListMsg{peers: peerview.Reconcile(dump.Peers, localPeers, func(pk string) bool { return profileSet[pk] }, time.Now())}
	}
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

type peerActionOption struct {
	label string
	value string
	risk  bool
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

	var options []peerActionOption
	if m.selectedPeer.State == "disabled" {
		options = append(options, peerActionOption{"Resume access", "enable", false})
	} else {
		options = append(options, peerActionOption{"Pause access (profile is kept)", "disable", false})
		options = append(options, peerActionOption{"Rotate keys and replace client profile", "rotate", false})
	}
	if m.selectedPeer.ProfileAvailable {
		options = append(options,
			peerActionOption{"Show QR code", "qr", false},
			peerActionOption{"Save client profile to home directory", "export", false},
		)
	}
	options = append(options, peerActionOption{"Revoke access and optionally delete local data", "revoke", true})
	m.actionOptions = options
	m.actionCursor = 0
	m.screen = screenPeerActionMenu
	return m, nil
}

func (m *model) updateActionMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		if m.actionCursor > 0 {
			m.actionCursor--
		}
	case "down", "j":
		if m.actionCursor < len(m.actionOptions)-1 {
			m.actionCursor++
		}
	case "enter":
		if len(m.actionOptions) == 0 {
			return m, nil
		}
		m.screen = screenServerDetail
		switch m.actionOptions[m.actionCursor].value {
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
		case "qr":
			m.showStoredProfile("qr")
			return m, nil
		}
	case "esc", "q":
		m.screen = screenServerDetail
	}
	return m, nil
}

func (m *model) viewActionMenu() string {
	p := m.selectedPeer
	var menu strings.Builder
	menu.WriteString(accentHeadingStyle.Render("Actions for "+terminalText(p.Name)) + "\n\n")
	for i, option := range m.actionOptions {
		label := "  " + option.label
		if i == m.actionCursor {
			label = "▶ " + option.label
			if option.risk {
				menu.WriteString(dangerSelectedStyle.Render(label))
			} else {
				menu.WriteString(menuSelectedStyle.Render(label))
			}
		} else if option.risk {
			menu.WriteString(dangerStyle.Render(label))
		} else {
			menu.WriteString(textStyle.Render(label))
		}
		menu.WriteString("\n")
	}
	card := accentHeadingStyle.Render("Peer") + "\n\n" +
		fmt.Sprintf("%s  %s\nVPN IP: %s   Interface: %s\nPublic key: %s\n\n",
			selectedStyle.Render(terminalText(p.Name)), peerStateBadge(p.State), orDefault(terminalText(p.VPNIP), "-"),
			orDefault(terminalText(p.Interface), "-"), terminalText(peerview.ShortenKey(p.PublicKey))) +
		menu.String()
	return appHeader(m.selected.DisplayName+" / Peers") + "\n" +
		fullPanel(m, card) + "\n" +
		keyBar("↑/↓ Select", "Enter Continue", "Esc Back", "Ctrl+C Quit")
}

func (m *model) showStoredProfile(mode string) {
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
	m.revealName, m.revealProfile, m.revealMode = meta.Name, profile, mode
	m.screen = screenPeerReveal
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

	sess := m.detailSession
	if sess == nil || sess.closed {
		return func() tea.Msg {
			return planReadyMsg{err: fmt.Errorf("server session is not connected; return to Overview and press r"), cancelScreen: screenServerDetail}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		iface, err := resolvePeerInterface(ctx, sess, meta.Interface, "")
		if err != nil {
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		cc, err := peerops.FetchCurrentConfig(ctx, sess.client, sess.prompts, iface)
		if err != nil {
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}

		switch action {
		case "disable":
			return planReadyMsg{
				sess: sess, borrowed: true, cancelScreen: screenServerDetail,
				title: "Disable " + meta.Name + "?",
				plan:  fmt.Sprintf("- peer %s (%s)\n  Removed from runtime and persistent config.\n  Local profile and metadata kept, so it can be resumed later.", meta.Name, peerview.ShortenKey(meta.PublicKey)),
				apply: m.applyDisable(serverID, meta, cc, iface),
			}
		case "enable":
			return planReadyMsg{
				sess: sess, borrowed: true, cancelScreen: screenServerDetail,
				title: "Resume " + meta.Name + "?",
				plan:  fmt.Sprintf("+ peer %s (%s)\n  Restored to persistent config exactly as it was before disabling.", meta.Name, peerview.ShortenKey(meta.PublicKey)),
				apply: m.applyEnable(serverID, meta, cc, iface),
			}
		case "rotate":
			idx := cc.Doc.FindPeerByPublicKey(meta.PublicKey)
			if idx == -1 {
				return planReadyMsg{err: fmt.Errorf("peer %s not found in the current config (is it disabled?)", peerview.ShortenKey(meta.PublicKey)), cancelScreen: screenServerDetail}
			}
			block := cc.Doc.Blocks[idx]
			allowedIPs := peerops.SplitTrim(block.Value("AllowedIPs"), ",")
			keepalive, _ := strconv.Atoi(block.Value("PersistentKeepalive"))
			withPSK := block.Value("PresharedKey") != ""
			return planReadyMsg{
				sess: sess, borrowed: true, cancelScreen: screenServerDetail,
				title: "Rotate keys for " + meta.Name + "?",
				plan:  fmt.Sprintf("Rotate %s: a fresh keypair replaces %s on the server.\nThe old local profile is removed.", meta.Name, peerview.ShortenKey(meta.PublicKey)),
				apply: m.applyRotate(serverID, meta, cc, iface, allowedIPs, keepalive, withPSK),
			}
		}
		return errMsg{err: fmt.Errorf("unknown action %q", action)}
	}
}

func (m *model) applyDisable(serverID string, meta profilevault.PeerMetadata, cc peerops.CurrentConfig, iface string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			_, rawBlock, err := peerops.DisablePeer(ctx, sess.client, sess.prompts, cc, meta.PublicKey)
			if err != nil {
				return actionDoneMsg{err: err}
			}
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				meta.Status = profilevault.PeerStatusDisabled
				meta.DisabledBlockRaw = rawBlock
				meta.Interface = iface
				m.vault.UpdatePeer(meta)
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("access was paused on the server, but local metadata could not be saved: %w", err))
					return nil
				}
				m.setStatus("Paused " + meta.Name + ". Server access removed; local profile kept.")
				return m.loadPeersFromSessionCmd()
			}}
		}
	}
}

func (m *model) applyEnable(serverID string, meta profilevault.PeerMetadata, cc peerops.CurrentConfig, iface string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			_, err := peerops.EnablePeer(ctx, sess.client, sess.prompts, cc, meta.DisabledBlockRaw)
			if err != nil {
				return actionDoneMsg{err: err}
			}
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				meta.Status = profilevault.PeerStatusActive
				meta.DisabledBlockRaw = ""
				meta.Interface = iface
				m.vault.UpdatePeer(meta)
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("access was resumed on the server, but local metadata could not be saved: %w", err))
					return nil
				}
				m.setStatus("Resumed " + meta.Name + ". Server access restored.")
				return m.loadPeersFromSessionCmd()
			}}
		}
	}
}

func (m *model) applyRotate(serverID string, meta profilevault.PeerMetadata, cc peerops.CurrentConfig, iface string, allowedIPs []string, keepalive int, withPSK bool) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		oldProfile, hadProfile := m.vault.GetProfile(serverID, meta.PublicKey)
		host := m.selected.SSHHostOrAlias
		return func() tea.Msg {
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
					m.setError(fmt.Errorf("keys were rotated on the server, but the local profile could not be saved: %w", err))
					return nil
				}
				m.setStatus(fmt.Sprintf("Rotated %s: %s → %s. New local profile saved.", meta.Name, peerview.ShortenKey(oldKey), peerview.ShortenKey(result.KeyPair.PublicKey)))
				return m.loadPeersFromSessionCmd()
			}}
		}
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
	sess := m.detailSession
	if sess == nil || sess.closed {
		return func() tea.Msg {
			return planReadyMsg{err: fmt.Errorf("server session is not connected; return to Overview and press r"), cancelScreen: screenServerDetail}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		iface, err := resolvePeerInterface(ctx, sess, meta.Interface, "")
		if err != nil {
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		cc, err := peerops.FetchCurrentConfig(ctx, sess.client, sess.prompts, iface)
		if err != nil {
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		return planReadyMsg{
			sess: sess, borrowed: true, cancelScreen: screenServerDetail,
			title:     "Revoke " + meta.Name,
			multiStep: true,
			apply:     m.applyRevoke(s.ID, meta, sess, cc),
		}
	}
}

func (m *model) applyRevoke(serverID string, meta profilevault.PeerMetadata, sess *session, cc peerops.CurrentConfig) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		deleteProfile := m.revokeDeleteProfile
		deleteMetadata := m.revokeDeleteMetadata
		revokeAccess := m.revokeYes
		return func() tea.Msg {
			if revokeAccess && sess != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				_, err := peerops.RevokePeer(ctx, sess.client, sess.prompts, cc, meta.PublicKey)
				if err != nil {
					return actionDoneMsg{err: err}
				}
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
				m.setStatus(fmt.Sprintf("Revoked %s. Server access: removed · Local profile: %s · Metadata: %s.", meta.Name, deletedLabel(deleteProfile), deletedLabel(deleteMetadata)))
				return m.loadPeersFromSessionCmd()
			}}
		}
	}
}

func deletedLabel(deleted bool) string {
	if deleted {
		return "deleted"
	}
	return "kept"
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
		m.releaseActiveSession()
		m.screen = m.onCancelled
		return m, nil
	}
	return m, cmd
}

func (m *model) viewRevokeConfirm() string {
	return formScreen(m, m.confirmTitle, m.revokeForm.View(), "Esc Cancel", "Ctrl+C Quit")
}

func (m *model) startExportForm() (tea.Model, tea.Cmd) {
	home, err := os.UserHomeDir()
	if err != nil {
		m.setError(fmt.Errorf("resolve home directory: %w", err))
		return m, nil
	}
	m.exportPath = safeProfileFilename(m.selectedPeer.Name)
	m.exportForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Save " + m.selectedPeer.Name + " in " + home).
			Description("File name only. The secret profile is written with mode 0600.").
			Value(&m.exportPath).
			Validate(func(s string) error {
				if s == "" || s == "." || s == ".." {
					return errRequired
				}
				if filepath.Base(s) != s || strings.ContainsAny(s, `/\`) {
					return fieldErr("enter a file name, not a path")
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
		home, err := os.UserHomeDir()
		if err != nil {
			m.setError(err)
			return m, nil
		}
		m.exportPeer(filepath.Join(home, m.exportPath))
		return m, nil
	case huh.StateAborted:
		m.screen = screenServerDetail
		return m, nil
	}
	return m, cmd
}

func safeProfileFilename(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		if b.Len() >= 72 {
			break
		}
	}
	base := strings.Trim(b.String(), "._")
	if base == "" {
		base = "wireguard-peer"
	}
	return base + ".conf"
}

func (m *model) viewExportForm() string {
	return formScreen(m, "Save client profile", m.exportForm.View(), "Esc Cancel", "Ctrl+C Quit")
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
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if os.IsExist(err) {
			m.setError(fmt.Errorf("%s already exists", path))
		} else {
			m.setError(err)
		}
		return
	}
	if _, err := f.WriteString(profile); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		m.setError(err)
		return
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		m.setError(err)
		return
	}
	m.setStatus("Exported " + meta.Name + " to " + path)
}
