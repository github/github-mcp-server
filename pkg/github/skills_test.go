package github

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/skills"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSkillsSource(t *testing.T) *skills.Registry {
	t.Helper()
	skill, err := skills.NewSkill("example.com/reviewing", []skills.File{
		{Path: "SKILL.md", Content: []byte("---\nname: reviewing\ndescription: Review a change.\n---\n\nRead the diff.\n")},
		{Path: "references/checks.md", Content: []byte("Read the checks.\n")},
	})
	require.NoError(t, err)
	skill.RequiredTools = [][]string{{"get_me"}}
	skill.OptionalTools = []string{"get_teams"}
	registry, err := skills.NewRegistry(skill)
	require.NoError(t, err)
	return registry
}

func connectSkillsClient(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Close() })
	client := mcp.NewClient(&mcp.Implementation{Name: "skills-test", Version: "1"}, nil)
	require.NoError(t, mcp.AddSendingCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](client, skills.MethodListSkills))
	require.NoError(t, mcp.AddSendingCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](client, skills.MethodGetSkill))
	require.NoError(t, mcp.AddSendingCustomMethod[*skills.ReadDirectoryParams, *skills.ReadDirectoryResult](client, skills.MethodReadDirectory))
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestSkillsFeatureAndInstructions(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name     string
		flags    []string
		insiders bool
		enabled  bool
	}{
		{name: "default"},
		{name: "explicit", flags: []string{FeatureFlagSkillsExtension}, enabled: true},
		{name: "insiders", insiders: true, enabled: true},
		{name: "unrelated", flags: []string{FeatureFlagCSVOutput}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			flags := ResolveFeatureFlags(tt.flags, tt.insiders)
			checker := func(_ context.Context, flag string) (bool, error) { return flags[flag], nil }
			cfg := MCPServerConfig{
				Version: "test", Translator: translations.NullTranslationHelper,
				SkillsSource:  testSkillsSource(t),
				ServerOptions: []MCPServerOption{func(opts *mcp.ServerOptions) { opts.Instructions = "custom instructions" }},
			}
			deps := NewBaseDeps(nil, nil, nil, nil, cfg.Translator, FeatureFlags{}, 0, checker, stubExporters())
			inv, err := NewInventory(cfg.Translator).WithToolsets([]string{"context"}).
				WithFeatureChecker(checker).WithServerInstructions().Build()
			require.NoError(t, err)
			server, err := NewMCPServer(context.Background(), &cfg, deps, inv)
			require.NoError(t, err)
			session := connectSkillsClient(t, server)
			init := session.InitializeResult()
			_, declared := init.Capabilities.Extensions[skills.ExtensionID]
			assert.Equal(t, tt.enabled, declared)
			raw, err := json.Marshal(init)
			require.NoError(t, err)
			if tt.enabled {
				assert.Empty(t, init.Instructions)
				assert.NotContains(t, string(raw), `"instructions"`)
			} else {
				assert.Equal(t, "custom instructions", init.Instructions)
			}
			listed, err := mcp.CallCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](
				context.Background(), session, skills.MethodListSkills, &skills.ListSkillsParams{})
			if !tt.enabled {
				var rpcErr *jsonrpc.Error
				require.ErrorAs(t, err, &rpcErr)
				assert.Equal(t, int64(jsonrpc.CodeMethodNotFound), rpcErr.Code)
				return
			}
			require.NoError(t, err)
			require.Len(t, listed.Skills, 1)
			assert.Equal(t, "reviewing", listed.Skills[0].Frontmatter.Name())
		})
	}
}

func TestSkillsAvailabilityAcrossRequestNarrowing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	source := testSkillsSource(t)
	for _, available := range []bool{true, false} {
		for _, method := range []string{"initialize", "server/discover", "skills/list", "skills/get", "resources/read", "resources/directory/read"} {
			t.Run(method+map[bool]string{true: "/available", false: "/withheld"}[available], func(t *testing.T) {
				t.Parallel()
				checker := featureCheckerFor(FeatureFlagSkillsExtension)
				cfg := MCPServerConfig{Version: "test", Translator: translations.NullTranslationHelper, SkillsSource: source}
				deps := NewBaseDeps(nil, nil, nil, nil, cfg.Translator, FeatureFlags{}, 0, checker, stubExporters())
				builder := NewInventory(cfg.Translator).WithToolsets([]string{"context"}).WithFeatureChecker(checker)
				if !available {
					builder.WithExcludeTools([]string{"get_me"})
				}
				inv, err := builder.Build()
				require.NoError(t, err)
				server, err := NewMCPServer(ctx, &cfg, deps, inv.ForMCPRequest(method, ""))
				require.NoError(t, err)
				session := connectSkillsClient(t, server)
				listed, err := mcp.CallCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](
					ctx, session, skills.MethodListSkills, &skills.ListSkillsParams{})
				require.NoError(t, err)
				assert.Len(t, listed.Skills, map[bool]int{true: 1, false: 0}[available])
				uri := "skill://example.com/reviewing/SKILL.md"
				got, getErr := mcp.CallCustomMethod[*skills.GetSkillParams, *skills.GetSkillResult](
					ctx, session, skills.MethodGetSkill, &skills.GetSkillParams{URI: uri})
				dir, dirErr := mcp.CallCustomMethod[*skills.ReadDirectoryParams, *skills.ReadDirectoryResult](
					ctx, session, skills.MethodReadDirectory, &skills.ReadDirectoryParams{URI: "skill://example.com/reviewing"})
				read, readErr := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
				_, supportingErr := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: "skill://example.com/reviewing/references/checks.md"})
				if !available {
					require.Error(t, getErr)
					require.Error(t, dirErr)
					require.Error(t, readErr)
					require.Error(t, supportingErr)
					return
				}
				require.NoError(t, getErr)
				require.NoError(t, dirErr)
				assert.Len(t, dir.Resources, 2)
				require.NoError(t, readErr)
				require.NoError(t, supportingErr)
				assert.Equal(t, listed.Skills[0], got.Skill)
				require.Len(t, read.Contents, 1)
				assert.NoError(t, skills.Verify([]byte(read.Contents[0].Text), got.Skill.Resources[0]))
				raw, err := json.Marshal(got)
				require.NoError(t, err)
				assert.NotContains(t, string(raw), "allowed-tools")
				assert.NotContains(t, string(raw), "RequiredTools")
			})
		}
	}
	assert.Equal(t, 1, source.Len(), "per-request filtering must not mutate the source")
}

func TestBundledSkillsAreTheDefaultSource(t *testing.T) {
	t.Parallel()
	checker := featureCheckerFor(FeatureFlagSkillsExtension)
	cfg := MCPServerConfig{Version: "test", Translator: translations.NullTranslationHelper}
	deps := NewBaseDeps(nil, nil, nil, nil, cfg.Translator, FeatureFlags{}, 0, checker, stubExporters())
	inv, err := NewInventory(cfg.Translator).WithToolsets([]string{"context"}).WithFeatureChecker(checker).Build()
	require.NoError(t, err)
	server, err := NewMCPServer(context.Background(), &cfg, deps, inv)
	require.NoError(t, err)
	session := connectSkillsClient(t, server)
	listed, err := mcp.CallCustomMethod[*skills.ListSkillsParams, *skills.ListSkillsResult](
		context.Background(), session, skills.MethodListSkills, &skills.ListSkillsParams{})
	require.NoError(t, err)
	require.Len(t, listed.Skills, 1)
	assert.Equal(t, "account-and-team-context", listed.Skills[0].Frontmatter.Name())
}

func TestSkillsModernDiscoveryAndCacheEnvelopes(t *testing.T) {
	t.Parallel()
	checker := featureCheckerFor(FeatureFlagSkillsExtension)
	cfg := MCPServerConfig{Version: "test", Translator: translations.NullTranslationHelper, SkillsSource: testSkillsSource(t)}
	deps := NewBaseDeps(nil, nil, nil, nil, cfg.Translator, FeatureFlags{}, 0, checker, stubExporters())
	inv, err := NewInventory(cfg.Translator).WithToolsets([]string{"context"}).
		WithFeatureChecker(checker).WithServerInstructions().Build()
	require.NoError(t, err)
	server, err := NewMCPServer(context.Background(), &cfg, deps, inv.ForMCPRequest("server/discover", ""))
	require.NoError(t, err)
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	for i, method := range []string{"server/discover", "skills/list", "skills/get", "resources/directory/read"} {
		params := map[string]any{
			"_meta": map[string]any{
				mcp.MetaKeyProtocolVersion:    "2026-07-28",
				mcp.MetaKeyClientInfo:         map[string]any{"name": "skills-test", "version": "1"},
				mcp.MetaKeyClientCapabilities: map[string]any{},
			},
		}
		switch method {
		case "skills/get":
			params["uri"] = "skill://example.com/reviewing/SKILL.md"
		case "resources/directory/read":
			params["uri"] = "skill://example.com/reviewing"
		}
		body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": i + 1, "method": method, "params": params})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
		req.Header.Set("Mcp-Method", method)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var reply struct {
			Result map[string]any `json:"result"`
			Error  any            `json:"error"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &reply))
		require.Nil(t, reply.Error)
		if method == "server/discover" {
			assert.NotContains(t, reply.Result, "instructions")
		} else {
			assert.Equal(t, "complete", reply.Result["resultType"])
			assert.Equal(t, float64(skills.ListTTL), reply.Result["ttlMs"])
			assert.Equal(t, "private", reply.Result["cacheScope"])
		}
	}
}

func TestSkillsDisabledPreservesInventoryInstructions(t *testing.T) {
	t.Parallel()
	cfg := MCPServerConfig{Version: "test", Translator: translations.NullTranslationHelper}
	deps := NewBaseDeps(nil, nil, nil, nil, cfg.Translator, FeatureFlags{}, 0, nil, stubExporters())
	inv, err := NewInventory(cfg.Translator).WithToolsets([]string{"context"}).WithServerInstructions().Build()
	require.NoError(t, err)
	require.NotEmpty(t, inv.Instructions())
	server, err := NewMCPServer(context.Background(), &cfg, deps, inv)
	require.NoError(t, err)
	session := connectSkillsClient(t, server)
	assert.Equal(t, inv.Instructions(), session.InitializeResult().Instructions)
}
