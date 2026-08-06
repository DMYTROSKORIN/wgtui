package osadapter

import (
	"strings"
	"testing"
)

func TestPlanOpenPort(t *testing.T) {
	backends := []string{"ufw", "firewalld", "nftables", "iptables"}
	for _, b := range backends {
		plan, err := PlanOpenPort(b, 51820)
		if err != nil {
			t.Fatalf("PlanOpenPort(%s): %v", b, err)
		}
		if len(plan.Commands) == 0 {
			t.Fatalf("PlanOpenPort(%s) returned no commands", b)
		}
		if plan.Backend != b {
			t.Errorf("PlanOpenPort(%s).Backend = %q", b, plan.Backend)
		}
	}
}

func TestPlanOpenPort_NoBackend(t *testing.T) {
	if _, err := PlanOpenPort("", 51820); err == nil {
		t.Fatal("expected an error when no backend was detected")
	}
}

func TestPlanOpenPort_UnknownBackend(t *testing.T) {
	if _, err := PlanOpenPort("some-future-thing", 51820); err == nil {
		t.Fatal("expected an error for an unrecognized backend")
	}
}

func TestPlanOpenPort_NeverFlushes(t *testing.T) {
	dangerous := []string{" -F", "flush ruleset", "reset"}
	for _, b := range []string{"ufw", "firewalld", "nftables", "iptables"} {
		plan, _ := PlanOpenPort(b, 51820)
		for _, cmd := range plan.Commands {
			for _, d := range dangerous {
				if strings.Contains(cmd, d) {
					t.Errorf("backend %s command %q looks destructive (contains %q)", b, cmd, d)
				}
			}
		}
	}
}
