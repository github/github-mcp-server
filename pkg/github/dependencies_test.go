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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestDepsAPIHost struct {
	url *url.URL
}

func (h requestDepsAPIHost) BaseRESTURL(context.Context) (*url.URL, error) { return h.url, nil }
func (h requestDepsAPIHost) GraphqlURL(context.Context) (*url.URL, error)  { return h.url, nil }
func (h requestDepsAPIHost) UploadURL(context.Context) (*url.URL, error)   { return h.url, nil }
func (h requestDepsAPIHost) RawURL(context.Context) (*url.URL, error)      { return h.url, nil }
func (h requestDepsAPIHost) AuthorizationServerURL(context.Context) (*url.URL, error) {
	return h.url, nil
}

func testExporters() observability.Exporters {
	obs, _ := observability.NewExporters(slog.New(slog.DiscardHandler), metrics.NewNoopMetrics())
	return obs
}

func TestRequestDepsGetClientSetsAPIVersion(t *testing.T) {
	t.Parallel()

	var gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get(headers.GitHubAPIVersionHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	apiHost := requestDepsAPIHost{url: serverURL}
	deps := github.NewRequestDeps(apiHost, "test", false, nil, nil, 0, nil, testExporters())
	ctx := ghcontext.WithTokenInfo(context.Background(), &ghcontext.TokenInfo{Token: "test-token"})
	client, err := deps.GetClient(ctx)
	require.NoError(t, err)

	req, err := client.NewRequest(ctx, http.MethodGet, "rate_limit", nil)
	require.NoError(t, err)
	resp, err := client.Do(req, nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, headers.GitHubAPIVersion, gotVersion)
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
