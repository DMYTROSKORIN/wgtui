package osadapter

import "testing"

func TestPlanInstall(t *testing.T) {
	apt, err := PlanInstall(Distro{ID: "ubuntu", PackageManager: "apt"})
	if err != nil {
		t.Fatalf("PlanInstall(ubuntu): %v", err)
	}
	if len(apt.Commands) == 0 {
		t.Fatal("expected non-empty apt install commands")
	}

	dnf, err := PlanInstall(Distro{ID: "fedora", PackageManager: "dnf"})
	if err != nil {
		t.Fatalf("PlanInstall(fedora): %v", err)
	}
	if len(dnf.Commands) == 0 {
		t.Fatal("expected non-empty dnf install commands")
	}

	if _, err := PlanInstall(Distro{ID: "arch", PackageManager: ""}); err == nil {
		t.Fatal("expected an error for an unsupported distro, got nil")
	}
}
