package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/netip"
	"strings"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/osadapter"
)

func runServerInstallWireguard(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui server install-wireguard: expected the server ID as the first argument")
		return 2
	}
	serverID := args[0]
	fs := flag.NewFlagSet("server install-wireguard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui server install-wireguard: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	client, _, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: %v\n", err)
		return 1
	}
	defer cleanup()

	if res, err := client.RunCommand(ctx, "command -v wg"); err == nil && res.ExitCode == 0 {
		fmt.Fprintln(stdout, "WireGuard is already installed - nothing to do.")
		return 0
	}

	distro, err := osadapter.DetectDistro(ctx, client)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: %v\n", err)
		return 1
	}
	if !distro.Supported() {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: unsupported distribution %q - only Ubuntu, Debian, and Fedora are supported in this version (SPEC.md §30.6)\n", distro.ID)
		return 1
	}

	plan, err := osadapter.PlanInstall(distro)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: %v\n", err)
		return 1
	}

	planDesc := fmt.Sprintf("Distribution: %s %s\nCommands:\n  sudo %s", distro.ID, distro.VersionID, strings.Join(plan.Commands, "\n  sudo "))
	if !confirm(stdout, *yes, planDesc) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}

	if err := osadapter.ApplyInstall(ctx, client, prompts, plan); err != nil {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: %v\n", err)
		return 1
	}
	version, err := osadapter.VerifyInstalled(ctx, client)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server install-wireguard: install ran but post-install verification failed: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Installed. %s", version)
	return 0
}

func runServerConfigure(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui server configure: expected the server ID as the first argument")
		return 2
	}
	serverID := args[0]

	fs := flag.NewFlagSet("server configure", flag.ContinueOnError)
	fs.SetOutput(stderr)
	iface := fs.String("interface", "wg0", "interface name to create")
	vpnCIDR := fs.String("vpn-cidr", "", "VPN IPv4 CIDR (auto-suggested if omitted)")
	port := fs.Int("port", 51820, "UDP listen port")
	outbound := fs.String("outbound-interface", "", "outbound interface for NAT (auto-detected if omitted)")
	fullTunnelDefault := fs.Bool("full-tunnel-default", true, "embed NAT/forwarding for full-tunnel clients")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui server configure: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server configure: %v\n", err)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	client, _, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server configure: %v\n", err)
		return 1
	}
	defer cleanup()

	if res, err := client.RunCommand(ctx, "command -v wg-quick"); err != nil || res.ExitCode != 0 {
		fmt.Fprintln(stderr, "wgtui server configure: wg-quick not found - run 'wgtui server install-wireguard' first")
		return 1
	}

	var cidr netip.Prefix
	if *vpnCIDR != "" {
		cidr, err = netip.ParsePrefix(*vpnCIDR)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui server configure: --vpn-cidr: %v\n", err)
			return 1
		}
	} else {
		suggested, rejected, err := osadapter.SuggestVPNSubnet(ctx, client)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui server configure: %v\n", err)
			return 1
		}
		cidr = suggested
		for cand, reason := range rejected {
			fmt.Fprintf(stdout, "(skipped candidate %s: %s)\n", cand, reason)
		}
	}
	serverAddr := netip.PrefixFrom(cidr.Addr().Next(), cidr.Bits())

	outboundIface := *outbound
	if outboundIface == "" {
		outboundIface, err = osadapter.DetectOutboundInterface(ctx, client)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui server configure: %v\n", err)
			return 1
		}
		if outboundIface == "" && *fullTunnelDefault {
			fmt.Fprintln(stderr, "wgtui server configure: could not auto-detect an outbound interface for NAT; pass --outbound-interface or --full-tunnel-default=false")
			return 1
		}
	}

	planDesc := fmt.Sprintf(
		"+ interface: %s\n+ VPN address: %s\n+ listen port: %d\n+ NAT/forwarding: %v (outbound: %s)\n+ firewall: open UDP %d\n+ systemctl enable --now wg-quick@%s",
		*iface, serverAddr, *port, *fullTunnelDefault, outboundIface, *port, *iface)
	if !confirm(stdout, *yes, planDesc) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}

	pubKey, err := osadapter.CreateInterface(ctx, client, prompts, osadapter.NewInterfaceSpec{
		Name: *iface, Address: serverAddr, ListenPort: *port,
		NAT: *fullTunnelDefault, OutboundInterface: outboundIface,
	})
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server configure: create interface: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Interface created. Server public key: %s\n", pubKey)

	if *fullTunnelDefault {
		if err := osadapter.EnableForwarding(ctx, client, prompts, false); err != nil {
			fmt.Fprintf(stderr, "wgtui server configure: enable forwarding: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "IPv4 forwarding enabled.")
	}

	backend := osadapter.DetectFirewallBackend(ctx, client)
	if backend == "" {
		fmt.Fprintln(stdout, "No supported firewall backend detected - open UDP port", *port, "manually if traffic is being filtered.")
	} else if alreadyOpen, err := osadapter.IsPortOpen(ctx, client, prompts, backend, *port); err != nil {
		fmt.Fprintf(stderr, "wgtui server configure: check existing firewall rules: %v\n", err)
		return 1
	} else if alreadyOpen {
		fmt.Fprintf(stdout, "Firewall (%s): UDP %d already open, nothing to do.\n", backend, *port)
	} else {
		fwPlan, err := osadapter.PlanOpenPort(backend, *port)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui server configure: %v\n", err)
			return 1
		}
		if err := osadapter.ApplyFirewallPlan(ctx, client, prompts, fwPlan); err != nil {
			fmt.Fprintf(stderr, "wgtui server configure: apply firewall plan: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Firewall (%s): UDP %d opened.\n", backend, *port)
	}
	if backend == "ufw" && osadapter.UfwIsInactive(ctx, client, prompts) {
		fmt.Fprintf(stdout, "Note: ufw is disabled - the UDP %d rule is saved but won't take effect until you run `ufw enable` on the server.\n", *port)
	}

	if err := osadapter.EnableAndStart(ctx, client, prompts, *iface); err != nil {
		fmt.Fprintf(stderr, "wgtui server configure: %v\n", err)
		return 1
	}

	checks, err := osadapter.RunPostStartChecks(ctx, client, *iface, *port)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui server configure: post-start checks: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Post-start checks: interface_active=%v address_assigned=%v port_listening=%v forwarding_on=%v\n",
		checks.InterfaceActive, checks.AddressAssigned, checks.PortListening, checks.IPv4ForwardingOn)
	if !checks.AllPassed() {
		fmt.Fprintln(stderr, "wgtui server configure: one or more post-start checks failed - see above")
		return 1
	}

	if outboundIface != "" {
		if lanIP, err := osadapter.LANAddress(ctx, client, outboundIface); err == nil {
			instr := osadapter.BuildRouterInstructions(*port, lanIP)
			fmt.Fprintln(stdout, "\nIf this server is behind a router, forward this port to it:")
			fmt.Fprintln(stdout, instr.String())
			for _, w := range instr.Warnings {
				fmt.Fprintln(stdout, "  - "+w)
			}
		}
	}

	fmt.Fprintln(stdout, "\nDone. SSH is still reachable, since this command ran over it.")
	return 0
}
