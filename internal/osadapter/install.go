package osadapter

import (
	"context"
	"fmt"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// InstallPlan is the exact sequence of commands installing WireGuard
// would run - shown to the user before anything executes (SPEC.md §12.3:
// "показать точную команду; запросить подтверждение"). Only the official
// wireguard-tools package and its normal dependencies are ever installed;
// nothing here removes an existing package.
type InstallPlan struct {
	Distro   string
	Commands []string
}

// PlanInstall returns the install commands for d, or an error if d isn't
// one of the MVP-supported distros (Ubuntu/Debian/Fedora).
func PlanInstall(d Distro) (InstallPlan, error) {
	switch d.PackageManager {
	case "apt":
		return InstallPlan{Distro: d.ID, Commands: []string{
			"apt-get update",
			"DEBIAN_FRONTEND=noninteractive apt-get install -y wireguard",
		}}, nil
	case "dnf":
		return InstallPlan{Distro: d.ID, Commands: []string{
			"dnf install -y wireguard-tools",
		}}, nil
	default:
		return InstallPlan{}, fmt.Errorf("no install adapter for distro %q - only Ubuntu/Debian (apt) and Fedora (dnf) are supported in MVP (SPEC.md §30.6)", d.ID)
	}
}

// ApplyInstall runs plan's commands in order, stopping at the first
// failure. Nothing here decides *whether* to install - that confirmation
// happens in the caller (cli/tui) before ApplyInstall is ever called
// (SPEC.md §12.1: "Никакая установка не начинается автоматически").
func ApplyInstall(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, plan InstallPlan) error {
	for _, cmd := range plan.Commands {
		res, err := client.RunSudo(ctx, cmd, prompts)
		if err != nil {
			return fmt.Errorf("%s: %w", cmd, err)
		}
		if res.ExitCode != 0 {
			return fmt.Errorf("%s exited %d: %s", cmd, res.ExitCode, res.Stderr)
		}
	}
	return nil
}

// VerifyInstalled checks that `wg` and `wg-quick` are now on PATH and
// returns `wg --version`'s output (SPEC.md §12.3 post-install checks).
func VerifyInstalled(ctx context.Context, client *sshtransport.Client) (version string, err error) {
	for _, bin := range []string{"wg", "wg-quick"} {
		res, err := client.RunCommand(ctx, "command -v "+bin)
		if err != nil {
			return "", fmt.Errorf("check for %s: %w", bin, err)
		}
		if res.ExitCode != 0 {
			return "", fmt.Errorf("%s not found on PATH after install", bin)
		}
	}
	res, err := client.RunCommand(ctx, "wg --version")
	if err != nil {
		return "", fmt.Errorf("wg --version: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("wg --version exited %d: %s", res.ExitCode, res.Stderr)
	}
	return string(res.Stdout), nil
}
