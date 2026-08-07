// Package peerview merges a server's runtime WireGuard peers with wgtui's
// local metadata registry into one reconciled view (SPEC.md §16.1). Shared
// by cli, tui, and mcpserver so the merge logic - which is expected to
// grow (e.g. Stage 3/4 already added Interface and Managed) - exists once,
// not duplicated per front end.
package peerview

import (
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// Peer is the JSON shape from SPEC.md §21's example, extended with
// Interface (Stage 4: multi-interface servers) and Managed. Name/Managed/
// ProfileAvailable are reconciled against the local metadata registry: a
// peer wgtui itself created shows its real name and profile availability;
// anything found on the server but not in local metadata is "unmanaged"
// and shown by shortened public key, exactly as §16.1 describes.
type Peer struct {
	Name                   string `json:"name"`
	Interface              string `json:"interface,omitempty"`
	VPNIP                  string `json:"vpn_ip,omitempty"`
	PublicKey              string `json:"public_key"`
	State                  string `json:"state"`
	LatestHandshakeSeconds *int64 `json:"latest_handshake_seconds,omitempty"`
	RxBytes                int64  `json:"rx_bytes"`
	TxBytes                int64  `json:"tx_bytes"`
	ProfileAvailable       bool   `json:"profile_available"`
	Managed                bool   `json:"managed_by_wgtui"`
}

// Reconcile merges runtime peers (from `wg show all dump`) with the local
// metadata registry: a peer wgtui itself created shows its real name and
// profile availability; anything found on the server but not in local
// metadata is "unmanaged" and shown by shortened public key. A peer wgtui
// disabled is deliberately absent from runtime *and* persistent config,
// but the user still asked wgtui to manage it - it's included too (state
// "disabled") rather than vanishing from the list just because it isn't
// on the wire right now. Pure function, no network.
func Reconcile(dumpPeers []wgconfig.RuntimePeer, localPeers []profilevault.PeerMetadata, hasProfile func(publicKey string) bool, now time.Time) []Peer {
	metaByKey := make(map[string]profilevault.PeerMetadata, len(localPeers))
	for _, m := range localPeers {
		metaByKey[m.PublicKey] = m
	}

	seen := make(map[string]bool, len(dumpPeers))
	views := make([]Peer, 0, len(dumpPeers))
	for _, p := range dumpPeers {
		seen[p.PublicKey] = true
		meta, managed := metaByKey[p.PublicKey]

		status := domain.HandshakeStatus(now, p.LatestHandshake, domain.DefaultOnlineThreshold, false)
		name := ShortenKey(p.PublicKey)
		if managed && meta.Name != "" {
			name = meta.Name
		}

		pv := Peer{
			Name:             name,
			Interface:        p.Interface,
			PublicKey:        p.PublicKey,
			State:            string(status),
			RxBytes:          p.ReceiveBytes,
			TxBytes:          p.TransmitBytes,
			ProfileAvailable: hasProfile(p.PublicKey),
			Managed:          managed,
		}
		if len(p.AllowedIPs) > 0 {
			pv.VPNIP = p.AllowedIPs[0]
		}
		if !p.LatestHandshake.IsZero() {
			secs := int64(now.Sub(p.LatestHandshake).Seconds())
			pv.LatestHandshakeSeconds = &secs
		}
		views = append(views, pv)
	}

	for _, meta := range localPeers {
		if seen[meta.PublicKey] || meta.Status != profilevault.PeerStatusDisabled {
			continue
		}
		views = append(views, Peer{
			Name:             meta.Name,
			Interface:        meta.Interface,
			PublicKey:        meta.PublicKey,
			State:            string(domain.StatusDisabled),
			ProfileAvailable: hasProfile(meta.PublicKey),
			Managed:          true,
		})
	}

	return views
}

// ShortenKey truncates a base64 WireGuard key for display.
func ShortenKey(key string) string {
	if len(key) <= 12 {
		return key
	}
	return key[:8] + "…"
}
