package tui

import (
	"testing"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

func TestServerLabel(t *testing.T) {
	cases := []struct {
		name string
		s    domain.Server
		want string
	}{
		{
			name: "display name and user",
			s:    domain.Server{DisplayName: "office", SSHHostOrAlias: "1.2.3.4", SSHUser: "admin"},
			want: "office (admin@1.2.3.4)",
		},
		{
			name: "no display name falls back to host",
			s:    domain.Server{SSHHostOrAlias: "nas.example"},
			want: "nas.example",
		},
		{
			name: "display name equal to host is not duplicated",
			s:    domain.Server{DisplayName: "nas.example", SSHHostOrAlias: "nas.example"},
			want: "nas.example",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := serverLabel(tc.s); got != tc.want {
				t.Errorf("serverLabel(%+v) = %q, want %q", tc.s, got, tc.want)
			}
		})
	}
}
