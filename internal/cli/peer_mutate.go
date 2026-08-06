package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/peerops"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/transaction"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// parsePeerTargetArgs handles the "<server-id> <peer-name> [flags]" shape
// shared by disable/enable/revoke/rotate/export, peeling both positional
// arguments off before handing the rest to a FlagSet (see
// args_order_test.go for why the order matters: the standard flag package
// stops parsing at the first non-flag token).
func parsePeerTargetArgs(cmdName string, args []string, stderr io.Writer, extra func(*flag.FlagSet)) (serverID, peerName string, fs *flag.FlagSet, ok bool) {
	if len(args) < 2 {
		fmt.Fprintf(stderr, "wgtui %s: expected <server-id> <peer-name>\n", cmdName)
		return "", "", nil, false
	}
	serverID, peerName = args[0], args[1]
	fs = flag.NewFlagSet(cmdName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(args[2:]); err != nil {
		return "", "", nil, false
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(stderr, "wgtui %s: unexpected extra argument(s): %s\n", cmdName, strings.Join(fs.Args(), " "))
		return "", "", nil, false
	}
	return serverID, peerName, fs, true
}

type peerAddSummary struct {
	Name       string `json:"name"`
	PublicKey  string `json:"public_key"`
	VPNAddress string `json:"vpn_address"`
	BackupPath string `json:"backup_path"`
}

func tunnelModeLabel(full bool) string {
	if full {
		return "full tunnel"
	}
	return "split tunnel"
}

func outputDescription(output string) string {
	if output == "" {
		return "no file export"
	}
	return output
}

// writeProfileFile writes content to path with 0600 permissions,
// refusing to overwrite an existing file unless force is set (SPEC.md
// §7.3: "запрет на запись поверх существующего файла без подтверждения").
func writeProfileFile(path, content string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists (use --force to overwrite)", path)
		}
	}
	return os.WriteFile(path, []byte(content), 0o600)
}

func runPeerAdd(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui peer add: expected the server ID as the first argument")
		return 2
	}
	serverID := args[0]

	fs := flag.NewFlagSet("peer add", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "peer display name (required)")
	iface := fs.String("interface", "", "WireGuard interface (auto-detected if only one is active)")
	fullTunnel := fs.Bool("full-tunnel", true, "client routes all traffic through the tunnel")
	splitTunnel := fs.String("split-tunnel", "", "comma-separated CIDRs to route through the tunnel instead of full-tunnel")
	keepalive := fs.Int("keepalive", 25, "PersistentKeepalive seconds (0 to omit)")
	dns := fs.String("dns", "", "comma-separated DNS servers for the client profile")
	endpointHost := fs.String("endpoint-host", "", "VPN endpoint hostname/IP, if different from the SSH host")
	noPSK := fs.Bool("no-psk", false, "do not generate a preshared key (on by default, SPEC.md §30.8)")
	output := fs.String("output", "", "also write the client .conf to this path")
	force := fs.Bool("force", false, "overwrite --output if it already exists")
	showQR := fs.Bool("qr", false, "print the client profile as a QR code (SECRET, SPEC.md §7.4)")
	show := fs.Bool("show", false, "print the raw client .conf to stdout (SECRET)")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	jsonOut := fs.Bool("json", false, "output a structured, secret-free JSON summary")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui peer add: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}
	if *name == "" {
		fmt.Fprintln(stderr, "wgtui peer add: --name is required")
		return 2
	}
	if *splitTunnel != "" {
		*fullTunnel = false
	}

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer add: %v\n", err)
		return 1
	}
	if _, exists := v.GetPeerByName(serverID, *name); exists {
		fmt.Fprintf(stderr, "wgtui peer add: a peer named %q is already registered for %q\n", *name, serverID)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, server, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer add: %v\n", err)
		return 1
	}
	defer cleanup()

	resolvedIface, err := peerops.ResolveInterface(ctx, client, *iface)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer add: %v\n", err)
		return 1
	}
	cc, err := peerops.FetchCurrentConfig(ctx, client, prompts, resolvedIface)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer add: %v\n", err)
		return 1
	}

	addr, err := peerops.NextFreeAddress(cc.VPNCIDR, cc.UsedAddresses)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer add: allocate VPN address: %v\n", err)
		return 1
	}
	// The server-side peer block's AllowedIPs governs which source
	// addresses this peer may claim - it is always just their own
	// assigned address, never 0.0.0.0/0, regardless of tunnel mode.
	// Full/split tunnel only affects the *client's* AllowedIPs below.
	serverAllowedIPs := []string{addr.String() + "/32"}

	var clientAllowedIPs []string
	if *fullTunnel {
		clientAllowedIPs = []string{"0.0.0.0/0", "::/0"}
	} else {
		for _, c := range strings.Split(*splitTunnel, ",") {
			if c = strings.TrimSpace(c); c != "" {
				clientAllowedIPs = append(clientAllowedIPs, c)
			}
		}
		if len(clientAllowedIPs) == 0 {
			fmt.Fprintln(stderr, "wgtui peer add: --split-tunnel requires at least one CIDR")
			return 1
		}
	}

	planDesc := fmt.Sprintf("+ peer %s\n+ server allowed_ips: %s\n+ client vpn address: %s\n+ client mode: %s\n+ local profile: saved to vault (%s)",
		*name, serverAllowedIPs[0], addr, tunnelModeLabel(*fullTunnel), outputDescription(*output))
	if !confirm(stdout, *yes, planDesc) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}

	result, err := peerops.AddPeer(ctx, client, prompts, cc, *name, serverAllowedIPs, *keepalive, !*noPSK)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer add: %v\n", err)
		return 1
	}

	host := *endpointHost
	if host == "" {
		host = server.SSHHostOrAlias
	}
	var dnsList []string
	if *dns != "" {
		for _, d := range strings.Split(*dns, ",") {
			if d = strings.TrimSpace(d); d != "" {
				dnsList = append(dnsList, d)
			}
		}
	}
	clientConf := peerops.BuildClientProfile(peerops.ClientProfileParams{
		ClientPrivateKey:    result.KeyPair.PrivateKey,
		ClientAddress:       addr,
		DNS:                 dnsList,
		ServerPublicKey:     cc.ServerPublicKey,
		PresharedKey:        result.PresharedKey,
		EndpointHost:        host,
		EndpointPort:        cc.ListenPort,
		AllowedIPs:          clientAllowedIPs,
		PersistentKeepalive: *keepalive,
	})

	v.SetProfile(server.ID, result.KeyPair.PublicKey, clientConf)
	_ = v.AddPeer(profilevault.PeerMetadata{
		ServerID: server.ID, Name: *name, PublicKey: result.KeyPair.PublicKey,
		CreatedAt: time.Now(), Status: profilevault.PeerStatusActive,
	})

	if *output != "" {
		if err := writeProfileFile(*output, clientConf, *force); err != nil {
			fmt.Fprintf(stderr, "wgtui peer add: peer was created on the server, but writing --output failed: %v\n", err)
			fmt.Fprintf(stderr, "The profile is still saved in your local vault - retrieve it with:\n  wgtui peer export %s %s --output <path>\n", serverID, *name)
		} else {
			fmt.Fprintf(stdout, "Client profile written to %s\n", *output)
		}
	}

	if err := v.Save(vaultPath); err != nil {
		// SPEC.md §24.3: don't silently lose the key and don't claim
		// success falsely - print it now so the user has one more chance
		// to save it, and be explicit that the server already has this
		// peer (so they know to revoke it if they can't recover).
		fmt.Fprintf(stderr, "wgtui peer add: peer was created on the server (backup: %s), but saving the local vault failed: %v\n", result.Outcome.BackupPath, err)
		fmt.Fprintln(stderr, "To avoid losing this profile, here it is now - save it somewhere safe, or revoke the peer if you can't:")
		fmt.Fprintln(stderr, "----- BEGIN CLIENT PROFILE (SECRET) -----")
		fmt.Fprintln(stderr, clientConf)
		fmt.Fprintln(stderr, "----- END CLIENT PROFILE -----")
		return 1
	}

	if *showQR {
		art, err := peerops.RenderQRTerminal(clientConf)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui peer add: render QR: %v\n", err)
		} else {
			fmt.Fprintln(stdout, "\nWARNING: this QR code encodes the client's private key - it is a secret (SPEC.md §7.4).")
			fmt.Fprintln(stdout, art)
		}
	}
	if *show {
		fmt.Fprintln(stdout, "\nWARNING: the following is a secret client profile (SPEC.md §23.3) - do not paste it anywhere untrusted.")
		fmt.Fprintln(stdout, clientConf)
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(peerAddSummary{Name: *name, PublicKey: result.KeyPair.PublicKey, VPNAddress: addr.String(), BackupPath: result.Outcome.BackupPath}); err != nil {
			fmt.Fprintf(stderr, "wgtui peer add: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(stdout, "Added peer %q (%s), VPN address %s. Backup: %s\n", *name, shortenKey(result.KeyPair.PublicKey), addr, result.Outcome.BackupPath)
	fmt.Fprintln(stdout, "A fresh handshake from the client is the only real proof it works (SPEC.md §19) - this command does not wait for one.")
	return 0
}

func runPeerDisable(args []string, stdout, stderr io.Writer) int {
	var yes bool
	serverID, name, _, ok := parsePeerTargetArgs("peer disable", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	})
	if !ok {
		return 2
	}

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer disable: %v\n", err)
		return 1
	}
	meta, ok := v.GetPeerByName(serverID, name)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer disable: no peer named %q registered for %q\n", name, serverID)
		return 1
	}
	if meta.Status == profilevault.PeerStatusDisabled {
		fmt.Fprintf(stderr, "wgtui peer disable: %q is already disabled\n", name)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, _, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer disable: %v\n", err)
		return 1
	}
	defer cleanup()

	resolvedIface, err := peerops.ResolveInterface(ctx, client, "")
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer disable: %v\n", err)
		return 1
	}
	cc, err := peerops.FetchCurrentConfig(ctx, client, prompts, resolvedIface)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer disable: %v\n", err)
		return 1
	}

	if !confirm(stdout, yes, fmt.Sprintf("- peer %s (%s): removed from runtime and persistent config; local profile and metadata kept for resume", name, shortenKey(meta.PublicKey))) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}

	outcome, rawBlock, err := peerops.DisablePeer(ctx, client, prompts, cc, meta.PublicKey)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer disable: %v\n", err)
		return 1
	}

	meta.Status = profilevault.PeerStatusDisabled
	meta.DisabledBlockRaw = rawBlock
	v.UpdatePeer(meta)
	if err := v.Save(vaultPath); err != nil {
		fmt.Fprintf(stderr, "wgtui peer disable: applied on the server (backup: %s) but failed to save local metadata: %v\n", outcome.BackupPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "Disabled peer %q. Backup: %s\n", name, outcome.BackupPath)
	return 0
}

func runPeerEnable(args []string, stdout, stderr io.Writer) int {
	var yes bool
	serverID, name, _, ok := parsePeerTargetArgs("peer enable", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	})
	if !ok {
		return 2
	}

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer enable: %v\n", err)
		return 1
	}
	meta, ok := v.GetPeerByName(serverID, name)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer enable: no peer named %q registered for %q\n", name, serverID)
		return 1
	}
	if meta.Status != profilevault.PeerStatusDisabled || meta.DisabledBlockRaw == "" {
		fmt.Fprintf(stderr, "wgtui peer enable: %q is not currently disabled\n", name)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, _, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer enable: %v\n", err)
		return 1
	}
	defer cleanup()

	resolvedIface, err := peerops.ResolveInterface(ctx, client, "")
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer enable: %v\n", err)
		return 1
	}
	cc, err := peerops.FetchCurrentConfig(ctx, client, prompts, resolvedIface)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer enable: %v\n", err)
		return 1
	}

	if !confirm(stdout, yes, fmt.Sprintf("+ peer %s (%s): restored to persistent config exactly as it was before disabling", name, shortenKey(meta.PublicKey))) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}

	outcome, err := peerops.EnablePeer(ctx, client, prompts, cc, meta.DisabledBlockRaw)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer enable: %v\n", err)
		return 1
	}

	meta.Status = profilevault.PeerStatusActive
	meta.DisabledBlockRaw = ""
	v.UpdatePeer(meta)
	if err := v.Save(vaultPath); err != nil {
		fmt.Fprintf(stderr, "wgtui peer enable: applied on the server (backup: %s) but failed to save local metadata: %v\n", outcome.BackupPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "Resumed peer %q. Backup: %s\n", name, outcome.BackupPath)
	return 0
}

func runPeerRevoke(args []string, stdout, stderr io.Writer) int {
	var yes, deleteProfileFlag, deleteMetadataFlag bool
	serverID, name, _, ok := parsePeerTargetArgs("peer revoke", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&yes, "yes", false, "skip confirmation prompts (still never deletes local data unless the flags below are also set)")
		fs.BoolVar(&deleteProfileFlag, "delete-profile", false, "also delete the local client profile (contains the private key)")
		fs.BoolVar(&deleteMetadataFlag, "delete-metadata", false, "also delete the local peer metadata entry")
	})
	if !ok {
		return 2
	}

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer revoke: %v\n", err)
		return 1
	}
	meta, ok := v.GetPeerByName(serverID, name)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer revoke: no peer named %q registered for %q\n", name, serverID)
		return 1
	}

	// SPEC.md §17.5: three separate questions - revoking server access
	// never implies deleting local data too.
	if !confirm(stdout, yes, fmt.Sprintf("1/3 Revoke server access for peer %s (%s)?", name, shortenKey(meta.PublicKey))) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}
	deleteProfile := deleteProfileFlag || confirmOptIn(stdout, yes, "2/3 Also delete the local client profile (contains the private key)? [y/N] ")
	deleteMetadata := deleteMetadataFlag || confirmOptIn(stdout, yes, "3/3 Also delete the local peer metadata entry? [y/N] ")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, _, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer revoke: %v\n", err)
		return 1
	}
	defer cleanup()

	var outcome transaction.Outcome
	if meta.Status != profilevault.PeerStatusDisabled {
		resolvedIface, err := peerops.ResolveInterface(ctx, client, "")
		if err != nil {
			fmt.Fprintf(stderr, "wgtui peer revoke: %v\n", err)
			return 1
		}
		cc, err := peerops.FetchCurrentConfig(ctx, client, prompts, resolvedIface)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui peer revoke: %v\n", err)
			return 1
		}
		outcome, err = peerops.RevokePeer(ctx, client, prompts, cc, meta.PublicKey)
		if err != nil {
			fmt.Fprintf(stderr, "wgtui peer revoke: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(stdout, "(peer was already disabled - only removing local data as requested)")
	}

	if deleteProfile {
		v.DeleteProfile(serverID, meta.PublicKey)
	}
	if deleteMetadata {
		v.RemovePeer(serverID, meta.PublicKey)
	} else {
		meta.Status = profilevault.PeerStatusRevoked
		meta.DisabledBlockRaw = ""
		v.UpdatePeer(meta)
	}

	if err := v.Save(vaultPath); err != nil {
		fmt.Fprintf(stderr, "wgtui peer revoke: server change applied but failed to save the local vault: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Revoked peer %q. profile_deleted=%v metadata_deleted=%v. Backup: %s\n", name, deleteProfile, deleteMetadata, outcome.BackupPath)
	return 0
}

func runPeerRotate(args []string, stdout, stderr io.Writer) int {
	var yes bool
	serverID, name, _, ok := parsePeerTargetArgs("peer rotate", args, stderr, func(fs *flag.FlagSet) {
		fs.BoolVar(&yes, "yes", false, "skip the confirmation prompt")
	})
	if !ok {
		return 2
	}

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer rotate: %v\n", err)
		return 1
	}
	meta, ok := v.GetPeerByName(serverID, name)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer rotate: no peer named %q registered for %q\n", name, serverID)
		return 1
	}
	if meta.Status != profilevault.PeerStatusActive {
		fmt.Fprintf(stderr, "wgtui peer rotate: %q is not active (status=%s)\n", name, meta.Status)
		return 1
	}
	oldProfile, hadProfile := v.GetProfile(serverID, meta.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, server, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer rotate: %v\n", err)
		return 1
	}
	defer cleanup()

	resolvedIface, err := peerops.ResolveInterface(ctx, client, "")
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer rotate: %v\n", err)
		return 1
	}
	cc, err := peerops.FetchCurrentConfig(ctx, client, prompts, resolvedIface)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer rotate: %v\n", err)
		return 1
	}

	oldBlockIdx := cc.Doc.FindPeerByPublicKey(meta.PublicKey)
	if oldBlockIdx == -1 {
		fmt.Fprintf(stderr, "wgtui peer rotate: peer %s not found in the current config (is it disabled?)\n", shortenKey(meta.PublicKey))
		return 1
	}
	oldBlock := cc.Doc.Blocks[oldBlockIdx]
	allowedIPs := splitTrim(oldBlock.Value("AllowedIPs"), ",")
	keepalive, _ := strconv.Atoi(oldBlock.Value("PersistentKeepalive"))
	withPSK := oldBlock.Value("PresharedKey") != ""

	if !confirm(stdout, yes, fmt.Sprintf("rotate peer %s: new keypair replaces %s on the server; old local profile removed", name, shortenKey(meta.PublicKey))) {
		fmt.Fprintln(stdout, "Aborted.")
		return 1
	}

	result, err := peerops.RotatePeer(ctx, client, prompts, cc, meta.PublicKey, name, allowedIPs, keepalive, withPSK)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer rotate: %v\n", err)
		return 1
	}

	clientAddr, endpointHost, endpointPort, clientAllowedIPs, dnsList := rotateProfileParams(oldProfile, hadProfile, allowedIPs, server.SSHHostOrAlias, cc.ListenPort)

	newConf := peerops.BuildClientProfile(peerops.ClientProfileParams{
		ClientPrivateKey:    result.KeyPair.PrivateKey,
		ClientAddress:       clientAddr,
		DNS:                 dnsList,
		ServerPublicKey:     cc.ServerPublicKey,
		PresharedKey:        result.PresharedKey,
		EndpointHost:        endpointHost,
		EndpointPort:        endpointPort,
		AllowedIPs:          clientAllowedIPs,
		PersistentKeepalive: keepalive,
	})

	v.DeleteProfile(serverID, meta.PublicKey)
	v.SetProfile(serverID, result.KeyPair.PublicKey, newConf)
	meta.PublicKey = result.KeyPair.PublicKey
	v.UpdatePeer(meta)
	// UpdatePeer matches by the *new* public key, which isn't registered
	// yet under the old entry's identity - re-add if the match failed.
	if _, ok := v.GetPeerByPublicKey(serverID, result.KeyPair.PublicKey); !ok {
		_ = v.AddPeer(meta)
	}

	if err := v.Save(vaultPath); err != nil {
		fmt.Fprintf(stderr, "wgtui peer rotate: applied on the server (backup: %s), but saving the local vault failed: %v\n", result.Outcome.BackupPath, err)
		fmt.Fprintln(stderr, "New client profile (SECRET) - save it manually:")
		fmt.Fprintln(stderr, newConf)
		return 1
	}

	fmt.Fprintf(stdout, "Rotated peer %q: %s -> %s. Backup: %s\n", name, shortenKey(oldBlock.PublicKey()), shortenKey(result.KeyPair.PublicKey), result.Outcome.BackupPath)
	return 0
}

// rotateProfileParams recovers the client-facing profile parameters
// (endpoint, client AllowedIPs, DNS) that live only in the *client*
// profile, not the server-side block, by re-parsing the peer's previous
// profile if one was stored. Falls back to sane defaults (full tunnel, no
// DNS, endpoint = SSH host) if there wasn't one - e.g. rotating a peer
// that was imported without ever having a local profile.
func rotateProfileParams(oldProfile string, had bool, serverAllowedIPs []string, defaultHost string, defaultPort int) (addr netip.Addr, host string, port int, clientAllowedIPs, dns []string) {
	host, port = defaultHost, defaultPort
	clientAllowedIPs = []string{"0.0.0.0/0", "::/0"}
	if len(serverAllowedIPs) > 0 {
		if prefix, err := netip.ParsePrefix(serverAllowedIPs[0]); err == nil {
			addr = prefix.Addr()
		}
	}
	if !had {
		return addr, host, port, clientAllowedIPs, dns
	}
	doc := wgconfig.Parse(oldProfile)
	if peerBlk, ok := doc.FirstPeerBlock(); ok {
		if ep := peerBlk.Value("Endpoint"); ep != "" {
			if h, p, err := net.SplitHostPort(ep); err == nil {
				host = h
				if pn, err := strconv.Atoi(p); err == nil {
					port = pn
				}
			}
		}
		if aips := splitTrim(peerBlk.Value("AllowedIPs"), ","); len(aips) > 0 {
			clientAllowedIPs = aips
		}
	}
	if ifaceBlk, ok := doc.InterfaceBlock(); ok {
		dns = splitTrim(ifaceBlk.Value("DNS"), ",")
	}
	return addr, host, port, clientAllowedIPs, dns
}

func splitTrim(s, sep string) []string {
	var out []string
	for _, part := range strings.Split(s, sep) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

func runPeerExport(args []string, stdout, stderr io.Writer) int {
	var output string
	var force bool
	serverID, name, _, ok := parsePeerTargetArgs("peer export", args, stderr, func(fs *flag.FlagSet) {
		fs.StringVar(&output, "output", "", "file to write the profile to (required)")
		fs.BoolVar(&force, "force", false, "overwrite the output file if it already exists")
	})
	if !ok {
		return 2
	}
	if output == "" {
		fmt.Fprintln(stderr, "wgtui peer export: --output is required")
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer export: %v\n", err)
		return 1
	}
	meta, ok := v.GetPeerByName(serverID, name)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer export: no peer named %q registered for %q\n", name, serverID)
		return 1
	}
	profile, ok := v.GetProfile(serverID, meta.PublicKey)
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer export: no local profile stored for %q (profile not available, SPEC.md §7.2)\n", name)
		return 1
	}
	if err := writeProfileFile(output, profile, force); err != nil {
		fmt.Fprintf(stderr, "wgtui peer export: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "Exported %q to %s\n", name, output)
	return 0
}
