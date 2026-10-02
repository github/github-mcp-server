package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypedRepositoryCommitOutputs(t *testing.T) {
	commit := &github.RepositoryCommit{
		SHA:     new("abc123"),
		HTMLURL: new("https://github.com/owner/repo/commit/abc123"),
		Commit:  &github.Commit{Message: new("A commit")},
	}
	handlers := map[string]http.HandlerFunc{
		GetReposCommitsByOwnerByRepoByRef: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "2", r.URL.Query().Get("page"))
			assert.Equal(t, "7", r.URL.Query().Get("per_page"))
			mockResponse(t, http.StatusOK, commit)(w, r)
		},
		GetReposCommitsByOwnerByRepo: func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "4", r.URL.Query().Get("page"))
			assert.Equal(t, "9", r.URL.Query().Get("per_page"))
			mockResponse(t, http.StatusOK, []*github.RepositoryCommit{commit})(w, r)
		},
	}
	deps := BaseDeps{Client: mustNewGHClient(t, MockHTTPClientWithHandlers(handlers))}
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
			}
			for _, call := range calls {
				result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
				require.NoError(t, err, call.name)
				require.False(t, result.IsError, "%s: %s", call.name, result)
				require.Len(t, result.Content, 1, "%s must preserve one legacy text result", call.name)
				assert.Equal(t, call.text, getTextResult(t, result).Text, "%s legacy text is byte-exact", call.name)
				if protocolVersion == "2025-11-25" {
					assert.Nil(t, result.StructuredContent)
					continue
				}

				require.NotNil(t, result.StructuredContent, "%s must return structured content", call.name)
				structuredJSON := mustMarshalJSON(t, result.StructuredContent)
				assert.JSONEq(t, call.text, structuredJSON)

				var schema jsonschema.Schema
				require.NoError(t, json.Unmarshal([]byte(mustMarshalJSON(t, outputSchemas[call.name].OutputSchema)), &schema))
				resolved, err := schema.Resolve(nil)
				require.NoError(t, err)
				var output any
				require.NoError(t, json.Unmarshal([]byte(structuredJSON), &output))
				require.NoError(t, resolved.Validate(output), "%s output must conform to its schema", call.name)
			}

			filtered, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "list_commits",
				Arguments: map[string]any{
					"owner": "owner", "repo": "repo", "page": "4", "perPage": "9",
					"fields": []any{"sha"},
				},
			})
			require.NoError(t, err)
			require.False(t, filtered.IsError, filtered)
			assert.Equal(t, `[{"sha":"abc123"}]`, getTextResult(t, filtered).Text)
			assert.Nil(t, filtered.StructuredContent, "field projection keeps the legacy wire shape")
		})
	}
}
