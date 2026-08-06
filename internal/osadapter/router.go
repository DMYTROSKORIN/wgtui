package osadapter

import "fmt"

// RouterInstructions is the SPEC.md §13.8 port-forward guidance: wgtui
// cannot configure an external router itself, so it generates the exact
// values a human needs to do it, plus the warnings that make the
// instructions honest about what they can't guarantee.
type RouterInstructions struct {
	Protocol     string
	ExternalPort int
	InternalIP   string
	InternalPort int
	Warnings     []string
}

// BuildRouterInstructions never touches the network - pure formatting.
func BuildRouterInstructions(listenPort int, serverLANIP string) RouterInstructions {
	return RouterInstructions{
		Protocol:     "UDP",
		ExternalPort: listenPort,
		InternalIP:   serverLANIP,
		InternalPort: listenPort,
		Warnings: []string{
			"CGNAT: if the WAN IP shown by your ISP doesn't match this server's public IP, port forwarding will not work - you're likely behind Carrier-Grade NAT and need a different approach (e.g. a VPS relay).",
			"Double NAT: if there is more than one router between this server and the internet (e.g. an ISP modem/router plus your own router), port forwarding must be configured on all of them.",
			"Use a DHCP reservation or a static LAN IP for this server - if its LAN IP changes, the router's port forward will silently start pointing at the wrong machine.",
		},
	}
}

func (r RouterInstructions) String() string {
	return fmt.Sprintf(
		"Protocol: %s\nExternal port: %d\nInternal IP: %s\nInternal port: %d",
		r.Protocol, r.ExternalPort, r.InternalIP, r.InternalPort,
	)
}
