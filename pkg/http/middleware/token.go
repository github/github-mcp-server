package middleware

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	httpheaders "github.com/github/github-mcp-server/pkg/http/headers"
)

type authType int

const (
	authTypeUnknown authType = iota
	authTypeIDE
	authTypeGhToken
)

var (
	errMissingAuthorizationHeader     = errors.New("missing Authorization header")
	errBadAuthorizationHeader         = errors.New("bad Authorization header format")
	errUnsupportedAuthorizationHeader = errors.New("unsupported Authorization header format")
)

var supportedThirdPartyTokenPrefixes = []string{
	"ghp_",        // Personal access token (classic)
	"github_pat_", // Fine-grained personal access token
	"gho_",        // OAuth access token
	"ghu_",        // User access token for a GitHub App
	"ghs_",        // Installation access token for a GitHub App (a.k.a. server-to-server token)
}

// oldPatternRegexp is the regular expression for the old pattern of the token.
// Until 2021, GitHub API tokens did not have an identifiable prefix. They
// were 40 characters long and only contained the characters a-f and 0-9.
var oldPatternRegexp = regexp.MustCompile(`\A[a-f0-9]{40}\z`)

type tokenCtxKey string

var tokenContextKey tokenCtxKey = "tokenctx"

type TokenData struct {
	Token string
}

// AddToken adds the given token data to the context.
func AddToken(ctx context.Context, data *TokenData) context.Context {
	return context.WithValue(ctx, tokenContextKey, data)
}

// ReqData returns the request data from the context. It will panic if there is
// no data in the context (which should never happen in production).
func Token(ctx context.Context) *TokenData {
	d, ok := ctx.Value(tokenContextKey).(*TokenData)
	if !ok || d == nil {
		// This should never happen in production, so making it a panic saves us a lot of unnecessary error handling.
		panic(errors.New("context does not contain request context token data"))
	}
	return d
}

// ExtractUserToken is a middleware that extracts the user token from the request
// and adds it to the request context. It also validates the token format.
func ExtractUserToken() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, token, err := parseAuthorizationHeader(r)
			if err != nil {
				// For missing Authorization header, return 401 with WWW-Authenticate header per MCP spec
				if errors.Is(err, errMissingAuthorizationHeader) {
					// sendAuthChallenge(w, r, cfg, obsv)
					return
				}
				// For other auth errors (bad format, unsupported), return 400
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Add token info to context
			ctx := r.Context()
			ctx = AddToken(ctx, &TokenData{Token: token})

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func parseAuthorizationHeader(req *http.Request) (authType authType, token string, _ error) {
	authHeader := req.Header.Get(httpheaders.AuthorizationHeader)
	if authHeader == "" {
		return 0, "", errMissingAuthorizationHeader
	}

	switch {
	// decrypt dotcom token and set it as token
	case strings.HasPrefix(authHeader, "GitHub-Bearer "):
		return 0, "", errUnsupportedAuthorizationHeader
	default:
		// support both "Bearer" and "bearer" to conform to api.github.com
		if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
			token = authHeader[7:]
		} else {
			token = authHeader
		}
	}

	// Do a naïve check for a colon in the token - currently, only the IDE token has a colon in it.
	// ex: tid=1;exp=25145314523;chat=1:<hmac>
	if strings.Contains(token, ":") {
		return authTypeIDE, token, nil
	}

	for _, prefix := range supportedThirdPartyTokenPrefixes {
		if strings.HasPrefix(token, prefix) {
			return authTypeGhToken, token, nil
		}
	}

	matchesOldTokenPattern := oldPatternRegexp.MatchString(token)
	if matchesOldTokenPattern {
		return authTypeGhToken, token, nil
	}

	return authTypeUnknown, "", errBadAuthorizationHeader
}
