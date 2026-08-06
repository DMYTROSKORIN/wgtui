package osadapter

import "testing"

func TestParseOSRelease(t *testing.T) {
	text := "NAME=\"Ubuntu\"\nID=ubuntu\nVERSION_ID=\"22.04\"\n"
	id, ver := parseOSRelease(text)
	if id != "ubuntu" || ver != "22.04" {
		t.Errorf("parseOSRelease() = (%q, %q)", id, ver)
	}
}

func TestPackageManagerFor(t *testing.T) {
	cases := map[string]string{"ubuntu": "apt", "debian": "apt", "fedora": "dnf", "arch": ""}
	for id, want := range cases {
		if got := packageManagerFor(id); got != want {
			t.Errorf("packageManagerFor(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestDistro_Supported(t *testing.T) {
	if !(Distro{PackageManager: "apt"}).Supported() {
		t.Error("apt distro should be Supported")
	}
	if (Distro{PackageManager: ""}).Supported() {
		t.Error("empty package manager should not be Supported")
	}
}
