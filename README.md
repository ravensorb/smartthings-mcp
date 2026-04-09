# Lango SmartThings MCP Server

[![CI/CD](https://github.com/ravensorb/smartthings-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/ravensorb/smartthings-mcp/actions/workflows/ci.yml)
[![GitHub release](https://img.shields.io/github/v/release/ravensorb/smartthings-mcp)](https://github.com/ravensorb/smartthings-mcp/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/langowarny/smartthings-mcp)](https://goreportcard.com/report/github.com/langowarny/smartthings-mcp)
[![Docker Image](https://img.shields.io/badge/ghcr.io-smartthings--mcp-blue?logo=docker)](https://ghcr.io/ravensorb/smartthings-mcp)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![smithery badge](https://smithery.ai/badge/@langowarny/smartthings-mcp)](https://smithery.ai/server/@langowarny/smartthings-mcp)

A **Model Context Protocol (MCP)** server that exposes Samsung **SmartThings Public API** as
LLM-friendly tools, resources and real-time events.

---

## Features

* **Lazy Loading**: Tools are discoverable without authentication - only validates API keys when tools are invoked
* **53 MCP Tools** covering the full SmartThings API surface
  * **Devices** (16): list, get, status, preferences, health, history, capabilities (IDs and full schemas), commands, create, update, delete, component/capability status
  * **Locations** (5): list, get, create, update, delete
  * **Rooms** (6): list, get, create, update, delete, list devices in room
  * **Modes** (3): list, get current, set current (Home/Away/Night)
  * **Scenes** (2): list, execute
  * **Rules** (5): list, get, create, update, delete, execute
  * **Hubs** (3): list, health, list installed Edge drivers
  * **Capabilities** (3): get definition, list standard, list namespaces
  * **Installed Apps** (2): list, get
  * **Subscriptions** (3): list, create, delete
  * **Schedules** (4): list, get, create, delete
  * **Notifications** (1): send push notification
* **MCP Tool Annotations** for safety — all tools annotated with `readOnlyHint`, `destructiveHint`, and `idempotentHint` per the MCP spec
  * Destructive operations (delete device/location/rule) clearly marked
  * Physical side-effect operations (send command, execute scene, change mode) marked
  * `delete_location` gated behind `smartthings:location:delete` scope when auth is enabled
* **Capability Discovery** — resolve full capability schemas (attributes, commands, parameters) for any device, including custom Edge driver capabilities
* **Edge Driver Discovery** — enumerate installed Edge drivers on any hub
* **Preference Fallback** — gracefully handles the broken per-device preferences endpoint by falling back to device profile definitions
* Exposes device / status / location data as **MCP Resources** with read-through cache
* Supports all official **MCP-Go transports**
  * **Stdio** (CLI / local), **StreamableHTTP**, **Server-Sent Events (SSE)**
* Zero external dependencies apart from `mcp-go` and `zap` logger

## Requirements

* A valid **SmartThings PAT** (Personal Access Token)

### Getting a Personal Access Token (PAT)

1. Go to [SmartThings Personal Access Tokens](https://account.smartthings.com/tokens).
2. Log in with your Samsung Account.
3. Click **Generate new token**.
4. Enter a name for your token and select the authorized scopes (e.g., `devices`, `locations`, `scenes`, `rules`, `schedules`).
5. Click **Generate token**.
6. **Copy and save** the token immediately (it won't be shown again).

## Quick Start with Claude Desktop

Pick whichever method suits your setup. All three produce the same result: SmartThings tools available inside Claude Desktop.

### Option 1 - Pre-built binary (recommended)

Download the latest release for your platform from the
[Releases](https://github.com/ravensorb/smartthings-mcp/releases) page, then add to your
Claude Desktop config:

<details>
<summary>macOS config path</summary>

`~/Library/Application Support/Claude/claude_desktop_config.json`
</details>

<details>
<summary>Windows config path</summary>

`%APPDATA%\Claude\claude_desktop_config.json`
</details>

```json
{
  "mcpServers": {
    "smartthings": {
      "command": "/path/to/smartthings-mcp",
      "args": ["-transport", "stdio"],
      "env": {
        "SMARTTHINGS_TOKEN": "your-token-here"
      }
    }
  }
}
```

Replace `/path/to/smartthings-mcp` with the actual path to the downloaded binary.

### Option 2 - Docker

No toolchain required - just Docker.

```json
{
  "mcpServers": {
    "smartthings": {
      "command": "docker",
      "args": [
        "run", "--rm", "-i",
        "-e", "SMARTTHINGS_TOKEN=your-token-here",
        "ghcr.io/ravensorb/smartthings-mcp:latest",
        "-transport", "stdio"
      ]
    }
  }
}
```

### Option 3 - Remote server with auth

Connect Claude Desktop to a remote SmartThings MCP server protected with JWT auth:

```json
{
  "mcpServers": {
    "smartthings": {
      "type": "http",
      "url": "https://your-server.example.com/smartthings-mcp",
      "headers": {
        "Authorization": "Bearer ${MCP_AUTH_TOKEN}"
      }
    }
  }
}
```

Set the `MCP_AUTH_TOKEN` environment variable to a valid JWT from your OIDC provider, or replace `${MCP_AUTH_TOKEN}` with the token directly.

### Option 4 - Remote server (auto detect if OAuth is enabled)

```json
{
  "mcpServers": {
    "smartthings": {
      "type": "http",
      "url": "https://your-server.example.com/smartthings-mcp"
    }
  }
}
```

### Option 5 - Smithery

Install automatically via [Smithery](https://smithery.ai/server/@langowarny/smartthings-mcp):

```bash
npx -y @smithery/cli install @langowarny/smartthings-mcp --client claude
```

### After configuring

Restart Claude Desktop. You should see the MCP hammer icon in the input area confirming the server is connected.

## Environment Variables

### SmartThings API

| Name | Default | Description |
|------|---------|-------------|
| `SMARTTHINGS_TOKEN` | - | Bearer token for SmartThings API. **Required for operations**, but server starts without it for tool discovery |
| `ST_BASE_URL` | `https://api.smartthings.com` | Override for testing / mock servers |

### MCP Server Authentication (Optional)

Protect HTTP transports (SSE, StreamableHTTP) with JWT bearer token validation. The server acts as an OAuth 2.0 Resource Server, validating tokens issued by an external Identity Provider (Authentik, Keycloak, Auth0, etc.). Stdio transport is unaffected (inherently trusted).

**OIDC Discovery (recommended)** - just provide the issuer URL:

| Name | Default | Description |
|------|---------|-------------|
| `MCP_AUTH_ENABLED` | `false` | Set to `true` to enable JWT auth |
| `MCP_AUTH_OIDC_ISSUER_URL` | - | OIDC issuer URL (discovery auto-resolves `/.well-known/openid-configuration`) |
| `MCP_AUTH_AUDIENCE` | - | Expected `aud` claim (typically the OAuth client ID in your IdP) |
| `MCP_AUTH_CLIENT_SECRET` | - | Client secret for confidential clients (used by DCR proxy) |
| `MCP_AUTH_SCOPES` | - | Comma-separated required scopes (optional) |
| `MCP_AUTH_RESOURCE_ID` | - | Enables RFC 9728 metadata at `/.well-known/oauth-protected-resource` (optional) |

**Manual fallback** - for providers without OIDC discovery:

| Name | Default | Description |
|------|---------|-------------|
| `MCP_AUTH_JWKS_URL` | - | JWKS endpoint URL |
| `MCP_AUTH_ISSUER` | - | Expected `iss` claim |

#### Example: Authentik

```bash
MCP_AUTH_ENABLED=true
MCP_AUTH_OIDC_ISSUER_URL="https://authentik.example.com/application/o/smartthings-mcp"
MCP_AUTH_AUDIENCE="smartthings-mcp"
```

#### Example: Keycloak

```bash
MCP_AUTH_ENABLED=true
MCP_AUTH_OIDC_ISSUER_URL="https://keycloak.example.com/realms/myrealm"
MCP_AUTH_AUDIENCE="smartthings-mcp"
```

When auth is enabled, MCP clients must include an `Authorization: Bearer <token>` header. Requests without a valid token receive a `401 Unauthorized` response.

## Running standalone

### Stdio

```bash
SMARTTHINGS_TOKEN=your-token go run ./cmd/server -transport stdio
```

### StreamableHTTP

```bash
SMARTTHINGS_TOKEN=your-token go run ./cmd/server -transport stream -port 8081
```

### Docker

```bash
docker run --rm -p 8081:8081 \
  -e SMARTTHINGS_TOKEN=your-token \
  ghcr.io/ravensorb/smartthings-mcp:latest
```

### Version check

```bash
./smartthings-mcp -version
```

## Tool Catalogue

All tools include MCP [tool annotations](https://modelcontextprotocol.io/specification/2025-06-18/basic/utilities/annotations) indicating whether they are read-only, destructive, or trigger physical side effects.

### Devices

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_devices` | `location_id?` | read-only | List devices (optionally filtered by location) |
| `get_device` | `device_id` | read-only | Device metadata |
| `get_device_status` | `device_id` | read-only | Live device status |
| `get_device_preferences` | `device_id` | read-only | Read device preferences (falls back to profile definitions if API unavailable) |
| `update_device_preferences` | `device_id`, `preferences` | idempotent | Write device preferences (see [Known Limitations](#known-limitations)) |
| `get_device_health` | `device_id` | read-only | Check if device is online/offline |
| `delete_device` | `device_id` | **destructive** | Remove a device (irreversible) |
| `list_device_capabilities` | `device_id` | read-only | Supported capability IDs |
| `get_device_capabilities` | `device_id` | read-only | Full capability schemas with commands and attributes for all capabilities |
| `send_device_command` | `device_id`, `capability`, `command`, `component?`, `arguments?` | **side-effect** | Send a command (physically actuates device) |
| `get_device_history` | `device_id` | read-only | Recent event history |
| `update_device` | `device_id`, `label?`, `room_id?` | idempotent | Rename or move a device to a different room |
| `get_component_status` | `device_id`, `component_id` | read-only | Status for a specific component |
| `get_capability_status` | `device_id`, `component_id`, `capability_id` | read-only | Status for a specific capability on a component |
| `create_device` | `label`, `location_id`, `profile_id`, `room_id?` | safe-write | Create a virtual/cloud device |

### Locations

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_locations` | - | read-only | List all locations |
| `get_location` | `location_id` | read-only | Location details |
| `create_location` | `name`, `country_code`, `latitude?`, `longitude?` | safe-write | Create a new location |
| `update_location` | `location_id`, `name` | idempotent | Rename a location |
| `delete_location` | `location_id` | **destructive** | Delete a location (cascading delete of all devices, rooms, scenes, rules). Requires `smartthings:location:delete` scope when auth is enabled. |

### Rooms

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_rooms` | `location_id` | read-only | List rooms in a location |
| `get_room` | `location_id`, `room_id` | read-only | Room details |
| `create_room` | `location_id`, `name` | safe-write | Create a room |
| `update_room` | `location_id`, `room_id`, `name` | idempotent | Rename a room |
| `delete_room` | `location_id`, `room_id` | **destructive** | Delete a room |
| `list_room_devices` | `location_id`, `room_id` | read-only | List devices in a room |

### Modes

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_modes` | `location_id` | read-only | List modes (Home, Away, Night, etc.) |
| `get_current_mode` | `location_id` | read-only | Get the active mode |
| `set_current_mode` | `location_id`, `mode_id` | **side-effect** | Change mode (may trigger cascading automations) |

### Scenes

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_scenes` | - | read-only | List all scenes |
| `execute_scene` | `scene_id` | **side-effect** | Run a scene (triggers physical device actions) |

### Rules & Automations

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_rules` | - | read-only | List automation rules |
| `get_rule` | `rule_id` | read-only | Rule details |
| `create_rule` | `name`, `actions` | safe-write | Create an automation rule |
| `update_rule` | `rule_id`, `name`, `actions` | idempotent | Update an existing rule |
| `delete_rule` | `rule_id` | **destructive** | Delete a rule |
| `execute_rule` | `rule_id` | **side-effect** | Manually trigger a rule |

### Hubs & Edge Drivers

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_hubs` | - | read-only | List hubs |
| `get_hub_health` | `hub_id` | read-only | Hub health status |
| `list_hub_drivers` | `hub_id` | read-only | List Edge drivers installed on a hub |

### Capabilities

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `get_capability` | `capability_id`, `version?` | read-only | Capability definition (attributes, commands) |
| `list_capabilities` | - | read-only | List all standard capabilities |
| `list_capability_namespaces` | - | read-only | List capability namespaces |

### Installed Apps

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_installed_apps` | - | read-only | List installed apps |
| `get_installed_app` | `installed_app_id` | read-only | Installed app details |

### Subscriptions

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_subscriptions` | `installed_app_id` | read-only | List event subscriptions |
| `create_subscription` | `installed_app_id`, `device_id`, `capability`, `attribute` | safe-write | Subscribe to device events |
| `delete_subscription` | `installed_app_id`, `subscription_id` | **destructive** | Remove a subscription |

### Schedules

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `list_schedules` | `installed_app_id` | read-only | List schedules |
| `get_schedule` | `installed_app_id`, `schedule_name` | read-only | Get schedule details |
| `create_schedule` | `installed_app_id`, `name`, `cron_expression`, `timezone?` | safe-write | Create a cron schedule |
| `delete_schedule` | `installed_app_id`, `schedule_id` | **destructive** | Delete a schedule |

### Notifications

| Tool | Params | Hint | Description |
|------|--------|------|-------------|
| `send_notification` | `location_id`, `message` | safe-write | Send push notification to SmartThings mobile app |

## Resource Patterns

| URI | Description | MIME |
|-----|-------------|------|
| `st://devices/{device_id}` | Device metadata | `application/json` |
| `st://devices/{device_id}/status` | Live status | `application/json` |
| `st://locations/{location_id}` | Location metadata | `application/json` |
| `st://locations` | All locations | `application/json` |

## Known Limitations

### Per-device preferences endpoint (406 Not Acceptable)

The SmartThings per-device preferences endpoint (`/devices/{deviceId}/preferences`) is **not functional** in the public API. It returns 406 for all device types with all Accept headers. This is confirmed by:
* The endpoint is not in the [official OpenAPI spec](https://swagger.api.smartthings.com/public/st-api.yml)
* The SmartThings Core SDK has a `getPreferences()` method but no corresponding server-side route exists
* Samsung staff have confirmed preferences can only be set via the mobile app

**How this server handles it:**

* `get_device_preferences` gracefully falls back to fetching the device's profile, returning preference **definitions** (available settings, types, and defaults) rather than current values
* `update_device_preferences` returns a clear error explaining the limitation
* The response includes a `_note` field so LLMs understand the context

To programmatically set device configuration parameters (e.g., Zigbee/Z-Wave settings), the recommended path is a custom Edge driver that exposes configuration as a callable capability command via `send_device_command`.

## Development

```bash
go vet ./...
go test ./...
go run ./cmd/server -transport stream
```

### Releasing

```bash
./scripts/release.sh patch          # 0.1.0 -> 0.1.1 (local only)
./scripts/release.sh minor --push   # 0.1.1 -> 0.2.0 (push + trigger CI)
./scripts/release.sh major --push   # 0.2.0 -> 1.0.0
```

### Local CI with act

```bash
cp .act.env.example .act.env
cp .act.vars.example .act.vars
cp .act.secrets.example .act.secrets
# Edit .act.secrets with your GitHub token

act push -P ubuntu-latest=catthehacker/ubuntu:act-latest \
  --env-file .act.env --var-file .act.vars --secret-file .act.secrets
```

## License

MIT
