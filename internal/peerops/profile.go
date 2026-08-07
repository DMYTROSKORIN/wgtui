package peerops

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// DefaultDNSCSV is written into newly generated client profiles unless a
// caller supplies a different list. Full-tunnel clients must not depend on
// DNS learned from the physical network after their default route moves into
// WireGuard.
const DefaultDNSCSV = "8.8.8.8, 8.8.4.4"

// DefaultDNS returns a fresh slice so callers cannot mutate shared defaults.
func DefaultDNS() []string { return []string{"8.8.8.8", "8.8.4.4"} }

// ClientProfileParams holds everything needed to render a client-side
// .conf - this is the file the *client* imports into their own WireGuard
// app, distinct from the server-side [Peer] block wgconfig.BuildPeerBlock
// produces (SPEC.md §7.1: assembled entirely locally, never fetched from
// the server).
type ClientProfileParams struct {
	ClientPrivateKey    string
	ClientAddress       netip.Addr
	DNS                 []string
	ServerPublicKey     string
	PresharedKey        string // "" to omit
	EndpointHost        string
	EndpointPort        int
	AllowedIPs          []string
	PersistentKeepalive int
}

// BuildClientProfile renders a ready-to-import wg-quick config for a
// client. Every value here either came from local key generation or from
// the server's own already-verified public key/listen port - nothing
// user-typed reaches this without already having passed
// wgconfig.Validate* at the point it was collected (peer add's caller is
// responsible for that before constructing ClientProfileParams).
func BuildClientProfile(p ClientProfileParams) string {
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", p.ClientPrivateKey)
	fmt.Fprintf(&b, "Address = %s/32\n", p.ClientAddress)
	if len(p.DNS) > 0 {
		fmt.Fprintf(&b, "DNS = %s\n", strings.Join(p.DNS, ", "))
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", p.ServerPublicKey)
	if p.PresharedKey != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", p.PresharedKey)
	}
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", p.EndpointHost, p.EndpointPort)
	fmt.Fprintf(&b, "AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", "))
	if p.PersistentKeepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", p.PersistentKeepalive)
	}
	return b.String()
}

// RenderQRTerminal renders confText as a QR code using half-block Unicode
// characters for compact terminal display (SPEC.md §7.4: generated
// entirely locally, no qrencode dependency). Callers must warn the user
// this is a secret before printing it and must never write the result to
// a log (§7.4, §23.3).
func RenderQRTerminal(confText string) (string, error) {
	qr, err := qrcode.New(confText, qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("encode QR code: %w", err)
	}
	return qr.ToSmallString(false), nil
}

// RotateProfileParams recovers the client-facing profile parameters
// (endpoint, client AllowedIPs, DNS) that live only in the *client*
// profile, not the server-side block, by re-parsing the peer's previous
// profile if one was stored. Falls back to sane defaults (full tunnel,
// Google DNS, endpoint = the given default host) if there wasn't one - e.g.
// rotating a peer that was imported without ever having a local profile.
// Shared by cli and mcpserver, which both drive peer rotate.
func RotateProfileParams(oldProfile string, had bool, serverAllowedIPs []string, defaultHost string, defaultPort int) (addr netip.Addr, host string, port int, clientAllowedIPs, dns []string) {
	host, port = defaultHost, defaultPort
	clientAllowedIPs = []string{"0.0.0.0/0", "::/0"}
	if len(serverAllowedIPs) > 0 {
		if prefix, err := netip.ParsePrefix(serverAllowedIPs[0]); err == nil {
			addr = prefix.Addr()
		}
	}
	if !had {
		return addr, host, port, clientAllowedIPs, DefaultDNS()
	}
	doc := wgconfig.Parse(oldProfile)
	if peerBlk, ok := doc.FirstPeerBlock(); ok {
		if ep := peerBlk.Value("Endpoint"); ep != "" {
			if h, p, err := net.SplitHostPort(ep); err == nil {
				host = h
				if pn, err := strconv.Atoi(p); err == nil {
					port = pn
				}
			}
		}
		if aips := SplitTrim(peerBlk.Value("AllowedIPs"), ","); len(aips) > 0 {
			clientAllowedIPs = aips
		}
	}
	if ifaceBlk, ok := doc.InterfaceBlock(); ok {
		dns = SplitTrim(ifaceBlk.Value("DNS"), ",")
	}
	if len(dns) == 0 {
		dns = DefaultDNS()
	}
	return addr, host, port, clientAllowedIPs, dns
}

// SplitTrim splits s on sep, trims whitespace from each part, and drops
// empty parts.
func SplitTrim(s, sep string) []string {
	var out []string
	for _, part := range strings.Split(s, sep) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
