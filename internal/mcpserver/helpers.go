package mcpserver

import "github.com/modelcontextprotocol/go-sdk/mcp"

// textResult wraps a short human-readable summary alongside the
// structured Out value every handler also returns - the SDK renders Out
// as the tool's structured content automatically; this text is what a
// client shows inline without necessarily parsing structured content.
func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
