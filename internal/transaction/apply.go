// Package transaction implements the locked backup-write-validate-apply-
// verify-rollback sequence (SPEC.md §15.2) used by every mutating
// peer/interface operation. See script.go's buildScript doc comment for
// the step-by-step mapping to the spec.
package transaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// Outcome is the parsed result of a transaction attempt.
type Outcome struct {
	Applied    bool
	BackupPath string
	RolledBack bool
	// Reason is set for any non-Applied outcome: "lock_failed",
	// "fingerprint_mismatch", "validate_failed", or
	// "apply_failed_rolled_back".
	Reason string
	// RawOutput is the full remote stdout+stderr, useful for
	// troubleshooting a failure. It never contains a private key: the
	// script only ever echoes its own control markers, file paths, and
	// `wg-quick`/`wg` error text, never the config content it wrote.
	RawOutput string
}

// Apply runs Plan's mutation as a single remote script (script.go) over
// an already-Dialed client. It either fully succeeds (Outcome.Applied)
// or leaves the remote interface exactly as it was, with a specific
// Reason explaining why.
func Apply(ctx context.Context, client *sshtransport.Client, plan Plan, prompts sshtransport.Prompts) (Outcome, error) {
	script, err := buildScript(plan)
	if err != nil {
		return Outcome{}, err
	}

	res, err := client.RunSudoScript(ctx, script, prompts)
	if err != nil {
		return Outcome{}, fmt.Errorf("run transaction script: %w", err)
	}
	return parseOutcome(string(res.Stdout), res.ExitCode, plan)
}

// parseOutcome interprets the transaction script's stdout markers. Split
// out from Apply so it can be unit tested against captured sample output
// without a network dependency.
func parseOutcome(out string, exitCode int, plan Plan) (Outcome, error) {
	switch {
	case strings.Contains(out, "WGTUI_LOCK_FAILED"):
		return Outcome{Reason: "lock_failed", RawOutput: out},
			fmt.Errorf("another wgtui operation is already in progress on interface %s", plan.Interface)
	case strings.Contains(out, "WGTUI_FINGERPRINT_MISMATCH"):
		return Outcome{Reason: "fingerprint_mismatch", RawOutput: out},
			fmt.Errorf("%s changed on the server since it was last read - re-run discovery/inspect and try again", plan.ConfigPath)
	case strings.Contains(out, "WGTUI_VALIDATE_FAILED"):
		return Outcome{Reason: "validate_failed", RawOutput: out},
			fmt.Errorf("new config failed wg-quick validation, nothing was changed:\n%s", out)
	case strings.Contains(out, "WGTUI_APPLY_FAILED_ROLLED_BACK"):
		return Outcome{Reason: "apply_failed_rolled_back", RolledBack: true, RawOutput: out},
			fmt.Errorf("applying the new config failed; the backup was restored:\n%s", out)
	}

	idx := strings.Index(out, "WGTUI_OK")
	if idx == -1 || exitCode != 0 {
		return Outcome{RawOutput: out}, fmt.Errorf("transaction script did not report success (exit %d):\n%s", exitCode, out)
	}
	backup := strings.TrimSpace(strings.TrimPrefix(out[idx:], "WGTUI_OK"))
	if fields := strings.Fields(backup); len(fields) > 0 {
		backup = fields[0]
	}
	return Outcome{Applied: true, BackupPath: backup, RawOutput: out}, nil
}
