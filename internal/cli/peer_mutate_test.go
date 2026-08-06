package cli

import (
	"reflect"
	"testing"
)

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
		if got := splitTrim(tc.in, ","); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("splitTrim(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestTunnelModeLabel(t *testing.T) {
	if got := tunnelModeLabel(true); got != "full tunnel" {
		t.Errorf("tunnelModeLabel(true) = %q", got)
	}
	if got := tunnelModeLabel(false); got != "split tunnel" {
		t.Errorf("tunnelModeLabel(false) = %q", got)
	}
}

func TestOutputDescription(t *testing.T) {
	if got := outputDescription(""); got != "no file export" {
		t.Errorf("outputDescription(\"\") = %q", got)
	}
	if got := outputDescription("/tmp/x.conf"); got != "/tmp/x.conf" {
		t.Errorf("outputDescription(path) = %q", got)
	}
}

func TestRotateProfileParams_NoOldProfile(t *testing.T) {
	addr, host, port, allowedIPs, dns := rotateProfileParams("", false, []string{"10.10.0.5/32"}, "example.com", 51820)
	if addr.String() != "10.10.0.5" {
		t.Errorf("addr = %s, want 10.10.0.5", addr)
	}
	if host != "example.com" || port != 51820 {
		t.Errorf("host/port = %s:%d, want example.com:51820", host, port)
	}
	if len(allowedIPs) != 2 || allowedIPs[0] != "0.0.0.0/0" {
		t.Errorf("allowedIPs = %v, want full-tunnel default", allowedIPs)
	}
	if dns != nil {
		t.Errorf("dns = %v, want nil", dns)
	}
}

func TestRotateProfileParams_ReusesOldProfileFields(t *testing.T) {
	oldProfile := "[Interface]\nPrivateKey = old\nAddress = 10.10.0.5/32\nDNS = 1.1.1.1, 1.0.0.1\n\n[Peer]\nPublicKey = server\nEndpoint = old-host.example:12345\nAllowedIPs = 192.168.1.0/24\n"
	addr, host, port, allowedIPs, dns := rotateProfileParams(oldProfile, true, []string{"10.10.0.5/32"}, "fallback.example", 51820)
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
