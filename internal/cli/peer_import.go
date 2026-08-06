package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/wgconfig"
)

// runPeerImport implements SPEC.md §17.7: the user picks an existing local
// .conf, wgtui derives its public key from the private key already in the
// file (never sending the private key anywhere), and tries to match that
// public key against what the server actually has. It never fabricates a
// match, and the private key never reaches a log or stdout.
func runPeerImport(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui peer import: expected the server ID as the first argument")
		return 2
	}
	serverID := args[0]

	fs := flag.NewFlagSet("peer import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	file := fs.String("file", "", "path to the existing client .conf (required)")
	name := fs.String("name", "", "name to register the peer under locally (required)")
	ifaceFlag := fs.String("interface", "", "interface to search (auto-detected if only one is active)")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui peer import: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}
	if *file == "" || *name == "" {
		fmt.Fprintln(stderr, "wgtui peer import: --file and --name are both required")
		return 2
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: %v\n", err)
		return 1
	}
	doc := wgconfig.Parse(string(raw))
	ifaceBlk, ok := doc.InterfaceBlock()
	if !ok {
		fmt.Fprintf(stderr, "wgtui peer import: %s has no [Interface] section - not a valid client config\n", *file)
		return 1
	}
	privKey := ifaceBlk.Value("PrivateKey")
	if privKey == "" {
		fmt.Fprintf(stderr, "wgtui peer import: %s has no PrivateKey - not a valid client config\n", *file)
		return 1
	}
	publicKey, err := wgconfig.DerivePublicKey(privKey)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: derive public key: %v\n", err)
		return 1
	}

	v, vaultPath, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: %v\n", err)
		return 1
	}
	if _, exists := v.GetPeerByName(serverID, *name); exists {
		fmt.Fprintf(stderr, "wgtui peer import: a peer named %q is already registered for %q\n", *name, serverID)
		return 1
	}
	if _, exists := v.GetPeerByPublicKey(serverID, publicKey); exists {
		fmt.Fprintf(stderr, "wgtui peer import: this key is already registered under a different name for %q\n", serverID)
		return 1
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client, _, prompts, cleanup, err := connectToServer(ctx, stdout, v, serverID)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: %v\n", err)
		return 1
	}
	defer cleanup()

	if err := client.SudoValidate(ctx, prompts); err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: sudo: %v\n", err)
		return 1
	}
	res, err := client.RunSudo(ctx, "wg show all dump", prompts)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: wg show all dump: %v\n", err)
		return 1
	}
	if res.ExitCode != 0 {
		fmt.Fprintf(stderr, "wgtui peer import: wg show all dump exited %d: %s\n", res.ExitCode, res.Stderr)
		return 1
	}
	dump, err := wgconfig.ParseDump(string(res.Stdout))
	if err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: parse wg dump: %v\n", err)
		return 1
	}

	matchedIface := ""
	for _, p := range dump.Peers {
		if p.PublicKey == publicKey && (*ifaceFlag == "" || p.Interface == *ifaceFlag) {
			matchedIface = p.Interface
			break
		}
	}
	if matchedIface == "" {
		fmt.Fprintf(stderr, "wgtui peer import: no peer with this public key was found on %q - not registering (SPEC.md §7.2: profile not available on server)\n", serverID)
		return 1
	}

	v.SetProfile(serverID, publicKey, string(raw))
	if err := v.AddPeer(profilevault.PeerMetadata{
		ServerID: serverID, Name: *name, PublicKey: publicKey,
		Interface: matchedIface, CreatedAt: time.Now(), Status: profilevault.PeerStatusActive,
	}); err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: %v\n", err)
		return 1
	}
	if err := v.Save(vaultPath); err != nil {
		fmt.Fprintf(stderr, "wgtui peer import: save vault: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Imported %q as %q (interface %s, public key %s). Profile stored locally.\n", *file, *name, matchedIface, shortenKey(publicKey))
	return 0
}
