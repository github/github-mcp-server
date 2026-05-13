package github

import (
	"context"
	"fmt"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/utils"
	gogithub "github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func firstStringSlice(values [][]string) []string {
	if len(values) == 0 {
		return nil
	}
	return values[0]
}

func buildPRAuthorAllowlist(authors []string) map[string]struct{} {
	if len(authors) == 0 {
		return nil
	}

	allowlist := make(map[string]struct{}, len(authors))
	for _, author := range authors {
		author = strings.TrimSpace(author)
		if author == "" {
			continue
		}
		allowlist[strings.ToLower(author)] = struct{}{}
	}
	if len(allowlist) == 0 {
		return nil
	}
	return allowlist
}

func isPRAuthorAllowed(allowlist map[string]struct{}, login string) (bool, bool) {
	if len(allowlist) == 0 {
		return true, false
	}
	_, ok := allowlist[strings.ToLower(strings.TrimSpace(login))]
	return ok, true
}

// enforcePRAuthorAllowlist returns a tool result error if an allowlist is
// configured and the PR's author is not on it. Callers that already have the
// pull request can pass it to avoid a duplicate API fetch.
func enforcePRAuthorAllowlist(
	ctx context.Context,
	deps ToolDependencies,
	owner, repo string,
	pullNumber int,
	pr *gogithub.PullRequest,
) (*mcp.CallToolResult, error) {
	if allowed, enforced := deps.IsPRAuthorAllowed(""); !enforced {
		return nil, nil
	} else if allowed {
		return nil, nil
	}

	if pr == nil {
		client, err := deps.GetClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil
		}

		var resp *gogithub.Response
		pr, resp, err = client.PullRequests.Get(ctx, owner, repo, pullNumber)
		if err != nil {
			return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get pull request", resp, err), nil
		}
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}
	}

	login := pr.GetUser().GetLogin()
	if allowed, _ := deps.IsPRAuthorAllowed(login); allowed {
		return nil, nil
	}

	return utils.NewToolResultError(fmt.Sprintf("pull request author %q is not in --allowed-pr-authors", login)), nil
}
