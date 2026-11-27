package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/github/github-mcp-server/v2/internal/toolsnaps"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ListDeployments(t *testing.T) {
	serverTool := ListDeployments(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Equal(t, "list_deployments", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	for _, prop := range []string{"owner", "repo", "sha", "ref", "task", "environment", "page", "perPage"} {
		assert.Contains(t, schema.Properties, prop)
	}
	assert.ElementsMatch(t, []string{"owner", "repo"}, schema.Required)

	created := github.Timestamp{Time: time.Date(2026, 11, 17, 9, 30, 0, 0, time.UTC)}
	mockDeployments := []*github.Deployment{
		{
			ID:          new(int64(42)),
			SHA:         new("abc123"),
			Ref:         new("main"),
			Task:        new("deploy"),
			Environment: new("production"),
			Description: new("Release v1.2.0"),
			Creator:     &github.User{Login: new("deploy-bot")},
			CreatedAt:   &created,
			UpdatedAt:   &created,
			URL:         new("https://api.github.com/repos/owner/repo/deployments/42"),
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "lists deployments with filters and pagination",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(
					GetReposDeploymentsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"sha":         "abc123",
						"ref":         "main",
						"task":        "deploy",
						"environment": "production",
						"page":        "2",
						"per_page":    "5",
					}).andThen(mockResponse(t, http.StatusOK, mockDeployments)),
				),
			),
			requestArgs: map[string]any{
				"owner":       "owner",
				"repo":        "repo",
				"sha":         "abc123",
				"ref":         "main",
				"task":        "deploy",
				"environment": "production",
				"page":        float64(2),
				"perPage":     float64(5),
			},
		},
		{
			name: "lists deployments without filters",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(
					GetReposDeploymentsByOwnerByRepo,
					expectQueryParams(t, map[string]string{
						"page":     "1",
						"per_page": "30",
					}).andThen(mockResponse(t, http.StatusOK, mockDeployments)),
				),
			),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
		},
		{
			name: "API error is returned as a tool error",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(
					GetReposDeploymentsByOwnerByRepo,
					mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}),
				),
			),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "missing",
			},
			expectError:    true,
			expectedErrMsg: "failed to list deployments",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, tc.mockedClient)}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}
			require.False(t, result.IsError)

			var got []MinimalDeployment
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
			require.Len(t, got, 1)
			assert.Equal(t, MinimalDeployment{
				ID:          42,
				SHA:         "abc123",
				Ref:         "main",
				Task:        "deploy",
				Environment: "production",
				Description: "Release v1.2.0",
				Creator:     "deploy-bot",
				CreatedAt:   "2026-11-17T09:30:00Z",
				UpdatedAt:   "2026-11-17T09:30:00Z",
			}, got[0])
		})
	}
}

func Test_ListDeploymentStatuses(t *testing.T) {
	serverTool := ListDeploymentStatuses(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	schema, ok := tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.Equal(t, "list_deployment_statuses", tool.Name)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	for _, prop := range []string{"owner", "repo", "deployment_id", "page", "perPage"} {
		assert.Contains(t, schema.Properties, prop)
	}
	assert.ElementsMatch(t, []string{"owner", "repo", "deployment_id"}, schema.Required)

	newer := github.Timestamp{Time: time.Date(2026, 11, 17, 9, 35, 0, 0, time.UTC)}
	older := github.Timestamp{Time: time.Date(2026, 11, 17, 9, 31, 0, 0, time.UTC)}
	mockStatuses := []*github.DeploymentStatus{
		{
			ID:             new(int64(7)),
			State:          new("success"),
			Description:    new("Deployed"),
			Environment:    new("production"),
			EnvironmentURL: new("https://example.com"),
			LogURL:         new("https://github.com/owner/repo/actions/runs/1"),
			Creator:        &github.User{Login: new("deploy-bot")},
			CreatedAt:      &newer,
			UpdatedAt:      &newer,
		},
		{
			ID:          new(int64(6)),
			State:       new("in_progress"),
			Environment: new("production"),
			CreatedAt:   &older,
			UpdatedAt:   &older,
		},
	}

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
	}{
		{
			name: "lists statuses newest first",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(
					GetReposDeploymentStatusesByOwnerByRepoByDeploymentID,
					expectPath(t, "/repos/owner/repo/deployments/42/statuses").
						andThen(mockResponse(t, http.StatusOK, mockStatuses)),
				),
			),
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"deployment_id": float64(42),
			},
		},
		{
			name:         "missing deployment_id",
			mockedClient: NewMockedHTTPClient(),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
			},
			expectError:    true,
			expectedErrMsg: "deployment_id",
		},
		{
			name: "API error is returned as a tool error",
			mockedClient: NewMockedHTTPClient(
				WithRequestMatchHandler(
					GetReposDeploymentStatusesByOwnerByRepoByDeploymentID,
					mockResponse(t, http.StatusNotFound, map[string]string{"message": "Not Found"}),
				),
			),
			requestArgs: map[string]any{
				"owner":         "owner",
				"repo":          "repo",
				"deployment_id": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to list deployment statuses",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			deps := BaseDeps{Client: mustNewGHClient(t, tc.mockedClient)}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)

			if tc.expectError {
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tc.expectedErrMsg)
				return
			}
			require.False(t, result.IsError)

			var got []MinimalDeploymentStatus
			require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &got))
			require.Len(t, got, 2)
			assert.Equal(t, MinimalDeploymentStatus{
				ID:             7,
				State:          "success",
				Description:    "Deployed",
				Environment:    "production",
				EnvironmentURL: "https://example.com",
				LogURL:         "https://github.com/owner/repo/actions/runs/1",
				Creator:        "deploy-bot",
				CreatedAt:      "2026-11-17T09:35:00Z",
				UpdatedAt:      "2026-11-17T09:35:00Z",
			}, got[0])
			assert.Equal(t, "in_progress", got[1].State)
			assert.Empty(t, got[1].Creator)
		})
	}
}

func Test_ListDeployments_IFCLabel(t *testing.T) {
	serverTool := ListDeployments(translations.NullTranslationHelper)

	for _, tc := range []struct {
		name            string
		private         bool
		confidentiality string
	}{
		{name: "public repo", private: false, confidentiality: "public"},
		{name: "private repo", private: true, confidentiality: "private"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposByOwnerByRepo: mockResponse(t, http.StatusOK, map[string]any{
					"name":    "repo",
					"private": tc.private,
				}),
				GetReposDeploymentsByOwnerByRepo: mockResponse(t, http.StatusOK, []*github.Deployment{
					{ID: new(int64(1)), SHA: new("abc"), Environment: new("preview")},
				}),
			})
			deps := BaseDeps{
				Client:         mustNewGHClient(t, client),
				featureChecker: featureCheckerFor(FeatureFlagIFCLabels),
			}
			handler := serverTool.Handler(deps)
			request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)
			require.NoError(t, err)
			require.False(t, result.IsError)

			require.NotNil(t, result.Meta)
			ifcJSON, err := json.Marshal(result.Meta["ifc"])
			require.NoError(t, err)
			var ifcMap map[string]any
			require.NoError(t, json.Unmarshal(ifcJSON, &ifcMap))
			assert.Equal(t, "untrusted", ifcMap["integrity"])
			assert.Equal(t, tc.confidentiality, ifcMap["confidentiality"])
		})
	}
}
