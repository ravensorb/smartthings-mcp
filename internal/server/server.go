package server

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/langowarny/smartthings-mcp/internal/smartthings"
)

// NewMCPServer creates and initializes a new MCP server.
func NewMCPServer(logger *zap.SugaredLogger, client *smartthings.Client) *mcp.Server {
	// Initialize the server implementation info
	impl := &mcp.Implementation{
		Name:    "SmartThings MCP",
		Version: "0.1.0",
	}

	// Create the server instance
	s := mcp.NewServer(impl, &mcp.ServerOptions{
		HasTools:     true,
		HasResources: true,
		HasPrompts:   true,
	})

	// Register tools
	RegisterTools(s, client)

	// Register resources
	RegisterResources(s, client)

	// Register prompts
	RegisterPrompts(s)

	return s
}
