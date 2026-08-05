package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const endpointSemanticallySimilar = EndpointPattern("GET /repos/{owner}/{repo}/issues/{issue_number}/semantically_similar")

func Test_FindDuplicate(t *testing.T) {
	// Verify tool definition once (flag-gated variant snap).
	serverTool := FindDuplicate(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name+"_ff_"+FeatureFlagDuplicateDetection, tool))
	require.Equal(t, FeatureFlagDuplicateDetection, serverTool.FeatureFlagEnable)

	assert.Equal(t, "find_duplicate", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.True(t, tool.Annotations.ReadOnlyHint)
	assert.ElementsMatch(t, serverTool.RequiredScopes, []string{"repo"})

	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "issue_number")
	assert.Contains(t, schema.Properties, "confidence_threshold")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "issue_number"})
}

func Test_FindDuplicate_RankedResults(t *testing.T) {
	serverTool := FindDuplicate(translations.NullTranslationHelper)

	rankedResults := []map[string]any{
		{
			"issue": map[string]any{
				"number":   456,
				"title":    "Example failure when saving",
				"state":    "open",
				"html_url": "https://github.com/owner/repo/issues/456",
			},
			"score":            0.95,
			"confidence":       "high",
			"likely_duplicate": true,
		},
		{
			"issue": map[string]any{
				"number":   789,
				"title":    "Possibly related",
				"state":    "closed",
				"html_url": "https://github.com/owner/repo/issues/789",
			},
			"score":            nil, // score is nullable
			"confidence":       "low",
			"likely_duplicate": false,
		},
	}

	var capturedURL *url.URL
	var capturedMethod string
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		capturedMethod = r.Method
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(MustMarshal(rankedResults))
	}

	client := mustNewGHClient(t, NewMockedHTTPClient(WithRequestMatchHandler(endpointSemanticallySimilar, http.HandlerFunc(handler))))
	deps := BaseDeps{Client: client}
	toolHandler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":                "owner",
		"repo":                 "repo",
		"issue_number":         float64(123),
		"confidence_threshold": float64(0.8),
		"perPage":              float64(10),
		"page":                 float64(1),
	})
	result, err := toolHandler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, "expected result to not be an error")

	// The tool must be read-only: only a GET is issued.
	assert.Equal(t, http.MethodGet, capturedMethod)

	// confidence_threshold maps to threshold; perPage maps to per_page; page is forwarded.
	require.NotNil(t, capturedURL)
	assert.Equal(t, "0.8", capturedURL.Query().Get("threshold"))
	assert.Equal(t, "10", capturedURL.Query().Get("per_page"))
	assert.Equal(t, "1", capturedURL.Query().Get("page"))

	text := getTextResult(t, result)
	var candidates []duplicateIssueResult
	require.NoError(t, json.Unmarshal([]byte(text.Text), &candidates))
	require.Len(t, candidates, 2)

	assert.Equal(t, "high", candidates[0].Confidence)
	assert.True(t, candidates[0].LikelyDuplicate)
	require.NotNil(t, candidates[0].Score)
	assert.InDelta(t, 0.95, *candidates[0].Score, 0.0001)
	var firstIssue struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
		URL    string `json:"html_url"`
	}
	require.NoError(t, json.Unmarshal(candidates[0].Issue, &firstIssue))
	assert.Equal(t, 456, firstIssue.Number)
	assert.Equal(t, "open", firstIssue.State)
	assert.NotEmpty(t, firstIssue.URL)

	// A null score must decode successfully.
	assert.Nil(t, candidates[1].Score)
	assert.Equal(t, "low", candidates[1].Confidence)
	assert.False(t, candidates[1].LikelyDuplicate)
}

func Test_FindDuplicate_OmitsUnsetParams(t *testing.T) {
	serverTool := FindDuplicate(translations.NullTranslationHelper)

	var capturedURL *url.URL
	handler := func(w http.ResponseWriter, r *http.Request) {
		capturedURL = r.URL
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}

	client := mustNewGHClient(t, NewMockedHTTPClient(WithRequestMatchHandler(endpointSemanticallySimilar, http.HandlerFunc(handler))))
	deps := BaseDeps{Client: client}
	toolHandler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(123),
	})
	result, err := toolHandler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	require.NotNil(t, capturedURL)
	q := capturedURL.Query()
	_, hasThreshold := q["threshold"]
	_, hasPerPage := q["per_page"]
	_, hasPage := q["page"]
	assert.False(t, hasThreshold, "threshold should be omitted when unset")
	assert.False(t, hasPerPage, "per_page should be omitted when unset")
	assert.False(t, hasPage, "page should be omitted when unset")
}

func Test_FindDuplicate_EmptyResults(t *testing.T) {
	serverTool := FindDuplicate(translations.NullTranslationHelper)

	client := mustNewGHClient(t, NewMockedHTTPClient(WithRequestMatch(endpointSemanticallySimilar, []map[string]any{})))
	deps := BaseDeps{Client: client}
	toolHandler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(123),
	})
	result, err := toolHandler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError, "empty results is a successful search")

	text := getTextResult(t, result)
	var candidates []duplicateIssueResult
	require.NoError(t, json.Unmarshal([]byte(text.Text), &candidates))
	assert.Empty(t, candidates)
}

func Test_FindDuplicate_LegacyBareIssueResponse(t *testing.T) {
	serverTool := FindDuplicate(translations.NullTranslationHelper)

	// When ranked duplicate detection is disabled the endpoint returns bare
	// issue resources (no ranking metadata), which must fail clearly.
	bareIssues := []map[string]any{
		{
			"number":   456,
			"title":    "Example",
			"state":    "open",
			"html_url": "https://github.com/owner/repo/issues/456",
		},
	}

	client := mustNewGHClient(t, NewMockedHTTPClient(WithRequestMatch(endpointSemanticallySimilar, bareIssues)))
	deps := BaseDeps{Client: client}
	toolHandler := serverTool.Handler(deps)

	request := createMCPRequest(map[string]any{
		"owner":        "owner",
		"repo":         "repo",
		"issue_number": float64(123),
	})
	result, err := toolHandler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	getErrorResult(t, result)
}

func Test_FindDuplicate_Errors(t *testing.T) {
	serverTool := FindDuplicate(translations.NullTranslationHelper)

	t.Run("missing required param", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient())
		deps := BaseDeps{Client: client}
		toolHandler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner": "owner",
			"repo":  "repo",
		})
		result, err := toolHandler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		getErrorResult(t, result)
	})

	t.Run("API error is surfaced", func(t *testing.T) {
		client := mustNewGHClient(t, NewMockedHTTPClient(
			WithRequestMatchHandler(endpointSemanticallySimilar, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"message": "Not Found"}`))
			})),
		))
		deps := BaseDeps{Client: client}
		toolHandler := serverTool.Handler(deps)
		request := createMCPRequest(map[string]any{
			"owner":        "owner",
			"repo":         "repo",
			"issue_number": float64(123),
		})
		result, err := toolHandler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		getErrorResult(t, result)
	})
}
