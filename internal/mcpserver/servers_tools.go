package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DMYTROSKORIN/wgtui/internal/diagnostics"
	"github.com/DMYTROSKORIN/wgtui/internal/discovery"
)

type ListServersArgs struct{}

type ServerSummary struct {
	ID             string `json:"id"`
	DisplayName    string `json:"display_name"`
	SSHHostOrAlias string `json:"ssh_host_or_alias"`
	SSHUser        string `json:"ssh_user,omitempty"`
	SSHPort        int    `json:"ssh_port,omitempty"`
}

func (s *Server) listServers(ctx context.Context, req *mcp.CallToolRequest, args ListServersArgs) (*mcp.CallToolResult, []ServerSummary, error) {
	out := make([]ServerSummary, 0, len(s.vault.Registry.Servers))
	for _, srv := range s.vault.Registry.Servers {
		out = append(out, ServerSummary{
			ID: srv.ID, DisplayName: srv.DisplayName, SSHHostOrAlias: srv.SSHHostOrAlias,
			SSHUser: srv.SSHUser, SSHPort: srv.SSHPort,
		})
	}
	return textResult(fmt.Sprintf("%d server(s) registered", len(out))), out, nil
}

type ServerIDArgs struct {
	ServerID string `json:"server_id" jsonschema:"the wgtui server ID from list_servers"`
	Credentials
}

type InspectionResult struct {
	State  discovery.State  `json:"state"`
	Report discovery.Report `json:"report"`
}

func (s *Server) serverInspect(ctx context.Context, req *mcp.CallToolRequest, args ServerIDArgs) (*mcp.CallToolResult, InspectionResult, error) {
	client, _, cleanup, err := s.connect(ctx, args.ServerID, args.Credentials)
	if err != nil {
		return nil, InspectionResult{}, err
	}
	defer cleanup()

	prompts := args.Credentials.prompts()
	report := discovery.Discover(ctx, client, prompts)
	state := discovery.Classify(report)

	return textResult(fmt.Sprintf("state=%s wireguard_present=%v interfaces=%d", state, report.WireGuard.WgPresent, len(report.WireGuard.Interfaces))),
		InspectionResult{State: state, Report: report}, nil
}

type DiagnoseResult struct {
	State       discovery.State     `json:"state"`
	Checks      []diagnostics.Check `json:"checks"`
	Limitations []string            `json:"limitations"`
}

func (s *Server) serverDiagnose(ctx context.Context, req *mcp.CallToolRequest, args ServerIDArgs) (*mcp.CallToolResult, DiagnoseResult, error) {
	client, _, cleanup, err := s.connect(ctx, args.ServerID, args.Credentials)
	if err != nil {
		return nil, DiagnoseResult{}, err
	}
	defer cleanup()

	prompts := args.Credentials.prompts()
	report := discovery.Discover(ctx, client, prompts)
	state := discovery.Classify(report)
	checks := diagnostics.BuildChecks(report, s.vault.ListPeers(args.ServerID), time.Now())

	warnOrFail := 0
	for _, c := range checks {
		if c.Status == diagnostics.StatusWarn || c.Status == diagnostics.StatusFail {
			warnOrFail++
		}
	}
	return textResult(fmt.Sprintf("state=%s, %d/%d checks need attention", state, warnOrFail, len(checks))),
		DiagnoseResult{State: state, Checks: checks, Limitations: diagnostics.Limitations}, nil
}
