package peerops

import (
	"net/netip"
	"strings"
	"testing"
)

func TestBuildClientProfile(t *testing.T) {
	p := ClientProfileParams{
		ClientPrivateKey:    "CLIENT_PRIVATE_KEY_MARKER",
		ClientAddress:       netip.MustParseAddr("10.10.0.9"),
		DNS:                 []string{"1.1.1.1"},
		ServerPublicKey:     "SERVER_PUBLIC_KEY_MARKER",
		PresharedKey:        "PSK_MARKER",
		EndpointHost:        "nas.paynet.one",
		EndpointPort:        51820,
		AllowedIPs:          []string{"0.0.0.0/0", "::/0"},
		PersistentKeepalive: 25,
	}
	conf := BuildClientProfile(p)

	mustContain := []string{
		"[Interface]",
		"PrivateKey = CLIENT_PRIVATE_KEY_MARKER",
		"Address = 10.10.0.9/32",
		"DNS = 1.1.1.1",
		"[Peer]",
		"PublicKey = SERVER_PUBLIC_KEY_MARKER",
		"PresharedKey = PSK_MARKER",
		"Endpoint = nas.paynet.one:51820",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	}
	for _, want := range mustContain {
		if !strings.Contains(conf, want) {
			t.Errorf("client profile missing %q:\n%s", want, conf)
		}
	}
}

func TestBuildClientProfile_OmitsOptionalFields(t *testing.T) {
	conf := BuildClientProfile(ClientProfileParams{
		ClientPrivateKey: "K",
		ClientAddress:    netip.MustParseAddr("10.10.0.2"),
		ServerPublicKey:  "S",
		EndpointHost:     "h",
		EndpointPort:     51820,
		AllowedIPs:       []string{"10.10.0.0/24"},
	})
	for _, notWant := range []string{"DNS =", "PresharedKey =", "PersistentKeepalive ="} {
		if strings.Contains(conf, notWant) {
			t.Errorf("client profile should omit unset field %q:\n%s", notWant, conf)
		}
	}
}

func TestRenderQRTerminal(t *testing.T) {
	art, err := RenderQRTerminal("[Interface]\nPrivateKey = x\n")
	if err != nil {
		t.Fatalf("RenderQRTerminal: %v", err)
	}
	if len(art) == 0 {
		t.Fatal("expected non-empty QR terminal art")
	}
}
