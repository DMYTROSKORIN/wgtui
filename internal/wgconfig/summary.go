package wgconfig

import (
	"bufio"
	"strings"
)

// ConfigSummary is a read-only peek at a wg-quick config file: just enough
// to drive discovery's drift/consistency checks (SPEC.md §11.2, §15.1). It
// is not a full parse - Stage 2's writer needs to preserve comments, field
// order and unknown directives byte-for-byte (SPEC.md §15.5), which this
// type does not attempt.
type ConfigSummary struct {
	ListenPort     int
	Addresses      []string
	SaveConfig     bool
	HasPostUp      bool
	HasPostDown    bool
	PeerPublicKeys []string
}

// Summarize scans raw wg-quick config text. It never returns the private
// key: callers that need redacted display text should use Redact
// separately, and Summarize itself does not surface PrivateKey/
// PresharedKey values in any field.
func Summarize(raw string) ConfigSummary {
	var s ConfigSummary
	section := ""
	sc := bufio.NewScanner(strings.NewReader(raw))
	for sc.Scan() {
		line := strings.TrimSpace(strings.TrimRight(sc.Text(), "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)

		switch section {
		case "interface":
			switch key {
			case "listenport":
				s.ListenPort = atoiSafe(val)
			case "address":
				for _, a := range strings.Split(val, ",") {
					if a = strings.TrimSpace(a); a != "" {
						s.Addresses = append(s.Addresses, a)
					}
				}
			case "saveconfig":
				s.SaveConfig = strings.EqualFold(val, "true")
			case "postup":
				s.HasPostUp = true
			case "postdown":
				s.HasPostDown = true
			}
		case "peer":
			if key == "publickey" {
				s.PeerPublicKeys = append(s.PeerPublicKeys, val)
			}
		}
	}
	return s
}

func atoiSafe(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0
		}
		n = n*10 + int(r-'0')
	}
	return n
}
