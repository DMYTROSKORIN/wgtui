package peerops

import (
	"net/netip"
	"reflect"
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

func TestRenderQRPNG(t *testing.T) {
	png, err := RenderQRPNG("[Interface]\nPrivateKey = x\n")
	if err != nil {
		t.Fatalf("RenderQRPNG: %v", err)
	}
	if len(png) < 8 || string(png[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Fatalf("result is not a PNG: %x", png[:min(len(png), 8)])
	}
}

func TestSplitTrim(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"a, b,c", []string{"a", "b", "c"}},
		{"", nil},
		{"  ", nil},
		{"only-one", []string{"only-one"}},
	}
	for _, tc := range cases {
		if got := SplitTrim(tc.in, ","); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("SplitTrim(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestRotateProfileParams_NoOldProfile(t *testing.T) {
	addr, host, port, allowedIPs, dns := RotateProfileParams("", false, []string{"10.10.0.5/32"}, "example.com", 51820)
	if addr.String() != "10.10.0.5" {
		t.Errorf("addr = %s, want 10.10.0.5", addr)
	}
	if host != "example.com" || port != 51820 {
		t.Errorf("host/port = %s:%d, want example.com:51820", host, port)
	}
	if len(allowedIPs) != 2 || allowedIPs[0] != "0.0.0.0/0" {
		t.Errorf("allowedIPs = %v, want full-tunnel default", allowedIPs)
	}
	if !reflect.DeepEqual(dns, DefaultDNS()) {
		t.Errorf("dns = %v, want %v", dns, DefaultDNS())
	}
}

func TestRotateProfileParams_OldProfileWithoutDNSGetsDefault(t *testing.T) {
	oldProfile := "[Interface]\nPrivateKey = old\nAddress = 10.10.0.5/32\n\n[Peer]\nPublicKey = server\nEndpoint = example.com:51820\nAllowedIPs = 0.0.0.0/0, ::/0\n"
	_, _, _, _, dns := RotateProfileParams(oldProfile, true, []string{"10.10.0.5/32"}, "fallback.example", 51820)
	if !reflect.DeepEqual(dns, DefaultDNS()) {
		t.Errorf("dns = %v, want %v", dns, DefaultDNS())
	}
}

func TestRotateProfileParams_ReusesOldProfileFields(t *testing.T) {
	oldProfile := "[Interface]\nPrivateKey = old\nAddress = 10.10.0.5/32\nDNS = 1.1.1.1, 1.0.0.1\n\n[Peer]\nPublicKey = server\nEndpoint = old-host.example:12345\nAllowedIPs = 192.168.1.0/24\n"
	addr, host, port, allowedIPs, dns := RotateProfileParams(oldProfile, true, []string{"10.10.0.5/32"}, "fallback.example", 51820)
	if addr.String() != "10.10.0.5" {
		t.Errorf("addr = %s", addr)
	}
	if host != "old-host.example" || port != 12345 {
		t.Errorf("host/port = %s:%d, want old-host.example:12345", host, port)
	}
	if !reflect.DeepEqual(allowedIPs, []string{"192.168.1.0/24"}) {
		t.Errorf("allowedIPs = %v", allowedIPs)
	}
	if !reflect.DeepEqual(dns, []string{"1.1.1.1", "1.0.0.1"}) {
		t.Errorf("dns = %v", dns)
	}
}
