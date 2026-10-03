package github

import (
	"context"
	"fmt"

	"github.com/shurcooL/githubv4"
)

// Pull request readiness is queried only for explicitly selected fields. REST
// list/search responses carry node IDs, so one nodes query enriches the whole
// page, including search pages spanning several repositories.
type pullRequestReadinessQuery struct {
	Nodes []struct {
		PullRequest struct {
			ID             githubv4.ID
			ReviewDecision githubv4.String `graphql:"reviewDecision @include(if: $includeReview)"`
			Commits        struct {
				Nodes []struct {
					Commit struct {
						StatusCheckRollup struct {
							State githubv4.String
						}
					}
				}
			} `graphql:"commits(last: 1) @include(if: $includeChecks)"`
		} `graphql:"... on PullRequest"`
	} `graphql:"nodes(ids: $ids)"`
}

type pullRequestReadiness struct {
	ReviewDecision    *string
	StatusCheckRollup *string
}

func requestedPullRequestReadiness(fields []string) bool {
	for _, field := range fields {
		if field == "review_decision" || field == "status_check_rollup" {
			return true
		}
	}
	return false
}

func fetchPullRequestReadiness(ctx context.Context, deps ToolDependencies, nodeIDs []string, fields []string) (map[string]pullRequestReadiness, error) {
	if len(nodeIDs) == 0 {
		return nil, nil
	}
	ids := make([]githubv4.ID, len(nodeIDs))
	for i, id := range nodeIDs {
		if id == "" {
			return nil, fmt.Errorf("pull request readiness requires a node ID for item %d", i+1)
		}
		ids[i] = githubv4.ID(id)
	}
	client, err := deps.GetGQLClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("get GraphQL client for pull request readiness: %w", err)
	}
	if client == nil {
		return nil, fmt.Errorf("GraphQL client for pull request readiness is unavailable")
	}
	var includeReview, includeChecks bool
	for _, field := range fields {
		switch field {
		case "review_decision":
			includeReview = true
		case "status_check_rollup":
			includeChecks = true
		}
	}
	var q pullRequestReadinessQuery
	if err := client.Query(ctx, &q, map[string]any{
		"ids":           ids,
		"includeReview": githubv4.Boolean(includeReview),
		"includeChecks": githubv4.Boolean(includeChecks),
	}); err != nil {
		return nil, fmt.Errorf("query pull request readiness: %w", err)
	}
	result := make(map[string]pullRequestReadiness, len(q.Nodes))
	for _, node := range q.Nodes {
		id, ok := node.PullRequest.ID.(string)
		if !ok || id == "" {
			return nil, fmt.Errorf("pull request readiness response contained a missing or non-PR node")
		}
		var item pullRequestReadiness
		if value := string(node.PullRequest.ReviewDecision); value != "" {
			item.ReviewDecision = &value
		}
		if commits := node.PullRequest.Commits.Nodes; len(commits) > 0 {
			if value := string(commits[0].Commit.StatusCheckRollup.State); value != "" {
				item.StatusCheckRollup = &value
			}
		}
		result[id] = item
	}
	for _, id := range nodeIDs {
		if _, ok := result[id]; !ok {
			return nil, fmt.Errorf("pull request readiness response omitted node %s", id)
		}
	}
	return result, nil
}

func addPullRequestReadinessFields(items []map[string]any, nodeIDs []string, fields []string, readiness map[string]pullRequestReadiness) error {
	if len(items) != len(nodeIDs) {
		return fmt.Errorf("pull request readiness item count mismatch")
	}
	for i, id := range nodeIDs {
		value, ok := readiness[id]
		if !ok {
			return fmt.Errorf("pull request readiness missing for node %s", id)
		}
		for _, field := range fields {
			switch field {
			case "review_decision":
				items[i][field] = value.ReviewDecision
			case "status_check_rollup":
				items[i][field] = value.StatusCheckRollup
			}
		}
	}
	return nil
}
