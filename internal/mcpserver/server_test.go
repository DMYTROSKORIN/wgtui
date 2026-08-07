package mcpserver

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/DMYTROSKORIN/wgtui/internal/domain"
	"github.com/DMYTROSKORIN/wgtui/internal/profilevault"
)

// newTestSession builds a vault (with one registered server, no network
// access needed to build it), serves it over an in-memory transport, and
// connects a client to it - exercising the real MCP protocol encode/decode
// path, not just calling the Go handler functions directly.
func newTestSession(t *testing.T) *mcp.ClientSession {
	t.Helper()
	v, err := profilevault.New("test-master-password")
	if err != nil {
		t.Fatalf("profilevault.New: %v", err)
	}
	if err := v.AddServer(domain.Server{
		ID: "nas", DisplayName: "NAS", SSHHostOrAlias: "nas.example", SSHUser: "root", SSHPort: 22,
	}); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	srv := New(v, "")
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server Connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.0"}, nil)
	cs, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client Connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func callTool[Out any](t *testing.T, cs *mcp.ClientSession, name string, args any) (Out, *mcp.CallToolResult) {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	var out Out
	if res.IsError {
		return out, res
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured content into %T: %v", out, err)
	}
	return out, res
}

func TestListServers(t *testing.T) {
	cs := newTestSession(t)
	out, res := callTool[[]ServerSummary](t, cs, "list_servers", ListServersArgs{})
	if res.IsError {
		t.Fatalf("list_servers returned an error result: %+v", res)
	}
	if len(out) != 1 || out[0].ID != "nas" {
		t.Fatalf("list_servers = %+v, want one server with ID 'nas'", out)
	}
}

func TestPeerAdd_UnknownServer_Errors(t *testing.T) {
	cs := newTestSession(t)
	_, res := callTool[PeerAddResult](t, cs, "peer_add", PeerAddArgs{ServerID: "does-not-exist", Name: "alice"})
	if !res.IsError {
		t.Fatalf("expected an error result for an unregistered server_id, got %+v", res)
	}
}

func TestPeerAdd_MissingName_Errors(t *testing.T) {
	cs := newTestSession(t)
	_, res := callTool[PeerAddResult](t, cs, "peer_add", PeerAddArgs{ServerID: "nas"})
	if !res.IsError {
		t.Fatalf("expected an error result when name is empty, got %+v", res)
	}
}

func TestPeerDisable_UnknownPeer_Errors(t *testing.T) {
	cs := newTestSession(t)
	_, res := callTool[PeerActionResult](t, cs, "peer_disable", PeerTargetArgs{ServerID: "nas", Name: "ghost"})
	if !res.IsError {
		t.Fatalf("expected an error result for an unregistered peer name, got %+v", res)
	}
}
