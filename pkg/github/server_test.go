package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/shurcooL/githubv4"
)

// stubDeps is a test helper that implements ToolDependencies with configurable behavior.
// Use this when you need to test error paths or when you need closure-based client creation.
type stubDeps struct {
	clientFn    func(context.Context) (*github.Client, error)
	gqlClientFn func(context.Context) (*githubv4.Client, error)
	rawClientFn func(context.Context) (*raw.Client, error)

	repoAccessCache   *lockdown.RepoAccessCache
	t                 translations.TranslationHelperFunc
	flags             FeatureFlags
	contentWindowSize int
}

func (s stubDeps) GetClient(ctx context.Context) (*github.Client, error) {
	if s.clientFn != nil {
		return s.clientFn(ctx)
	}
	return nil, nil
}

func (s stubDeps) GetGQLClient(ctx context.Context) (*githubv4.Client, error) {
	if s.gqlClientFn != nil {
		return s.gqlClientFn(ctx)
	}
	return nil, nil
}

func (s stubDeps) GetRawClient(ctx context.Context) (*raw.Client, error) {
	if s.rawClientFn != nil {
		return s.rawClientFn(ctx)
	}
	return nil, nil
}

func (s stubDeps) GetRepoAccessCache() *lockdown.RepoAccessCache { return s.repoAccessCache }
func (s stubDeps) GetT() translations.TranslationHelperFunc      { return s.t }
func (s stubDeps) GetFlags() FeatureFlags                        { return s.flags }
func (s stubDeps) GetContentWindowSize() int                     { return s.contentWindowSize }

// Helper functions to create stub client functions for error testing
func stubClientFnFromHTTP(httpClient *http.Client) func(context.Context) (*github.Client, error) {
	return func(_ context.Context) (*github.Client, error) {
		return github.NewClient(httpClient), nil
	}
}

func stubClientFnErr(errMsg string) func(context.Context) (*github.Client, error) {
	return func(_ context.Context) (*github.Client, error) {
		return nil, errors.New(errMsg)
	}
}

func stubGQLClientFnErr(errMsg string) func(context.Context) (*githubv4.Client, error) {
	return func(_ context.Context) (*githubv4.Client, error) {
		return nil, errors.New(errMsg)
	}
}

func stubRepoAccessCache(client *githubv4.Client, ttl time.Duration) *lockdown.RepoAccessCache {
	cacheName := fmt.Sprintf("repo-access-cache-test-%d", time.Now().UnixNano())
	return lockdown.GetInstance(client, lockdown.WithTTL(ttl), lockdown.WithCacheName(cacheName))
}

func stubFeatureFlags(enabledFlags map[string]bool) FeatureFlags {
	return FeatureFlags{
		LockdownMode: enabledFlags["lockdown-mode"],
	}
}

func badRequestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		structuredErrorResponse := github.ErrorResponse{
			Message: msg,
		}

		b, err := json.Marshal(structuredErrorResponse)
		if err != nil {
			http.Error(w, "failed to marshal error response", http.StatusInternalServerError)
		}

		http.Error(w, string(b), http.StatusBadRequest)
	}
}
