package auth

import (
	"fmt"
	"os"
	"strings"
)

// AuthConfig holds MCP server authentication settings.
// All fields are populated from environment variables.
type AuthConfig struct {
	// Enabled controls whether JWT auth is enforced on HTTP transports.
	Enabled bool

	// OIDCIssuerURL is the OIDC issuer URL. Discovery is auto-resolved
	// by appending /.well-known/openid-configuration.
	// Example: "https://authentik.example.com/application/o/smartthings-mcp/"
	OIDCIssuerURL string

	// JWKSURL is the URL of the IdP's JWKS endpoint (manual fallback).
	// Example: "https://provider.example.com/.well-known/jwks.json"
	JWKSURL string

	// Issuer is the expected "iss" claim in JWTs (manual fallback).
	// Example: "https://provider.example.com/"
	Issuer string

	// Audience is the expected "aud" claim in JWTs.
	// Example: "smartthings-mcp"
	Audience string

	// RequiredScopes is a list of scopes that every request must have.
	// Example: "mcp:access"
	RequiredScopes []string

	// ResourceID is the resource identifier for RFC 9728 metadata.
	// When set, enables the /.well-known/oauth-protected-resource endpoint.
	ResourceID string

	// AuthorizationServers is a list of authorization server issuer
	// identifiers for RFC 9728 metadata.
	AuthorizationServers []string

	// ClientID is a pre-registered OAuth client ID to hand out via the
	// DCR proxy endpoint (RFC 7591). Optional.
	ClientID string

	// ClientSecret is the corresponding client secret. Optional (empty
	// for public clients).
	ClientSecret string
}

// LoadAuthConfig reads auth configuration from environment variables.
// Returns an error if the configuration is invalid.
func LoadAuthConfig() (AuthConfig, error) {
	cfg := AuthConfig{
		Enabled:       strings.EqualFold(os.Getenv("MCP_AUTH_ENABLED"), "true"),
		OIDCIssuerURL: strings.TrimRight(os.Getenv("MCP_AUTH_OIDC_ISSUER_URL"), "/"),
		JWKSURL:       os.Getenv("MCP_AUTH_JWKS_URL"),
		Issuer:        os.Getenv("MCP_AUTH_ISSUER"),
		Audience:      os.Getenv("MCP_AUTH_AUDIENCE"),
		ResourceID:    os.Getenv("MCP_AUTH_RESOURCE_ID"),
		ClientID:      os.Getenv("MCP_AUTH_CLIENT_ID"),
		ClientSecret:  os.Getenv("MCP_AUTH_CLIENT_SECRET"),
	}

	if scopes := os.Getenv("MCP_AUTH_SCOPES"); scopes != "" {
		cfg.RequiredScopes = strings.Split(scopes, ",")
	}

	if servers := os.Getenv("MCP_AUTH_AUTHORIZATION_SERVERS"); servers != "" {
		cfg.AuthorizationServers = strings.Split(servers, ",")
	}

	if !cfg.Enabled {
		return cfg, nil
	}

	// Validate required fields when auth is enabled.
	if cfg.OIDCIssuerURL == "" && (cfg.JWKSURL == "" || cfg.Issuer == "") {
		return cfg, fmt.Errorf("MCP auth enabled: set MCP_AUTH_OIDC_ISSUER_URL, or both MCP_AUTH_JWKS_URL and MCP_AUTH_ISSUER")
	}

	if cfg.Audience == "" {
		return cfg, fmt.Errorf("MCP auth enabled: MCP_AUTH_AUDIENCE is required")
	}

	return cfg, nil
}
