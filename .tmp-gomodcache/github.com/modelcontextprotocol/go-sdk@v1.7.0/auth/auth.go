// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// TokenInfo holds information from a bearer token.
type TokenInfo struct {
	Scopes     []string
	Expiration time.Time
	// UserID is an optional identifier for the authenticated user.
	// If set by a TokenVerifier, it can be used by transports to prevent
	// session hijacking by ensuring that all requests for a given session
	// come from the same user.
	UserID string
	Extra  map[string]any
}

// The error that a TokenVerifier should return if the token cannot be verified.
var ErrInvalidToken = errors.New("invalid token")

// The error that a TokenVerifier should return for OAuth-specific protocol errors.
var ErrOAuth = errors.New("oauth error")

// A TokenVerifier checks the validity of a bearer token, and extracts information
// from it. If verification fails, it should return an error that unwraps to ErrInvalidToken.
// The HTTP request is provided in case verifying the token involves checking it.
type TokenVerifier func(ctx context.Context, token string, req *http.Request) (*TokenInfo, error)

// RequireBearerTokenOptions are options for [RequireBearerToken].
type RequireBearerTokenOptions struct {
	// The URL for the resource server metadata OAuth flow, to be returned as part
	// of the WWW-Authenticate header.
	ResourceMetadataURL string
	// The required scopes.
	Scopes []string
	// AllowMissingExpiration opts the middleware out of the
	// `tokenInfo.Expiration.IsZero()` reject. Default false preserves the
	// existing strict behaviour (every TokenInfo must carry an Expiration).
	//
	// Some IdPs emit session-bound bearer tokens that do not carry a standalone
	// `exp` claim — the token's lifetime is bounded by an external session and
	// is not advertised in-band. Resource servers integrating with such IdPs
	// need to opt in to validating the rest of the token (scopes, signature
	// via the verifier callback, etc.) without requiring the expiration field
	// to be present.
	//
	// When enabled, the verifier is still responsible for any session-level
	// validity check it can perform; this option only relaxes the middleware's
	// own expiration enforcement.
	AllowMissingExpiration bool
	// ClockSkew bounds the tolerance applied to a token's Expiration when
	// deciding whether it has elapsed. A token is rejected only if
	// Expiration + ClockSkew is before the current time. Zero (the default)
	// preserves strict comparison: any expired token is rejected immediately.
	//
	// Resource servers running behind a CDN, in distributed deployments, or
	// communicating with an authorization server whose clock drifts a few
	// seconds (common with cloud-managed IdPs) need a small positive value
	// here to avoid rejecting tokens that are valid by the issuer's clock
	// but momentarily appear expired by the verifier's. The same tolerance
	// guards against an issuer's clock running slightly fast at /token
	// issuance time.
	ClockSkew time.Duration
}

type tokenInfoKey struct{}

// TokenInfoFromContext returns the [TokenInfo] stored in ctx, or nil if none.
func TokenInfoFromContext(ctx context.Context) *TokenInfo {
	ti := ctx.Value(tokenInfoKey{})
	if ti == nil {
		return nil
	}
	return ti.(*TokenInfo)
}

// RequireBearerToken returns a piece of middleware that verifies a bearer token using the verifier.
// If verification succeeds, the [TokenInfo] is added to the request's context and the request proceeds.
// If verification fails, the request fails with a 401 Unauthenticated, and the WWW-Authenticate header
// is populated to enable [protected resource metadata].
//
// [protected resource metadata]: https://datatracker.ietf.org/doc/rfc9728
func RequireBearerToken(verifier TokenVerifier, opts *RequireBearerTokenOptions) func(http.Handler) http.Handler {
	// Based on typescript-sdk/src/server/auth/middleware/bearerAuth.ts.

	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenInfo, errmsg, code := verify(r, verifier, opts)
			if code != 0 {
				if code == http.StatusUnauthorized || code == http.StatusForbidden {
					if opts != nil {
						var params []string
						if opts.ResourceMetadataURL != "" {
							params = append(params, fmt.Sprintf("resource_metadata=%q", opts.ResourceMetadataURL))
						}
						if len(opts.Scopes) > 0 {
							params = append(params, fmt.Sprintf("scope=%q", strings.Join(opts.Scopes, " ")))
						}
						if len(params) > 0 {
							w.Header().Add("WWW-Authenticate", "Bearer "+strings.Join(params, ", "))
						}
					}
				}
				http.Error(w, errmsg, code)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), tokenInfoKey{}, tokenInfo))
			handler.ServeHTTP(w, r)
		})
	}
}

func verify(req *http.Request, verifier TokenVerifier, opts *RequireBearerTokenOptions) (_ *TokenInfo, errmsg string, code int) {
	// Extract bearer token.
	authHeader := req.Header.Get("Authorization")
	fields := strings.Fields(authHeader)
	if len(fields) != 2 || strings.ToLower(fields[0]) != "bearer" {
		return nil, "no bearer token", http.StatusUnauthorized
	}

	// Verify the token and get information from it.
	tokenInfo, err := verifier(req.Context(), fields[1], req)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return nil, err.Error(), http.StatusUnauthorized
		}
		if errors.Is(err, ErrOAuth) {
			return nil, err.Error(), http.StatusBadRequest
		}
		return nil, err.Error(), http.StatusInternalServerError
	}
	if tokenInfo == nil {
		return nil, "token validation failed", http.StatusInternalServerError
	}

	// Check scopes. All must be present.
	if opts != nil {
		// Note: quadratic, but N is small.
		for _, s := range opts.Scopes {
			if !slices.Contains(tokenInfo.Scopes, s) {
				return nil, "insufficient scope", http.StatusForbidden
			}
		}
	}

	if opts == nil {
		opts = &RequireBearerTokenOptions{}
	}
	// Check expiration, with optional clock-skew tolerance. Skew only applies
	// when an expiration is present; a missing expiration is governed solely by
	// AllowMissingExpiration.
	if tokenInfo.Expiration.IsZero() {
		if !opts.AllowMissingExpiration {
			return nil, "token missing expiration", http.StatusUnauthorized
		}
	} else if tokenInfo.Expiration.Add(opts.ClockSkew).Before(time.Now()) {
		return nil, "token expired", http.StatusUnauthorized
	}
	return tokenInfo, "", 0
}

// ProtectedResourceMetadataHandler returns an http.Handler that serves OAuth 2.0
// protected resource metadata (RFC 9728) with CORS support.
//
// This handler allows cross-origin requests from any origin (Access-Control-Allow-Origin: *)
// because OAuth metadata is public information intended for client discovery (RFC 9728 §3.1).
// The metadata contains only non-sensitive configuration data about authorization servers
// and supported scopes.
//
// No validation of metadata fields is performed; ensure metadata accuracy at configuration time.
//
// For more sophisticated CORS policies or to restrict origins, wrap this handler with a
// CORS middleware like github.com/rs/cors or github.com/jub0bs/cors.
func ProtectedResourceMetadataHandler(metadata *oauthex.ProtectedResourceMetadata) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers for cross-origin client discovery.
		// OAuth metadata is public information, so allowing any origin is safe.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		// Handle CORS preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Only GET allowed for metadata retrieval
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(metadata); err != nil {
			http.Error(w, "Failed to encode metadata", http.StatusInternalServerError)
			return
		}
	})
}
