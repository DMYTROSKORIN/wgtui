package cli

import (
	"testing"
)

func TestTunnelModeLabel(t *testing.T) {
	if got := tunnelModeLabel(true); got != "full tunnel" {
		t.Errorf("tunnelModeLabel(true) = %q", got)
	}
	if got := tunnelModeLabel(false); got != "split tunnel" {
		t.Errorf("tunnelModeLabel(false) = %q", got)
	}
}

func TestOutputDescription(t *testing.T) {
	if got := outputDescription(""); got != "no file export" {
		t.Errorf("outputDescription(\"\") = %q", got)
	}
	if got := outputDescription("/tmp/x.conf"); got != "/tmp/x.conf" {
		t.Errorf("outputDescription(path) = %q", got)
	}
}
