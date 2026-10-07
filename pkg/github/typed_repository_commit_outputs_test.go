package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedRepositoryCommitPaginationDefaults(t *testing.T) {
	for _, protocol := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocol, func(t *testing.T) {
			for _, tool := range []inventory.ServerTool{
				GetCommit(translations.NullTranslationHelper),
				ListCommits(translations.NullTranslationHelper),
			} {
				t.Run(tool.Tool.Name, func(t *testing.T) {
					for _, tc := range []struct {
						name string
						args map[string]any
					}{
						{"omitted", map[string]any{}},
						{"numeric_zero", map[string]any{"page": 0, "perPage": 0}},
						{"string_zero", map[string]any{"page": "0", "perPage": "0"}},
					} {
						t.Run(tc.name, func(t *testing.T) {
							calls := 0
							handler := func(w http.ResponseWriter, r *http.Request) {
								calls++
								assert.Equal(t, "1", r.URL.Query().Get("page"))
								assert.Equal(t, "30", r.URL.Query().Get("per_page"))
								var response any = &github.RepositoryCommit{SHA: new("abc123")}
								if tool.Tool.Name == "list_commits" {
									response = []*github.RepositoryCommit{{SHA: new("abc123")}}
								}
								mockResponse(t, http.StatusOK, response)(w, r)
							}
							deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
								GetReposCommitsByOwnerByRepoByRef: handler,
								GetReposCommitsByOwnerByRepo:      handler,
							}))}
							server := mcp.NewServer(&mcp.Implementation{Name: "commit-pagination-test", Version: "v1"}, nil)
							server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
							tool.RegisterFunc(server, deps)
							session := connectCommentVisibilityClient(t, server, protocol)
							tc.args["owner"] = "owner"
							tc.args["repo"] = "repo"
							if tool.Tool.Name == "get_commit" {
								tc.args["sha"] = "abc123"
							}
							result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
								Name: tool.Tool.Name, Arguments: tc.args,
							})
							require.NoError(t, err)
							require.False(t, result.IsError, "%+v", result)
							assert.Equal(t, 1, calls)
							request := createMCPRequest(tc.args)
							directResult, err := tool.Handler(deps)(ContextWithDeps(context.Background(), deps), &request)
							require.NoError(t, err)
							require.False(t, directResult.IsError, "%+v", directResult)
							assert.Equal(t, 2, calls)
						})
					}
				})
			}
		})
	}
}
func TestTypedRepositoryCommitOutputs(t *testing.T) {
	commit := &github.RepositoryCommit{
		SHA:     new("abc123"),
		HTMLURL: new("https://github.com/owner/repo/commit/abc123"),
		Commit:  &github.Commit{Message: new("A commit")},
	}
	handlers := map[string]http.HandlerFunc{
		GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, &github.Repository{Private: new(false)}),
		GetReposCommitsByOwnerByRepoByRef: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2", r.URL.Query().Get("page"))
			assert.Equal(t, "7", r.URL.Query().Get("per_page"))
			mockResponse(t, http.StatusOK, commit)(w, r)
		},
		GetReposCommitsByOwnerByRepo: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "4", r.URL.Query().Get("page"))
			assert.Equal(t, "9", r.URL.Query().Get("per_page"))
			if r.URL.Query().Get("sha") == "null" {
				mockResponse(t, http.StatusOK, nil)(w, r)
				return
			}
			if r.URL.Query().Get("sha") == "empty" {
				mockResponse(t, http.StatusOK, []*github.RepositoryCommit{})(w, r)
				return
			}
			mockResponse(t, http.StatusOK, []*github.RepositoryCommit{commit})(w, r)
		},
	}
	deps := BaseDeps{
		Client:         mustNewGHClient(t, MockHTTPClientWithHandlers(handlers)),
		featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
	}
	tools := []inventory.ServerTool{
		GetCommit(translations.NullTranslationHelper),
		ListCommits(translations.NullTranslationHelper),
	}
	expectedGet := `{"sha":"abc123","html_url":"https://github.com/owner/repo/commit/abc123","commit":{"message":"A commit"}}`
	expectedList := `[{"sha":"abc123","html_url":"https://github.com/owner/repo/commit/abc123","commit":{"message":"A commit"}}]`

	for _, protocolVersion := range []string{"2025-11-25", inventory.ProtocolVersionMultiRoundTrip} {
		t.Run(protocolVersion, func(t *testing.T) {
			server := mcp.NewServer(&mcp.Implementation{Name: "typed-repository-commit-test", Version: "v1"}, nil)
			server.AddReceivingMiddleware(InjectDepsMiddleware(deps))
			for _, tool := range tools {
				tool.RegisterFunc(server, deps)
			}

			session := connectCommentVisibilityClient(t, server, protocolVersion)
			list, err := session.ListTools(context.Background(), nil)
			require.NoError(t, err)
			require.Len(t, list.Tools, len(tools))
			outputSchemas := make(map[string]*mcp.Tool, len(list.Tools))
			for _, tool := range list.Tools {
				outputSchemas[tool.Name] = tool
				if protocolVersion == "2025-11-25" {
					assert.Nil(t, tool.OutputSchema, "%s must not expose outputSchema to legacy clients", tool.Name)
				} else {
					require.NotNil(t, tool.OutputSchema, "%s must expose its typed output schema", tool.Name)
					require.NoError(t, toolsnaps.Test(tool.Name, tool))
				}
			}

			calls := []struct {
				name string
				args map[string]any
				text string
			}{
				{
					name: "get_commit",
					args: map[string]any{"owner": "owner", "repo": "repo", "sha": "abc123", "detail": "none", "page": "2", "perPage": "7"},
					text: expectedGet,
				},
				{
					name: "list_commits",
					args: map[string]any{"owner": "owner", "repo": "repo", "page": "4", "perPage": "9"},
					text: expectedList,
				},
				{
					name: "list_commits",
					args: map[string]any{"owner": "owner", "repo": "repo", "page": "4", "perPage": "9", "fields": []any{"sha"}},
					text: `[{"sha":"abc123"}]`,
				},
				{
					name: "list_commits",
					args: map[string]any{"owner": "owner", "repo": "repo", "page": "4", "perPage": "9", "fields": []any{"commit"}},
					text: `[{"commit":{"message":"A commit"}}]`,
				},
				{
					name: "list_commits",
					args: map[string]any{"owner": "owner", "repo": "repo", "page": "4", "perPage": "9", "sha": "empty"},
					text: `[]`,
				},
				{
					name: "list_commits",
					args: map[string]any{"owner": "owner", "repo": "repo", "page": "4", "perPage": "9", "sha": "empty", "fields": []any{"sha", "commit"}},
					text: `[]`,
				},
				{
					name: "list_commits",
					args: map[string]any{"owner": "owner", "repo": "repo", "page": "4", "perPage": "9", "sha": "null"},
					text: `[]`,
				},
			}
			for _, call := range calls {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
				require.NoError(t, err, call.name)
				require.False(t, result.IsError, "%s: %s", call.name, result)
				require.Len(t, result.Content, 1, "%s must preserve one legacy text result", call.name)
				assert.NotNil(t, result.Meta["ifc"], "typed and legacy outputs retain IFC labeling")
				text := getTextResult(t, result).Text
				if protocolVersion == "2025-11-25" {
					assert.Equal(t, call.text, text, "%s legacy text is byte-exact", call.name)
					assert.Nil(t, result.StructuredContent)
					continue
				}

				require.NotNil(t, result.StructuredContent, "%s must return structured content", call.name)
				structuredJSON := mustMarshalJSON(t, result.StructuredContent)
				assert.JSONEq(t, call.text, text)
				assert.JSONEq(t, text, structuredJSON, "modern text must serialize the typed DTO")

				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, outputSchemas[call.name].OutputSchema)), &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				var output any
				require.NoError(t, json.Unmarshal([]byte(structuredJSON), &output))
				require.NoError(t, resolved.Validate(output), "%s output must conform to its schema", call.name)
			}
			for _, call := range []mcp.CallToolParams{
				{Name: "get_commit", Arguments: map[string]any{"owner": "owner", "repo": "repo", "sha": "abc123", "detail": "invalid"}},
				{Name: "list_commits", Arguments: map[string]any{"owner": "owner", "repo": "repo", "since": "invalid"}},
			} {
				result, err := session.CallTool(context.Background(), &call)
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Nil(t, result.StructuredContent, "errors must not expose a success DTO")
			}

		})
	}
}
