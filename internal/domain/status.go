package domain

import "time"

// PeerStatus is the set of peer states from SPEC.md §9.3. WireGuard itself
// has no notion of a peer being "online" - this is always a heuristic, and
// UI built on it must say so (SPEC.md §19).
type PeerStatus string

const (
	StatusOnline         PeerStatus = "online"
	StatusIdle           PeerStatus = "idle"
	StatusNeverConnected PeerStatus = "never_connected"
	StatusDisabled       PeerStatus = "disabled"
	StatusUnmanaged      PeerStatus = "unmanaged"
	StatusDrifted        PeerStatus = "drifted"
	StatusError          PeerStatus = "error"
)

// DefaultOnlineThreshold is SPEC.md §9.3's default: a handshake is
// considered fresh (peer "online") if it happened within the last 180s.
// This must be a configurable value at the call site, not a hardcoded
// assumption baked into UI code - see HandshakeStatus's threshold param.
const DefaultOnlineThreshold = 180 * time.Second

// HandshakeStatus computes the online/idle/never_connected/disabled subset
// of PeerStatus from handshake timing alone. It does not decide
// unmanaged/drifted/error - those require comparing runtime state against
// persistent config and the local metadata registry, which is a
// reconciliation step done by the discovery package, not a pure function
// of one peer's handshake time.
func HandshakeStatus(now, lastHandshake time.Time, onlineThreshold time.Duration, disabled bool) PeerStatus {
	if disabled {
		return StatusDisabled
	}
	if lastHandshake.IsZero() {
		return StatusNeverConnected
	}
	if now.Sub(lastHandshake) <= onlineThreshold {
		return StatusOnline
	}
	return StatusIdle
}
