package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/ifc"
	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/translations"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

var (
	issueStatusNames = []string{"Backlog", "Todo", "In progress", "In review", "Done", "Not planned", "Duplicate"}
	// Issues can only be created with an open status.
	issueCreationStatusNames = issueStatusNames[:4:4]
	issueStatusCategories    = []string{"backlog", "planned", "started", "completed", "not_planned", "duplicate"}
	issueClosingStatusNames  = issueStatusNames[4:]
)

type IssuePlanningStatus struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

// issueWithStatus decodes the REST status field, which go-github does not yet expose.
// The status stays raw so an unexpected shape cannot fail decoding the issue.
type issueWithStatus struct {
	github.Issue
	RawStatus json.RawMessage `json:"status"`
}

// planningStatus returns nil when the issue has no status or an unrecognized one,
// so unexpected API values never fail output schema validation.
func (issue *issueWithStatus) planningStatus() *IssuePlanningStatus {
	var status *IssuePlanningStatus
	if json.Unmarshal(issue.RawStatus, &status) != nil || status == nil {
		return nil
	}
	if !slices.Contains(issueStatusNames, status.Name) || !slices.Contains(issueStatusCategories, status.Category) {
		return nil
	}
	return status
}

func enumValues(values []string) []any {
	enum := make([]any, len(values))
	for i, value := range values {
		enum[i] = value
	}
	return enum
}

type IssueStatusUpdateOutput struct {
	ID     string               `json:"id"`
	URL    string               `json:"url"`
	Status *IssuePlanningStatus `json:"status"`
}

type IssueStatusInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	IssueNumber  int    `json:"issue_number"`
	Status       string `json:"status"`
	DuplicateOf  int    `json:"duplicate_of,omitempty"`
	ConfirmClose bool   `json:"confirm_close,omitempty"`
}

func issuePlanningStatusSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Types: []string{"object", "null"},
		Description: "The issue's planning status, or null when it has none or it is not recognized. " +
			"Backlog is backlog; Todo is planned; In progress and In review are started; " +
			"Done is completed; Not planned is not_planned; Duplicate is duplicate.",
		Properties: map[string]*jsonschema.Schema{
			"id":       {Type: "integer", Description: "The status ID."},
			"name":     {Type: "string", Enum: enumValues(issueStatusNames)},
			"category": {Type: "string", Enum: enumValues(issueStatusCategories)},
		},
		Required:             []string{"id", "name", "category"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}

func canonicalIssueStatus(status string, allowed []string) (string, error) {
	for _, name := range allowed {
		if strings.EqualFold(status, name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("status must be one of: %s", strings.Join(allowed, ", "))
}

func normalizeIssueCreationStatus(next inventory.InputNormalizer, consolidated bool) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		if _, provided := args["status"]; provided {
			if consolidated && args["method"] != "create" {
				return nil, fmt.Errorf("status is only supported with the create method; use update_issue_status to update it")
			}
			status, err := RequiredParam[string](args, "status")
			if err != nil {
				return nil, err
			}
			status, err = canonicalIssueStatus(status, issueCreationStatusNames)
			if err != nil {
				return nil, err
			}
			for _, field := range []string{"state", "state_reason", "duplicate_of"} {
				if _, provided := args[field]; provided {
					return nil, fmt.Errorf("status cannot be combined with %s when creating an issue", field)
				}
			}
			args["status"] = status
			raw, err = json.Marshal(args)
			if err != nil {
				return nil, err
			}
		}
		return next(raw)
	}
}

func normalizeIssueStatusArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	for _, field := range []string{"owner", "repo", "status"} {
		if _, err := RequiredParam[string](args, field); err != nil {
			return nil, err
		}
	}
	number, err := RequiredInt(args, "issue_number")
	if err != nil {
		return nil, err
	}
	if number < 1 {
		return nil, fmt.Errorf("issue_number must be positive")
	}
	status, err := canonicalIssueStatus(args["status"].(string), issueStatusNames)
	if err != nil {
		return nil, err
	}
	duplicate := 0
	if _, provided := args["duplicate_of"]; provided {
		duplicate, err = OptionalIntParam(args, "duplicate_of")
		if err != nil {
			return nil, err
		}
		if duplicate < 1 {
			return nil, fmt.Errorf("duplicate_of must be positive")
		}
		if status != "Duplicate" {
			return nil, fmt.Errorf("duplicate_of can only be used when status is 'Duplicate'")
		}
		args["duplicate_of"] = duplicate
	}
	if status == "Duplicate" && duplicate == 0 {
		return nil, fmt.Errorf("duplicate_of is required when status is 'Duplicate'")
	}
	if _, err := OptionalParam[bool](args, "confirm_close"); err != nil {
		return nil, err
	}
	args["issue_number"], args["status"] = number, status
	return json.Marshal(args)
}

// UpdateIssueStatus changes an issue's planning status.
func UpdateIssueStatus(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool[IssueStatusInput, *IssueStatusUpdateOutput](
		ToolsetMetadataIssues,
		mcp.Tool{
			Name: "update_issue_status",
			Description: t("TOOL_UPDATE_ISSUE_STATUS_DESCRIPTION", "Update an issue's planning status. "+
				"Done, Not planned and Duplicate close the issue; the other statuses keep it open or reopen it. "+
				"Closing statuses require confirm_close: true, set only after the user confirms closing the issue. "+
				"Duplicate requires duplicate_of, the canonical issue number in the same repository. A status cannot be removed."),
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner":        {Type: "string", Description: "Repository owner (username or organization)"},
					"repo":         {Type: "string", Description: "Repository name"},
					"issue_number": {Type: "number", Minimum: new(1.0), Description: "The issue number to update"},
					"status":       {Type: "string", Enum: enumValues(issueStatusNames), Description: "The planning status to assign."},
					"duplicate_of": {Type: "number", Minimum: new(1.0), Description: "The canonical issue number. Required only for Duplicate."},
					"confirm_close": {
						Type:        "boolean",
						Description: "Must be true for Done, Not planned or Duplicate. Set only after the user confirms closing the issue.",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "status"},
			},
			OutputSchema: issueStatusUpdateOutputSchema(),
			Annotations: &mcp.ToolAnnotations{
				Title:           t("TOOL_UPDATE_ISSUE_STATUS_USER_TITLE", "Update Issue Status"),
				ReadOnlyHint:    false,
				DestructiveHint: new(true),
				OpenWorldHint:   new(true),
			},
		},
		publicRepositoryWriteScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args IssueStatusInput) (*mcp.CallToolResult, *IssueStatusUpdateOutput, error) {
			// This is a caller-supplied confirmation policy, not proof that a trusted UI prompted the user.
			if slices.Contains(issueClosingStatusNames, args.Status) && !args.ConfirmClose {
				return utils.NewToolResultError("confirm_close must be true to use a status that closes the issue"), nil, nil
			}
			repoPath, err := repositoryAPIPath(args.Owner, args.Repo)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			body := struct {
				Status           string `json:"status"`
				DuplicateIssueID *int64 `json:"duplicate_issue_id,omitempty"`
			}{Status: args.Status}
			if args.DuplicateOf != 0 {
				duplicate, resp, err := client.Issues.Get(ctx, args.Owner, args.Repo, args.DuplicateOf)
				if resp != nil {
					defer func() { _ = resp.Body.Close() }()
				}
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get duplicate issue", resp, err), nil, nil
				}
				body.DuplicateIssueID = duplicate.ID
				if body.DuplicateIssueID == nil || *body.DuplicateIssueID < 1 {
					return utils.NewToolResultError("failed to get duplicate issue: response did not include a valid issue ID"), nil, nil
				}
			}
			req, err := client.NewRequest(ctx, http.MethodPatch, fmt.Sprintf("%s/issues/%d", repoPath, args.IssueNumber), body)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create status update request", err), nil, nil
			}
			var issue issueWithStatus
			resp, err := client.Do(req, &issue)
			if resp != nil {
				defer func() { _ = resp.Body.Close() }()
			}
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update issue", resp, err), nil, nil
			}
			status := issue.planningStatus()
			if status == nil || status.Name != args.Status {
				return utils.NewToolResultError(fmt.Sprintf("requested status %q was not applied; issue statuses may be unavailable in this repository", args.Status)), nil, nil
			}
			output := &IssueStatusUpdateOutput{ID: fmt.Sprintf("%d", issue.GetID()), URL: issue.GetHTMLURL(), Status: status}
			result := attachRepoVisibilityIFCLabel(ctx, deps, client, args.Owner, args.Repo, MarshalledTextResult(output), ifc.LabelRepoMetadata)
			return result, output, nil
		},
		granularIssueInputNormalizer(normalizeIssueStatusArguments),
	)
}

func issueCreationStatusProperty() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string", Enum: enumValues(issueCreationStatusNames),
		Description: "Initial planning status (create only). Only open statuses are supported. " +
			"Backlog is backlog; Todo is planned; In progress and In review are started. " +
			"Omit to use the default status, Backlog.",
	}
}

// repositoryAPIPath rejects coordinates that are not single path segments and
// escapes them, so hand-built REST URLs cannot target a different API path.
func repositoryAPIPath(owner, repo string) (string, error) {
	for _, segment := range []string{owner, repo} {
		if strings.TrimSpace(segment) == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, "/\\") {
			return "", fmt.Errorf("owner and repo must be nonempty repository path segments")
		}
	}
	return fmt.Sprintf("repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo)), nil
}

// createRESTIssue preserves the standard request fields while adding the API's
// status field, which go-github does not yet expose.
func createRESTIssue(ctx context.Context, client *github.Client, owner, repo string, body github.CreateIssueRequest, status string) (*issueWithStatus, *github.Response, error) {
	if status == "" {
		issue, resp, err := client.Issues.Create(ctx, owner, repo, body)
		if err != nil {
			return nil, resp, err
		}
		if issue == nil {
			issue = &github.Issue{}
		}
		return &issueWithStatus{Issue: *issue}, resp, nil
	}
	repoPath, err := repositoryAPIPath(owner, repo)
	if err != nil {
		return nil, nil, err
	}
	requestBody := struct {
		github.CreateIssueRequest
		Status string `json:"status"`
	}{body, status}
	req, err := client.NewRequest(ctx, http.MethodPost, repoPath+"/issues", requestBody)
	if err != nil {
		return nil, nil, err
	}
	var issue issueWithStatus
	resp, err := client.Do(req, &issue)
	return &issue, resp, err
}

// unappliedIssueStatusError reports a created issue whose status differs from
// the requested one, so callers can fix that issue instead of creating another.
func unappliedIssueStatusError(requested string, issue *issueWithStatus) error {
	if requested == "" {
		return nil
	}
	applied := ""
	if status := issue.planningStatus(); status != nil {
		applied = status.Name
	}
	if applied == requested {
		return nil
	}
	return fmt.Errorf("requested=%q, applied=%q, issue_id=%d, issue_url=%q; issue statuses may be unavailable in this repository; use update_issue_status to correct the issue instead of creating it again",
		requested, applied, issue.GetID(), issue.GetHTMLURL())
}

func resolveIssueStatusID(ctx context.Context, client *githubv4.Client, owner, repo, status string) (githubv4.ID, error) {
	var query struct {
		Repository struct {
			PlanningStatuses struct {
				Nodes []struct {
					ID   githubv4.ID
					Name string
				}
			} `graphql:"planningStatuses(first: 10)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	if err := client.Query(ctx, &query, map[string]any{"owner": githubv4.String(owner), "repo": githubv4.String(repo)}); err != nil {
		return nil, err
	}
	for _, node := range query.Repository.PlanningStatuses.Nodes {
		if node.Name == status && node.ID != nil {
			return node.ID, nil
		}
	}
	return nil, fmt.Errorf("planning status %q is unavailable in %s/%s", status, owner, repo)
}

func issueStatusUpdateOutputSchema() *jsonschema.Schema {
	schema := issueDTOschema[IssueStatusUpdateOutput]()
	schema.Properties["status"] = issuePlanningStatusSchema()
	return schema
}
