package wgconfig

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
)

// NewPeerSpec describes a peer block to add. Every field ends up embedded
// in a remote config file, so each one is validated before BuildPeerBlock
// will use it (SPEC.md §23.1/§23.2 - never write unvalidated remote-bound
// text; a name in particular has to be defended against newline injection,
// since a literal "\n" in a display name could otherwise smuggle a whole
// extra directive line into the config).
type NewPeerSpec struct {
	Name                string // optional; becomes a "# name" comment
	PublicKey           string
	PresharedKey        string // "" to omit
	AllowedIPs          []string
	PersistentKeepalive int // 0 to omit
}

// wgKeyPattern matches a WireGuard key as base64: 44 characters, the last
// of which is always "=" for a 32-byte value.
var wgKeyPattern = regexp.MustCompile(`^[A-Za-z0-9+/]{43}=$`)

// ValidatePublicKey reports whether s is a plausible WireGuard public (or
// preshared) key: 32 bytes, standard base64 encoded (SPEC.md §26.1).
func ValidatePublicKey(s string) error {
	if !wgKeyPattern.MatchString(s) {
		return fmt.Errorf("not a valid WireGuard key: wrong shape")
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("not valid base64: %w", err)
	}
	if len(decoded) != 32 {
		return fmt.Errorf("decodes to %d bytes, want 32", len(decoded))
	}
	return nil
}

// ValidateAllowedIPs reports whether every entry parses as a CIDR prefix.
func ValidateAllowedIPs(ips []string) error {
	if len(ips) == 0 {
		return fmt.Errorf("at least one AllowedIPs entry is required")
	}
	for _, ip := range ips {
		if _, err := netip.ParsePrefix(strings.TrimSpace(ip)); err != nil {
			return fmt.Errorf("invalid AllowedIPs entry %q: %w", ip, err)
		}
	}
	return nil
}

// sanitizeComment makes s safe to embed as a single "# ..." comment line:
// strips characters that could terminate the line early or inject a new
// one (SPEC.md §23.1/§23.2), and caps length generously.
func sanitizeComment(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r':
			return ' '
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	const maxLen = 200
	if len(s) > maxLen {
		s = s[:maxLen]
	}
	return s
}

// BuildPeerBlock validates spec and, if valid, returns a ready-to-append
// Block. It never accepts an unvalidated field silently - every value
// that ends up in the returned Block's text has passed a check above.
func BuildPeerBlock(spec NewPeerSpec) (*Block, error) {
	if err := ValidatePublicKey(spec.PublicKey); err != nil {
		return nil, fmt.Errorf("public key: %w", err)
	}
	if spec.PresharedKey != "" {
		if err := ValidatePublicKey(spec.PresharedKey); err != nil {
			return nil, fmt.Errorf("preshared key: %w", err)
		}
	}
	if err := ValidateAllowedIPs(spec.AllowedIPs); err != nil {
		return nil, err
	}
	if spec.PersistentKeepalive < 0 || spec.PersistentKeepalive > 65535 {
		return nil, fmt.Errorf("persistent keepalive %d out of range", spec.PersistentKeepalive)
	}

	var body strings.Builder
	fmt.Fprintf(&body, "PublicKey = %s\n", spec.PublicKey)
	if spec.PresharedKey != "" {
		fmt.Fprintf(&body, "PresharedKey = %s\n", spec.PresharedKey)
	}
	fmt.Fprintf(&body, "AllowedIPs = %s\n", strings.Join(spec.AllowedIPs, ", "))
	if spec.PersistentKeepalive > 0 {
		fmt.Fprintf(&body, "PersistentKeepalive = %d\n", spec.PersistentKeepalive)
	}

	header := "[Peer]\n"
	if name := sanitizeComment(spec.Name); name != "" {
		header = fmt.Sprintf("# %s\n[Peer]\n", name)
	}

	return &Block{HeaderRaw: header, Kind: "peer", Body: body.String()}, nil
}
