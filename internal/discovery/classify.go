package discovery

// Classify maps a filled-out Report to one of the seven discovery states
// from SPEC.md §11.4. It is a pure function so the decision logic can be
// unit tested without a real SSH connection.
func Classify(r Report) State {
	if !r.System.SudoAvailable {
		// Most of the checks that distinguish the remaining states need
		// root (wg show, /etc/wireguard listing) - without it we cannot
		// tell "stopped" from "drifted" from "consistent" reliably.
		return StateInsufficientPrivileges
	}
	if !r.WireGuard.WgPresent {
		return StateWireGuardMissing
	}
	if r.System.OSID == "" || r.System.PackageManager == "" {
		return StateAmbiguous
	}
	if len(r.WireGuard.Interfaces) == 0 {
		return StateToolsNoInterfaces
	}

	anyActive := false
	anyDrift := false
	for _, iface := range r.WireGuard.Interfaces {
		if iface.Active {
			anyActive = true
			if iface.Drift {
				anyDrift = true
			}
		}
	}
	if !anyActive {
		return StateConfigExistsStopped
	}
	if anyDrift {
		return StateDrift
	}
	return StateActiveConsistent
}

// ComputeDrift reports whether the runtime and persistent peer sets for an
// interface differ - order-independent, since wg dump order need not match
// config file order.
func ComputeDrift(runtimeKeys, persistentKeys []string) bool {
	if len(runtimeKeys) != len(persistentKeys) {
		return true
	}
	set := make(map[string]int, len(persistentKeys))
	for _, k := range persistentKeys {
		set[k]++
	}
	for _, k := range runtimeKeys {
		if set[k] == 0 {
			return true
		}
		set[k]--
	}
	for _, count := range set {
		if count != 0 {
			return true
		}
	}
	return false
}
