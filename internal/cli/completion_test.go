package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCompletion_KnownShells(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var out, errBuf bytes.Buffer
		if code := runCompletion([]string{shell}, &out, &errBuf); code != 0 {
			t.Fatalf("runCompletion(%s) = %d, want 0; stderr: %s", shell, code, errBuf.String())
		}
		if !strings.Contains(out.String(), "wgtui") {
			t.Errorf("runCompletion(%s) output doesn't mention wgtui:\n%s", shell, out.String())
		}
	}
}

func TestRunCompletion_UnknownShell(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := runCompletion([]string{"powershell"}, &out, &errBuf); code != 2 {
		t.Errorf("runCompletion(powershell) = %d, want 2", code)
	}
}

func TestRunCompletion_WrongArgCount(t *testing.T) {
	var out, errBuf bytes.Buffer
	if code := runCompletion(nil, &out, &errBuf); code != 2 {
		t.Errorf("runCompletion() = %d, want 2", code)
	}
	if code := runCompletion([]string{"bash", "zsh"}, &out, &errBuf); code != 2 {
		t.Errorf("runCompletion(bash, zsh) = %d, want 2", code)
	}
}
