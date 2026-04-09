package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/langowarny/smartthings-mcp/internal/auth"
	srv "github.com/langowarny/smartthings-mcp/internal/server"
	"github.com/langowarny/smartthings-mcp/internal/smartthings"
	"github.com/langowarny/smartthings-mcp/internal/version"
)

type Config struct {
	Transport  string
	Host       string
	Port       int
	Token      string
	BaseURL    string
	AuthConfig auth.AuthConfig
}

type Application struct {
	cfg         Config
	logger      *zap.SugaredLogger
	server      *mcp.Server
	httpServer  *http.Server
	authCleanup func()
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewApplication(cfg Config) (*Application, error) {
	// Configure logger
	zapCfg := zap.NewProductionConfig()
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, err := zapCfg.Build()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Application{
		cfg:    cfg,
		logger: logger.Sugar(),
		ctx:    ctx,
		cancel: cancel,
		done:   make(chan struct{}),
	}, nil
}

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// initAuth initializes JWT auth middleware if enabled. Returns the middleware
// wrapper (identity function if auth disabled) and any error.
func (a *Application) initAuth() (func(http.Handler) http.Handler, error) {
	cfg := a.cfg.AuthConfig
	if !cfg.Enabled {
		return func(h http.Handler) http.Handler { return h }, nil
	}

	verifier, cleanup, err := auth.NewJWTVerifier(a.ctx, cfg, a.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JWT auth: %w", err)
	}
	a.authCleanup = cleanup

	opts := &sdkauth.RequireBearerTokenOptions{
		Scopes: cfg.RequiredScopes,
	}
	if cfg.ResourceID != "" {
		opts.ResourceMetadataURL = cfg.ResourceID + "/.well-known/oauth-protected-resource"
	}

	return sdkauth.RequireBearerToken(verifier, opts), nil
}

// newServerFactory returns an HTTP handler factory that creates a per-request
// MCP server with SmartThings credentials from query parameters or config.
func (a *Application) newServerFactory() func(r *http.Request) *mcp.Server {
	return func(r *http.Request) *mcp.Server {
		// Accept both camelCase (Smithery) and uppercase (legacy) param names.
		token := r.URL.Query().Get("smartThingsToken")
		if token == "" {
			token = r.URL.Query().Get("SMARTTHINGS_TOKEN")
		}
		if token == "" {
			token = a.cfg.Token
		}

		baseURL := r.URL.Query().Get("stBaseUrl")
		if baseURL == "" {
			baseURL = r.URL.Query().Get("ST_BASE_URL")
		}
		if baseURL == "" {
			baseURL = a.cfg.BaseURL
		}

		if token == "" {
			a.logger.Warn("No SmartThings token provided in request or config; tools will be discoverable but execution will fail.")
		}

		stClient := smartthings.NewClient(token, baseURL)
		return srv.NewMCPServer(a.logger, stClient)
	}
}

// corsMiddleware wraps a handler with CORS headers and request/response logging.
func (a *Application) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		a.logger.Debugf("Request: %s %s from %s (session: %s)", r.Method, r.URL.Path, r.RemoteAddr, r.Header.Get("mcp-session-id"))

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, mcp-session-id, mcp-protocol-version")
		w.Header().Set("Access-Control-Expose-Headers", "mcp-session-id, mcp-protocol-version")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)

		a.logger.Debugf("Response: %s %s %d (%s)", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

// setupMux creates an HTTP mux with auth middleware, routing /sse to the SSE
// handler, /mcp to the stream handler, and / to the primary transport.
// Unauthenticated endpoints (RFC 9728 metadata) are registered on a top-level
// mux that delegates auth-protected paths to an inner mux.
func (a *Application) setupMux(sseHandler, streamHandler http.Handler, primaryTransport string, authMiddleware func(http.Handler) http.Handler) http.Handler {
	// Inner mux: all routes require auth.
	authMux := http.NewServeMux()
	authMux.Handle("/sse", authMiddleware(sseHandler))
	authMux.Handle("/mcp", authMiddleware(streamHandler))
	if primaryTransport == "sse" {
		authMux.Handle("/", authMiddleware(sseHandler))
	} else {
		authMux.Handle("/", authMiddleware(streamHandler))
	}

	// Outer mux: unauthenticated routes first, then delegate to authMux.
	topMux := http.NewServeMux()
	if a.cfg.AuthConfig.ResourceID != "" {
		topMux.Handle("GET /.well-known/oauth-protected-resource", auth.NewProtectedResourceHandler(a.cfg.AuthConfig))
	}
	// Redirect OIDC discovery to the authorization server so clients
	// that fetch /.well-known/openid-configuration from this host can
	// find the IdP's metadata.
	if issuer := a.cfg.AuthConfig.OIDCIssuerURL; issuer != "" {
		target := issuer + "/.well-known/openid-configuration"
		topMux.Handle("GET /.well-known/openid-configuration", http.RedirectHandler(target, http.StatusFound))
	}
	topMux.Handle("/", authMux)

	return a.corsMiddleware(topMux)
}

func (a *Application) Start() error {
	a.logger.Infof("Starting SmartThings MCP Server v%s (commit: %s, built: %s)", version.Version, version.Commit, version.Date)

	// Log loaded configuration.
	a.logger.Infof("Config: transport=%s, host=%s, port=%d", a.cfg.Transport, a.cfg.Host, a.cfg.Port)
	if a.cfg.BaseURL != "" {
		a.logger.Infof("Config: SmartThings base URL=%s", a.cfg.BaseURL)
	}
	if a.cfg.Token != "" {
		a.logger.Info("Config: SmartThings token=***configured***")
	} else {
		a.logger.Warn("Config: SmartThings token not set; tools will fail without it")
	}
	if a.cfg.AuthConfig.Enabled {
		if a.cfg.AuthConfig.OIDCIssuerURL != "" {
			a.logger.Infof("Config: auth enabled, OIDC issuer=%s, audience=%s", a.cfg.AuthConfig.OIDCIssuerURL, a.cfg.AuthConfig.Audience)
		} else {
			a.logger.Infof("Config: auth enabled, JWKS=%s, issuer=%s, audience=%s", a.cfg.AuthConfig.JWKSURL, a.cfg.AuthConfig.Issuer, a.cfg.AuthConfig.Audience)
		}
		if len(a.cfg.AuthConfig.RequiredScopes) > 0 {
			a.logger.Infof("Config: required scopes=%v", a.cfg.AuthConfig.RequiredScopes)
		}
		if a.cfg.AuthConfig.ResourceID != "" {
			a.logger.Infof("Config: RFC 9728 metadata enabled, resource=%s", a.cfg.AuthConfig.ResourceID)
		}
	} else {
		a.logger.Info("Config: auth disabled")
	}

	// Initialize SmartThings Client
	stClient := smartthings.NewClient(a.cfg.Token, a.cfg.BaseURL)

	// Initialize MCP Server
	s := srv.NewMCPServer(a.logger, stClient)
	a.server = s

	// Initialize auth middleware (no-op if disabled).
	authMiddleware, err := a.initAuth()
	if err != nil {
		return err
	}

	// Handle Transport
	switch a.cfg.Transport {
	case "stdio":
		go func() {
			defer close(a.done)
			// StdioTransport uses stdin/stdout — no auth (inherently trusted).
			transport := &mcp.StdioTransport{}
			if err := s.Run(a.ctx, transport); err != nil {
				a.logger.Errorf("Stdio server error: %v", err)
			}
		}()
	case "sse", "stream":
		port := a.cfg.Port
		if envPort := os.Getenv("PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil {
				port = p
			}
		}
		addr := fmt.Sprintf("%s:%d", a.cfg.Host, port)
		a.logger.Infof("Starting %s server on %s (all transports available: /sse, /mcp, /)", a.cfg.Transport, addr)

		if a.cfg.AuthConfig.Enabled && a.cfg.Transport == "sse" {
			a.logger.Warn("SSE transport with auth: TokenInfo will not propagate to tool handlers (SDK v1.1.0 limitation). Consider using 'stream' transport instead.")
		}

		factory := a.newServerFactory()
		sseHandler := mcp.NewSSEHandler(factory, nil)
		streamHandler := mcp.NewStreamableHTTPHandler(factory, nil)

		a.httpServer = &http.Server{
			Addr:    addr,
			Handler: a.setupMux(sseHandler, streamHandler, a.cfg.Transport, authMiddleware),
		}

		go func() {
			defer close(a.done)
			if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.logger.Errorf("HTTP server error: %v", err)
			}
		}()
	default:
		return fmt.Errorf("unsupported transport: %s", a.cfg.Transport)
	}

	// Handle Shutdown Signals
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-sigChan:
			a.logger.Infof("Received signal %v, shutting down...", sig)
			a.Stop()
		case <-a.ctx.Done():
		}
	}()

	return nil
}

func (a *Application) Stop() {
	a.cancel()

	if a.authCleanup != nil {
		a.authCleanup()
	}

	if a.httpServer != nil {
		a.logger.Info("Shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.httpServer.Shutdown(ctx); err != nil {
			a.logger.Errorf("Server forced to shutdown: %v", err)
		}
		a.logger.Info("HTTP server stopped")
	}
}

func (a *Application) Wait() {
	<-a.done
	a.logger.Sync()
}
