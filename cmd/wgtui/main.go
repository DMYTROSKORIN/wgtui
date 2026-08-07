// Command wgtui is the entry point for the wgtui CLI/TUI.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/DMYTROSKORIN/wgtui/internal/cli"
	"github.com/DMYTROSKORIN/wgtui/internal/mcpserver"
	"github.com/DMYTROSKORIN/wgtui/internal/tui"
)

func main() {
	// No arguments launches the interactive TUI (SPEC.md §2); "mcp" runs
	// the MCP server front end over stdio; any other subcommand goes
	// through the non-interactive CLI (§21). main is the only place that
	// decides between them - cli, tui, and mcpserver are independent
	// front ends over the same lower layers (docs/ARCHITECTURE.md) and
	// none of them calls another.
	if len(os.Args) <= 1 {
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "wgtui:", err)
			os.Exit(1)
		}
		return
	}
	if os.Args[1] == "mcp" {
		if err := mcpserver.Run(context.Background()); err != nil {
			fmt.Fprintln(os.Stderr, "wgtui mcp:", err)
			os.Exit(1)
		}
		return
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
