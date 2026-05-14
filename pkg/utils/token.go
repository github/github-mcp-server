package utils //nolint:revive //TODO: figure out a better name for this package

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	httpheaders "github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/http/mark"
)

type TokenType int

const (
	TokenTypeUnknown TokenType = iota
	TokenTypePersonalAccessToken
	TokenTypeFineGrainedPersonalAccessToken
	TokenTypeOAuthAccessToken
	TokenTypeUserToServerGitHubAppToken
	TokenTypeServerToServerGitHubAppToken
)

var supportedGitHubPrefixes = map[string]TokenType{
	"ghp_":        TokenTypePersonalAccessToken,            // Personal access token (classic)
	"github_pat_": TokenTypeFineGrainedPersonalAccessToken, // Fine-grained personal access token
	"gho_":        TokenTypeOAuthAccessToken,               // OAuth access token
	"ghu_":        TokenTypeUserToServerGitHubAppToken,     // User access token for a GitHub App
	"ghs_":        TokenTypeServerToServerGitHubAppToken,   // Installation access token for a GitHub App (a.k.a. server-to-server token)
}

var (
	ErrMissingAuthorizationHeader     = fmt.Errorf("%w: missing required Authorization header", mark.ErrBadRequest)
	ErrBadAuthorizationHeader         = fmt.Errorf("%w: Authorization header is badly formatted", mark.ErrBadRequest)
	ErrUnsupportedAuthorizationHeader = fmt.Errorf("%w: unsupported Authorization header", mark.ErrBadRequest)
)

// oldPatternRegexp is the regular expression for the old pattern of the token.
// Until 2021, GitHub API tokens did not have an identifiable prefix. They
// were 40 characters long and only contained the characters a-f and 0-9.
var oldPatternRegexp = regexp.MustCompile(`\A[a-f0-9]{40}\z`)

// ParseAuthorizationHeader parses the Authorization header from the HTTP request
func ParseAuthorizationHeader(req *http.Request) (tokenType TokenType, token string, _ error) {
	authHeader := req.Header.Get(httpheaders.AuthorizationHeader)
	if authHeader == "" {
		return 0, "", ErrMissingAuthorizationHeader
	}

	switch {
	// decrypt dotcom token and set it as token
	case strings.HasPrefix(authHeader, "GitHub-Bearer "):
		return 0, "", ErrUnsupportedAuthorizationHeader
	default:
		// support both "Bearer" and "bearer" to conform to api.github.com
		if len(authHeader) > 7 && strings.EqualFold(authHeader[:7], "Bearer ") {
			token = authHeader[7:]
		} else {
			token = authHeader
		}
	}

	for prefix, tokenType := range supportedGitHubPrefixes {
		if strings.HasPrefix(token, prefix) {
			return tokenType, token, nil
		}
	}

	matchesOldTokenPattern := oldPatternRegexp.MatchString(token)
	if matchesOldTokenPattern {
		return TokenTypePersonalAccessToken, token, nil
	}

	return 0, "", ErrBadAuthorizationHeader
}
