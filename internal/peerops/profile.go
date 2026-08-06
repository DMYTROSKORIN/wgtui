package peerops

import (
	"fmt"
	"net/netip"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

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
