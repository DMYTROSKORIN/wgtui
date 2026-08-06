package discovery

import "testing"

func TestClassify(t *testing.T) {
	baseSystem := SystemInfo{SudoAvailable: true, OSID: "ubuntu", PackageManager: "apt"}

	cases := []struct {
		name string
		r    Report
		want State
	}{
		{
			name: "insufficient privileges wins over everything else",
			r:    Report{System: SystemInfo{SudoAvailable: false}},
			want: StateInsufficientPrivileges,
		},
		{
			name: "wireguard missing",
			r:    Report{System: baseSystem, WireGuard: WireGuardInfo{WgPresent: false}},
			want: StateWireGuardMissing,
		},
		{
			name: "ambiguous distro",
			r:    Report{System: SystemInfo{SudoAvailable: true}, WireGuard: WireGuardInfo{WgPresent: true}},
			want: StateAmbiguous,
		},
		{
			name: "tools installed, no interfaces",
			r: Report{
				System:    baseSystem,
				WireGuard: WireGuardInfo{WgPresent: true, Interfaces: map[string]InterfaceDetail{}},
			},
			want: StateToolsNoInterfaces,
		},
		{
			name: "config exists, interface stopped",
			r: Report{
				System: baseSystem,
				WireGuard: WireGuardInfo{
					WgPresent: true,
					Interfaces: map[string]InterfaceDetail{
						"wg0": {Active: false, ConfigExists: true},
					},
				},
			},
			want: StateConfigExistsStopped,
		},
		{
			name: "active and consistent",
			r: Report{
				System: baseSystem,
				WireGuard: WireGuardInfo{
					WgPresent: true,
					Interfaces: map[string]InterfaceDetail{
						"wg0": {Active: true, Drift: false},
					},
				},
			},
			want: StateActiveConsistent,
		},
		{
			name: "drift detected",
			r: Report{
				System: baseSystem,
				WireGuard: WireGuardInfo{
					WgPresent: true,
					Interfaces: map[string]InterfaceDetail{
						"wg0": {Active: true, Drift: true},
					},
				},
			},
			want: StateDrift,
		},
		{
			name: "one active-consistent and one stopped interface still counts as consistent overall",
			r: Report{
				System: baseSystem,
				WireGuard: WireGuardInfo{
					WgPresent: true,
					Interfaces: map[string]InterfaceDetail{
						"wg0": {Active: true, Drift: false},
						"wg1": {Active: false, ConfigExists: true},
					},
				},
			},
			want: StateActiveConsistent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.r); got != tc.want {
				t.Errorf("Classify() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestComputeDrift(t *testing.T) {
	cases := []struct {
		name                string
		runtime, persistent []string
		want                bool
	}{
		{"identical", []string{"a", "b"}, []string{"b", "a"}, false},
		{"empty both", nil, nil, false},
		{"extra runtime peer", []string{"a", "b"}, []string{"a"}, true},
		{"missing runtime peer", []string{"a"}, []string{"a", "b"}, true},
		{"same length different members", []string{"a", "c"}, []string{"a", "b"}, true},
		{"duplicate in one side only", []string{"a", "a"}, []string{"a", "b"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ComputeDrift(tc.runtime, tc.persistent); got != tc.want {
				t.Errorf("ComputeDrift(%v, %v) = %v, want %v", tc.runtime, tc.persistent, got, tc.want)
			}
		})
	}
}
