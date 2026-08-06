package osadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// DetectOutboundInterface returns the interface used by the server's
// default route ("" if none found) - unprivileged, read-only.
func DetectOutboundInterface(ctx context.Context, client *sshtransport.Client) (string, error) {
	res, err := client.RunCommand(ctx, "ip route show default")
	if err != nil {
		return "", fmt.Errorf("ip route show default: %w", err)
	}
	fields := strings.Fields(string(res.Stdout))
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1], nil
		}
	}
	return "", nil
}

// LANAddress returns the first IPv4 address assigned to iface, without
// its prefix length - used to fill in the SPEC.md §13.8 router
// instructions' "Internal IP".
func LANAddress(ctx context.Context, client *sshtransport.Client, iface string) (string, error) {
	if !interfaceNamePattern.MatchString(iface) {
		return "", fmt.Errorf("invalid interface name %q", iface)
	}
	res, err := client.RunCommand(ctx, "ip -o -4 addr show dev "+iface)
	if err != nil {
		return "", fmt.Errorf("ip addr show dev %s: %w", iface, err)
	}
	fields := strings.Fields(string(res.Stdout))
	for i, f := range fields {
		if f == "inet" && i+1 < len(fields) {
			addr := fields[i+1]
			if idx := strings.Index(addr, "/"); idx != -1 {
				addr = addr[:idx]
			}
			return addr, nil
		}
	}
	return "", fmt.Errorf("no IPv4 address found on %s", iface)
}
