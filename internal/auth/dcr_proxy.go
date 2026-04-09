package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// DCRProxyHandler implements a minimal RFC 7591 Dynamic Client Registration
// endpoint that returns a pre-registered client_id (and optional secret)
// for IdPs like Authentik that don't support DCR natively.
type DCRProxyHandler struct {
	clientID     string
	clientSecret string
	logger       *zap.SugaredLogger
}

// NewDCRProxyHandler creates a DCR proxy that hands out the given client
// credentials to any registration request.
func NewDCRProxyHandler(clientID, clientSecret string, logger *zap.SugaredLogger) *DCRProxyHandler {
	return &DCRProxyHandler{
		clientID:     clientID,
		clientSecret: clientSecret,
		logger:       logger,
	}
}

func (h *DCRProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the incoming registration request (we echo back select fields).
	var req map[string]any
	if r.Body != nil {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req = map[string]any{}
		}
	} else {
		req = map[string]any{}
	}

	resp := map[string]any{
		"client_id":                h.clientID,
		"client_id_issued_at":     time.Now().Unix(),
		"client_secret_expires_at": 0,
	}

	if h.clientSecret != "" {
		resp["client_secret"] = h.clientSecret
	}

	// Echo back standard fields from the request if provided.
	for _, field := range []string{
		"redirect_uris",
		"grant_types",
		"response_types",
		"token_endpoint_auth_method",
		"client_name",
	} {
		if v, ok := req[field]; ok {
			resp[field] = v
		}
	}

	h.logger.Infof("DCR proxy: registered client_id=%s for client_name=%v", h.clientID, req["client_name"])

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Debugf("DCR proxy: write response: %v", err)
	}
}
