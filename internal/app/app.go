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

// setupMux creates an HTTP mux with auth middleware and optional metadata endpoint.
func (a *Application) setupMux(mcpHandler http.Handler, authMiddleware func(http.Handler) http.Handler) http.Handler {
	mux := http.NewServeMux()

	// RFC 9728 metadata endpoint (unauthenticated).
	if a.cfg.AuthConfig.ResourceID != "" {
		mux.Handle("/.well-known/oauth-protected-resource", auth.NewProtectedResourceHandler(a.cfg.AuthConfig))
	}

	// MCP handler wrapped with auth middleware.
	protected := authMiddleware(mcpHandler)
	mux.Handle("/mcp", protected)
	mux.Handle("/", protected)

	// CORS wraps everything (outermost).
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, mcp-session-id, mcp-protocol-version")
		w.Header().Set("Access-Control-Expose-Headers", "mcp-session-id, mcp-protocol-version")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		mux.ServeHTTP(w, r)
	})
}

func (a *Application) Start() error {
	a.logger.Info("Starting SmartThings MCP Server...")

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
	case "sse":
		port := a.cfg.Port
		if envPort := os.Getenv("PORT"); envPort != "" {
			if p, err := strconv.Atoi(envPort); err == nil {
				port = p
			}
		}
		addr := fmt.Sprintf("%s:%d", a.cfg.Host, port)
		a.logger.Infof("Starting SSE server on %s", addr)

		if a.cfg.AuthConfig.Enabled {
			a.logger.Warn("SSE transport with auth: TokenInfo will not propagate to tool handlers (SDK v1.1.0 limitation). Consider using 'stream' transport instead.")
		}

		sseHandler := mcp.NewSSEHandler(func(r *http.Request) *mcp.Server {
			// Parse configuration from query parameters
			token := r.URL.Query().Get("SMARTTHINGS_TOKEN")
			if token == "" {
				token = a.cfg.Token
			}

			baseURL := r.URL.Query().Get("ST_BASE_URL")
			if baseURL == "" {
				baseURL = a.cfg.BaseURL
			}

			if token == "" {
				a.logger.Warn("No SmartThings token provided in request or config; tools will be discoverable but execution will fail.")
			}

			stClient := smartthings.NewClient(token, baseURL)
			return srv.NewMCPServer(a.logger, stClient)
		}, nil)

		a.httpServer = &http.Server{
			Addr:    addr,
			Handler: a.setupMux(sseHandler, authMiddleware),
		}

		go func() {
			defer close(a.done)
			if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.logger.Errorf("SSE server error: %v", err)
			}
		}()
	case "stream":
		addr := fmt.Sprintf("%s:%d", a.cfg.Host, a.cfg.Port)
		a.logger.Infof("Starting Stream server on %s", addr)

		streamHandler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
			// Parse configuration from query parameters
			token := r.URL.Query().Get("smartThingsToken")
			if token == "" {
				token = a.cfg.Token
			}

			baseURL := r.URL.Query().Get("stBaseUrl")
			if baseURL == "" {
				baseURL = a.cfg.BaseURL
			}

			if token == "" {
				a.logger.Warn("No SmartThings token provided in request or config; tools will be discoverable but execution will fail.")
			}

			stClient := smartthings.NewClient(token, baseURL)
			return srv.NewMCPServer(a.logger, stClient)
		}, nil)

		a.httpServer = &http.Server{
			Addr:    addr,
			Handler: a.setupMux(streamHandler, authMiddleware),
		}

		go func() {
			defer close(a.done)
			if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.logger.Errorf("Stream server error: %v", err)
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
