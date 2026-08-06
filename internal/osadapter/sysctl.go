package osadapter

import (
	"context"
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

const sysctlDropInPath = "/etc/sysctl.d/99-wgtui-wireguard-forward.conf"

// ForwardingStatus is the current, pre-change state of IPv4/IPv6
// forwarding, checked before EnableForwarding decides whether there's
// anything to do (SPEC.md §13.6: "до изменения проверить существующие
// значения и конфликты").
type ForwardingStatus struct {
	IPv4Enabled bool
	IPv6Enabled bool
}

// CheckForwarding reads the live sysctl values - unprivileged, read-only.
func CheckForwarding(ctx context.Context, client *sshtransport.Client, ipv6 bool) (ForwardingStatus, error) {
	var st ForwardingStatus
	res, err := client.RunCommand(ctx, "cat /proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return st, fmt.Errorf("read ipv4 forwarding: %w", err)
	}
	st.IPv4Enabled = strings.TrimSpace(string(res.Stdout)) == "1"

	if ipv6 {
		res, err := client.RunCommand(ctx, "cat /proc/sys/net/ipv6/conf/all/forwarding")
		if err != nil {
			return st, fmt.Errorf("read ipv6 forwarding: %w", err)
		}
		st.IPv6Enabled = strings.TrimSpace(string(res.Stdout)) == "1"
	}
	return st, nil
}

// EnableForwarding writes a dedicated sysctl.d drop-in (never touching
// /etc/sysctl.conf as a whole - SPEC.md §13.6) and applies it immediately.
// A no-op if the requested settings are already enabled.
func EnableForwarding(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, ipv6 bool) error {
	status, err := CheckForwarding(ctx, client, ipv6)
	if err != nil {
		return err
	}
	if status.IPv4Enabled && (!ipv6 || status.IPv6Enabled) {
		return nil
	}

	var b strings.Builder
	b.WriteString("set -e\n")
	fmt.Fprintf(&b, "FILE=%s\n", shellQuote(sysctlDropInPath))
	b.WriteString("{\n")
	b.WriteString(`  echo "# Managed by wgtui - do not edit by hand."` + "\n")
	b.WriteString(`  echo "net.ipv4.ip_forward=1"` + "\n")
	if ipv6 {
		b.WriteString(`  echo "net.ipv6.conf.all.forwarding=1"` + "\n")
	}
	b.WriteString(`} > "$FILE"` + "\n")
	b.WriteString(`chmod 644 "$FILE"` + "\n")
	b.WriteString(`sysctl -p "$FILE" > /dev/null` + "\n")
	b.WriteString(`echo WGTUI_FORWARDING_OK` + "\n")

	res, err := client.RunSudoScript(ctx, b.String(), prompts)
	if err != nil {
		return fmt.Errorf("run forwarding script: %w", err)
	}
	if res.ExitCode != 0 || !strings.Contains(string(res.Stdout), "WGTUI_FORWARDING_OK") {
		return fmt.Errorf("enabling forwarding failed (exit %d): %s", res.ExitCode, res.Stdout)
	}
	return nil
}
