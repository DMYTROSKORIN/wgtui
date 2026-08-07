package transaction

import (
	"strings"
	"testing"
)

func TestParseOutcome(t *testing.T) {
	plan := validPlan()
	cases := []struct {
		name        string
		out         string
		exitCode    int
		wantApplied bool
		wantReason  string
		wantErr     bool
	}{
		{"lock failed", "WGTUI_LOCK_FAILED\n", 1, false, "lock_failed", true},
		{"fingerprint mismatch", "WGTUI_FINGERPRINT_MISMATCH\n", 1, false, "fingerprint_mismatch", true},
		{"validate failed", "WGTUI_VALIDATE_FAILED\nsome wg-quick error\n", 1, false, "validate_failed", true},
		{"apply failed rolled back", "WGTUI_APPLY_FAILED_ROLLED_BACK\nsyncconf error\n", 1, false, "apply_failed_rolled_back", true},
		{"recovery required", "WGTUI_RECOVERY_REQUIRED /etc/wireguard/wg0.conf.bak.20260101120000\n", 1, false, "recovery_required", true},
		{"success", "WGTUI_OK\n", 0, true, "", false},
		{"unexpected output", "something went sideways\n", 1, false, "", true},
		{"exit nonzero without marker", "", 1, false, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := parseOutcome(tc.out, tc.exitCode, plan)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if outcome.Applied != tc.wantApplied {
				t.Errorf("Applied = %v, want %v", outcome.Applied, tc.wantApplied)
			}
			if outcome.Reason != tc.wantReason {
				t.Errorf("Reason = %q, want %q", outcome.Reason, tc.wantReason)
			}
		})
	}
}

func TestParseOutcome_ExtractsRecoveryPathOnlyWhenNeeded(t *testing.T) {
	outcome, err := parseOutcome("WGTUI_RECOVERY_REQUIRED /etc/wireguard/wg0.conf.bak.20260101120000\n", 1, validPlan())
	if err == nil {
		t.Fatal("expected recovery-required error")
	}
	want := "/etc/wireguard/wg0.conf.bak.20260101120000"
	if outcome.BackupPath != want {
		t.Errorf("BackupPath = %q, want %q", outcome.BackupPath, want)
	}
}

func TestParseOutcome_RawOutputNeverEmptyOnFailure(t *testing.T) {
	// Sanity check that troubleshooting output survives into Outcome even
	// on the generic failure path.
	outcome, err := parseOutcome("garbage\n", 3, validPlan())
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(outcome.RawOutput, "garbage") {
		t.Errorf("RawOutput = %q, want it to contain the raw script output", outcome.RawOutput)
	}
}
