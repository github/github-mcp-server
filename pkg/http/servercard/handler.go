package servercard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/go-chi/chi/v5"
)

// Handler serves the GitHub MCP Server's Server Card over HTTP as public,
// no-auth metadata. It mirrors the OAuth protected-resource-metadata handler so
// the remote server repository can mount it and supply a per-environment remote
// URL via Config.
type Handler struct {
	cfg Config
}

// NewHandler returns a Handler that serves the card built from cfg.
func NewHandler(cfg Config) *Handler {
	return &Handler{cfg: cfg}
}

// RegisterRoutes mounts the handler at the single canonical Path for every
// method (mirroring oauth.AuthHandler) so it owns the route — answering non-GET
// requests itself rather than falling through to the auth-gated MCP endpoint —
// and is deliberately exposed at no alternate path.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Handle(Path, h)
}

// ServeHTTP serves the Server Card as application/mcp-server-card+json.
//
// It honors GET and HEAD (with OPTIONS preflight), performs content negotiation
// against the Accept header, supports ETag conditional requests, and is safe to
// mount at <streamable-http-url>/server-card without authentication middleware.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodOptions:
		setCORSHeaders(w)
		w.WriteHeader(http.StatusOK)
		return
	case http.MethodGet, http.MethodHead:
		// served below
	default:
		setCORSHeaders(w)
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !acceptsCard(joinFieldLines(r.Header.Values(headers.AcceptHeader))) {
		setCORSHeaders(w)
		http.Error(w, "not acceptable: expected "+MediaType, http.StatusNotAcceptable)
		return
	}

	ServeCard(w, r, h.resolveCard(r))
}

// resolveCard builds the card for a request, applying the per-request
// RemoteURLFunc override when configured.
func (h *Handler) resolveCard(r *http.Request) *ServerCard {
	cfg := h.cfg
	if h.cfg.RemoteURLFunc != nil {
		if url := h.cfg.RemoteURLFunc(r); url != "" {
			cfg.RemoteURL = url
		}
	}
	return NewServerCard(cfg)
}

// ServeCard writes card to w as the canonical Server Card response and is the
// single source of truth for its headers and conditional-request behavior, so
// callers that build a card per request get byte-identical ETag and headers.
//
// It sets the CORS headers, a one-hour Cache-Control, and a strong ETag. A
// matching If-None-Match (strong, weak, or `*`) yields 304; HEAD omits the body.
// Callers handle method dispatch and Accept negotiation first.
func ServeCard(w http.ResponseWriter, r *http.Request, card *ServerCard) {
	body, err := json.Marshal(card)
	if err != nil {
		setCORSHeaders(w)
		http.Error(w, "failed to encode server card", http.StatusInternalServerError)
		return
	}

	etag := computeETag(body)

	setCORSHeaders(w)
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("ETag", etag)

	if ifNoneMatchSatisfied(joinFieldLines(r.Header.Values("If-None-Match")), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set(headers.ContentTypeHeader, MediaType)
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// computeETag returns a strong ETag: the lowercase-hex SHA-256 of body, wrapped
// in double quotes. It is deterministic for identical content.
func computeETag(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// ifNoneMatchSatisfied reports whether an If-None-Match header value matches the
// given strong ETag using RFC 9110 weak comparison: `*` always matches, and a
// listed entity-tag matches if its opaque tag equals etag's, ignoring any weak
// `W/` prefix.
func ifNoneMatchSatisfied(ifNoneMatch, etag string) bool {
	ifNoneMatch = strings.TrimSpace(ifNoneMatch)
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}

	target := strings.TrimPrefix(etag, "W/")
	for _, candidate := range splitList(ifNoneMatch, ',', false) {
		if strings.TrimPrefix(strings.TrimSpace(candidate), "W/") == target {
			return true
		}
	}
	return false
}

// setCORSHeaders applies the read-only CORS + caching headers required by the
// experimental-ext-server-card discovery spec. The card is public metadata, so
// any origin may read it. Because the response is publicly cacheable and the
// remote URL derives from the trusted X-Forwarded-Host, Vary declares Accept and
// X-Forwarded-Host so a shared cache cannot serve one tenant's card to another;
// it is added (not set) to preserve values contributed by upstream middleware.
func setCORSHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Access-Control-Allow-Origin", "*")
	h.Set("Access-Control-Allow-Methods", "GET")
	h.Set("Access-Control-Allow-Headers", "Content-Type, If-None-Match")
	h.Set("Access-Control-Expose-Headers", "ETag")
	h.Add("Vary", "Accept, X-Forwarded-Host")
}

// joinFieldLines merges a list-valued header's repeated field-lines, since net/http's Header.Get returns only the first.
func joinFieldLines(values []string) string {
	return strings.Join(values, ",")
}

// acceptsCard reports whether an Accept header value permits the Server Card
// media type, honoring RFC 9110 quality values. An empty header accepts
// anything. Otherwise the most specific matching media range decides the
// result — the exact card type over application/* over */* — and an explicit
// q=0 on that range rejects the representation (yielding 406). A range that
// carries a media parameter (one appearing before q) does not match the
// parameterless card and is skipped.
func acceptsCard(accept string) bool {
	if strings.TrimSpace(accept) == "" {
		return true
	}

	bestSpecificity := -1
	var bestQuality float64
	for _, part := range splitList(accept, ',', true) {
		mediaRange, quality, hasParams := parseMediaRange(part)
		specificity := -1
		if !hasParams {
			switch mediaRange {
			case MediaType:
				specificity = 2
			case "application/*":
				specificity = 1
			case "*/*":
				specificity = 0
			}
		}
		if specificity > bestSpecificity {
			bestSpecificity = specificity
			bestQuality = quality
		}
	}
	return bestSpecificity >= 0 && bestQuality > 0
}

// parseMediaRange splits one Accept media range into its lowercased media type,
// quality value, and whether it carries a media parameter. Parameters up to the
// first q are media parameters (which the parameterless card cannot satisfy, so
// hasParams is set); q sets the quality (default 1.0); segments after q are
// accept extensions and are ignored.
func parseMediaRange(part string) (mediaType string, quality float64, hasParams bool) {
	segments := splitList(part, ';', true)
	mediaType = strings.ToLower(strings.TrimSpace(segments[0]))
	quality = 1.0
	seenQ := false
	for _, seg := range segments[1:] {
		name, value, _ := strings.Cut(seg, "=")
		if strings.EqualFold(strings.TrimSpace(name), "q") {
			seenQ = true
			if q, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				quality = q
			}
			continue
		}
		if !seenQ && strings.TrimSpace(name) != "" {
			hasParams = true
		}
	}
	return mediaType, quality, hasParams
}

// splitList splits an HTTP list value on sep, honoring RFC 9110 quoted strings so a
// quoted separator does not divide the list. escape enables backslash quoted-pairs
// (Accept parameter values); entity-tags pass false and treat backslash literally.
func splitList(s string, sep byte, escape bool) []string {
	var out []string
	start, inQuote, esc := 0, false, false
	for i := range len(s) {
		switch {
		case esc:
			esc = false
		case inQuote && escape && s[i] == '\\':
			esc = true
		case s[i] == '"':
			inQuote = !inQuote
		case s[i] == sep && !inQuote:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
