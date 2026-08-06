package cli

import "testing"

func TestShortenKey(t *testing.T) {
	cases := map[string]string{
		"short":                            "short",
		"exactly12ch=":                     "exactly12ch=",
		"4LZENKxxxxxxxxxxxxxxxxxxxxX5WhY=": "4LZENKxx…",
	}
	for in, want := range cases {
		if got := shortenKey(in); got != want {
			t.Errorf("shortenKey(%q) = %q, want %q", in, got, want)
		}
	}
}
