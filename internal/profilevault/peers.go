package profilevault

// profileKey is the Profiles map key for one peer's generated client
// profile - scoped by server since public keys are only unique per server,
// not globally.
func profileKey(serverID, publicKey string) string {
	return serverID + "/" + publicKey
}

// AddPeer appends metadata for a new peer. Returns an error if
// (ServerID, PublicKey) is already registered.
func (v *Vault) AddPeer(p PeerMetadata) error {
	for _, existing := range v.Registry.Peers {
		if existing.ServerID == p.ServerID && existing.PublicKey == p.PublicKey {
			return &DuplicatePeerError{ServerID: p.ServerID, PublicKey: p.PublicKey}
		}
	}
	v.Registry.Peers = append(v.Registry.Peers, p)
	return nil
}

// GetPeerByName returns the peer metadata for serverID with the given
// display name, if any.
func (v *Vault) GetPeerByName(serverID, name string) (PeerMetadata, bool) {
	for _, p := range v.Registry.Peers {
		if p.ServerID == serverID && p.Name == name {
			return p, true
		}
	}
	return PeerMetadata{}, false
}

// GetPeerByPublicKey returns the peer metadata for serverID with the
// given public key, if any.
func (v *Vault) GetPeerByPublicKey(serverID, publicKey string) (PeerMetadata, bool) {
	for _, p := range v.Registry.Peers {
		if p.ServerID == serverID && p.PublicKey == publicKey {
			return p, true
		}
	}
	return PeerMetadata{}, false
}

// ListPeers returns every peer registered for serverID.
func (v *Vault) ListPeers(serverID string) []PeerMetadata {
	var out []PeerMetadata
	for _, p := range v.Registry.Peers {
		if p.ServerID == serverID {
			out = append(out, p)
		}
	}
	return out
}

// UpdatePeer replaces the stored metadata matching (ServerID, PublicKey).
// Returns false if no such peer was found.
func (v *Vault) UpdatePeer(p PeerMetadata) bool {
	for i, existing := range v.Registry.Peers {
		if existing.ServerID == p.ServerID && existing.PublicKey == p.PublicKey {
			v.Registry.Peers[i] = p
			return true
		}
	}
	return false
}

// RemovePeer deletes the peer metadata entry for (serverID, publicKey).
// It does not touch Profiles - callers that also want the profile gone
// call DeleteProfile explicitly (SPEC.md §17.5's separate confirmations).
func (v *Vault) RemovePeer(serverID, publicKey string) bool {
	for i, p := range v.Registry.Peers {
		if p.ServerID == serverID && p.PublicKey == publicKey {
			v.Registry.Peers = append(v.Registry.Peers[:i], v.Registry.Peers[i+1:]...)
			return true
		}
	}
	return false
}

// SetProfile stores confText (a full client .conf, including its private
// key) for (serverID, publicKey).
func (v *Vault) SetProfile(serverID, publicKey, confText string) {
	if v.Registry.Profiles == nil {
		v.Registry.Profiles = make(map[string]string)
	}
	v.Registry.Profiles[profileKey(serverID, publicKey)] = confText
}

// GetProfile retrieves a stored profile, if any.
func (v *Vault) GetProfile(serverID, publicKey string) (string, bool) {
	s, ok := v.Registry.Profiles[profileKey(serverID, publicKey)]
	return s, ok
}

// DeleteProfile removes a stored profile. Returns false if none existed.
func (v *Vault) DeleteProfile(serverID, publicKey string) bool {
	key := profileKey(serverID, publicKey)
	if _, ok := v.Registry.Profiles[key]; !ok {
		return false
	}
	delete(v.Registry.Profiles, key)
	return true
}

// DuplicatePeerError is returned by AddPeer when (ServerID, PublicKey) is
// already registered.
type DuplicatePeerError struct{ ServerID, PublicKey string }

func (e *DuplicatePeerError) Error() string {
	return "profilevault: peer already registered for server " + e.ServerID + ": " + e.PublicKey
}
