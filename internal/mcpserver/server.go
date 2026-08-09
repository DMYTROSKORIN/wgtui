// Package mcpserver is wgtui's third thin front end (alongside cli and
// tui, see docs/ARCHITECTURE.md) exposing server/peer operations as MCP
// tools, so an AI agent can drive wgtui through the same validated
// discovery/peerops/osadapter/transaction layer a human uses via the TUI -
// same checks, same plan-before-apply safety model for every mutation -
// instead of having to shell out to the CLI and re-derive that model from
// scratch with no guardrails of its own. It never calls into cli or tui.
//
// See credentials.go for why secrets are handled differently here than in
// cli/tui (no terminal to prompt on), and each mutating tool's Description
// for the confirm-gate that stands in for cli's interactive "Apply? [y/N]"
// and tui's plan screen.
package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// Version mirrors internal/cli's placeholder - see that package's doc
// comment for why each front end carries its own rather than importing
// one another for it. A var, not a const, so -ldflags -X can override it
// at release-build time (.goreleaser.yaml).
var Version = "0.0.0-dev"

// Server bundles the already-unlocked vault every tool handler needs.
// Vault unlock happens once, before New is called - see run.go.
type Server struct {
	vault     *profilevault.Vault
	vaultPath string
}

// New builds an MCP server exposing wgtui's server/peer operations as
// tools and registers all of them. v must already be unlocked.
func New(v *profilevault.Vault, vaultPath string) *mcp.Server {
	s := &Server{vault: v, vaultPath: vaultPath}

	srv := mcp.NewServer(&mcp.Implementation{Name: "wgtui", Version: Version}, nil)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "list_servers",
		Description: "List the WireGuard servers registered locally with wgtui. No network access - " +
			"reads only the local encrypted registry. Call this first to find valid server_id values for every other tool.",
	}, s.listServers)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "server_inspect",
		Description: "Connect to a registered server over SSH and run full read-only discovery: OS, " +
			"WireGuard install state, interfaces, network/firewall state. Never changes anything on the server.",
	}, s.serverInspect)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "server_diagnose",
		Description: "Connect to a registered server and run the diagnostic checklist: reachability, " +
			"WireGuard tooling, interface/config drift, forwarding, firewall backend, peer key age. Read-only. " +
			"Always states its own limitations (e.g. a passing check here does not prove the server is reachable " +
			"from the public internet).",
	}, s.serverDiagnose)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "peer_list",
		Description: "List a server's WireGuard peers, reconciled with wgtui's local metadata: a peer wgtui " +
			"knows about shows its real name and whether a local profile is available; anything else is " +
			"reported as unmanaged, identified by its public key. Read-only.",
	}, s.peerList)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "peer_add",
		Description: "Add a new WireGuard peer: generates a client keypair locally (the private key never " +
			"leaves this process) and a preshared key, and allocates the next free VPN address. The result " +
			"reports only non-secret fields (name, public key, VPN address); the client profile - which " +
			"contains the private key - is saved to the local vault and is deliberately NOT returned here, so " +
			"no secret lands in this conversation. Retrieve the ready-to-import .conf afterwards with the " +
			"`wgtui peer export` CLI command. With confirm=false (the default), only computes and returns the " +
			"plan - nothing on the server changes and no keys are generated. To actually apply it, call again " +
			"with the exact same arguments plus confirm=true. Never set confirm=true unless the user has seen " +
			"the plan from a confirm=false call and approved it.",
	}, s.peerAdd)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "peer_disable",
		Description: "Disable (pause) a peer: removed from the server's runtime and persistent config, but " +
			"its local profile and metadata are kept so it can be resumed later with peer_enable. " +
			"confirm=false (default) only previews; confirm=true applies. Never set confirm=true unless the " +
			"user has seen the confirm=false preview and approved it.",
	}, s.peerDisable)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "peer_enable",
		Description: "Resume a peer previously disabled with peer_disable, restoring the exact config entry " +
			"it had before. confirm=false (default) only previews; confirm=true applies. Never set confirm=true " +
			"unless the user has seen the confirm=false preview and approved it.",
	}, s.peerEnable)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "peer_revoke",
		Description: "Revoke a peer's server access. Deleting its local profile (which contains the private " +
			"key) and/or local metadata are separate, independent choices via delete_profile/delete_metadata - " +
			"revoking server access never implies either on its own. confirm=false (default) only previews; " +
			"confirm=true applies. Never set confirm=true unless the user has seen the confirm=false preview " +
			"and approved it, and never set delete_profile=true unless the user explicitly asked to delete the " +
			"saved profile too.",
	}, s.peerRevoke)

	mcp.AddTool(srv, &mcp.Tool{
		Name: "peer_rotate",
		Description: "Replace a peer's keypair: a fresh keypair is generated locally and swapped in for the " +
			"old one on the server in a single atomic change, and the old local profile is removed. " +
			"confirm=false (default) only previews; confirm=true applies. Never set confirm=true unless the " +
			"user has seen the confirm=false preview and approved it.",
	}, s.peerRotate)

	return srv
}
