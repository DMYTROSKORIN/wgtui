package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// connectToServer resolves serverID in v, dials it, and returns a ready
// client plus a cleanup func that closes the connection and removes its
// control-socket directory. Callers must always run the returned cleanup
// (e.g. via defer) once err is nil.
func connectToServer(ctx context.Context, out io.Writer, v *profilevault.Vault, serverID string) (*sshtransport.Client, domain.Server, sshtransport.Prompts, func(), error) {
	server, ok := v.GetServer(serverID)
	if !ok {
		return nil, domain.Server{}, sshtransport.Prompts{}, nil, fmt.Errorf("no server named %q (use 'wgtui server add' first)", serverID)
	}

	controlDir, dirCleanup, err := newControlDir()
	if err != nil {
		return nil, domain.Server{}, sshtransport.Prompts{}, nil, err
	}

	client := sshtransport.NewClient(sshtransport.Target{
		HostOrAlias:  server.SSHHostOrAlias,
		Port:         server.SSHPort,
		User:         server.SSHUser,
		IdentityFile: server.SSHIdentityFile,
	}, filepath.Join(controlDir, "cm.sock"))

	prompts := newTerminalPrompts(out)
	if err := client.Dial(ctx, prompts); err != nil {
		dirCleanup()
		return nil, domain.Server{}, sshtransport.Prompts{}, nil, fmt.Errorf("connect: %w", err)
	}

	cleanup := func() {
		client.Close(ctx)
		dirCleanup()
	}
	return client, server, prompts, cleanup, nil
}
