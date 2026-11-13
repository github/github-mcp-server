package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	ghErrors "github.com/github/github-mcp-server/v2/pkg/errors"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/google/go-github/v92/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// issueMetadataScope is the repository or account whose issue types and
// issue fields a lookup lists.
type issueMetadataScope struct {
	Kind  issueMetadataScopeKind
	Owner string
	Repo  string
}

type issueMetadataScopeKind int

const (
	issueMetadataRepo issueMetadataScopeKind = iota
	issueMetadataOrg
	// issueMetadataViewer is a user account listing its own definitions.
	// GitHub lists a user's issue types and fields only to that user.
	issueMetadataViewer
)

// resolveIssueMetadataScope picks where to list issue types and fields. A
// repository always wins. Without one, the owner's account type decides. A
// failed lookup returns a tool error result.
func resolveIssueMetadataScope(ctx context.Context, client *github.Client, owner, repo string) (issueMetadataScope, *mcp.CallToolResult) {
	scope := issueMetadataScope{Owner: owner, Repo: repo}
	if repo != "" {
		return scope, nil
	}

	ownerType, resp, err := ownerAccountType(ctx, client, owner)
	if err != nil {
		return scope, ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to look up owner", resp, err)
	}
	if ownerType == "org" {
		scope.Kind = issueMetadataOrg
		return scope, nil
	}

	viewer, resp, err := client.Users.Get(ctx, "")
	if err != nil {
		return scope, ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get authenticated user", resp, err)
	}
	if !strings.EqualFold(viewer.GetLogin(), owner) {
		// owner is untrusted model input, so the error doesn't echo it.
		return scope, utils.NewToolResultError(
			"The owner is a user account other than the authenticated user, and GitHub lists a user's issue types and fields only to that user. Pass repo to list them for one of the owner's repositories.",
		)
	}
	scope.Kind = issueMetadataViewer
	return scope, nil
}

func fetchIssueTypes(ctx context.Context, client *github.Client, scope issueMetadataScope) ([]*github.IssueType, *github.Response, error) {
	var path string
	switch scope.Kind {
	case issueMetadataOrg:
		path = fmt.Sprintf("orgs/%s/issue-types", scope.Owner)
	case issueMetadataViewer:
		path = "user/issue-types"
	default:
		path = fmt.Sprintf("repos/%s/%s/issue-types", scope.Owner, scope.Repo)
	}

	req, err := client.NewRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, nil, err
	}
	var issueTypes []*github.IssueType
	resp, err := client.Do(req, &issueTypes)
	return issueTypes, resp, err
}
