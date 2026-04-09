package auth

import (
	"encoding/json"
	"net/http"
)

// ProtectedResourceMetadata represents RFC 9728 Protected Resource Metadata.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers,omitempty"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ResourceName           string   `json:"resource_name,omitempty"`
}

// NewProtectedResourceHandler returns an http.HandlerFunc that serves
// RFC 9728 Protected Resource Metadata. This endpoint is unauthenticated
// so clients can discover how to authenticate.
func NewProtectedResourceHandler(cfg AuthConfig) http.HandlerFunc {
	authServers := cfg.AuthorizationServers
	if len(authServers) == 0 && cfg.ResourceID != "" && cfg.OIDCIssuerURL != "" {
		// Point clients at the MCP server itself, which proxies the
		// upstream IdP's discovery document with a rewritten issuer.
		authServers = []string{cfg.ResourceID}
	}
	if len(authServers) == 0 && cfg.OIDCIssuerURL != "" {
		authServers = []string{cfg.OIDCIssuerURL}
	}
	if len(authServers) == 0 && cfg.Issuer != "" {
		authServers = []string{cfg.Issuer}
	}

	meta := ProtectedResourceMetadata{
		Resource:               cfg.ResourceID,
		AuthorizationServers:   authServers,
		ScopesSupported:        cfg.RequiredScopes,
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "SmartThings MCP Server",
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(meta)
	}
}
