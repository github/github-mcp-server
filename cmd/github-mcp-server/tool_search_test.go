package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/tooldiscovery"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolSearchCommandRegistered(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"tool-search"})
	require.NoError(t, err)
	assert.Same(t, toolSearchCmd, command)

	maxResults := toolSearchCmd.Flags().Lookup("max-results")
	require.NotNil(t, maxResults)
	assert.Equal(t, "3", maxResults.DefValue)
}

func TestSearchConfiguredTools(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		cfg         toolSearchConfig
		wantTool    string
		excludeTool string
		wantCount   int
	}{
		{
			name:     "searches selected toolset",
			query:    "issue_read",
			cfg:      toolSearchConfig{enabledToolsets: []string{"issues"}},
			wantTool: "issue_read",
		},
		{
			name:      "specific tools replace defaults",
			query:     "get_me",
			cfg:       toolSearchConfig{enabledTools: []string{"get_me"}},
			wantTool:  "get_me",
			wantCount: 1,
		},
		{
			name:        "honors read only mode",
			query:       "issue_write",
			cfg:         toolSearchConfig{enabledToolsets: []string{"issues"}, readOnly: true},
			excludeTool: "issue_write",
		},
		{
			name:        "honors excluded tools",
			query:       "issue_read",
			cfg:         toolSearchConfig{enabledToolsets: []string{"issues"}, excludedTools: []string{"issue_read"}},
			excludeTool: "issue_read",
		},
		{
			name:     "honors feature flags",
			query:    "find_duplicate",
			cfg:      toolSearchConfig{enabledToolsets: []string{"issues"}, enabledFeatures: []string{"duplicate_detection"}},
			wantTool: "find_duplicate",
		},
		{
			name:        "omits disabled feature tools",
			query:       "find_duplicate",
			cfg:         toolSearchConfig{enabledToolsets: []string{"issues"}},
			excludeTool: "find_duplicate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := searchConfiguredTools(
				context.Background(),
				tt.query,
				10,
				tt.cfg,
				translations.NullTranslationHelper,
			)
			require.NoError(t, err)

			names := make([]string, len(results))
			for i, result := range results {
				names[i] = result.Tool.Name
			}
			if tt.wantTool != "" {
				assert.Contains(t, names, tt.wantTool)
			}
			if tt.excludeTool != "" {
				assert.NotContains(t, names, tt.excludeTool)
			}
			if tt.wantCount > 0 {
				assert.Len(t, results, tt.wantCount)
			}
		})
	}
}

func TestWriteToolSearchResults(t *testing.T) {
	t.Run("results", func(t *testing.T) {
		var output bytes.Buffer
		err := writeToolSearchResults(&output, []tooldiscovery.SearchResult{
			{Tool: mcp.Tool{Name: "issue_read", Description: "Read an issue."}},
		})
		require.NoError(t, err)
		assert.Contains(t, output.String(), "Found 1 matching tool:")
		assert.Contains(t, output.String(), "issue_read")
		assert.Contains(t, output.String(), "Read an issue.")
	})

	t.Run("no results", func(t *testing.T) {
		var output bytes.Buffer
		require.NoError(t, writeToolSearchResults(&output, nil))
		assert.Equal(t, "No matching tools found.\n", output.String())
	})
}
