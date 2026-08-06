package osadapter

import (
	"strings"
	"testing"
)

func TestBuildRouterInstructions(t *testing.T) {
	r := BuildRouterInstructions(51820, "192.168.1.50")
	if r.Protocol != "UDP" || r.ExternalPort != 51820 || r.InternalPort != 51820 || r.InternalIP != "192.168.1.50" {
		t.Errorf("unexpected instructions: %+v", r)
	}
	if len(r.Warnings) == 0 {
		t.Error("expected CGNAT/double-NAT/DHCP warnings to be present")
	}
	str := r.String()
	for _, want := range []string{"UDP", "51820", "192.168.1.50"} {
		if !strings.Contains(str, want) {
			t.Errorf("String() missing %q: %s", want, str)
		}
	}
}
