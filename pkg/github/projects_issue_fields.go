package github

import (
	"context"
	"fmt"
	"time"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/google/go-github/v89/github"
	"github.com/shurcooL/githubv4"
)

func buildIssueFieldUpdate(field *ResolvedField, raw any) (*IssueFieldCreateOrUpdateInput, error) {
	if field == nil || field.IssueFieldNodeID == "" {
		name := ""
		if field != nil {
			name = field.Name
		}
		return nil, ghErrors.NewStructuredResolutionError(
			"issue_field_metadata_unavailable",
			name,
			"the attached Project field did not include the underlying Issue Field node ID; refresh field metadata and retry",
			nil,
		)
	}

	input := &IssueFieldCreateOrUpdateInput{FieldID: githubv4.ID(field.IssueFieldNodeID)}
	if raw == nil {
		deleteValue := githubv4.Boolean(true)
		input.Delete = &deleteValue
		return input, nil
	}

	invalidValue := func(hint string) (*IssueFieldCreateOrUpdateInput, error) {
		return nil, ghErrors.NewStructuredResolutionError("invalid_field_value", field.Name, hint, nil)
	}

	switch field.DataType {
	case "TEXT":
		value, ok := raw.(string)
		if !ok {
			return invalidValue(fmt.Sprintf("Issue Field %q is TEXT; value must be a string or null to clear it", field.Name))
		}
		input.TextValue = githubv4.NewString(githubv4.String(value))
	case "NUMBER":
		value, ok := toFloat64(raw)
		if !ok {
			return invalidValue(fmt.Sprintf("Issue Field %q is NUMBER; value must be a finite number or null to clear it", field.Name))
		}
		number := githubv4.Float(value)
		input.NumberValue = &number
	case "DATE":
		value, ok := raw.(string)
		if !ok {
			return invalidValue(fmt.Sprintf("Issue Field %q is DATE; value must be a YYYY-MM-DD string or null to clear it", field.Name))
		}
		if _, err := time.Parse("2006-01-02", value); err != nil {
			return invalidValue(fmt.Sprintf("Issue Field %q is DATE; value %q must use YYYY-MM-DD format", field.Name, value))
		}
		input.DateValue = githubv4.NewString(githubv4.String(value))
	case "SINGLE_SELECT":
		value, ok := raw.(string)
		if !ok || value == "" {
			return invalidValue(fmt.Sprintf("Issue Field %q is SINGLE_SELECT; value must be a non-empty option name or ID, or null to clear it", field.Name))
		}
		optionID, err := resolveSingleSelectOptionByNameOrID(field, value)
		if err != nil {
			return nil, err
		}
		id := githubv4.ID(optionID)
		input.SingleSelectOptionID = &id
	default:
		return nil, ghErrors.NewStructuredResolutionError(
			"unsupported_field_type",
			field.Name,
			fmt.Sprintf("Issue Field %q has unsupported data type %q; supported types are TEXT, NUMBER, DATE, and SINGLE_SELECT", field.Name, field.DataType),
			nil,
		)
	}
	return input, nil
}

func projectItemIssueNodeID(item *github.ProjectV2Item) (githubv4.ID, error) {
	if item == nil || item.ContentType == nil {
		return "", ghErrors.NewStructuredResolutionError(
			"issue_field_metadata_unavailable",
			"",
			"the project item response did not identify its content type; Issue Fields can only be updated on Issue items",
			nil,
		)
	}

	contentType := string(*item.ContentType)
	if contentType != "Issue" {
		return "", ghErrors.NewStructuredResolutionError(
			"unsupported_item_type",
			contentType,
			"Issue Fields can only be updated on Issue project items, not pull requests or draft issues",
			nil,
		)
	}
	if item.Content == nil || item.Content.Issue == nil || item.Content.Issue.GetNodeID() == "" {
		return "", ghErrors.NewStructuredResolutionError(
			"issue_field_metadata_unavailable",
			"Issue",
			"the project item response did not include the underlying Issue node ID needed to update the Issue Field",
			nil,
		)
	}
	return githubv4.ID(item.Content.Issue.GetNodeID()), nil
}

type batchProjectItemIssueNode struct {
	ID             githubv4.ID
	FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
	Project        struct {
		ID githubv4.ID
	}
	Content struct {
		TypeName githubv4.String `graphql:"__typename"`
		Issue    struct {
			ID githubv4.ID
		} `graphql:"... on Issue"`
		PullRequest struct {
			ID githubv4.ID
		} `graphql:"... on PullRequest"`
		DraftIssue struct {
			ID githubv4.ID
		} `graphql:"... on DraftIssue"`
	}
}

type batchProjectItemsByNodeIDQuery struct {
	Nodes []struct {
		ProjectV2Item batchProjectItemIssueNode `graphql:"... on ProjectV2Item"`
	} `graphql:"nodes(ids: $ids)"`
}

func resolveItemIssuesByNodeID(ctx context.Context, gqlClient *githubv4.Client, projectID githubv4.ID, items []parsedBatchItem, requireIssue bool) map[string]itemLookupResult {
	if !requireIssue {
		return nil
	}

	seen := make(map[string]struct{}, len(items))
	var ids []githubv4.ID
	for _, item := range items {
		if item.err != nil || item.refKind != batchRefNodeID {
			continue
		}
		if _, exists := seen[item.nodeID]; exists {
			continue
		}
		seen[item.nodeID] = struct{}{}
		ids = append(ids, githubv4.ID(item.nodeID))
	}
	if len(ids) == 0 {
		return nil
	}

	var query batchProjectItemsByNodeIDQuery
	if err := gqlClient.Query(ctx, &query, map[string]any{"ids": ids}); err != nil {
		results := make(map[string]itemLookupResult, len(ids))
		for _, id := range ids {
			results[fmt.Sprintf("%v", id)] = itemLookupResult{err: fmt.Errorf("failed to inspect project item content: %w", err)}
		}
		return results
	}

	results := make(map[string]itemLookupResult, len(ids))
	for _, node := range query.Nodes {
		item := node.ProjectV2Item
		nodeID := fmt.Sprintf("%v", item.ID)
		if item.ID == nil {
			continue
		}
		if item.Project.ID != projectID {
			results[nodeID] = itemLookupResult{err: ghErrors.NewStructuredResolutionError(
				"item_not_in_project",
				nodeID,
				"the project item does not belong to the named project",
				nil,
			)}
			continue
		}

		issueNodeID, err := batchProjectItemIssueNodeID(item)
		fullDatabaseID := int64(0)
		if item.FullDatabaseID != "" {
			var parseErr error
			fullDatabaseID, parseErr = parseInt64(string(item.FullDatabaseID))
			if parseErr != nil {
				err = fmt.Errorf("project item %s has invalid full database ID %q: %w", nodeID, item.FullDatabaseID, parseErr)
			}
		}
		results[nodeID] = itemLookupResult{
			nodeID:         nodeID,
			fullDatabaseID: fullDatabaseID,
			issueNodeID:    issueNodeID,
			err:            err,
		}
	}

	for _, id := range ids {
		nodeID := fmt.Sprintf("%v", id)
		if _, exists := results[nodeID]; !exists {
			results[nodeID] = itemLookupResult{err: fmt.Errorf("project item %s was not found", nodeID)}
		}
	}
	return results
}

func batchProjectItemIssueNodeID(item batchProjectItemIssueNode) (string, error) {
	switch item.Content.TypeName {
	case "Issue":
		if item.Content.Issue.ID != nil {
			return fmt.Sprintf("%v", item.Content.Issue.ID), nil
		}
	case "PullRequest", "DraftIssue":
		return "", ghErrors.NewStructuredResolutionError(
			"unsupported_item_type",
			string(item.Content.TypeName),
			"Issue Fields can only be updated on Issue project items, not pull requests or draft issues",
			nil,
		)
	}
	return "", ghErrors.NewStructuredResolutionError(
		"issue_field_metadata_unavailable",
		fmt.Sprintf("%v", item.ID),
		"the project item did not include the underlying Issue node ID needed to update the Issue Field",
		nil,
	)
}
