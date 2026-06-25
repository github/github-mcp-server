package servercard

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/go-chi/chi/v5"
)

// Handler serves the GitHub MCP Server's Server Card over HTTP.
//
// The card is public metadata: the handler requires no authentication, sets
// permissive CORS headers so browser-based clients can fetch it, and advises
// caching. It mirrors the structure of the OAuth protected-resource-metadata
// handler so the remote server repository can mount it identically and supply
// a per-environment remote URL via Config.
type Handler struct {
	cfg Config
}

// NewHandler returns a Handler that serves the card built from cfg.
func NewHandler(cfg Config) *Handler {
	return &Handler{cfg: cfg}
}

// RegisterRoutes mounts the Server Card handler at the reserved
// `<streamable-http-url>/server-card` location. Because GitHub's hosted edge
// strips the `/mcp` base path before forwarding, both `/server-card` and
// `/mcp/server-card` are registered so the card is reachable either way. The
// handler is registered for all methods (mirroring oauth.AuthHandler) so it
// owns the path and answers non-GET requests itself rather than falling through
// to the auth-gated MCP endpoint.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Handle(Path, h)
	r.Handle("/mcp"+Path, h)
}

// ServeHTTP serves the Server Card as application/mcp-server-card+json.
//
// It honors GET (and OPTIONS preflight), performs content negotiation against
// the Accept header, and is safe to mount at <streamable-http-url>/server-card
// without any authentication middleware.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setCORSHeaders(w)

	switch r.Method {
	case http.MethodOptions:
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodGet, http.MethodHead:
		// served below
	default:
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !acceptsCard(r.Header.Get(headers.AcceptHeader)) {
		http.Error(w, "not acceptable: expected "+MediaType, http.StatusNotAcceptable)
		return
	}

	body, err := json.Marshal(NewServerCard(h.cfg))
	if err != nil {
		http.Error(w, "failed to encode server card", http.StatusInternalServerError)
		return
	}

	w.Header().Set(headers.ContentTypeHeader, MediaType)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// setCORSHeaders applies the read-only CORS policy required for discovery
// endpoints. The card contains only public metadata, so any origin may read it.
func setCORSHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

// acceptsCard reports whether an Accept header value permits the Server Card
// media type. An empty header accepts anything; otherwise the header must list
// the card media type, application/*, or */*. Parameters such as q-values and
// the media-type suffix structure are tolerated.
func acceptsCard(accept string) bool {
	if strings.TrimSpace(accept) == "" {
		return true
	}

	for part := range strings.SplitSeq(accept, ",") {
		mediaRange := strings.TrimSpace(part)
		if i := strings.IndexByte(mediaRange, ';'); i >= 0 {
			mediaRange = strings.TrimSpace(mediaRange[:i])
		}
		switch strings.ToLower(mediaRange) {
		case MediaType, "*/*", "application/*":
			return true
		}
	}
	return false
}
