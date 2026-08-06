package discovery

import (
	"context"
	"strconv"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

func discoverNetwork(ctx context.Context, client *sshtransport.Client, r *Report) {
	if res, err := client.RunCommand(ctx, "ip route show default"); err == nil && res.ExitCode == 0 {
		r.Network.DefaultOutboundInterface = parseDefaultRouteInterface(string(res.Stdout))
	} else {
		r.Errors = append(r.Errors, checkErr("ip route show default", res, err))
	}

	if res, err := client.RunCommand(ctx, "cat /proc/sys/net/ipv4/ip_forward"); err == nil && res.ExitCode == 0 {
		r.Network.IPv4Forwarding = strings.TrimSpace(string(res.Stdout)) == "1"
	}
	if res, err := client.RunCommand(ctx, "cat /proc/sys/net/ipv6/conf/all/forwarding"); err == nil && res.ExitCode == 0 {
		r.Network.IPv6Forwarding = strings.TrimSpace(string(res.Stdout)) == "1"
	}

	if res, err := client.RunCommand(ctx, "ss -lun"); err == nil && res.ExitCode == 0 {
		r.Network.ListeningUDPPorts = parseListeningUDPPorts(string(res.Stdout))
	} else {
		r.Errors = append(r.Errors, checkErr("ss -lun", res, err))
	}

	r.Network.FirewallBackend = detectFirewallBackend(ctx, client)
}

func parseDefaultRouteInterface(out string) string {
	fields := strings.Fields(out)
	for i, f := range fields {
		if f == "dev" && i+1 < len(fields) {
			return fields[i+1]
		}
	}
	return ""
}

// parseListeningUDPPorts parses `ss -lun` output. It is tolerant of the
// header line being present or absent and only trusts fields that parse
// as a plain integer port.
func parseListeningUDPPorts(out string) []int {
	var ports []int
	seen := make(map[int]bool)
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		local := fields[4]
		idx := strings.LastIndex(local, ":")
		if idx == -1 {
			continue
		}
		port, err := strconv.Atoi(local[idx+1:])
		if err != nil {
			continue
		}
		if !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	return ports
}

func detectFirewallBackend(ctx context.Context, client *sshtransport.Client) string {
	if isActiveService(ctx, client, "ufw") {
		return "ufw"
	}
	if isActiveService(ctx, client, "firewalld") {
		return "firewalld"
	}
	if commandExists(ctx, client, "nft") {
		return "nftables"
	}
	if commandExists(ctx, client, "iptables") {
		return "iptables"
	}
	return ""
}

func isActiveService(ctx context.Context, client *sshtransport.Client, name string) bool {
	res, err := client.RunCommand(ctx, "systemctl is-active "+name)
	return err == nil && strings.TrimSpace(string(res.Stdout)) == "active"
}

func commandExists(ctx context.Context, client *sshtransport.Client, name string) bool {
	res, err := client.RunCommand(ctx, "command -v "+name)
	return err == nil && res.ExitCode == 0
}
