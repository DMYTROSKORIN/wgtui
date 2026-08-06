package domain

import (
	"net/netip"
	"time"
)

// Peer is the minimal peer record from SPEC.md §9.2. It never carries a
// private key — that lives only in the local profile file referenced by
// LocalProfilePath (SPEC.md §6.3, §7.3).
type Peer struct {
	ServerID              string
	Name                  string
	PublicKey             string
	PresharedKeyPresent   bool
	VPNIPv4               netip.Addr
	VPNIPv6               netip.Addr // zero value (IsValid() == false) if unset
	AllowedIPsOnServer    []netip.Prefix
	ClientAllowedIPs      []netip.Prefix
	Endpoint              string
	PersistentKeepalive   int
	CreatedAt             time.Time // zero value if unknown
	Status                PeerStatus
	ManagedByWgtui        bool
	LocalProfilePath      string // "" if no local profile is available
	LastHandshake         time.Time
	ReceiveBytes          int64
	TransmitBytes         int64
	CurrentRemoteEndpoint string
	Notes                 string
}

// ProfileAvailability is the three-way status from SPEC.md §7.2: whether a
// local .conf exists for this peer and, if so, whether its public key
// still matches what the server has.
type ProfileAvailability string

const (
	ProfileAvailable    ProfileAvailability = "profile_available"
	ProfileNotAvailable ProfileAvailability = "profile_not_available"
	ProfileMismatch     ProfileAvailability = "profile_mismatch"
)
