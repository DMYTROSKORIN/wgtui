package domain

import (
	"net/netip"
	"time"
)

// Server is the minimal server record from SPEC.md §9.1. It never carries
// the server private key — only the public key, which the server itself
// hands back after setup/discovery.
type Server struct {
	ID                    string
	DisplayName           string
	SSHHostOrAlias        string
	SSHUser               string
	SSHPort               int
	SSHIdentityFile       string // path only, never the key itself
	WireGuardInterface    string
	WireGuardConfigPath   string
	EndpointHost          string
	EndpointPort          int
	ServerPublicKey       string
	VPNIPv4CIDR           netip.Prefix
	VPNIPv6CIDR           netip.Prefix // zero value if unset
	OutboundInterface     string
	FirewallBackend       string
	ServiceManager        string
	InstallType           string
	ProfileDefaultsDNS    []string
	ProfileDefaultsFull   bool // true = full tunnel is this server's default
	LastSeen              time.Time
}
