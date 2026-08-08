package tui

import (
	"fmt"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
)

// serverLabel is a short, unambiguous identifier for a registered server,
// used in every connection state and credential modal so it's always clear
// *which* machine a question is about.
func serverLabel(s domain.Server) string {
	host := s.SSHHostOrAlias
	if s.SSHUser != "" {
		host = s.SSHUser + "@" + host
	}
	if s.DisplayName != "" && s.DisplayName != host {
		return fmt.Sprintf("%s (%s)", s.DisplayName, host)
	}
	return host
}
