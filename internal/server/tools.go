package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/langowarny/smartthings-mcp/internal/smartthings"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterTools registers SmartThings tools with MCP server.
func RegisterTools(s *mcp.Server, client *smartthings.Client) {
	// list_devices
	s.AddTool(&mcp.Tool{
		Name:        "list_devices",
		Description: "List SmartThings devices",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location_id": map[string]interface{}{
					"type":        "string",
					"description": "Optional location ID to filter devices",
				},
			},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		loc, _ := args["location_id"].(string)

		var devices []smartthings.Device
		var err error
		if loc != "" {
			devices, err = client.ListDevicesByLocation(loc)
		} else {
			devices, err = client.ListDevices()
		}
		if err != nil {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{
					&mcp.TextContent{
						Text: fmt.Sprintf("Error: %v", err),
					},
				},
			}, nil
		}
		data, _ := json.Marshal(devices)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{
					Text: string(data),
				},
			},
		}, nil
	})

	// get_device
	s.AddTool(&mcp.Tool{
		Name:        "get_device",
		Description: "Get SmartThings device metadata",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"device_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the device to retrieve",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id required"), nil
		}
		d, err := client.GetDevice(id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(d)
		return successResult(string(data)), nil
	})

	// get_device_status
	s.AddTool(&mcp.Tool{
		Name:        "get_device_status",
		Description: "Get SmartThings device status",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"device_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the device to check status for",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, ok := args["device_id"].(string)
		if !ok {
			return errorResult("device_id is required"), nil
		}
		status, err := client.GetDeviceStatus(id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(status)
		return successResult(string(data)), nil
	})

	// list_device_capabilities
	s.AddTool(&mcp.Tool{
		Name:        "list_device_capabilities",
		Description: "List capabilities supported by a SmartThings device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"device_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the device to list capabilities for",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id required"), nil
		}
		dev, err := client.GetDevice(id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		capSet := make(map[string]struct{})
		for _, comp := range dev.Components {
			for _, capab := range comp.Capabilities {
				capSet[capab.ID] = struct{}{}
			}
		}
		caps := make([]string, 0, len(capSet))
		for c := range capSet {
			caps = append(caps, c)
		}
		data, _ := json.Marshal(caps)
		return successResult(string(data)), nil
	})

	// send_device_command
	s.AddTool(&mcp.Tool{
		Name:        "send_device_command",
		Description: "Send command to SmartThings device",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"device_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the device to command",
				},
				"component": map[string]interface{}{
					"type":        "string",
					"description": "The component ID (default: main)",
				},
				"capability": map[string]interface{}{
					"type":        "string",
					"description": "The capability ID (e.g., switch)",
				},
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The command to execute (e.g., on, off)",
				},
				"arguments": map[string]interface{}{
					"type":        "array",
					"description": "List of arguments for the command",
				},
			},
			"required": []string{"device_id", "capability", "command"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		component, _ := args["component"].(string)
		if component == "" {
			component = "main"
		}
		capability, _ := args["capability"].(string)
		command, _ := args["command"].(string)
		arguments, _ := args["arguments"].([]interface{})

		body := map[string]interface{}{
			"commands": []interface{}{
				map[string]interface{}{
					"component":  component,
					"capability": capability,
					"command":    command,
					"arguments":  arguments,
				},
			},
		}
		if err := client.SendDeviceCommand(id, body); err != nil {
			return errorResult(err.Error()), nil
		}
		return successResult("ok"), nil
	})

	// list_locations
	s.AddTool(&mcp.Tool{
		Name:        "list_locations",
		Description: "List SmartThings locations",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		locs, err := client.ListLocations()
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(locs)
		return successResult(string(data)), nil
	})

	// execute_scene
	s.AddTool(&mcp.Tool{
		Name:        "execute_scene",
		Description: "Execute SmartThings scene",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"scene_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the scene to execute",
				},
			},
			"required": []string{"scene_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		sceneID, _ := args["scene_id"].(string)
		resp, err := client.ExecuteScene(sceneID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		out, _ := json.Marshal(resp)
		return successResult(string(out)), nil
	})

	// list_scenes
	s.AddTool(&mcp.Tool{
		Name:        "list_scenes",
		Description: "List SmartThings scenes",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		scenes, err := client.ListScenes()
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(scenes)
		return successResult(string(data)), nil
	})

	// list_rooms
	s.AddTool(&mcp.Tool{
		Name:        "list_rooms",
		Description: "List rooms in a SmartThings location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the location to list rooms for",
				},
			},
			"required": []string{"location_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		if locID == "" {
			return errorResult("location_id is required"), nil
		}
		rooms, err := client.ListRooms(locID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(rooms)
		return successResult(string(data)), nil
	})

	// create_room
	s.AddTool(&mcp.Tool{
		Name:        "create_room",
		Description: "Create a new room in a SmartThings location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the location to create room in",
				},
				"name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the new room",
				},
			},
			"required": []string{"location_id", "name"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		name, _ := args["name"].(string)
		if locID == "" || name == "" {
			return errorResult("location_id and name are required"), nil
		}
		room, err := client.CreateRoom(locID, name)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(room)
		return successResult(string(data)), nil
	})

	// delete_room
	s.AddTool(&mcp.Tool{
		Name:        "delete_room",
		Description: "Delete a room from a SmartThings location",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"location_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the location",
				},
				"room_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the room to delete",
				},
			},
			"required": []string{"location_id", "room_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		roomID, _ := args["room_id"].(string)
		if locID == "" || roomID == "" {
			return errorResult("location_id and room_id are required"), nil
		}
		if err := client.DeleteRoom(locID, roomID); err != nil {
			return errorResult(err.Error()), nil
		}
		return successResult("ok"), nil
	})

	// list_rules
	s.AddTool(&mcp.Tool{
		Name:        "list_rules",
		Description: "List SmartThings automation rules",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rules, err := client.ListRules()
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(rules)
		return successResult(string(data)), nil
	})

	// list_hubs
	s.AddTool(&mcp.Tool{
		Name:        "list_hubs",
		Description: "List SmartThings hubs",
		InputSchema: map[string]interface{}{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		hubs, err := client.ListHubs()
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(hubs)
		return successResult(string(data)), nil
	})

	// get_hub_health
	s.AddTool(&mcp.Tool{
		Name:        "get_hub_health",
		Description: "Get health status of a SmartThings hub",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"hub_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the hub",
				},
			},
			"required": []string{"hub_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]interface{}
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		hubID, _ := args["hub_id"].(string)
		if hubID == "" {
			return errorResult("hub_id is required"), nil
		}
		health, err := client.GetHubHealth(hubID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		data, _ := json.Marshal(health)
		return successResult(string(data)), nil
	})
}

func successResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
			},
		},
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: msg,
			},
		},
	}
}
