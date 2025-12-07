package smartthings

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// GetLocation returns metadata for a single location by ID.
func (c *Client) GetLocation(id string) (*Location, error) {
	var loc Location
	if err := c.get(fmt.Sprintf("/v1/locations/%s", id), &loc); err != nil {
		return nil, err
	}
	return &loc, nil
}

// DeviceStatus wraps /status response (partial).
type DeviceStatus map[string]interface{}

// ListDevices fetches devices.
func (c *Client) ListDevices() ([]Device, error) {
	var resp struct {
		Items []Device `json:"items"`
	}
	if err := c.get("/v1/devices", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// GetDevice returns metadata for a device.
func (c *Client) GetDevice(id string) (*Device, error) {
	var d Device
	if err := c.get(fmt.Sprintf("/v1/devices/%s", id), &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// GetDeviceStatus returns live status of a device.
func (c *Client) GetDeviceStatus(id string) (DeviceStatus, error) {
	var status DeviceStatus
	if err := c.get(fmt.Sprintf("/v1/devices/%s/status", id), &status); err != nil {
		return nil, err
	}
	return status, nil
}

// SendDeviceCommand issues a command.
func (c *Client) SendDeviceCommand(id string, body interface{}) error {
	return c.post(fmt.Sprintf("/v1/devices/%s/commands", id), body, nil)
}

// ListLocations returns locations.
func (c *Client) ListLocations() ([]Location, error) {
	var resp struct {
		Items []Location `json:"items"`
	}
	if err := c.get("/v1/locations", &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

type ExecuteSceneResponse struct {
	SceneID string `json:"sceneId"`
	Status  string `json:"status"`
}

// ListDevicesByLocation fetches devices filtered by location ID.
func (c *Client) ListDevicesByLocation(locationID string) ([]Device, error) {
	var resp struct {
		Items []Device `json:"items"`
	}
	path := fmt.Sprintf("/v1/devices?locationId=%s", locationID)
	if err := c.get(path, &resp); err != nil {
		return nil, err
	}
	return resp.Items, nil
}

// ExecuteScene triggers a scene.
func (c *Client) ExecuteScene(sceneID string) (*ExecuteSceneResponse, error) {
	var res ExecuteSceneResponse
	if err := c.post(fmt.Sprintf("/v1/scenes/%s/execute", sceneID), nil, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Helpers
func (c *Client) get(path string, out interface{}) error {
	// Check if token is configured before making API calls
	if c.token == "" {
		return fmt.Errorf("SmartThings token not configured. Please set the SMARTTHINGS_TOKEN environment variable")
	}

	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("SmartThings API error: %s (path: %s)", res.Status, path)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (c *Client) post(path string, body interface{}, out interface{}) error {
	// Check if token is configured before making API calls
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
	req, err := http.NewRequest(http.MethodPost, c.baseURL+path, buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("SmartThings API error: %s (path: %s)", res.Status, path)
	}
	if out != nil {
		return json.NewDecoder(res.Body).Decode(out)
	}
	return nil
}
