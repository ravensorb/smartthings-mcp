package smartthings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	BaseURL = "https://api.smartthings.com"
)

// Client is a minimal SmartThings REST API client.
// It is **not** feature-complete—only endpoints needed by the MCP server.

type Client struct {
	token   string
	baseURL string
	http    *http.Client
}

// NewClient returns a new SmartThings API client.
func NewClient(token, baseURL string) *Client {
	if baseURL == "" {
		baseURL = BaseURL
	}
	return &Client{
		token:   token,
		baseURL: baseURL,
		http: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Device represents SmartThings device metadata.
type Device struct {
	DeviceID   string `json:"deviceId"`
	Name       string `json:"name"`
	Label      string `json:"label"`
	DeviceType string `json:"deviceTypeName"`
	Components []struct {
		ID           string `json:"id"`
		Capabilities []struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"capabilities"`
	} `json:"components,omitempty"`
}

// Location represents a SmartThings location.
type Location struct {
	LocationID string `json:"locationId"`
	Name       string `json:"name"`
}

// Scene represents a SmartThings scene.
type Scene struct {
	SceneID          string    `json:"sceneId"`
	SceneName        string    `json:"sceneName"`
	LocationID       string    `json:"locationId"`
	CreatedDate      time.Time `json:"createdDate"`
	LastExecutedDate time.Time `json:"lastExecutedDate"`
}

// GetLocation returns metadata for a single location by ID.
func (c *Client) GetLocation(ctx context.Context, id string) (*Location, error) {
	var loc Location
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s", id), &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// DevicePreferences wraps /preferences response.
type DevicePreferences map[string]any

// DeviceStatus wraps /status response (partial).
type DeviceStatus map[string]any

// ListDevices fetches devices.
func (c *Client) ListDevices(ctx context.Context) ([]Device, error) {
	var resp struct {
		Items []Device `json:"items"`
	}
	if err := c.get(ctx, "/v1/devices", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetDevice returns metadata for a device.
func (c *Client) GetDevice(ctx context.Context, id string) (*Device, error) {
	var d Device
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s", id), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// DeviceHealth represents the health status of a device.
type DeviceHealth struct {
	DeviceID string `json:"deviceId"`
	State    string `json:"state"` // e.g., "ONLINE", "OFFLINE"
}

// GetDevicePreferences returns preferences for a device.
func (c *Client) GetDevicePreferences(ctx context.Context, id string) (DevicePreferences, error) {
	var prefs DevicePreferences
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s/preferences", id), &prefs); err != nil {
		return nil, err
	}
	return prefs, nil
}

// UpdateDevicePreferences writes preferences for a device.
func (c *Client) UpdateDevicePreferences(ctx context.Context, id string, prefs map[string]any) (DevicePreferences, error) {
	var out DevicePreferences
	if err := c.put(ctx, fmt.Sprintf("/v1/devices/%s/preferences", id), prefs, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDeviceHealth returns the health status of a device.
func (c *Client) GetDeviceHealth(ctx context.Context, id string) (*DeviceHealth, error) {
	var health DeviceHealth
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s/health", id), &health); err != nil {
		return nil, err
	}
	return &health, nil
}

// DeleteDevice removes a device.
func (c *Client) DeleteDevice(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/devices/%s", id))
}

// GetDeviceStatus returns live status of a device.
func (c *Client) GetDeviceStatus(ctx context.Context, id string) (DeviceStatus, error) {
	var status DeviceStatus
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s/status", id), &status); err != nil {
		return nil, err
	}
	return status, nil
}

// SendDeviceCommand issues a command.
func (c *Client) SendDeviceCommand(ctx context.Context, id string, body any) error {
	return c.post(ctx, fmt.Sprintf("/v1/devices/%s/commands", id), body, nil)
}

// ListLocations returns locations.
func (c *Client) ListLocations(ctx context.Context) ([]Location, error) {
	var resp struct {
		Items []Location `json:"items"`
	}
	if err := c.get(ctx, "/v1/locations", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

type ExecuteSceneResponse struct {
	SceneID string `json:"sceneId"`
	Status  string `json:"status"`
}

// ListDevicesByLocation fetches devices filtered by location ID.
func (c *Client) ListDevicesByLocation(ctx context.Context, locationID string) ([]Device, error) {
	var resp struct {
		Items []Device `json:"items"`
	}
	path := fmt.Sprintf("/v1/devices?locationId=%s", locationID)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ExecuteScene triggers a scene.
func (c *Client) ExecuteScene(ctx context.Context, sceneID string) (*ExecuteSceneResponse, error) {
	var res ExecuteSceneResponse
	if err := c.post(ctx, fmt.Sprintf("/v1/scenes/%s/execute", sceneID), nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// ListScenes returns scenes.
func (c *Client) ListScenes(ctx context.Context) ([]Scene, error) {
	var resp struct {
		Items []Scene `json:"items"`
	}
	if err := c.get(ctx, "/v1/scenes", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// Room represents a SmartThings room.
type Room struct {
	RoomID          string `json:"roomId"`
	LocationID      string `json:"locationId"`
	Name            string `json:"name"`
	BackgroundImage string `json:"backgroundImage,omitempty"`
}

// ListRooms returns rooms for a location.
func (c *Client) ListRooms(ctx context.Context, locationID string) ([]Room, error) {
	var resp struct {
		Items []Room `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s/rooms", locationID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateRoom creates a room in a location.
func (c *Client) CreateRoom(ctx context.Context, locationID, name string) (*Room, error) {
	req := map[string]string{
		"name": name,
	}
	var room Room
	if err := c.post(ctx, fmt.Sprintf("/v1/locations/%s/rooms", locationID), req, &room); err != nil {
		return nil, err
	}
	return &room, nil
}

// GetRoom returns a single room.
func (c *Client) GetRoom(ctx context.Context, locationID, roomID string) (*Room, error) {
	var room Room
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s/rooms/%s", locationID, roomID), &room); err != nil {
		return nil, err
	}
	return &room, nil
}

// UpdateRoom updates a room (e.g. rename).
func (c *Client) UpdateRoom(ctx context.Context, locationID, roomID string, body map[string]any) (*Room, error) {
	var room Room
	if err := c.put(ctx, fmt.Sprintf("/v1/locations/%s/rooms/%s", locationID, roomID), body, &room); err != nil {
		return nil, err
	}
	return &room, nil
}

// DeleteRoom deletes a room.
func (c *Client) DeleteRoom(ctx context.Context, locationID, roomID string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/locations/%s/rooms/%s", locationID, roomID))
}

// Rule represents a SmartThings rule.
type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Actions any    `json:"actions,omitempty"`
}

// ListRules returns rules.
func (c *Client) ListRules(ctx context.Context) ([]Rule, error) {
	var resp struct {
		Items []Rule `json:"items"`
	}
	if err := c.get(ctx, "/v1/rules", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetRule returns a single rule by ID.
func (c *Client) GetRule(ctx context.Context, ruleID string) (*Rule, error) {
	var rule Rule
	if err := c.get(ctx, fmt.Sprintf("/v1/rules/%s", ruleID), &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// CreateRule creates a new rule.
func (c *Client) CreateRule(ctx context.Context, body any) (*Rule, error) {
	var rule Rule
	if err := c.post(ctx, "/v1/rules", body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// DeleteRule deletes a rule.
func (c *Client) DeleteRule(ctx context.Context, ruleID string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/rules/%s", ruleID))
}

// Hub represents a SmartThings hub.
type Hub struct {
	HubID           string `json:"hubId"`
	Name            string `json:"name"`
	FirmwareVersion string `json:"firmwareVersion"`
}

// HubHealth represents the health status of a hub.
type HubHealth struct {
	State string `json:"state"` // e.g., "ONLINE", "OFFLINE"
}

// ListHubs returns list of hubs.
func (c *Client) ListHubs(ctx context.Context) ([]Hub, error) {
	var resp struct {
		Items []Hub `json:"items"`
	}
	if err := c.get(ctx, "/v1/hubs", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetHubHealth returns health status of a hub.
func (c *Client) GetHubHealth(ctx context.Context, hubID string) (*HubHealth, error) {
	var health HubHealth
	if err := c.get(ctx, fmt.Sprintf("/v1/hubs/%s/health", hubID), &health); err != nil {
		return nil, err
	}
	return &health, nil
}

// Subscription represents a SmartThings subscription.
type Subscription struct {
	ID               string `json:"id"`
	InstalledAppID   string `json:"installedAppId"`
	SourceType       string `json:"sourceType"` // e.g. DEVICE, CAPABILITY, MODE
	Capability       string `json:"capability,omitempty"`
	Attribute        string `json:"attribute,omitempty"`
	Value            string `json:"value,omitempty"`
	StateChangeOnly  bool   `json:"stateChangeOnly,omitempty"`
	SubscriptionName string `json:"subscriptionName,omitempty"`
}

// ListSubscriptions returns subscriptions for an installed app.
func (c *Client) ListSubscriptions(ctx context.Context, installedAppID string) ([]Subscription, error) {
	var resp struct {
		Items []Subscription `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/v1/installedapps/%s/subscriptions", installedAppID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateSubscriptionRequest defines the payload for creating a subscription.
type CreateSubscriptionRequest struct {
	SourceType string `json:"sourceType"`
	Device     *struct {
		DeviceID        string `json:"deviceId"`
		ComponentID     string `json:"componentId,omitempty"`
		Capability      string `json:"capability"`
		Attribute       string `json:"attribute"`
		StateChangeOnly bool   `json:"stateChangeOnly,omitempty"`
	} `json:"device,omitempty"`
	// Simplified for device subscriptions for now
}

// CreateSubscription creates a new subscription.
func (c *Client) CreateSubscription(ctx context.Context, installedAppID string, req CreateSubscriptionRequest) (*Subscription, error) {
	var sub Subscription
	if err := c.post(ctx, fmt.Sprintf("/v1/installedapps/%s/subscriptions", installedAppID), req, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// DeleteSubscription deletes a subscription.
func (c *Client) DeleteSubscription(ctx context.Context, installedAppID, subscriptionID string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/installedapps/%s/subscriptions/%s", installedAppID, subscriptionID))
}

// Schedule represents a SmartThings schedule.
type Schedule struct {
	ScheduleID string `json:"scheduleId"`
	Name       string `json:"name"`
	Cron       *struct {
		Expression string `json:"expression"`
		Timezone   string `json:"timezone"`
	} `json:"cron,omitempty"`
}

// ListSchedules returns schedules for an installed app.
func (c *Client) ListSchedules(ctx context.Context, installedAppID string) ([]Schedule, error) {
	var resp struct {
		Items []Schedule `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/v1/installedapps/%s/schedules", installedAppID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CreateScheduleRequest defines the payload for creating a schedule.
type CreateScheduleRequest struct {
	Name string `json:"name"`
	Cron *struct {
		Expression string `json:"expression"`
		Timezone   string `json:"timezone"`
	} `json:"cron,omitempty"`
	// Simplified for cron schedules
}

// CreateSchedule creates a new schedule.
func (c *Client) CreateSchedule(ctx context.Context, installedAppID string, req CreateScheduleRequest) (*Schedule, error) {
	var sch Schedule
	if err := c.post(ctx, fmt.Sprintf("/v1/installedapps/%s/schedules", installedAppID), req, &sch); err != nil {
		return nil, err
	}
	return &sch, nil
}

// DeleteSchedule deletes a schedule.
func (c *Client) DeleteSchedule(ctx context.Context, installedAppID, scheduleID string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/installedapps/%s/schedules/%s", installedAppID, scheduleID))
}

// DeviceEvent represents a single event in history.
type DeviceEvent struct {
	Date       time.Time `json:"date"`
	DeviceID   string    `json:"deviceId"`
	Component  string    `json:"component"`
	Capability string    `json:"capability"`
	Attribute  string    `json:"attribute"`
	Value      any       `json:"value"`
	Unit       string    `json:"unit,omitempty"`
}

// GetDeviceHistory returns event history for a device.
func (c *Client) GetDeviceHistory(ctx context.Context, deviceID string) ([]DeviceEvent, error) {
	var resp struct {
		Items []DeviceEvent `json:"items"`
	}
	// Limit to recent 20 events for simplicity
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s/history?limit=20", deviceID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CapabilityDefinition represents a SmartThings capability.
type CapabilityDefinition struct {
	ID         string         `json:"id"`
	Version    int            `json:"version"`
	Status     string         `json:"status"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Commands   map[string]any `json:"commands,omitempty"`
}

// GetCapability returns capability definition.
func (c *Client) GetCapability(ctx context.Context, capabilityID string, version int) (*CapabilityDefinition, error) {
	var cap CapabilityDefinition
	if err := c.get(ctx, fmt.Sprintf("/v1/capabilities/%s/%d", capabilityID, version), &cap); err != nil {
		return nil, err
	}
	return &cap, nil
}

// ---------- Modes ----------

// Mode represents a SmartThings location mode (e.g., Home, Away, Night).
type Mode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Name  string `json:"name"`
}

// ListModes returns all modes for a location.
func (c *Client) ListModes(ctx context.Context, locationID string) ([]Mode, error) {
	var resp struct {
		Items []Mode `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s/modes", locationID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetMode returns a single mode.
func (c *Client) GetMode(ctx context.Context, locationID, modeID string) (*Mode, error) {
	var mode Mode
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s/modes/%s", locationID, modeID), &mode); err != nil {
		return nil, err
	}
	return &mode, nil
}

// GetCurrentMode returns the current mode for a location.
func (c *Client) GetCurrentMode(ctx context.Context, locationID string) (*Mode, error) {
	var mode Mode
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s/modes/current", locationID), &mode); err != nil {
		return nil, err
	}
	return &mode, nil
}

// SetCurrentMode sets the current mode for a location.
func (c *Client) SetCurrentMode(ctx context.Context, locationID, modeID string) (*Mode, error) {
	var mode Mode
	body := map[string]string{"mode": modeID}
	if err := c.put(ctx, fmt.Sprintf("/v1/locations/%s/modes/current", locationID), body, &mode); err != nil {
		return nil, err
	}
	return &mode, nil
}

// ---------- Additional Device operations ----------

// UpdateDevice updates device properties (e.g., label, roomId).
func (c *Client) UpdateDevice(ctx context.Context, id string, body map[string]any) (*Device, error) {
	var d Device
	if err := c.put(ctx, fmt.Sprintf("/v1/devices/%s", id), body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// CreateDevice creates a new device.
func (c *Client) CreateDevice(ctx context.Context, body map[string]any) (*Device, error) {
	var d Device
	if err := c.post(ctx, "/v1/devices", body, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ComponentStatus represents the status of a device component.
type ComponentStatus map[string]any

// GetComponentStatus returns status for a specific component of a device.
func (c *Client) GetComponentStatus(ctx context.Context, deviceID, componentID string) (ComponentStatus, error) {
	var status ComponentStatus
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s/components/%s/status", deviceID, componentID), &status); err != nil {
		return nil, err
	}
	return status, nil
}

// CapabilityStatus represents the status of a specific capability on a component.
type CapabilityStatus map[string]any

// GetCapabilityStatus returns status for a specific capability on a device component.
func (c *Client) GetCapabilityStatus(ctx context.Context, deviceID, componentID, capabilityID string) (CapabilityStatus, error) {
	var status CapabilityStatus
	if err := c.get(ctx, fmt.Sprintf("/v1/devices/%s/components/%s/capabilities/%s/status", deviceID, componentID, capabilityID), &status); err != nil {
		return nil, err
	}
	return status, nil
}

// ---------- Additional Location operations ----------

// CreateLocation creates a new location.
func (c *Client) CreateLocation(ctx context.Context, body map[string]any) (*Location, error) {
	var loc Location
	if err := c.post(ctx, "/v1/locations", body, &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// UpdateLocation updates a location.
func (c *Client) UpdateLocation(ctx context.Context, id string, body map[string]any) (*Location, error) {
	var loc Location
	if err := c.put(ctx, fmt.Sprintf("/v1/locations/%s", id), body, &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// DeleteLocation deletes a location.
func (c *Client) DeleteLocation(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("/v1/locations/%s", id))
}

// ---------- Additional Rule operations ----------

// UpdateRule updates an existing rule.
func (c *Client) UpdateRule(ctx context.Context, ruleID string, body any) (*Rule, error) {
	var rule Rule
	if err := c.put(ctx, fmt.Sprintf("/v1/rules/%s", ruleID), body, &rule); err != nil {
		return nil, err
	}
	return &rule, nil
}

// RuleExecutionResponse represents the result of executing a rule.
type RuleExecutionResponse struct {
	ID       string `json:"id"`
	Result   string `json:"result,omitempty"`
	Actions  any    `json:"actions,omitempty"`
}

// ExecuteRule manually triggers a rule.
func (c *Client) ExecuteRule(ctx context.Context, ruleID string) (*RuleExecutionResponse, error) {
	var resp RuleExecutionResponse
	if err := c.post(ctx, fmt.Sprintf("/v1/rules/execute/%s", ruleID), nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ---------- Room devices ----------

// ListRoomDevices returns devices assigned to a specific room.
func (c *Client) ListRoomDevices(ctx context.Context, locationID, roomID string) ([]Device, error) {
	var resp struct {
		Items []Device `json:"items"`
	}
	if err := c.get(ctx, fmt.Sprintf("/v1/locations/%s/rooms/%s/devices", locationID, roomID), &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ---------- Additional Capability operations ----------

// CapabilitySummary represents a brief capability listing.
type CapabilitySummary struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
	Status  string `json:"status,omitempty"`
}

// ListStandardCapabilities returns all standard SmartThings capabilities.
func (c *Client) ListStandardCapabilities(ctx context.Context) ([]CapabilitySummary, error) {
	var resp struct {
		Items []CapabilitySummary `json:"items"`
	}
	if err := c.get(ctx, "/v1/capabilities", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// CapabilityNamespace represents a capability namespace.
type CapabilityNamespace struct {
	Name      string `json:"name"`
	OwnerType string `json:"ownerType,omitempty"`
	OwnerID   string `json:"ownerId,omitempty"`
}

// ListCapabilityNamespaces returns all capability namespaces.
func (c *Client) ListCapabilityNamespaces(ctx context.Context) ([]CapabilityNamespace, error) {
	var resp []CapabilityNamespace
	if err := c.get(ctx, "/v1/capabilities/namespaces", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ---------- Installed Apps ----------

// InstalledApp represents a SmartThings installed app.
type InstalledApp struct {
	InstalledAppID   string `json:"installedAppId"`
	InstalledAppType string `json:"installedAppType"`
	AppID            string `json:"appId"`
	DisplayName      string `json:"displayName"`
	LocationID       string `json:"locationId"`
	InstalledAppStatus string `json:"installedAppStatus"`
}

// ListInstalledApps returns installed apps.
func (c *Client) ListInstalledApps(ctx context.Context) ([]InstalledApp, error) {
	var resp struct {
		Items []InstalledApp `json:"items"`
	}
	if err := c.get(ctx, "/v1/installedapps", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetInstalledApp returns a single installed app.
func (c *Client) GetInstalledApp(ctx context.Context, id string) (*InstalledApp, error) {
	var app InstalledApp
	if err := c.get(ctx, fmt.Sprintf("/v1/installedapps/%s", id), &app); err != nil {
		return nil, err
	}
	return &app, nil
}

// ---------- Notifications ----------

// NotificationRequest defines a push notification to send.
type NotificationRequest struct {
	LocationID string            `json:"locationId,omitempty"`
	Type       string            `json:"type,omitempty"` // e.g., "ALERT"
	Messages   map[string]string `json:"messages,omitempty"` // locale -> message
	DefaultMessage string        `json:"defaultMessage,omitempty"`
}

// SendNotification sends a push notification to the SmartThings mobile app.
func (c *Client) SendNotification(ctx context.Context, req NotificationRequest) error {
	return c.post(ctx, "/v1/notification", req, nil)
}

// ---------- Additional Schedule operations ----------

// GetSchedule returns a specific schedule by name.
func (c *Client) GetSchedule(ctx context.Context, installedAppID, scheduleName string) (*Schedule, error) {
	var sch Schedule
	if err := c.get(ctx, fmt.Sprintf("/v1/installedapps/%s/schedules/%s", installedAppID, scheduleName), &sch); err != nil {
		return nil, err
	}
	return &sch, nil
}

// Helpers

func (c *Client) get(ctx context.Context, path string, out any) error {
	if c.token == "" {
		return fmt.Errorf("SmartThings token not configured. Please set the SMARTTHINGS_TOKEN environment variable")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("SmartThings API error: %s (path: %s): %s", res.Status, path, string(body))
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	if c.token == "" {
		return fmt.Errorf("SmartThings token not configured. Please set the SMARTTHINGS_TOKEN environment variable")
	}

	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("SmartThings API error: %s (path: %s): %s", res.Status, path, string(respBody))
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

func (c *Client) put(ctx context.Context, path string, body any, out any) error {
	if c.token == "" {
		return fmt.Errorf("SmartThings token not configured. Please set the SMARTTHINGS_TOKEN environment variable")
	}

	var buf *bytes.Buffer
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		buf = bytes.NewBuffer(data)
	} else {
		buf = bytes.NewBuffer(nil)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+path, buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("SmartThings API error: %s (path: %s): %s", res.Status, path, string(respBody))
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}

func (c *Client) delete(ctx context.Context, path string) error {
	if c.token == "" {
		return fmt.Errorf("SmartThings token not configured. Please set the SMARTTHINGS_TOKEN environment variable")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("SmartThings API error: %s (path: %s): %s", res.Status, path, string(body))
	}
	return nil
}
