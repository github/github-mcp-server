package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetStack_ToolDefinition(t *testing.T) {
	serverTool := GetStack(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "get_stack", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "stackNumber")
	assert.Contains(t, schema.Properties, "pullNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})
}

func Test_ListStacks_ToolDefinition(t *testing.T) {
	serverTool := ListStacks(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "list_stacks", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "page")
	assert.Contains(t, schema.Properties, "perPage")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})
}

func Test_LinkStack_ToolDefinition(t *testing.T) {
	serverTool := LinkStack(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "link_stack", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "pullNumbers")
	assert.Contains(t, schema.Properties, "base")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "pullNumbers"})
}

func Test_UpdateStack_ToolDefinition(t *testing.T) {
	serverTool := UpdateStack(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "update_stack", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "stackNumber")
	assert.Contains(t, schema.Properties, "pullNumbers")
	assert.Contains(t, schema.Properties, "base")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "stackNumber"})
}

func Test_DissolveStack_ToolDefinition(t *testing.T) {
	serverTool := DissolveStack(translations.NullTranslationHelper)
	tool := serverTool.Tool
	require.NoError(t, toolsnaps.Test(tool.Name, tool))

	assert.Equal(t, "dissolve_stack", tool.Name)
	assert.NotEmpty(t, tool.Description)
	schema := tool.InputSchema.(*jsonschema.Schema)
	assert.Contains(t, schema.Properties, "owner")
	assert.Contains(t, schema.Properties, "repo")
	assert.Contains(t, schema.Properties, "stackNumber")
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "stackNumber"})
}

func Test_GetStack_Execution(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/stacks/10", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Stack{
			ID:          100,
			StackNumber: 10,
			Title:       "Test Stack",
			Base:        "main",
			PullRequests: []StackLayer{
				{PullNumber: 101, Head: "feature-1", Base: "main"},
				{PullNumber: 102, Head: "feature-2", Base: "feature-1"},
			},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := github.NewClient(ts.Client())
	url, _ := url.Parse(ts.URL + "/")
	client.BaseURL = url

	deps := ToolDependencies{
		GetClient: func(ctx context.Context) (*github.Client, error) {
			return client, nil
		},
	}

	serverTool := GetStack(translations.NullTranslationHelper)
	handler := serverTool.HandlerFunc(deps)

	res, err := handler(context.Background(), &mcp.CallToolRequest{}, map[string]any{
		"owner":       "owner",
		"repo":        "repo",
		"stackNumber": 10,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Content[0].(mcp.TextContent).Text, `"stack_number":10`)
}

func Test_LinkStack_Execution(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/stacks", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		var body LinkStackInput
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "main", body.Base)
		assert.Equal(t, []int{101, 102}, body.PullNumbers)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Stack{
			ID:          200,
			StackNumber: 15,
			Base:        body.Base,
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := github.NewClient(ts.Client())
	url, _ := url.Parse(ts.URL + "/")
	client.BaseURL = url

	deps := ToolDependencies{
		GetClient: func(ctx context.Context) (*github.Client, error) {
			return client, nil
		},
	}

	serverTool := LinkStack(translations.NullTranslationHelper)
	handler := serverTool.HandlerFunc(deps)

	res, err := handler(context.Background(), &mcp.CallToolRequest{}, map[string]any{
		"owner":       "owner",
		"repo":        "repo",
		"base":        "main",
		"pullNumbers": []any{101, 102},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Content[0].(mcp.TextContent).Text, `"stack_number":15`)
}

func Test_DissolveStack_Execution(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/stacks/10", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "DELETE", r.Method)
		w.WriteHeader(http.StatusNoContent)
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := github.NewClient(ts.Client())
	url, _ := url.Parse(ts.URL + "/")
	client.BaseURL = url

	deps := ToolDependencies{
		GetClient: func(ctx context.Context) (*github.Client, error) {
			return client, nil
		},
	}

	serverTool := DissolveStack(translations.NullTranslationHelper)
	handler := serverTool.HandlerFunc(deps)

	res, err := handler(context.Background(), &mcp.CallToolRequest{}, map[string]any{
		"owner":       "owner",
		"repo":        "repo",
		"stackNumber": 10,
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Contains(t, res.Content[0].(mcp.TextContent).Text, "Successfully dissolved stack 10")
}
