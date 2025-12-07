package server

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterPrompts registers SmartThings prompts with MCP server.
func RegisterPrompts(s *mcp.Server) {
	// summarize_device
	s.AddPrompt(&mcp.Prompt{
		Name:        "summarize_device",
		Description: "Summarize the status of a SmartThings device",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "device_id",
				Description: "The ID of the device to summarize",
				Required:    true,
			},
		},
	}, func(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		var deviceID string
		if val, ok := req.Params.Arguments["device_id"]; ok {
			deviceID = val
		}

		if deviceID == "" {
			return &mcp.GetPromptResult{
				Description: "Error: device_id is required",
				Messages: []*mcp.PromptMessage{
					{
						Role: mcp.Role("user"),
						Content: &mcp.TextContent{
							Text: "Please provide a device_id to summarize.",
						},
					},
				},
			}, nil
		}

		// In a real implementation, we might fetch the device status here and pre-fill the prompt.
		// For now, we'll just return a prompt that asks the LLM to use the tools.
		return &mcp.GetPromptResult{
			Description: "Summarize device status",
			Messages: []*mcp.PromptMessage{
				{
					Role: mcp.Role("user"),
					Content: &mcp.TextContent{
						Text: "Please summarize the status of the SmartThings device with ID: " + deviceID + ". Use the get_device and get_device_status tools to retrieve the necessary information.",
					},
				},
			},
		}, nil
	})
}
