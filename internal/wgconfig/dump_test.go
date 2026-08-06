package wgconfig

import (
	"fmt"
	"strings"
	"testing"
)

const fakeInterfacePrivateKey = "ZHVtcFRlc3RQcml2YXRlS2V5U2hvdWxkTmV2ZXJMZWFrPT0="

func sampleDump() string {
	lines := []string{
		strings.Join([]string{"wg0", fakeInterfacePrivateKey, "wg0pubkeyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "51820", "off"}, "\t"),
		strings.Join([]string{"wg0", "peer1pubkeyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "(none)", "203.0.113.5:51820", "10.10.0.2/32", "1700000000", "1024", "2048", "25"}, "\t"),
		strings.Join([]string{"wg0", "peer2pubkeyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "psk2AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "(none)", "10.10.0.3/32", "0", "0", "0", "off"}, "\t"),
		strings.Join([]string{"wg1", "", "wg1pubkeyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", "51821", "12345"}, "\t"),
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestParseDump_Interfaces(t *testing.T) {
	d, err := ParseDump(sampleDump())
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if len(d.Interfaces) != 2 {
		t.Fatalf("got %d interfaces, want 2", len(d.Interfaces))
	}
	wg0 := d.Interfaces[0]
	if wg0.Name != "wg0" || wg0.PublicKey != "wg0pubkeyAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=" || wg0.ListenPort != 51820 || wg0.FwMark != "" {
		t.Errorf("unexpected wg0 interface: %+v", wg0)
	}
	wg1 := d.Interfaces[1]
	if wg1.Name != "wg1" || wg1.ListenPort != 51821 || wg1.FwMark != "12345" {
		t.Errorf("unexpected wg1 interface: %+v", wg1)
	}
}

func TestParseDump_Peers(t *testing.T) {
	d, err := ParseDump(sampleDump())
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	if len(d.Peers) != 2 {
		t.Fatalf("got %d peers, want 2", len(d.Peers))
	}

	p1 := d.Peers[0]
	if p1.PresharedKeyPresent {
		t.Errorf("peer1 PresharedKeyPresent = true, want false ((none) in fixture)")
	}
	if p1.Endpoint != "203.0.113.5:51820" {
		t.Errorf("peer1 endpoint = %q", p1.Endpoint)
	}
	if len(p1.AllowedIPs) != 1 || p1.AllowedIPs[0] != "10.10.0.2/32" {
		t.Errorf("peer1 allowed ips = %v", p1.AllowedIPs)
	}
	if p1.LatestHandshake.IsZero() || p1.LatestHandshake.Unix() != 1700000000 {
		t.Errorf("peer1 handshake = %v", p1.LatestHandshake)
	}
	if p1.ReceiveBytes != 1024 || p1.TransmitBytes != 2048 || p1.PersistentKeepalive != 25 {
		t.Errorf("peer1 counters wrong: %+v", p1)
	}

	p2 := d.Peers[1]
	if !p2.PresharedKeyPresent {
		t.Errorf("peer2 PresharedKeyPresent = false, want true")
	}
	if !p2.LatestHandshake.IsZero() {
		t.Errorf("peer2 should never have connected, got handshake %v", p2.LatestHandshake)
	}
	if p2.PersistentKeepalive != 0 {
		t.Errorf("peer2 keepalive = %d, want 0 (off)", p2.PersistentKeepalive)
	}
}

// TestParseDump_NeverExposesInterfacePrivateKey is a structural + behavioral
// guarantee: RuntimeInterface has no field that could hold the private key
// (compile-time), and this test additionally proves the parser actually
// read past a real-looking private key value without ever surfacing it in
// any string representation of the result.
func TestParseDump_NeverExposesInterfacePrivateKey(t *testing.T) {
	d, err := ParseDump(sampleDump())
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	rendered := fmt.Sprintf("%+v", d)
	if strings.Contains(rendered, fakeInterfacePrivateKey) {
		t.Fatalf("interface private key leaked into parsed result: %s", rendered)
	}
}

func TestParseDump_RejectsMalformedLine(t *testing.T) {
	_, err := ParseDump("wg0\tonly\tthree\n")
	if err == nil {
		t.Fatal("expected error for malformed line, got nil")
	}
}

func TestParseDump_EmptyInput(t *testing.T) {
	d, err := ParseDump("")
	if err != nil {
		t.Fatalf("ParseDump(\"\"): %v", err)
	}
	if len(d.Interfaces) != 0 || len(d.Peers) != 0 {
		t.Fatalf("expected empty Dump, got %+v", d)
	}
}
