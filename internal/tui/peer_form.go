package tui

import (
	"context"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/peerops"
	"github.com/DMYTROSKORIN/wgtui/internal/peerview"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func (m *model) startPeerForm() {
	m.pfName, m.pfSplitCIDRs, m.pfKeepalive, m.pfDNS, m.pfEndpointHost = "", "", "25", "", ""
	m.pfFullTunnel, m.pfPSK = true, true
	serverID := m.selected.ID

	m.peerForm = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Peer name").Value(&m.pfName).
				Validate(func(s string) error {
					if s == "" {
						return errRequired
					}
					if _, exists := m.vault.GetPeerByName(serverID, s); exists {
						return fieldErr("a peer named " + s + " already exists")
					}
					return nil
				}),
			huh.NewSelect[bool]().Title("Tunnel mode").Height(4).
				Options(huh.NewOption("Full tunnel (all traffic)", true), huh.NewOption("Split tunnel", false)).
				Value(&m.pfFullTunnel),
		),
		huh.NewGroup(
			huh.NewInput().Title("Split tunnel CIDRs").Description("Comma-separated, e.g. 10.0.0.0/8, 192.168.1.0/24").
				Value(&m.pfSplitCIDRs).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fieldErr("at least one CIDR is required for split tunnel")
					}
					return nil
				}),
		).WithHideFunc(func() bool { return m.pfFullTunnel }),
		huh.NewGroup(
			huh.NewInput().Title("Keepalive (seconds)").Description("0 to omit.").Value(&m.pfKeepalive).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if _, err := strconv.Atoi(s); err != nil {
						return fieldErr("must be a number")
					}
					return nil
				}),
			huh.NewInput().Title("DNS servers").Description("Optional, comma-separated.").Value(&m.pfDNS),
			huh.NewInput().Title("Endpoint host").Description("Optional - defaults to the server's SSH host.").Value(&m.pfEndpointHost),
			huh.NewConfirm().Title("Add an extra preshared key (recommended)?").
				Description("Optional 256-bit secret mixed into WireGuard's normal public-key handshake.").
				Affirmative("Yes").Negative("No").Value(&m.pfPSK),
		),
	).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.screen = screenPeerForm
}

func (m *model) updatePeerForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.peerForm.Update(msg)
	m.peerForm = form.(*huh.Form)

	switch m.peerForm.State {
	case huh.StateCompleted:
		return m, m.startPeerAdd()
	case huh.StateAborted:
		m.screen = screenServerDetail
		return m, nil
	}
	return m, cmd
}

func (m *model) viewPeerForm() string {
	return formScreen(m, "Add peer", m.peerForm.View(), "Esc Cancel", "Ctrl+C Quit")
}

// startPeerAdd connects, resolves the interface, and allocates the next
// free VPN address so the plan screen can show it - exactly the same
// preparatory calls internal/cli/peer_mutate.go's runPeerAdd makes before
// asking for confirmation.
func (m *model) startPeerAdd() tea.Cmd {
	s := m.selected
	name := m.pfName
	fullTunnel := m.pfFullTunnel
	splitCIDRs := peerops.SplitTrim(m.pfSplitCIDRs, ",")
	keepalive, _ := strconv.Atoi(m.pfKeepalive)
	dns := peerops.SplitTrim(m.pfDNS, ",")
	endpointHost := m.pfEndpointHost
	withPSK := m.pfPSK

	sess := m.detailSession
	if sess == nil || sess.closed {
		return func() tea.Msg {
			return planReadyMsg{err: fmt.Errorf("server session is not connected; return to Overview and press r"), cancelScreen: screenServerDetail}
		}
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		iface, err := peerops.ResolveInterface(ctx, sess.client, "")
		if err != nil {
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		cc, err := peerops.FetchCurrentConfig(ctx, sess.client, sess.prompts, iface)
		if err != nil {
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		addr, err := peerops.NextFreeAddress(cc.VPNCIDR, cc.UsedAddresses)
		if err != nil {
			return planReadyMsg{err: fmt.Errorf("allocate VPN address: %w", err), cancelScreen: screenServerDetail}
		}
		serverAllowedIPs := []string{addr.String() + "/32"}

		clientAllowedIPs := splitCIDRs
		mode := "split tunnel"
		if fullTunnel {
			clientAllowedIPs = []string{"0.0.0.0/0", "::/0"}
			mode = "full tunnel"
		}
		host := endpointHost
		if host == "" {
			host = s.SSHHostOrAlias
		}

		plan := fmt.Sprintf("+ peer %s\n+ server allowed_ips: %s\n+ client vpn address: %s\n+ client mode: %s\n+ interface: %s",
			name, serverAllowedIPs[0], addr, mode, iface)

		params := peerAddParams{
			serverID: s.ID, name: name, cc: cc,
			serverAllowedIPs: serverAllowedIPs, clientAllowedIPs: clientAllowedIPs,
			keepalive: keepalive, withPSK: withPSK, dns: dns,
			endpointHost: host, addr: addr, iface: iface,
		}
		return planReadyMsg{
			sess: sess, borrowed: true, cancelScreen: screenServerDetail,
			title: "Add peer " + name + "?",
			plan:  plan,
			apply: m.applyPeerAdd(params),
		}
	}
}

// peerAddParams carries the AddPeer call's already-computed arguments from
// startPeerAdd's connect phase through to applyPeerAdd's apply phase - cc
// is reused as-is rather than refetched (same single-fetch assumption
// internal/cli/peer_mutate.go's runPeerAdd makes; transaction.Apply's own
// backup/verify/rollback is what actually guards against drift, per
// docs/THREAT_MODEL.md T4).
type peerAddParams struct {
	serverID                           string
	name, iface                        string
	cc                                 peerops.CurrentConfig
	serverAllowedIPs, clientAllowedIPs []string
	keepalive                          int
	withPSK                            bool
	dns                                []string
	endpointHost                       string
	addr                               netip.Addr
}

func (m *model) applyPeerAdd(p peerAddParams) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			result, err := peerops.AddPeer(ctx, sess.client, sess.prompts, p.cc, p.name, p.serverAllowedIPs, p.keepalive, p.withPSK)
			if err != nil {
				return actionDoneMsg{err: err}
			}
			clientConf := peerops.BuildClientProfile(peerops.ClientProfileParams{
				ClientPrivateKey: result.KeyPair.PrivateKey, ClientAddress: p.addr, DNS: p.dns,
				ServerPublicKey: p.cc.ServerPublicKey, PresharedKey: result.PresharedKey,
				EndpointHost: p.endpointHost, EndpointPort: p.cc.ListenPort,
				AllowedIPs: p.clientAllowedIPs, PersistentKeepalive: p.keepalive,
			})
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				m.vault.SetProfile(p.serverID, result.KeyPair.PublicKey, clientConf)
				_ = m.vault.AddPeer(profilevault.PeerMetadata{
					ServerID: p.serverID, Name: p.name, PublicKey: result.KeyPair.PublicKey,
					Interface: p.iface, CreatedAt: time.Now(), Status: profilevault.PeerStatusActive,
				})
				if err := m.vault.Save(m.vaultPath); err != nil {
					m.setError(fmt.Errorf("peer was created on the server, but its local encrypted profile could not be saved: %w", err))
					return nil
				}
				m.peersLoaded = false
				m.revealName = p.name
				m.revealProfile = clientConf
				m.revealMode = ""
				m.screen = screenPeerReveal
				m.setStatus("Added " + p.name + ". Client profile saved in the encrypted vault.")
				return m.loadPeersFromSessionCmd()
			}}
		}
	}
}

// updatePeerReveal drives the post-creation screen: the client profile
// (private key included) is never shown or QR-encoded by default - the
// user has to explicitly ask, same as the CLI's --show/--qr flags, just
// as an in-place choice instead of an upfront one.
func (m *model) updatePeerReveal(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "s":
		m.revealMode = "text"
	case "g", "Q":
		m.revealMode = "qr"
	case "e":
		m.selectedPeer = peerview.Peer{Name: m.revealName}
		return m.startExportForm()
	case "enter", "q", "esc":
		m.revealMode = ""
		m.screen = screenServerDetail
	}
	return m, nil
}

func (m *model) viewPeerReveal() string {
	var b strings.Builder
	b.WriteString(accentHeadingStyle.Render("Client profile: "+m.revealName) + "\n\n")

	switch m.revealMode {
	case "text":
		b.WriteString(dangerStyle.Render("WARNING: this is a secret client profile - do not paste it anywhere untrusted.") + "\n\n")
		b.WriteString(m.revealProfile)
	case "qr":
		art, err := peerops.RenderQRTerminal(m.revealProfile)
		if err != nil {
			b.WriteString(dangerStyle.Render("render QR: " + err.Error()))
		} else {
			b.WriteString(dangerStyle.Render("WARNING: this QR code encodes the client's private key - it is a secret.") + "\n\n")
			b.WriteString(art)
		}
	default:
		b.WriteString(textStyle.Render("The client profile is saved in the local vault.") + "\n")
		b.WriteString(dimStyle.Render("A fresh handshake from the client is the only real proof it works - this doesn't wait for one."))
	}

	return appHeader(m.selected.DisplayName+" / "+m.revealName) + "\n" + fullPanel(m, b.String()) + "\n" +
		keyBar("s Show config", "g Show QR", "e Save to home", "Enter Done", "Ctrl+C Quit")
}
