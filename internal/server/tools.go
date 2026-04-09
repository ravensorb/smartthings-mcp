package server

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"

	"github.com/langowarny/smartthings-mcp/internal/smartthings"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Bool pointer helpers for ToolAnnotations.
func boolPtr(b bool) *bool { return &b }

// Common tool annotation presets.
var (
	// readOnly: tool only reads data, no side effects.
	readOnly = &mcp.ToolAnnotations{
		ReadOnlyHint:    true,
		DestructiveHint: boolPtr(false),
		IdempotentHint:  true,
		OpenWorldHint:   boolPtr(true),
	}
	// safeWrite: tool creates or updates data but is not destructive.
	safeWrite = &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: boolPtr(false),
		IdempotentHint:  false,
		OpenWorldHint:   boolPtr(true),
	}
	// idempotentWrite: tool updates data idempotently (e.g., set mode, update device).
	idempotentWrite = &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: boolPtr(false),
		IdempotentHint:  true,
		OpenWorldHint:   boolPtr(true),
	}
	// destructive: tool deletes or may cause irreversible changes.
	destructive = &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: boolPtr(true),
		IdempotentHint:  false,
		OpenWorldHint:   boolPtr(true),
	}
	// sideEffect: tool triggers physical actions (commands, scenes, rules).
	sideEffect = &mcp.ToolAnnotations{
		ReadOnlyHint:    false,
		DestructiveHint: boolPtr(true),
		IdempotentHint:  false,
		OpenWorldHint:   boolPtr(true),
	}
)

// RegisterTools registers SmartThings tools with MCP server.
func RegisterTools(s *mcp.Server, client *smartthings.Client) {
	// list_devices
	s.AddTool(&mcp.Tool{
		Name:        "list_devices",
		Description: "List SmartThings devices",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "Optional location ID to filter devices. If not provided, returns devices from all locations.",
				},
			},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		loc, _ := args["location_id"].(string)

		var devices []smartthings.Device
		var err error
		if loc != "" {
			devices, err = client.ListDevicesByLocation(ctx, loc)
		} else {
			devices, err = client.ListDevices(ctx)
		}
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(devices, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// get_device
	s.AddTool(&mcp.Tool{
		Name:        "get_device",
		Description: "Get SmartThings device metadata",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to retrieve (e.g., UUID)",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id required"), nil
		}
		d, err := client.GetDevice(ctx, id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(d, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// get_device_status
	s.AddTool(&mcp.Tool{
		Name:        "get_device_status",
		Description: "Get SmartThings device status",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to check status for",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		status, err := client.GetDeviceStatus(ctx, id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(status, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// get_device_preferences
	s.AddTool(&mcp.Tool{
		Name:        "get_device_preferences",
		Description: "Get preferences for a SmartThings device",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to retrieve preferences for",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		prefs, err := client.GetDevicePreferences(ctx, id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(prefs, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// update_device_preferences
	s.AddTool(&mcp.Tool{
		Name:        "update_device_preferences",
		Description: "Update preferences for a SmartThings device (e.g., motion sensitivity, LED settings)",
		Annotations: idempotentWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device",
				},
				"preferences": map[string]any{
					"type":        "object",
					"description": "Key-value map of preferences to set (e.g., {\"parameter101\": 3}). Use get_device_preferences first to discover available keys.",
				},
			},
			"required": []string{"device_id", "preferences"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		prefs, ok := args["preferences"].(map[string]any)
		if !ok || len(prefs) == 0 {
			return errorResult("preferences must be a non-empty object"), nil
		}
		result, err := client.UpdateDevicePreferences(ctx, id, prefs)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(result, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0)
	})

	// get_device_health
	s.AddTool(&mcp.Tool{
		Name:        "get_device_health",
		Description: "Check if a SmartThings device is online or offline",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		health, err := client.GetDeviceHealth(ctx, id)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(health, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// delete_device
	s.AddTool(&mcp.Tool{
		Name:        "delete_device",
		Description: "Remove a SmartThings device. WARNING: This is irreversible and may break automations that reference this device.",
		Annotations: destructive,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to remove",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		if err := client.DeleteDevice(ctx, id); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0), nil
	})

	// list_device_capabilities
	s.AddTool(&mcp.Tool{
		Name:        "list_device_capabilities",
		Description: "List capabilities supported by a SmartThings device",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to list capabilities for",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id required"), nil
		}
		dev, err := client.GetDevice(ctx, id)
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
		return marshalResult(caps, []mcp.Role{mcp.Role("assistant")}, 0.5)
	})

	// send_device_command
	s.AddTool(&mcp.Tool{
		Name:        "send_device_command",
		Description: "Send command to SmartThings device. This physically actuates the device (e.g., unlock door, turn off heater).",
		Annotations: sideEffect,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to command",
				},
				"component": map[string]any{
					"type":        "string",
					"description": "The component ID (default: main). Use 'main' for most single-component devices.",
				},
				"capability": map[string]any{
					"type":        "string",
					"description": "The capability ID (e.g., switch, audioVolume). Must be supported by the device.",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "The command to execute (e.g., on, off, setVolume).",
				},
				"arguments": map[string]any{
					"type":        "array",
					"description": "List of arguments for the command (e.g. [50] for setVolume). Optional.",
				},
			},
			"required": []string{"device_id", "capability", "command"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		component, _ := args["component"].(string)
		if component == "" {
			component = "main"
		}
		capability, _ := args["capability"].(string)
		if capability == "" {
			return errorResult("capability is required"), nil
		}
		command, _ := args["command"].(string)
		if command == "" {
			return errorResult("command is required"), nil
		}
		arguments, _ := args["arguments"].([]any)

		body := map[string]any{
			"commands": []any{
				map[string]any{
					"component":  component,
					"capability": capability,
					"command":    command,
					"arguments":  arguments,
				},
			},
		}
		if err := client.SendDeviceCommand(ctx, id, body); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0), nil
	})

	// list_locations
	s.AddTool(&mcp.Tool{
		Name:        "list_locations",
		Description: "List SmartThings locations",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		locs, err := client.ListLocations(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(locs, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// get_location
	s.AddTool(&mcp.Tool{
		Name:        "get_location",
		Description: "Get details for a SmartThings location",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
			},
			"required": []string{"location_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		if locID == "" {
			return errorResult("location_id is required"), nil
		}
		loc, err := client.GetLocation(ctx, locID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(loc, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// list_rooms
	s.AddTool(&mcp.Tool{
		Name:        "list_rooms",
		Description: "List rooms in a SmartThings location",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location to list rooms for",
				},
			},
			"required": []string{"location_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		if locID == "" {
			return errorResult("location_id is required"), nil
		}
		rooms, err := client.ListRooms(ctx, locID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(rooms, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// get_room
	s.AddTool(&mcp.Tool{
		Name:        "get_room",
		Description: "Get details of a room in a SmartThings location",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
				"room_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the room",
				},
			},
			"required": []string{"location_id", "room_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		roomID, _ := args["room_id"].(string)
		if locID == "" || roomID == "" {
			return errorResult("location_id and room_id are required"), nil
		}
		room, err := client.GetRoom(ctx, locID, roomID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(room, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// update_room
	s.AddTool(&mcp.Tool{
		Name:        "update_room",
		Description: "Update a room in a SmartThings location (e.g., rename)",
		Annotations: idempotentWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
				"room_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the room to update",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "The new name for the room",
				},
			},
			"required": []string{"location_id", "room_id", "name"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		roomID, _ := args["room_id"].(string)
		name, _ := args["name"].(string)
		if locID == "" || roomID == "" || name == "" {
			return errorResult("location_id, room_id, and name are required"), nil
		}
		room, err := client.UpdateRoom(ctx, locID, roomID, map[string]any{"name": name})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(room, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// create_room
	s.AddTool(&mcp.Tool{
		Name:        "create_room",
		Description: "Create a new room in a SmartThings location",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location to create room in",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the new room (e.g., 'Living Room', 'Office')",
				},
			},
			"required": []string{"location_id", "name"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		name, _ := args["name"].(string)
		if locID == "" || name == "" {
			return errorResult("location_id and name are required"), nil
		}
		room, err := client.CreateRoom(ctx, locID, name)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(room, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// delete_room
	s.AddTool(&mcp.Tool{
		Name:        "delete_room",
		Description: "Delete a room from a SmartThings location",
		Annotations: destructive,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
				"room_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the room to delete",
				},
			},
			"required": []string{"location_id", "room_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		roomID, _ := args["room_id"].(string)
		if locID == "" || roomID == "" {
			return errorResult("location_id and room_id are required"), nil
		}
		if err := client.DeleteRoom(ctx, locID, roomID); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8), nil
	})

	// list_scenes
	s.AddTool(&mcp.Tool{
		Name:        "list_scenes",
		Description: "List SmartThings scenes",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		scenes, err := client.ListScenes(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(scenes, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.6)
	})

	// execute_scene
	s.AddTool(&mcp.Tool{
		Name:        "execute_scene",
		Description: "Execute SmartThings scene. This triggers physical device actions across multiple devices.",
		Annotations: sideEffect,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"scene_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the scene to execute",
				},
			},
			"required": []string{"scene_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		sceneID, _ := args["scene_id"].(string)
		if sceneID == "" {
			return errorResult("scene_id is required"), nil
		}
		resp, err := client.ExecuteScene(ctx, sceneID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(resp, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0)
	})

	// list_rules
	s.AddTool(&mcp.Tool{
		Name:        "list_rules",
		Description: "List SmartThings automation rules",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rules, err := client.ListRules(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(rules, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// get_rule
	s.AddTool(&mcp.Tool{
		Name:        "get_rule",
		Description: "Get details of a SmartThings automation rule",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rule_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the rule",
				},
			},
			"required": []string{"rule_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		ruleID, _ := args["rule_id"].(string)
		if ruleID == "" {
			return errorResult("rule_id is required"), nil
		}
		rule, err := client.GetRule(ctx, ruleID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(rule, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// create_rule
	s.AddTool(&mcp.Tool{
		Name:        "create_rule",
		Description: "Create a SmartThings automation rule",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the rule",
				},
				"actions": map[string]any{
					"type":        "array",
					"description": "The rule actions definition (SmartThings rule actions array)",
				},
			},
			"required": []string{"name", "actions"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		name, _ := args["name"].(string)
		actions := args["actions"]
		if name == "" || actions == nil {
			return errorResult("name and actions are required"), nil
		}
		body := map[string]any{
			"name":    name,
			"actions": actions,
		}
		rule, err := client.CreateRule(ctx, body)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(rule, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// delete_rule
	s.AddTool(&mcp.Tool{
		Name:        "delete_rule",
		Description: "Delete a SmartThings automation rule. This may break expected home automation behavior.",
		Annotations: destructive,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rule_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the rule to delete",
				},
			},
			"required": []string{"rule_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		ruleID, _ := args["rule_id"].(string)
		if ruleID == "" {
			return errorResult("rule_id is required"), nil
		}
		if err := client.DeleteRule(ctx, ruleID); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8), nil
	})

	// list_hubs
	s.AddTool(&mcp.Tool{
		Name:        "list_hubs",
		Description: "List SmartThings hubs",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		hubs, err := client.ListHubs(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(hubs, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// get_hub_health
	s.AddTool(&mcp.Tool{
		Name:        "get_hub_health",
		Description: "Get health status of a SmartThings hub",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"hub_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the hub",
				},
			},
			"required": []string{"hub_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		hubID, _ := args["hub_id"].(string)
		if hubID == "" {
			return errorResult("hub_id is required"), nil
		}
		health, err := client.GetHubHealth(ctx, hubID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(health, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// list_subscriptions
	s.AddTool(&mcp.Tool{
		Name:        "list_subscriptions",
		Description: "List subscriptions for an installed app",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
			},
			"required": []string{"installed_app_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		if appID == "" {
			return errorResult("installed_app_id is required"), nil
		}
		subs, err := client.ListSubscriptions(ctx, appID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(subs, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// create_subscription
	s.AddTool(&mcp.Tool{
		Name:        "create_subscription",
		Description: "Create a subscription for device events",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to subscribe to",
				},
				"capability": map[string]any{
					"type":        "string",
					"description": "The capability to monitor (e.g., switch, audioVolume)",
				},
				"attribute": map[string]any{
					"type":        "string",
					"description": "The attribute to monitor (e.g., switch, volume)",
				},
			},
			"required": []string{"installed_app_id", "device_id", "capability", "attribute"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		deviceID, _ := args["device_id"].(string)
		capability, _ := args["capability"].(string)
		attribute, _ := args["attribute"].(string)
		if appID == "" || deviceID == "" || capability == "" || attribute == "" {
			return errorResult("installed_app_id, device_id, capability, and attribute are required"), nil
		}

		subReq := smartthings.CreateSubscriptionRequest{
			SourceType: "DEVICE",
			Device: &struct {
				DeviceID        string `json:"deviceId"`
				ComponentID     string `json:"componentId,omitempty"`
				Capability      string `json:"capability"`
				Attribute       string `json:"attribute"`
				StateChangeOnly bool   `json:"stateChangeOnly,omitempty"`
			}{
				DeviceID:        deviceID,
				Capability:      capability,
				Attribute:       attribute,
				StateChangeOnly: true,
			},
		}

		sub, err := client.CreateSubscription(ctx, appID, subReq)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(sub, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// delete_subscription
	s.AddTool(&mcp.Tool{
		Name:        "delete_subscription",
		Description: "Delete a subscription. This may break event-driven app functionality.",
		Annotations: destructive,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
				"subscription_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the subscription to delete",
				},
			},
			"required": []string{"installed_app_id", "subscription_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		subID, _ := args["subscription_id"].(string)
		if appID == "" || subID == "" {
			return errorResult("installed_app_id and subscription_id are required"), nil
		}
		if err := client.DeleteSubscription(ctx, appID, subID); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8), nil
	})

	// list_schedules
	s.AddTool(&mcp.Tool{
		Name:        "list_schedules",
		Description: "List schedules for an installed app",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
			},
			"required": []string{"installed_app_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		if appID == "" {
			return errorResult("installed_app_id is required"), nil
		}
		schedules, err := client.ListSchedules(ctx, appID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(schedules, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// create_schedule
	s.AddTool(&mcp.Tool{
		Name:        "create_schedule",
		Description: "Create a cron schedule",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "The name of the schedule (e.g., 'Morning Routine')",
				},
				"cron_expression": map[string]any{
					"type":        "string",
					"description": "Cron expression for the schedule (e.g., '0 0 7 * * ?' for daily at 7 AM)",
				},
				"timezone": map[string]any{
					"type":        "string",
					"description": "Timezone for the schedule (e.g., 'Asia/Seoul', 'UTC'). Defaults to UTC.",
				},
			},
			"required": []string{"installed_app_id", "name", "cron_expression"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		name, _ := args["name"].(string)
		cronExpr, _ := args["cron_expression"].(string)
		if appID == "" || name == "" || cronExpr == "" {
			return errorResult("installed_app_id, name, and cron_expression are required"), nil
		}
		timezone, _ := args["timezone"].(string)
		if timezone == "" {
			timezone = "UTC"
		}

		schReq := smartthings.CreateScheduleRequest{
			Name: name,
			Cron: &struct {
				Expression string `json:"expression"`
				Timezone   string `json:"timezone"`
			}{
				Expression: cronExpr,
				Timezone:   timezone,
			},
		}

		sch, err := client.CreateSchedule(ctx, appID, schReq)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(sch, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// delete_schedule
	s.AddTool(&mcp.Tool{
		Name:        "delete_schedule",
		Description: "Delete a schedule. This may break timed automations.",
		Annotations: destructive,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
				"schedule_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the schedule to delete",
				},
			},
			"required": []string{"installed_app_id", "schedule_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		schID, _ := args["schedule_id"].(string)
		if appID == "" || schID == "" {
			return errorResult("installed_app_id and schedule_id are required"), nil
		}
		if err := client.DeleteSchedule(ctx, appID, schID); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8), nil
	})

	// get_device_history
	s.AddTool(&mcp.Tool{
		Name:        "get_device_history",
		Description: "Get recent event history for a device",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to retrieve history for",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		deviceID, _ := args["device_id"].(string)
		if deviceID == "" {
			return errorResult("device_id is required"), nil
		}
		history, err := client.GetDeviceHistory(ctx, deviceID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(history, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// get_capability
	s.AddTool(&mcp.Tool{
		Name:        "get_capability",
		Description: "Get definition of a SmartThings capability",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"capability_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the capability (e.g., 'switch', 'audioVolume')",
				},
				"version": map[string]any{
					"type":        "number",
					"description": "The version of the capability (default: 1)",
				},
			},
			"required": []string{"capability_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		capID, _ := args["capability_id"].(string)
		if capID == "" {
			return errorResult("capability_id is required"), nil
		}
		versionFloat, ok := args["version"].(float64)
		version := 1
		if ok {
			version = int(versionFloat)
		}
		capDef, err := client.GetCapability(ctx, capID, version)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(capDef, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// ==================== Modes ====================

	// list_modes
	s.AddTool(&mcp.Tool{
		Name:        "list_modes",
		Description: "List modes for a SmartThings location (e.g., Home, Away, Night)",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
			},
			"required": []string{"location_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		if locID == "" {
			return errorResult("location_id is required"), nil
		}
		modes, err := client.ListModes(ctx, locID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(modes, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// get_current_mode
	s.AddTool(&mcp.Tool{
		Name:        "get_current_mode",
		Description: "Get the current mode for a SmartThings location (e.g., Home, Away, Night)",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
			},
			"required": []string{"location_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		if locID == "" {
			return errorResult("location_id is required"), nil
		}
		mode, err := client.GetCurrentMode(ctx, locID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(mode, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// set_current_mode
	s.AddTool(&mcp.Tool{
		Name:        "set_current_mode",
		Description: "Change the current mode for a SmartThings location (e.g., switch to Away, Night, Home). May trigger cascading automations.",
		Annotations: sideEffect,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
				"mode_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the mode to activate. Use list_modes to discover available modes.",
				},
			},
			"required": []string{"location_id", "mode_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		modeID, _ := args["mode_id"].(string)
		if locID == "" || modeID == "" {
			return errorResult("location_id and mode_id are required"), nil
		}
		mode, err := client.SetCurrentMode(ctx, locID, modeID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(mode, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0)
	})

	// ==================== Additional Device operations ====================

	// update_device
	s.AddTool(&mcp.Tool{
		Name:        "update_device",
		Description: "Update a SmartThings device (e.g., rename, move to a different room)",
		Annotations: idempotentWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device to update",
				},
				"label": map[string]any{
					"type":        "string",
					"description": "New display label for the device",
				},
				"room_id": map[string]any{
					"type":        "string",
					"description": "ID of the room to move the device to",
				},
			},
			"required": []string{"device_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		id, _ := args["device_id"].(string)
		if id == "" {
			return errorResult("device_id is required"), nil
		}
		body := make(map[string]any)
		if label, ok := args["label"].(string); ok && label != "" {
			body["label"] = label
		}
		if roomID, ok := args["room_id"].(string); ok && roomID != "" {
			body["roomId"] = roomID
		}
		if len(body) == 0 {
			return errorResult("at least one field to update (label, room_id) is required"), nil
		}
		d, err := client.UpdateDevice(ctx, id, body)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(d, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// get_component_status
	s.AddTool(&mcp.Tool{
		Name:        "get_component_status",
		Description: "Get status for a specific component of a SmartThings device",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device",
				},
				"component_id": map[string]any{
					"type":        "string",
					"description": "The component ID (e.g., 'main')",
				},
			},
			"required": []string{"device_id", "component_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		deviceID, _ := args["device_id"].(string)
		componentID, _ := args["component_id"].(string)
		if deviceID == "" || componentID == "" {
			return errorResult("device_id and component_id are required"), nil
		}
		status, err := client.GetComponentStatus(ctx, deviceID, componentID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(status, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// get_capability_status
	s.AddTool(&mcp.Tool{
		Name:        "get_capability_status",
		Description: "Get status for a specific capability on a device component (e.g., switch status on main component)",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"device_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the device",
				},
				"component_id": map[string]any{
					"type":        "string",
					"description": "The component ID (e.g., 'main')",
				},
				"capability_id": map[string]any{
					"type":        "string",
					"description": "The capability ID (e.g., 'switch', 'temperatureMeasurement')",
				},
			},
			"required": []string{"device_id", "component_id", "capability_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		deviceID, _ := args["device_id"].(string)
		componentID, _ := args["component_id"].(string)
		capabilityID, _ := args["capability_id"].(string)
		if deviceID == "" || componentID == "" || capabilityID == "" {
			return errorResult("device_id, component_id, and capability_id are required"), nil
		}
		status, err := client.GetCapabilityStatus(ctx, deviceID, componentID, capabilityID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(status, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// create_device
	s.AddTool(&mcp.Tool{
		Name:        "create_device",
		Description: "Create a new SmartThings device (typically virtual/cloud devices)",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"label": map[string]any{
					"type":        "string",
					"description": "Display label for the device",
				},
				"location_id": map[string]any{
					"type":        "string",
					"description": "ID of the location for the device",
				},
				"room_id": map[string]any{
					"type":        "string",
					"description": "ID of the room to place the device in (optional)",
				},
				"profile_id": map[string]any{
					"type":        "string",
					"description": "Device profile ID defining capabilities",
				},
			},
			"required": []string{"label", "location_id", "profile_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		label, _ := args["label"].(string)
		locationID, _ := args["location_id"].(string)
		profileID, _ := args["profile_id"].(string)
		if label == "" || locationID == "" || profileID == "" {
			return errorResult("label, location_id, and profile_id are required"), nil
		}
		body := map[string]any{
			"label":      label,
			"locationId": locationID,
			"profileId":  profileID,
		}
		if roomID, ok := args["room_id"].(string); ok && roomID != "" {
			body["roomId"] = roomID
		}
		d, err := client.CreateDevice(ctx, body)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(d, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.9)
	})

	// ==================== Additional Location operations ====================

	// create_location
	s.AddTool(&mcp.Tool{
		Name:        "create_location",
		Description: "Create a new SmartThings location",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the location (e.g., 'Home', 'Office')",
				},
				"country_code": map[string]any{
					"type":        "string",
					"description": "ISO country code (e.g., 'US', 'KR')",
				},
				"latitude": map[string]any{
					"type":        "number",
					"description": "Latitude coordinate (optional)",
				},
				"longitude": map[string]any{
					"type":        "number",
					"description": "Longitude coordinate (optional)",
				},
			},
			"required": []string{"name", "country_code"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		name, _ := args["name"].(string)
		countryCode, _ := args["country_code"].(string)
		if name == "" || countryCode == "" {
			return errorResult("name and country_code are required"), nil
		}
		body := map[string]any{
			"name":        name,
			"countryCode": countryCode,
		}
		if lat, ok := args["latitude"].(float64); ok {
			body["latitude"] = lat
		}
		if lon, ok := args["longitude"].(float64); ok {
			body["longitude"] = lon
		}
		loc, err := client.CreateLocation(ctx, body)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(loc, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// update_location
	s.AddTool(&mcp.Tool{
		Name:        "update_location",
		Description: "Update a SmartThings location (e.g., rename)",
		Annotations: idempotentWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "New name for the location",
				},
			},
			"required": []string{"location_id", "name"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		name, _ := args["name"].(string)
		if locID == "" || name == "" {
			return errorResult("location_id and name are required"), nil
		}
		loc, err := client.UpdateLocation(ctx, locID, map[string]any{"name": name})
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(loc, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// delete_location
	s.AddTool(&mcp.Tool{
		Name:        "delete_location",
		Description: "Delete a SmartThings location. DANGEROUS: This cascading-deletes ALL devices, rooms, scenes, and rules in the location. Requires 'smartthings:location:delete' scope.",
		Annotations: destructive,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location to delete",
				},
			},
			"required": []string{"location_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		if locID == "" {
			return errorResult("location_id is required"), nil
		}
		// Gate behind scope: delete_location is catastrophic (cascading delete of
		// all devices, rooms, scenes, and rules). Require explicit scope when auth
		// is enabled and TokenInfo is available in context.
		if ti := sdkauth.TokenInfoFromContext(ctx); ti != nil {
			if !slices.Contains(ti.Scopes, "smartthings:location:delete") {
				return errorResult("delete_location requires the 'smartthings:location:delete' scope"), nil
			}
		}
		if err := client.DeleteLocation(ctx, locID); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("ok", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0), nil
	})

	// ==================== Additional Rule operations ====================

	// update_rule
	s.AddTool(&mcp.Tool{
		Name:        "update_rule",
		Description: "Update an existing SmartThings automation rule",
		Annotations: idempotentWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rule_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the rule to update",
				},
				"name": map[string]any{
					"type":        "string",
					"description": "The updated name of the rule",
				},
				"actions": map[string]any{
					"type":        "array",
					"description": "The updated rule actions definition (SmartThings rule actions array)",
				},
			},
			"required": []string{"rule_id", "name", "actions"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		ruleID, _ := args["rule_id"].(string)
		name, _ := args["name"].(string)
		actions := args["actions"]
		if ruleID == "" || name == "" || actions == nil {
			return errorResult("rule_id, name, and actions are required"), nil
		}
		body := map[string]any{
			"name":    name,
			"actions": actions,
		}
		rule, err := client.UpdateRule(ctx, ruleID, body)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(rule, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// execute_rule
	s.AddTool(&mcp.Tool{
		Name:        "execute_rule",
		Description: "Manually trigger a SmartThings automation rule. This triggers physical device actions.",
		Annotations: sideEffect,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"rule_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the rule to execute",
				},
			},
			"required": []string{"rule_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		ruleID, _ := args["rule_id"].(string)
		if ruleID == "" {
			return errorResult("rule_id is required"), nil
		}
		resp, err := client.ExecuteRule(ctx, ruleID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(resp, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0)
	})

	// ==================== Room devices ====================

	// list_room_devices
	s.AddTool(&mcp.Tool{
		Name:        "list_room_devices",
		Description: "List all devices in a specific room",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the location",
				},
				"room_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the room",
				},
			},
			"required": []string{"location_id", "room_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		roomID, _ := args["room_id"].(string)
		if locID == "" || roomID == "" {
			return errorResult("location_id and room_id are required"), nil
		}
		devices, err := client.ListRoomDevices(ctx, locID, roomID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(devices, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.8)
	})

	// ==================== Additional Capability operations ====================

	// list_capabilities
	s.AddTool(&mcp.Tool{
		Name:        "list_capabilities",
		Description: "List all standard SmartThings capabilities",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		caps, err := client.ListStandardCapabilities(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(caps, []mcp.Role{mcp.Role("assistant")}, 0.5)
	})

	// list_capability_namespaces
	s.AddTool(&mcp.Tool{
		Name:        "list_capability_namespaces",
		Description: "List all capability namespaces",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		ns, err := client.ListCapabilityNamespaces(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(ns, []mcp.Role{mcp.Role("assistant")}, 0.4)
	})

	// ==================== Installed Apps ====================

	// list_installed_apps
	s.AddTool(&mcp.Tool{
		Name:        "list_installed_apps",
		Description: "List installed SmartThings apps",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		apps, err := client.ListInstalledApps(ctx)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(apps, []mcp.Role{mcp.Role("assistant")}, 0.6)
	})

	// get_installed_app
	s.AddTool(&mcp.Tool{
		Name:        "get_installed_app",
		Description: "Get details of an installed SmartThings app",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
			},
			"required": []string{"installed_app_id"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		if appID == "" {
			return errorResult("installed_app_id is required"), nil
		}
		app, err := client.GetInstalledApp(ctx, appID)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(app, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})

	// ==================== Notifications ====================

	// send_notification
	s.AddTool(&mcp.Tool{
		Name:        "send_notification",
		Description: "Send a push notification to the SmartThings mobile app",
		Annotations: safeWrite,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location_id": map[string]any{
					"type":        "string",
					"description": "The location ID to send the notification to",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "The notification message text",
				},
			},
			"required": []string{"location_id", "message"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		locID, _ := args["location_id"].(string)
		message, _ := args["message"].(string)
		if locID == "" || message == "" {
			return errorResult("location_id and message are required"), nil
		}
		notifReq := smartthings.NotificationRequest{
			LocationID:     locID,
			Type:           "ALERT",
			DefaultMessage: message,
		}
		if err := client.SendNotification(ctx, notifReq); err != nil {
			return errorResult(err.Error()), nil
		}
		return toolResult("notification sent", []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 1.0), nil
	})

	// ==================== Additional Schedule operations ====================

	// get_schedule
	s.AddTool(&mcp.Tool{
		Name:        "get_schedule",
		Description: "Get details of a specific schedule",
		Annotations: readOnly,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"installed_app_id": map[string]any{
					"type":        "string",
					"description": "The unique identifier of the installed app",
				},
				"schedule_name": map[string]any{
					"type":        "string",
					"description": "The name of the schedule",
				},
			},
			"required": []string{"installed_app_id", "schedule_name"},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return errorResult("invalid arguments"), nil
		}
		appID, _ := args["installed_app_id"].(string)
		schName, _ := args["schedule_name"].(string)
		if appID == "" || schName == "" {
			return errorResult("installed_app_id and schedule_name are required"), nil
		}
		sch, err := client.GetSchedule(ctx, appID, schName)
		if err != nil {
			return errorResult(err.Error()), nil
		}
		return marshalResult(sch, []mcp.Role{mcp.Role("user"), mcp.Role("assistant")}, 0.7)
	})
}

// toolResult creates a CallToolResult with text content and annotations.
func toolResult(text string, audience []mcp.Role, priority float64) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: text,
				Annotations: &mcp.Annotations{
					Audience: audience,
					Priority: priority,
				},
			},
		},
	}
}

// marshalResult marshals v to JSON and returns a CallToolResult with annotations.
func marshalResult(v any, audience []mcp.Role, priority float64) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("failed to marshal response: %v", err)), nil
	}
	return toolResult(string(data), audience, priority), nil
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
