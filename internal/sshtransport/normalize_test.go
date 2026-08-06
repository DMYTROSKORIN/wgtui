package sshtransport

import "testing"

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
