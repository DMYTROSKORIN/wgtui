// Package peerops orchestrates peer lifecycle mutations (add, disable,
// resume, revoke, rotate - SPEC.md §17) on top of sshtransport,
// wgconfig, and transaction. It exists so cli and tui can share this
// logic rather than either reimplementing it (docs/ARCHITECTURE.md: cli
// and tui are both thin front ends).
package peerops

import (
	"context"
	"fmt"
	"net/netip"
	"regexp"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
	"github.com/DMYTROSKORIN/wgtui/internal/transaction"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// validInterfaceName mirrors the identical check in internal/discovery,
// internal/osadapter and internal/transaction. Duplicated deliberately per
// SPEC.md §23.1: this package is the one concatenating the interface name
// into a remote command ("cat /etc/wireguard/<iface>.conf"), so it
// validates at that point of use rather than trusting an upstream caller -
// the name can arrive from an operator's --interface flag or, on the
// auto-detect path, straight from the remote server's own `wg show
// interfaces` output (THREAT_MODEL.md T8: input sourced from the server
// itself is still untrusted).
var validInterfaceName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,15}$`)

// shellQuote wraps s in POSIX single quotes, escaping any embedded single
// quote - the standard technique for making a string inert to shell
// interpretation. Matches the identical helpers in internal/transaction
// and internal/osadapter (each package quotes independently rather than
// sharing a cross-package helper for these few lines).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// CurrentConfig is a snapshot of one interface's live state, fetched
// fresh immediately before building a mutation Plan.
type CurrentConfig struct {
	Interface       string
	ConfigPath      string
	Raw             string
	Fingerprint     string
	Doc             *wgconfig.Document
	ServerPublicKey string
	ListenPort      int
	VPNCIDR         netip.Prefix // zero value if the interface has no parseable IPv4 Address
	UsedAddresses   map[netip.Addr]bool
}

// ResolveInterface returns hint if set, otherwise the sole active
// WireGuard interface reported by `wg show interfaces`. It errors rather
// than guessing if there is more than one.
func ResolveInterface(ctx context.Context, client *sshtransport.Client, hint string) (string, error) {
	if hint != "" {
		if !validInterfaceName.MatchString(hint) {
			return "", fmt.Errorf("invalid interface name %q", hint)
		}
		return hint, nil
	}
	res, err := client.RunCommand(ctx, "wg show interfaces")
	if err != nil {
		return "", fmt.Errorf("list interfaces: %w", err)
	}
	names := strings.Fields(string(res.Stdout))
	switch len(names) {
	case 0:
		return "", fmt.Errorf("no active WireGuard interfaces found")
	case 1:
		if !validInterfaceName.MatchString(names[0]) {
			return "", fmt.Errorf("server reported an interface with an unsafe name %q", names[0])
		}
		return names[0], nil
	default:
		return "", fmt.Errorf("multiple active interfaces (%s); specify one with --interface", strings.Join(names, ", "))
	}
}

// FetchCurrentConfig reads the persistent config and runtime dump for
// iface and combines them into a CurrentConfig ready to base a Plan on.
func FetchCurrentConfig(ctx context.Context, client *sshtransport.Client, prompts sshtransport.Prompts, iface string) (CurrentConfig, error) {
	// Validate again at the point of use, even though the usual caller path
	// runs through ResolveInterface: iface is about to be concatenated into
	// a remote command, and a bad value must never reach the shell (SPEC.md
	// §23.1). configPath is single-quoted as a second, independent layer.
	if !validInterfaceName.MatchString(iface) {
		return CurrentConfig{}, fmt.Errorf("invalid interface name %q", iface)
	}
	configPath := "/etc/wireguard/" + iface + ".conf"

	confRes, err := client.RunSudo(ctx, "cat "+shellQuote(configPath), prompts)
	if err != nil {
		return CurrentConfig{}, fmt.Errorf("read %s: %w", configPath, err)
	}
	if confRes.ExitCode != 0 {
		return CurrentConfig{}, fmt.Errorf("read %s exited %d: %s", configPath, confRes.ExitCode, confRes.Stderr)
	}
	raw := string(confRes.Stdout)
	doc := wgconfig.Parse(raw)
	if _, ok := doc.InterfaceBlock(); !ok {
		return CurrentConfig{}, fmt.Errorf("%s has no [Interface] section - not a valid wg-quick config", configPath)
	}

	dumpRes, err := client.RunSudo(ctx, "wg show all dump", prompts)
	if err != nil {
		return CurrentConfig{}, fmt.Errorf("wg show all dump: %w", err)
	}
	if dumpRes.ExitCode != 0 {
		return CurrentConfig{}, fmt.Errorf("wg show all dump exited %d: %s", dumpRes.ExitCode, dumpRes.Stderr)
	}
	dump, err := wgconfig.ParseDump(string(dumpRes.Stdout))
	if err != nil {
		return CurrentConfig{}, fmt.Errorf("parse wg dump: %w", err)
	}

	cc := CurrentConfig{
		Interface:     iface,
		ConfigPath:    configPath,
		Raw:           raw,
		Fingerprint:   transaction.Fingerprint(raw),
		Doc:           doc,
		UsedAddresses: map[netip.Addr]bool{},
	}
	for _, ri := range dump.Interfaces {
		if ri.Name == iface {
			cc.ServerPublicKey = ri.PublicKey
			cc.ListenPort = ri.ListenPort
		}
	}
	for _, p := range dump.Peers {
		if p.Interface != iface {
			continue
		}
		for _, a := range p.AllowedIPs {
			if prefix, err := netip.ParsePrefix(strings.TrimSpace(a)); err == nil {
				cc.UsedAddresses[prefix.Addr()] = true
			}
		}
	}

	summary := wgconfig.Summarize(raw)
	for _, a := range summary.Addresses {
		if prefix, err := netip.ParsePrefix(strings.TrimSpace(a)); err == nil {
			cc.UsedAddresses[prefix.Addr()] = true
			if prefix.Addr().Is4() && !cc.VPNCIDR.IsValid() {
				cc.VPNCIDR = prefix
			}
		}
	}

	return cc, nil
}
