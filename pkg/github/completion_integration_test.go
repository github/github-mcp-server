package github

import (
	"context"
	"testing"

	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitHubMCPServerCompletionIntegration(t *testing.T) {
	// Mock client function
	getClient := func(_ context.Context) (*github.Client, error) {
		// Return a nil client - this will cause API calls to fail gracefully
		// which is fine for testing the completion request handling flow
		return nil, nil
	}

	// Create a GitHub MCP server with completion support
	ghServer := NewGitHubServer("test", getClient)
	require.NotNil(t, ghServer)

	// Create an in-process client with our custom GitHubMCPServer transport
	mcpClient, err := NewInProcessClientWithGitHubServer(ghServer)
	require.NoError(t, err)

	// Initialize the client
	ctx := context.Background()
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = "2025-03-26"
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	result, err := mcpClient.Initialize(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, "github-mcp-server", result.ServerInfo.Name)

	// Test completion request - this should work even with a nil GitHub client
	// because non-repo URIs return empty completions without calling GitHub APIs
	completionRequest := mcp.CompleteRequest{
		Params: struct {
			Ref      any `json:"ref"`
			Argument struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"argument"`
		}{
			Ref: map[string]interface{}{
				"type": "ref/resource",
				"uri":  "file:///some/non-repo/path",
			},
			Argument: struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}{
				Name:  "param",
				Value: "test",
			},
		},
	}

	completionResult, err := mcpClient.Complete(ctx, completionRequest)
	require.NoError(t, err)
	require.NotNil(t, completionResult)

	// Should return empty completion for non-repo URIs
	assert.Equal(t, []string{}, completionResult.Completion.Values)
	assert.Equal(t, 0, completionResult.Completion.Total)

	// Test repo URI completion with unsupported argument
	repoCompletionRequest := mcp.CompleteRequest{
		Params: struct {
			Ref      any `json:"ref"`
			Argument struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			} `json:"argument"`
		}{
			Ref: map[string]interface{}{
				"type": "ref/resource",
				"uri":  "repo://{owner}/{repo}/contents{/path*}",
			},
			Argument: struct {
				Name  string `json:"name"`
				Value string `json:"value"`
			}{
				Name:  "unsupported",
				Value: "test",
			},
		},
	}

	repoCompletionResult, err := mcpClient.Complete(ctx, repoCompletionRequest)
	require.NoError(t, err)
	require.NotNil(t, repoCompletionResult)

	// Should return empty completion for unsupported arguments
	assert.Equal(t, []string{}, repoCompletionResult.Completion.Values)
	assert.Equal(t, 0, repoCompletionResult.Completion.Total)

	// Clean up
	err = mcpClient.Close()
	assert.NoError(t, err)
}

func TestGitHubMCPServerCompletionCapabilities(t *testing.T) {
	// Mock client function
	getClient := func(_ context.Context) (*github.Client, error) {
		return nil, nil
	}

	// Create a GitHub MCP server with completion support
	ghServer := NewGitHubServer("test", getClient)
	require.NotNil(t, ghServer)

	// Create an in-process client with our custom GitHubMCPServer transport
	mcpClient, err := NewInProcessClientWithGitHubServer(ghServer)
	require.NoError(t, err)

	// Initialize the client
	ctx := context.Background()
	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = "2025-03-26"
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	result, err := mcpClient.Initialize(ctx, request)
	require.NoError(t, err)

	// Check basic server info
	assert.Equal(t, "github-mcp-server", result.ServerInfo.Name)

	// Clean up
	err = mcpClient.Close()
	assert.NoError(t, err)
}