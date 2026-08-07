package tui

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"

	"github.com/DMYTROSKORIN/wgtui/internal/osadapter"
)

// startInstallWizard mirrors internal/cli/server_setup.go's
// runServerInstallWireguard: check if it's already installed, detect the
// distro, and build a plan before asking to apply it.
func (m *model) startInstallWizard() tea.Cmd {
	s := m.selected
	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		target := connectingBanner(os.Stdout, s)
		prompts := newTerminalPrompts(os.Stdout, target)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return planReadyMsg{err: fmt.Errorf("connect: %w", err), cancelScreen: screenServerDetail}
		}

		if res, err := sess.client.RunCommand(ctx, "command -v wg"); err == nil && res.ExitCode == 0 {
			sess.close()
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				m.setStatus("WireGuard is already installed - nothing to do.")
				return nil
			}}
		}

		distro, err := osadapter.DetectDistro(ctx, sess.client)
		if err != nil {
			sess.close()
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}
		if !distro.Supported() {
			sess.close()
			return planReadyMsg{err: fmt.Errorf("unsupported distribution %q - only Ubuntu, Debian, and Fedora are supported", distro.ID), cancelScreen: screenServerDetail}
		}
		plan, err := osadapter.PlanInstall(distro)
		if err != nil {
			sess.close()
			return planReadyMsg{err: err, cancelScreen: screenServerDetail}
		}

		planText := fmt.Sprintf("Distribution: %s %s\nCommands:\n  sudo %s", distro.ID, distro.VersionID, strings.Join(plan.Commands, "\n  sudo "))
		return planReadyMsg{
			sess: sess, cancelScreen: screenServerDetail,
			title: "Install WireGuard?",
			plan:  planText,
			apply: m.applyInstall(plan),
		}
	})
}

func (m *model) applyInstall(plan osadapter.InstallPlan) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		s := m.selected
		return m.releaseTerminalAndRun(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()
			if err := osadapter.ApplyInstall(ctx, sess.client, sess.prompts, plan); err != nil {
				return actionDoneMsg{err: err}
			}
			version, err := osadapter.VerifyInstalled(ctx, sess.client)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("install ran but post-install verification failed: %w", err)}
			}
			return actionDoneMsg{done: func(m *model) tea.Cmd {
				m.setStatus("Installed. " + version)
				m.loading = true
				m.detailErr = ""
				return m.connectAndDiscoverCmd(s)
			}}
		})
	}
}

// configurePrepMsg carries the connected session plus auto-suggested
// defaults (VPN subnet, outbound interface) needed to pre-fill the
// configure form - both require a connection to compute, exactly like
// internal/cli/server_setup.go's runServerConfigure does before showing
// its plan.
type configurePrepMsg struct {
	err              error
	sess             *session
	suggestedCIDR    string
	detectedOutbound string
}

func (m *model) startConfigurePrep() tea.Cmd {
	s := m.selected
	return m.releaseTerminalAndRun(func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		target := connectingBanner(os.Stdout, s)
		prompts := newTerminalPrompts(os.Stdout, target)
		sess, err := dial(ctx, s, prompts)
		if err != nil {
			return configurePrepMsg{err: fmt.Errorf("connect: %w", err)}
		}
		if res, err := sess.client.RunCommand(ctx, "command -v wg-quick"); err != nil || res.ExitCode != 0 {
			sess.close()
			return configurePrepMsg{err: fmt.Errorf("wg-quick not found - install WireGuard first ([i])")}
		}

		var suggestedCIDR string
		if suggested, rejected, err := osadapter.SuggestVPNSubnet(ctx, sess.client); err == nil {
			suggestedCIDR = suggested.String()
			_ = rejected // shown as CLI stdout notes; not surfaced here to keep the form simple
		}
		outbound, _ := osadapter.DetectOutboundInterface(ctx, sess.client)

		return configurePrepMsg{sess: sess, suggestedCIDR: suggestedCIDR, detectedOutbound: outbound}
	})
}

func (m *model) handleConfigurePrep(msg configurePrepMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		msg.sess.close()
		m.setError(msg.err)
		return m, nil
	}
	m.activeSession = msg.sess
	m.cfSuggestedCIDR = msg.suggestedCIDR
	m.cfDetectedOutbound = msg.detectedOutbound
	m.startConfigureForm()
	m.screen = screenConfigureForm
	return m, m.cfForm.Init()
}

func (m *model) startConfigureForm() {
	m.cfInterface = "wg0"
	m.cfVPNCIDR = m.cfSuggestedCIDR
	m.cfPort = "51820"
	m.cfOutboundInterface = m.cfDetectedOutbound
	m.cfFullTunnelDefault = true

	m.cfForm = huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Interface name").Value(&m.cfInterface).
			Validate(func(s string) error {
				if s == "" {
					return errRequired
				}
				return nil
			}),
		huh.NewInput().Title("VPN IPv4 CIDR").Description("Auto-suggested; edit if you want something else.").Value(&m.cfVPNCIDR).
			Validate(func(s string) error {
				if s == "" {
					return errRequired
				}
				if _, err := netip.ParsePrefix(s); err != nil {
					return fieldErr("must be a valid CIDR, e.g. 10.10.0.0/24")
				}
				return nil
			}),
		huh.NewInput().Title("UDP listen port").Value(&m.cfPort).
			Validate(func(s string) error {
				if n, err := strconv.Atoi(s); err != nil || n <= 0 || n > 65535 {
					return fieldErr("must be a valid port number")
				}
				return nil
			}),
		huh.NewInput().Title("Outbound interface").Description("Used for NAT/forwarding; auto-detected if left blank.").Value(&m.cfOutboundInterface),
		huh.NewConfirm().Title("Enable full-tunnel NAT/forwarding by default?").Affirmative("Yes").Negative("No").Value(&m.cfFullTunnelDefault),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
}

func (m *model) updateConfigureForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	form, cmd := m.cfForm.Update(msg)
	m.cfForm = form.(*huh.Form)

	switch m.cfForm.State {
	case huh.StateCompleted:
		return m.submitConfigureForm()
	case huh.StateAborted:
		m.activeSession.close()
		m.activeSession = nil
		m.screen = screenServerDetail
		return m, nil
	}
	return m, cmd
}

func (m *model) viewConfigureForm() string {
	return titleStyle.Render("wgtui") + "  " + subtitleStyle.Render("Configure "+m.selected.DisplayName) + "\n\n" + m.cfForm.View()
}

// submitConfigureForm needs no further network calls - the session opened
// by startConfigurePrep is still held open - so it transitions straight to
// the confirm screen synchronously instead of round-tripping through
// planReadyMsg.
func (m *model) submitConfigureForm() (tea.Model, tea.Cmd) {
	cidr, _ := netip.ParsePrefix(m.cfVPNCIDR)
	serverAddr := netip.PrefixFrom(cidr.Addr().Next(), cidr.Bits())
	port, _ := strconv.Atoi(m.cfPort)
	iface := m.cfInterface
	outbound := m.cfOutboundInterface
	fullTunnel := m.cfFullTunnelDefault

	if outbound == "" && fullTunnel {
		m.setError(fmt.Errorf("could not auto-detect an outbound interface for NAT - set one, or turn off full-tunnel NAT/forwarding"))
		return m, nil
	}

	plan := fmt.Sprintf(
		"+ interface: %s\n+ VPN address: %s\n+ listen port: %d\n+ NAT/forwarding: %v (outbound: %s)\n+ firewall: open UDP %d\n+ enable and start %s",
		iface, serverAddr, port, fullTunnel, orDefault(outbound, "-"), port, iface)

	m.confirmYes = false
	m.confirmForm = huh.NewForm(huh.NewGroup(
		huh.NewConfirm().Title("Configure " + m.selected.DisplayName + "?").Affirmative("Apply").Negative("Cancel").Value(&m.confirmYes),
	)).WithTheme(formTheme()).WithKeyMap(formKeyMap())
	m.confirmTitle = "Configure " + m.selected.DisplayName + "?"
	m.confirmPlan = plan
	m.onCancelled = screenServerDetail
	m.onConfirmed = m.applyConfigure(iface, serverAddr, port, fullTunnel, outbound)
	m.screen = screenConfirm
	return m, m.confirmForm.Init()
}

func (m *model) applyConfigure(iface string, serverAddr netip.Prefix, port int, fullTunnel bool, outbound string) func(*model) tea.Cmd {
	return func(m *model) tea.Cmd {
		sess := m.activeSession
		s := m.selected
		return m.releaseTerminalAndRun(func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
			defer cancel()

			pubKey, err := osadapter.CreateInterface(ctx, sess.client, sess.prompts, osadapter.NewInterfaceSpec{
				Name: iface, Address: serverAddr, ListenPort: port, NAT: fullTunnel, OutboundInterface: outbound,
			})
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("create interface: %w", err)}
			}

			var steps []string
			steps = append(steps, "Interface created. Server public key: "+pubKey)

			if fullTunnel {
				if err := osadapter.EnableForwarding(ctx, sess.client, sess.prompts, false); err != nil {
					return actionDoneMsg{err: fmt.Errorf("enable forwarding: %w", err)}
				}
				steps = append(steps, "IPv4 forwarding enabled.")
			}

			backend := osadapter.DetectFirewallBackend(ctx, sess.client)
			switch {
			case backend == "":
				steps = append(steps, fmt.Sprintf("No supported firewall backend detected - open UDP %d manually if traffic is filtered.", port))
			default:
				alreadyOpen, err := osadapter.IsPortOpen(ctx, sess.client, sess.prompts, backend, port)
				if err != nil {
					return actionDoneMsg{err: fmt.Errorf("check firewall rules: %w", err)}
				}
				if alreadyOpen {
					steps = append(steps, fmt.Sprintf("Firewall (%s): UDP %d already open.", backend, port))
				} else {
					fwPlan, err := osadapter.PlanOpenPort(backend, port)
					if err != nil {
						return actionDoneMsg{err: err}
					}
					if err := osadapter.ApplyFirewallPlan(ctx, sess.client, sess.prompts, fwPlan); err != nil {
						return actionDoneMsg{err: fmt.Errorf("apply firewall plan: %w", err)}
					}
					steps = append(steps, fmt.Sprintf("Firewall (%s): UDP %d opened.", backend, port))
				}
			}

			if err := osadapter.EnableAndStart(ctx, sess.client, sess.prompts, iface); err != nil {
				return actionDoneMsg{err: err}
			}

			checks, err := osadapter.RunPostStartChecks(ctx, sess.client, iface, port)
			if err != nil {
				return actionDoneMsg{err: fmt.Errorf("post-start checks: %w", err)}
			}
			steps = append(steps, fmt.Sprintf("Post-start checks: interface_active=%v address_assigned=%v port_listening=%v forwarding_on=%v",
				checks.InterfaceActive, checks.AddressAssigned, checks.PortListening, checks.IPv4ForwardingOn))
			if !checks.AllPassed() {
				return actionDoneMsg{err: fmt.Errorf("one or more post-start checks failed: %s", strings.Join(steps, " / "))}
			}

			if outbound != "" {
				if lanIP, err := osadapter.LANAddress(ctx, sess.client, outbound); err == nil {
					instr := osadapter.BuildRouterInstructions(port, lanIP)
					steps = append(steps, "If this server is behind a router, forward this port to it: "+instr.String())
				}
			}

			return actionDoneMsg{done: func(m *model) tea.Cmd {
				m.setStatus(strings.Join(steps, "\n"))
				m.loading = true
				m.detailErr = ""
				return m.connectAndDiscoverCmd(s)
			}}
		})
	}
}
