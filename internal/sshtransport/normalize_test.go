package sshtransport

import (
	"errors"
	"testing"
)

func TestNormalizePTYCapture(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "sudo line-clearing artifact plus CRLF, as seen against the real test server",
			in:   " \r\n[Interface]\r\nAddress = 10.10.0.1/24\r\nListenPort = 51820\r\n",
			want: "[Interface]\nAddress = 10.10.0.1/24\nListenPort = 51820\n",
		},
		{
			name: "already clean input is unchanged",
			in:   "[Interface]\nAddress = 10.10.0.1/24\n",
			want: "[Interface]\nAddress = 10.10.0.1/24\n",
		},
		{
			name: "no leading artifact, just CRLF",
			in:   "line one\r\nline two\r\n",
			want: "line one\nline two\n",
		},
		{
			name: "empty input",
			in:   "",
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := string(normalizePTYCapture([]byte(tc.in))); got != tc.want {
				t.Errorf("normalizePTYCapture(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestInteractiveResultPreservesFailureDiagnostics(t *testing.T) {
	failed := interactiveResult([]byte(" \r\nssh: connect to host vpn.example: Connection timed out\r\n"), errors.New("exit status 255"))
	if len(failed.Stdout) != 0 {
		t.Fatalf("failed command stdout = %q, want empty", failed.Stdout)
	}
	if got, want := string(failed.Stderr), "ssh: connect to host vpn.example: Connection timed out\n"; got != want {
		t.Fatalf("failed command stderr = %q, want %q", got, want)
	}

	success := interactiveResult([]byte("peer data\r\n"), nil)
	if len(success.Stderr) != 0 || string(success.Stdout) != "peer data\n" {
		t.Fatalf("successful command result = %#v", success)
	}
}
