package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
)

type diagnoseResult struct {
	State       discovery.State     `json:"state"`
	Checks      []diagnostics.Check `json:"checks"`
	Limitations []string            `json:"limitations"`
}

func runDiagnose(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "wgtui diagnose: expected the server ID as the first argument")
		return 2
	}
	id := args[0]

	fs := flag.NewFlagSet("diagnose", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output structured JSON instead of text")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "wgtui diagnose: unexpected extra argument(s): "+strings.Join(fs.Args(), " "))
		return 2
	}

	v, _, err := openOrCreateVault(stdout)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui diagnose: %v\n", err)
		return 1
	}
	server, ok := v.GetServer(id)
	if !ok {
		fmt.Fprintf(stderr, "wgtui diagnose: no server named %q\n", id)
		return 1
	}

	result, err := inspectServer(stdout, server)
	if err != nil {
		fmt.Fprintf(stderr, "wgtui diagnose: %v\n", err)
		return 1
	}
	diag := diagnoseResult{
		State:       result.State,
		Checks:      diagnostics.BuildChecks(result.Report, v.ListPeers(server.ID), time.Now()),
		Limitations: diagnostics.Limitations,
	}

	if *jsonOut {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(diag); err != nil {
			fmt.Fprintf(stderr, "wgtui diagnose: encode JSON: %v\n", err)
			return 1
		}
		return 0
	}
	printDiagnoseText(stdout, diag)
	return 0
}

func printDiagnoseText(out io.Writer, d diagnoseResult) {
	fmt.Fprintf(out, "State: %s\n\n", d.State)
	for _, c := range d.Checks {
		symbol := map[diagnostics.Status]string{
			diagnostics.StatusPass: "OK  ", diagnostics.StatusWarn: "WARN",
			diagnostics.StatusFail: "FAIL", diagnostics.StatusSkip: "SKIP",
		}[c.Status]
		line := fmt.Sprintf("[%s] %s", symbol, c.Name)
		if c.Detail != "" {
			line += " - " + c.Detail
		}
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "\nLimitations of this check:")
	for _, l := range d.Limitations {
		fmt.Fprintln(out, "  - "+l)
	}
}
