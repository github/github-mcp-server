package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/lockdown"
	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/github/github-mcp-server/pkg/observability/metrics"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubDeps is 一个test helper that implements ToolDependencies with configurable behavior.
// Use this when you need to test 错误 路径s 或when you need closure-based 客户端 creation.
type stubDeps struct {
	clientFn    func(context.Context) (*gogithub.Client, error)
	gqlClientFn func(context.Context) (*githubv4.Client, error)
	rawClientFn func(context.Context) (*raw.Client, error)

	repoAccessCache   *lockdown.RepoAccessCache
	t                 translations.TranslationHelperFunc
	flags             FeatureFlags
	contentWindowSize int
	obsv              observability.Exporters
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
func (s stubDeps) Logger(_ context.Context) *slog.Logger {
	return s.obsv.Logger()
}
func (s stubDeps) Metrics(ctx context.Context) metrics.Metrics {
	return s.obsv.Metrics(ctx)
}

// Helper 函数s to 创建 stub 客户端 函数s f或错误 testing

// stubExporters 返回 一个discard-logger + noop-metrics Exporters f或tests.
func stubExporters() observability.Exporters {
	obs, _ := observability.NewExporters(slog.New(slog.DiscardHandler), metrics.NewNoopMetrics())
	return obs
}

func stubClientFnFromHTTP(t *testing.T, httpClient *http.Client) func(context.Context) (*gogithub.Client, error) {
	t.Helper()
	return func(_ context.Context) (*gogithub.Client, error) {
		return mustNewGHClient(t, httpClient), nil
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

func stubRepoAccessCache(restClient *gogithub.Client, ttl time.Duration) *lockdown.RepoAccessCache {
	cacheName := fmt.Sprintf("repo-access-cache-test-%d", time.Now().UnixNano())
	return lockdown.NewRepoAccessCache(
		githubv4.NewClient(newRepoAccessHTTPClient()),
		restClient,
		lockdown.WithTTL(ttl),
		lockdown.WithCacheName(cacheName),
	)
}

func mockRESTPermissionServer(t *testing.T, defaultPerm string, overrides map[string]string) *gogithub.Client {
	t.Helper()
	return mustNewGHClient(t, MockHTTPClientWithHandler(func(w http.ResponseWriter, r *http.Request) {
		perm := defaultPerm
		for user, p := range overrides {
			if strings.Contains(r.URL.Path, "/collaborators/"+user+"/") {
				perm = p
				break
			}
		}
		resp := gogithub.RepositoryPermissionLevel{
			Permission: gogithub.Ptr(perm),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func stubFeatureFlags(enabledFlags map[string]bool) FeatureFlags {
	return FeatureFlags{
		LockdownMode: enabledFlags["lockdown-mode"],
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

// TestNewMCPServer_CreatesSuccessfully verifies that 服务器 可以 创建d
// 使用deps injection middleware properly configured.
func TestNewMCPServer_CreatesSuccessfully(t *testing.T) {
	t.Parallel()

	// Create 一个minimal 服务器 configuration
	cfg := MCPServerConfig{
		Version:           "test",
		Host:              "", // defaults to github.com
		Token:             "test-token",
		EnabledToolsets:   []string{"context"},
		ReadOnly:          false,
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 5000,
		LockdownMode:      false,
	}

	deps := stubDeps{obsv: stubExporters()}

	// Build inventory
	inv, err := NewInventory(cfg.Translator).
		WithDeprecatedAliases(DeprecatedToolAliases).
		WithToolsets(cfg.EnabledToolsets).
		Build()

	require.NoError(t, err, "expected inventory build to succeed")

	// Create 服务器
	server, err := NewMCPServer(context.Background(), &cfg, deps, inv)
	require.NoError(t, err, "expected server creation to succeed")
	require.NotNil(t, server, "expected server to be non-nil")

	// fact that 服务器 was 创建d 成功fully indicates that:
	// 1. deps injection middleware is properly added
	// 2. Tools 可以 registered without panicking
	//
	// 如果middleware wasn't properly added, 工具 调用 would panic with
	// "ToolDependencies 不found in 上下文" when executed.
	//
	// actual middleware 函数ality 和工具 execution with ContextWithDeps
	// is al读取y tested in pkg/github/*_test.go.
}

// advertisedServerCapabilities connects 一个in-memory 客户端 到given 服务器
// 和返回 能力 服务器 advertised during initialization.
func advertisedServerCapabilities(t *testing.T, server *mcp.Server) *mcp.ServerCapabilities {
	t.Helper()

	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err, "expected server to connect")
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err, "expected client to connect")
	t.Cleanup(func() { _ = clientSession.Close() })

	result := clientSession.InitializeResult()
	require.NotNil(t, result, "expected an initialize result")
	return result.Capabilities
}

// TestNewMCPServer_AdvertisedCapabilities locks 在能力 contract set by
// NewMCPServer: 工具, 提示, 和资源 are advertised without 列出-changed
// notifications (the 服务器 has 一个static item set 和绝不emits 列出_changed),
// deprecated logging 能力 is 不advertised, 以及inferred
// completions 能力 is preserved. 此is asserted f或both stdio 路径
// (full inventory, items present) 以及HTTP 路径 (inventory emptied f或the
// discovery/initialize 请求), which share 相同 NewMCPServer entry point.
func TestNewMCPServer_AdvertisedCapabilities(t *testing.T) {
	t.Parallel()

	cfg := MCPServerConfig{
		Version:           "test",
		Token:             "test-token",
		EnabledToolsets:   []string{"context"},
		Translator:        translations.NullTranslationHelper,
		ContentWindowSize: 5000,
	}

	deps := stubDeps{obsv: stubExporters()}

	fullInventory, err := NewInventory(cfg.Translator).
		WithDeprecatedAliases(DeprecatedToolAliases).
		WithToolsets(cfg.EnabledToolsets).
		Build()
	require.NoError(t, err, "expected inventory build to succeed")

	tests := []struct {
		name string
		inv  *inventory.Inventory
	}{
		{
			name: "stdio path with registered items",
			inv:  fullInventory,
		},
		{
			// HTTP 处理器 registers 仅items relevant to 一个请求;
			// f或initialize/discover that is nothing, so 能力 must come
			// 来自explicit declaration rather than being inferred from items.
			name: "http path with no registered items for discovery",
			inv:  fullInventory.ForMCPRequest(inventory.MCPMethodDiscover, ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server, err := NewMCPServer(context.Background(), &cfg, deps, tt.inv)
			require.NoError(t, err, "expected server creation to succeed")

			caps := advertisedServerCapabilities(t, server)

			require.NotNil(t, caps.Tools, "tools capability should be advertised")
			assert.False(t, caps.Tools.ListChanged, "tools list-changed must not be advertised")

			require.NotNil(t, caps.Prompts, "prompts capability should be advertised")
			assert.False(t, caps.Prompts.ListChanged, "prompts list-changed must not be advertised")

			require.NotNil(t, caps.Resources, "resources capability should be advertised")
			assert.False(t, caps.Resources.ListChanged, "resources list-changed must not be advertised")
			assert.False(t, caps.Resources.Subscribe, "resources subscribe must not be advertised")

			assert.NotNil(t, caps.Completions, "completions capability should be preserved")
			// Intentionally asserting deprecated logging 能力 is absent.
			assert.Nil(t, caps.Logging, "deprecated logging capability should not be advertised") //nolint:static检查 // SA1019: verifying deprecated 能力 is 不advertised
		})
	}
}

// TestNewServer_NameAndTitleViaTranslation verifies that 服务器 name 和title
// 可以 overridden via translation helper (GITHUB_MCP_SERVER_NAME /
// GITHUB_MCP_SERVER_TITLE env vars 或github-mcp-服务器-config.json) and
// f所有back to sensible defaults when 不overridden.
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

			// Connect 一个客户端 to retrieve initialize 结果 和verify ServerInfo.
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

// TestResolveEnabledToolsets verifies 工具集 resolution logic.
func TestResolveEnabledToolsets(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		cfg            MCPServerConfig
		expectedResult []string
	}{
		{
			name: "nil toolsets and no tools - use defaults",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				EnabledTools:    nil,
			},
			expectedResult: nil, // nil means "use defaults"
		},
		{
			name: "explicit toolsets",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{"repos", "issues"},
			},
			expectedResult: []string{"repos", "issues"},
		},
		{
			name: "empty toolsets - disable all",
			cfg: MCPServerConfig{
				EnabledToolsets: []string{},
			},
			expectedResult: []string{},
		},
		{
			name: "specific tools without toolsets - no default toolsets",
			cfg: MCPServerConfig{
				EnabledToolsets: nil,
				EnabledTools:    []string{"get_me"},
			},
			expectedResult: []string{}, // 空 slice when 工具 specified 但no 工具集s
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := ResolvedEnabledToolsets(tc.cfg.EnabledToolsets, tc.cfg.EnabledTools)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestCompletionsHandler_RejectsMissingRef(t *testing.T) {
	getClient := func(_ context.Context) (*gogithub.Client, error) {
		return &gogithub.Client{}, nil
	}
	handler := CompletionsHandler(getClient)

	tests := []struct {
		name string
		req  *mcp.CompleteRequest
	}{
		{name: "nil request", req: nil},
		{name: "nil params", req: &mcp.CompleteRequest{}},
		{name: "nil ref", req: &mcp.CompleteRequest{Params: &mcp.CompleteParams{}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := handler(context.Background(), tc.req)
			require.Error(t, err)
			assert.Nil(t, result)
			assert.Contains(t, err.Error(), "missing required parameter: ref")
		})
	}
}
