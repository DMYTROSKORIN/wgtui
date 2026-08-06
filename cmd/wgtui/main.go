// Command wgtui is the entry point for the wgtui CLI/TUI.
package main

import (
	"fmt"
	"os"
)

// version is set to a real value once release builds exist (docs/ARCHITECTURE.md, stage 5).
const version = "0.0.0-dev"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version") {
		fmt.Println("wgtui " + version)
		return
	}
	fmt.Fprintln(os.Stderr, "wgtui: not implemented yet (stage 0 in progress, see docs/ARCHITECTURE.md)")
	os.Exit(1)
}
