package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/shurcooL/githubv4"
)

// resolverFieldsPageSize is the GraphQL ProjectV2 max page size; covers most
// projects in a single round-trip.
const resolverFieldsPageSize = 100

// ResolvedFieldOption is one option on a SINGLE_SELECT project field.
type ResolvedFieldOption struct {
	ID   string
	Name string
}

// ResolvedField is a project field resolved by name; Options is only set when
// DataType == "SINGLE_SELECT".
type ResolvedField struct {
	ID       string
	Name     string
	DataType string
	Options  []ResolvedFieldOption
}

// projectFieldsQueryOrg fetches all fields on an org-owned project (paginated).
type projectFieldsQueryOrg struct {
	Organization struct {
		ProjectV2 struct {
			Fields projectFieldsConnection `graphql:"fields(first: $first, after: $after)"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"organization(login: $owner)"`
}

// projectFieldsQueryUser fetches all fields on a user-owned project (paginated).
type projectFieldsQueryUser struct {
	User struct {
		ProjectV2 struct {
			Fields projectFieldsConnection `graphql:"fields(first: $first, after: $after)"`
		} `graphql:"projectV2(number: $projectNumber)"`
	} `graphql:"user(login: $owner)"`
}

// projectFieldsConnection is a paginated list of project fields. We select `id`
// to discriminate the union variant and `databaseId` for the numeric ID REST needs.
type projectFieldsConnection struct {
	Nodes []struct {
		ProjectV2Field struct {
			ID         githubv4.ID
			DatabaseID githubv4.Int `graphql:"databaseId"`
			Name       githubv4.String
			DataType   githubv4.String
		} `graphql:"... on ProjectV2Field"`
		ProjectV2IterationField struct {
			ID         githubv4.ID
			DatabaseID githubv4.Int `graphql:"databaseId"`
			Name       githubv4.String
			DataType   githubv4.String
		} `graphql:"... on ProjectV2IterationField"`
		ProjectV2SingleSelectField struct {
			ID         githubv4.ID
			DatabaseID githubv4.Int `graphql:"databaseId"`
			Name       githubv4.String
			DataType   githubv4.String
			Options    []struct {
				ID   githubv4.String
				Name githubv4.String
			}
		} `graphql:"... on ProjectV2SingleSelectField"`
	}
	PageInfo PageInfoFragment
}

// listAllProjectFields fetches every field on a project, paginating as needed.
func listAllProjectFields(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int) ([]ResolvedField, error) {
	all := []ResolvedField{}
	var after *githubv4.String

	for {
		vars := map[string]any{
			"owner":         githubv4.String(owner),
			"projectNumber": githubv4.Int(int32(projectNumber)), //nolint:gosec // Project numbers are small
			"first":         githubv4.Int(resolverFieldsPageSize),
			"after":         (*githubv4.String)(nil),
		}
		if after != nil {
			vars["after"] = after
		}

		var conn projectFieldsConnection
		if ownerType == "org" {
			var q projectFieldsQueryOrg
			if err := gqlClient.Query(ctx, &q, vars); err != nil {
				return nil, fmt.Errorf("failed to list project fields: %w", err)
			}
			conn = q.Organization.ProjectV2.Fields
		} else {
			var q projectFieldsQueryUser
			if err := gqlClient.Query(ctx, &q, vars); err != nil {
				return nil, fmt.Errorf("failed to list project fields: %w", err)
			}
			conn = q.User.ProjectV2.Fields
		}

		for _, n := range conn.Nodes {
			switch {
			case n.ProjectV2SingleSelectField.ID != nil:
				opts := make([]ResolvedFieldOption, 0, len(n.ProjectV2SingleSelectField.Options))
				for _, o := range n.ProjectV2SingleSelectField.Options {
					opts = append(opts, ResolvedFieldOption{ID: string(o.ID), Name: string(o.Name)})
				}
				all = append(all, ResolvedField{
					ID:       fmt.Sprintf("%d", n.ProjectV2SingleSelectField.DatabaseID),
					Name:     string(n.ProjectV2SingleSelectField.Name),
					DataType: string(n.ProjectV2SingleSelectField.DataType),
					Options:  opts,
				})
			case n.ProjectV2IterationField.ID != nil:
				all = append(all, ResolvedField{
					ID:       fmt.Sprintf("%d", n.ProjectV2IterationField.DatabaseID),
					Name:     string(n.ProjectV2IterationField.Name),
					DataType: string(n.ProjectV2IterationField.DataType),
				})
			case n.ProjectV2Field.ID != nil:
				all = append(all, ResolvedField{
					ID:       fmt.Sprintf("%d", n.ProjectV2Field.DatabaseID),
					Name:     string(n.ProjectV2Field.Name),
					DataType: string(n.ProjectV2Field.DataType),
				})
			}
		}

		if !bool(conn.PageInfo.HasNextPage) {
			break
		}
		end := conn.PageInfo.EndCursor
		after = &end
	}

	return all, nil
}

// resolveProjectFieldByName resolves a field by display name. Returns a
// structured error on not-found, ambiguous, or wrong-data-type (when
// expectedDataType is set) so the agent can self-correct.
func resolveProjectFieldByName(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, fieldName, expectedDataType string) (*ResolvedField, error) {
	if fieldName == "" {
		return nil, fmt.Errorf("field name must not be empty")
	}

	all, err := listAllProjectFields(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return nil, err
	}

	var matches []ResolvedField
	for _, f := range all {
		if strings.EqualFold(f.Name, fieldName) {
			matches = append(matches, f)
		}
	}

	if len(matches) == 0 {
		candidates := make([]any, 0, len(all))
		for _, f := range all {
			candidates = append(candidates, map[string]any{
				"name":      f.Name,
				"data_type": f.DataType,
			})
		}
		return nil, ghErrors.NewStructuredResolutionError(
			"field_not_found",
			fieldName,
			fmt.Sprintf("no project field named %q on project %s#%d; see candidates for available names", fieldName, owner, projectNumber),
			candidates,
		)
	}

	if len(matches) > 1 {
		candidates := make([]any, 0, len(matches))
		for _, f := range matches {
			candidates = append(candidates, map[string]any{
				"id":        f.ID,
				"data_type": f.DataType,
			})
		}
		return nil, ghErrors.NewStructuredResolutionError(
			"field_ambiguous",
			fieldName,
			"multiple fields share this name; pass updated_field.id to disambiguate",
			candidates,
		)
	}

	field := matches[0]

	if expectedDataType != "" && field.DataType != expectedDataType {
		return nil, ghErrors.NewStructuredResolutionError(
			"wrong_field_type",
			fieldName,
			fmt.Sprintf("field %q has data type %q but %q was expected", fieldName, field.DataType, expectedDataType),
			[]any{map[string]any{"id": field.ID, "data_type": field.DataType}},
		)
	}

	return &field, nil
}

// resolveSingleSelectOptionByName resolves an option name to its ID on a
// SINGLE_SELECT field. Returns a structured error if not found or ambiguous.
func resolveSingleSelectOptionByName(field *ResolvedField, optionName string) (string, error) {
	if field == nil {
		return "", fmt.Errorf("field must not be nil")
	}
	if field.DataType != "SINGLE_SELECT" {
		return "", ghErrors.NewStructuredResolutionError(
			"wrong_field_type",
			field.Name,
			fmt.Sprintf("cannot resolve option name on non-SINGLE_SELECT field %q (data type %q)", field.Name, field.DataType),
			nil,
		)
	}

	var matchIDs []string
	for _, o := range field.Options {
		if strings.EqualFold(o.Name, optionName) {
			matchIDs = append(matchIDs, o.ID)
		}
	}

	switch len(matchIDs) {
	case 0:
		candidates := make([]any, 0, len(field.Options))
		for _, o := range field.Options {
			candidates = append(candidates, map[string]any{"name": o.Name})
		}
		return "", ghErrors.NewStructuredResolutionError(
			"option_not_found",
			optionName,
			fmt.Sprintf("no option named %q on field %q; see candidates for available options", optionName, field.Name),
			candidates,
		)
	case 1:
		return matchIDs[0], nil
	default:
		candidates := make([]any, 0, len(matchIDs))
		for _, id := range matchIDs {
			candidates = append(candidates, map[string]any{"id": id})
		}
		return "", ghErrors.NewStructuredResolutionError(
			"option_ambiguous",
			optionName,
			fmt.Sprintf("multiple options on field %q share the name %q", field.Name, optionName),
			candidates,
		)
	}
}

// resolveProjectItemIDByIssueNumber resolves a (project, issue) pair to the
// project item's full database ID in one GraphQL hop. Returns a structured
// error if the issue is not an item on the project.
func resolveProjectItemIDByIssueNumber(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, issueOwner, issueRepo string, issueNumber int) (int64, error) {
	projectID, err := resolveProjectNodeID(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return 0, err
	}

	type projectItemsConnection struct {
		Nodes []struct {
			FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
			Project        struct {
				ID githubv4.ID
			}
		}
		PageInfo PageInfoFragment
	}

	var firstPageQuery struct {
		Repository struct {
			Issue struct {
				ProjectItems projectItemsConnection `graphql:"projectItems(first: 50, includeArchived: true)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $issueOwner, name: $issueRepo)"`
	}

	vars := map[string]any{
		"issueOwner":  githubv4.String(issueOwner),
		"issueRepo":   githubv4.String(issueRepo),
		"issueNumber": githubv4.Int(int32(issueNumber)), //nolint:gosec // Issue numbers are small
	}

	if err := gqlClient.Query(ctx, &firstPageQuery, vars); err != nil {
		return 0, fmt.Errorf("failed to resolve project item for %s/%s#%d: %w", issueOwner, issueRepo, issueNumber, err)
	}

	projectItems := firstPageQuery.Repository.Issue.ProjectItems
	for {
		for _, item := range projectItems.Nodes {
			if item.Project.ID == projectID {
				itemID, parseErr := parseInt64(string(item.FullDatabaseID))
				if parseErr != nil {
					return 0, fmt.Errorf("project item ID %q is not an integer: %w", string(item.FullDatabaseID), parseErr)
				}
				return itemID, nil
			}
		}

		if !projectItems.PageInfo.HasNextPage {
			break
		}

		var nextPageQuery struct {
			Repository struct {
				Issue struct {
					ProjectItems projectItemsConnection `graphql:"projectItems(first: 50, after: $after, includeArchived: true)"`
				} `graphql:"issue(number: $issueNumber)"`
			} `graphql:"repository(owner: $issueOwner, name: $issueRepo)"`
		}
		vars["after"] = projectItems.PageInfo.EndCursor
		if err := gqlClient.Query(ctx, &nextPageQuery, vars); err != nil {
			return 0, fmt.Errorf("failed to resolve project item for %s/%s#%d: %w", issueOwner, issueRepo, issueNumber, err)
		}
		projectItems = nextPageQuery.Repository.Issue.ProjectItems
	}

	return 0, ghErrors.NewStructuredResolutionError(
		"item_not_in_project",
		fmt.Sprintf("%s/%s#%d", issueOwner, issueRepo, issueNumber),
		"the issue exists but is not an item on the named project; add it first via add_project_item",
		nil,
	)
}

// resolveItemIDFromIssueArgs reads (item_owner, item_repo, issue_number) from args
// and resolves them to a project item ID. Returns a single friendly error if any input is missing.
func resolveItemIDFromIssueArgs(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, args map[string]any) (int64, error) {
	issueOwner, ownerErr := RequiredParam[string](args, "item_owner")
	issueRepo, repoErr := RequiredParam[string](args, "item_repo")
	issueNumber, numErr := RequiredInt(args, "issue_number")
	if ownerErr != nil || repoErr != nil || numErr != nil {
		return 0, fmt.Errorf("update_project_item requires either item_id, or item_owner + item_repo + issue_number to resolve the item by issue")
	}
	return resolveProjectItemIDByIssueNumber(ctx, gqlClient, owner, ownerType, projectNumber, issueOwner, issueRepo, issueNumber)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// resolveFieldNamesToIDs resolves field names to numeric IDs in one GraphQL
// hop. Fails fast with a structured error on any unresolved or ambiguous name.
func resolveFieldNamesToIDs(ctx context.Context, gqlClient *githubv4.Client, owner, ownerType string, projectNumber int, names []string) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}

	all, err := listAllProjectFields(ctx, gqlClient, owner, ownerType, projectNumber)
	if err != nil {
		return nil, err
	}

	// Build a name -> []ResolvedField map so we can detect duplicates per name.
	// Matching is case-insensitive to align with the GraphQL API's behaviour.
	byName := make(map[string][]ResolvedField, len(all))
	for _, f := range all {
		key := strings.ToLower(f.Name)
		byName[key] = append(byName[key], f)
	}

	out := make([]int64, 0, len(names))
	for _, name := range names {
		matches := byName[strings.ToLower(name)]
		switch len(matches) {
		case 0:
			candidates := make([]any, 0, len(all))
			for _, f := range all {
				candidates = append(candidates, map[string]any{"name": f.Name, "data_type": f.DataType})
			}
			return nil, ghErrors.NewStructuredResolutionError(
				"field_not_found",
				name,
				fmt.Sprintf("no project field named %q on project %s#%d", name, owner, projectNumber),
				candidates,
			)
		case 1:
			id, parseErr := parseInt64(matches[0].ID)
			if parseErr != nil {
				return nil, fmt.Errorf("resolved field %q has non-numeric ID %q; pass it via 'fields' instead", name, matches[0].ID)
			}
			out = append(out, id)
		default:
			candidates := make([]any, 0, len(matches))
			for _, f := range matches {
				candidates = append(candidates, map[string]any{"id": f.ID, "data_type": f.DataType})
			}
			return nil, ghErrors.NewStructuredResolutionError(
				"field_ambiguous",
				name,
				"multiple fields share this name; pass numeric IDs via 'fields' to disambiguate",
				candidates,
			)
		}
	}
	return out, nil
}
