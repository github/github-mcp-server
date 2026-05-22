package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/ifc"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/sanitize"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shurcooL/githubv4"
)

// CloseIssueInput represents the input for closing an issue via the GraphQL API.
// Used to extend the functionality of the githubv4 library to support closing issues as duplicates.
type CloseIssueInput struct {
	IssueID          githubv4.ID             `json:"issueId"`
	ClientMutationID *githubv4.String        `json:"clientMutationId,omitempty"`
	StateReason      *IssueClosedStateReason `json:"stateReason,omitempty"`
	DuplicateIssueID *githubv4.ID            `json:"duplicateIssueId,omitempty"`
}

// IssueClosedStateReason represents the reason an issue was closed.
// Used to extend the functionality of the githubv4 library to support closing issues as duplicates.
type IssueClosedStateReason string

const (
	IssueClosedStateReasonCompleted  IssueClosedStateReason = "COMPLETED"
	IssueClosedStateReasonDuplicate  IssueClosedStateReason = "DUPLICATE"
	IssueClosedStateReasonNotPlanned IssueClosedStateReason = "NOT_PLANNED"
)

// fetchIssueIDs retrieves issue IDs via the GraphQL API.
// When duplicateOf is 0, it fetches only the main issue ID.
// When duplicateOf is non-zero, it fetches both the main issue and duplicate issue IDs in a single query.
func fetchIssueIDs(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueNumber int, duplicateOf int) (githubv4.ID, githubv4.ID, error) {
	// Build query variables common to both cases
	vars := map[string]any{
		"owner":       githubv4.String(owner),
		"repo":        githubv4.String(repo),
		"issueNumber": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
	}

	if duplicateOf == 0 {
		// Only fetch the main issue ID
		var query struct {
			Repository struct {
				Issue struct {
					ID githubv4.ID
				} `graphql:"issue(number: $issueNumber)"`
			} `graphql:"repository(owner: $owner, name: $repo)"`
		}

		if err := gqlClient.Query(ctx, &query, vars); err != nil {
			return "", "", fmt.Errorf("failed to get issue ID: %w", err)
		}

		return query.Repository.Issue.ID, "", nil
	}

	// Fetch both issue IDs in a single query
	var query struct {
		Repository struct {
			Issue struct {
				ID githubv4.ID
			} `graphql:"issue(number: $issueNumber)"`
			DuplicateIssue struct {
				ID githubv4.ID
			} `graphql:"duplicateIssue: issue(number: $duplicateOf)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	// Add duplicate issue number to variables
	vars["duplicateOf"] = githubv4.Int(duplicateOf) // #nosec G115 - issue numbers are always small positive integers

	if err := gqlClient.Query(ctx, &query, vars); err != nil {
		return "", "", fmt.Errorf("failed to get issue ID: %w", err)
	}

	return query.Repository.Issue.ID, query.Repository.DuplicateIssue.ID, nil
}

// getCloseStateReason converts a string state reason to the appropriate enum value
func getCloseStateReason(stateReason string) IssueClosedStateReason {
	switch stateReason {
	case "not_planned":
		return IssueClosedStateReasonNotPlanned
	case "duplicate":
		return IssueClosedStateReasonDuplicate
	default: // Default to "completed" for empty or "completed" values
		return IssueClosedStateReasonCompleted
	}
}

// IssueWriteFieldInput is a user-supplied issue field assignment: a field name and a string value.
// The value is forwarded directly to the REST API — for single-select fields it must be the
// option name (e.g. "P1"), not an option ID. Field ID resolution is done internally via GQL.
type IssueWriteFieldInput struct {
	FieldName string
	Value     string
}

// issueFieldWriteMetadataNode queries only the fields needed to resolve a write: the field's
// fullDatabaseId (BigInt scalar, returned as string) plus its name and data type for validation.
// shurcooL/githubv4 cannot use interface-level fragments at union top-level, so we repeat
// fullDatabaseId on each concrete type; all four implement IssueFieldCommon.
type issueFieldWriteMetadataNode struct {
	TypeName       githubv4.String `graphql:"__typename"`
	IssueFieldText struct {
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		DataType       githubv4.String
	} `graphql:"... on IssueFieldText"`
	IssueFieldNumber struct {
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		DataType       githubv4.String
	} `graphql:"... on IssueFieldNumber"`
	IssueFieldDate struct {
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		DataType       githubv4.String
	} `graphql:"... on IssueFieldDate"`
	IssueFieldSingleSelect struct {
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
		Name           githubv4.String
		DataType       githubv4.String
	} `graphql:"... on IssueFieldSingleSelect"`
}

// issueFieldWriteMetadata holds the resolved name, database ID, and data type for a single field.
type issueFieldWriteMetadata struct {
	DatabaseID int64
	Name       string
	DataType   string
}

type issueFieldWriteMetadataQuery struct {
	Repository struct {
		IssueFields struct {
			Nodes []issueFieldWriteMetadataNode
		} `graphql:"issueFields(first: 100)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func optionalIssueWriteFields(args map[string]any) ([]IssueWriteFieldInput, error) {
	raw, exists := args["issue_fields"]
	if !exists {
		return nil, nil
	}

	var inputMaps []map[string]any
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			m, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("each issue_fields item must be an object")
			}
			inputMaps = append(inputMaps, m)
		}
	case []map[string]any:
		inputMaps = v
	default:
		return nil, fmt.Errorf("issue_fields must be an array")
	}

	out := make([]IssueWriteFieldInput, 0, len(inputMaps))
	for _, m := range inputMaps {
		fieldName, err := RequiredParam[string](m, "field_name")
		if err != nil || strings.TrimSpace(fieldName) == "" {
			return nil, fmt.Errorf("field_name is required for each issue_fields item")
		}
		value, err := RequiredParam[string](m, "value")
		if err != nil {
			return nil, fmt.Errorf("issue_fields item %q: value is required", fieldName)
		}
		out = append(out, IssueWriteFieldInput{FieldName: fieldName, Value: value})
	}
	return out, nil
}

func resolveIssueWriteFieldValues(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, inputs []IssueWriteFieldInput) ([]*github.IssueRequestFieldValue, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issue_fields", "repo_issue_fields")
	var query issueFieldWriteMetadataQuery
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}
	if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
		return nil, fmt.Errorf("failed to query issue field metadata: %w", err)
	}

	// Build name → metadata map from the GQL response.
	byName := make(map[string]issueFieldWriteMetadata, len(query.Repository.IssueFields.Nodes))
	for _, node := range query.Repository.IssueFields.Nodes {
		var name, dataType, fullDBID string
		switch string(node.TypeName) {
		case "IssueFieldText":
			name, dataType, fullDBID = string(node.IssueFieldText.Name), string(node.IssueFieldText.DataType), string(node.IssueFieldText.FullDatabaseID)
		case "IssueFieldNumber":
			name, dataType, fullDBID = string(node.IssueFieldNumber.Name), string(node.IssueFieldNumber.DataType), string(node.IssueFieldNumber.FullDatabaseID)
		case "IssueFieldDate":
			name, dataType, fullDBID = string(node.IssueFieldDate.Name), string(node.IssueFieldDate.DataType), string(node.IssueFieldDate.FullDatabaseID)
		case "IssueFieldSingleSelect":
			name, dataType, fullDBID = string(node.IssueFieldSingleSelect.Name), string(node.IssueFieldSingleSelect.DataType), string(node.IssueFieldSingleSelect.FullDatabaseID)
		default:
			continue
		}
		dbID, _ := strconv.ParseInt(fullDBID, 10, 64)
		byName[strings.ToLower(strings.TrimSpace(name))] = issueFieldWriteMetadata{
			DatabaseID: dbID,
			Name:       name,
			DataType:   dataType,
		}
	}

	resolved := make([]*github.IssueRequestFieldValue, 0, len(inputs))
	for _, input := range inputs {
		meta, ok := byName[strings.ToLower(strings.TrimSpace(input.FieldName))]
		if !ok {
			return nil, fmt.Errorf("issue field %q was not found in %s/%s", input.FieldName, owner, repo)
		}
		if meta.DatabaseID == 0 {
			return nil, fmt.Errorf("issue field %q is missing fullDatabaseId", input.FieldName)
		}
		// For single-select the REST API expects the option name as a string value.
		// For all other types, pass the value through as-is.
		resolved = append(resolved, &github.IssueRequestFieldValue{
			FieldID: meta.DatabaseID,
			Value:   input.Value,
		})
	}
	return resolved, nil
}

// IssueFieldRef resolves the name of an issue field across its concrete types.
// IssueFields is a union of IssueFieldDate, IssueFieldNumber, IssueFieldSingleSelect, IssueFieldText,
// so we have to ask for `name` on each member.
type IssueFieldRef struct {
	Date         struct{ Name githubv4.String } `graphql:"... on IssueFieldDate"`
	Number       struct{ Name githubv4.String } `graphql:"... on IssueFieldNumber"`
	SingleSelect struct{ Name githubv4.String } `graphql:"... on IssueFieldSingleSelect"`
	Text         struct{ Name githubv4.String } `graphql:"... on IssueFieldText"`
}

// Name returns the populated name from whichever IssueFields union variant the field resolved to.
func (r IssueFieldRef) Name() string {
	switch {
	case r.Date.Name != "":
		return string(r.Date.Name)
	case r.Number.Name != "":
		return string(r.Number.Name)
	case r.SingleSelect.Name != "":
		return string(r.SingleSelect.Name)
	case r.Text.Name != "":
		return string(r.Text.Name)
	}
	return ""
}

// IssueFieldValueFragment captures the value of a custom issue field. IssueFieldValue is a union
// of 4 concrete value types; each carries its own value scalar and a reference to its parent field.
// The Number variant's `value` is aliased to `valueNumber` to avoid a Float vs String type clash on decode.
type IssueFieldValueFragment struct {
	TypeName  string `graphql:"__typename"`
	DateValue struct {
		Field IssueFieldRef
		Value githubv4.String
	} `graphql:"... on IssueFieldDateValue"`
	NumberValue struct {
		Field IssueFieldRef
		Value githubv4.Float `graphql:"valueNumber: value"`
	} `graphql:"... on IssueFieldNumberValue"`
	SingleSelectValue struct {
		Field IssueFieldRef
		Value githubv4.String
	} `graphql:"... on IssueFieldSingleSelectValue"`
	TextValue struct {
		Field IssueFieldRef
		Value githubv4.String
	} `graphql:"... on IssueFieldTextValue"`
}

// IssueFragment represents a fragment of an issue node in the GraphQL API.
type IssueFragment struct {
	Number     githubv4.Int
	Title      githubv4.String
	Body       githubv4.String
	State      githubv4.String
	DatabaseID int64

	Author struct {
		Login githubv4.String
	}
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	Labels    struct {
		Nodes []struct {
			Name        githubv4.String
			ID          githubv4.String
			Description githubv4.String
		}
	} `graphql:"labels(first: 100)"`
	Comments struct {
		TotalCount githubv4.Int
	} `graphql:"comments"`
	IssueFieldValues struct {
		Nodes []IssueFieldValueFragment
	} `graphql:"issueFieldValues(first: 25)"`
}

// Common interface for all issue query types
type IssueQueryResult interface {
	GetIssueFragment() IssueQueryFragment
	GetIsPrivate() bool
}

type IssueQueryFragment struct {
	Nodes    []IssueFragment `graphql:"nodes"`
	PageInfo struct {
		HasNextPage     githubv4.Boolean
		HasPreviousPage githubv4.Boolean
		StartCursor     githubv4.String
		EndCursor       githubv4.String
	}
	TotalCount int
}

// ListIssuesQuery is the root query structure for fetching issues with optional label filtering.
type ListIssuesQuery struct {
	Repository struct {
		Issues    IssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryTypeWithLabels is the query structure for fetching issues with optional label filtering.
type ListIssuesQueryTypeWithLabels struct {
	Repository struct {
		Issues    IssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {issueFieldValues: $issueFieldValues})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryWithSince is the query structure for fetching issues without label filtering but with since filtering.
type ListIssuesQueryWithSince struct {
	Repository struct {
		Issues    IssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since, issueFieldValues: $issueFieldValues})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// ListIssuesQueryTypeWithLabelsWithSince is the query structure for fetching issues with both label and since filtering.
type ListIssuesQueryTypeWithLabelsWithSince struct {
	Repository struct {
		Issues    IssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since, issueFieldValues: $issueFieldValues})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// IssueFieldValueFilter mirrors the GraphQL IssueFieldValueFilter input. Exactly one typed value
// field should be set per filter (the monolith resolver rejects multiple).
type IssueFieldValueFilter struct {
	FieldName               githubv4.String  `json:"fieldName"`
	TextValue               *githubv4.String `json:"textValue,omitempty"`
	DateValue               *githubv4.String `json:"dateValue,omitempty"`
	NumberValue             *githubv4.Float  `json:"numberValue,omitempty"`
	SingleSelectOptionValue *githubv4.String `json:"singleSelectOptionValue,omitempty"`
}

// Implement the interface for all query types
func (q *ListIssuesQueryTypeWithLabels) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryTypeWithLabels) GetIsPrivate() bool { return bool(q.Repository.IsPrivate) }

func (q *ListIssuesQuery) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQuery) GetIsPrivate() bool { return bool(q.Repository.IsPrivate) }

func (q *ListIssuesQueryWithSince) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryWithSince) GetIsPrivate() bool { return bool(q.Repository.IsPrivate) }

func (q *ListIssuesQueryTypeWithLabelsWithSince) GetIssueFragment() IssueQueryFragment {
	return q.Repository.Issues
}

func (q *ListIssuesQueryTypeWithLabelsWithSince) GetIsPrivate() bool {
	return bool(q.Repository.IsPrivate)
}

func getIssueQueryType(hasLabels bool, hasSince bool) any {
	switch {
	case hasLabels && hasSince:
		return &ListIssuesQueryTypeWithLabelsWithSince{}
	case hasLabels:
		return &ListIssuesQueryTypeWithLabels{}
	case hasSince:
		return &ListIssuesQueryWithSince{}
	default:
		return &ListIssuesQuery{}
	}
}

// --- Legacy list_issues GraphQL types ---
//
// These mirror the pre-Issues-2.0 shape of the list_issues query and exist solely
// to back the FeatureFlagIssueFields-disabled variant of the tool. They omit the
// IssueFieldValues selection and the filterBy: {issueFieldValues: ...} clause so
// the request does not depend on server-side issue_fields GraphQL features and
// does not pay the wire/server cost of fetching custom field values when the flag
// is off. Delete this whole block (and its callers) when FeatureFlagIssueFields
// is removed.

type LegacyIssueFragment struct {
	Number     githubv4.Int
	Title      githubv4.String
	Body       githubv4.String
	State      githubv4.String
	DatabaseID int64

	Author struct {
		Login githubv4.String
	}
	CreatedAt githubv4.DateTime
	UpdatedAt githubv4.DateTime
	Labels    struct {
		Nodes []struct {
			Name        githubv4.String
			ID          githubv4.String
			Description githubv4.String
		}
	} `graphql:"labels(first: 100)"`
	Comments struct {
		TotalCount githubv4.Int
	} `graphql:"comments"`
}

type LegacyIssueQueryFragment struct {
	Nodes    []LegacyIssueFragment `graphql:"nodes"`
	PageInfo struct {
		HasNextPage     githubv4.Boolean
		HasPreviousPage githubv4.Boolean
		StartCursor     githubv4.String
		EndCursor       githubv4.String
	}
	TotalCount int
}

type LegacyIssueQueryResult interface {
	GetLegacyIssueFragment() LegacyIssueQueryFragment
	GetIsPrivate() bool
}

type LegacyListIssuesQuery struct {
	Repository struct {
		Issues    LegacyIssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type LegacyListIssuesQueryTypeWithLabels struct {
	Repository struct {
		Issues    LegacyIssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type LegacyListIssuesQueryWithSince struct {
	Repository struct {
		Issues    LegacyIssueQueryFragment `graphql:"issues(first: $first, after: $after, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

type LegacyListIssuesQueryTypeWithLabelsWithSince struct {
	Repository struct {
		Issues    LegacyIssueQueryFragment `graphql:"issues(first: $first, after: $after, labels: $labels, states: $states, orderBy: {field: $orderBy, direction: $direction}, filterBy: {since: $since})"`
		IsPrivate githubv4.Boolean
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

func (q *LegacyListIssuesQuery) GetLegacyIssueFragment() LegacyIssueQueryFragment {
	return q.Repository.Issues
}
func (q *LegacyListIssuesQuery) GetIsPrivate() bool { return bool(q.Repository.IsPrivate) }

func (q *LegacyListIssuesQueryTypeWithLabels) GetLegacyIssueFragment() LegacyIssueQueryFragment {
	return q.Repository.Issues
}
func (q *LegacyListIssuesQueryTypeWithLabels) GetIsPrivate() bool {
	return bool(q.Repository.IsPrivate)
}

func (q *LegacyListIssuesQueryWithSince) GetLegacyIssueFragment() LegacyIssueQueryFragment {
	return q.Repository.Issues
}
func (q *LegacyListIssuesQueryWithSince) GetIsPrivate() bool {
	return bool(q.Repository.IsPrivate)
}

func (q *LegacyListIssuesQueryTypeWithLabelsWithSince) GetLegacyIssueFragment() LegacyIssueQueryFragment {
	return q.Repository.Issues
}
func (q *LegacyListIssuesQueryTypeWithLabelsWithSince) GetIsPrivate() bool {
	return bool(q.Repository.IsPrivate)
}

func getLegacyIssueQueryType(hasLabels bool, hasSince bool) any {
	switch {
	case hasLabels && hasSince:
		return &LegacyListIssuesQueryTypeWithLabelsWithSince{}
	case hasLabels:
		return &LegacyListIssuesQueryTypeWithLabels{}
	case hasSince:
		return &LegacyListIssuesQueryWithSince{}
	default:
		return &LegacyListIssuesQuery{}
	}
}

// IssueRead creates a tool to get details of a specific issue in a GitHub repository.
func IssueRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"method": {
				Type: "string",
				Description: `The read operation to perform on a single issue.
Options are:
1. get - Get details of a specific issue.
2. get_comments - Get issue comments.
3. get_sub_issues - Get sub-issues of the issue.
4. get_labels - Get labels assigned to the issue.
`,
				Enum: []any{"get", "get_comments", "get_sub_issues", "get_labels"},
			},
			"owner": {
				Type:        "string",
				Description: "The owner of the repository",
			},
			"repo": {
				Type:        "string",
				Description: "The name of the repository",
			},
			"issue_number": {
				Type:        "number",
				Description: "The number of the issue",
			},
		},
		Required: []string{"method", "owner", "repo", "issue_number"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_read",
			Description: t("TOOL_ISSUE_READ_DESCRIPTION", "Get information about a specific issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_READ_USER_TITLE", "Get issue details"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub graphql client", err), nil, nil
			}

			// attachIFC adds the IFC label to a successful tool result when
			// IFC labels are enabled. If the visibility lookup fails the
			// label is omitted rather than misclassifying the result.
			attachIFC := func(r *mcp.CallToolResult) *mcp.CallToolResult {
				if r == nil || r.IsError || !deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
					return r
				}
				isPrivate, err := FetchRepoIsPrivate(ctx, client, owner, repo)
				if err != nil {
					return r
				}
				if r.Meta == nil {
					r.Meta = mcp.Meta{}
				}
				r.Meta["ifc"] = ifc.LabelListIssues(isPrivate)
				return r
			}

			switch method {
			case "get":
				result, err := GetIssue(ctx, client, deps, owner, repo, issueNumber)
				return attachIFC(result), nil, err
			case "get_comments":
				result, err := GetIssueComments(ctx, client, deps, owner, repo, issueNumber, pagination)
				return attachIFC(result), nil, err
			case "get_sub_issues":
				result, err := GetSubIssues(ctx, client, deps, owner, repo, issueNumber, pagination)
				return attachIFC(result), nil, err
			case "get_labels":
				result, err := GetIssueLabels(ctx, gqlClient, owner, repo, issueNumber)
				return attachIFC(result), nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
}

func GetIssue(ctx context.Context, client *github.Client, deps ToolDependencies, owner string, repo string, issueNumber int) (*mcp.CallToolResult, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	flags := deps.GetFlags(ctx)

	issue, resp, err := client.Issues.Get(ctx, owner, repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get issue", resp, body), nil
	}

	if flags.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		login := issue.GetUser().GetLogin()
		if login != "" {
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
			}
			if !isSafeContent {
				return utils.NewToolResultError("access to issue details is restricted by lockdown mode"), nil
			}
		}
	}

	// Sanitize title/body on response
	if issue != nil {
		if issue.Title != nil {
			issue.Title = github.Ptr(sanitize.Sanitize(*issue.Title))
		}
		if issue.Body != nil {
			issue.Body = github.Ptr(sanitize.Sanitize(*issue.Body))
		}
	}

	minimalIssue := convertToMinimalIssue(issue)

	return MarshalledTextResult(minimalIssue), nil
}

func GetIssueComments(ctx context.Context, client *github.Client, deps ToolDependencies, owner string, repo string, issueNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	flags := deps.GetFlags(ctx)

	opts := &github.IssueListCommentsOptions{
		ListOptions: github.ListOptions{
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		},
	}

	comments, resp, err := client.Issues.ListComments(ctx, owner, repo, issueNumber, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue comments: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to get issue comments", resp, body), nil
	}
	if flags.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		filteredComments := make([]*github.IssueComment, 0, len(comments))
		for _, comment := range comments {
			user := comment.User
			if user == nil {
				continue
			}
			login := user.GetLogin()
			if login == "" {
				continue
			}
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
			}
			if isSafeContent {
				filteredComments = append(filteredComments, comment)
			}
		}
		comments = filteredComments
	}

	minimalComments := make([]MinimalIssueComment, 0, len(comments))
	for _, comment := range comments {
		minimalComments = append(minimalComments, convertToMinimalIssueComment(comment))
	}

	return MarshalledTextResult(minimalComments), nil
}

func GetSubIssues(ctx context.Context, client *github.Client, deps ToolDependencies, owner string, repo string, issueNumber int, pagination PaginationParams) (*mcp.CallToolResult, error) {
	cache, err := deps.GetRepoAccessCache(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo access cache: %w", err)
	}
	featureFlags := deps.GetFlags(ctx)

	opts := &github.ListOptions{
		Page:    pagination.Page,
		PerPage: pagination.PerPage,
	}

	subIssues, resp, err := client.SubIssue.ListByIssue(ctx, owner, repo, int64(issueNumber), opts)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to list sub-issues",
			resp,
			err,
		), nil
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list sub-issues", resp, body), nil
	}

	if featureFlags.LockdownMode {
		if cache == nil {
			return nil, fmt.Errorf("lockdown cache is not configured")
		}
		filteredSubIssues := make([]*github.SubIssue, 0, len(subIssues))
		for _, subIssue := range subIssues {
			user := subIssue.User
			if user == nil {
				continue
			}
			login := user.GetLogin()
			if login == "" {
				continue
			}
			isSafeContent, err := cache.IsSafeContent(ctx, login, owner, repo)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to check lockdown mode: %v", err)), nil
			}
			if isSafeContent {
				filteredSubIssues = append(filteredSubIssues, subIssue)
			}
		}
		subIssues = filteredSubIssues
	}

	r, err := json.Marshal(subIssues)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func GetIssueLabels(ctx context.Context, client *githubv4.Client, owner string, repo string, issueNumber int) (*mcp.CallToolResult, error) {
	// Get current labels on the issue using GraphQL
	var query struct {
		Repository struct {
			Issue struct {
				Labels struct {
					Nodes []struct {
						ID          githubv4.ID
						Name        githubv4.String
						Color       githubv4.String
						Description githubv4.String
					}
					TotalCount githubv4.Int
				} `graphql:"labels(first: 100)"`
			} `graphql:"issue(number: $issueNumber)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]any{
		"owner":       githubv4.String(owner),
		"repo":        githubv4.String(repo),
		"issueNumber": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
	}

	if err := client.Query(ctx, &query, vars); err != nil {
		return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to get issue labels", err), nil
	}

	// Extract label information
	issueLabels := make([]map[string]any, len(query.Repository.Issue.Labels.Nodes))
	for i, label := range query.Repository.Issue.Labels.Nodes {
		issueLabels[i] = map[string]any{
			"id":          fmt.Sprintf("%v", label.ID),
			"name":        string(label.Name),
			"color":       string(label.Color),
			"description": string(label.Description),
		}
	}

	response := map[string]any{
		"labels":     issueLabels,
		"totalCount": int(query.Repository.Issue.Labels.TotalCount),
	}

	out, err := json.Marshal(response)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(out)), nil
}

// ListIssueTypes creates a tool to list defined issue types for an organization. This can be used to understand supported issue type values for creating or updating issues.
func ListIssueTypes(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issue_types",
			Description: t("TOOL_LIST_ISSUE_TYPES_FOR_ORG", "List supported issue types for repository owner (organization)."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUE_TYPES_USER_TITLE", "List available issue types"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The organization owner of the repository",
					},
				},
				Required: []string{"owner"},
			},
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			issueTypes, resp, err := client.Organizations.ListIssueTypes(ctx, owner)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to list issue types", err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to list issue types", resp, body), nil, nil
			}

			r, err := json.Marshal(issueTypes)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal issue types", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// AddIssueComment creates a tool to add a comment to an issue.
func AddIssueComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "add_issue_comment",
			Description: t("TOOL_ADD_ISSUE_COMMENT_DESCRIPTION", "Add a comment to a specific issue in a GitHub repository. Use this tool to add comments to pull requests as well (in this case pass pull request number as issue_number), but only if user is not asking specifically to add review comments."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_ISSUE_COMMENT_USER_TITLE", "Add comment to issue"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
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
					"issue_number": {
						Type:        "number",
						Description: "Issue number to comment on",
					},
					"body": {
						Type:        "string",
						Description: "Comment content",
					},
				},
				Required: []string{"owner", "repo", "issue_number", "body"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			body, err := RequiredParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			comment := &github.IssueComment{
				Body: github.Ptr(body),
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}
			createdComment, resp, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, comment)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to create comment", err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusCreated {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
				}
				return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create comment", resp, body), nil, nil
			}

			minimalResponse := MinimalResponse{
				ID:  fmt.Sprintf("%d", createdComment.GetID()),
				URL: createdComment.GetHTMLURL(),
			}

			r, err := json.Marshal(minimalResponse)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil, nil
			}

			return utils.NewToolResultText(string(r)), nil, nil
		})
}

// SubIssueWrite creates a tool to add a sub-issue to a parent issue.
func SubIssueWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "sub_issue_write",
			Description: t("TOOL_SUB_ISSUE_WRITE_DESCRIPTION", "Add a sub-issue to a parent issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SUB_ISSUE_WRITE_USER_TITLE", "Change sub-issue"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: `The action to perform on a single sub-issue
Options are:
- 'add' - add a sub-issue to a parent issue in a GitHub repository.
- 'remove' - remove a sub-issue from a parent issue in a GitHub repository.
- 'reprioritize' - change the order of sub-issues within a parent issue in a GitHub repository. Use either 'after_id' or 'before_id' to specify the new position.
				`,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "The number of the parent issue",
					},
					"sub_issue_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to add. ID is not the same as issue number",
					},
					"replace_parent": {
						Type:        "boolean",
						Description: "When true, replaces the sub-issue's current parent issue. Use with 'add' method only.",
					},
					"after_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to be prioritized after (either after_id OR before_id should be specified)",
					},
					"before_id": {
						Type:        "number",
						Description: "The ID of the sub-issue to be prioritized before (either after_id OR before_id should be specified)",
					},
				},
				Required: []string{"method", "owner", "repo", "issue_number", "sub_issue_id"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			issueNumber, err := RequiredInt(args, "issue_number")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			subIssueID, err := RequiredInt(args, "sub_issue_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			replaceParent, err := OptionalParam[bool](args, "replace_parent")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			afterID, err := OptionalIntParam(args, "after_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			beforeID, err := OptionalIntParam(args, "before_id")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			switch strings.ToLower(method) {
			case "add":
				result, err := AddSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, replaceParent)
				return result, nil, err
			case "remove":
				// Call the remove sub-issue function
				result, err := RemoveSubIssue(ctx, client, owner, repo, issueNumber, subIssueID)
				return result, nil, err
			case "reprioritize":
				// Call the reprioritize sub-issue function
				result, err := ReprioritizeSubIssue(ctx, client, owner, repo, issueNumber, subIssueID, afterID, beforeID)
				return result, nil, err
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown method: %s", method)), nil, nil
			}
		})
	st.FeatureFlagDisable = FeatureFlagIssuesGranular
	return st
}

func AddSubIssue(ctx context.Context, client *github.Client, owner string, repo string, issueNumber int, subIssueID int, replaceParent bool) (*mcp.CallToolResult, error) {
	subIssueRequest := github.SubIssueRequest{
		SubIssueID:    int64(subIssueID),
		ReplaceParent: github.Ptr(replaceParent),
	}

	subIssue, resp, err := client.SubIssue.Add(ctx, owner, repo, int64(issueNumber), subIssueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to add sub-issue",
			resp,
			err,
		), nil
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to add sub-issue", resp, body), nil
	}

	r, err := json.Marshal(subIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func RemoveSubIssue(ctx context.Context, client *github.Client, owner string, repo string, issueNumber int, subIssueID int) (*mcp.CallToolResult, error) {
	subIssueRequest := github.SubIssueRequest{
		SubIssueID: int64(subIssueID),
	}

	subIssue, resp, err := client.SubIssue.Remove(ctx, owner, repo, int64(issueNumber), subIssueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to remove sub-issue",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to remove sub-issue", resp, body), nil
	}

	r, err := json.Marshal(subIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

func ReprioritizeSubIssue(ctx context.Context, client *github.Client, owner string, repo string, issueNumber int, subIssueID int, afterID int, beforeID int) (*mcp.CallToolResult, error) {
	// Validate that either after_id or before_id is specified, but not both
	if afterID == 0 && beforeID == 0 {
		return utils.NewToolResultError("either after_id or before_id must be specified"), nil
	}
	if afterID != 0 && beforeID != 0 {
		return utils.NewToolResultError("only one of after_id or before_id should be specified, not both"), nil
	}

	subIssueRequest := github.SubIssueRequest{
		SubIssueID: int64(subIssueID),
	}

	if afterID != 0 {
		afterIDInt64 := int64(afterID)
		subIssueRequest.AfterID = &afterIDInt64
	}
	if beforeID != 0 {
		beforeIDInt64 := int64(beforeID)
		subIssueRequest.BeforeID = &beforeIDInt64
	}

	subIssue, resp, err := client.SubIssue.Reprioritize(ctx, owner, repo, int64(issueNumber), subIssueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to reprioritize sub-issue",
			resp,
			err,
		), nil
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to reprioritize sub-issue", resp, body), nil
	}

	r, err := json.Marshal(subIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

// SearchIssues creates a tool to search for issues.
func SearchIssues(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {
				Type:        "string",
				Description: "Search query using GitHub issues search syntax",
			},
			"owner": {
				Type:        "string",
				Description: "Optional repository owner. If provided with repo, only issues for this repository are listed.",
			},
			"repo": {
				Type:        "string",
				Description: "Optional repository name. If provided with owner, only issues for this repository are listed.",
			},
			"sort": {
				Type:        "string",
				Description: "Sort field by number of matches of categories, defaults to best match",
				Enum: []any{
					"comments",
					"reactions",
					"reactions-+1",
					"reactions--1",
					"reactions-smile",
					"reactions-thinking_face",
					"reactions-heart",
					"reactions-tada",
					"interactions",
					"created",
					"updated",
				},
			},
			"order": {
				Type:        "string",
				Description: "Sort order",
				Enum:        []any{"asc", "desc"},
			},
		},
		Required: []string{"query"},
	}
	WithPagination(schema)

	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "search_issues",
			Description: t("TOOL_SEARCH_ISSUES_DESCRIPTION", "Search for issues in GitHub repositories using issues search syntax already scoped to is:issue"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_SEARCH_ISSUES_USER_TITLE", "Search issues"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			var options []searchOption
			if deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
				options = append(options, withSearchPostProcess(searchIssuesIFCPostProcess(deps)))
			}
			result, err := searchIssuesHandler(ctx, deps, args, options...)
			return result, nil, err
		})
}

// searchIssuesIFCPostProcess returns a searchPostProcessFn that attaches the
// IFC label for a search_issues result. It looks up the visibility (and, for
// private repos, collaborators) of every repository represented in the search
// payload and joins the labels via ifc.LabelSearchIssues. If any per-repo
// lookup fails the label is omitted to avoid misclassifying the result.
func searchIssuesIFCPostProcess(deps ToolDependencies) searchPostProcessFn {
	return func(ctx context.Context, result *github.IssuesSearchResult, callResult *mcp.CallToolResult) {
		if callResult == nil || callResult.IsError || result == nil {
			return
		}

		client, err := deps.GetClient(ctx)
		if err != nil {
			return
		}

		uniqueRepos := uniqueSearchIssuesRepos(result)
		visibilities := make([]bool, 0, len(uniqueRepos))
		for _, r := range uniqueRepos {
			isPrivate, err := FetchRepoIsPrivate(ctx, client, r.owner, r.repo)
			if err != nil {
				return
			}
			visibilities = append(visibilities, isPrivate)
		}

		if callResult.Meta == nil {
			callResult.Meta = mcp.Meta{}
		}
		callResult.Meta["ifc"] = ifc.LabelSearchIssues(visibilities)
	}
}

type searchIssuesRepoRef struct {
	owner string
	repo  string
}

// uniqueSearchIssuesRepos extracts the owner/repo pairs of every issue in the
// search result, preserving order of first appearance and deduplicating.
func uniqueSearchIssuesRepos(result *github.IssuesSearchResult) []searchIssuesRepoRef {
	if result == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []searchIssuesRepoRef
	for _, issue := range result.Issues {
		if issue == nil {
			continue
		}
		owner, repo, ok := parseRepositoryURL(issue.GetRepositoryURL())
		if !ok {
			continue
		}
		key := owner + "/" + repo
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, searchIssuesRepoRef{owner: owner, repo: repo})
	}
	return out
}

// parseRepositoryURL extracts the owner and repo from a GitHub API repository
// URL of the form https://api.github.com/repos/{owner}/{repo}.
func parseRepositoryURL(repoURL string) (string, string, bool) {
	if repoURL == "" {
		return "", "", false
	}
	const marker = "/repos/"
	idx := strings.LastIndex(repoURL, marker)
	if idx < 0 {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(repoURL[idx+len(marker):], "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// SearchIssueResult wraps a REST search hit with its custom issue field values, fetched in a follow-up GraphQL nodes() query.
type SearchIssueResult struct {
	*github.Issue
	FieldValues []MinimalIssueFieldValue `json:"field_values,omitempty"`
}

// MarshalJSON serializes SearchIssueResult, suppressing the raw issue_field_values from the
// embedded REST response in favour of the normalized field_values populated via GraphQL enrichment.
func (r SearchIssueResult) MarshalJSON() ([]byte, error) {
	issueBytes, err := json.Marshal(r.Issue)
	if err != nil {
		return nil, err
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(issueBytes, &m); err != nil {
		return nil, err
	}
	delete(m, "issue_field_values")
	if r.FieldValues != nil {
		fv, err := json.Marshal(r.FieldValues)
		if err != nil {
			return nil, err
		}
		m["field_values"] = fv
	}
	return json.Marshal(m)
}

// SearchIssuesResponse mirrors the REST IssuesSearchResult JSON shape and adds field_values
// per item, sourced from a single GraphQL nodes() round-trip.
type SearchIssuesResponse struct {
	Total             *int                `json:"total_count,omitempty"`
	IncompleteResults *bool               `json:"incomplete_results,omitempty"`
	Items             []SearchIssueResult `json:"items"`
}

// searchIssuesNodesQuery batches a nodes(ids:) lookup over the REST search results to retrieve
// each issue's custom field values in a single GraphQL request.
type searchIssuesNodesQuery struct {
	Nodes []struct {
		Issue struct {
			ID               githubv4.ID
			IssueFieldValues struct {
				Nodes []IssueFieldValueFragment
			} `graphql:"issueFieldValues(first: 25)"`
		} `graphql:"... on Issue"`
	} `graphql:"nodes(ids: $ids)"`
}

// fetchIssueFieldValuesByNodeID runs one GraphQL nodes() query for the given REST issues and
// returns a map of node_id -> flattened field values. Issues without a node_id are skipped, and
// an empty result set short-circuits the round-trip.
func fetchIssueFieldValuesByNodeID(ctx context.Context, gqlClient *githubv4.Client, issues []*github.Issue) (map[string][]MinimalIssueFieldValue, error) {
	ids := make([]githubv4.ID, 0, len(issues))
	for _, iss := range issues {
		if iss == nil || iss.NodeID == nil || *iss.NodeID == "" {
			continue
		}
		ids = append(ids, githubv4.ID(*iss.NodeID))
	}
	if len(ids) == 0 {
		return nil, nil
	}

	var q searchIssuesNodesQuery
	if err := gqlClient.Query(ctx, &q, map[string]any{"ids": ids}); err != nil {
		return nil, err
	}

	result := make(map[string][]MinimalIssueFieldValue, len(q.Nodes))
	for _, n := range q.Nodes {
		idStr, ok := n.Issue.ID.(string)
		if !ok || idStr == "" {
			continue
		}
		vals := make([]MinimalIssueFieldValue, 0, len(n.Issue.IssueFieldValues.Nodes))
		for _, fv := range n.Issue.IssueFieldValues.Nodes {
			if m, ok := fragmentToMinimalIssueFieldValue(fv); ok {
				vals = append(vals, m)
			}
		}
		result[idStr] = vals
	}
	return result, nil
}

// searchIssuesHandler runs the REST issues search, enriches each hit with custom field values
// fetched via a single follow-up GraphQL nodes() query, and applies any post-process options
// (e.g. IFC labelling).
func searchIssuesHandler(ctx context.Context, deps ToolDependencies, args map[string]any, options ...searchOption) (*mcp.CallToolResult, error) {
	const errorPrefix = "failed to search issues"

	query, opts, err := prepareSearchArgs(args, "issue")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil
	}

	client, err := deps.GetClient(ctx)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to get GitHub client", err), nil
	}
	result, resp, err := client.Search.Issues(ctx, query, opts)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix, err), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr(errorPrefix+": failed to read response body", err), nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, errorPrefix, resp, body), nil
	}

	var fieldValuesByID map[string][]MinimalIssueFieldValue
	if deps.IsFeatureEnabled(ctx, FeatureFlagIssueFields) && len(result.Issues) > 0 {
		gqlClient, err := deps.GetGQLClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr(errorPrefix+": failed to get GitHub GraphQL client", err), nil
		}
		fieldValuesByID, err = fetchIssueFieldValuesByNodeID(ctx, gqlClient, result.Issues)
		if err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, errorPrefix+": failed to fetch issue field values", err), nil
		}
	}

	items := make([]SearchIssueResult, 0, len(result.Issues))
	for _, iss := range result.Issues {
		hit := SearchIssueResult{Issue: iss}
		if iss != nil && iss.NodeID != nil {
			hit.FieldValues = fieldValuesByID[*iss.NodeID]
		}
		items = append(items, hit)
	}

	response := SearchIssuesResponse{
		Total:             result.Total,
		IncompleteResults: result.IncompleteResults,
		Items:             items,
	}

	r, err := json.Marshal(response)
	if err != nil {
		return utils.NewToolResultErrorFromErr(errorPrefix+": failed to marshal response", err), nil
	}

	callResult := utils.NewToolResultText(string(r))
	cfg := searchConfig{}
	for _, opt := range options {
		opt(&cfg)
	}
	if cfg.postProcess != nil {
		cfg.postProcess(ctx, result, callResult)
	}
	return callResult, nil
}

// IssueWrite creates a tool to create a new or update an existing issue in a GitHub repository.
// IssueWriteUIResourceURI is the URI for the issue_write tool's MCP App UI resource.
const IssueWriteUIResourceURI = "ui://github-mcp-server/issue-write"

func IssueWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_write",
			Description: t("TOOL_ISSUE_WRITE_DESCRIPTION", "Create a new or update an existing issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_WRITE_USER_TITLE", "Create or update issue"),
				ReadOnlyHint: false,
			},
			Meta: mcp.Meta{
				"ui": map[string]any{
					"resourceUri": IssueWriteUIResourceURI,
					"visibility":  []string{"model", "app"},
				},
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"method": {
						Type: "string",
						Description: `Write operation to perform on a single issue.
Options are:
- 'create' - creates a new issue.
- 'update' - updates an existing issue.
`,
						Enum: []any{"create", "update"},
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"issue_number": {
						Type:        "number",
						Description: "Issue number to update",
					},
					"title": {
						Type:        "string",
						Description: "Issue title",
					},
					"body": {
						Type:        "string",
						Description: "Issue body content",
					},
					"assignees": {
						Type:        "array",
						Description: "Usernames to assign to this issue",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"labels": {
						Type:        "array",
						Description: "Labels to apply to this issue",
						Items: &jsonschema.Schema{
							Type: "string",
						},
					},
					"milestone": {
						Type:        "number",
						Description: "Milestone number",
					},
					"type": {
						Type:        "string",
						Description: "Type of this issue. Only use if the repository has issue types configured. Use list_issue_types tool to get valid type values for the organization. If the repository doesn't support issue types, omit this parameter.",
					},
					"state": {
						Type:        "string",
						Description: "New state",
						Enum:        []any{"open", "closed"},
					},
					"state_reason": {
						Type:        "string",
						Description: "Reason for the state change. Ignored unless state is changed.",
						Enum:        []any{"completed", "not_planned", "duplicate"},
					},
					"duplicate_of": {
						Type:        "number",
						Description: "Issue number that this issue is a duplicate of. Only used when state_reason is 'duplicate'.",
					},
					"issue_fields": {
						Type:        "array",
						Description: "Custom issue field values to set. Each entry specifies a field by name and its value. For single-select fields, value must be the option name (e.g. \"P1\"). For date fields, value must be YYYY-MM-DD.",
						Items: &jsonschema.Schema{
							Type:                 "object",
							AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
							Properties: map[string]*jsonschema.Schema{
								"field_name": {
									Type:        "string",
									Description: "Name of the custom field (case-insensitive).",
								},
								"value": {
									Type:        "string",
									Description: "Value to set. For single-select, the option name. For dates, YYYY-MM-DD. For numbers, the numeric value as a string.",
								},
							},
							Required: []string{"field_name", "value"},
						},
					},
				},
				Required: []string{"method", "owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			method, err := RequiredParam[string](args, "method")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// When MCP Apps are enabled and the client supports UI,
			// check if this is a UI form submission. The UI sends _ui_submitted=true
			// to distinguish form submissions from LLM calls.
			uiSubmitted, _ := OptionalParam[bool](args, "_ui_submitted")

			if deps.IsFeatureEnabled(ctx, MCPAppsFeatureFlag) && clientSupportsUI(ctx, req) && !uiSubmitted {
				if method == "update" {
					// Skip the UI form when a state change is requested because
					// the form only handles title/body editing and would lose the
					// state transition (e.g. closing or reopening the issue).
					if _, hasState := args["state"]; !hasState {
						issueNumber, numErr := RequiredInt(args, "issue_number")
						if numErr != nil {
							return utils.NewToolResultError("issue_number is required for update method"), nil, nil
						}
						return utils.NewToolResultText(fmt.Sprintf("Ready to update issue #%d in %s/%s. IMPORTANT: The issue has NOT been updated yet. Do NOT tell the user the issue was updated. The user MUST click Submit in the form to update it.", issueNumber, owner, repo)), nil, nil
					}
				} else {
					return utils.NewToolResultText(fmt.Sprintf("Ready to create an issue in %s/%s. IMPORTANT: The issue has NOT been created yet. Do NOT tell the user the issue was created. The user MUST click Submit in the form to create it.", owner, repo)), nil, nil
				}
			}

			title, err := OptionalParam[string](args, "title")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Optional parameters
			body, err := OptionalParam[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get assignees
			assignees, err := OptionalStringArrayParam(args, "assignees")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get labels
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get optional milestone
			milestone, err := OptionalIntParam(args, "milestone")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			var milestoneNum int
			if milestone != 0 {
				milestoneNum = milestone
			}

			// Get optional type
			issueType, err := OptionalParam[string](args, "type")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Handle state, state_reason and duplicateOf parameters
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			stateReason, err := OptionalParam[string](args, "state_reason")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			duplicateOf, err := OptionalIntParam(args, "duplicate_of")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if duplicateOf != 0 && stateReason != "duplicate" {
				return utils.NewToolResultError("duplicate_of can only be used when state_reason is 'duplicate'"), nil, nil
			}

			issueFieldInputs, err := optionalIssueWriteFields(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			gqlClient, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GraphQL client", err), nil, nil
			}

			issueFieldValues, err := resolveIssueWriteFieldValues(ctx, gqlClient, owner, repo, issueFieldInputs)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to resolve issue_fields: %v", err)), nil, nil
			}

			switch method {
			case "create":
				result, err := CreateIssue(ctx, client, owner, repo, title, body, assignees, labels, milestoneNum, issueType, issueFieldValues)
				return result, nil, err
			case "update":
				issueNumber, err := RequiredInt(args, "issue_number")
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
				result, err := UpdateIssue(ctx, client, gqlClient, owner, repo, issueNumber, title, body, assignees, labels, milestoneNum, issueType, issueFieldValues, state, stateReason, duplicateOf)
				return result, nil, err
			default:
				return utils.NewToolResultError("invalid method, must be either 'create' or 'update'"), nil, nil
			}
		})
	st.FeatureFlagDisable = FeatureFlagIssuesGranular
	return st
}

func CreateIssue(ctx context.Context, client *github.Client, owner string, repo string, title string, body string, assignees []string, labels []string, milestoneNum int, issueType string, issueFieldValues []*github.IssueRequestFieldValue) (*mcp.CallToolResult, error) {
	if title == "" {
		return utils.NewToolResultError("missing required parameter: title"), nil
	}

	// Create the issue request
	issueRequest := &github.IssueRequest{
		Title:            github.Ptr(title),
		Body:             github.Ptr(body),
		Assignees:        &assignees,
		Labels:           &labels,
		IssueFieldValues: issueFieldValues,
	}

	if milestoneNum != 0 {
		issueRequest.Milestone = &milestoneNum
	}

	if issueType != "" {
		issueRequest.Type = github.Ptr(issueType)
	}

	issue, resp, err := client.Issues.Create(ctx, owner, repo, issueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to create issue",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return utils.NewToolResultErrorFromErr("failed to read response body", err), nil
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create issue", resp, body), nil
	}

	// Return minimal response with just essential information
	minimalResponse := MinimalResponse{
		ID:  fmt.Sprintf("%d", issue.GetID()),
		URL: issue.GetHTMLURL(),
	}

	r, err := json.Marshal(minimalResponse)
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to marshal response", err), nil
	}

	return utils.NewToolResultText(string(r)), nil
}

func UpdateIssue(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, owner string, repo string, issueNumber int, title string, body string, assignees []string, labels []string, milestoneNum int, issueType string, issueFieldValues []*github.IssueRequestFieldValue, state string, stateReason string, duplicateOf int) (*mcp.CallToolResult, error) {
	// Create the issue request with only provided fields
	issueRequest := &github.IssueRequest{}

	// Set optional parameters if provided
	if title != "" {
		issueRequest.Title = github.Ptr(title)
	}

	if body != "" {
		issueRequest.Body = github.Ptr(body)
	}

	if len(labels) > 0 {
		issueRequest.Labels = &labels
	}

	if len(assignees) > 0 {
		issueRequest.Assignees = &assignees
	}

	if milestoneNum != 0 {
		issueRequest.Milestone = &milestoneNum
	}

	if issueType != "" {
		issueRequest.Type = github.Ptr(issueType)
	}

	if len(issueFieldValues) > 0 {
		issueRequest.IssueFieldValues = issueFieldValues
	}

	updatedIssue, resp, err := client.Issues.Edit(ctx, owner, repo, issueNumber, issueRequest)
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx,
			"failed to update issue",
			resp,
			err,
		), nil
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to update issue", resp, body), nil
	}

	// Use GraphQL API for state updates
	if state != "" {
		// Mandate specifying duplicateOf when trying to close as duplicate
		if state == "closed" && stateReason == "duplicate" && duplicateOf == 0 {
			return utils.NewToolResultError("duplicate_of must be provided when state_reason is 'duplicate'"), nil
		}

		// Get target issue ID (and duplicate issue ID if needed)
		issueID, duplicateIssueID, err := fetchIssueIDs(ctx, gqlClient, owner, repo, issueNumber, duplicateOf)
		if err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to find issues", err), nil
		}

		switch state {
		case "open":
			// Use ReopenIssue mutation for opening
			var mutation struct {
				ReopenIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
						State  githubv4.String
					}
				} `graphql:"reopenIssue(input: $input)"`
			}

			err = gqlClient.Mutate(ctx, &mutation, githubv4.ReopenIssueInput{
				IssueID: issueID,
			}, nil)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to reopen issue", err), nil
			}
		case "closed":
			// Use CloseIssue mutation for closing
			var mutation struct {
				CloseIssue struct {
					Issue struct {
						ID     githubv4.ID
						Number githubv4.Int
						URL    githubv4.String
						State  githubv4.String
					}
				} `graphql:"closeIssue(input: $input)"`
			}

			stateReasonValue := getCloseStateReason(stateReason)
			closeInput := CloseIssueInput{
				IssueID:     issueID,
				StateReason: &stateReasonValue,
			}

			// Set duplicate issue ID if needed
			if stateReason == "duplicate" {
				closeInput.DuplicateIssueID = &duplicateIssueID
			}

			err = gqlClient.Mutate(ctx, &mutation, closeInput, nil)
			if err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "Failed to close issue", err), nil
			}
		}
	}

	// Return minimal response with just essential information
	minimalResponse := MinimalResponse{
		ID:  fmt.Sprintf("%d", updatedIssue.GetID()),
		URL: updatedIssue.GetHTMLURL(),
	}

	r, err := json.Marshal(minimalResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	return utils.NewToolResultText(string(r)), nil
}

// ListIssues creates a tool to list and filter repository issues. This variant is
// gated by FeatureFlagIssueFields and exposes the Issues 2.0 field_filters input
// plus field_values output enrichment. When the flag is off, LegacyListIssues is
// served instead. Both registrations share the tool name "list_issues" and rely on
// the inventory's feature-flag filter to make exactly one active at a time.
func ListIssues(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"state": {
				Type:        "string",
				Description: "Filter by state, by default both open and closed issues are returned when not provided",
				Enum:        []any{"OPEN", "CLOSED"},
			},
			"labels": {
				Type:        "array",
				Description: "Filter by labels",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"orderBy": {
				Type:        "string",
				Description: "Order issues by field. If provided, the 'direction' also needs to be provided.",
				Enum:        []any{"CREATED_AT", "UPDATED_AT", "COMMENTS"},
			},
			"direction": {
				Type:        "string",
				Description: "Order direction. If provided, the 'orderBy' also needs to be provided.",
				Enum:        []any{"ASC", "DESC"},
			},
			"since": {
				Type:        "string",
				Description: "Filter by date (ISO 8601 timestamp)",
			},
			"field_filters": {
				Type:        "array",
				Description: "Filter by custom issue field values. Each entry takes a field_name and a value; the server looks up the field and coerces the value to its type (single-select option name, text, number, or YYYY-MM-DD date).",
				Items: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"field_name": {
							Type:        "string",
							Description: "Name of the custom field (e.g. \"Priority\"). Case-insensitive.",
						},
						"value": {
							Type:        "string",
							Description: "Value to filter on. For single-select fields, the option name (e.g. \"P1\"). For dates, YYYY-MM-DD. For numbers, the numeric value as a string. For text, the text value.",
						},
					},
					Required: []string{"field_name", "value"},
				},
			},
		},
		Required: []string{"owner", "repo"},
	}
	WithCursorPagination(schema)

	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issues",
			Description: t("TOOL_LIST_ISSUES_DESCRIPTION", "List issues in a GitHub repository. For pagination, use the 'endCursor' from the previous response's 'pageInfo' in the 'after' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUES_USER_TITLE", "List issues"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Set optional parameters if provided
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Normalize and filter by state
			state = strings.ToUpper(state)
			var states []githubv4.IssueState

			switch state {
			case "OPEN", "CLOSED":
				states = []githubv4.IssueState{githubv4.IssueState(state)}
			default:
				states = []githubv4.IssueState{githubv4.IssueStateOpen, githubv4.IssueStateClosed}
			}

			// Get labels
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			orderBy, err := OptionalParam[string](args, "orderBy")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Normalize and validate orderBy
			orderBy = strings.ToUpper(orderBy)
			switch orderBy {
			case "CREATED_AT", "UPDATED_AT", "COMMENTS":
				// Valid, keep as is
			default:
				orderBy = "CREATED_AT"
			}

			// Normalize and validate direction
			direction = strings.ToUpper(direction)
			switch direction {
			case "ASC", "DESC":
				// Valid, keep as is
			default:
				direction = "DESC"
			}

			since, err := OptionalParam[string](args, "since")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// There are two optional parameters: since and labels.
			var sinceTime time.Time
			var hasSince bool
			if since != "" {
				sinceTime, err = parseISOTimestamp(since)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to list issues: %s", err.Error())), nil, nil
				}
				hasSince = true
			}
			hasLabels := len(labels) > 0

			rawFilters, err := parseRawFieldFilters(args)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			// Get pagination parameters and convert to GraphQL format
			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return nil, nil, err
			}

			// Check if someone tried to use page-based pagination instead of cursor-based
			if _, pageProvided := args["page"]; pageProvided {
				return utils.NewToolResultError("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'."), nil, nil
			}

			// Check if pagination parameters were explicitly provided
			_, perPageProvided := args["perPage"]
			paginationExplicit := perPageProvided

			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, nil, err
			}

			// Use default of 30 if pagination was not explicitly provided
			if !paginationExplicit {
				defaultFirst := int32(DefaultGraphQLPageSize)
				paginationParams.First = &defaultFirst
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			// Resolve field filters by looking up the repo's issue fields so we can
			// coerce each value into the right typed slot on IssueFieldValueFilter.
			fieldFilters := []IssueFieldValueFilter{}
			if len(rawFilters) > 0 {
				fields, err := fetchIssueFields(ctx, client, owner, repo)
				if err != nil {
					return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to look up issue fields for field_filters", err), nil, nil
				}
				fieldFilters, err = resolveFieldFilters(rawFilters, fields)
				if err != nil {
					return utils.NewToolResultError(err.Error()), nil, nil
				}
			}

			vars := map[string]any{
				"owner":            githubv4.String(owner),
				"repo":             githubv4.String(repo),
				"states":           states,
				"orderBy":          githubv4.IssueOrderField(orderBy),
				"direction":        githubv4.OrderDirection(direction),
				"first":            githubv4.Int(*paginationParams.First),
				"issueFieldValues": fieldFilters,
			}

			if paginationParams.After != nil {
				vars["after"] = githubv4.String(*paginationParams.After)
			} else {
				// Used within query, therefore must be set to nil and provided as $after
				vars["after"] = (*githubv4.String)(nil)
			}

			// Ensure optional parameters are set
			if hasLabels {
				// Use query with labels filtering - convert string labels to githubv4.String slice
				labelStrings := make([]githubv4.String, len(labels))
				for i, label := range labels {
					labelStrings[i] = githubv4.String(label)
				}
				vars["labels"] = labelStrings
			}

			if hasSince {
				vars["since"] = githubv4.DateTime{Time: sinceTime}
			}

			issueQuery := getIssueQueryType(hasLabels, hasSince)
			// The list_issues query references the issue_fields-gated IssueFieldValueFilter
			// input type unconditionally, so we always opt into the feature via header. This
			// is a no-op once the flags are globally rolled out.
			ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issue_fields", "repo_issue_fields")
			if err := client.Query(ctxWithFeatures, issueQuery, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(
					ctx,
					"failed to list issues",
					err,
				), nil, nil
			}

			var resp MinimalIssuesResponse
			var isPrivate bool
			if queryResult, ok := issueQuery.(IssueQueryResult); ok {
				resp = convertToMinimalIssuesResponse(queryResult.GetIssueFragment())
				isPrivate = queryResult.GetIsPrivate()
			}

			result := MarshalledTextResult(resp)
			if deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
				if result.Meta == nil {
					result.Meta = mcp.Meta{}
				}
				result.Meta["ifc"] = ifc.LabelListIssues(isPrivate)
			}
			return result, nil, nil
		})
	st.FeatureFlagEnable = FeatureFlagIssueFields
	return st
}

// LegacyListIssues is the FeatureFlagIssueFields-disabled variant of list_issues.
// It exposes the pre-Issues-2.0 schema (no field_filters) and uses a GraphQL query
// path that does not select issueFieldValues or pass the issue_fields filter, so
// the request does not depend on server-side issue_fields features and does not pay
// for custom field values when the flag is off. Both this and ListIssues register
// under the tool name "list_issues"; exactly one is active for any given request
// thanks to mutually exclusive FeatureFlagEnable / FeatureFlagDisable annotations.
// Delete this function (and the rest of the Legacy* block) when the flag is removed.
func LegacyListIssues(t translations.TranslationHelperFunc) inventory.ServerTool {
	schema := &jsonschema.Schema{
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
			"state": {
				Type:        "string",
				Description: "Filter by state, by default both open and closed issues are returned when not provided",
				Enum:        []any{"OPEN", "CLOSED"},
			},
			"labels": {
				Type:        "array",
				Description: "Filter by labels",
				Items: &jsonschema.Schema{
					Type: "string",
				},
			},
			"orderBy": {
				Type:        "string",
				Description: "Order issues by field. If provided, the 'direction' also needs to be provided.",
				Enum:        []any{"CREATED_AT", "UPDATED_AT", "COMMENTS"},
			},
			"direction": {
				Type:        "string",
				Description: "Order direction. If provided, the 'orderBy' also needs to be provided.",
				Enum:        []any{"ASC", "DESC"},
			},
			"since": {
				Type:        "string",
				Description: "Filter by date (ISO 8601 timestamp)",
			},
		},
		Required: []string{"owner", "repo"},
	}
	WithCursorPagination(schema)

	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issues",
			Description: t("TOOL_LIST_ISSUES_DESCRIPTION", "List issues in a GitHub repository. For pagination, use the 'endCursor' from the previous response's 'pageInfo' in the 'after' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUES_USER_TITLE", "List issues"),
				ReadOnlyHint: true,
			},
			InputSchema: schema,
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			state = strings.ToUpper(state)
			var states []githubv4.IssueState
			switch state {
			case "OPEN", "CLOSED":
				states = []githubv4.IssueState{githubv4.IssueState(state)}
			default:
				states = []githubv4.IssueState{githubv4.IssueStateOpen, githubv4.IssueStateClosed}
			}

			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			orderBy, err := OptionalParam[string](args, "orderBy")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			direction, err := OptionalParam[string](args, "direction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			orderBy = strings.ToUpper(orderBy)
			switch orderBy {
			case "CREATED_AT", "UPDATED_AT", "COMMENTS":
			default:
				orderBy = "CREATED_AT"
			}
			direction = strings.ToUpper(direction)
			switch direction {
			case "ASC", "DESC":
			default:
				direction = "DESC"
			}

			since, err := OptionalParam[string](args, "since")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			var sinceTime time.Time
			var hasSince bool
			if since != "" {
				sinceTime, err = parseISOTimestamp(since)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to list issues: %s", err.Error())), nil, nil
				}
				hasSince = true
			}
			hasLabels := len(labels) > 0

			pagination, err := OptionalCursorPaginationParams(args)
			if err != nil {
				return nil, nil, err
			}
			if _, pageProvided := args["page"]; pageProvided {
				return utils.NewToolResultError("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'."), nil, nil
			}
			_, perPageProvided := args["perPage"]
			paginationExplicit := perPageProvided
			paginationParams, err := pagination.ToGraphQLParams()
			if err != nil {
				return nil, nil, err
			}
			if !paginationExplicit {
				defaultFirst := int32(DefaultGraphQLPageSize)
				paginationParams.First = &defaultFirst
			}

			client, err := deps.GetGQLClient(ctx)
			if err != nil {
				return utils.NewToolResultError(fmt.Sprintf("failed to get GitHub GQL client: %v", err)), nil, nil
			}

			vars := map[string]any{
				"owner":     githubv4.String(owner),
				"repo":      githubv4.String(repo),
				"states":    states,
				"orderBy":   githubv4.IssueOrderField(orderBy),
				"direction": githubv4.OrderDirection(direction),
				"first":     githubv4.Int(*paginationParams.First),
			}
			if paginationParams.After != nil {
				vars["after"] = githubv4.String(*paginationParams.After)
			} else {
				vars["after"] = (*githubv4.String)(nil)
			}
			if hasLabels {
				labelStrings := make([]githubv4.String, len(labels))
				for i, label := range labels {
					labelStrings[i] = githubv4.String(label)
				}
				vars["labels"] = labelStrings
			}
			if hasSince {
				vars["since"] = githubv4.DateTime{Time: sinceTime}
			}

			issueQuery := getLegacyIssueQueryType(hasLabels, hasSince)
			if err := client.Query(ctx, issueQuery, vars); err != nil {
				return ghErrors.NewGitHubGraphQLErrorResponse(
					ctx,
					"failed to list issues",
					err,
				), nil, nil
			}

			var resp MinimalIssuesResponse
			var isPrivate bool
			if queryResult, ok := issueQuery.(LegacyIssueQueryResult); ok {
				resp = convertLegacyToMinimalIssuesResponse(queryResult.GetLegacyIssueFragment())
				isPrivate = queryResult.GetIsPrivate()
			}

			result := MarshalledTextResult(resp)
			if deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
				if result.Meta == nil {
					result.Meta = mcp.Meta{}
				}
				result.Meta["ifc"] = ifc.LabelListIssues(isPrivate)
			}
			return result, nil, nil
		})
	st.FeatureFlagDisable = FeatureFlagIssueFields
	return st
}

// rawFieldFilter is the user-supplied {field_name, value} pair before type resolution.
type rawFieldFilter struct {
	Name  string
	Value string
}

// parseRawFieldFilters extracts the optional field_filters parameter into a list of
// {name, value} pairs. The value is always a string here; type-aware coercion happens
// later in resolveFieldFilters once we know each field's data_type.
func parseRawFieldFilters(args map[string]any) ([]rawFieldFilter, error) {
	raw, ok := args["field_filters"]
	if !ok {
		return nil, nil
	}

	var entries []map[string]any
	switch v := raw.(type) {
	case []any:
		for _, f := range v {
			entry, ok := f.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("each field_filters entry must be an object")
			}
			entries = append(entries, entry)
		}
	case []map[string]any:
		entries = v
	default:
		return nil, fmt.Errorf("field_filters must be an array")
	}

	filters := make([]rawFieldFilter, 0, len(entries))
	for _, entry := range entries {
		fieldName, err := RequiredParam[string](entry, "field_name")
		if err != nil {
			return nil, fmt.Errorf("field_filters entry: %s", err.Error())
		}
		value, err := RequiredParam[string](entry, "value")
		if err != nil {
			return nil, fmt.Errorf("field_filters entry %q: %s", fieldName, err.Error())
		}
		filters = append(filters, rawFieldFilter{Name: fieldName, Value: value})
	}
	return filters, nil
}

// resolveFieldFilters matches each raw filter against a known field definition and
// coerces the value into the right typed slot on IssueFieldValueFilter. Matching is
// case-insensitive on field name; option names are also matched case-insensitively for
// single-select fields.
func resolveFieldFilters(rawFilters []rawFieldFilter, fields []IssueField) ([]IssueFieldValueFilter, error) {
	byName := make(map[string]IssueField, len(fields))
	knownNames := make([]string, 0, len(fields))
	for _, f := range fields {
		byName[strings.ToLower(f.Name)] = f
		knownNames = append(knownNames, f.Name)
	}

	out := make([]IssueFieldValueFilter, 0, len(rawFilters))
	for _, rf := range rawFilters {
		field, ok := byName[strings.ToLower(rf.Name)]
		if !ok {
			return nil, fmt.Errorf("field_filters: unknown field %q. Known fields: %s", rf.Name, strings.Join(knownNames, ", "))
		}

		filter := IssueFieldValueFilter{FieldName: githubv4.String(field.Name)}
		switch field.DataType {
		case "SINGLE_SELECT":
			// Validate the option name against the field's options so we fail fast
			// with a useful error instead of an opaque GraphQL one.
			var matched string
			for _, o := range field.Options {
				if strings.EqualFold(o.Name, rf.Value) {
					matched = o.Name
					break
				}
			}
			if matched == "" {
				optionNames := make([]string, 0, len(field.Options))
				for _, o := range field.Options {
					optionNames = append(optionNames, o.Name)
				}
				return nil, fmt.Errorf("field_filters: %q is not a valid option for %q. Valid options: %s", rf.Value, field.Name, strings.Join(optionNames, ", "))
			}
			v := githubv4.String(matched)
			filter.SingleSelectOptionValue = &v
		case "TEXT":
			v := githubv4.String(rf.Value)
			filter.TextValue = &v
		case "DATE":
			if _, err := time.Parse("2006-01-02", rf.Value); err != nil {
				return nil, fmt.Errorf("field_filters: %q is not a valid date for %q (expected YYYY-MM-DD): %s", rf.Value, field.Name, err.Error())
			}
			v := githubv4.String(rf.Value)
			filter.DateValue = &v
		case "NUMBER":
			n, err := strconv.ParseFloat(rf.Value, 64)
			if err != nil {
				return nil, fmt.Errorf("field_filters: %q is not a valid number for %q: %s", rf.Value, field.Name, err.Error())
			}
			v := githubv4.Float(n)
			filter.NumberValue = &v
		default:
			return nil, fmt.Errorf("field_filters: field %q has unsupported data_type %q", field.Name, field.DataType)
		}
		out = append(out, filter)
	}
	return out, nil
}

// parseISOTimestamp parses an ISO 8601 timestamp string into a time.Time object.
// Returns the parsed time or an error if parsing fails.
// Example formats supported: "2023-01-15T14:30:00Z", "2023-01-15"
func parseISOTimestamp(timestamp string) (time.Time, error) {
	if timestamp == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	// Try RFC3339 format (standard ISO 8601 with time)
	t, err := time.Parse(time.RFC3339, timestamp)
	if err == nil {
		return t, nil
	}

	// Try simple date format (YYYY-MM-DD)
	t, err = time.Parse("2006-01-02", timestamp)
	if err == nil {
		return t, nil
	}

	// Return error with supported formats
	return time.Time{}, fmt.Errorf("invalid ISO 8601 timestamp: %s (supported formats: YYYY-MM-DDThh:mm:ssZ or YYYY-MM-DD)", timestamp)
}
