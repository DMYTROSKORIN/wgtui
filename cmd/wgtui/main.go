// Command wgtui is the entry point for the wgtui CLI/TUI.
package main

import (
	"os"

	"github.com/DMYTROSKORIN/wgtui/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
