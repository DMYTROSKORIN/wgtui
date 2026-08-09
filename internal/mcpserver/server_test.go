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

// TestToolOutputSchemasAreObjects guards a real bug reported by an actual
// MCP client (Claude Code): list_servers and peer_list returned a bare
// Go slice ([]ServerSummary / []peerview.Peer) from their handler, which
// AddTool's reflection-based schema inference turns into an outputSchema of
// "type": "array" instead of "object". The MCP spec requires structured
// tool output to be a JSON object, so a client enforcing that rejects the
// tool during listing with "Invalid input: expected 'object'" - which
// breaks discovery for every tool this server registers, not just the
// offending one, since tools/list returns them all in one response. Every
// tool's outputSchema must describe an object at the top level (or have no
// outputSchema at all, for a tool with no structured output).
func TestToolOutputSchemasAreObjects(t *testing.T) {
	cs := newTestSession(t)
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) == 0 {
		t.Fatal("ListTools returned no tools")
	}
	for _, tool := range res.Tools {
		if tool.OutputSchema == nil {
			continue // no structured output declared - fine
		}
		schema, ok := tool.OutputSchema.(map[string]any)
		if !ok {
			t.Errorf("tool %q: outputSchema is %T, want map[string]any after JSON round-trip", tool.Name, tool.OutputSchema)
			continue
		}
		if got := schema["type"]; got != "object" {
			t.Errorf("tool %q: outputSchema[\"type\"] = %v, want \"object\"", tool.Name, got)
		}
	}
}

func TestListServers(t *testing.T) {
	cs := newTestSession(t)
	out, res := callTool[ServerListResult](t, cs, "list_servers", ListServersArgs{})
	if res.IsError {
		t.Fatalf("list_servers returned an error result: %+v", res)
	}
	if len(out.Servers) != 1 || out.Servers[0].ID != "nas" {
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
