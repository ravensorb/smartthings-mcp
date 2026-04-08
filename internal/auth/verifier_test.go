package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// testSetup creates a test RSA keypair and JWKS server.
func testSetup(t *testing.T) (*rsa.PrivateKey, *httptest.Server) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	// Serve JWKS endpoint with the public key.
	jwksServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := privateKey.PublicKey.N
		e := privateKey.PublicKey.E

		// Encode modulus as base64url (unpadded).
		nBytes := n.Bytes()
		eBytes := big.NewInt(int64(e)).Bytes()

		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kty": "RSA",
					"use": "sig",
					"kid": "test-key-1",
					"alg": "RS256",
					"n":   base64urlEncode(nBytes),
					"e":   base64urlEncode(eBytes),
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jwks)
	}))

	t.Cleanup(func() { jwksServer.Close() })
	return privateKey, jwksServer
}

// base64urlEncode encodes bytes as unpadded base64url.
func base64urlEncode(data []byte) string {
	const encodeURL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	result := make([]byte, 0, (len(data)*8+5)/6)
	for i := 0; i < len(data); i += 3 {
		var val uint32
		remaining := len(data) - i
		switch {
		case remaining >= 3:
			val = uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
			result = append(result, encodeURL[val>>18&0x3F], encodeURL[val>>12&0x3F], encodeURL[val>>6&0x3F], encodeURL[val&0x3F])
		case remaining == 2:
			val = uint32(data[i])<<16 | uint32(data[i+1])<<8
			result = append(result, encodeURL[val>>18&0x3F], encodeURL[val>>12&0x3F], encodeURL[val>>6&0x3F])
		case remaining == 1:
			val = uint32(data[i]) << 16
			result = append(result, encodeURL[val>>18&0x3F], encodeURL[val>>12&0x3F])
		}
	}
	return string(result)
}

// signToken creates a signed JWT with the given claims.
func signToken(t *testing.T, privateKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = "test-key-1"
	signed, err := token.SignedString(privateKey)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func testLogger(t *testing.T) *zap.SugaredLogger {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	return logger.Sugar()
}

func TestValidToken(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss":   "https://test-issuer.example.com",
		"aud":   "test-audience",
		"exp":   time.Now().Add(time.Hour).Unix(),
		"sub":   "user-123",
		"email": "user@example.com",
		"scope": "mcp:access mcp:read",
	})

	info, err := verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Expiration.IsZero() {
		t.Error("expected non-zero expiration")
	}

	if len(info.Scopes) != 2 || info.Scopes[0] != "mcp:access" || info.Scopes[1] != "mcp:read" {
		t.Errorf("unexpected scopes: %v", info.Scopes)
	}

	if info.Extra["sub"] != "user-123" {
		t.Errorf("expected sub=user-123, got %v", info.Extra["sub"])
	}
	if info.Extra["email"] != "user@example.com" {
		t.Errorf("expected email=user@example.com, got %v", info.Extra["email"])
	}
}

func TestExpiredToken(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss": "https://test-issuer.example.com",
		"aud": "test-audience",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})

	_, err = verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestWrongIssuer(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss": "https://wrong-issuer.example.com",
		"aud": "test-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err = verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err == nil {
		t.Fatal("expected error for wrong issuer")
	}
}

func TestWrongAudience(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss": "https://test-issuer.example.com",
		"aud": "wrong-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err = verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err == nil {
		t.Fatal("expected error for wrong audience")
	}
}

func TestUnknownSigningKey(t *testing.T) {
	_, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	// Generate a different key not in the JWKS.
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating other key: %v", err)
	}

	tokenStr := signToken(t, otherKey, jwt.MapClaims{
		"iss": "https://test-issuer.example.com",
		"aud": "test-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	_, err = verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err == nil {
		t.Fatal("expected error for unknown signing key")
	}
}

func TestMalformedToken(t *testing.T) {
	_, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	_, err = verifier(context.Background(), "not-a-jwt", httptest.NewRequest("GET", "/", nil))
	if err == nil {
		t.Fatal("expected error for malformed token")
	}
}

func TestScopeExtraction_SCP(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	// Azure AD style: "scp" claim
	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss": "https://test-issuer.example.com",
		"aud": "test-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
		"scp": "User.Read Mail.Send",
	})

	info, err := verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(info.Scopes) != 2 || info.Scopes[0] != "User.Read" || info.Scopes[1] != "Mail.Send" {
		t.Errorf("unexpected scopes from scp claim: %v", info.Scopes)
	}
}

func TestScopeExtraction_Array(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	// JSON array style: "scopes" claim
	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss":    "https://test-issuer.example.com",
		"aud":    "test-audience",
		"exp":    time.Now().Add(time.Hour).Unix(),
		"scopes": []string{"read", "write"},
	})

	info, err := verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(info.Scopes) != 2 || info.Scopes[0] != "read" || info.Scopes[1] != "write" {
		t.Errorf("unexpected scopes from scopes array claim: %v", info.Scopes)
	}
}

func TestNoScopes(t *testing.T) {
	privateKey, jwksServer := testSetup(t)
	logger := testLogger(t)

	verifier, cleanup, err := NewJWTVerifierForTest(jwksServer.URL, "https://test-issuer.example.com", "test-audience", logger)
	if err != nil {
		t.Fatalf("creating verifier: %v", err)
	}
	defer cleanup()

	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss": "https://test-issuer.example.com",
		"aud": "test-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	info, err := verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(info.Scopes) != 0 {
		t.Errorf("expected no scopes, got: %v", info.Scopes)
	}
}

func TestOIDCDiscovery(t *testing.T) {
	privateKey, jwksServer := testSetup(t)

	// Serve OIDC discovery document.
	discoveryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/openid-configuration" {
			http.NotFound(w, r)
			return
		}
		doc := map[string]string{
			"issuer":   "https://test-issuer.example.com",
			"jwks_uri": jwksServer.URL,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	}))
	defer discoveryServer.Close()

	logger := testLogger(t)

	cfg := AuthConfig{
		Enabled:       true,
		OIDCIssuerURL: discoveryServer.URL,
		Audience:      "test-audience",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	verifier, cleanup, err := NewJWTVerifier(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("creating verifier with OIDC discovery: %v", err)
	}
	defer cleanup()

	tokenStr := signToken(t, privateKey, jwt.MapClaims{
		"iss": "https://test-issuer.example.com",
		"aud": "test-audience",
		"exp": time.Now().Add(time.Hour).Unix(),
		"sub": "discovered-user",
	})

	info, err := verifier(context.Background(), tokenStr, httptest.NewRequest("GET", "/", nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Extra["sub"] != "discovered-user" {
		t.Errorf("expected sub=discovered-user, got %v", info.Extra["sub"])
	}
}

func TestLoadAuthConfig_Disabled(t *testing.T) {
	// Clear all MCP_AUTH env vars.
	for _, key := range []string{"MCP_AUTH_ENABLED", "MCP_AUTH_OIDC_ISSUER_URL", "MCP_AUTH_JWKS_URL", "MCP_AUTH_ISSUER", "MCP_AUTH_AUDIENCE"} {
		t.Setenv(key, "")
	}

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Enabled {
		t.Error("expected auth to be disabled")
	}
}

func TestLoadAuthConfig_MissingAudience(t *testing.T) {
	t.Setenv("MCP_AUTH_ENABLED", "true")
	t.Setenv("MCP_AUTH_OIDC_ISSUER_URL", "https://example.com")
	t.Setenv("MCP_AUTH_AUDIENCE", "")

	_, err := LoadAuthConfig()
	if err == nil {
		t.Fatal("expected error for missing audience")
	}
}

func TestLoadAuthConfig_MissingIssuerAndJWKS(t *testing.T) {
	t.Setenv("MCP_AUTH_ENABLED", "true")
	t.Setenv("MCP_AUTH_OIDC_ISSUER_URL", "")
	t.Setenv("MCP_AUTH_JWKS_URL", "")
	t.Setenv("MCP_AUTH_ISSUER", "")
	t.Setenv("MCP_AUTH_AUDIENCE", "test")

	_, err := LoadAuthConfig()
	if err == nil {
		t.Fatal("expected error for missing issuer config")
	}
}

func TestLoadAuthConfig_Valid_OIDC(t *testing.T) {
	t.Setenv("MCP_AUTH_ENABLED", "true")
	t.Setenv("MCP_AUTH_OIDC_ISSUER_URL", "https://example.com/")
	t.Setenv("MCP_AUTH_AUDIENCE", "my-app")
	t.Setenv("MCP_AUTH_SCOPES", "mcp:access,mcp:read")
	t.Setenv("MCP_AUTH_JWKS_URL", "")
	t.Setenv("MCP_AUTH_ISSUER", "")

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !cfg.Enabled {
		t.Error("expected auth to be enabled")
	}
	// Trailing slash should be trimmed.
	if cfg.OIDCIssuerURL != "https://example.com" {
		t.Errorf("expected trimmed issuer URL, got %q", cfg.OIDCIssuerURL)
	}
	if cfg.Audience != "my-app" {
		t.Errorf("expected audience=my-app, got %q", cfg.Audience)
	}
	if len(cfg.RequiredScopes) != 2 {
		t.Errorf("expected 2 scopes, got %v", cfg.RequiredScopes)
	}
}

func TestLoadAuthConfig_Valid_Manual(t *testing.T) {
	t.Setenv("MCP_AUTH_ENABLED", "true")
	t.Setenv("MCP_AUTH_OIDC_ISSUER_URL", "")
	t.Setenv("MCP_AUTH_JWKS_URL", "https://example.com/jwks")
	t.Setenv("MCP_AUTH_ISSUER", "https://example.com")
	t.Setenv("MCP_AUTH_AUDIENCE", "my-app")
	t.Setenv("MCP_AUTH_SCOPES", "")

	cfg, err := LoadAuthConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.JWKSURL != "https://example.com/jwks" {
		t.Errorf("expected JWKS URL, got %q", cfg.JWKSURL)
	}
	if cfg.Issuer != "https://example.com" {
		t.Errorf("expected issuer, got %q", cfg.Issuer)
	}
}
