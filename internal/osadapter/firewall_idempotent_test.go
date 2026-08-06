package osadapter

import "testing"

func TestPortLooksOpen(t *testing.T) {
	cases := []struct {
		backend, output, port string
		want                  bool
	}{
		{"ufw", "Status: active\n\nTo                         Action      From\n--                         ------      ----\n51820/udp                  ALLOW       Anywhere\n", "51820", true},
		{"ufw", "Status: active\n\n22/tcp                     ALLOW       Anywhere\n", "51820", false},
		{"firewalld", "51820/udp 8080/tcp\n", "51820", true},
		{"firewalld", "8080/tcp\n", "51820", false},
		{"firewalld", "151820/udp\n", "51820", false}, // must not substring-match a longer port
		{"nftables", "table inet wgtui {\n\tchain input {\n\t\tudp dport 51820 accept\n\t}\n}\n", "51820", true},
		{"nftables", "table inet wgtui {\n\tchain input {\n\t\tudp dport 51821 accept\n\t}\n}\n", "51820", false},
		{"iptables", "-P INPUT ACCEPT\n-A INPUT -p udp -m udp --dport 51820 -j ACCEPT\n", "51820", true},
		{"iptables", "-P INPUT ACCEPT\n", "51820", false},
	}
	for _, tc := range cases {
		if got := portLooksOpen(tc.backend, tc.output, tc.port); got != tc.want {
			t.Errorf("portLooksOpen(%s, %q, %s) = %v, want %v", tc.backend, tc.output, tc.port, got, tc.want)
		}
	}
}

func TestStatusCommandFor(t *testing.T) {
	for _, b := range []string{"ufw", "firewalld", "nftables", "iptables"} {
		if _, ok := statusCommandFor(b); !ok {
			t.Errorf("statusCommandFor(%s) = false, want true", b)
		}
	}
	if _, ok := statusCommandFor("something-else"); ok {
		t.Error("statusCommandFor(unknown) = true, want false")
	}
}
