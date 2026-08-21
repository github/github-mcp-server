package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

type CreateIssueInput struct {
	RepositoryID githubv4.ID     `json:"repositoryId"`
	Title        githubv4.String `json:"title"`

	Body          *githubv4.String                 `json:"body,omitempty"`
	AssigneeIDs   *[]githubv4.ID                   `json:"assigneeIds,omitempty"`
	MilestoneID   *githubv4.ID                     `json:"milestoneId,omitempty"`
	LabelIDs      *[]githubv4.ID                   `json:"labelIds,omitempty"`
	IssueTypeID   *githubv4.ID                     `json:"issueTypeId,omitempty"`
	ParentIssueID *githubv4.ID                     `json:"parentIssueId,omitempty"`
	IssueFields   *[]IssueFieldCreateOrUpdateInput `json:"issueFields,omitempty"`
}

type createIssueMutation struct {
	CreateIssue struct {
		Issue struct {
			FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
			URL            githubv4.URI
		}
	} `graphql:"createIssue(input: $input)"`
}

type createIssueParentMetadataQuery struct {
	Repository struct {
		ID    githubv4.ID
		Issue struct {
			ID githubv4.ID
		} `graphql:"issue(number: $parentIssueNumber)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func createIssueWithParent(
	ctx context.Context,
	client *github.Client,
	gqlClient *githubv4.Client,
	owner string,
	repo string,
	title string,
	body string,
	assignees []string,
	labels []string,
	milestoneNumber int,
	issueType string,
	issueFields []issueWriteFieldInput,
	parentIssueNumber int,
) (*mcp.CallToolResult, error) {
	if title == "" {
		return utils.NewToolResultError("missing required parameter: title"), nil
	}
	if parentIssueNumber < 1 {
		return utils.NewToolResultError("parent_issue_number must be greater than 0"), nil
	}

	repositoryID, parentIssueID, err := resolveCreateIssueParent(ctx, gqlClient, owner, repo, parentIssueNumber)
	if err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to resolve parent issue", err), nil
	}

	input := CreateIssueInput{
		RepositoryID:  repositoryID,
		Title:         githubv4.String(title),
		ParentIssueID: &parentIssueID,
	}
	if body != "" {
		input.Body = githubv4.NewString(githubv4.String(body))
	}

	if len(labels) > 0 {
		labelIDs := make([]githubv4.ID, 0, len(labels))
		for _, label := range labels {
			labelID, err := getLabelID(ctx, gqlClient, owner, repo, label)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, fmt.Sprintf("failed to resolve label %q", label), err), nil
			}
			labelIDs = append(labelIDs, labelID)
		}
		input.LabelIDs = &labelIDs
	}

	if len(assignees) > 0 {
		assigneeIDs := make([]githubv4.ID, 0, len(assignees))
		for _, assignee := range assignees {
			assigneeID, err := resolveUserID(ctx, gqlClient, assignee)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, fmt.Sprintf("failed to resolve assignee %q", assignee), err), nil
			}
			assigneeIDs = append(assigneeIDs, assigneeID)
		}
		input.AssigneeIDs = &assigneeIDs
	}

	if milestoneNumber != 0 {
		milestoneID, err := resolveMilestoneID(ctx, gqlClient, owner, repo, milestoneNumber)
		if err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to resolve milestone", err), nil
		}
		input.MilestoneID = &milestoneID
	}

	if issueType != "" {
		issueTypeID, resp, err := resolveIssueTypeID(ctx, client, owner, repo, issueType)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to resolve issue type %q", issueType), resp, err), nil
		}
		input.IssueTypeID = &issueTypeID
	}

	if len(issueFields) > 0 {
		resolvedIssueFields, err := resolveIssueFieldCreateInputs(ctx, gqlClient, owner, repo, issueFields)
		if err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to resolve issue_fields", err), nil
		}
		if len(resolvedIssueFields) > 0 {
			input.IssueFields = &resolvedIssueFields
		}
	}

	var mutation createIssueMutation
	mutationContext := ctx
	if input.IssueFields != nil {
		mutationContext = ghcontext.WithGraphQLFeatures(ctx, "issue_fields", "repo_issue_fields")
	}
	if err := gqlClient.Mutate(mutationContext, &mutation, input, nil); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to create issue", err), nil
	}

	response := MinimalResponse{
		ID:  string(mutation.CreateIssue.Issue.FullDatabaseID),
		URL: mutation.CreateIssue.Issue.URL.String(),
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil
	}
	return utils.NewToolResultText(string(encoded)), nil
}

func resolveCreateIssueParent(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, parentIssueNumber int) (githubv4.ID, githubv4.ID, error) {
	var query createIssueParentMetadataQuery
	variables := map[string]any{
		"owner":             githubv4.String(owner),
		"repo":              githubv4.String(repo),
		"parentIssueNumber": githubv4.Int(parentIssueNumber), // #nosec G115 - issue numbers are small positive integers
	}
	if err := gqlClient.Query(ctx, &query, variables); err != nil {
		return "", "", err
	}
	if query.Repository.ID == "" {
		return "", "", fmt.Errorf("repository %s/%s was not found", owner, repo)
	}
	if query.Repository.Issue.ID == "" {
		return "", "", fmt.Errorf("parent issue #%d was not found in %s/%s", parentIssueNumber, owner, repo)
	}
	return query.Repository.ID, query.Repository.Issue.ID, nil
}

func resolveUserID(ctx context.Context, gqlClient *githubv4.Client, login string) (githubv4.ID, error) {
	var query struct {
		User struct {
			ID    githubv4.ID
			Login githubv4.String
		} `graphql:"user(login: $login)"`
	}
	if err := gqlClient.Query(ctx, &query, map[string]any{"login": githubv4.String(login)}); err != nil {
		return "", err
	}
	if query.User.ID == "" {
		return "", fmt.Errorf("user %q was not found", login)
	}
	return query.User.ID, nil
}

func resolveMilestoneID(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, milestoneNumber int) (githubv4.ID, error) {
	var query struct {
		Repository struct {
			Milestone struct {
				ID githubv4.ID
			} `graphql:"milestone(number: $milestoneNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}
	variables := map[string]any{
		"owner":           githubv4.String(owner),
		"repo":            githubv4.String(repo),
		"milestoneNumber": githubv4.Int(milestoneNumber), // #nosec G115 - milestone numbers are small positive integers
	}
	if err := gqlClient.Query(ctx, &query, variables); err != nil {
		return "", err
	}
	if query.Repository.Milestone.ID == "" {
		return "", fmt.Errorf("milestone #%d was not found in %s/%s", milestoneNumber, owner, repo)
	}
	return query.Repository.Milestone.ID, nil
}

func resolveIssueTypeID(ctx context.Context, client *github.Client, owner, repo, issueTypeName string) (githubv4.ID, *github.Response, error) {
	req, err := client.NewRequest(ctx, "GET", fmt.Sprintf("repos/%s/%s/issue-types", owner, repo), nil)
	if err != nil {
		return "", nil, err
	}

	var issueTypes []*github.IssueType
	resp, err := client.Do(req, &issueTypes)
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return "", resp, err
	}
	for _, issueType := range issueTypes {
		if issueType != nil && strings.EqualFold(strings.TrimSpace(issueType.GetName()), strings.TrimSpace(issueTypeName)) {
			if issueType.GetNodeID() == "" {
				return "", resp, fmt.Errorf("issue type %q is missing a node ID", issueTypeName)
			}
			return githubv4.ID(issueType.GetNodeID()), resp, nil
		}
	}
	return "", resp, fmt.Errorf("issue type %q was not found in %s/%s", issueTypeName, owner, repo)
}

func resolveIssueFieldCreateInputs(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueFields []issueWriteFieldInput) ([]IssueFieldCreateOrUpdateInput, error) {
	fieldByName, err := fetchIssueFieldWriteMetadata(ctx, gqlClient, owner, repo)
	if err != nil {
		return nil, err
	}

	resolved := make([]IssueFieldCreateOrUpdateInput, 0, len(issueFields))
	for _, fieldInput := range issueFields {
		if fieldInput.Delete {
			continue
		}

		node, ok := fieldByName[strings.ToLower(strings.TrimSpace(fieldInput.FieldName))]
		if !ok {
			return nil, fmt.Errorf("issue field %q was not found in %s/%s", fieldInput.FieldName, owner, repo)
		}

		input, err := issueFieldCreateInput(node, fieldInput)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, input)
	}
	return resolved, nil
}

func issueFieldCreateInput(node issueFieldWriteMetadataNode, fieldInput issueWriteFieldInput) (IssueFieldCreateOrUpdateInput, error) {
	input := IssueFieldCreateOrUpdateInput{}
	var dataType string

	switch string(node.TypeName) {
	case "IssueFieldText":
		input.FieldID = node.IssueFieldText.ID
		dataType = string(node.IssueFieldText.DataType)
	case "IssueFieldNumber":
		input.FieldID = node.IssueFieldNumber.ID
		dataType = string(node.IssueFieldNumber.DataType)
	case "IssueFieldDate":
		input.FieldID = node.IssueFieldDate.ID
		dataType = string(node.IssueFieldDate.DataType)
	case "IssueFieldSingleSelect":
		input.FieldID = node.IssueFieldSingleSelect.ID
		dataType = string(node.IssueFieldSingleSelect.DataType)
	default:
		return input, fmt.Errorf("issue field %q has unsupported type %q", fieldInput.FieldName, node.TypeName)
	}
	if input.FieldID == "" {
		return input, fmt.Errorf("issue field %q is missing a node ID", fieldInput.FieldName)
	}

	switch strings.ToLower(dataType) {
	case "text":
		value := fmt.Sprint(fieldInput.Value)
		input.TextValue = githubv4.NewString(githubv4.String(value))
	case "number":
		value, err := issueFieldNumberValue(fieldInput.Value)
		if err != nil {
			return input, fmt.Errorf("issue field %q: %w", fieldInput.FieldName, err)
		}
		input.NumberValue = &value
	case "date":
		value, ok := fieldInput.Value.(string)
		if !ok {
			return input, fmt.Errorf("issue field %q requires a date string", fieldInput.FieldName)
		}
		input.DateValue = githubv4.NewString(githubv4.String(value))
	case "single_select":
		optionName := fieldInput.FieldOptionName
		if optionName == "" {
			var ok bool
			optionName, ok = fieldInput.Value.(string)
			if !ok {
				return input, fmt.Errorf("issue field %q requires a single-select option name", fieldInput.FieldName)
			}
		}
		for _, option := range node.IssueFieldSingleSelect.Options {
			if strings.EqualFold(strings.TrimSpace(string(option.Name)), strings.TrimSpace(optionName)) {
				if option.ID == "" {
					return input, fmt.Errorf("issue field option %q for field %q is missing a node ID", optionName, fieldInput.FieldName)
				}
				optionID := option.ID
				input.SingleSelectOptionID = &optionID
				return input, nil
			}
		}
		return input, fmt.Errorf("issue field option %q was not found for field %q", optionName, fieldInput.FieldName)
	default:
		return input, fmt.Errorf("issue field %q has unsupported data type %q", fieldInput.FieldName, dataType)
	}

	return input, nil
}

func issueFieldNumberValue(value any) (githubv4.Float, error) {
	switch value := value.(type) {
	case float64:
		return githubv4.Float(value), nil
	case float32:
		return githubv4.Float(value), nil
	case int:
		return githubv4.Float(value), nil
	case int8:
		return githubv4.Float(value), nil
	case int16:
		return githubv4.Float(value), nil
	case int32:
		return githubv4.Float(value), nil
	case int64:
		return githubv4.Float(value), nil
	case uint:
		return githubv4.Float(value), nil
	case uint8:
		return githubv4.Float(value), nil
	case uint16:
		return githubv4.Float(value), nil
	case uint32:
		return githubv4.Float(value), nil
	case uint64:
		return githubv4.Float(value), nil
	case json.Number:
		number, err := strconv.ParseFloat(string(value), 64)
		return githubv4.Float(number), err
	default:
		return 0, fmt.Errorf("requires a numeric value")
	}
}
