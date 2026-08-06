package discovery

import (
	"reflect"
	"testing"
)

func TestParseOSRelease(t *testing.T) {
	text := "NAME=\"Ubuntu\"\nID=ubuntu\nID_LIKE=debian\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04.3 LTS\"\n"
	id, ver := parseOSRelease(text)
	if id != "ubuntu" || ver != "22.04" {
		t.Errorf("parseOSRelease() = (%q, %q), want (ubuntu, 22.04)", id, ver)
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

func TestIsValidInterfaceName(t *testing.T) {
	valid := []string{"wg0", "wg-office", "wg_1", "a"}
	invalid := []string{"", "wg0; rm -rf /", "wg0 && echo pwned", "../../etc/passwd", "toolongtoolongtoolong0"}
	for _, n := range valid {
		if !isValidInterfaceName(n) {
			t.Errorf("isValidInterfaceName(%q) = false, want true", n)
		}
	}
	for _, n := range invalid {
		if isValidInterfaceName(n) {
			t.Errorf("isValidInterfaceName(%q) = true, want false", n)
		}
	}
}

func TestParseDefaultRouteInterface(t *testing.T) {
	out := "default via 192.168.1.1 dev wlp4s0 proto dhcp metric 600\n"
	if got := parseDefaultRouteInterface(out); got != "wlp4s0" {
		t.Errorf("parseDefaultRouteInterface() = %q, want wlp4s0", got)
	}
	if got := parseDefaultRouteInterface(""); got != "" {
		t.Errorf("parseDefaultRouteInterface(\"\") = %q, want \"\"", got)
	}
}

func TestParseListeningUDPPorts(t *testing.T) {
	out := "Netid State  Recv-Q Send-Q Local Address:Port   Peer Address:Port\n" +
		"udp   UNCONN 0      0            0.0.0.0:51820        0.0.0.0:*\n" +
		"udp   UNCONN 0      0               [::]:68              [::]:*\n" +
		"udp   UNCONN 0      0            0.0.0.0:51820        0.0.0.0:*\n" // duplicate on purpose
	got := parseListeningUDPPorts(out)
	want := []int{51820, 68}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListeningUDPPorts() = %v, want %v", got, want)
	}
}

// TestParseListeningUDPPorts_NoNetidColumn locks in a real-world format
// difference found running against nas.paynet.one:2626: this system's `ss
// -lun` prints no leading "Netid" column at all, which broke a
// fixed-column-index parser.
func TestParseListeningUDPPorts_NoNetidColumn(t *testing.T) {
	out := "State  Recv-Q Send-Q        Local Address:Port  Peer Address:PortProcess\n" +
		"UNCONN 0      0                   0.0.0.0:1900       0.0.0.0:*          \n" +
		"UNCONN 0      0                   0.0.0.0:51820      0.0.0.0:*          \n" +
		"UNCONN 0      0             127.0.0.53%lo:53         0.0.0.0:*          \n"
	got := parseListeningUDPPorts(out)
	want := []int{1900, 51820, 53}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseListeningUDPPorts() = %v, want %v", got, want)
	}
}
