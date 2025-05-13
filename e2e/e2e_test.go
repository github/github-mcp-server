//go:build e2e

package e2e_test

import (
	"context"
	"encoding/base64"
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

func TestFileDeletion(t *testing.T) {
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

	// Create a branch on which to create a new commit
	createBranchRequest := mcp.CallToolRequest{}
	createBranchRequest.Params.Name = "create_branch"
	createBranchRequest.Params.Arguments = map[string]any{
		"owner":       currentOwner,
		"repo":        repoName,
		"branch":      "test-branch",
		"from_branch": "main",
	}

	t.Logf("Creating branch in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createBranchRequest)
	require.NoError(t, err, "expected to call 'create_branch' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a commit with a new file
	commitRequest := mcp.CallToolRequest{}
	commitRequest.Params.Name = "create_or_update_file"
	commitRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-file.txt",
		"content": fmt.Sprintf("Created by e2e test %s", t.Name()),
		"message": "Add test file",
		"branch":  "test-branch",
	}

	t.Logf("Creating commit with new file in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, commitRequest)
	require.NoError(t, err, "expected to call 'create_or_update_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedCommitText struct {
		SHA string `json:"sha"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	// Check the file exists
	getFileContentsRequest := mcp.CallToolRequest{}
	getFileContentsRequest.Params.Name = "get_file_contents"
	getFileContentsRequest.Params.Arguments = map[string]any{
		"owner":  currentOwner,
		"repo":   repoName,
		"path":   "test-file.txt",
		"branch": "test-branch",
	}

	t.Logf("Getting file contents in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, getFileContentsRequest)
	require.NoError(t, err, "expected to call 'get_file_contents' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedGetFileText struct {
		Content string `json:"content"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedGetFileText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	b, err := base64.StdEncoding.DecodeString(trimmedGetFileText.Content)
	require.NoError(t, err, "expected to decode base64 content successfully")
	require.Equal(t, fmt.Sprintf("Created by e2e test %s", t.Name()), string(b), "expected file content to match")

	// Delete the file
	deleteFileRequest := mcp.CallToolRequest{}
	deleteFileRequest.Params.Name = "delete_file"
	deleteFileRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-file.txt",
		"message": "Delete test file",
		"branch":  "test-branch",
	}

	t.Logf("Deleting file in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, deleteFileRequest)
	require.NoError(t, err, "expected to call 'delete_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// See that there is a commit that removes the file
	listCommitsRequest := mcp.CallToolRequest{}
	listCommitsRequest.Params.Name = "list_commits"
	listCommitsRequest.Params.Arguments = map[string]any{
		"owner": currentOwner,
		"repo":  repoName,
		"sha":   "test-branch", // can be SHA or branch, which is an unfortunate API design
	}

	t.Logf("Listing commits in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, listCommitsRequest)
	require.NoError(t, err, "expected to call 'list_commits' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedListCommitsText []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
		}
		Files []struct {
			Filename  string `json:"filename"`
			Deletions int    `json:"deletions"`
		}
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedListCommitsText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	require.GreaterOrEqual(t, len(trimmedListCommitsText), 1, "expected to find at least one commit")

	deletionCommit := trimmedListCommitsText[0]
	require.Equal(t, "Delete test file", deletionCommit.Commit.Message, "expected commit message to match")

	// Now get the commit so we can look at the file changes because list_commits doesn't include them
	getCommitRequest := mcp.CallToolRequest{}
	getCommitRequest.Params.Name = "get_commit"
	getCommitRequest.Params.Arguments = map[string]any{
		"owner": currentOwner,
		"repo":  repoName,
		"sha":   deletionCommit.SHA,
	}

	t.Logf("Getting commit %s/%s:%s...", currentOwner, repoName, deletionCommit.SHA)
	resp, err = mcpClient.CallTool(ctx, getCommitRequest)
	require.NoError(t, err, "expected to call 'get_commit' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedGetCommitText struct {
		Files []struct {
			Filename  string `json:"filename"`
			Deletions int    `json:"deletions"`
		}
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedGetCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	require.Len(t, trimmedGetCommitText.Files, 1, "expected to find one file change")
	require.Equal(t, "test-file.txt", trimmedGetCommitText.Files[0].Filename, "expected filename to match")
	require.Equal(t, 1, trimmedGetCommitText.Files[0].Deletions, "expected one deletion")
}

func TestDirectoryDeletion(t *testing.T) {
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

	// Create a branch on which to create a new commit
	createBranchRequest := mcp.CallToolRequest{}
	createBranchRequest.Params.Name = "create_branch"
	createBranchRequest.Params.Arguments = map[string]any{
		"owner":       currentOwner,
		"repo":        repoName,
		"branch":      "test-branch",
		"from_branch": "main",
	}

	t.Logf("Creating branch in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createBranchRequest)
	require.NoError(t, err, "expected to call 'create_branch' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a commit with a new file
	commitRequest := mcp.CallToolRequest{}
	commitRequest.Params.Name = "create_or_update_file"
	commitRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-dir/test-file.txt",
		"content": fmt.Sprintf("Created by e2e test %s", t.Name()),
		"message": "Add test file",
		"branch":  "test-branch",
	}

	t.Logf("Creating commit with new file in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, commitRequest)
	require.NoError(t, err, "expected to call 'create_or_update_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedCommitText struct {
		SHA string `json:"sha"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	// Check the file exists
	getFileContentsRequest := mcp.CallToolRequest{}
	getFileContentsRequest.Params.Name = "get_file_contents"
	getFileContentsRequest.Params.Arguments = map[string]any{
		"owner":  currentOwner,
		"repo":   repoName,
		"path":   "test-dir/test-file.txt",
		"branch": "test-branch",
	}

	t.Logf("Getting file contents in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, getFileContentsRequest)
	require.NoError(t, err, "expected to call 'get_file_contents' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedGetFileText struct {
		Content string `json:"content"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedGetFileText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	b, err := base64.StdEncoding.DecodeString(trimmedGetFileText.Content)
	require.NoError(t, err, "expected to decode base64 content successfully")
	require.Equal(t, fmt.Sprintf("Created by e2e test %s", t.Name()), string(b), "expected file content to match")

	// Delete the directory containing the file
	deleteFileRequest := mcp.CallToolRequest{}
	deleteFileRequest.Params.Name = "delete_file"
	deleteFileRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-dir",
		"message": "Delete test directory",
		"branch":  "test-branch",
	}

	t.Logf("Deleting directory in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, deleteFileRequest)
	require.NoError(t, err, "expected to call 'delete_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// See that there is a commit that removes the directory
	listCommitsRequest := mcp.CallToolRequest{}
	listCommitsRequest.Params.Name = "list_commits"
	listCommitsRequest.Params.Arguments = map[string]any{
		"owner": currentOwner,
		"repo":  repoName,
		"sha":   "test-branch", // can be SHA or branch, which is an unfortunate API design
	}

	t.Logf("Listing commits in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, listCommitsRequest)
	require.NoError(t, err, "expected to call 'list_commits' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedListCommitsText []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
		}
		Files []struct {
			Filename  string `json:"filename"`
			Deletions int    `json:"deletions"`
		} `json:"files"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedListCommitsText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	require.GreaterOrEqual(t, len(trimmedListCommitsText), 1, "expected to find at least one commit")

	deletionCommit := trimmedListCommitsText[0]
	require.Equal(t, "Delete test directory", deletionCommit.Commit.Message, "expected commit message to match")

	// Now get the commit so we can look at the file changes because list_commits doesn't include them
	getCommitRequest := mcp.CallToolRequest{}
	getCommitRequest.Params.Name = "get_commit"
	getCommitRequest.Params.Arguments = map[string]any{
		"owner": currentOwner,
		"repo":  repoName,
		"sha":   deletionCommit.SHA,
	}

	t.Logf("Getting commit %s/%s:%s...", currentOwner, repoName, deletionCommit.SHA)
	resp, err = mcpClient.CallTool(ctx, getCommitRequest)
	require.NoError(t, err, "expected to call 'get_commit' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedGetCommitText struct {
		Files []struct {
			Filename  string `json:"filename"`
			Deletions int    `json:"deletions"`
		}
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedGetCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	require.Len(t, trimmedGetCommitText.Files, 1, "expected to find one file change")
	require.Equal(t, "test-dir/test-file.txt", trimmedGetCommitText.Files[0].Filename, "expected filename to match")
	require.Equal(t, 1, trimmedGetCommitText.Files[0].Deletions, "expected one deletion")
}

func TestPullRequestAtomicCreateAndSubmit(t *testing.T) {
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

	// Create a branch on which to create a new commit
	createBranchRequest := mcp.CallToolRequest{}
	createBranchRequest.Params.Name = "create_branch"
	createBranchRequest.Params.Arguments = map[string]any{
		"owner":       currentOwner,
		"repo":        repoName,
		"branch":      "test-branch",
		"from_branch": "main",
	}

	t.Logf("Creating branch in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createBranchRequest)
	require.NoError(t, err, "expected to call 'create_branch' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a commit with a new file
	commitRequest := mcp.CallToolRequest{}
	commitRequest.Params.Name = "create_or_update_file"
	commitRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-file.txt",
		"content": fmt.Sprintf("Created by e2e test %s", t.Name()),
		"message": "Add test file",
		"branch":  "test-branch",
	}

	t.Logf("Creating commit with new file in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, commitRequest)
	require.NoError(t, err, "expected to call 'create_or_update_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedCommitText struct {
		SHA string `json:"sha"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	commitId := trimmedCommitText.SHA

	// Create a pull request
	prRequest := mcp.CallToolRequest{}
	prRequest.Params.Name = "create_pull_request"
	prRequest.Params.Arguments = map[string]any{
		"owner":    currentOwner,
		"repo":     repoName,
		"title":    "Test PR",
		"body":     "This is a test PR",
		"head":     "test-branch",
		"base":     "main",
		"commitId": commitId,
	}

	t.Logf("Creating pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, prRequest)
	require.NoError(t, err, "expected to call 'create_pull_request' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create and submit a review
	createAndSubmitReviewRequest := mcp.CallToolRequest{}
	createAndSubmitReviewRequest.Params.Name = "create_and_submit_pull_request_review"
	createAndSubmitReviewRequest.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
		"event":      "COMMENT", // the only event we can use as the creator of the PR
		"body":       "Looks good if you like bad code I guess!",
		"commitId":   commitId,
	}

	t.Logf("Creating and submitting review for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createAndSubmitReviewRequest)
	require.NoError(t, err, "expected to call 'create_and_submit_pull_request_review' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Finally, get the list of reviews and see that our review has been submitted
	getPullRequestsReview := mcp.CallToolRequest{}
	getPullRequestsReview.Params.Name = "get_pull_request_reviews"
	getPullRequestsReview.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
	}

	t.Logf("Getting reviews for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, getPullRequestsReview)
	require.NoError(t, err, "expected to call 'get_pull_request_reviews' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var reviews []struct {
		State string `json:"state"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &reviews)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	// Check that there is one review
	require.Len(t, reviews, 1, "expected to find one review")
	require.Equal(t, "COMMENTED", reviews[0].State, "expected review state to be COMMENTED")
}

func TestPullRequestReviewCommentSubmit(t *testing.T) {
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

	// Create a branch on which to create a new commit
	createBranchRequest := mcp.CallToolRequest{}
	createBranchRequest.Params.Name = "create_branch"
	createBranchRequest.Params.Arguments = map[string]any{
		"owner":       currentOwner,
		"repo":        repoName,
		"branch":      "test-branch",
		"from_branch": "main",
	}

	t.Logf("Creating branch in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createBranchRequest)
	require.NoError(t, err, "expected to call 'create_branch' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a commit with a new file
	commitRequest := mcp.CallToolRequest{}
	commitRequest.Params.Name = "create_or_update_file"
	commitRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-file.txt",
		"content": fmt.Sprintf("Created by e2e test %s", t.Name()),
		"message": "Add test file",
		"branch":  "test-branch",
	}

	t.Logf("Creating commit with new file in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, commitRequest)
	require.NoError(t, err, "expected to call 'create_or_update_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedCommitText struct {
		SHA string `json:"sha"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	commitId := trimmedCommitText.SHA

	// Create a pull request
	prRequest := mcp.CallToolRequest{}
	prRequest.Params.Name = "create_pull_request"
	prRequest.Params.Arguments = map[string]any{
		"owner":    currentOwner,
		"repo":     repoName,
		"title":    "Test PR",
		"body":     "This is a test PR",
		"head":     "test-branch",
		"base":     "main",
		"commitId": commitId,
	}

	t.Logf("Creating pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, prRequest)
	require.NoError(t, err, "expected to call 'create_pull_request' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a review for the pull request, but we can't approve it
	// because the current owner also owns the PR.
	createPendingPullRequestReviewRequest := mcp.CallToolRequest{}
	createPendingPullRequestReviewRequest.Params.Name = "create_pending_pull_request_review"
	createPendingPullRequestReviewRequest.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
	}

	t.Logf("Creating pending review for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createPendingPullRequestReviewRequest)
	require.NoError(t, err, "expected to call 'create_pending_pull_request_review' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")
	require.Equal(t, "", textContent.Text, "expected content to be empty")

	// Add a review comment
	addReviewCommentRequest := mcp.CallToolRequest{}
	addReviewCommentRequest.Params.Name = "add_pull_request_review_comment_to_pending_review"
	addReviewCommentRequest.Params.Arguments = map[string]any{
		"owner":       currentOwner,
		"repo":        repoName,
		"pullNumber":  1,
		"path":        "test-file.txt",
		"subjectType": "LINE",
		"body":        "Very nice!",
		"line":        1,
	}

	t.Logf("Adding review comment to pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, addReviewCommentRequest)
	require.NoError(t, err, "expected to call 'add_pull_request_review_comment_to_pending_review' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Submit the review
	submitReviewRequest := mcp.CallToolRequest{}
	submitReviewRequest.Params.Name = "submit_pending_pull_request_review"
	submitReviewRequest.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
		"event":      "COMMENT", // the only event we can use as the creator of the PR
		"body":       "Looks good if you like bad code I guess!",
	}

	t.Logf("Submitting review for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, submitReviewRequest)
	require.NoError(t, err, "expected to call 'submit_pending_pull_request_review' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Finally, get the review and see that it has been created
	getPullRequestsReview := mcp.CallToolRequest{}
	getPullRequestsReview.Params.Name = "get_pull_request_reviews"
	getPullRequestsReview.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
	}

	t.Logf("Getting reviews for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, getPullRequestsReview)
	require.NoError(t, err, "expected to call 'get_pull_request_reviews' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var reviews []struct {
		State string `json:"state"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &reviews)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	// Check that there is one review
	require.Len(t, reviews, 1, "expected to find one review")
	require.Equal(t, "COMMENTED", reviews[0].State, "expected review state to be COMMENTED")
}

func TestPullRequestReviewDeletion(t *testing.T) {
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

	// Create a branch on which to create a new commit
	createBranchRequest := mcp.CallToolRequest{}
	createBranchRequest.Params.Name = "create_branch"
	createBranchRequest.Params.Arguments = map[string]any{
		"owner":       currentOwner,
		"repo":        repoName,
		"branch":      "test-branch",
		"from_branch": "main",
	}

	t.Logf("Creating branch in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createBranchRequest)
	require.NoError(t, err, "expected to call 'create_branch' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a commit with a new file
	commitRequest := mcp.CallToolRequest{}
	commitRequest.Params.Name = "create_or_update_file"
	commitRequest.Params.Arguments = map[string]any{
		"owner":   currentOwner,
		"repo":    repoName,
		"path":    "test-file.txt",
		"content": fmt.Sprintf("Created by e2e test %s", t.Name()),
		"message": "Add test file",
		"branch":  "test-branch",
	}

	t.Logf("Creating commit with new file in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, commitRequest)
	require.NoError(t, err, "expected to call 'create_or_update_file' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var trimmedCommitText struct {
		SHA string `json:"sha"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &trimmedCommitText)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	commitId := trimmedCommitText.SHA

	// Create a pull request
	prRequest := mcp.CallToolRequest{}
	prRequest.Params.Name = "create_pull_request"
	prRequest.Params.Arguments = map[string]any{
		"owner":    currentOwner,
		"repo":     repoName,
		"title":    "Test PR",
		"body":     "This is a test PR",
		"head":     "test-branch",
		"base":     "main",
		"commitId": commitId,
	}

	t.Logf("Creating pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, prRequest)
	require.NoError(t, err, "expected to call 'create_pull_request' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// Create a review for the pull request, but we can't approve it
	// because the current owner also owns the PR.
	createPendingPullRequestReviewRequest := mcp.CallToolRequest{}
	createPendingPullRequestReviewRequest.Params.Name = "create_pending_pull_request_review"
	createPendingPullRequestReviewRequest.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
	}

	t.Logf("Creating pending review for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, createPendingPullRequestReviewRequest)
	require.NoError(t, err, "expected to call 'create_pending_pull_request_review' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")
	require.Equal(t, "", textContent.Text, "expected content to be empty")

	// See that there is a pending review
	getPullRequestsReview := mcp.CallToolRequest{}
	getPullRequestsReview.Params.Name = "get_pull_request_reviews"
	getPullRequestsReview.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
	}

	t.Logf("Getting reviews for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, getPullRequestsReview)
	require.NoError(t, err, "expected to call 'get_pull_request_reviews' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var reviews []struct {
		State string `json:"state"`
	}
	err = json.Unmarshal([]byte(textContent.Text), &reviews)
	require.NoError(t, err, "expected to unmarshal text content successfully")

	// Check that there is one review
	require.Len(t, reviews, 1, "expected to find one review")
	require.Equal(t, "PENDING", reviews[0].State, "expected review state to be PENDING")

	// Delete the review
	deleteReviewRequest := mcp.CallToolRequest{}
	deleteReviewRequest.Params.Name = "delete_pending_pull_request_review"
	deleteReviewRequest.Params.Arguments = map[string]any{
		"owner":      currentOwner,
		"repo":       repoName,
		"pullNumber": 1,
	}

	t.Logf("Deleting review for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, deleteReviewRequest)
	require.NoError(t, err, "expected to call 'delete_pending_pull_request_review' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	// See that there are no reviews
	t.Logf("Getting reviews for pull request in %s/%s...", currentOwner, repoName)
	resp, err = mcpClient.CallTool(ctx, getPullRequestsReview)
	require.NoError(t, err, "expected to call 'get_pull_request_reviews' tool successfully")
	require.False(t, resp.IsError, fmt.Sprintf("expected result not to be an error: %+v", resp))

	textContent, ok = resp.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")

	var noReviews []struct{}
	err = json.Unmarshal([]byte(textContent.Text), &noReviews)
	require.NoError(t, err, "expected to unmarshal text content successfully")
	require.Len(t, noReviews, 0, "expected to find no reviews")

}
