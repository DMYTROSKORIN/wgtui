package wgconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// knownSecrets lists the exact PrivateKey/PresharedKey values present in
// each fixture, so tests can assert they never survive redaction —
// independent of whatever the redaction regex happens to look like today.
var knownSecrets = map[string][]string{
	"minimal.conf": {
		"SGVsbG9Xb3JsZFRoaXNJc0FGYWtlS2V5MDE=",
	},
	"multi_peer_saveconfig.conf": {
		"MTIzNDU2Nzg5MGFiY2RlZmdoaWprbG1ub3BxcnN0dXY=", // interface PrivateKey
		"cHNrMWZha2VmYWtlZmFrZWZha2VmYWtlZmFrZWZha2U=", // peer PresharedKey
	},
	"crlf_lowercase_key.conf": {
		"Q1JMRmFrZUtleUZvclRlc3RpbmdPbmx5PT0=",
	},
	"unknown_fields.conf": {
		"VW5rbm93bkZpZWxkc0Zha2VLZXlGb3JUZXN0aW5nMDE=",
	},
}

func TestRedact_StripsKnownSecrets(t *testing.T) {
	for name, secrets := range knownSecrets {
		t.Run(name, func(t *testing.T) {
			raw := readFixture(t, name)
			for _, s := range secrets {
				if !strings.Contains(raw, s) {
					t.Fatalf("fixture %s does not actually contain expected secret %q — test fixture is stale", name, s)
				}
			}

			redacted := Redact(raw)
			for _, s := range secrets {
				if strings.Contains(redacted, s) {
					t.Errorf("secret %q survived redaction in %s:\n%s", s, name, redacted)
				}
			}
			if !strings.Contains(redacted, redactedValue) {
				t.Errorf("expected %q placeholder in redacted output for %s, got:\n%s", redactedValue, name, redacted)
			}
		})
	}
}

func TestRedact_PreservesEverythingElse(t *testing.T) {
	raw := readFixture(t, "unknown_fields.conf")
	redacted := Redact(raw)

	mustContain := []string{
		"Address = 10.10.0.1/24, fd00:10:10::1/64",
		"DNS = 1.1.1.1, 1.0.0.1",
		"MTU = 1420",
		"Table = off",
		"PreUp = echo starting >> /var/log/wgtui-test.log",
		"SomeFutureDirective = keep-me-untouched",
		"# a peer with only an allowed IPv6 range",
		"AllowedIPs = fd00:10:10::2/128",
	}
	for _, want := range mustContain {
		if !strings.Contains(redacted, want) {
			t.Errorf("redaction altered or dropped unrelated line %q\ngot:\n%s", want, redacted)
		}
	}
}

func TestRedact_CaseAndWhitespaceInsensitive(t *testing.T) {
	raw := readFixture(t, "crlf_lowercase_key.conf")
	redacted := Redact(raw)
	if strings.Contains(redacted, "Q1JMRmFrZUtleUZvclRlc3RpbmdPbmx5PT0=") {
		t.Fatalf("lowercase 'privatekey' with extra spaces before '=' was not redacted:\n%s", redacted)
	}
}

func TestRedact_EmptyInput(t *testing.T) {
	if got := Redact(""); got != "" {
		t.Errorf("Redact(\"\") = %q, want \"\"", got)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "testdata", "wgconfigs", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}
