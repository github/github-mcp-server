package github_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testExporters() observability.Exporters {
	obs, _ := observability.NewExporters(slog.New(slog.DiscardHandler), metrics.NewNoopMetrics())
	return obs
}

type requestDepsAPIHostResolver struct {
	endpoint *url.URL
}

func newRequestDepsAPIHostResolver(t *testing.T, endpoint string) requestDepsAPIHostResolver {
	t.Helper()

	u, err := url.Parse(endpoint)
	require.NoError(t, err)
	return requestDepsAPIHostResolver{endpoint: u}
}

func (r requestDepsAPIHostResolver) BaseRESTURL(context.Context) (*url.URL, error) {
	return r.endpoint, nil
}

func (r requestDepsAPIHostResolver) GraphqlURL(context.Context) (*url.URL, error) {
	return r.endpoint, nil
}

func (r requestDepsAPIHostResolver) UploadURL(context.Context) (*url.URL, error) {
	return r.endpoint, nil
}

func (r requestDepsAPIHostResolver) RawURL(context.Context) (*url.URL, error) {
	return r.endpoint, nil
}

func (r requestDepsAPIHostResolver) AuthorizationServerURL(context.Context) (*url.URL, error) {
	return r.endpoint, nil
}

func TestRequestDepsScopesTokensToConfiguredHosts(t *testing.T) {
	t.Parallel()

	var foreignAuth string
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		foreignAuth = r.Header.Get(headers.AuthorizationHeader)
		w.Header().Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
		_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"octocat"}}}`))
	}))
	defer foreign.Close()

	var sourceAuth string
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sourceAuth = r.Header.Get(headers.AuthorizationHeader)
		http.Redirect(w, r, foreign.URL, http.StatusFound)
	}))
	defer source.Close()

	deps := github.NewRequestDeps(
		newRequestDepsAPIHostResolver(t, source.URL),
		"test",
		false,
		nil,
		translations.NullTranslationHelper,
		0,
		nil,
		testExporters(),
	)
	ctx := ghcontext.WithTokenInfo(context.Background(), &ghcontext.TokenInfo{Token: "request-token"})

	sourceAuth = ""
	foreignAuth = ""
	restClient, err := deps.GetClient(ctx)
	require.NoError(t, err)
	resp, err := restClient.Client().Get(source.URL + "/rest")
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, sourceAuth, "REST request must authenticate to the configured host")
	assert.Empty(t, foreignAuth, "REST redirect must not authenticate to a foreign host")

	sourceAuth = ""
	foreignAuth = ""
	rawClient, err := deps.GetRawClient(ctx)
	require.NoError(t, err)
	resp, err = rawClient.GetRawContent(ctx, "owner", "repo", "file", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEmpty(t, sourceAuth, "raw request must authenticate to the configured host")
	assert.Empty(t, foreignAuth, "raw redirect must not authenticate to a foreign host")

	sourceAuth = ""
	foreignAuth = ""
	gqlClient, err := deps.GetGQLClient(ctx)
	require.NoError(t, err)
	var query struct {
		Viewer struct {
			Login githubv4.String
		}
	}
	err = gqlClient.Query(ctx, &query, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, sourceAuth, "GraphQL request must authenticate to the configured host")
	assert.Empty(t, foreignAuth, "GraphQL redirect must not authenticate to a foreign host")
}

// Regression test for #3107: RequestDeps is built once at startup and shared,
// so identity scoping has to happen per request in GetRepoAccessCache.
func TestGetRepoAccessCacheIsolatesTrustDecisionsPerIdentity(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var gqlCalls, restCalls int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headers.ContentTypeHeader, headers.ContentTypeJSON)
		mu.Lock()
		defer mu.Unlock()
		if strings.Contains(r.URL.Path, "/collaborators/") {
			restCalls++
			_, _ = w.Write([]byte(`{"permission":"write"}`))
			return
		}
		gqlCalls++
		_, _ = w.Write([]byte(`{"data":{"viewer":{"login":"someone"},"repository":{"isPrivate":false}}}`))
	}))
	defer server.Close()

	callCounts := func() (int, int) {
		mu.Lock()
		defer mu.Unlock()
		return gqlCalls, restCalls
	}

	// Built as pkg/http/server.go does: no per-identity options.
	deps := github.NewRequestDeps(
		newRequestDepsAPIHostResolver(t, server.URL),
		"test",
		true, // lockdownMode
		nil,  // RepoAccessOpts
		translations.NullTranslationHelper,
		0,
		nil,
		testExporters(),
	)

	ctxAlice := ghcontext.WithTokenInfo(context.Background(), &ghcontext.TokenInfo{Token: "token-for-alice"})
	cacheAlice, err := deps.GetRepoAccessCache(ctxAlice)
	require.NoError(t, err)
	require.NotNil(t, cacheAlice)

	_, err = cacheAlice.IsSafeContent(ctxAlice, "mallory", "owner", "repo")
	require.NoError(t, err)

	gqlN, restN := callCounts()
	require.Equal(t, 1, gqlN)
	require.Equal(t, 1, restN)

	ctxBob := ghcontext.WithTokenInfo(context.Background(), &ghcontext.TokenInfo{Token: "token-for-bob"})
	cacheBob, err := deps.GetRepoAccessCache(ctxBob)
	require.NoError(t, err)
	require.NotNil(t, cacheBob)

	_, err = cacheBob.IsSafeContent(ctxBob, "mallory", "owner", "repo")
	require.NoError(t, err)

	gqlN, restN = callCounts()
	require.Equal(t, 2, gqlN, "a different identity's request must not be served from another identity's cached trust decision")
	require.Equal(t, 2, restN, "a different identity's request must not be served from another identity's cached trust decision")

	cacheAliceAgain, err := deps.GetRepoAccessCache(ctxAlice)
	require.NoError(t, err)
	_, err = cacheAliceAgain.IsSafeContent(ctxAlice, "mallory", "owner", "repo")
	require.NoError(t, err)

	gqlN, restN = callCounts()
	require.Equal(t, 2, gqlN, "repeated requests from the same identity should reuse the warm cache")
	require.Equal(t, 2, restN, "repeated requests from the same identity should reuse the warm cache")
}

func TestIsFeatureEnabled_WithEnabledFlag(t *testing.T) {
	t.Parallel()

	// Create a feature checker that returns true for "test_flag"
	checker := func(_ context.Context, flagName string) (bool, error) {
		return flagName == "test_flag", nil
	}

	// Create deps with the checker using NewBaseDeps
	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // contentWindowSize
		checker, // featureChecker
		testExporters(),
	)

	// Test enabled flag
	result := deps.IsFeatureEnabled(context.Background(), "test_flag")
	assert.True(t, result, "Expected test_flag to be enabled")

	// Test disabled flag
	result = deps.IsFeatureEnabled(context.Background(), "other_flag")
	assert.False(t, result, "Expected other_flag to be disabled")
}

func TestIsFeatureEnabled_WithoutChecker(t *testing.T) {
	t.Parallel()

	// Create deps without feature checker (nil)
	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,   // contentWindowSize
		nil, // featureChecker (nil)
		testExporters(),
	)

	// Should return false when checker is nil
	result := deps.IsFeatureEnabled(context.Background(), "any_flag")
	assert.False(t, result, "Expected false when checker is nil")
}

func TestIsFeatureEnabled_EmptyFlagName(t *testing.T) {
	t.Parallel()

	// Create a feature checker
	checker := func(_ context.Context, _ string) (bool, error) {
		return true, nil
	}

	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // contentWindowSize
		checker, // featureChecker
		testExporters(),
	)

	// Should return false for empty flag name
	result := deps.IsFeatureEnabled(context.Background(), "")
	assert.False(t, result, "Expected false for empty flag name")
}

// TestRequestDepsLockdownModeIsUpperBound verifies the X-MCP-Lockdown header
// can only enable lockdown, never disable the operator's server-side setting.
func TestRequestDepsLockdownModeIsUpperBound(t *testing.T) {
	t.Parallel()

	resolver := newRequestDepsAPIHostResolver(t, "https://example.com")

	newDeps := func(serverLockdown bool) *github.RequestDeps {
		return github.NewRequestDeps(
			resolver,
			"test",
			serverLockdown,
			nil,
			translations.NullTranslationHelper,
			0,
			nil,
			testExporters(),
		)
	}

	tokenCtx := func(requestLockdown bool) context.Context {
		ctx := ghcontext.WithTokenInfo(context.Background(), &ghcontext.TokenInfo{Token: "request-token"})
		if requestLockdown {
			ctx = ghcontext.WithLockdownMode(ctx, true)
		}
		return ctx
	}

	tests := []struct {
		name             string
		serverLockdown   bool
		requestLockdown  bool
		wantLockdownMode bool
	}{
		{
			name:             "neither server nor request enable lockdown",
			serverLockdown:   false,
			requestLockdown:  false,
			wantLockdownMode: false,
		},
		{
			name:             "server-only lockdown is enforced without a request header",
			serverLockdown:   true,
			requestLockdown:  false,
			wantLockdownMode: true,
		},
		{
			name:             "request-only lockdown can enable it when the server has not",
			serverLockdown:   false,
			requestLockdown:  true,
			wantLockdownMode: true,
		},
		{
			name:             "server and request both enabling lockdown stays enabled",
			serverLockdown:   true,
			requestLockdown:  true,
			wantLockdownMode: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			deps := newDeps(tt.serverLockdown)
			ctx := tokenCtx(tt.requestLockdown)

			flags := deps.GetFlags(ctx)
			assert.Equal(t, tt.wantLockdownMode, flags.LockdownMode, "GetFlags().LockdownMode")

			cache, err := deps.GetRepoAccessCache(ctx)
			require.NoError(t, err)
			if tt.wantLockdownMode {
				assert.NotNil(t, cache, "expected a repo access cache to be built when lockdown mode is effectively enabled")
			} else {
				assert.Nil(t, cache, "expected no repo access cache when lockdown mode is effectively disabled")
			}
		})
	}
}

// TestRequestDepsLockdownModeCannotBeDisabledByOmittingHeader is a regression
// test for #3104: omitting the X-MCP-Lockdown header must not disable
// server-enabled lockdown mode.
func TestRequestDepsLockdownModeCannotBeDisabledByOmittingHeader(t *testing.T) {
	t.Parallel()

	resolver := newRequestDepsAPIHostResolver(t, "https://example.com")
	deps := github.NewRequestDeps(
		resolver,
		"test",
		true, // server-enabled lockdown
		nil,
		translations.NullTranslationHelper,
		0,
		nil,
		testExporters(),
	)

	// No X-MCP-Lockdown header sent.
	ctx := ghcontext.WithTokenInfo(context.Background(), &ghcontext.TokenInfo{Token: "request-token"})

	flags := deps.GetFlags(ctx)
	assert.True(t, flags.LockdownMode, "server-enabled lockdown mode must remain enabled when a request omits the lockdown header")

	cache, err := deps.GetRepoAccessCache(ctx)
	require.NoError(t, err)
	assert.NotNil(t, cache, "repo access cache must still be built so server-enabled lockdown mode can be enforced")
}

func TestIsFeatureEnabled_CheckerError(t *testing.T) {
	t.Parallel()

	// Create a feature checker that returns an error
	checker := func(_ context.Context, _ string) (bool, error) {
		return false, errors.New("checker error")
	}

	deps := github.NewBaseDeps(
		nil, // client
		nil, // gqlClient
		nil, // rawClient
		nil, // repoAccessCache
		translations.NullTranslationHelper,
		github.FeatureFlags{},
		0,       // contentWindowSize
		checker, // featureChecker
		testExporters(),
	)

	// Should return false and log error (not crash)
	result := deps.IsFeatureEnabled(context.Background(), "error_flag")
	assert.False(t, result, "Expected false when checker returns error")
}
