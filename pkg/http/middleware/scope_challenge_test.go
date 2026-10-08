package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/github/github-mcp-server/v2/pkg/http/oauth"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWithScopeChallenge_MaxBodySize covers the fallback body-parsing path,
// used when WithMCPParse has not already populated MCPMethodInfo in context.
func TestWithScopeChallenge_MaxBodySize(t *testing.T) {
	const limit = 64
	oauthCfg := &oauth.Config{}
	fetcher := &mockScopeFetcher{scopes: []string{"repo"}}

	newRequestWithBody := func(body io.Reader) *http.Request {
		req := httptest.NewRequest(http.MethodPost, "/mcp", body)
		ctx := ghcontext.WithTokenInfo(req.Context(), &ghcontext.TokenInfo{
			Token:     "******",
			TokenType: utils.TokenTypeOAuthAccessToken,
		})
		return req.WithContext(ctx)
	}

	newRequest := func(body string) *http.Request {
		return newRequestWithBody(strings.NewReader(body))
	}

	t.Run("oversized body is rejected before the fallback parse", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"` + strings.Repeat("x", limit) + `"}}`
		require.Greater(t, len(body), limit)

		var nextCalled bool
		next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			nextCalled = true
		})

		handler := WithMaxBodySize(limit)(WithScopeChallenge(oauthCfg, fetcher)(next))

		// An unknown length skips WithMaxBodySize's Content-Length fast path,
		// so the overflow surfaces from the fallback read.
		req := newRequestWithBody(unknownLengthBody(body))
		require.Equal(t, int64(-1), req.ContentLength, "test setup: Content-Length should be unknown")

		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.False(t, nextCalled, "downstream handler must not run for an oversized request")
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
		assert.Contains(t, rr.Body.String(), "request body too large")
	})

	t.Run("allowed body still reaches the fallback parse and next handler", func(t *testing.T) {
		body := `{"jsonrpc":"2.0","method":"tools/list"}`
		require.LessOrEqual(t, len(body), limit)

		var nextCalled bool
		var capturedBody string
		next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			nextCalled = true
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			capturedBody = string(b)
		})

		handler := WithMaxBodySize(limit)(WithScopeChallenge(oauthCfg, fetcher)(next))

		req := newRequest(body)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)

		assert.True(t, nextCalled, "downstream handler should run for an allowed request")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, body, capturedBody, "body should be preserved for downstream handlers")
	})
}

func TestWithScopeChallengeResolvesScopesFromParsedArguments(t *testing.T) {
	setDynamicScopeTestMap(t)

	tests := []struct {
		name       string
		arguments  map[string]any
		wantStatus int
		wantNext   bool
	}{
		{
			name:       "regular file only requires repo",
			arguments:  map[string]any{"path": "README.md"},
			wantStatus: http.StatusNoContent,
			wantNext:   true,
		},
		{
			name:       "non-ASCII workflow path without header requires workflow",
			arguments:  map[string]any{"path": ".github/workflows/构建.yml"},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "workflow in array requires workflow",
			arguments: map[string]any{"files": []any{
				map[string]any{"path": "README.md"},
				map[string]any{"path": ".github/workflows/ci.yml"},
			}},
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusNoContent)
			})
			handler := WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next)

			request := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			assert.Empty(t, request.Header.Get("Mcp-Param-path"))
			rawArguments, err := json.Marshal(tt.arguments)
			require.NoError(t, err)
			ctx := scopeChallengeContext(request.Context())
			ctx = ghcontext.WithMCPMethodInfo(ctx, &ghcontext.MCPMethodInfo{
				Method:       "tools/call",
				ItemName:     "write_file",
				RawArguments: rawArguments,
			})
			request = request.WithContext(ctx)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			assert.Equal(t, tt.wantStatus, response.Code)
			assert.Equal(t, tt.wantNext, nextCalled)
			if tt.wantStatus == http.StatusForbidden {
				challenge := response.Header().Get("WWW-Authenticate")
				assert.Contains(t, challenge, `scope="repo workflow"`)
				assert.Contains(t, challenge, "Additional scopes required: repo, workflow")
			}
		})
	}
}

func TestWithScopeChallengeNormalizesArgumentsBeforeDynamicPolicy(t *testing.T) {
	normalizerCalls := 0
	var challengedArguments map[string]any
	setScopeTestMap(t, inventory.ScopeAccess{
		Scopes:  []string{"repo", "workflow"},
		Dynamic: true,
		ArgumentNormalizer: func(raw json.RawMessage) (json.RawMessage, error) {
			normalizerCalls++
			var arguments map[string]json.RawMessage
			if err := json.Unmarshal(raw, &arguments); err != nil {
				return nil, err
			}
			arguments["state"] = json.RawMessage(`"OPEN"`)
			return json.Marshal(arguments)
		},
		Challenge: func(arguments map[string]any, _ []string) []string {
			challengedArguments = arguments
			if arguments["state"] == "OPEN" {
				return nil
			}
			return []string{"workflow"}
		},
	})

	for _, parsed := range []bool{true, false} {
		t.Run(fmt.Sprintf("parsed=%t", parsed), func(t *testing.T) {
			normalizerCalls = 0
			nextCalled := false
			var normalizedInfo *ghcontext.MCPMethodInfo
			next := http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				nextCalled = true
				normalizedInfo, _ = ghcontext.MCPMethod(request.Context())
			})
			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(
				`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"write_file","arguments":{"state":"open"}}}`,
			))
			ctx := scopeChallengeContext(request.Context())
			methodInfo := &ghcontext.MCPMethodInfo{
				Method:       "tools/call",
				ItemName:     "write_file",
				RawArguments: json.RawMessage(`{"state":"open"}`),
			}
			if parsed {
				ctx = ghcontext.WithMCPMethodInfo(ctx, methodInfo)
			}
			request = request.WithContext(ctx)

			response := httptest.NewRecorder()
			WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next).ServeHTTP(response, request)

			require.Equal(t, http.StatusOK, response.Code)
			require.True(t, nextCalled)
			assert.Equal(t, 1, normalizerCalls)
			assert.Equal(t, "OPEN", challengedArguments["state"])
			require.NotNil(t, normalizedInfo)
			assert.True(t, normalizedInfo.ArgumentsNormalized)
			assert.JSONEq(t, `{"state":"OPEN"}`, string(normalizedInfo.NormalizedArguments))
			assert.False(t, methodInfo.ArgumentsNormalized, "normalization must not mutate the original parsed metadata")
		})
	}
}

func TestWithScopeChallengeResolvesScopesFromFallbackBody(t *testing.T) {
	setDynamicScopeTestMap(t)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	handler := WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next)

	body := []byte(`{"jsonrpc":"2.0","method":"tools/call","params":{"name":"write_file","arguments":{"path":".github/workflows/ci.yml"}}}`)
	request := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
	request = request.WithContext(scopeChallengeContext(request.Context()))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
	assert.False(t, nextCalled)
	assert.Contains(t, response.Header().Get("WWW-Authenticate"), "Additional scopes required: repo, workflow")
}

func TestWithScopeChallengeSkipsArgumentDecodeWhenMaximumScopesGranted(t *testing.T) {
	callbackCalls := 0
	setScopeTestMap(t, scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.Workflow},
		func([]string) bool { return true },
		func(map[string]any, []string) []string {
			callbackCalls++
			return []string{"unexpected"}
		},
	))

	tests := []struct {
		name      string
		arguments string
		validate  func(*testing.T, *ghcontext.MCPMethodInfo)
	}{
		{
			name:      "invalid argument shape reaches downstream validation",
			arguments: `["not","an","object"]`,
			validate: func(t *testing.T, info *ghcontext.MCPMethodInfo) {
				_, err := info.DecodeArguments()
				assert.Error(t, err)
			},
		},
		{
			name:      "large nested arguments remain raw",
			arguments: `{"nested":[{"payload":"` + strings.Repeat("x", 32*1024) + `"},[1,2,3]]}`,
			validate: func(t *testing.T, info *ghcontext.MCPMethodInfo) {
				assert.Greater(t, len(info.RawArguments), 32*1024)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			callbackCalls = 0
			body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"write_file","arguments":` + tt.arguments + `}}`
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				info, ok := ghcontext.MCPMethod(r.Context())
				require.True(t, ok)
				require.NotNil(t, info)
				tt.validate(t, info)
				receivedBody, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				assert.Equal(t, body, string(receivedBody))
				w.WriteHeader(http.StatusUnprocessableEntity)
			})
			handler := WithMCPParse()(WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next))

			request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
			request = request.WithContext(scopeChallengeContextWithScopes(request.Context(), []string{"repo", "workflow"}))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			assert.True(t, nextCalled)
			assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
			assert.Zero(t, callbackCalls)
		})
	}
}

func TestWithScopeChallengeMaximumScopeHierarchyFastPath(t *testing.T) {
	callbackCalls := 0
	setScopeTestMap(t, scopes.DynamicChallenge(
		[]scopes.Scope{scopes.ReadOrg},
		func([]string) bool { return true },
		func(map[string]any, []string) []string {
			callbackCalls++
			return []string{"unexpected"}
		},
	))

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})
	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"write_file","arguments":["invalid","shape"]}}`
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request = request.WithContext(scopeChallengeContextWithScopes(request.Context(), []string{"admin:org"}))
	response := httptest.NewRecorder()

	WithMCPParse()(WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next)).ServeHTTP(response, request)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Zero(t, callbackCalls)
}

func TestWithScopeChallengeFallbackFastPathPreservesBody(t *testing.T) {
	callbackCalls := 0
	setScopeTestMap(t, scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.Workflow},
		func([]string) bool { return true },
		func(map[string]any, []string) []string {
			callbackCalls++
			return []string{"unexpected"}
		},
	))

	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"write_file","arguments":["invalid","shape"]}}`
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedBody, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, body, string(receivedBody))
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request = request.WithContext(scopeChallengeContextWithScopes(request.Context(), []string{"repo", "workflow"}))
	response := httptest.NewRecorder()

	WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next).ServeHTTP(response, request)

	assert.Equal(t, http.StatusNoContent, response.Code)
	assert.Zero(t, callbackCalls)
}

func TestWithScopeChallengeUnderScopedInvalidArgumentsReachHandler(t *testing.T) {
	callbackCalls := 0
	setScopeTestMap(t, scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.Workflow},
		func([]string) bool { return true },
		func(map[string]any, []string) []string {
			callbackCalls++
			return []string{"unexpected"}
		},
	))

	body := `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"write_file","arguments":["invalid","shape"]}}`
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		receivedBody, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		assert.Equal(t, body, string(receivedBody))
		w.WriteHeader(http.StatusUnprocessableEntity)
	})
	request := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	request = request.WithContext(scopeChallengeContext(request.Context()))
	response := httptest.NewRecorder()

	WithMCPParse()(WithScopeChallenge(&oauth.Config{}, &mockScopeFetcher{})(next)).ServeHTTP(response, request)

	assert.True(t, nextCalled)
	assert.Equal(t, http.StatusUnprocessableEntity, response.Code)
	assert.Zero(t, callbackCalls)
}

func setDynamicScopeTestMap(t *testing.T) {
	t.Helper()
	setScopeTestMap(t, scopes.DynamicChallenge(
		[]scopes.Scope{scopes.Repo, scopes.Workflow},
		func([]string) bool { return true },
		func(arguments map[string]any, activeScopes []string) []string {
			if path, _ := arguments["path"].(string); strings.HasPrefix(path, ".github/workflows/") {
				return scopes.ChallengeAll(activeScopes, scopes.Repo, scopes.Workflow)
			}
			files, _ := arguments["files"].([]any)
			for _, file := range files {
				fileMap, _ := file.(map[string]any)
				if path, _ := fileMap["path"].(string); strings.HasPrefix(path, ".github/workflows/") {
					return scopes.ChallengeAll(activeScopes, scopes.Repo, scopes.Workflow)
				}
			}
			return scopes.ChallengeAll(activeScopes, scopes.Repo)
		},
	))
}

func setScopeTestMap(t *testing.T, access inventory.ScopeAccess) {
	t.Helper()
	scopes.SetGlobalToolScopeMap(scopes.ToolScopeMap{"write_file": access})
	t.Cleanup(func() {
		scopes.SetGlobalToolScopeMap(nil)
	})
}

func scopeChallengeContext(ctx context.Context) context.Context {
	return scopeChallengeContextWithScopes(ctx, []string{"repo"})
}

func scopeChallengeContextWithScopes(ctx context.Context, activeScopes []string) context.Context {
	ctx = ghcontext.WithTokenInfo(ctx, &ghcontext.TokenInfo{ //nolint:gosec // G101: "oauth-token" is a synthetic context fixture, not an authentication credential.
		Token:     "oauth-token",
		TokenType: utils.TokenTypeOAuthAccessToken,
	})
	ctx = ghcontext.WithTokenScopes(ctx, activeScopes)
	return ctx
}
