// Command wgtui is the entry point for the wgtui CLI/TUI.
package main

import (
	"fmt"
	"os"

	"github.com/DMYTROSKORIN/wgtui/internal/cli"
	"github.com/DMYTROSKORIN/wgtui/internal/tui"
)

func main() {
	// No arguments launches the interactive TUI (SPEC.md §2); any
	// subcommand goes through the non-interactive CLI (§21). main is the
	// only place that decides between them - cli and tui are independent
	// front ends over the same lower layers (docs/ARCHITECTURE.md) and
	// neither calls the other.
	if len(os.Args) <= 1 {
		if err := tui.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "wgtui:", err)
			os.Exit(1)
		}
		return
	}
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
