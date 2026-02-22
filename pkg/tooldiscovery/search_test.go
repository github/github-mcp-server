package tooldiscovery

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestSearchTools_EmptyQueryReturnsNil(t *testing.T) {
	results, err := SearchTools([]mcp.Tool{{Name: "issue_list"}}, "   ")
	require.NoError(t, err)
	require.Nil(t, results)
}

func TestSearchTools_FindsByName(t *testing.T) {
	tools := []mcp.Tool{
		{Name: "issue_list", Description: "List issues"},
		{Name: "repo_get", Description: "Get repository"},
	}

	results, err := SearchTools(tools, "issue", SearchOptions{MaxResults: 10})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Equal(t, "issue_list", results[0].Tool.Name)
}

func TestSearchTools_FindsByParameterName_JSONSchema(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name:        "unrelated_tool",
			Description: "does something else",
			InputSchema: &jsonschema.Schema{Properties: map[string]*jsonschema.Schema{"owner": {}}},
		},
	}

	results, err := SearchTools(tools, "owner", SearchOptions{MaxResults: 10})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Equal(t, "unrelated_tool", results[0].Tool.Name)
}

func TestSearchTools_FindsByParameterName_MapSchema(t *testing.T) {
	tools := []mcp.Tool{
		{
			Name:        "unrelated_tool",
			Description: "does something else",
			InputSchema: map[string]any{"properties": map[string]any{"repo": map[string]any{}}},
		},
	}

	results, err := SearchTools(tools, "repo", SearchOptions{MaxResults: 10})
	require.NoError(t, err)
	require.NotEmpty(t, results)
	require.Equal(t, "unrelated_tool", results[0].Tool.Name)
}
