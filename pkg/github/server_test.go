package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDeps is a test helper that implements ToolDependencies with configurable behavior.
// Use this when you need to test error paths or when you need closure-based client creation.
type stubDeps struct {
	clientFn    func(context.Context) (*gogithub.Client, error)
	gqlClientFn func(context.Context) (*githubv4.Client, error)
	rawClientFn func(context.Context) (*raw.Client, error)

	repoAccessCache   *lockdown.RepoAccessCache
	t                 translations.TranslationHelperFunc
	flags             FeatureFlags
	contentWindowSize int
}

func (s stubDeps) GetClient(ctx context.Context) (*gogithub.Client, error) {
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

func (s stubDeps) GetRepoAccessCache(_ context.Context) (*lockdown.RepoAccessCache, error) {
	return s.repoAccessCache, nil
}
func (s stubDeps) GetT() translations.TranslationHelperFunc          { return s.t }
func (s stubDeps) GetFlags(_ context.Context) FeatureFlags           { return s.flags }
func (s stubDeps) GetContentWindowSize() int                         { return s.contentWindowSize }
func (s stubDeps) IsFeatureEnabled(_ context.Context, _ string) bool { return false }

// Helper functions to create stub client functions for error testing
func stubClientFnFromHTTP(httpClient *http.Client) func(context.Context) (*gogithub.Client, error) {
	return func(_ context.Context) (*gogithub.Client, error) {
		return gogithub.NewClient(httpClient), nil
	}
}

func stubClientFnErr(errMsg string) func(context.Context) (*gogithub.Client, error) {
	return func(_ context.Context) (*gogithub.Client, error) {
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
		InsidersMode: enabledFlags["insiders-mode"],
	}
}

func badRequestHandler(msg string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		structuredErrorResponse := gogithub.ErrorResponse{
			Message: msg,
		}

		b, err := json.Marshal(structuredErrorResponse)
		if err != nil {
			http.Error(w, "failed to marshal error response", http.StatusInternalServerError)
		}

		http.Error(w, string(b), http.StatusBadRequest)
	}
}

// TestNewMCPServer_CreatesSuccessfully verifies that the server can be created
// with the deps injection middleware properly configured.
func TestNewMCPServer_CreatesSuccessfully(t *testing.T) {
	t.Parallel()

	// Create a minimal server configuration
	cfg := MCPServerConfig{
		Version:           "test",
		Host:              "", // defaults to github.com
		Token:             "test-token",
		EnabledToolsets:   []string{"context"},
		ReadOnly:          false,
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 5000,
		LockdownMode:      false,
		InsidersMode:      false,
	}

	deps := stubDeps{}

	// Build inventory
	inv, err := NewInventory(cfg.Translator).
		WithDeprecatedAliases(DeprecatedToolAliases).
		WithToolsets(cfg.EnabledToolsets).
		Build()

	require.NoError(t, err, "expected inventory build to succeed")

	// Create the server
	server, err := NewMCPServer(context.Background(), &cfg, deps, inv)
	require.NoError(t, err, "expected server creation to succeed")
	require.NotNil(t, server, "expected server to be non-nil")

	// The fact that the server was created successfully indicates that:
	// 1. The deps injection middleware is properly added
	// 2. Tools can be registered without panicking
	//
	// If the middleware wasn't properly added, tool calls would panic with
	// "ToolDependencies not found in context" when executed.
	//
	// The actual middleware functionality and tool execution with ContextWithDeps
	// is already tested in pkg/github/*_test.go.
}

// TestNewServer_NameAndTitleViaTranslation verifies that server name and title
// can be overridden via the translation helper (GITHUB_MCP_SERVER_NAME /
// GITHUB_MCP_SERVER_TITLE env vars or github-mcp-server-config.json) and
// fall back to sensible defaults when not overridden.
func TestNewServer_NameAndTitleViaTranslation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		translator    translations.TranslationHelperFunc
		expectedName  string
		expectedTitle string
	}{
		{
			name:          "defaults when using NullTranslationHelper",
			translator:    translations.NullTranslationHelper,
			expectedName:  "github-mcp-server",
			expectedTitle: "GitHub MCP Server",
		},
		{
			name: "custom name and title via translator",
			translator: func(key, defaultValue string) string {
				switch key {
				case "SERVER_NAME":
					return "my-github-server"
				case "SERVER_TITLE":
					return "My GitHub MCP Server"
				default:
					return defaultValue
				}
			},
			expectedName:  "my-github-server",
			expectedTitle: "My GitHub MCP Server",
		},
		{
			name: "custom name only via translator",
			translator: func(key, defaultValue string) string {
				if key == "SERVER_NAME" {
					return "ghes-server"
				}
				return defaultValue
			},
			expectedName:  "ghes-server",
			expectedTitle: "GitHub MCP Server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := NewServer("v1.0.0", tt.translator("SERVER_NAME", "github-mcp-server"), tt.translator("SERVER_TITLE", "GitHub MCP Server"), nil)
			require.NotNil(t, srv)

			// Connect a client to retrieve the initialize result and verify ServerInfo.
			st, ct := mcp.NewInMemoryTransports()
			client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)

			type clientResult struct {
				result *mcp.InitializeResult
				err    error
			}
			clientResultCh := make(chan clientResult, 1)
			go func() {
				cs, err := client.Connect(context.Background(), ct, nil)
				if err != nil {
					clientResultCh <- clientResult{err: err}
					return
				}
				t.Cleanup(func() { _ = cs.Close() })
				clientResultCh <- clientResult{result: cs.InitializeResult()}
			}()

			ss, err := srv.Connect(context.Background(), st, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = ss.Close() })

			got := <-clientResultCh
			require.NoError(t, got.err)
			require.NotNil(t, got.result)
			require.NotNil(t, got.result.ServerInfo)
			assert.Equal(t, tt.expectedName, got.result.ServerInfo.Name)
			assert.Equal(t, tt.expectedTitle, got.result.ServerInfo.Title)
		})
	}
}

// TestResolveEnabledToolsets verifies the toolset resolution logic.
func TestResolveEnabledToolsets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cfg            MCPServerConfig
		expectedResult []string
	}{
		{
			name: "nil toolsets without dynamic mode and no tools - use defaults",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				DynamicToolsets: false,
				EnabledTools:    nil,
			},
			expectedResult: nil, // nil means "use defaults"
		},
		{
			name: "nil toolsets with dynamic mode - start empty",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				DynamicToolsets: true,
				EnabledTools:    nil,
			},
			expectedResult: []string{}, // empty slice means no toolsets
		},
		{
			name: "explicit toolsets",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{"repos", "issues"},
				DynamicToolsets: false,
			},
			expectedResult: []string{"repos", "issues"},
		},
		{
			name: "empty toolsets - disable all",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{},
				DynamicToolsets: false,
			},
			expectedResult: []string{}, // empty slice means no toolsets
		},
		{
			name: "specific tools without toolsets - no default toolsets",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				DynamicToolsets: false,
				EnabledTools:    []string{"get_me"},
			},
			expectedResult: []string{}, // empty slice when tools specified but no toolsets
		},
		{
			name: "dynamic mode with explicit toolsets removes all and default",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{"all", "repos"},
				DynamicToolsets: true,
			},
			expectedResult: []string{"repos"}, // "all" is removed in dynamic mode
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ResolvedEnabledToolsets(tc.cfg.DynamicToolsets, tc.cfg.EnabledToolsets, tc.cfg.EnabledTools)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
