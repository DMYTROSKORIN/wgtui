package mcpserver

import (
	"fmt"

	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// Credentials are supplied per tool call, never cached across calls or
// written to disk. This differs from cli/tui, which prompt interactively
// on the controlling terminal: MCP's stdio transport *is* the protocol
// channel, so there is no terminal left to prompt on, and no human
// sitting at one to answer even if there were. See
// docs/THREAT_MODEL.md's MCP section for the full trade-off this implies
// (a password passed as a tool argument lands in the calling agent's
// context/transcript, which is a different and generally wider exposure
// than a value typed into a local terminal prompt).
type Credentials struct {
	SSHPassword  string `json:"ssh_password,omitempty" jsonschema:"SSH login password for this one call. Only pass this if the user has explicitly provided it for this specific action and the target doesn't use key-based auth - never invent one or reuse one mentioned earlier in the conversation for a different server."`
	SudoPassword string `json:"sudo_password,omitempty" jsonschema:"sudo password on the target server for this one call. Only pass this if the user has explicitly provided it for this specific action - never invent one or reuse one mentioned earlier in the conversation for a different server. Held in memory only for the duration of this single tool call."`
}

// prompts turns Credentials into an sshtransport.Prompts. Host-key
// confirmation always refuses: there is no terminal to show the
// fingerprint on and no human present to verify it out of band, so a
// first-time connection must be established once from a real terminal
// (e.g. `wgtui server inspect`) before that server can be used through
// MCP (SPEC.md §10.2 - refusing an unverified host key is the safe
// default, not a missing feature).
func (c Credentials) prompts() sshtransport.Prompts {
	return sshtransport.Prompts{
		HostKey: func(prompt string) bool { return false },
		Password: func() (string, error) {
			if c.SSHPassword == "" {
				return "", fmt.Errorf("this server needs an SSH password and none was supplied - retry with ssh_password set")
			}
			return c.SSHPassword, nil
		},
		SudoPassword: func() (string, error) {
			if c.SudoPassword == "" {
				return "", fmt.Errorf("this operation needs a sudo password and none was supplied - retry with sudo_password set")
			}
			return c.SudoPassword, nil
		},
	}
}
