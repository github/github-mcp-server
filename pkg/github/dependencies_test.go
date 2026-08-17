package github_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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
