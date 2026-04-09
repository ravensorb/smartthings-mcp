# Lango SmartThings MCP Server

[![smithery badge](https://smithery.ai/badge/@langowarny/smartthings-mcp)](https://smithery.ai/server/@langowarny/smartthings-mcp)

A **Model Context Protocol (MCP)** server that exposes Samsung **SmartThings Public API** as
LLM-friendly tools, resources and real-time events.

## Features

* **Lazy Loading**: Tools are discoverable without authentication - only validates API keys when tools are invoked
* Wraps common SmartThings operations as **MCP Tools**
  * **Devices**: `list_devices`, `get_device`, `get_device_status`, `get_device_preferences`, `update_device_preferences`, `get_device_health`, `delete_device`, `list_device_capabilities`, `send_device_command`
  * **Locations & Rooms**: `list_locations`, `get_location`, `list_rooms`, `get_room`, `update_room`, `create_room`, `delete_room`
  * **Scenes**: `list_scenes`, `execute_scene`
  * **Rules & Automations**: `list_rules`, `get_rule`, `create_rule`, `delete_rule`
  * **Hubs**: `list_hubs`, `get_hub_health`
  * **Subscriptions**: `list_subscriptions`, `create_subscription`, `delete_subscription`
  * **Schedules**: `list_schedules`, `create_schedule`, `delete_schedule`
  * **History & Capabilities**: `get_device_history`, `get_capability`
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
[Releases](https://github.com/langowarny/smartthings-mcp/releases) page, then add to your
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
      "url": "https://your-server.example.com/smartthings-mcp",
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

### Devices

| Tool | Params | Description |
|------|--------|-------------|
| `list_devices` | `location_id?` | List devices (optionally filtered by location) |
| `get_device` | `device_id` | Device metadata |
| `get_device_status` | `device_id` | Live device status |
| `get_device_preferences` | `device_id` | Read device preferences (parameter101, etc.) |
| `update_device_preferences` | `device_id`, `preferences` | Write device preferences (e.g., motion sensitivity) |
| `get_device_health` | `device_id` | Check if device is online/offline |
| `delete_device` | `device_id` | Remove a device |
| `list_device_capabilities` | `device_id` | Supported capabilities |
| `send_device_command` | `device_id`, `capability`, `command`, `component?`, `arguments?` | Send a command to a device |
| `get_device_history` | `device_id` | Recent event history |

### Locations & Rooms

| Tool | Params | Description |
|------|--------|-------------|
| `list_locations` | - | List all locations |
| `get_location` | `location_id` | Location details |
| `list_rooms` | `location_id` | List rooms in a location |
| `get_room` | `location_id`, `room_id` | Room details |
| `update_room` | `location_id`, `room_id`, `name` | Rename/update a room |
| `create_room` | `location_id`, `name` | Create a room |
| `delete_room` | `location_id`, `room_id` | Delete a room |

### Scenes

| Tool | Params | Description |
|------|--------|-------------|
| `list_scenes` | - | List all scenes |
| `execute_scene` | `scene_id` | Run a scene |

### Rules & Automations

| Tool | Params | Description |
|------|--------|-------------|
| `list_rules` | - | List automation rules |
| `get_rule` | `rule_id` | Rule details |
| `create_rule` | `name`, `actions` | Create an automation rule |
| `delete_rule` | `rule_id` | Delete a rule |

### Hubs

| Tool | Params | Description |
|------|--------|-------------|
| `list_hubs` | - | List hubs |
| `get_hub_health` | `hub_id` | Hub health status |

### Subscriptions & Schedules

| Tool | Params | Description |
|------|--------|-------------|
| `list_subscriptions` | `installed_app_id` | List event subscriptions |
| `create_subscription` | `installed_app_id`, `device_id`, `capability`, `attribute` | Subscribe to device events |
| `delete_subscription` | `installed_app_id`, `subscription_id` | Remove a subscription |
| `list_schedules` | `installed_app_id` | List schedules |
| `create_schedule` | `installed_app_id`, `name`, `cron_expression`, `timezone?` | Create a cron schedule |
| `delete_schedule` | `installed_app_id`, `schedule_id` | Delete a schedule |

### Capabilities

| Tool | Params | Description |
|------|--------|-------------|
| `get_capability` | `capability_id`, `version?` | Capability definition (attributes, commands) |

## Resource Patterns

| URI Template | Description | MIME |
|--------------|-------------|------|
| `st://devices/{device_id}` | Device metadata | `application/json` |
| `st://devices/{device_id}/status` | Live status | `application/json` |
| `st://locations/{location_id}` | Location metadata | `application/json` |

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
