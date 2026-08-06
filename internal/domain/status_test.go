package domain

import (
	"testing"
	"time"
)

func TestHandshakeStatus(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name      string
		handshake time.Time
		disabled  bool
		want      PeerStatus
	}{
		{"disabled wins over recent handshake", now.Add(-1 * time.Second), true, StatusDisabled},
		{"never connected", time.Time{}, false, StatusNeverConnected},
		{"just under threshold is online", now.Add(-179 * time.Second), false, StatusOnline},
		{"exactly at threshold is online", now.Add(-180 * time.Second), false, StatusOnline},
		{"just over threshold is idle", now.Add(-181 * time.Second), false, StatusIdle},
		{"long idle", now.Add(-30 * 24 * time.Hour), false, StatusIdle},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := HandshakeStatus(now, tc.handshake, DefaultOnlineThreshold, tc.disabled)
			if got != tc.want {
				t.Errorf("HandshakeStatus() = %q, want %q", got, tc.want)
			}
		})
	}
}
