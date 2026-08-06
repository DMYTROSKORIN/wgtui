//go:build manual

// This file is excluded from normal `go test ./...` (SPEC.md §25: "unit
// tests не требуют root" — and this test needs a real, specific, reachable
// server). Run it explicitly and only against a server you own:
//
//	WGTUI_MANUAL_HOST=nas.paynet.one \
//	WGTUI_MANUAL_PORT=2626 \
//	WGTUI_MANUAL_USER=skorin \
//	WGTUI_MANUAL_SUDO_PASS_FILE=/path/to/private/0600/file \
//	go test -tags manual ./internal/sshtransport/... -run TestManualDiscoverRealServer -v
//
// It is read-only: it never writes to any remote file, never restarts a
// service, never touches an existing peer. It exists to prove the
// sshtransport + wgconfig redaction path against a real WireGuard
// deployment, not just fixtures.
package sshtransport

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

func TestManualDiscoverRealServer(t *testing.T) {
	host := os.Getenv("WGTUI_MANUAL_HOST")
	if host == "" {
		t.Skip("WGTUI_MANUAL_HOST not set; skipping manual integration test")
	}
	port, _ := strconv.Atoi(os.Getenv("WGTUI_MANUAL_PORT"))
	user := os.Getenv("WGTUI_MANUAL_USER")
	sudoPassFile := os.Getenv("WGTUI_MANUAL_SUDO_PASS_FILE")
	if sudoPassFile == "" {
		t.Skip("WGTUI_MANUAL_SUDO_PASS_FILE not set; skipping manual integration test")
	}
	sudoPassRaw, err := os.ReadFile(sudoPassFile)
	if err != nil {
		t.Fatalf("read sudo password file: %v", err)
	}
	sudoPass := strings.TrimRight(string(sudoPassRaw), "\r\n")

	target := Target{HostOrAlias: host, Port: port, User: user}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	debug := os.Getenv("WGTUI_MANUAL_DEBUG") == "1"

	controlDir := t.TempDir() // t.TempDir() is 0700 by default on unix.
	client := NewClient(target, filepath.Join(controlDir, "cm.sock"))

	prompts := Prompts{
		HostKey: func(prompt string) bool {
			// The host is expected to already be in known_hosts from prior
			// manual use. A prompt here means something changed - refuse
			// and fail loudly rather than trust it automatically (T5).
			t.Errorf("unexpected host key prompt (host should already be known): %s", prompt)
			return false
		},
		Password: func() (string, error) {
			t.Errorf("unexpected SSH password prompt; this target is expected to use key auth")
			return "", context.Canceled
		},
		SudoPassword: func() (string, error) { return sudoPass, nil },
	}
	if debug {
		prompts.Debug = func(chunk string) { t.Logf("DEBUG raw chunk: %q", chunk) }
	}

	if err := client.Dial(ctx, prompts); err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer client.Close(ctx)

	// 1. Basic read-only reachability, no privilege needed.
	res, err := client.RunCommand(ctx, "uname -srm && id -un")
	if err != nil {
		t.Fatalf("RunCommand(uname): %v", err)
	}
	if res.ExitCode != 0 {
		t.Fatalf("uname/id exited %d, stderr=%s", res.ExitCode, res.Stderr)
	}
	t.Logf("remote system: %s", strings.TrimSpace(string(res.Stdout)))

	// 2. wg show without sudo is expected to fail with "Operation not
	// permitted" on a normal install - confirms the sudo path below is
	// actually being exercised, not a no-op.
	if res, err := client.RunCommand(ctx, "wg show all dump"); err != nil {
		t.Fatalf("RunCommand(wg show, no sudo): %v", err)
	} else if res.ExitCode == 0 {
		t.Logf("note: wg show all dump succeeded without sudo on this host (unusual but not an error)")
	}

	// 3. Validate the sudo password early (fails fast with a clear error
	// if it's wrong), then use it again for the actual privileged read -
	// RunSudo cannot rely on SudoValidate's cache, see its doc comment.
	if err := client.SudoValidate(ctx, prompts); err != nil {
		t.Fatalf("SudoValidate: %v", err)
	}
	dumpRes, err := client.RunSudo(ctx, "wg show all dump", prompts)
	if err != nil {
		t.Fatalf("RunSudo(wg show all dump): %v", err)
	}
	if dumpRes.ExitCode != 0 {
		t.Fatalf("wg show all dump (sudo) exited %d, stderr=%s", dumpRes.ExitCode, dumpRes.Stderr)
	}

	rawDump := string(dumpRes.Stdout)
	dump, err := wgconfig.ParseDump(rawDump)
	if err != nil {
		t.Fatalf("ParseDump: %v", err)
	}
	t.Logf("discovered %d interface(s), %d peer(s) (no key material logged)", len(dump.Interfaces), len(dump.Peers))
	if len(dump.Interfaces) == 0 {
		t.Fatalf("expected at least one WireGuard interface on this server")
	}

	// Sanity check that we actually read past *real* secret material
	// (not an empty/"(none)" field) without it ending up in `dump` or in
	// this log - i.e. that the redaction-by-construction in ParseDump is
	// doing real work here, not a no-op on empty input.
	var firstLine string
	for _, l := range strings.Split(rawDump, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) != "" {
			firstLine = l
			break
		}
	}
	fields := strings.Split(firstLine, "\t")
	if len(fields) != 5 {
		t.Fatalf("expected first non-empty dump line to be a 5-field interface line, got %d fields from %q", len(fields), firstLine)
	}
	if fields[1] == "" || fields[1] == "(none)" {
		t.Fatalf("expected interface line to carry a real private-key field to redact against; got an empty/none value - fixture assumption is wrong for this server")
	}

	// 4. End-to-end proof of the config-text redaction path (T1): fetch
	// the persistent config as root, confirm the raw text contains the
	// exact private key we just saw in the dump, then confirm Redact()
	// removes it. Never log the raw text or the key itself.
	iface := dump.Interfaces[0].Name
	confRes, err := client.RunSudo(ctx, "cat /etc/wireguard/"+iface+".conf", prompts)
	if err != nil {
		t.Fatalf("RunSudo(cat config): %v", err)
	}
	if confRes.ExitCode != 0 {
		t.Fatalf("cat %s.conf exited %d, stderr=%s", iface, confRes.ExitCode, confRes.Stderr)
	}
	rawConf := string(confRes.Stdout)
	if !strings.Contains(rawConf, fields[1]) {
		t.Fatalf("raw config for %s does not contain the private key seen in `wg show dump` - either the dump/private-key column assumption is wrong, or the config on disk has drifted from the running interface", iface)
	}
	redacted := wgconfig.Redact(rawConf)
	if strings.Contains(redacted, fields[1]) {
		t.Fatal("Redact() failed to strip the real server private key read from /etc/wireguard - THIS WOULD BE A SECRET LEAK, see THREAT_MODEL.md T1")
	}
	t.Logf("config redaction verified against real server: private key present in raw read, absent after Redact()")
}
