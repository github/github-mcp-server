package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/octicons"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// PullRequestAutoMergeInput identifies the pull request whose auto-merge
// setting changes, plus the optional merge settings used when enabling it.
type PullRequestAutoMergeInput struct {
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	PullNumber    int    `json:"pullNumber"`
	MergeMethod   string `json:"merge_method,omitempty"`
	CommitTitle   string `json:"commit_title,omitempty"`
	CommitMessage string `json:"commit_message,omitempty"`
}

// PullRequestAutoMergeResult is the response returned by the enable and
// disable auto-merge tools.
type PullRequestAutoMergeResult struct {
	NodeID           string `json:"node_id"`
	AutoMergeEnabled bool   `json:"auto_merge_enabled"`
	MergeMethod      string `json:"merge_method,omitempty" jsonschema:"The merge method auto-merge will use, in GitHub's lowercase form (merge, squash, or rebase); omitted when auto-merge is disabled or no method was reported."`
}

const autoMergePermissionNote = " Requires write access to the repository. Auto-merge can only be enabled when the repository allows it, and disabling requires the pull request to have auto-merge enabled."

// EnablePullRequestAutoMerge creates a tool to enable auto-merge on a pull request.
func EnablePullRequestAutoMerge(t translations.TranslationHelperFunc) inventory.ServerTool {
	return pullRequestAutoMergeTool(t, true)
}

// DisablePullRequestAutoMerge creates a tool to disable auto-merge on a pull request.
func DisablePullRequestAutoMerge(t translations.TranslationHelperFunc) inventory.ServerTool {
	return pullRequestAutoMergeTool(t, false)
}

func pullRequestAutoMergeTool(t translations.TranslationHelperFunc, enable bool) inventory.ServerTool {
	action, titleAction, icon := "disable", "Disable", "git-pull-request"
	description := "Disable auto-merge on a pull request, so it no longer merges automatically when requirements are met."
	if enable {
		action, titleAction, icon = "enable", "Enable", "git-merge"
		description = "Enable auto-merge on a pull request, so it merges automatically once all required reviews and status checks pass."
	}
	name := action + "_pull_request_auto_merge"

	properties := map[string]*jsonschema.Schema{
		"owner": {
			Type:        "string",
			Description: "Repository owner",
		},
		"repo": {
			Type:        "string",
			Description: "Repository name",
		},
		"pullNumber": {
			Type:        "number",
			Description: "Pull request number",
			Minimum:     new(1.0),
		},
	}
	if enable {
		properties["merge_method"] = &jsonschema.Schema{
			Type:        "string",
			Description: "Merge method to use when the pull request becomes mergeable. If omitted, defaults to merge. Ignored when the pull request enters a merge queue.",
			Enum:        []any{"merge", "squash", "rebase"},
		}
		properties["commit_title"] = &jsonschema.Schema{
			Type:        "string",
			Description: "Title (headline) for the merge commit. If omitted, a default message will be used. Ignored when the pull request enters a merge queue.",
		}
		properties["commit_message"] = &jsonschema.Schema{
			Type:        "string",
			Description: "Extra detail (body) for the merge commit. If omitted, a default message will be used. Ignored when the pull request enters a merge queue.",
		}
	}

	outputSchema, err := inventory.CachedSchemaFor[PullRequestAutoMergeResult](nil)
	if err != nil {
		panic(fmt.Sprintf("failed to generate pull request auto-merge output schema: %v", err))
	}

	return NewTool[PullRequestAutoMergeInput, PullRequestAutoMergeResult](
		ToolsetMetadataPullRequests,
		mcp.Tool{
			Name:        name,
			Description: t("TOOL_"+strings.ToUpper(name)+"_DESCRIPTION", description+autoMergePermissionNote),
			Icons:       octicons.Icons(icon),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_"+strings.ToUpper(name)+"_USER_TITLE", titleAction+" pull request auto-merge"),
				ReadOnlyHint:    false,
				DestructiveHint: new(false),
				OpenWorldHint:   new(true),
			},
			InputSchema: &jsonschema.Schema{
				Type:       "object",
				Properties: properties,
				Required:   []string{"owner", "repo", "pullNumber"},
			},
			OutputSchema: outputSchema,
		},
		scopes.RequireAll(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input PullRequestAutoMergeInput) (*mcp.CallToolResult, PullRequestAutoMergeResult, error) {
			result, output := setPullRequestAutoMerge(ctx, deps, input, enable)
			return result, output, nil
		},
		normalizePullRequestArguments("auto_merge"),
	)
}

func setPullRequestAutoMerge(ctx context.Context, deps ToolDependencies, input PullRequestAutoMergeInput, enable bool) (*mcp.CallToolResult, PullRequestAutoMergeResult) {
	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), PullRequestAutoMergeResult{}
	}
	pr, resp, err := client.PullRequests.Get(ctx, input.Owner, input.Repo, input.PullNumber)
	nodeID, errResult := nodeIDFromResponse(ctx, "failed to get pull request", pr.GetNodeID(), resp, err)
	if errResult != nil {
		return errResult, PullRequestAutoMergeResult{}
	}

	gqlClient, err := deps.GetGQLClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to get GitHub GraphQL client", err), PullRequestAutoMergeResult{}
	}

	output := PullRequestAutoMergeResult{NodeID: nodeID}
	if enable {
		method, err := enablePullRequestAutoMerge(ctx, gqlClient, nodeID, input)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to enable auto-merge for pull request", nil, err), PullRequestAutoMergeResult{}
		}
		output.AutoMergeEnabled = true
		output.MergeMethod = method
	} else {
		if err := disablePullRequestAutoMerge(ctx, gqlClient, nodeID); err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to disable auto-merge for pull request", nil, err), PullRequestAutoMergeResult{}
		}
	}

	r, err := json.Marshal(output)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), PullRequestAutoMergeResult{}
	}
	return utils.NewToolResultText(string(r)), output
}

// enablePullRequestAutoMerge runs the enablePullRequestAutoMerge mutation and
// returns the merge method GitHub recorded for the auto-merge request, in
// lowercase form ("" when GitHub does not report one).
func enablePullRequestAutoMerge(ctx context.Context, client *githubv4.Client, nodeID string, input PullRequestAutoMergeInput) (string, error) {
	var mutation struct {
		EnablePullRequestAutoMerge struct {
			PullRequest struct {
				AutoMergeRequest struct {
					MergeMethod githubv4.String
				}
			}
		} `graphql:"enablePullRequestAutoMerge(input: $input)"`
	}
	mutationInput := githubv4.EnablePullRequestAutoMergeInput{
		PullRequestID: githubv4.ID(nodeID),
	}
	if input.MergeMethod != "" {
		method := githubv4.PullRequestMergeMethod(strings.ToUpper(input.MergeMethod))
		mutationInput.MergeMethod = &method
	}
	if input.CommitTitle != "" {
		headline := githubv4.String(input.CommitTitle)
		mutationInput.CommitHeadline = &headline
	}
	if input.CommitMessage != "" {
		body := githubv4.String(input.CommitMessage)
		mutationInput.CommitBody = &body
	}
	if err := client.Mutate(ctx, &mutation, mutationInput, nil); err != nil {
		return "", err
	}
	return strings.ToLower(string(mutation.EnablePullRequestAutoMerge.PullRequest.AutoMergeRequest.MergeMethod)), nil
}

func disablePullRequestAutoMerge(ctx context.Context, client *githubv4.Client, nodeID string) error {
	var mutation struct {
		DisablePullRequestAutoMerge struct {
			PullRequest struct {
				ID githubv4.ID
			}
		} `graphql:"disablePullRequestAutoMerge(input: $input)"`
	}
	mutationInput := githubv4.DisablePullRequestAutoMergeInput{
		PullRequestID: githubv4.ID(nodeID),
	}
	return client.Mutate(ctx, &mutation, mutationInput, nil)
}
