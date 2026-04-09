package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"go.uber.org/zap"
)

// oidcDiscovery is the minimal set of fields we need from OIDC discovery.
type oidcDiscovery struct {
	Issuer  string `json:"issuer"`
	JWKSURI string `json:"jwks_uri"`
}

// discoverOIDC fetches the OIDC discovery document from the issuer URL.
func discoverOIDC(ctx context.Context, issuerURL string) (*oidcDiscovery, error) {
	discoveryURL := issuerURL + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, discoveryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating discovery request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching discovery document from %s: %w", discoveryURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discovery endpoint %s returned status %d", discoveryURL, resp.StatusCode)
	}

	var doc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding discovery document: %w", err)
	}

	if doc.Issuer == "" {
		return nil, fmt.Errorf("discovery document missing 'issuer' field")
	}
	if doc.JWKSURI == "" {
		return nil, fmt.Errorf("discovery document missing 'jwks_uri' field")
	}

	return &doc, nil
}

// NewJWTVerifier creates a TokenVerifier that validates JWTs using JWKS-based
// key discovery. It resolves the JWKS URL via OIDC discovery or manual config.
//
// The returned cleanup function must be called to stop background JWKS refresh.
func NewJWTVerifier(ctx context.Context, cfg AuthConfig, logger *zap.SugaredLogger) (sdkauth.TokenVerifier, func(), error) {
	jwksURL := cfg.JWKSURL
	issuer := cfg.Issuer

	// Resolve via OIDC discovery if issuer URL is provided.
	if cfg.OIDCIssuerURL != "" {
		logger.Infof("Discovering OIDC configuration from %s", cfg.OIDCIssuerURL)
		doc, err := discoverOIDC(ctx, cfg.OIDCIssuerURL)
		if err != nil {
			return nil, nil, fmt.Errorf("OIDC discovery failed: %w", err)
		}
		jwksURL = doc.JWKSURI
		issuer = doc.Issuer
		logger.Infof("OIDC discovery: issuer=%s, jwks_uri=%s", issuer, jwksURL)
	}

	if jwksURL == "" {
		return nil, nil, fmt.Errorf("no JWKS URL configured (set MCP_AUTH_OIDC_ISSUER_URL or MCP_AUTH_JWKS_URL)")
	}

	// Initialize JWKS key fetching with background refresh.
	// Use a cancellable context to stop the background goroutine on shutdown.
	jwksCtx, jwksCancel := context.WithCancel(ctx)
	jwks, err := keyfunc.NewDefaultCtx(jwksCtx, []string{jwksURL})
	if err != nil {
		jwksCancel()
		return nil, nil, fmt.Errorf("initializing JWKS from %s: %w", jwksURL, err)
	}

	cleanup := func() {
		jwksCancel()
		_ = jwks // prevent lint warning
	}

	audience := cfg.Audience

	verifier := func(ctx context.Context, tokenString string, req *http.Request) (*sdkauth.TokenInfo, error) {
		// Parse and validate the JWT.
		parserOpts := []jwt.ParserOption{
			jwt.WithValidMethods([]string{"RS256", "RS384", "RS512", "ES256", "ES384", "ES512"}),
			jwt.WithExpirationRequired(),
		}
		if issuer != "" {
			parserOpts = append(parserOpts, jwt.WithIssuer(issuer))
		}
		if audience != "" {
			parserOpts = append(parserOpts, jwt.WithAudience(audience))
		}

		token, err := jwt.Parse(tokenString, jwks.KeyfuncCtx(ctx), parserOpts...)
		if err != nil {
			logger.Debugf("Auth failed: %v", err)
			return nil, fmt.Errorf("%w: %v", sdkauth.ErrInvalidToken, err)
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, fmt.Errorf("%w: unexpected claims type", sdkauth.ErrInvalidToken)
		}

		// Extract expiration.
		exp, err := claims.GetExpirationTime()
		if err != nil || exp == nil {
			return nil, fmt.Errorf("%w: missing expiration", sdkauth.ErrInvalidToken)
		}

		// Extract scopes from "scope" (space-delimited, RFC 8693) or "scp" (Azure AD).
		scopes := extractScopes(claims)

		// Populate extra claims for tool handlers.
		extra := make(map[string]any)
		for _, key := range []string{"sub", "email", "preferred_username", "client_id"} {
			if v, ok := claims[key]; ok {
				extra[key] = v
			}
		}

		// Log authenticated identity.
		sub, _ := extra["sub"].(string)
		email, _ := extra["email"].(string)
		identity := sub
		if email != "" {
			identity = email
		}
		if identity != "" {
			logger.Infof("Authenticated: %s (scopes: %v)", identity, scopes)
		} else {
			logger.Info("Authenticated: unknown identity (no sub/email claim)")
		}

		return &sdkauth.TokenInfo{
			Scopes:     scopes,
			Expiration: exp.Time,
			Extra:      extra,
		}, nil
	}

	logger.Info("JWT auth verifier initialized")
	return verifier, cleanup, nil
}

// extractScopes extracts scopes from JWT claims, supporting multiple conventions.
func extractScopes(claims jwt.MapClaims) []string {
	// RFC 8693 / Keycloak / Authentik: "scope" as space-delimited string
	if scope, ok := claims["scope"].(string); ok && scope != "" {
		return strings.Fields(scope)
	}

	// Azure AD: "scp" as space-delimited string
	if scp, ok := claims["scp"].(string); ok && scp != "" {
		return strings.Fields(scp)
	}

	// Some providers: "scopes" as JSON array
	if scopes, ok := claims["scopes"].([]any); ok {
		result := make([]string, 0, len(scopes))
		for _, s := range scopes {
			if str, ok := s.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}

	return nil
}

// NewJWTVerifierForTest creates a verifier with an explicit JWKS URL and issuer,
// bypassing OIDC discovery. Useful for testing.
func NewJWTVerifierForTest(jwksURL, issuer, audience string, logger *zap.SugaredLogger) (sdkauth.TokenVerifier, func(), error) {
	cfg := AuthConfig{
		Enabled:  true,
		JWKSURL:  jwksURL,
		Issuer:   issuer,
		Audience: audience,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return NewJWTVerifier(ctx, cfg, logger)
}
