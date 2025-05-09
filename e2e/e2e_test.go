//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/github/github-mcp-server/internal/ghmcp"
	"github.com/github/github-mcp-server/pkg/github"
	"github.com/github/github-mcp-server/pkg/translations"
	gogithub "github.com/google/go-github/v69/github"
	mcpClient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

var (
	// Shared variables and sync.Once instances to ensure one-time execution
	getTokenOnce sync.Once
	token        string

	buildOnce  sync.Once
	buildError error
)

// getE2EToken ensures the environment variable is checked only once and returns the token
func getE2EToken(t *testing.T) string {
	getTokenOnce.Do(func() {
		token = os.Getenv("GITHUB_MCP_SERVER_E2E_TOKEN")
		if token == "" {
			t.Fatalf("GITHUB_MCP_SERVER_E2E_TOKEN environment variable is not set")
		}
	})
	return token
}

// ensureDockerImageBuilt makes sure the Docker image is built only once across all tests
func ensureDockerImageBuilt(t *testing.T) {
	buildOnce.Do(func() {
		t.Log("Building Docker image for e2e tests...")
		cmd := exec.Command("docker", "build", "-t", "github/e2e-github-mcp-server", ".")
		cmd.Dir = ".." // Run this in the context of the root, where the Dockerfile is located.
		output, err := cmd.CombinedOutput()
		buildError = err
		if err != nil {
			t.Logf("Docker build output: %s", string(output))
		}
	})

	// Check if the build was successful
	require.NoError(t, buildError, "expected to build Docker image successfully")
}

// clientOpts holds configuration options for the MCP client setup
type clientOpts struct {
	// Toolsets to enable in the MCP server
	enabledToolsets []string
}

// clientOption defines a function type for configuring ClientOpts
type clientOption func(*clientOpts)

// withToolsets returns an option that either sets an Env Var when executing in docker,
// or sets the toolsets in the MCP server when running in-process.
func withToolsets(toolsets []string) clientOption {
	return func(opts *clientOpts) {
		opts.enabledToolsets = toolsets
	}
}

func setupMCPClient(t *testing.T, options ...clientOption) *mcpClient.Client {
	// Get token and ensure Docker image is built
	token := getE2EToken(t)

	// Create and configure options
	opts := &clientOpts{}

	// Apply all options to configure the opts struct
	for _, option := range options {
		option(opts)
	}

	// By default, we run the tests including the Docker image, but with DEBUG
	// enabled, we run the server in-process, allowing for easier debugging.
	var client *mcpClient.Client
	if os.Getenv("GITHUB_MCP_SERVER_E2E_DEBUG") == "" {
		ensureDockerImageBuilt(t)

		// Prepare Docker arguments
		args := []string{
			"docker",
			"run",
			"-i",
			"--rm",
			"-e",
			"GITHUB_PERSONAL_ACCESS_TOKEN", // Personal access token is all required
		}

		// Add toolsets environment variable to the Docker arguments
		if len(opts.enabledToolsets) > 0 {
			args = append(args, "-e", "GITHUB_TOOLSETS")
		}

		// Add the image name
		args = append(args, "github/e2e-github-mcp-server")

		// Construct the env vars for the MCP Client to execute docker with
		dockerEnvVars := []string{
			fmt.Sprintf("GITHUB_PERSONAL_ACCESS_TOKEN=%s", token),
			fmt.Sprintf("GITHUB_TOOLSETS=%s", strings.Join(opts.enabledToolsets, ",")),
		}

		// Create the client
		t.Log("Starting Stdio MCP client...")
		var err error
		client, err = mcpClient.NewStdioMCPClient(args[0], dockerEnvVars, args[1:]...)
		require.NoError(t, err, "expected to create client successfully")
	} else {
		// We need this because the fully compiled server has a default for the viper config, which is
		// not in scope for using the MCP server directly. This probably indicates that we should refactor
		// so that there is a shared setup mechanism, but let's wait till we feel more friction.
		enabledToolsets := opts.enabledToolsets
		if enabledToolsets == nil {
			enabledToolsets = github.DefaultTools
		}

		ghServer, err := ghmcp.NewMCPServer(ghmcp.MCPServerConfig{
			Token:           token,
			EnabledToolsets: enabledToolsets,
			Translator:      translations.NullTranslationHelper,
		})
		require.NoError(t, err, "expected to construct MCP server successfully")

		t.Log("Starting In Process MCP client...")
		client, err = mcpClient.NewInProcessClient(ghServer)
		require.NoError(t, err, "expected to create in-process client successfully")
	}

	t.Cleanup(func() {
		require.NoError(t, client.Close(), "expected to close client successfully")
	})

	// Initialize the client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request := mcp.InitializeRequest{}
	request.Params.ProtocolVersion = "2025-03-26"
	request.Params.ClientInfo = mcp.Implementation{
		Name:    "e2e-test-client",
		Version: "0.0.1",
	}

	result, err := client.Initialize(ctx, request)
	require.NoError(t, err, "failed to initialize client")
	require.Equal(t, "github-mcp-server", result.ServerInfo.Name, "unexpected server name")

	return client
}

func TestGetMe(t *testing.T) {
	t.Parallel()

	mcpClient := setupMCPClient(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// When we call the "get_me" tool
	request := mcp.CallToolRequest{}
	request.Params.Name = "get_me"

	response, err := mcpClient.CallTool(ctx, request)
	require.NoError(t, err, "expected to call 'get_me' tool successfully")

	require.False(t, response.IsError, "expected result not to be an error")
	require.Len(t, response.Content, 1, "expected content to have one item")

	textContent, ok := response.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedContent struct {
		Login string `json:"login"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedContent)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	// Then the login in the response should match the login obtained via the same
	// token using the GitHub API.
	ghClient := gogithub.NewClient(nil).WithAuthToken(getE2EToken(t))
	user, _, err := ghClient.Users.Get(context.Background(), "")
	require.NoError(t, err, "expected to get user successfully")
	require.Equal(t, trimmedContent.Login, *user.Login, "expected login to match")

}

func TestToolsets(t *testing.T) {
	t.Parallel()

	mcpClient := setupMCPClient(
		t,
		withToolsets([]string{"repos", "issues"}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	request := mcp.ListToolsRequest{}
	response, err := mcpClient.ListTools(ctx, request)
	require.NoError(t, err, "expected to list tools successfully")

	// We could enumerate the tools here, but we'll need to expose that information
	// declaratively in the MCP server, so for the moment let's just check the existence
	// of an issue and repo tool, and the non-existence of a pull_request tool.
	var toolsContains = func(expectedName string) bool {
		return slices.ContainsFunc(response.Tools, func(tool mcp.Tool) bool {
			return tool.Name == expectedName
		})
	}

	require.True(t, toolsContains("get_issue"), "expected to find 'get_issue' tool")
	require.True(t, toolsContains("list_branches"), "expected to find 'list_branches' tool")
	require.False(t, toolsContains("get_pull_request"), "expected not to find 'get_pull_request' tool")
}

func TestTags(t *testing.T) {
	t.Parallel()

	mcpClient := setupMCPClient(t)

	ctx := context.Background()

	// First, who am I
	getMeRequest := mcp.CallToolRequest{}
	getMeRequest.Params.Name = "get_me"

	t.Log("Getting current user...")
	resp, err := mcpClient.CallTool(ctx, getMeRequest)
	require.NoError(t, err, "expected to call 'get_me' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	require.False(t, resp.IsError, "expected result not to be an error")
	require.Len(t, resp.Content, 1, "expected content to have one item")

	textContent, ok := resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedGetMeText struct {
		Login string `json:"login"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedGetMeText)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	currentOwner := trimmedGetMeText.Login

	// Then create a repository with a README (via autoInit)
	repoName := fmt.Sprintf("github-mcp-server-e2e-%s-%d", t.Name(), time.Now().UnixMilli())
	createRepoRequest := mcp.CallToolRequest{}
	createRepoRequest.Params.Name = "create_repository"
	createRepoRequest.Params.Arguments = map[string]any{
		"name":     repoName,
		"private":  true,
		"autoInit": true,
	}

	t.Logf("Creating repository %s/%s...", currentOwner, repoName)
	_, err = mcpClient.CallTool(ctx, createRepoRequest)
	require.NoError(t, err, "expected to call 'get_me' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Cleanup the repository after the test
	t.Cleanup(func() {
		// MCP Server doesn't support deletions, but we can use the GitHub Client
		ghClient := gogithub.NewClient(nil).WithAuthToken(getE2EToken(t))
		t.Logf("Deleting repository %s/%s...", currentOwner, repoName)
		_, err := ghClient.Repositories.Delete(context.Background(), currentOwner, repoName)
		require.NoError(t, err, "expected to delete repository successfully")
	})

	// Then create a tag
	// MCP Server doesn't support tag creation, but we can use the GitHub Client
	ghClient := gogithub.NewClient(nil).WithAuthToken(getE2EToken(t))
	t.Logf("Creating tag %s/%s:%s...", currentOwner, repoName, "v0.0.1")
	ref, _, err := ghClient.Git.GetRef(context.Background(), currentOwner, repoName, "refs/heads/main")
	require.NoError(t, err, "expected to get ref successfully")

	tagObj, _, err := ghClient.Git.CreateTag(context.Background(), currentOwner, repoName, &gogithub.Tag{
		Tag:     gogithub.Ptr("v0.0.1"),
		Message: gogithub.Ptr("v0.0.1"),
		Object: &gogithub.GitObject{
			SHA:  ref.Object.SHA,
			Type: gogithub.Ptr("commit"),
		},
	})
	require.NoError(t, err, "expected to create tag object successfully")

	_, _, err = ghClient.Git.CreateRef(context.Background(), currentOwner, repoName, &gogithub.Reference{
		Ref: gogithub.Ptr("refs/tags/v0.0.1"),
		Object: &gogithub.GitObject{
			SHA: tagObj.SHA,
		},
	})
	require.NoError(t, err, "expected to create tag ref successfully")

	// List the tags
	listTagsRequest := mcp.CallToolRequest{}
	listTagsRequest.Params.Name = "list_tags"
	listTagsRequest.Params.Arguments = map[string]any{
		"owner": currentOwner,
		"repo":  repoName,
	}

	t.Logf("Listing tags for %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, listTagsRequest)
	require.NoError(t, err, "expected to call 'list_tags' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	require.False(t, resp.IsError, "expected result not to be an error")
	require.Len(t, resp.Content, 1, "expected content to have one item")

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedTags []struct {
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedTags)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	require.Len(t, trimmedTags, 1, "expected to find one tag")
	require.Equal(t, "v0.0.1", trimmedTags[0].Name, "expected tag name to match")
	require.Equal(t, *ref.Object.SHA, trimmedTags[0].Commit.SHA, "expected tag SHA to match")

	// And fetch an individual tag
	getTagRequest := mcp.CallToolRequest{}
	getTagRequest.Params.Name = "get_tag"
	getTagRequest.Params.Arguments = map[string]any{
		"owner": currentOwner,
		"repo":  repoName,
		"tag":   "v0.0.1",
	}

	t.Logf("Getting tag %s/%s:%s...", currentOwner, repoName, "v0.0.1")
	resp, err = mcpClient.CallTool(ctx, getTagRequest)
	require.NoError(t, err, "expected to call 'get_tag' tool successfully")
	require.False(t, resp.IsError, "expected result not to be an error")

	var trimmedTag []struct { // don't understand why this is an array
		Name   string `json:"name"`
		Commit struct {
			SHA string `json:"sha"`
		} `json:"commit"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedTag)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	require.Len(t, trimmedTag, 1, "expected to find one tag")
	require.Equal(t, "v0.0.1", trimmedTag[0].Name, "expected tag name to match")
	require.Equal(t, *ref.Object.SHA, trimmedTag[0].Commit.SHA, "expected tag SHA to match")
}
