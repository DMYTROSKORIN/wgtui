package osadapter

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// candidateVPNSubnets are tried in order; the first one that doesn't
// overlap anything in ExistingRoutes wins (SPEC.md §13.2).
var candidateVPNSubnets = []string{
	"10.10.0.0/24", "10.10.1.0/24", "10.20.0.0/24", "10.66.66.0/24",
	"10.100.0.0/24", "192.168.90.0/24", "192.168.91.0/24",
}

// SuggestVPNSubnet fetches the server's current routes/addresses and
// returns the first candidate subnet that doesn't overlap any of them,
// along with the list of candidates that were rejected and why (for
// display - SPEC.md §13.2: "показать выбранную подсеть и найденные
// потенциальные конфликты").
func SuggestVPNSubnet(ctx context.Context, client *sshtransport.Client) (chosen netip.Prefix, rejected map[string]string, err error) {
	existing, err := existingSubnets(ctx, client)
	if err != nil {
		return netip.Prefix{}, nil, err
	}

	rejected = map[string]string{}
	for _, cand := range candidateVPNSubnets {
		p := netip.MustParsePrefix(cand)
		if conflict, ok := firstOverlap(p, existing); ok {
			rejected[cand] = "overlaps " + conflict.String()
			continue
		}
		return p, rejected, nil
	}
	return netip.Prefix{}, rejected, fmt.Errorf("no candidate VPN subnet is free of conflicts on this server; all %d candidates overlap an existing route", len(candidateVPNSubnets))
}

// existingSubnets collects prefixes from `ip -o addr show` (interface
// addresses) and `ip route show` (routed subnets), both read-only and
// unprivileged.
func existingSubnets(ctx context.Context, client *sshtransport.Client) ([]netip.Prefix, error) {
	var out []netip.Prefix

	addrRes, err := client.RunCommand(ctx, "ip -o -4 addr show")
	if err != nil {
		return nil, fmt.Errorf("ip addr show: %w", err)
	}
	for _, line := range strings.Split(string(addrRes.Stdout), "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "inet" && i+1 < len(fields) {
				if p, err := netip.ParsePrefix(fields[i+1]); err == nil {
					out = append(out, p)
				}
			}
		}
	}

	routeRes, err := client.RunCommand(ctx, "ip -4 route show")
	if err != nil {
		return nil, fmt.Errorf("ip route show: %w", err)
	}
	for _, line := range strings.Split(string(routeRes.Stdout), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] == "default" {
			continue
		}
		if p, err := netip.ParsePrefix(fields[0]); err == nil {
			out = append(out, p)
		}
	}

	return out, nil
}

func firstOverlap(candidate netip.Prefix, existing []netip.Prefix) (netip.Prefix, bool) {
	for _, e := range existing {
		if candidate.Overlaps(e) {
			return e, true
		}
	}
	return netip.Prefix{}, false
}
