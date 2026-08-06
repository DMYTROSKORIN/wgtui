package wgconfig

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// RuntimeInterface is the redacted view of one "wg show all dump" interface
// line. It has no field for the private key: ParseDump reads that column
// and discards it before any struct is built, so the secret cannot leak
// through a debug print, a JSON encoder, or a future refactor that adds a
// careless String()/MarshalJSON() method (SPEC.md §14, §26.1).
type RuntimeInterface struct {
	Name       string
	PublicKey  string
	ListenPort int
	FwMark     string
}

// RuntimePeer is one peer line from "wg show all dump".
type RuntimePeer struct {
	Interface           string
	PublicKey           string
	PresharedKeyPresent bool
	Endpoint            string // "" if the peer has none yet
	AllowedIPs          []string
	LatestHandshake     time.Time // zero value if there has been no handshake
	ReceiveBytes        int64
	TransmitBytes       int64
	PersistentKeepalive int // 0 if off
}

// Dump is the parsed result of "wg show all dump".
type Dump struct {
	Interfaces []RuntimeInterface
	Peers      []RuntimePeer
}

// ParseDump parses the output of `wg show all dump`. Field counts per line
// distinguish interface lines (5: interface, private-key, public-key,
// listen-port, fwmark) from peer lines (9: interface, public-key,
// preshared-key, endpoint, allowed-ips, latest-handshake, rx, tx,
// persistent-keepalive) — this is the documented wireguard-tools dump
// format. The private-key column on interface lines is read and discarded
// on the spot; see RuntimeInterface.
func ParseDump(output string) (Dump, error) {
	var d Dump
	sc := bufio.NewScanner(strings.NewReader(output))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		switch len(fields) {
		case 5:
			iface, err := parseInterfaceLine(fields)
			if err != nil {
				return Dump{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			d.Interfaces = append(d.Interfaces, iface)
		case 9:
			peer, err := parsePeerLine(fields)
			if err != nil {
				return Dump{}, fmt.Errorf("line %d: %w", lineNo, err)
			}
			d.Peers = append(d.Peers, peer)
		default:
			return Dump{}, fmt.Errorf("line %d: unexpected field count %d (want 5 for an interface line or 9 for a peer line)", lineNo, len(fields))
		}
	}
	if err := sc.Err(); err != nil {
		return Dump{}, err
	}
	return d, nil
}

func parseInterfaceLine(fields []string) (RuntimeInterface, error) {
	// fields[1] is the private key column. It is intentionally never read
	// into a variable beyond this point.
	port, err := strconv.Atoi(fields[3])
	if err != nil {
		return RuntimeInterface{}, fmt.Errorf("bad listen-port %q: %w", fields[3], err)
	}
	return RuntimeInterface{
		Name:       fields[0],
		PublicKey:  noneEmpty(fields[2]),
		ListenPort: port,
		FwMark:     unsetToEmpty(fields[4]),
	}, nil
}

func parsePeerLine(fields []string) (RuntimePeer, error) {
	p := RuntimePeer{
		Interface:           fields[0],
		PublicKey:           fields[1],
		PresharedKeyPresent: fields[2] != "(none)",
		Endpoint:            noneEmpty(fields[3]),
	}
	if aips := noneEmpty(fields[4]); aips != "" {
		p.AllowedIPs = strings.Split(aips, ",")
	}
	if fields[5] != "0" {
		secs, err := strconv.ParseInt(fields[5], 10, 64)
		if err != nil {
			return RuntimePeer{}, fmt.Errorf("bad latest-handshake %q: %w", fields[5], err)
		}
		p.LatestHandshake = time.Unix(secs, 0)
	}
	rx, err := strconv.ParseInt(fields[6], 10, 64)
	if err != nil {
		return RuntimePeer{}, fmt.Errorf("bad transfer-rx %q: %w", fields[6], err)
	}
	tx, err := strconv.ParseInt(fields[7], 10, 64)
	if err != nil {
		return RuntimePeer{}, fmt.Errorf("bad transfer-tx %q: %w", fields[7], err)
	}
	p.ReceiveBytes = rx
	p.TransmitBytes = tx
	if fields[8] != "off" {
		ka, err := strconv.Atoi(fields[8])
		if err != nil {
			return RuntimePeer{}, fmt.Errorf("bad persistent-keepalive %q: %w", fields[8], err)
		}
		p.PersistentKeepalive = ka
	}
	return p, nil
}

func noneEmpty(s string) string {
	if s == "(none)" {
		return ""
	}
	return s
}

// unsetToEmpty additionally treats "off" as unset, which is how
// wireguard-tools represents an unset fwmark (and, separately,
// PersistentKeepalive — handled inline in parsePeerLine since that field
// also needs numeric parsing when it is set).
func unsetToEmpty(s string) string {
	if s == "(none)" || s == "off" {
		return ""
	}
	return s
}
