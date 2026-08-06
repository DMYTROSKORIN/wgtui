package wgconfig

import (
	"reflect"
	"testing"
)

func TestSummarize_MultiPeerSaveConfig(t *testing.T) {
	raw := readFixture(t, "multi_peer_saveconfig.conf")
	s := Summarize(raw)

	if s.ListenPort != 51820 {
		t.Errorf("ListenPort = %d, want 51820", s.ListenPort)
	}
	if !reflect.DeepEqual(s.Addresses, []string{"10.10.0.1/24"}) {
		t.Errorf("Addresses = %v", s.Addresses)
	}
	if !s.SaveConfig {
		t.Error("SaveConfig = false, want true")
	}
	if !s.HasPostUp || !s.HasPostDown {
		t.Errorf("HasPostUp=%v HasPostDown=%v, want both true", s.HasPostUp, s.HasPostDown)
	}
	wantKeys := []string{
		"cGVlcjFwdWJsaWNrZXlmYWtlZmFrZWZha2VmYWtlZmFrZQ==",
		"cGVlcjJwdWJsaWNrZXlmYWtlZmFrZWZha2VmYWtlZmFrZQ==",
	}
	if !reflect.DeepEqual(s.PeerPublicKeys, wantKeys) {
		t.Errorf("PeerPublicKeys = %v, want %v", s.PeerPublicKeys, wantKeys)
	}
}

func TestSummarize_MinimalNoSaveConfig(t *testing.T) {
	raw := readFixture(t, "minimal.conf")
	s := Summarize(raw)
	if s.SaveConfig {
		t.Error("SaveConfig = true, want false (not set in fixture)")
	}
	if s.HasPostUp || s.HasPostDown {
		t.Error("expected no PostUp/PostDown in minimal fixture")
	}
	if len(s.PeerPublicKeys) != 1 {
		t.Errorf("got %d peer keys, want 1", len(s.PeerPublicKeys))
	}
}

func TestSummarize_NeverExposesPrivateKey(t *testing.T) {
	raw := readFixture(t, "minimal.conf")
	s := Summarize(raw)
	rendered := reflect.TypeOf(s)
	for i := 0; i < rendered.NumField(); i++ {
		name := rendered.Field(i).Name
		if name == "PrivateKey" {
			t.Fatal("ConfigSummary must never gain a PrivateKey field")
		}
	}
}
