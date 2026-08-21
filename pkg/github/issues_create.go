package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

type CreateIssueInput struct {
	RepositoryID githubv4.ID     `json:"repositoryId"`
	Title        githubv4.String `json:"title"`

	Body          *githubv4.String `json:"body,omitempty"`
	AssigneeIDs   *[]githubv4.ID   `json:"assigneeIds,omitempty"`
	MilestoneID   *githubv4.ID     `json:"milestoneId,omitempty"`
	LabelIDs      *[]githubv4.ID   `json:"labelIds,omitempty"`
	IssueTypeID   *githubv4.ID     `json:"issueTypeId,omitempty"`
	ParentIssueID *githubv4.ID     `json:"parentIssueId,omitempty"`
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
	ChildRepository struct {
		ID githubv4.ID
	} `graphql:"childRepository: repository(owner: $owner, name: $repo)"`
	ParentRepository struct {
		Issue struct {
			ID githubv4.ID
		} `graphql:"issue(number: $parentIssueNumber)"`
	} `graphql:"parentRepository: repository(owner: $parentOwner, name: $parentRepo)"`
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
	parentIssueNumber int,
	parentOwner string,
	parentRepo string,
) (*mcp.CallToolResult, error) {
	if title == "" {
		return utils.NewToolResultError("missing required parameter: title"), nil
	}
	if parentIssueNumber < 1 {
		return utils.NewToolResultError("parent_issue_number must be greater than 0"), nil
	}

	parentOwner, parentRepo = parentRepository(owner, repo, parentOwner, parentRepo)
	repositoryID, parentIssueID, err := resolveCreateIssueParent(ctx, gqlClient, owner, repo, parentOwner, parentRepo, parentIssueNumber)
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

	var mutation createIssueMutation
	if err := gqlClient.Mutate(ctx, &mutation, input, nil); err != nil {
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

func parentRepository(owner, repo, parentOwner, parentRepo string) (string, string) {
	if parentOwner == "" && parentRepo == "" {
		return owner, repo
	}
	return parentOwner, parentRepo
}

func validateParentRepository(parentProvided bool, parentOwner, parentRepo string) error {
	if !parentProvided {
		if parentOwner != "" || parentRepo != "" {
			return errors.New("parent_owner and parent_repo can only be used when parent_issue_number is provided")
		}
		return nil
	}
	if (parentOwner == "") != (parentRepo == "") {
		return errors.New("parent_owner and parent_repo must be provided together")
	}
	return nil
}

func resolveCreateIssueParent(ctx context.Context, gqlClient *githubv4.Client, owner, repo, parentOwner, parentRepo string, parentIssueNumber int) (githubv4.ID, githubv4.ID, error) {
	var query createIssueParentMetadataQuery
	variables := map[string]any{
		"owner":             githubv4.String(owner),
		"repo":              githubv4.String(repo),
		"parentOwner":       githubv4.String(parentOwner),
		"parentRepo":        githubv4.String(parentRepo),
		"parentIssueNumber": githubv4.Int(parentIssueNumber), // #nosec G115 - issue numbers are small positive integers
	}
	if err := gqlClient.Query(ctx, &query, variables); err != nil {
		return "", "", err
	}
	if query.ChildRepository.ID == "" {
		return "", "", fmt.Errorf("repository %s/%s was not found", owner, repo)
	}
	if query.ParentRepository.Issue.ID == "" {
		return "", "", fmt.Errorf("parent issue #%d was not found in %s/%s", parentIssueNumber, parentOwner, parentRepo)
	}
	return query.ChildRepository.ID, query.ParentRepository.Issue.ID, nil
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
