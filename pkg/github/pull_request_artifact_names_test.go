package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/v2/internal/requeststate"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestArtifactNameRoundTrip(t *testing.T) {
	for _, names := range [][2]string{
		{"AT&amp;T", "AT&T"},
		{"report<b>", "report"},
		{"rep\u200bort", "report"},
		{"draft #1?% &amp;", "draft #1?% &"},
	} {
		t.Run(names[0], func(t *testing.T) {
			bodies := map[string]string{names[0]: "original", names[1]: "do not change"}
			var mutations []string
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/api/v3/repos/o/r" {
					assert.Equal(t, http.MethodGet, r.Method)
					_, _ = w.Write([]byte(`{"id":42}`))
					return
				}
				if r.Method == http.MethodGet {
					assert.Equal(t, "/api/v3/repositories/42/pulls/12/artifacts", r.URL.Path)
					artifacts := make([]map[string]any, 0, len(names))
					for i, name := range names {
						artifacts = append(artifacts, map[string]any{
							"id": "opaque-id", "number": i + 1, "name": name, "body": bodies[name],
						})
					}
					require.NoError(t, json.NewEncoder(w).Encode(artifacts))
					return
				}
				assert.Equal(t, "/api/v3/repositories/42/pulls/12/artifacts/"+url.PathEscape(names[0]), r.URL.EscapedPath())
				assert.Empty(t, r.URL.RawQuery)
				mutations = append(mutations, r.Method)
				switch r.Method {
				case http.MethodPut:
					var payload struct {
						Body string `json:"body"`
					}
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					bodies[names[0]] = payload.Body
					require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
						"id": "opaque-id", "number": 1, "name": names[0], "body": payload.Body,
					}))
				case http.MethodDelete:
					delete(bodies, names[0])
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected method %s", r.Method)
				}
			})
			sealer, err := requeststate.NewRandom()
			require.NoError(t, err)
			deps := BaseDeps{Client: client, StateSealer: sealer}
			target := fmt.Sprintf("o/r#12 artifact %q", names[0])
			session := pullRequestArtifactTestSession(t, deps, inventory.ProtocolVersionMultiRoundTrip, &mcp.ClientOptions{
				ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
					assert.Contains(t, req.Params.Message, fmt.Sprintf("%q", target))
					return &mcp.ElicitResult{
						Action: "accept",
						Content: map[string]any{
							deletePullRequestArtifactConfirmationField: target,
						},
					}, nil
				},
			})
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "pull_request_artifact_read",
				Arguments: map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": "list"},
			})
			require.NoError(t, err)
			require.False(t, result.IsError, mustMarshalJSON(t, result))
			var listed PullRequestArtifactReadOutput
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &listed))
			require.Len(t, listed.List.Artifacts, 2)
			first, second := listed.List.Artifacts[0], listed.List.Artifacts[1]
			assert.Equal(t, first.Name, second.Name, "distinct names collide after display sanitization")
			assert.NotEqual(t, first.NameKey, second.NameKey)
			assert.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(names[0])), first.NameKey)

			updated, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "pull_request_artifact_write",
				Arguments: map[string]any{
					"owner": "o", "repo": "r", "pull_number": 12,
					"method": "upsert", "name_key": first.NameKey, "body": "updated",
				},
			})
			require.NoError(t, err)
			require.False(t, updated.IsError, mustMarshalJSON(t, updated))
			var written PullRequestArtifactWriteOutput
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, updated).Text), &written))
			require.NotNil(t, written.Artifact)
			assert.Equal(t, first.NameKey, written.Artifact.NameKey)
			assert.Equal(t, "updated", bodies[names[0]])
			assert.Equal(t, "do not change", bodies[names[1]])

			deleted, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name: "pull_request_artifact_write",
				Arguments: map[string]any{
					"owner": "o", "repo": "r", "pull_number": 12,
					"method": "delete", "name_key": written.Artifact.NameKey,
				},
			})
			require.NoError(t, err)
			require.False(t, deleted.IsError, mustMarshalJSON(t, deleted))
			var confirmation PullRequestArtifactWriteOutput
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, deleted).Text), &confirmation))
			require.NotNil(t, confirmation.Deleted)
			assert.Equal(t, first.NameKey, confirmation.Deleted.NameKey)
			assert.Equal(t, first.Name, confirmation.Deleted.Name)
			assert.NotContains(t, bodies, names[0])
			assert.Equal(t, "do not change", bodies[names[1]])
			assert.Equal(t, []string{http.MethodPut, http.MethodDelete}, mutations)
		})
	}
}
