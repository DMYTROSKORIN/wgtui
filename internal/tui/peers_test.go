package tui

import (
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

// Regression test: startPeerForm built the form but initially forgot to
// set m.screen, so pressing 'a' on the Peers tab silently did nothing -
// only caught by manually driving the TUI in a real terminal, not by
// go build/vet/test.
func TestPeerFormKey_SwitchesScreen(t *testing.T) {
	m := testModel(t, domain.Server{ID: "a"})
	m.Update(key("enter"))
	m.screen = screenServerDetail
	m.tab = tabPeers

	res, cmd := m.updateServerDetail(key("a"))
	rm := res.(*model)
	if rm.screen != screenPeerForm {
		t.Errorf("screen = %v, want screenPeerForm", rm.screen)
	}
	if rm.peerForm == nil {
		t.Error("expected peerForm to be built")
	}
	if cmd == nil {
		t.Error("expected peerForm.Init() to be returned")
	}
}

func TestResolvePeerInterface_PrefersMetadata(t *testing.T) {
	// metaInterface set - must return immediately without touching sess
	// (which is nil here; a network fallback would nil-panic).
	got, err := resolvePeerInterface(nil, nil, "wg1", "wg0")
	if err != nil {
		t.Fatalf("resolvePeerInterface: %v", err)
	}
	if got != "wg1" {
		t.Errorf("got %q, want wg1 (metadata should win over the flag)", got)
	}
}

func TestValidateOptionalPort(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
	}{
		{"", false},
		{"22", false},
		{"65535", false},
		{"0", true},
		{"70000", true},
		{"not-a-number", true},
	}
	for _, tc := range cases {
		err := validateOptionalPort(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("validateOptionalPort(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
		}
	}
}
