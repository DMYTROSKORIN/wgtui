//go:build manual

package discovery_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
	"github.com/DMYTROSKORIN/wgtui/internal/osadapter"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
	"golang.org/x/term"
)

// TestManualMissingWireGuard exercises the complete read-only discovery and
// diagnostic path against a disposable supported server before WireGuard is
// installed. It also builds, but deliberately does not apply, the installation
// plan. Passwords are read only from a caller-owned 0600 file.
func TestManualMissingWireGuard(t *testing.T) {
	host := os.Getenv("WGTUI_MANUAL_HOST")
	if host == "" {
		t.Skip("WGTUI_MANUAL_HOST is required")
	}
	var rawPassword []byte
	var err error
	if passwordFile := os.Getenv("WGTUI_MANUAL_PASSWORD_FILE"); passwordFile != "" {
		rawPassword, err = os.ReadFile(passwordFile)
	} else if os.Getenv("WGTUI_MANUAL_INTERACTIVE_PASSWORD") == "1" && term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Test VM SSH/sudo password: ")
		rawPassword, err = term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
	} else {
		t.Skip("set WGTUI_MANUAL_PASSWORD_FILE or WGTUI_MANUAL_INTERACTIVE_PASSWORD=1")
	}
	if err != nil {
		t.Fatalf("read credential: %v", err)
	}
	password := strings.TrimRight(string(rawPassword), "\r\n")
	if password == "" {
		t.Fatal("credential file is empty")
	}
	port, _ := strconv.Atoi(os.Getenv("WGTUI_MANUAL_PORT"))
	target := sshtransport.Target{HostOrAlias: host, Port: port, User: os.Getenv("WGTUI_MANUAL_USER")}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	client := sshtransport.NewClient(target, filepath.Join(t.TempDir(), "cm.sock"))
	prompts := sshtransport.Prompts{
		HostKey:      func(string) bool { return os.Getenv("WGTUI_MANUAL_ACCEPT_HOST_KEY") == "1" },
		Password:     func() (string, error) { return password, nil },
		SudoPassword: func() (string, error) { return password, nil },
	}
	if err := client.Dial(ctx, prompts); err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close(ctx)

	report := discovery.Discover(ctx, client, prompts)
	if !report.System.SudoAvailable {
		t.Fatal("sudo validation failed")
	}
	if report.WireGuard.WgPresent || report.WireGuard.WgQuickPresent || len(report.WireGuard.Interfaces) != 0 {
		t.Fatalf("expected a clean missing-WireGuard host, got %+v", report.WireGuard)
	}
	if state := discovery.Classify(report); state != discovery.StateWireGuardMissing {
		t.Fatalf("state=%s, want %s", state, discovery.StateWireGuardMissing)
	}
	if report.Network.DefaultOutboundInterface == "" {
		t.Fatal("default outbound interface was not discovered")
	}

	checks := diagnostics.BuildChecks(report, nil, time.Now())
	assertCheckStatus(t, checks, "wireguard_tools", diagnostics.StatusFail)
	assertCheckStatus(t, checks, "interface_exists", diagnostics.StatusFail)

	distro, err := osadapter.DetectDistro(ctx, client)
	if err != nil {
		t.Fatalf("detect distro: %v", err)
	}
	if !distro.Supported() {
		t.Fatalf("distro is not supported: %+v", distro)
	}
	plan, err := osadapter.PlanInstall(distro)
	if err != nil {
		t.Fatalf("build install plan: %v", err)
	}
	if len(plan.Commands) == 0 {
		t.Fatal("installation plan is empty")
	}
	t.Logf("verified missing-WireGuard state on %s %s: %d diagnostics, %d planned install commands; no remote mutation applied",
		distro.ID, distro.VersionID, len(checks), len(plan.Commands))
}

func assertCheckStatus(t *testing.T, checks []diagnostics.Check, name string, want diagnostics.Status) {
	t.Helper()
	for _, check := range checks {
		if check.Name == name {
			if check.Status != want {
				t.Fatalf("check %s=%s, want %s", name, check.Status, want)
			}
			return
		}
	}
	t.Fatalf("check %s not found", name)
}
