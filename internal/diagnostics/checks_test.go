package diagnostics

import (
	"testing"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

func findCheck(checks []Check, name string) (Check, bool) {
	for _, c := range checks {
		if c.Name == name {
			return c, true
		}
	}
	return Check{}, false
}

func TestBuildChecks_SudoUnavailable(t *testing.T) {
	checks := BuildChecks(discovery.Report{System: discovery.SystemInfo{SudoAvailable: false}}, nil, time.Now())
	c, ok := findCheck(checks, "sudo_available")
	if !ok || c.Status != StatusFail {
		t.Errorf("sudo_available = %+v, ok=%v, want status=fail", c, ok)
	}
}

func TestBuildChecks_SaveConfigWarns(t *testing.T) {
	r := discovery.Report{
		System: discovery.SystemInfo{SudoAvailable: true},
		WireGuard: discovery.WireGuardInfo{
			WgPresent: true, WgQuickPresent: true,
			Interfaces: map[string]discovery.InterfaceDetail{
				"wg0": {Active: true, ConfigExists: true, SaveConfig: true},
			},
		},
	}
	checks := BuildChecks(r, nil, time.Now())
	c, ok := findCheck(checks, "saveconfig_risk:wg0")
	if !ok || c.Status != StatusWarn {
		t.Errorf("saveconfig_risk:wg0 = %+v, ok=%v, want status=warn", c, ok)
	}
}

func TestBuildChecks_DriftWarns(t *testing.T) {
	r := discovery.Report{
		System: discovery.SystemInfo{SudoAvailable: true},
		WireGuard: discovery.WireGuardInfo{
			WgPresent: true, WgQuickPresent: true,
			Interfaces: map[string]discovery.InterfaceDetail{
				"wg0": {Active: true, ConfigExists: true, Drift: true, RuntimePeerPublicKeys: []string{"a", "b"}, PersistentPeerPublicKeys: []string{"a"}},
			},
		},
	}
	checks := BuildChecks(r, nil, time.Now())
	c, ok := findCheck(checks, "runtime_persistent_drift:wg0")
	if !ok || c.Status != StatusWarn {
		t.Errorf("runtime_persistent_drift:wg0 = %+v, ok=%v, want status=warn", c, ok)
	}
}

func TestBuildChecks_NoDriftPasses(t *testing.T) {
	r := discovery.Report{
		System: discovery.SystemInfo{SudoAvailable: true},
		WireGuard: discovery.WireGuardInfo{
			WgPresent: true, WgQuickPresent: true,
			Interfaces: map[string]discovery.InterfaceDetail{
				"wg0": {Active: true, ConfigExists: true, Drift: false, RuntimePeerPublicKeys: []string{"a"}, PersistentPeerPublicKeys: []string{"a"}},
			},
		},
	}
	checks := BuildChecks(r, nil, time.Now())
	c, ok := findCheck(checks, "runtime_persistent_drift:wg0")
	if !ok || c.Status != StatusPass {
		t.Errorf("runtime_persistent_drift:wg0 = %+v, ok=%v, want status=pass", c, ok)
	}
}

func TestDuplicatePeerCheck(t *testing.T) {
	dup := discovery.InterfaceDetail{RuntimePeerPublicKeys: []string{"a", "b", "a"}}
	c := duplicatePeerCheck("wg0", dup)
	if c.Status != StatusFail {
		t.Errorf("duplicate keys: status = %v, want fail", c.Status)
	}

	unique := discovery.InterfaceDetail{RuntimePeerPublicKeys: []string{"a", "b"}}
	c = duplicatePeerCheck("wg0", unique)
	if c.Status != StatusPass {
		t.Errorf("unique keys: status = %v, want pass", c.Status)
	}
}

func TestBuildChecks_ListenPortMismatchWarns(t *testing.T) {
	r := discovery.Report{
		System: discovery.SystemInfo{SudoAvailable: true},
		WireGuard: discovery.WireGuardInfo{
			WgPresent: true, WgQuickPresent: true,
			Interfaces: map[string]discovery.InterfaceDetail{
				"wg0": {Active: true, RuntimeListenPort: 51821, ConfigListenPort: 51820},
			},
		},
	}
	checks := BuildChecks(r, nil, time.Now())
	c, ok := findCheck(checks, "listen_port:wg0")
	if !ok || c.Status != StatusWarn {
		t.Errorf("listen_port:wg0 = %+v, ok=%v, want status=warn", c, ok)
	}
}

func TestBuildChecks_OldKeyWarns(t *testing.T) {
	now := time.Now()
	local := []profilevault.PeerMetadata{
		{Name: "Alex", PublicKey: "pub1", Status: profilevault.PeerStatusActive, CreatedAt: now.Add(-200 * 24 * time.Hour)},
	}
	checks := BuildChecks(discovery.Report{System: discovery.SystemInfo{SudoAvailable: true}}, local, now)
	c, ok := findCheck(checks, "peer_key_age:Alex")
	if !ok || c.Status != StatusWarn {
		t.Errorf("peer_key_age:Alex = %+v, ok=%v, want status=warn", c, ok)
	}
}

func TestBuildChecks_RecentKeyNoWarning(t *testing.T) {
	now := time.Now()
	local := []profilevault.PeerMetadata{
		{Name: "Alex", PublicKey: "pub1", Status: profilevault.PeerStatusActive, CreatedAt: now.Add(-10 * 24 * time.Hour)},
	}
	checks := BuildChecks(discovery.Report{System: discovery.SystemInfo{SudoAvailable: true}}, local, now)
	if _, ok := findCheck(checks, "peer_key_age:Alex"); ok {
		t.Error("expected no peer_key_age check for a 10-day-old key")
	}
}

func TestBuildChecks_KeyAgeSkipsDisabledAndRevoked(t *testing.T) {
	now := time.Now()
	local := []profilevault.PeerMetadata{
		{Name: "Paused", PublicKey: "pub1", Status: profilevault.PeerStatusDisabled, CreatedAt: now.Add(-400 * 24 * time.Hour)},
		{Name: "Gone", PublicKey: "pub2", Status: profilevault.PeerStatusRevoked, CreatedAt: now.Add(-400 * 24 * time.Hour)},
	}
	checks := BuildChecks(discovery.Report{System: discovery.SystemInfo{SudoAvailable: true}}, local, now)
	if _, ok := findCheck(checks, "peer_key_age:Paused"); ok {
		t.Error("disabled peers should not get a key-age warning")
	}
	if _, ok := findCheck(checks, "peer_key_age:Gone"); ok {
		t.Error("revoked peers should not get a key-age warning")
	}
}
