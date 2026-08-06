package osadapter

import (
	"net/netip"
	"testing"
)

func TestFirstOverlap(t *testing.T) {
	existing := []netip.Prefix{
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("172.16.0.0/16"),
	}
	if _, ok := firstOverlap(netip.MustParsePrefix("10.10.0.0/24"), existing); ok {
		t.Error("10.10.0.0/24 should not overlap the existing set")
	}
	if _, ok := firstOverlap(netip.MustParsePrefix("192.168.1.128/25"), existing); !ok {
		t.Error("192.168.1.128/25 should overlap 192.168.1.0/24")
	}
	if _, ok := firstOverlap(netip.MustParsePrefix("172.16.5.0/24"), existing); !ok {
		t.Error("172.16.5.0/24 should overlap 172.16.0.0/16")
	}
}

func TestCandidateVPNSubnets_AreValidAndDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range candidateVPNSubnets {
		p, err := netip.ParsePrefix(c)
		if err != nil {
			t.Errorf("candidate %q does not parse: %v", c, err)
		}
		if seen[p.String()] {
			t.Errorf("duplicate candidate %q", c)
		}
		seen[p.String()] = true
	}
}
