package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/requeststate"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pullRequestArtifactApprovalRequest(args map[string]any) mcp.CallToolRequest {
	request := createMCPRequest(args)
	request.Params.Meta = mcp.Meta{
		mcp.MetaKeyProtocolVersion: inventory.ProtocolVersionMultiRoundTrip,
		mcp.MetaKeyClientCapabilities: map[string]any{
			"elicitation": map[string]any{"form": map[string]any{}},
		},
	}
	return request
}

func invokePullRequestArtifactWithApproval(ctx context.Context, t *testing.T, deps BaseDeps, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	sealer, err := requeststate.NewRandom()
	require.NoError(t, err)
	deps.StateSealer = sealer
	tool := pullRequestArtifactTestTool(args["method"].(string))
	handler := tool.Handler(deps)
	request := pullRequestArtifactApprovalRequest(args)
	result, err := handler(ContextWithDeps(ctx, deps), &request)
	require.NoError(t, err)
	if args["method"] != "delete" || result.IsError {
		return result
	}
	require.Contains(t, result.InputRequests, deletePullRequestArtifactConfirmationID)
	require.NotEmpty(t, result.RequestState)
	request.Params.RequestState = result.RequestState
	request.Params.InputResponses = mcp.InputResponseMap{
		deletePullRequestArtifactConfirmationID: &mcp.ElicitResult{
			Action: "accept",
			Content: map[string]any{
				deletePullRequestArtifactConfirmationField: artifactApprovalTarget(args),
			},
		},
	}
	result, err = handler(ContextWithDeps(ctx, deps), &request)
	require.NoError(t, err)
	return result
}

func artifactApprovalTarget(args map[string]any) string {
	name, _ := args["name"].(string)
	if key, ok := args["name_key"].(string); ok {
		decoded, _ := base64.RawURLEncoding.DecodeString(key)
		name = string(decoded)
	}
	return fmt.Sprintf("%s/%s#%d artifact %q", args["owner"], args["repo"], args["pull_number"], name)
}

func TestPullRequestArtifactDeleteApproval(t *testing.T) {
	for _, action := range []string{"accept", "decline", "cancel"} {
		t.Run(action, func(t *testing.T) {
			lookups, deletes, prompts := 0, 0, 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					lookups++
					assert.Equal(t, "/api/v3/repos/o/r", r.URL.Path)
					_, _ = w.Write([]byte(`{"id":42}`))
				case http.MethodDelete:
					deletes++
					assert.Equal(t, 1, prompts, "approval must precede DELETE")
					assert.Equal(t, "/api/v3/repositories/42/pulls/12/artifacts/report", r.URL.Path)
					w.WriteHeader(http.StatusNoContent)
				default:
					t.Errorf("unexpected method %s", r.Method)
				}
			})
			sealer, err := requeststate.NewRandom()
			require.NoError(t, err)
			session := pullRequestArtifactTestSession(t, BaseDeps{Client: client, StateSealer: sealer},
				inventory.ProtocolVersionMultiRoundTrip, &mcp.ClientOptions{
					ElicitationHandler: func(_ context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
						prompts++
						assert.Zero(t, deletes)
						assert.Equal(t, "form", req.Params.Mode)
						assert.Contains(t, req.Params.Message, `o/r#12 artifact \"report\"`)
						return &mcp.ElicitResult{
							Action: action,
							Content: map[string]any{
								deletePullRequestArtifactConfirmationField: `o/r#12 artifact "report"`,
							},
						}, nil
					},
				})
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "pull_request_artifact_write",
				Arguments: map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": "delete", "name": "report"},
			})
			require.NoError(t, err)
			assert.Equal(t, 1, prompts)
			if action == "accept" {
				require.False(t, result.IsError, mustMarshalJSON(t, result))
				assert.Equal(t, 2, lookups, "repository identity must be rechecked")
				assert.Equal(t, 1, deletes)
				assert.JSONEq(t, `{"deleted":{"pull_number":12,"name":"report","name_key":"cmVwb3J0","message":"artifact deleted"}}`, getTextResult(t, result).Text)
				assert.JSONEq(t, getTextResult(t, result).Text, mustMarshalJSON(t, result.StructuredContent))
			} else {
				require.True(t, result.IsError)
				assert.Equal(t, 1, lookups)
				assert.Zero(t, deletes)
			}
		})
	}
}

func TestPullRequestArtifactDeleteInvalidApproval(t *testing.T) {
	for _, name := range []string{
		"missing state", "tampered state", "invalid state JSON", "expired state",
		"changed owner", "changed repo", "changed PR", "changed name", "changed operation",
		"invalid repository ID", "recreated repository", "wrong confirmation", "missing confirmation",
		"invalid response", "lockdown after confirmation",
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				assert.Equal(t, http.MethodGet, r.Method, "invalid approval must never dispatch DELETE")
				id := 42
				if name == "recreated repository" && calls > 1 {
					id = 43
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"id": id}))
			})
			sealer, err := requeststate.NewRandom()
			require.NoError(t, err)
			deps := BaseDeps{Client: client, StateSealer: sealer}
			tool := PullRequestArtifactWrite(translations.NullTranslationHelper)
			handler := tool.Handler(deps)
			args := map[string]any{"owner": "o", "repo": "r", "pull_number": 12, "method": "delete", "name": "report"}
			request := pullRequestArtifactApprovalRequest(args)
			ctx := ContextWithDeps(context.Background(), deps)
			first, err := handler(ctx, &request)
			require.NoError(t, err)
			require.False(t, first.IsError, mustMarshalJSON(t, first))
			require.Contains(t, first.InputRequests, deletePullRequestArtifactConfirmationID)
			assert.Empty(t, first.Content)
			assert.Nil(t, first.StructuredContent)
			require.Equal(t, 1, calls, "requesting approval must only resolve the repository")
			stateJSON, err := sealer.Open(first.RequestState)
			require.NoError(t, err)
			var state deletePullRequestArtifactState
			require.NoError(t, json.Unmarshal(stateJSON, &state))
			assert.Equal(t, int64(42), state.RepositoryID)
			assert.Equal(t, 12, state.PullNumber)
			assert.Equal(t, "report", state.Name)
			assert.InDelta(t, time.Now().Add(10*time.Minute).Unix(), state.ExpiresAt, 2)
			confirmation := &mcp.ElicitResult{
				Action:  "accept",
				Content: map[string]any{deletePullRequestArtifactConfirmationField: artifactApprovalTarget(args)},
			}
			switch name {
			case "changed owner":
				args["owner"] = "other"
			case "changed repo":
				args["repo"] = "other"
			case "changed PR":
				args["pull_number"] = 13
			case "changed name":
				args["name"] = "other"
			case "changed operation":
				state.Operation = "another_tool:delete"
			case "invalid repository ID":
				state.RepositoryID = 0
			case "expired state":
				state.ExpiresAt = time.Now().Add(-time.Second).Unix()
			case "wrong confirmation":
				confirmation.Content[deletePullRequestArtifactConfirmationField] = "report"
			case "missing confirmation":
				confirmation.Content = nil
			case "lockdown after confirmation":
				deps.Flags.LockdownMode = true
				handler = tool.Handler(deps)
				ctx = ContextWithDeps(context.Background(), deps)
			}
			stateJSON, err = json.Marshal(state)
			require.NoError(t, err)
			if name == "invalid state JSON" {
				stateJSON = []byte(`{`)
			}
			request = pullRequestArtifactApprovalRequest(args)
			request.Params.RequestState, err = sealer.Seal(ctx, stateJSON)
			require.NoError(t, err)
			if name == "missing state" {
				request.Params.RequestState = ""
			}
			if name == "tampered state" {
				request.Params.RequestState = "forged-token"
			}
			request.Params.InputResponses = mcp.InputResponseMap{deletePullRequestArtifactConfirmationID: confirmation}
			if name == "invalid response" {
				request.Params.InputResponses[deletePullRequestArtifactConfirmationID] = nil
			}
			result, err := handler(ctx, &request)
			require.NoError(t, err)
			require.True(t, result.IsError, mustMarshalJSON(t, result))
			expectedCalls := 1
			if name == "recreated repository" {
				expectedCalls++
			}
			assert.Equal(t, expectedCalls, calls)
		})
	}
}

func TestPullRequestArtifactDeleteUnsupportedClient(t *testing.T) {
	for _, name := range []string{"legacy", "unknown protocol", "missing capabilities", "URL only", "no sealer", "nil request"} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			client := pullRequestArtifactTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
				calls++
				w.WriteHeader(http.StatusInternalServerError)
			})
			sealer, err := requeststate.NewRandom()
			require.NoError(t, err)
			deps := BaseDeps{Client: client, StateSealer: sealer}
			coordinates := PullRequestArtifactCoordinates{Owner: "o", Repo: "r", PullNumber: 12}
			input := PullRequestArtifactWriteInput{
				PullRequestArtifactCoordinates: coordinates,
				Method:                         "delete", Name: "report",
			}
			request := pullRequestArtifactApprovalRequest(nil)
			switch name {
			case "legacy":
				request.Params.Meta[mcp.MetaKeyProtocolVersion] = "2025-11-25"
			case "unknown protocol":
				delete(request.Params.Meta, mcp.MetaKeyProtocolVersion)
			case "missing capabilities":
				delete(request.Params.Meta, mcp.MetaKeyClientCapabilities)
			case "URL only":
				request.Params.Meta[mcp.MetaKeyClientCapabilities] = map[string]any{"elicitation": map[string]any{"url": map[string]any{}}}
			case "no sealer":
				deps.StateSealer = nil
			}
			req := &request
			if name == "nil request" {
				req = nil
			}
			_, result := confirmPullRequestArtifactDeletion(context.Background(), deps, req, input)
			require.NotNil(t, result)
			assert.True(t, result.IsError)
			assert.Zero(t, calls)
		})
	}
}
