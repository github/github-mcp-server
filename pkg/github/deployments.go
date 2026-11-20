package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/sanitize"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MinimalDeployment is the trimmed form of a repository deployment.
type MinimalDeployment struct {
	ID          int64  `json:"id"`
	SHA         string `json:"sha"`
	Ref         string `json:"ref"`
	Task        string `json:"task,omitempty"`
	Environment string `json:"environment"`
	Description string `json:"description,omitempty"`
	Creator     string `json:"creator,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// MinimalDeploymentStatus is the trimmed form of a deployment status.
type MinimalDeploymentStatus struct {
	ID             int64  `json:"id"`
	State          string `json:"state"`
	Description    string `json:"description,omitempty"`
	Environment    string `json:"environment,omitempty"`
	EnvironmentURL string `json:"environment_url,omitempty"`
	LogURL         string `json:"log_url,omitempty"`
	Creator        string `json:"creator,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

func formatDeploymentTimestamp(ts *github.Timestamp) string {
	if ts == nil || ts.IsZero() {
		return ""
	}
	return ts.UTC().Format(time.RFC3339)
}

func convertToMinimalDeployment(d *github.Deployment) MinimalDeployment {
	m := MinimalDeployment{
		ID:          d.GetID(),
		SHA:         d.GetSHA(),
		Ref:         d.GetRef(),
		Task:        d.GetTask(),
		Environment: d.GetEnvironment(),
		Description: sanitize.PlainText(d.GetDescription()),
		CreatedAt:   formatDeploymentTimestamp(d.CreatedAt),
		UpdatedAt:   formatDeploymentTimestamp(d.UpdatedAt),
	}
	if creator := d.GetCreator(); creator != nil {
		m.Creator = creator.GetLogin()
	}
	return m
}

func convertToMinimalDeploymentStatus(s *github.DeploymentStatus) MinimalDeploymentStatus {
	m := MinimalDeploymentStatus{
		ID:             s.GetID(),
		State:          s.GetState(),
		Description:    sanitize.PlainText(s.GetDescription()),
		Environment:    s.GetEnvironment(),
		EnvironmentURL: s.GetEnvironmentURL(),
		LogURL:         s.GetLogURL(),
		CreatedAt:      formatDeploymentTimestamp(s.CreatedAt),
		UpdatedAt:      formatDeploymentTimestamp(s.UpdatedAt),
	}
	if creator := s.GetCreator(); creator != nil {
		m.Creator = creator.GetLogin()
	}
	return m
}

// ListDeploymentsInput is the input for list_deployments.
type ListDeploymentsInput struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	SHA         string `json:"sha,omitempty"`
	Ref         string `json:"ref,omitempty"`
	Task        string `json:"task,omitempty"`
	Environment string `json:"environment,omitempty"`
	Page        *int   `json:"page,omitempty"`
	PerPage     *int   `json:"perPage,omitempty"`
}

// ListDeploymentStatusesInput is the input for list_deployment_statuses.
type ListDeploymentStatusesInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	DeploymentID int64  `json:"deployment_id"`
	Page         *int   `json:"page,omitempty"`
	PerPage      *int   `json:"perPage,omitempty"`
}

func deploymentListOptions(page, perPage *int) github.ListOptions {
	opts := github.ListOptions{Page: 1, PerPage: 30}
	if page != nil {
		opts.Page = *page
	}
	if perPage != nil {
		opts.PerPage = *perPage
	}
	return opts
}

// ListDeployments creates a tool to list a repository's deployments.
func ListDeployments(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[ListDeploymentsInput, []MinimalDeployment](
		ToolsetMetadataRepos,
		mcp.Tool{
			Name: "list_deployments",
			Description: t("TOOL_LIST_DEPLOYMENTS_DESCRIPTION",
				"List deployments in a GitHub repository, newest first. Filter by commit SHA, ref, task or environment to find which commit was deployed where. Use list_deployment_statuses to see whether a deployment succeeded."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_DEPLOYMENTS_USER_TITLE", "List deployments"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"sha": {
						Type:        "string",
						Description: "Only return deployments of this commit SHA",
					},
					"ref": {
						Type:        "string",
						Description: "Only return deployments of this ref (branch, tag or SHA name)",
					},
					"task": {
						Type:        "string",
						Description: "Only return deployments with this task name (e.g. 'deploy' or 'deploy:migrations')",
					},
					"environment": {
						Type:        "string",
						Description: "Only return deployments to this environment (e.g. 'production' or 'staging')",
					},
				},
				Required: []string{"owner", "repo"},
			}),
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input ListDeploymentsInput) (*mcp.CallToolResult, []MinimalDeployment, error) {
			if input.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if input.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.DeploymentsListOptions{
				SHA:         input.SHA,
				Ref:         input.Ref,
				Task:        input.Task,
				Environment: input.Environment,
				ListOptions: deploymentListOptions(input.Page, input.PerPage),
			}
			deployments, resp, err := client.Repositories.ListDeployments(ctx, input.Owner, input.Repo, opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list deployments", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list deployments", resp, body), nil, nil
			}

			out := make([]MinimalDeployment, 0, len(deployments))
			for _, d := range deployments {
				if d != nil {
					out = append(out, convertToMinimalDeployment(d))
				}
			}

			r, err := json.Marshal(out)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			result := utils.NewToolResultText(string(r))
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, input.Owner, input.Repo, result, ifc.LabelDeployment)
			return result, out, nil
		},
		normalizeTypedReadArguments(nil, false),
	)
}

// ListDeploymentStatuses creates a tool to list the statuses of a deployment.
func ListDeploymentStatuses(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[ListDeploymentStatusesInput, []MinimalDeploymentStatus](
		ToolsetMetadataRepos,
		mcp.Tool{
			Name: "list_deployment_statuses",
			Description: t("TOOL_LIST_DEPLOYMENT_STATUSES_DESCRIPTION",
				"List the statuses of a deployment in a GitHub repository, newest first. The first status on page 1 is the deployment's current state (e.g. success, failure, error, in_progress, queued, pending or inactive); later pages hold older history."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_DEPLOYMENT_STATUSES_USER_TITLE", "List deployment statuses"),
				ReadOnlyHint: true,
			},
			InputSchema: WithPagination(&jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"deployment_id": {
						Type:        "number",
						Description: "The deployment ID, as returned by list_deployments",
					},
				},
				Required: []string{"owner", "repo", "deployment_id"},
			}),
		},
		scopes.PublicRead(scopes.Repo),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input ListDeploymentStatusesInput) (*mcp.CallToolResult, []MinimalDeploymentStatus, error) {
			if input.Owner == "" {
				return utils.NewToolResultError("missing required parameter: owner"), nil, nil
			}
			if input.Repo == "" {
				return utils.NewToolResultError("missing required parameter: repo"), nil, nil
			}
			if input.DeploymentID <= 0 {
				return utils.NewToolResultError("missing required parameter: deployment_id"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := deploymentListOptions(input.Page, input.PerPage)
			statuses, resp, err := client.Repositories.ListDeploymentStatuses(ctx, input.Owner, input.Repo, input.DeploymentID, &opts)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list deployment statuses", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list deployment statuses", resp, body), nil, nil
			}

			out := make([]MinimalDeploymentStatus, 0, len(statuses))
			for _, s := range statuses {
				if s != nil {
					out = append(out, convertToMinimalDeploymentStatus(s))
				}
			}

			r, err := json.Marshal(out)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			result := utils.NewToolResultText(string(r))
			result = attachRepoVisibilityIFCLabel(ctx, deps, client, input.Owner, input.Repo, result, ifc.LabelDeployment)
			return result, out, nil
		},
		normalizeTypedReadArguments(nil, false),
	)
}
