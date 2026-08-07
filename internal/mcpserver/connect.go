package mcpserver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/sshtransport"
)

// connect resolves serverID in the server's vault, dials it, and returns a
// ready client plus a cleanup func. Mirrors internal/cli's connectToServer
// but takes Credentials instead of driving a terminal - see credentials.go.
func (s *Server) connect(ctx context.Context, serverID string, creds Credentials) (*sshtransport.Client, domain.Server, func(), error) {
	target, ok := s.vault.GetServer(serverID)
	if !ok {
		return nil, domain.Server{}, nil, fmt.Errorf("no server registered with ID %q - call list_servers to see what's available", serverID)
	}

	controlDir, err := os.MkdirTemp("", "wgtui-mcp-ssh-*")
	if err != nil {
		return nil, domain.Server{}, nil, fmt.Errorf("create control socket directory: %w", err)
	}

	client := sshtransport.NewClient(sshtransport.Target{
		HostOrAlias:  target.SSHHostOrAlias,
		Port:         target.SSHPort,
		User:         target.SSHUser,
		IdentityFile: target.SSHIdentityFile,
	}, filepath.Join(controlDir, "cm.sock"))

	prompts := creds.prompts()
	if err := client.Dial(ctx, prompts); err != nil {
		os.RemoveAll(controlDir)
		return nil, domain.Server{}, nil, fmt.Errorf("connect: %w", err)
	}

	cleanup := func() {
		client.Close(ctx)
		os.RemoveAll(controlDir)
	}
	return client, target, cleanup, nil
}
