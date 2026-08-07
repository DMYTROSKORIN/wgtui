package tui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// dial connects to s and returns an open session. tui keeps its own copy
// of this rather than importing internal/cli's cli-private connectToServer
// - cli and tui stay independent front ends over the shared lower layers
// (docs/ARCHITECTURE.md), the same reasoning internal/mcpserver/connect.go
// already follows for the same reason.
//
// ctx bounds the dial itself, not the session's lifetime: a tui session
// stays open across an interactive confirm screen the user might sit on
// for a while (unlike a CLI command, which never pauses for that), so
// cleanup must not reuse a ctx that could have since expired - it would
// silently skip `ssh -O exit` instead of closing the connection.
func dial(ctx context.Context, s domain.Server, prompts sshtransport.Prompts) (*session, error) {
	controlDir, err := os.MkdirTemp("", "wgtui-ssh-*")
	if err != nil {
		return nil, err
	}
	client := sshtransport.NewClient(sshtransport.Target{
		HostOrAlias: s.SSHHostOrAlias, Port: s.SSHPort, User: s.SSHUser, IdentityFile: s.SSHIdentityFile,
	}, filepath.Join(controlDir, "cm.sock"))

	if err := client.Dial(ctx, prompts); err != nil {
		os.RemoveAll(controlDir)
		return nil, err
	}
	return &session{
		client:  client,
		prompts: prompts,
		cleanup: func() {
			client.Close(context.Background())
			os.RemoveAll(controlDir)
		},
	}, nil
}
