package mcpserver

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// MasterPasswordEnvVar is the only way to supply the vault's master
// password to Run - see its doc comment for why.
const MasterPasswordEnvVar = "WGTUI_MASTER_PASSWORD"

// Run bootstraps the local vault and serves wgtui's MCP tools over stdio
// until ctx is canceled or the client disconnects. Unlike cli's
// openOrCreateVault, there is no terminal to prompt on - stdin/stdout
// *are* the MCP protocol channel (see credentials.go) - so unlike cli,
// Run cannot create a new vault or interactively retry a wrong password:
// the vault must already exist (create it once via any `wgtui` CLI
// command, e.g. `wgtui server add`) and its master password must be
// supplied via MasterPasswordEnvVar.
func Run(ctx context.Context) error {
	path, err := profilevault.VaultPath()
	if err != nil {
		return err
	}
	if !profilevault.Exists(path) {
		return fmt.Errorf("no vault found at %s - run any `wgtui` CLI command first to create one (wgtui mcp cannot prompt for a new master password over stdio)", path)
	}

	pw := os.Getenv(MasterPasswordEnvVar)
	if pw == "" {
		return fmt.Errorf("%s is not set - wgtui mcp cannot prompt for the master password over stdio", MasterPasswordEnvVar)
	}
	v, err := profilevault.Open(path, pw)
	if err != nil {
		return fmt.Errorf("unlock vault: %w", err)
	}

	return New(v, path).Run(ctx, &mcp.StdioTransport{})
}
