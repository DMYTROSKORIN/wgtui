package wgconfig

import (
	"bufio"
	"regexp"
	"strings"
)

// secretLine matches a "PrivateKey = ..." or "PresharedKey = ..." directive,
// tolerant of leading/trailing whitespace and key-name case (SPEC.md §14,
// §26.1). PresharedKey is included even though §14 only names the server
// private key: SPEC.md §23.3 lists both as things that must never be
// logged, and a redactor that only knows about one secret field invites the
// next contributor to add a new secret field without updating it.
var secretLine = regexp.MustCompile(`(?i)^(\s*(?:PrivateKey|PresharedKey)\s*=\s*).*$`)

const redactedValue = "<redacted>"

// Redact returns text with every PrivateKey/PresharedKey directive value
// replaced by a fixed placeholder, regardless of which section it appears
// in. This is line-based, not a full config parse: it is meant to be the
// mandatory last line of defense before any remote config text reaches a
// log, the terminal, or JSON output (SPEC.md §29.11), independent of
// whether the structured parser in this package has been used at all.
func Redact(text string) string {
	var out strings.Builder
	sc := bufio.NewScanner(strings.NewReader(text))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	first := true
	for sc.Scan() {
		line := sc.Text()
		if !first {
			out.WriteByte('\n')
		}
		first = false
		if secretLine.MatchString(line) {
			out.WriteString(secretLine.ReplaceAllString(line, "${1}"+redactedValue))
			continue
		}
		out.WriteString(line)
	}
	return out.String()
}
