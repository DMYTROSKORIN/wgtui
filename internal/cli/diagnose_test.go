package cli

import (
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
)

func findCheck(checks []checkResult, name string) (checkResult, bool) {
	for _, c := range checks {
		if c.Name == name {
			return c, true
		}
	}
	return checkResult{}, false
}

func TestBuildChecks_SudoUnavailable(t *testing.T) {
	checks := buildChecks(discovery.Report{System: discovery.SystemInfo{SudoAvailable: false}})
	c, ok := findCheck(checks, "sudo_available")
	if !ok || c.Status != checkFail {
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
	checks := buildChecks(r)
	c, ok := findCheck(checks, "saveconfig_risk:wg0")
	if !ok || c.Status != checkWarn {
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
	checks := buildChecks(r)
	c, ok := findCheck(checks, "runtime_persistent_drift:wg0")
	if !ok || c.Status != checkWarn {
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
	checks := buildChecks(r)
	c, ok := findCheck(checks, "runtime_persistent_drift:wg0")
	if !ok || c.Status != checkPass {
		t.Errorf("runtime_persistent_drift:wg0 = %+v, ok=%v, want status=pass", c, ok)
	}
}

func TestDuplicatePeerCheck(t *testing.T) {
	dup := discovery.InterfaceDetail{RuntimePeerPublicKeys: []string{"a", "b", "a"}}
	c := duplicatePeerCheck("wg0", dup)
	if c.Status != checkFail {
		t.Errorf("duplicate keys: status = %v, want fail", c.Status)
	}

	unique := discovery.InterfaceDetail{RuntimePeerPublicKeys: []string{"a", "b"}}
	c = duplicatePeerCheck("wg0", unique)
	if c.Status != checkPass {
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
	checks := buildChecks(r)
	c, ok := findCheck(checks, "listen_port:wg0")
	if !ok || c.Status != checkWarn {
		t.Errorf("listen_port:wg0 = %+v, ok=%v, want status=warn", c, ok)
	}
}
