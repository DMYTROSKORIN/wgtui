package osadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// EnableAndStart runs `systemctl enable --now wg-quick@<iface>` (SPEC.md
// §13.7).
func EnableAndStart(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, iface string) error {
	if !interfaceNamePattern.MatchString(iface) {
		return fmt.Errorf("refusing to manage an unsafe interface name %q", iface)
	}
	res, err := client.RunSudo(ctx, "systemctl enable --now wg-quick@"+iface, prompts)
	if err != nil {
		return fmt.Errorf("systemctl enable --now: %w", err)
	}
	if res.ExitCode != 0 {
		return fmt.Errorf("systemctl enable --now wg-quick@%s exited %d: %s", iface, res.ExitCode, res.Stderr)
	}
	return nil
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
