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
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/peerops"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

func (m *model) rebuildPeerTable() {
	available := m.peerTableAvailableWidth()
	var cols []table.Column
	mode := "compact"
	switch {
	case available >= 145:
		mode = "extended"
		// bubbles/table adds one cell of padding on each side of every
		// column. Budget that frame explicitly: otherwise the seven-column
		// table is six cells wider than the panel and overwrites its border.
		nameWidth := max(24, available-103)
		cols = []table.Column{
			{Title: "Peer / alias", Width: nameWidth},
			{Title: "VPN IP", Width: 16},
			{Title: "State", Width: 12},
			{Title: "Last seen", Width: 14},
			{Title: "Endpoint", Width: 27},
			{Title: "RX", Width: 10},
			{Title: "TX", Width: 10},
		}
	case available >= 108:
		mode = "traffic"
		nameWidth := max(24, available-74)
		cols = []table.Column{
			{Title: "Peer / alias", Width: nameWidth},
			{Title: "VPN IP", Width: 16},
			{Title: "State", Width: 12},
			{Title: "Last seen", Width: 14},
			{Title: "RX", Width: 10},
			{Title: "TX", Width: 10},
		}
	case available >= 78:
		mode = "standard"
		nameWidth := max(24, available-50)
		cols = []table.Column{
			{Title: "Peer / alias", Width: nameWidth},
			{Title: "VPN IP", Width: 16},
			{Title: "State", Width: 12},
			{Title: "Last seen", Width: 14},
		}
	default:
		nameWidth := max(16, available-32)
		cols = []table.Column{
			{Title: "Peer / alias", Width: nameWidth},
			{Title: "State", Width: 12},
			{Title: "Last seen", Width: 14},
		}
	}
	rows := make([]table.Row, 0, len(m.peers))
	for _, p := range m.peers {
		name := truncateCell(peerTableLabel(p), cols[0].Width)
		values := []string{name}
		if mode != "compact" {
			values = append(values, orDefault(terminalText(p.VPNIP), "-"))
		}
		values = append(values, peerStateTableLabel(p.State), formatHandshakeAge(p.LatestHandshakeSeconds))
		if mode == "extended" {
			values = append(values, orDefault(terminalText(p.Endpoint), "-"))
		}
		if mode == "traffic" || mode == "extended" {
			values = append(values, formatBytesCompact(p.RxBytes), formatBytesCompact(p.TxBytes))
		}
		row := make(table.Row, len(values))
		for i, value := range values {
			if lipgloss.Width(value) > cols[i].Width {
				value = truncateCell(ansi.Strip(value), cols[i].Width)
			}
			row[i] = value
		}
		rows = append(rows, row)
	}

	styles := table.DefaultStyles()
	styles.Header = styles.Header.Foreground(colorMuted).Bold(true)
	// Keep Cell free of ANSI foreground styles. Selected wraps the complete
	// joined row; an inner cell reset would cancel that background after the
	// first column and recreate the fragmented selection from the report.
	styles.Selected = styles.Selected.Foreground(colorText).Bold(true).Background(colorSelection)

	tableHeight := min(max(3, len(rows)+1), max(5, m.height-16))
	if layoutModeFor(m.width, m.height) == layoutWide {
		tableHeight = max(5, m.height-12)
	}

	// Window rows[m.peerWindowStart:] ourselves - the same "cursor moves
	// within a stable view, the view only shifts once the cursor would go
	// off top/bottom" rule internal/tui/diagnose.go's list already uses -
	// instead of handing bubbles/table the full row set and letting its
	// own (unexported, unfixable from here) scroll state decide, which
	// visibly moved the whole list instead of the selection.
	visible := max(1, tableHeight-1) // content rows only; header takes 1
	m.peerCursor = clamp(m.peerCursor, 0, max(0, len(m.peers)-1))
	m.peerWindowStart = clamp(m.peerCursor-visible+1, 0, max(0, len(rows)-visible))
	end := min(len(rows), m.peerWindowStart+visible)
	window := rows[m.peerWindowStart:end]

	m.peerTable = table.New(
		table.WithColumns(cols),
		table.WithRows(window),
		table.WithFocused(true),
		table.WithHeight(len(window)+1),
		table.WithStyles(styles),
	)
	if len(window) > 0 {
		m.peerTable.SetCursor(m.peerCursor - m.peerWindowStart)
	}
}

// peerStateTableLabel deliberately returns plain text. Pre-rendered ANSI
// styles inside table cells reset the parent row background, which made only
// fragments of the selected row appear highlighted and confused terminal
// width handling. The inspector keeps the fully colored peerStateBadge.
func peerStateTableLabel(state string) string {
	switch state {
	case "online":
		return "● online"
	case "disabled":
		return "● disabled"
	case "drifted":
		return "● drifted"
	case "error":
		return "● error"
	case "never_connected":
		return "○ never"
	case "unmanaged":
		return "○ unmanaged"
	case "idle":
		return "○ idle"
	default:
		return "○ " + orDefault(terminalText(state), "unknown")
	}
}

func peerTableLabel(p peerview.Peer) string {
	if p.Managed || p.PublicKey == "" {
		return terminalText(p.Name)
	}
	return "unmanaged · " + terminalText(peerview.ShortenKey(p.PublicKey))
}

func truncateCell(value string, width int) string {
	value = terminalText(value)
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	if width == 1 {
		return "…"
	}
	var b strings.Builder
	for _, r := range value {
		candidate := b.String() + string(r) + "…"
		if lipgloss.Width(candidate) > width {
			break
		}
		b.WriteRune(r)
	}
	return b.String() + "…"
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
	if layoutModeFor(m.width, m.height) == layoutWide {
		peersWidth, _ := peerSplitPanelWidths(m.width)
		return max(48, peersWidth-4)
	}
	return max(48, m.width-8)
}

func peerSplitPanelWidths(total int) (peers, inspector int) {
	inspector = clamp(total/3, 42, 58)
	peers = max(70, total-inspector-5)
	return
}

func (m *model) viewPeersTab() string {
	list := m.viewPeerList()
	if summary := m.viewPeerSummary(); summary != "" {
		list = summary + "\n" + list
	}
	if layoutModeFor(m.width, m.height) == layoutStandard && m.peersLoaded && m.peersErr == "" && len(m.peers) > 0 {
		list += "\n\n" + m.viewSelectedPeerCard()
	}
	return list
}

func (m *model) viewPeerSummary() string {
	if !m.peersLoaded || m.peersErr != "" {
		return ""
	}
	online, attention, unmanaged := 0, 0, 0
	for _, peer := range m.peers {
		if peer.State == "online" {
			online++
		}
		if peer.State == "disabled" || peer.State == "drifted" || peer.State == "error" || peer.State == "never_connected" {
			attention++
		}
		if !peer.Managed {
			unmanaged++
		}
	}
	return dimStyle.Render(fmt.Sprintf("%d total", len(m.peers))) + "  " +
		successStyle.Render(fmt.Sprintf("● %d online", online)) + "  " +
		warningStyle.Render(fmt.Sprintf("! %d attention", attention)) + "  " +
		dimStyle.Render(fmt.Sprintf("○ %d unmanaged", unmanaged))
}

func (m *model) viewPeerList() string {
	switch {
	case m.loading && !m.peersLoaded:
		return warningStyle.Render("↻ Connecting to the server…") + "\n" +
			dimStyle.Render("Peers will load automatically when the connection is ready.") + "\n"
	case !m.peersLoaded:
		return warningStyle.Render("↻ Loading peers from the server…") + "\n"
	case m.peersErr != "":
		return dangerStyle.Render(wrapPlainText("Error: "+m.peersErr, m.peerTableAvailableWidth(), 4)) + "\n"
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

func formatBytesCompact(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit && exp < 3; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%c", float64(n)/float64(div), "KMGT"[exp])
}

func (m *model) viewSelectedPeerCard() string {
	row := m.peerCursor
	if row < 0 || row >= len(m.peers) {
		return ""
	}
	p := m.peers[row]
	managed := "unmanaged (read-only)"
	managedStyle := warningStyle
	if p.Managed {
		managed = "managed by wgtui"
		managedStyle = successStyle
	}
	profile := "not available"
	if p.ProfileAvailable {
		profile = "stored securely in local vault"
	}
	nameWidth := max(12, m.width-24)
	return titleStyle.Render("Selected peer") + "\n" +
		selectedStyle.Render(truncateCell(p.Name, nameWidth)) + "  " + peerStateBadge(p.State) + "\n" +
		dimStyle.Render("Interface  ") + textStyle.Render(orDefault(terminalText(p.Interface), "-")) + "   " +
		dimStyle.Render("Public key  ") + textStyle.Render(terminalText(peerview.ShortenKey(p.PublicKey))) + "\n" +
		dimStyle.Render("Profile    ") + textStyle.Render(profile) +
		"\n" + managedStyle.Render(managed) + dimStyle.Render(" · Enter opens available actions")
}

func (m *model) viewSelectedPeerInspector() string {
	row := m.peerCursor
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
	ownershipStyle := warningStyle
	if p.Managed {
		ownership = "Managed by wgtui"
		ownershipStyle = successStyle
	}
	psk := "No"
	if p.State == "disabled" {
		psk = "Not active"
	} else if p.PresharedKeyPresent {
		psk = "Yes · secret hidden"
	}
	_, inspectorWidth := peerSplitPanelWidths(m.width)
	contentWidth := max(24, inspectorWidth-4)
	name := truncateCell(peerTableLabel(p), max(12, contentWidth-12))
	return accentHeadingStyle.Render("Selected peer") + "\n\n" +
		selectedStyle.Render(name) + "  " + peerStateBadge(p.State) + "\n\n" +
		inspectorLine("VPN address", orDefault(p.VPNIP, "-"), contentWidth) + "\n" +
		inspectorLine("Interface", orDefault(p.Interface, "-"), contentWidth) + "\n" +
		inspectorLine("Endpoint", orDefault(p.Endpoint, "Not observed"), contentWidth) + "\n" +
		inspectorLine("Last handshake", formatHandshakeAge(p.LatestHandshakeSeconds), contentWidth) + "\n" +
		inspectorLine("Traffic ↓ / ↑", formatBytes(p.RxBytes)+" / "+formatBytes(p.TxBytes), contentWidth) + "\n\n" +
		dimStyle.Render("Public key") + "\n" + textStyle.Render(wrapPlainText(p.PublicKey, contentWidth, 2)) + "\n" +
		inspectorLine("Preshared key", psk, contentWidth) + "\n" +
		inspectorLine("Client profile", profile, contentWidth) + "\n" +
		ownershipStyle.Render(ownership) + "\n\n" +
		dimStyle.Render("Enter opens actions · r refreshes")
}

func inspectorLine(label, value string, width int) string {
	const labelWidth = 16
	label = truncateCell(label, labelWidth-1)
	value = truncateCell(value, max(8, width-labelWidth))
	return dimStyle.Width(labelWidth).Render(label) + textStyle.Render(value)
}

func wrapPlainText(value string, width, maxLines int) string {
	value = terminalText(value)
	if width <= 0 || lipgloss.Width(value) <= width {
		return value
	}
	var lines []string
	for value != "" && len(lines) < maxLines {
		if len(lines) == maxLines-1 {
			lines = append(lines, truncateCell(value, width))
			break
		}
		part := truncateCell(value, width)
		part = strings.TrimSuffix(part, "…")
		if part == "" {
			break
		}
		lines = append(lines, part)
		value = strings.TrimPrefix(value, part)
	}
	return strings.Join(lines, "\n")
}

// loadPeersFromSessionCmd refreshes the peer table over the connection
// opened for the server detail screen. Discovery has already collected
// and cached any SSH/sudo credentials in this session, so changing tabs
// never drops out of the TUI to ask for another password.
func (m *model) loadPeersFromSessionCmd() tea.Cmd {
	if m.peersRefreshing {
		return nil
	}
	m.peerRequestID++
	requestID := m.peerRequestID
	m.peersRefreshing = true
	s := m.selected
	sess := m.detailSession
	if sess == nil || sess.closed {
		return func() tea.Msg {
			return peerListMsg{serverID: s.ID, requestID: requestID, err: "server session is not connected; return to Overview and press r to reconnect"}
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
			return peerListMsg{serverID: s.ID, requestID: requestID, err: fmt.Sprintf("read WireGuard peers: %v", err)}
		}
		if res.ExitCode != 0 {
			detail := strings.TrimSpace(string(res.Stderr))
			if detail == "" {
				detail = "the remote command returned no diagnostic output"
			}
			if res.ExitCode == 255 {
				return peerListMsg{serverID: s.ID, requestID: requestID, connectionLost: true, err: fmt.Sprintf("SSH connection was lost while reading peers: %s", detail)}
			}
			return peerListMsg{serverID: s.ID, requestID: requestID, err: fmt.Sprintf("cannot read WireGuard peers (exit %d): %s", res.ExitCode, detail)}
		}
		dump, err := wgconfig.ParseDump(string(res.Stdout))
		if err != nil {
			return peerListMsg{serverID: s.ID, requestID: requestID, err: fmt.Sprintf("parse WireGuard peer data: %v", err)}
		}
		return peerListMsg{serverID: s.ID, requestID: requestID, peers: peerview.Reconcile(dump.Peers, localPeers, func(pk string) bool { return profileSet[pk] }, time.Now())}
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
	row := m.peerCursor
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
			peerActionOption{"Save profile + QR bundle to home directory", "export", false},
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
	card := accentHeadingStyle.Render("Peer") + "\n\n" + selectedStyle.Render(terminalText(p.Name)) + "  " + peerStateBadge(p.State) + "\n\n"
	if layoutModeFor(m.width, m.height) != layoutCompact {
		card += fmt.Sprintf("VPN IP: %s   Interface: %s\nPublic key: %s\n\n",
			orDefault(terminalText(p.VPNIP), "-"), orDefault(terminalText(p.Interface), "-"),
			terminalText(peerview.ShortenKey(p.PublicKey)))
	}
	card += strings.TrimSuffix(menu.String(), "\n")
	return fullPanel(m, card) + "\n" +
		appHeader(m.selected.DisplayName+" / Peers") + "\n" +
		keyBar(m.width, "↑/↓ Select", "Enter Continue", "Esc Back", "Ctrl+C Quit")
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
	m.revealNew = false
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
	m.exportReturn = screenServerDetail
	if m.screen == screenPeerReveal {
		m.exportReturn = screenPeerReveal
	}
	home, err := os.UserHomeDir()
	if err != nil {
		m.setError(fmt.Errorf("resolve home directory: %w", err))
		return m, nil
	}
	m.exportPath = safeProfileBase(m.selectedPeer.Name) + "_WireGuard"
	m.exportForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Save " + m.selectedPeer.Name + " bundle in " + home).
			Description("Directory name only. It will contain a secret .conf and QR PNG.").
			Value(&m.exportPath).
			Validate(func(s string) error {
				if s == "" || s == "." || s == ".." {
					return errRequired
				}
				if filepath.Base(s) != s || strings.ContainsAny(s, `/\`) {
					return fieldErr("enter a file name, not a path")
				}
				if safeProfileBase(s) != s {
					return fieldErr("use letters, numbers, hyphens, and underscores only")
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
		m.screen = m.exportReturn
		home, err := os.UserHomeDir()
		if err != nil {
			m.setError(err)
			return m, nil
		}
		m.exportPeerBundle(filepath.Join(home, m.exportPath))
		if m.exportReturn == screenPeerReveal && !m.statusErr {
			m.revealMode = ""
			m.revealCursor = len(clientSetupChoices) - 1
		}
		return m, nil
	case huh.StateAborted:
		m.screen = m.exportReturn
		return m, nil
	}
	return m, cmd
}

func safeProfileFilename(name string) string {
	return safeProfileBase(name) + ".conf"
}

func safeProfileBase(name string) string {
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
	return base
}

func (m *model) viewExportForm() string {
	return formScreen(m, "Save profile + QR", m.exportForm.View(), "Esc Cancel", "Ctrl+C Quit")
}

func (m *model) exportPeerBundle(dirPath string) {
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
	png, err := peerops.RenderQRPNG(profile)
	if err != nil {
		m.setError(err)
		return
	}
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		if os.IsExist(err) {
			m.setError(fmt.Errorf("%s already exists", dirPath))
		} else {
			m.setError(err)
		}
		return
	}
	base := safeProfileBase(meta.Name)
	configPath := filepath.Join(dirPath, base+".conf")
	qrPath := filepath.Join(dirPath, base+"_QR.png")
	if err := writeSecretFile(configPath, []byte(profile)); err != nil {
		_ = os.Remove(dirPath)
		m.setError(err)
		return
	}
	if err := writeSecretFile(qrPath, png); err != nil {
		_ = os.Remove(configPath)
		_ = os.Remove(dirPath)
		m.setError(err)
		return
	}
	m.setStatus("Exported " + meta.Name + " profile and QR to " + dirPath)
}

func writeSecretFile(path string, content []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close %s: %w", path, err)
	}
	return nil
}
