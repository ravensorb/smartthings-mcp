package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	cacheTTL          = 5 * time.Minute
	upstreamTimeout   = 10 * time.Second
	cacheControlValue = "public, max-age=300"
)

// AuthServerMetadataProxy fetches and caches the upstream IdP's OIDC discovery
// document, rewrites the issuer field to match the MCP server's ResourceID,
// and serves the result so that RFC 8414 issuer validation passes.
type AuthServerMetadataProxy struct {
	oidcIssuerURL        string
	issuerOverride       string
	RegistrationEndpoint string
	logger               *zap.SugaredLogger
	client               *http.Client

	mu        sync.RWMutex
	cached    []byte
	fetchedAt time.Time
}

// NewAuthServerMetadataProxy creates a proxy that fetches OIDC discovery from
// oidcIssuerURL and rewrites the issuer field to issuerOverride.
func NewAuthServerMetadataProxy(oidcIssuerURL, issuerOverride string, logger *zap.SugaredLogger) *AuthServerMetadataProxy {
	return &AuthServerMetadataProxy{
		oidcIssuerURL:  oidcIssuerURL,
		issuerOverride: issuerOverride,
		logger:         logger,
		client:         &http.Client{Timeout: upstreamTimeout},
	}
}

func (p *AuthServerMetadataProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	data, err := p.get()
	if err != nil {
		p.logger.Errorf("authserver metadata proxy: %v", err)
		http.Error(w, "failed to fetch authorization server metadata", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", cacheControlValue)
	w.Write(data)
}

// get returns cached metadata if fresh, otherwise fetches from upstream.
// On upstream error it returns stale data if available.
func (p *AuthServerMetadataProxy) get() ([]byte, error) {
	// Fast path: read lock.
	p.mu.RLock()
	if p.cached != nil && time.Since(p.fetchedAt) < cacheTTL {
		data := p.cached
		p.mu.RUnlock()
		return data, nil
	}
	stale := p.cached
	p.mu.RUnlock()

	// Slow path: write lock with double-check.
	p.mu.Lock()
	defer p.mu.Unlock()

	// Another goroutine may have refreshed while we waited.
	if p.cached != nil && time.Since(p.fetchedAt) < cacheTTL {
		return p.cached, nil
	}

	data, err := p.fetchAndRewrite()
	if err != nil {
		if stale != nil {
			p.logger.Warnf("authserver metadata proxy: upstream fetch failed, serving stale: %v", err)
			return stale, nil
		}
		return nil, err
	}

	p.cached = data
	p.fetchedAt = time.Now()
	return data, nil
}

// fetchAndRewrite fetches the upstream OIDC discovery document and rewrites
// the issuer field.
func (p *AuthServerMetadataProxy) fetchAndRewrite() ([]byte, error) {
	url := p.oidcIssuerURL + "/.well-known/openid-configuration"
	resp, err := p.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", url, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("decoding JSON from %s: %w", url, err)
	}

	doc["issuer"] = p.issuerOverride
	if p.RegistrationEndpoint != "" {
		doc["registration_endpoint"] = p.RegistrationEndpoint
	}

	rewritten, err := json.Marshal(doc)
	if err != nil {
		return nil, fmt.Errorf("encoding rewritten metadata: %w", err)
	}

	return rewritten, nil
}
