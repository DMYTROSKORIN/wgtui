package discovery

import (
	"context"
	"path"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// Discover runs the full read-only inspection from SPEC.md §11 over an
// already-Dialed client. Individual checks that fail are recorded in
// Report.Errors rather than aborting the whole run - a partial picture is
// still useful, and Classify decides when a gap is severe enough to
// surface as its own state (insufficient privileges, ambiguous).
//
// Discover never mutates remote state.
func Discover(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts) Report {
	var r Report
	r.WireGuard.Interfaces = map[string]InterfaceDetail{}

	discoverSystem(ctx, client, prompts, &r)
	discoverWireGuard(ctx, client, prompts, &r)
	discoverNetwork(ctx, client, &r)

	return r
}

func discoverSystem(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, r *Report) {
	if res, err := client.RunCommand(ctx, "uname -srm"); err == nil && res.ExitCode == 0 {
		r.System.Kernel = strings.TrimSpace(string(res.Stdout))
	} else {
		r.Errors = append(r.Errors, checkErr("uname", res, err))
	}

	if res, err := client.RunCommand(ctx, "cat /etc/os-release"); err == nil && res.ExitCode == 0 {
		id, versionID := parseOSRelease(string(res.Stdout))
		r.System.OSID = id
		r.System.OSVersionID = versionID
		r.System.PackageManager = packageManagerFor(id)
	} else {
		r.Errors = append(r.Errors, checkErr("/etc/os-release", res, err))
	}

	if res, err := client.RunCommand(ctx, "command -v systemctl"); err == nil {
		r.System.HasSystemctl = res.ExitCode == 0
	}

	if res, err := client.RunCommand(ctx, "id -un"); err == nil && res.ExitCode == 0 {
		r.System.CurrentUser = strings.TrimSpace(string(res.Stdout))
	}

	if err := client.SudoValidate(ctx, prompts); err == nil {
		r.System.SudoAvailable = true
	} else {
		r.Errors = append(r.Errors, "sudo validation: "+err.Error())
	}
}

func discoverWireGuard(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, r *Report) {
	if res, err := client.RunCommand(ctx, "command -v wg"); err == nil {
		r.WireGuard.WgPresent = res.ExitCode == 0
	} else {
		r.Errors = append(r.Errors, checkErr("command -v wg", res, err))
	}
	if !r.WireGuard.WgPresent {
		return
	}

	if res, err := client.RunCommand(ctx, "command -v wg-quick"); err == nil {
		r.WireGuard.WgQuickPresent = res.ExitCode == 0
	}
	if res, err := client.RunCommand(ctx, "wg --version"); err == nil && res.ExitCode == 0 {
		r.WireGuard.WgVersion = strings.TrimSpace(string(res.Stdout))
	}

	if res, err := client.RunCommand(ctx, "wg show interfaces"); err == nil && res.ExitCode == 0 {
		r.WireGuard.ActiveInterfaces = filterValidNames(strings.Fields(string(res.Stdout)))
	}

	if !r.System.SudoAvailable {
		return
	}

	if len(r.WireGuard.ActiveInterfaces) == 0 {
		if res, err := client.RunSudo(ctx, "wg show interfaces", prompts); err == nil && res.ExitCode == 0 {
			r.WireGuard.ActiveInterfaces = filterValidNames(strings.Fields(string(res.Stdout)))
		}
	}
	for _, name := range r.WireGuard.ActiveInterfaces {
		d := r.WireGuard.Interfaces[name]
		d.Active = true
		r.WireGuard.Interfaces[name] = d
	}

	if res, err := client.RunSudo(ctx, "wg show all dump", prompts); err == nil && res.ExitCode == 0 {
		dump, perr := wgconfig.ParseDump(string(res.Stdout))
		if perr != nil {
			r.Errors = append(r.Errors, "parse wg show all dump: "+perr.Error())
		} else {
			for _, p := range dump.Peers {
				if !isValidInterfaceName(p.Interface) {
					continue
				}
				d := r.WireGuard.Interfaces[p.Interface]
				d.RuntimePeerPublicKeys = append(d.RuntimePeerPublicKeys, p.PublicKey)
				r.WireGuard.Interfaces[p.Interface] = d
			}
		}
	} else {
		r.Errors = append(r.Errors, checkErr("wg show all dump", res, err))
	}

	// The glob must expand as root, not as the login user: /etc/wireguard
	// is normally 0700 root-owned, so an unprivileged shell can't even
	// see there's anything to match and passes the literal "*.conf"
	// through - `ls` then fails with "No such file or directory" even
	// though the file exists (confirmed against nas.paynet.one:2626).
	// Wrapping the glob in `sudo sh -c '...'` makes the expansion happen
	// inside the root-owned shell sudo starts.
	if res, err := client.RunSudo(ctx, `sh -c 'ls -1 /etc/wireguard/*.conf 2>/dev/null'`, prompts); err == nil {
		for _, line := range strings.Split(string(res.Stdout), "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasSuffix(line, ".conf") {
				continue
			}
			name := strings.TrimSuffix(path.Base(line), ".conf")
			if !isValidInterfaceName(name) {
				r.Errors = append(r.Errors, "skipping config file with unsafe interface name: "+name)
				continue
			}
			d := r.WireGuard.Interfaces[name]
			d.ConfigExists = true
			r.WireGuard.Interfaces[name] = d
		}
	}

	for name, d := range r.WireGuard.Interfaces {
		if !d.ConfigExists || !isValidInterfaceName(name) {
			continue
		}
		confRes, err := client.RunSudo(ctx, "cat /etc/wireguard/"+name+".conf", prompts)
		if err != nil || confRes.ExitCode != 0 {
			r.Errors = append(r.Errors, checkErr("cat "+name+".conf", confRes, err))
			continue
		}
		summary := wgconfig.Summarize(string(confRes.Stdout))
		d.SaveConfig = summary.SaveConfig
		d.PersistentPeerPublicKeys = summary.PeerPublicKeys
		d.Drift = ComputeDrift(d.RuntimePeerPublicKeys, d.PersistentPeerPublicKeys)
		r.WireGuard.Interfaces[name] = d
	}
}

func filterValidNames(names []string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if isValidInterfaceName(n) {
			out = append(out, n)
		}
	}
	return out
}
