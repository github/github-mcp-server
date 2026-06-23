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

// issueWriteFieldInput is a user-friendly issue field input for issue_write.
// Field IDs and option IDs are resolved internally before calling the REST API.
type issueWriteFieldInput struct {
	FieldName       string
	Value           any
	FieldOptionName string
	Delete          bool
}

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
		Options        []struct {
			FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
			Name           githubv4.String
		}
	} `graphql:"... on IssueFieldSingleSelect"`
}

type issueFieldWriteMetadataQuery struct {
	Repository struct {
		IssueFields struct {
			Nodes []issueFieldWriteMetadataNode
		} `graphql:"issueFields(first: 100)"`
	} `graphql:"repository(owner: $owner, name: $repo)"`
}

// IssueFieldRef resolves the name of an issue field across its concrete types.
// IssueFields is a union of IssueFieldDate, IssueFieldNumber, IssueFieldSingleSelect, IssueFieldText,
// so we have to ask for `name` on each member.
type IssueFieldRef struct {
	Date struct {
		Name           githubv4.String
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
	} `graphql:"... on IssueFieldDate"`
	Number struct {
		Name           githubv4.String
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
	} `graphql:"... on IssueFieldNumber"`
	SingleSelect struct {
		Name           githubv4.String
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
	} `graphql:"... on IssueFieldSingleSelect"`
	Text struct {
		Name           githubv4.String
		FullDatabaseID githubv4.String `graphql:"fullDatabaseId"`
	} `graphql:"... on IssueFieldText"`
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

// FullDatabaseIDStr returns the fullDatabaseId string from whichever IssueFields union variant
// the field resolved to.
func (r IssueFieldRef) FullDatabaseIDStr() string {
	switch {
	case r.Date.FullDatabaseID != "":
		return string(r.Date.FullDatabaseID)
	case r.Number.FullDatabaseID != "":
		return string(r.Number.FullDatabaseID)
	case r.SingleSelect.FullDatabaseID != "":
		return string(r.SingleSelect.FullDatabaseID)
	case r.Text.FullDatabaseID != "":
		return string(r.Text.FullDatabaseID)
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

func optionalIssueWriteFields(args map[string]any) ([]issueWriteFieldInput, error) {
	issueFieldsRaw, exists := args["issue_fields"]
	if !exists {
		return nil, nil
	}

	var inputMaps []map[string]any
	switch v := issueFieldsRaw.(type) {
	case []any:
		for _, item := range v {
			itemMap, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("each issue_fields item must be an object")
			}
			inputMaps = append(inputMaps, itemMap)
		}
	case []map[string]any:
		inputMaps = v
	default:
		return nil, fmt.Errorf("issue_fields must be an array")
	}

	issueFields := make([]issueWriteFieldInput, 0, len(inputMaps))
	for _, itemMap := range inputMaps {
		fieldName, err := RequiredParam[string](itemMap, "field_name")
		if err != nil || strings.TrimSpace(fieldName) == "" {
			return nil, fmt.Errorf("field_name is required for each issue_fields item")
		}

		fieldOptionName, err := OptionalParam[string](itemMap, "field_option_name")
		if err != nil {
			return nil, err
		}

		deleteField, _ := OptionalParam[bool](itemMap, "delete")
		value, hasValue := itemMap["value"]
		if hasValue && value == nil {
			return nil, fmt.Errorf("value cannot be null for field %q", fieldName)
		}

		if deleteField {
			if hasValue || fieldOptionName != "" {
				return nil, fmt.Errorf("issue field %q cannot specify 'delete' together with 'value' or 'field_option_name'", fieldName)
			}
			issueFields = append(issueFields, issueWriteFieldInput{
				FieldName: fieldName,
				Delete:    true,
			})
			continue
		}

		if hasValue && fieldOptionName != "" {
			return nil, fmt.Errorf("issue field %q cannot specify both value and field_option_name", fieldName)
		}

		if !hasValue && fieldOptionName == "" {
			return nil, fmt.Errorf("issue field %q must specify either value or field_option_name", fieldName)
		}

		issueFields = append(issueFields, issueWriteFieldInput{
			FieldName:       fieldName,
			Value:           value,
			FieldOptionName: fieldOptionName,
		})
	}

	return issueFields, nil
}

func resolveIssueRequestFieldValues(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueFields []issueWriteFieldInput) ([]*github.IssueRequestFieldValue, []int64, error) {
	if len(issueFields) == 0 {
		return nil, nil, nil
	}

	ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issue_fields", "repo_issue_fields")
	var query issueFieldWriteMetadataQuery
	vars := map[string]any{
		"owner": githubv4.String(owner),
		"repo":  githubv4.String(repo),
	}
	if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
		return nil, nil, fmt.Errorf("failed to query issue fields metadata: %w", err)
	}

	// Build name → node map, dispatching on concrete type to extract name.
	fieldByName := make(map[string]issueFieldWriteMetadataNode, len(query.Repository.IssueFields.Nodes))
	for _, node := range query.Repository.IssueFields.Nodes {
		var name string
		switch string(node.TypeName) {
		case "IssueFieldText":
			name = string(node.IssueFieldText.Name)
		case "IssueFieldNumber":
			name = string(node.IssueFieldNumber.Name)
		case "IssueFieldDate":
			name = string(node.IssueFieldDate.Name)
		case "IssueFieldSingleSelect":
			name = string(node.IssueFieldSingleSelect.Name)
		default:
			continue
		}
		fieldByName[strings.ToLower(strings.TrimSpace(name))] = node
	}

	resolved := make([]*github.IssueRequestFieldValue, 0, len(issueFields))
	var fieldIDsToDelete []int64
	for _, fieldInput := range issueFields {
		node, ok := fieldByName[strings.ToLower(strings.TrimSpace(fieldInput.FieldName))]
		if !ok {
			return nil, nil, fmt.Errorf("issue field %q was not found in %s/%s", fieldInput.FieldName, owner, repo)
		}

		var fullDatabaseIDStr, dataType string
		switch string(node.TypeName) {
		case "IssueFieldText":
			fullDatabaseIDStr = string(node.IssueFieldText.FullDatabaseID)
			dataType = string(node.IssueFieldText.DataType)
		case "IssueFieldNumber":
			fullDatabaseIDStr = string(node.IssueFieldNumber.FullDatabaseID)
			dataType = string(node.IssueFieldNumber.DataType)
		case "IssueFieldDate":
			fullDatabaseIDStr = string(node.IssueFieldDate.FullDatabaseID)
			dataType = string(node.IssueFieldDate.DataType)
		case "IssueFieldSingleSelect":
			fullDatabaseIDStr = string(node.IssueFieldSingleSelect.FullDatabaseID)
			dataType = string(node.IssueFieldSingleSelect.DataType)
		}

		fieldID := parseFullDatabaseID(fullDatabaseIDStr)
		if fieldID == 0 {
			return nil, nil, fmt.Errorf("issue field %q is missing fullDatabaseId", fieldInput.FieldName)
		}

		if fieldInput.Delete {
			fieldIDsToDelete = append(fieldIDsToDelete, fieldID)
			continue
		}

		resolvedValue := fieldInput.Value
		if fieldInput.FieldOptionName != "" {
			if !strings.EqualFold(dataType, "single_select") {
				return nil, nil, fmt.Errorf("issue field %q is %q, so field_option_name cannot be used", fieldInput.FieldName, dataType)
			}

			optionFound := false
			for _, option := range node.IssueFieldSingleSelect.Options {
				if strings.EqualFold(strings.TrimSpace(string(option.Name)), strings.TrimSpace(fieldInput.FieldOptionName)) {
					// REST API expects the option name, not the ID
					resolvedValue = string(option.Name)
					optionFound = true
					break
				}
			}

			if !optionFound {
				return nil, nil, fmt.Errorf("issue field option %q was not found for field %q", fieldInput.FieldOptionName, fieldInput.FieldName)
			}
		}

		resolved = append(resolved, &github.IssueRequestFieldValue{
			FieldID: fieldID,
			Value:   resolvedValue,
		})
	}

	return resolved, fieldIDsToDelete, nil
}

// fetchExistingIssueFieldValues retrieves the current field values for an issue
// as IssueRequestFieldValue entries, ready to be merged before an update.
func fetchExistingIssueFieldValues(ctx context.Context, gqlClient *githubv4.Client, owner, repo string, issueNumber int) ([]*github.IssueRequestFieldValue, error) {
	ctxWithFeatures := ghcontext.WithGraphQLFeatures(ctx, "issue_fields", "repo_issue_fields")

	var query struct {
		Repository struct {
			Issue struct {
				IssueFieldValues struct {
					Nodes []IssueFieldValueFragment
				} `graphql:"issueFieldValues(first: 25)"`
			} `graphql:"issue(number: $number)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	vars := map[string]any{
		"owner":  githubv4.String(owner),
		"repo":   githubv4.String(repo),
		"number": githubv4.Int(issueNumber), // #nosec G115 - issue numbers are always small positive integers
	}

	if err := gqlClient.Query(ctxWithFeatures, &query, vars); err != nil {
		return nil, fmt.Errorf("failed to fetch existing issue field values: %w", err)
	}

	var result []*github.IssueRequestFieldValue
	for _, node := range query.Repository.Issue.IssueFieldValues.Nodes {
		var fieldIDStr string
		var value any

		switch node.TypeName {
		case "IssueFieldDateValue":
			fieldIDStr = node.DateValue.Field.FullDatabaseIDStr()
			value = string(node.DateValue.Value)
		case "IssueFieldNumberValue":
			fieldIDStr = node.NumberValue.Field.FullDatabaseIDStr()
			value = float64(node.NumberValue.Value)
		case "IssueFieldSingleSelectValue":
			fieldIDStr = node.SingleSelectValue.Field.FullDatabaseIDStr()
			value = string(node.SingleSelectValue.Value)
		case "IssueFieldTextValue":
			fieldIDStr = node.TextValue.Field.FullDatabaseIDStr()
			value = string(node.TextValue.Value)
		default:
			continue
		}

		fieldID := parseFullDatabaseID(fieldIDStr)
		if fieldID == 0 {
			continue
		}

		result = append(result, &github.IssueRequestFieldValue{
			FieldID: fieldID,
			Value:   value,
		})
	}

	return result, nil
}

// mergeIssueFieldValues returns a merged slice where incoming values override existing ones
// for the same field ID, and existing fields not present in incoming are preserved.
// Ordering is deterministic: incoming entries first in their original order, followed by any
// existing entries (in their original order) whose field IDs weren't seen in incoming.
func mergeIssueFieldValues(existing, incoming []*github.IssueRequestFieldValue) []*github.IssueRequestFieldValue {
	seen := make(map[int64]struct{}, len(incoming))
	result := make([]*github.IssueRequestFieldValue, 0, len(existing)+len(incoming))
	for _, v := range incoming {
		seen[v.FieldID] = struct{}{}
		result = append(result, v)
	}
	for _, v := range existing {
		if _, ok := seen[v.FieldID]; ok {
			continue
		}
		result = append(result, v)
	}
	return result
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
			attachIFC := newRepoVisibilityIFCLabeler(ctx, deps, client, owner, repo, ifc.LabelRepoUserContent)

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

	// Always drop the verbose REST IssueFieldValues; enrich with the GraphQL
	// field_values view instead.
	minimalIssue.IssueFieldValues = nil
	if issue != nil && issue.NodeID != nil && *issue.NodeID != "" {
		gqlClient, err := deps.GetGQLClient(ctx)
		if err == nil {
			if fieldValuesByID, err := fetchIssueFieldValuesByNodeID(ctx, gqlClient, []*github.Issue{issue}); err == nil {
				minimalIssue.FieldValues = fieldValuesByID[*issue.NodeID]
			}
		}
	}

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

// ListIssueTypes creates a tool to list defined issue types for an organization or repository.
// This can be used to understand supported issue type values for creating or updating issues.
func ListIssueTypes(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "list_issue_types",
			Description: t("TOOL_LIST_ISSUE_TYPES_FOR_ORG", "List supported issue types for a repository or its owner organization. When repo is omitted, returns org-level issue types directly."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_LIST_ISSUE_TYPES_USER_TITLE", "List available issue types"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "The account owner of the repository or organization.",
					},
					"repo": {
						Type:        "string",
						Description: "The name of the repository. When provided, returns issue types for this specific repository. When omitted, returns org-level issue types directly.",
					},
				},
				Required: []string{"owner"},
			},
		},
		[]scopes.Scope{scopes.Repo, scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := OptionalParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			if repo != "" {
				apiURL := fmt.Sprintf("repos/%s/%s/issue-types", owner, repo)
				req, err := client.NewRequest(ctx, "GET", apiURL, nil)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to create request", err), nil, nil
				}
				var issueTypes []*github.IssueType
				resp, err := client.Do(req, &issueTypes)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to list issue types", resp, err), nil, nil
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

				result := utils.NewToolResultText(string(r))
				result = attachRepoVisibilityIFCLabelLazy(ctx, deps, owner, repo, result, ifc.LabelRepoMetadata)
				return result, nil, nil
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

			result := utils.NewToolResultText(string(r))
			// Issue types are org-defined structural metadata (trusted, not
			// attacker-authored). They are scoped to an organization rather
			// than a single repo, so confidentiality is conservatively treated
			// as private (restricted to org members).
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelRepoMetadata(true))
			return result, nil, nil
		})
}

// AddIssueComment creates a tool to add a comment or reaction to an issue.
func AddIssueComment(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "add_issue_comment",
			Description: t("TOOL_ADD_ISSUE_COMMENT_DESCRIPTION", "Add a comment and/or reaction to a specific issue in a GitHub repository. Use this tool with pull requests as well (in this case pass pull request number as issue_number), but only if user is not asking specifically to add or react to review comments. At least one of body or reaction is required."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ADD_ISSUE_COMMENT_USER_TITLE", "Add comment to issue or pull request"),
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
						Description: "Issue number to comment on or react to",
					},
					"body": {
						Type:        "string",
						Description: "Comment content. Required unless reaction is provided.",
					},
					"reaction": {
						Type:        "string",
						Description: "Emoji reaction to add. Required unless body is provided.",
						Enum:        []any{"+1", "-1", "laugh", "confused", "heart", "hooray", "rocket", "eyes"},
					},
				},
				Required: []string{"owner", "repo", "issue_number"},
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
			body, hasBody, err := OptionalParamOK[string](args, "body")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			reactionContent, hasReaction, err := OptionalParamOK[string](args, "reaction")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			if !hasBody && !hasReaction {
				return utils.NewToolResultError("at least one of body or reaction is required"), nil, nil
			}
			if hasBody && body == "" {
				return utils.NewToolResultError("body cannot be empty when provided"), nil, nil
			}
			if hasReaction && reactionContent == "" {
				return utils.NewToolResultError("reaction cannot be empty when provided"), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return utils.NewToolResultErrorFromErr("failed to get GitHub client", err), nil, nil
			}

			var commentResponse *MinimalResponse
			if hasBody {
				comment := &github.IssueComment{
					Body: github.Ptr(body),
				}
				createdComment, resp, err := client.Issues.CreateComment(ctx, owner, repo, issueNumber, comment)
				if err != nil {
					return utils.NewToolResultErrorFromErr("failed to create comment", err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				if resp.StatusCode != http.StatusCreated {
					bodyBytes, err := io.ReadAll(resp.Body)
					if err != nil {
						return utils.NewToolResultErrorFromErr("failed to read response body", err), nil, nil
					}
					return ghErrors.NewGitHubAPIStatusErrorResponse(ctx, "failed to create comment", resp, bodyBytes), nil, nil
				}

				commentResponse = &MinimalResponse{
					ID:  fmt.Sprintf("%d", createdComment.GetID()),
					URL: createdComment.GetHTMLURL(),
				}
			}

			var reactionResponse *MinimalResponse
			if hasReaction {
				reaction, resp, err := client.Reactions.CreateIssueReaction(ctx, owner, repo, issueNumber, reactionContent)
				if err != nil {
					return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to add reaction to issue", resp, err), nil, nil
				}
				defer func() { _ = resp.Body.Close() }()

				reactionResponse = &MinimalResponse{
					ID:  fmt.Sprintf("%d", reaction.GetID()),
					URL: fmt.Sprintf("%srepos/%s/%s/issues/%d/reactions/%d", client.BaseURL(), owner, repo, issueNumber, reaction.GetID()),
				}
			}

			var result any
			switch {
			case hasBody && hasReaction:
				result = map[string]MinimalResponse{
					"comment":  *commentResponse,
					"reaction": *reactionResponse,
				}
			case hasReaction:
				result = reactionResponse
			default:
				result = commentResponse
			}

			r, err := json.Marshal(result)
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
	st.FeatureFlagDisable = []string{FeatureFlagIssuesGranular}
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
			result, err := searchIssuesHandler(ctx, deps, args, ifcSearchPostProcessOption(ctx, deps))
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
	FieldValues []MinimalFieldValue `json:"field_values,omitempty"`
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
func fetchIssueFieldValuesByNodeID(ctx context.Context, gqlClient *githubv4.Client, issues []*github.Issue) (map[string][]MinimalFieldValue, error) {
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

	result := make(map[string][]MinimalFieldValue, len(q.Nodes))
	for _, n := range q.Nodes {
		idStr, ok := n.Issue.ID.(string)
		if !ok || idStr == "" {
			continue
		}
		vals := make([]MinimalFieldValue, 0, len(n.Issue.IssueFieldValues.Nodes))
		for _, fv := range n.Issue.IssueFieldValues.Nodes {
			if m, ok := fragmentToMinimalFieldValue(fv); ok {
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

	var fieldValuesByID map[string][]MinimalFieldValue
	if len(result.Issues) > 0 {
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

// IssueWriteUIResourceURI is the URI for the issue_write tool's MCP App UI resource.
const IssueWriteUIResourceURI = "ui://github-mcp-server/issue-write"

// issueWriteFormParams are the parameters the issue_write MCP App form collects
// and re-sends on submit. Any other parameter present on a call cannot be
// represented by the form.
var issueWriteFormParams = map[string]struct{}{
	"method":        {},
	"owner":         {},
	"repo":          {},
	"title":         {},
	"body":          {},
	"issue_number":  {},
	"issue_fields":  {},
	"state":         {},
	"state_reason":  {},
	"duplicate_of":  {},
	"show_ui":       {},
	"_ui_submitted": {},
}

// issueWriteHasNonFormParams reports whether the call carries any parameter the
// issue_write MCP App form cannot represent (anything outside issueWriteFormParams,
// e.g. labels, assignees, milestones or issue types). Such calls must bypass
// the UI form and execute directly so the supplied values aren't silently dropped.
func issueWriteHasNonFormParams(args map[string]any) bool {
	for key, value := range args {
		if value == nil {
			continue
		}
		if _, ok := issueWriteFormParams[key]; !ok {
			return true
		}
	}
	return false
}

// issueWriteAwaitingFormResult builds the "awaiting form submission" stub
// returned when issue_write hands off to the MCP App form. The body is shared
// by IssueWrite and LegacyIssueWrite. The result is marked IsError=true so
// agents that bail on error don't claim success or chain dependent tool calls
// while the user is still interacting with the form; the host renders the UI
// regardless because rendering is keyed off the tool's _meta.ui resourceUri.
func issueWriteAwaitingFormResult(method, owner, repo string, issueNumber int) *mcp.CallToolResult {
	var msg string
	if method == "update" {
		msg = fmt.Sprintf(
			"An interactive form has been shown to the user for editing issue #%d in %s/%s. "+
				"STOP — do not call any other tools, do not respond as if the issue was updated, "+
				"and do not claim the operation succeeded. The issue has NOT been updated yet; "+
				"only the form was rendered. Wait silently for the user to review and click Submit. "+
				"When they do, the real result will be delivered to your context automatically.",
			issueNumber, owner, repo,
		)
	} else {
		msg = fmt.Sprintf(
			"An interactive form has been shown to the user for creating a new issue in %s/%s. "+
				"STOP — do not call any other tools, do not respond as if the issue was created, "+
				"and do not claim the operation succeeded. The issue has NOT been created yet; "+
				"only the form was rendered. Wait silently for the user to review and click Submit. "+
				"When they do, the real result will be delivered to your context automatically.",
			owner, repo,
		)
	}
	return utils.NewToolResultAwaitingFormSubmission(msg)
}

// IssueWrite is the FeatureFlagIssueFields-enabled variant of issue_write
// (with the issue_fields parameter). LegacyIssueWrite is served when the flag
// is off. Both register under the tool name "issue_write"; exactly one is
// active at a time via mutually exclusive feature-flag annotations. When the
// flag is removed, delete LegacyIssueWrite outright and drop the feature-flag
// fields on IssueWrite.
func IssueWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	st := NewTool(
		ToolsetMetadataIssues,
		mcp.Tool{
			Name:        "issue_write",
			Description: t("TOOL_ISSUE_WRITE_DESCRIPTION", "Create a new or update an existing issue in a GitHub repository."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_ISSUE_WRITE_USER_TITLE", "Create or update issue/pull request"),
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
						Description: "Type of this issue. Only use if issue types are enabled for this repository. Use list_issue_types tool to get valid type values for this repository or its owner organization. If the repository doesn't support issue types, omit this parameter.",
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
						Description: "Issue field values to set or clear. Each item requires 'field_name' and exactly one of 'value', 'field_option_name', or 'delete: true'.",
						Items: &jsonschema.Schema{
							Type:                 "object",
							AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
							Properties: map[string]*jsonschema.Schema{
								"field_name": {
									Type: "string",
									Description: "Issue field name (case-insensitive). Must match a field " +
										"returned by list_issue_fields for this repository or its organization.",
								},
								"value": {
									Types: []string{"string", "number", "boolean"},
									Description: "Value to set. Use for text, number, and date fields " +
										"(date as YYYY-MM-DD). For single-select fields, prefer " +
										"'field_option_name' so the option is validated before the API " +
										"call. Cannot be combined with 'field_option_name' or 'delete'.",
								},
								"field_option_name": {
									Type: "string",
									Description: "Option name for single-select fields. Validated against " +
										"the field's options before the API call. Cannot be combined with " +
										"'value' or 'delete'.",
								},
								"delete": {
									Type: "boolean",
									Enum: []any{true},
									Description: "Set to true to clear this field's current value on the " +
										"issue. Cannot be combined with 'value' or 'field_option_name'.",
								},
							},
							Required: []string{"field_name"},
						},
					},
					// show_ui is hidden from clients that do not advertise MCP App
					// UI support. The strip happens per-request in
					// inventory.ToolsForRegistration; it is present in the static
					// schema (and therefore in toolsnaps and the feature-flag /
					// insiders docs) so the UI-capable surface is fully
					// documented. It is intentionally not in the main README,
					// which renders the stripped (non-UI) schema.
					"show_ui": {
						Type:        "boolean",
						Description: "Whether to render the MCP App form instead of executing the request immediately. Defaults to true. Set to false to skip the form and execute directly — useful when you have all required values (especially ones the form does not collect, like labels, assignees, milestone, type, issue_fields, or state changes) and the user has already confirmed the action.",
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

			// When MCP Apps are enabled and the client supports UI, route the
			// call to the interactive form unless:
			//   - it is itself a form submission (the UI sends _ui_submitted=true),
			//   - the caller explicitly asked to skip the UI (show_ui=false), or
			//   - it carries parameters the form cannot represent (e.g. labels,
			//     assignees or issue_fields). Those must be applied directly so
			//     their values aren't silently dropped.
			uiSubmitted, _ := OptionalParam[bool](args, "_ui_submitted")
			showUI, err := OptionalBoolParamWithDefault(args, "show_ui", true)
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			if deps.IsFeatureEnabled(ctx, MCPAppsFeatureFlag) && clientSupportsUI(ctx, req) && !uiSubmitted && showUI && !issueWriteHasNonFormParams(args) {
				issueNumber := 0
				if method == "update" {
					n, numErr := RequiredInt(args, "issue_number")
					if numErr != nil {
						return utils.NewToolResultError("issue_number is required for update method"), nil, nil
					}
					issueNumber = n
				}
				return issueWriteAwaitingFormResult(method, owner, repo, issueNumber), nil, nil
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
			assigneesValue, assigneesProvided := args["assignees"]
			assigneesProvided = assigneesProvided && assigneesValue != nil

			// Get labels
			labels, err := OptionalStringArrayParam(args, "labels")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			labelsValue, labelsProvided := args["labels"]
			labelsProvided = labelsProvided && labelsValue != nil

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

			var issueFields []issueWriteFieldInput
			issueFields, err = optionalIssueWriteFields(args)
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

			var issueFieldValues []*github.IssueRequestFieldValue
			var fieldIDsToDelete []int64
			if len(issueFields) > 0 {
				issueFieldValues, fieldIDsToDelete, err = resolveIssueRequestFieldValues(ctx, gqlClient, owner, repo, issueFields)
				if err != nil {
					return utils.NewToolResultError(fmt.Sprintf("failed to resolve issue_fields: %v", err)), nil, nil
				}
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
				result, err := UpdateIssue(ctx, client, gqlClient, owner, repo, issueNumber, title, body, assignees, labels, milestoneNum, issueType, issueFieldValues, fieldIDsToDelete, state, stateReason, duplicateOf, UpdateIssueOptions{
					AssigneesProvided: assigneesProvided,
					LabelsProvided:    labelsProvided,
				})
				return result, nil, err
			default:
				return utils.NewToolResultError("invalid method, must be either 'create' or 'update'"), nil, nil
			}
		})
	st.FeatureFlagDisable = []string{FeatureFlagIssuesGranular}
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

// UpdateIssueOptions controls which optional fields are included in an issue update request.
type UpdateIssueOptions struct {
	// AssigneesProvided sends the assignees field even when the slice is empty.
	AssigneesProvided bool
	// LabelsProvided sends the labels field even when the slice is empty.
	LabelsProvided bool
}

func UpdateIssue(ctx context.Context, client *github.Client, gqlClient *githubv4.Client, owner string, repo string, issueNumber int, title string, body string, assignees []string, labels []string, milestoneNum int, issueType string, issueFieldValues []*github.IssueRequestFieldValue, fieldIDsToDelete []int64, state string, stateReason string, duplicateOf int, opts ...UpdateIssueOptions) (*mcp.CallToolResult, error) {
	updateOptions := UpdateIssueOptions{
		AssigneesProvided: len(assignees) > 0,
		LabelsProvided:    len(labels) > 0,
	}
	for _, opt := range opts {
		updateOptions.AssigneesProvided = updateOptions.AssigneesProvided || opt.AssigneesProvided
		updateOptions.LabelsProvided = updateOptions.LabelsProvided || opt.LabelsProvided
	}

	// Create the issue request with only provided fields
	issueRequest := &github.IssueRequest{}

	// Set optional parameters if provided
	if title != "" {
		issueRequest.Title = github.Ptr(title)
	}

	if body != "" {
		issueRequest.Body = github.Ptr(body)
	}

	if updateOptions.LabelsProvided {
		issueRequest.Labels = &labels
	}

	if updateOptions.AssigneesProvided {
		issueRequest.Assignees = &assignees
	}

	if milestoneNum != 0 {
		issueRequest.Milestone = &milestoneNum
	}

	if issueType != "" {
		issueRequest.Type = github.Ptr(issueType)
	}

	if len(issueFieldValues) > 0 || len(fieldIDsToDelete) > 0 {
		// The REST update endpoint uses "set" semantics — it overwrites all existing
		// field values with whatever is sent. Fetch the current values first, merge in
		// the new values, then remove any explicitly deleted fields.
		existing, err := fetchExistingIssueFieldValues(ctx, gqlClient, owner, repo, issueNumber)
		if err != nil {
			return ghErrors.NewGitHubGraphQLErrorResponse(ctx, "failed to fetch existing issue field values", err), nil
		}
		merged := mergeIssueFieldValues(existing, issueFieldValues)
		if len(fieldIDsToDelete) > 0 {
			deleteSet := make(map[int64]bool, len(fieldIDsToDelete))
			for _, id := range fieldIDsToDelete {
				deleteSet[id] = true
			}
			kept := make([]*github.IssueRequestFieldValue, 0, len(merged))
			for _, v := range merged {
				if !deleteSet[v.FieldID] {
					kept = append(kept, v)
				}
			}
			merged = kept
		}
		issueRequest.IssueFieldValues = merged
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

// ListIssues creates a tool to list and filter repository issues. It exposes the
// Issues 2.0 field_filters input plus field_values output enrichment.
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
			result = attachStaticIFCLabel(ctx, deps, result, ifc.LabelListIssues(isPrivate))
			return result, nil, nil
		})
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
