package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	httpheaders "github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/mark"
	"github.com/github/github-mcp-server/pkg/http/oauth"
)

type authType int

const (
	authTypeUnknown authType = iota
	authTypeIDE
	authTypeGhToken
)

var (
	errMissingAuthorizationHeader     = fmt.Errorf("%w: missing required Authorization header", mark.ErrBadRequest)
	errBadAuthorizationHeader         = fmt.Errorf("%w: Authorization header is badly formatted", mark.ErrBadRequest)
	errUnsupportedAuthorizationHeader = fmt.Errorf("%w: unsupported Authorization header", mark.ErrBadRequest)
)

var supportedGitHubPrefixes = []string{
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

func ExtractUserToken(oauthCfg *oauth.Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, token, err := parseAuthorizationHeader(r)
			if err != nil {
				// For missing Authorization header, return 401 with WWW-Authenticate header per MCP spec
				if errors.Is(err, errMissingAuthorizationHeader) {
					sendAuthChallenge(w, r, oauthCfg)
					return
				}
				// For other auth errors (bad format, unsupported), return 400
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			ctx := r.Context()
			ctx = ghcontext.WithTokenInfo(ctx, token)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// sendAuthChallenge sends a 401 Unauthorized response with WWW-Authenticate header
// containing the OAuth protected resource metadata URL as per RFC 6750 and MCP spec.
func sendAuthChallenge(w http.ResponseWriter, r *http.Request, oauthCfg *oauth.Config) {
	resourcePath := oauth.ResolveResourcePath(r, oauthCfg)
	resourceMetadataURL := oauth.BuildResourceMetadataURL(r, oauthCfg, resourcePath)
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q`, resourceMetadataURL))
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
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

	for _, prefix := range supportedGitHubPrefixes {
		if strings.HasPrefix(token, prefix) {
			return authTypeGhToken, token, nil
		}
	}

	matchesOldTokenPattern := oldPatternRegexp.MatchString(token)
	if matchesOldTokenPattern {
		return authTypeGhToken, token, nil
	}

	return 0, "", errBadAuthorizationHeader
}
