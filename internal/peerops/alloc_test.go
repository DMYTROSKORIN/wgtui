package peerops

import (
	"net/netip"
	"testing"
)

func TestNextFreeAddress(t *testing.T) {
	cidr := netip.MustParsePrefix("10.10.0.0/24")

	// Empty subnet: first usable address is .1 (network .0 is skipped).
	addr, err := NextFreeAddress(cidr, nil)
	if err != nil {
		t.Fatalf("NextFreeAddress: %v", err)
	}
	if addr.String() != "10.10.0.1" {
		t.Errorf("first free address = %s, want 10.10.0.1", addr)
	}

	// .1 used -> next is .2.
	used := map[netip.Addr]bool{netip.MustParseAddr("10.10.0.1"): true}
	addr, err = NextFreeAddress(cidr, used)
	if err != nil {
		t.Fatalf("NextFreeAddress: %v", err)
	}
	if addr.String() != "10.10.0.2" {
		t.Errorf("next free address = %s, want 10.10.0.2", addr)
	}
}

func TestNextFreeAddress_ExhaustedSubnet(t *testing.T) {
	cidr := netip.MustParsePrefix("10.10.0.0/30") // usable: .1, .2 only (.0 network, .3 broadcast)
	used := map[netip.Addr]bool{
		netip.MustParseAddr("10.10.0.1"): true,
		netip.MustParseAddr("10.10.0.2"): true,
	}
	if _, err := NextFreeAddress(cidr, used); err == nil {
		t.Fatal("expected an error for an exhausted /30 subnet, got nil")
	}
}

func TestNextFreeAddress_NeverReturnsNetworkOrBroadcast(t *testing.T) {
	cidr := netip.MustParsePrefix("192.168.88.0/29") // .0 network, .7 broadcast, usable .1-.6
	seen := map[netip.Addr]bool{}
	used := map[netip.Addr]bool{}
	for i := 0; i < 6; i++ {
		addr, err := NextFreeAddress(cidr, used)
		if err != nil {
			t.Fatalf("NextFreeAddress iteration %d: %v", i, err)
		}
		if addr.String() == "192.168.88.0" || addr.String() == "192.168.88.7" {
			t.Fatalf("allocated network/broadcast address: %s", addr)
		}
		if seen[addr] {
			t.Fatalf("allocated the same address twice: %s", addr)
		}
		seen[addr] = true
		used[addr] = true
	}
	if _, err := NextFreeAddress(cidr, used); err == nil {
		t.Fatal("expected exhaustion error after allocating all 6 usable addresses")
	}
}

func TestNextFreeAddress_InvalidCIDR(t *testing.T) {
	if _, err := NextFreeAddress(netip.Prefix{}, nil); err == nil {
		t.Fatal("expected an error for an invalid/zero-value prefix")
	}
}
