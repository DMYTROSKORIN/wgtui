package osadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// ServiceManager is the detected init system - independent of distro/
// package manager, since a future distro could pair a non-systemd init
// with either apt- or dnf-family packaging.
type ServiceManager string

const (
	ServiceManagerSystemd ServiceManager = "systemd"
	ServiceManagerOpenRC  ServiceManager = "openrc"
)

// DetectServiceManager identifies the init system in use.
func DetectServiceManager(ctx context.Context, client *sshtransport.Client) (ServiceManager, error) {
	if commandExists(ctx, client, "systemctl") {
		return ServiceManagerSystemd, nil
	}
	if commandExists(ctx, client, "rc-service") {
		return ServiceManagerOpenRC, nil
	}
	return "", fmt.Errorf("no supported service manager detected (systemd or OpenRC)")
}

// EnableAndStart brings iface up and makes it start on boot, via whichever
// service manager is detected (SPEC.md §13.7).
func EnableAndStart(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, iface string) error {
	if !interfaceNamePattern.MatchString(iface) {
		return fmt.Errorf("refusing to manage an unsafe interface name %q", iface)
	}
	sm, err := DetectServiceManager(ctx, client)
	if err != nil {
		return err
	}
	switch sm {
	case ServiceManagerSystemd:
		return enableAndStartSystemd(ctx, client, prompts, iface)
	case ServiceManagerOpenRC:
		return enableAndStartOpenRC(ctx, client, prompts, iface)
	default:
		return fmt.Errorf("unsupported service manager %q", sm)
	}
}

func enableAndStartSystemd(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, iface string) error {
	res, err := client.RunSudo(ctx, "systemctl enable --now wg-quick@"+iface, prompts)
	if err != nil {
		return fmt.Errorf("systemctl enable --now: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("systemctl enable --now wg-quick@%s exited %d: %s", iface, res.ExitCode, res.Stderr)
	}
	return nil
}

// enableAndStartOpenRC is best-effort and not yet validated against a real
// OpenRC target: none of the MVP distros (Ubuntu/Debian/Fedora - decision
// §30.6) use OpenRC, so this exists so a future non-systemd distro (e.g.
// Alpine) doesn't need a new abstraction layer, not because it's been
// proven against a live server the way every systemd path in this project
// has been. It follows the widely-documented community convention of a
// per-interface service script symlinked from a template, mirroring
// systemd's wg-quick@<iface> instancing.
func enableAndStartOpenRC(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, iface string) error {
	for _, cmd := range openRCEnableCommands(iface) {
		res, err := client.RunSudo(ctx, cmd, prompts)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("%s exited %d: %s", cmd, res.ExitCode, res.Stderr)
		}
	}
	return nil
}

// openRCEnableCommands builds the command sequence for
// enableAndStartOpenRC, split out for unit testing without a network
// dependency. iface must already be validated (EnableAndStart's caller
// does this) before it reaches here.
func openRCEnableCommands(iface string) []string {
	svc := "wg-quick." + iface
	return []string{
		"sh -c 'test -e /etc/init.d/" + svc + " || ln -s wg-quick /etc/init.d/" + svc + "'",
		"rc-update add " + svc + " default",
		"rc-service " + svc + " start",
	}
}

// PostStartChecks is the SPEC.md §13.7 checklist run immediately after
// EnableAndStart: interface active, address assigned, UDP port listening,
// forwarding enabled. "SSH connection stayed reachable" isn't a separate
// field - if these checks are running at all, the SSH session that ran
// EnableAndStart is still alive.
type PostStartChecks struct {
	InterfaceActive  bool
	AddressAssigned  bool
	PortListening    bool
	IPv4ForwardingOn bool
}

// RunPostStartChecks performs the checks read-only, without sudo where
// possible.
func RunPostStartChecks(ctx context.Context, client *sshtransport.Client, iface string, listenPort int) (PostStartChecks, error) {
	var c PostStartChecks

	res, err := client.RunCommand(ctx, "wg show interfaces")
	if err != nil {
		return c, fmt.Errorf("wg show interfaces: %w", err)
	}
	for _, name := range strings.Fields(string(res.Stdout)) {
		if name == iface {
			c.InterfaceActive = true
		}
	}

	addrRes, err := client.RunCommand(ctx, "ip -o -4 addr show dev "+iface)
	if err == nil && addrRes.ExitCode == 0 && strings.TrimSpace(string(addrRes.Stdout)) != "" {
		c.AddressAssigned = true
	}

	portRes, err := client.RunCommand(ctx, "ss -lun")
	if err == nil && portRes.ExitCode == 0 {
		c.PortListening = strings.Contains(string(portRes.Stdout), fmt.Sprintf(":%d", listenPort))
	}

	fwd, err := CheckForwarding(ctx, client, false)
	if err == nil {
		c.IPv4ForwardingOn = fwd.IPv4Enabled
	}

	return c, nil
}

// AllPassed reports whether every check succeeded.
func (c PostStartChecks) AllPassed() bool {
	return c.InterfaceActive && c.AddressAssigned && c.PortListening && c.IPv4ForwardingOn
}
