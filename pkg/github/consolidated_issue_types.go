package github

import (
	"encoding/json"
	"fmt"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type IssueReadInput struct {
	Method      string  `json:"method"`
	Owner       string  `json:"owner"`
	Repo        string  `json:"repo"`
	IssueNumber int     `json:"issue_number"`
	Page        *int    `json:"page,omitempty"`
	PerPage     *int    `json:"perPage,omitempty"`
	After       *string `json:"after,omitempty"`
}

type SubIssueWriteInput struct {
	Method        string `json:"method"`
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	IssueNumber   int    `json:"issue_number"`
	SubIssueID    int    `json:"sub_issue_id"`
	ReplaceParent bool   `json:"replace_parent,omitempty"`
	AfterID       *int   `json:"after_id,omitempty"`
	BeforeID      *int   `json:"before_id,omitempty"`
}

type IssueWriteInput struct {
	Method            string                 `json:"method"`
	Owner             string                 `json:"owner"`
	Repo              string                 `json:"repo"`
	IssueNumber       *int                   `json:"issue_number,omitempty"`
	ParentIssueNumber *int                   `json:"parent_issue_number,omitempty"`
	ParentOwner       *string                `json:"parent_owner,omitempty"`
	ParentRepo        *string                `json:"parent_repo,omitempty"`
	Title             *string                `json:"title,omitempty"`
	Body              *string                `json:"body,omitempty"`
	Assignees         *[]string              `json:"assignees,omitempty"`
	Labels            *[]string              `json:"labels,omitempty"`
	Milestone         *int                   `json:"milestone,omitempty"`
	Type              *string                `json:"type,omitempty"`
	State             *string                `json:"state,omitempty"`
	StateReason       *string                `json:"state_reason,omitempty"`
	DuplicateOf       *int                   `json:"duplicate_of,omitempty"`
	IssueFields       []IssueWriteFieldInput `json:"issue_fields,omitempty"`
	TypeProvided      bool                   `json:"-"`
}

func (input *IssueWriteInput) UnmarshalJSON(raw []byte) error {
	type plain IssueWriteInput
	var decoded plain
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	var presence struct {
		Type json.RawMessage `json:"type"`
	}
	if err := json.Unmarshal(raw, &presence); err != nil {
		return err
	}
	*input = IssueWriteInput(decoded)
	input.TypeProvided = len(presence.Type) != 0
	return nil
}

type IssueWriteFieldInput struct {
	FieldName       string           `json:"field_name"`
	Value           *IssueFieldValue `json:"value,omitempty"`
	FieldOptionName *string          `json:"field_option_name,omitempty"`
	Delete          *bool            `json:"delete,omitempty"`
}

// IssueFieldValue represents the REST scalar without losing its JSON type.
// Boolean inputs were historically accepted and forwarded to the API as well.
type IssueFieldValue struct {
	String *string
	Number *float64
	Bool   *bool
}

func (value IssueFieldValue) MarshalJSON() ([]byte, error) {
	switch {
	case value.String != nil:
		return json.Marshal(value.String)
	case value.Number != nil:
		return json.Marshal(value.Number)
	case value.Bool != nil:
		return json.Marshal(value.Bool)
	default:
		return []byte("null"), nil
	}
}

func (value *IssueFieldValue) UnmarshalJSON(raw []byte) error {
	*value = IssueFieldValue{}
	switch {
	case len(raw) > 0 && raw[0] == '"':
		return json.Unmarshal(raw, &value.String)
	case string(raw) == "true" || string(raw) == "false":
		return json.Unmarshal(raw, &value.Bool)
	case string(raw) == "null":
		return nil
	default:
		return json.Unmarshal(raw, &value.Number)
	}
}

func issueFieldValue(value any) (*IssueFieldValue, error) {
	switch value := value.(type) {
	case nil:
		return nil, nil
	case string:
		return &IssueFieldValue{String: &value}, nil
	case float64:
		return &IssueFieldValue{Number: &value}, nil
	case bool:
		return &IssueFieldValue{Bool: &value}, nil
	default:
		return nil, fmt.Errorf("unsupported issue field value type %T", value)
	}
}

// The read tool returns different top-level shapes, not an artificial envelope.
type IssueReadOutput struct {
	Issue     *IssueDetailsOutput
	Comments  *[]MinimalIssueComment
	SubIssues *[]*SubIssueOutput
	Parent    *IssueParentOutput
	Labels    *IssueLabelsOutput
}

func (out IssueReadOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.Issue != nil:
		return json.Marshal(out.Issue)
	case out.Comments != nil:
		return json.Marshal(out.Comments)
	case out.SubIssues != nil:
		return json.Marshal(out.SubIssues)
	case out.Parent != nil:
		return json.Marshal(out.Parent)
	case out.Labels != nil:
		return json.Marshal(out.Labels)
	default:
		// SDK v1.8 validates this zero value even for errors before the
		// protocol middleware removes it from the wire result.
		return []byte("null"), nil
	}
}

// IssueDetailsOutput excludes REST issue_field_values: get always drops them
// and replaces them with the GraphQL field_values representation.
type IssueDetailsOutput struct {
	Number               int                         `json:"number"`
	Title                string                      `json:"title"`
	Body                 string                      `json:"body,omitempty"`
	State                string                      `json:"state"`
	StateReason          string                      `json:"state_reason,omitempty"`
	Draft                bool                        `json:"draft,omitempty"`
	Locked               bool                        `json:"locked,omitempty"`
	HTMLURL              string                      `json:"html_url,omitempty"`
	User                 *MinimalUser                `json:"user,omitempty"`
	AuthorAssociation    string                      `json:"author_association,omitempty"`
	Labels               []string                    `json:"labels,omitempty"`
	Assignees            []string                    `json:"assignees"`
	Milestone            string                      `json:"milestone,omitempty"`
	Comments             int                         `json:"comments,omitempty"`
	Reactions            *MinimalReactions           `json:"reactions,omitempty"`
	CreatedAt            string                      `json:"created_at,omitempty"`
	UpdatedAt            string                      `json:"updated_at,omitempty"`
	ClosedAt             string                      `json:"closed_at,omitempty"`
	ClosedBy             string                      `json:"closed_by,omitempty"`
	IssueType            string                      `json:"issue_type,omitempty"`
	FieldValues          []MinimalFieldValue         `json:"field_values,omitempty"`
	HasParent            *bool                       `json:"has_parent,omitempty"`
	HasChildren          *bool                       `json:"has_children,omitempty"`
	Parent               *MinimalIssueRef            `json:"parent,omitempty"`
	SubIssuesSummary     *MinimalSubIssuesSummary    `json:"sub_issues_summary,omitempty"`
	ClosedByPullRequests *MinimalClosingPullRequests `json:"closed_by_pull_requests,omitempty"`
}

func issueDetailsOutput(issue MinimalIssue) *IssueDetailsOutput {
	return &IssueDetailsOutput{
		Number: issue.Number, Title: issue.Title, Body: issue.Body, State: issue.State,
		StateReason: issue.StateReason, Draft: issue.Draft, Locked: issue.Locked,
		HTMLURL: issue.HTMLURL, User: issue.User, AuthorAssociation: issue.AuthorAssociation,
		Labels: issue.Labels, Assignees: issue.Assignees, Milestone: issue.Milestone,
		Comments: issue.Comments, Reactions: issue.Reactions, CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt, ClosedAt: issue.ClosedAt, ClosedBy: issue.ClosedBy,
		IssueType: issue.IssueType, FieldValues: issue.FieldValues, HasParent: issue.HasParent,
		HasChildren: issue.HasChildren, Parent: issue.Parent, SubIssuesSummary: issue.SubIssuesSummary,
		ClosedByPullRequests: issue.ClosedByPullRequests,
	}
}

type IssueParentOutput struct {
	Parent *IssueParentRef `json:"parent"`
}

// These fields keep the original map's lexicographic JSON ordering.
type IssueParentRef struct {
	Number     int    `json:"number"`
	Repository string `json:"repository"`
	State      string `json:"state"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

type IssueLabelsOutput struct {
	Labels     []IssueLabelOutput `json:"labels"`
	TotalCount int                `json:"totalCount"`
}

type IssueWriteOutput struct {
	Issue    *MinimalResponse
	Awaiting *IssueWriteAwaitingOutput
}

type IssueWriteAwaitingOutput struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (out IssueWriteOutput) MarshalJSON() ([]byte, error) {
	if out.Awaiting != nil {
		return json.Marshal(out.Awaiting)
	}
	if out.Issue != nil {
		return json.Marshal(out.Issue)
	}
	return json.Marshal(MinimalResponse{})
}

func issueWriteOutputSchema() *jsonschema.Schema {
	mutation := repositoryOutputSchema[MinimalResponse]()
	awaiting := repositoryOutputSchema[IssueWriteAwaitingOutput]()
	awaiting.Properties["status"].Enum = []any{"awaiting_user_submission"}
	return repositoryUnionSchema(mutation, awaiting)
}

func issueWriteResult(result *mcp.CallToolResult, output *MinimalResponse, err error) (*mcp.CallToolResult, *IssueWriteOutput, error) {
	if output == nil {
		return result, nil, err
	}
	return result, &IssueWriteOutput{Issue: output}, err
}

func issueWriteFieldVariants() []*jsonschema.Schema {
	return []*jsonschema.Schema{
		{
			Required: []string{"value"},
			Properties: map[string]*jsonschema.Schema{
				"field_option_name": {Type: "string", Enum: []any{""}}, "delete": {Type: "boolean", Enum: []any{false}},
			},
		},
		{
			Required: []string{"field_option_name"},
			Properties: map[string]*jsonschema.Schema{
				"field_option_name": {Type: "string", MinLength: new(1)},
				"value":             {Not: &jsonschema.Schema{}}, "delete": {Type: "boolean", Enum: []any{false}},
			},
		},
		{
			Required: []string{"delete"},
			Properties: map[string]*jsonschema.Schema{
				"delete": {Type: "boolean", Enum: []any{true}}, "value": {Not: &jsonschema.Schema{}}, "field_option_name": {Type: "string", Enum: []any{""}},
			},
		},
	}
}

type IssueLabelOutput struct {
	Color       string `json:"color"`
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
}

type SubIssueOutput struct {
	ID                       *int64                           `json:"id,omitempty"`
	Number                   *int                             `json:"number,omitempty"`
	State                    *string                          `json:"state,omitempty"`
	StateReason              *string                          `json:"state_reason,omitempty"`
	Locked                   *bool                            `json:"locked,omitempty"`
	Title                    *string                          `json:"title,omitempty"`
	Body                     *string                          `json:"body,omitempty"`
	AuthorAssociation        *string                          `json:"author_association,omitempty"`
	User                     *github.User                     `json:"user,omitempty"`
	Labels                   []*github.Label                  `json:"labels,omitempty"`
	Assignee                 *github.User                     `json:"assignee,omitempty"`
	Comments                 *int                             `json:"comments,omitempty"`
	ClosedAt                 *github.Timestamp                `json:"closed_at,omitempty"`
	CreatedAt                *github.Timestamp                `json:"created_at,omitempty"`
	UpdatedAt                *github.Timestamp                `json:"updated_at,omitempty"`
	ClosedBy                 *github.User                     `json:"closed_by,omitempty"`
	URL                      *string                          `json:"url,omitempty"`
	HTMLURL                  *string                          `json:"html_url,omitempty"`
	CommentsURL              *string                          `json:"comments_url,omitempty"`
	EventsURL                *string                          `json:"events_url,omitempty"`
	LabelsURL                *string                          `json:"labels_url,omitempty"`
	RepositoryURL            *string                          `json:"repository_url,omitempty"`
	ParentIssueURL           *string                          `json:"parent_issue_url,omitempty"`
	Milestone                *github.Milestone                `json:"milestone,omitempty"`
	PullRequestLinks         *github.PullRequestLinks         `json:"pull_request,omitempty"`
	Repository               *github.Repository               `json:"repository,omitempty"`
	Reactions                *github.Reactions                `json:"reactions,omitempty"`
	Assignees                []*github.User                   `json:"assignees,omitempty"`
	NodeID                   *string                          `json:"node_id,omitempty"`
	Draft                    *bool                            `json:"draft,omitempty"`
	Type                     *github.IssueType                `json:"type,omitempty"`
	PinnedComment            *github.IssueComment             `json:"pinned_comment,omitempty"`
	PerformedViaGithubApp    *github.App                      `json:"performed_via_github_app,omitempty"`
	IssueDependenciesSummary *github.IssueDependenciesSummary `json:"issue_dependencies_summary,omitempty"`
	SubIssuesSummary         *github.SubIssuesSummary         `json:"sub_issues_summary,omitempty"`
	IssueFieldValues         []*SubIssueFieldValueOutput      `json:"issue_field_values,omitempty"`
	TextMatches              []*github.TextMatch              `json:"text_matches,omitempty"`
	ActiveLockReason         *string                          `json:"active_lock_reason,omitempty"`
}

type SubIssueFieldValueOutput struct {
	IssueFieldID       int64                                     `json:"issue_field_id"`
	NodeID             string                                    `json:"node_id"`
	DataType           string                                    `json:"data_type"`
	Value              *IssueFieldValue                          `json:"value"`
	SingleSelectOption *github.IssueFieldValueSingleSelectOption `json:"single_select_option,omitempty"`
}

func subIssueOutput(issue *github.SubIssue) (*SubIssueOutput, error) {
	if issue == nil {
		return nil, nil
	}
	out := &SubIssueOutput{
		ID: issue.ID, Number: issue.Number, State: issue.State, StateReason: issue.StateReason,
		Locked: issue.Locked, Title: issue.Title, Body: issue.Body,
		AuthorAssociation: issue.AuthorAssociation, //nolint:staticcheck // Preserve the existing REST response, including legacy fields.
		User:              issue.User, Labels: issue.Labels, Comments: issue.Comments,
		Assignee: issue.Assignee, //nolint:staticcheck // Preserve the singular assignee when supplied by older API versions.
		ClosedAt: issue.ClosedAt, CreatedAt: issue.CreatedAt, UpdatedAt: issue.UpdatedAt,
		ClosedBy: issue.ClosedBy, URL: issue.URL, HTMLURL: issue.HTMLURL, CommentsURL: issue.CommentsURL,
		EventsURL: issue.EventsURL, LabelsURL: issue.LabelsURL, RepositoryURL: issue.RepositoryURL,
		ParentIssueURL: issue.ParentIssueURL, Milestone: issue.Milestone, PullRequestLinks: issue.PullRequestLinks,
		Repository: issue.Repository, Reactions: issue.Reactions, Assignees: issue.Assignees,
		NodeID: issue.NodeID, Draft: issue.Draft, Type: issue.Type, PinnedComment: issue.PinnedComment,
		PerformedViaGithubApp: issue.PerformedViaGithubApp, IssueDependenciesSummary: issue.IssueDependenciesSummary,
		SubIssuesSummary: issue.SubIssuesSummary, TextMatches: issue.TextMatches, ActiveLockReason: issue.ActiveLockReason,
	}
	for _, field := range issue.IssueFieldValues {
		if field == nil {
			out.IssueFieldValues = append(out.IssueFieldValues, nil)
			continue
		}
		value, err := issueFieldValue(field.Value)
		if err != nil {
			return nil, err
		}
		out.IssueFieldValues = append(out.IssueFieldValues, &SubIssueFieldValueOutput{
			IssueFieldID: field.IssueFieldID, NodeID: field.NodeID, DataType: field.DataType,
			Value: value, SingleSelectOption: field.SingleSelectOption,
		})
	}
	return out, nil
}

func issueReadOutputSchema() *jsonschema.Schema {
	issue := repositoryOutputSchema[IssueDetailsOutput]()
	comments := repositoryOutputSchema[[]MinimalIssueComment]()
	subIssues := subIssueArraySchema()
	arrayDefinitions := subIssues.Defs
	subIssues.Defs = nil
	parent := repositoryOutputSchema[IssueParentOutput]()
	labels := repositoryOutputSchema[IssueLabelsOutput]()
	// Comments and sub-issues are both arrays; an empty array (or an array
	// of nulls) is valid for either, so they form one anyOf array variant.
	return repositoryUnionSchema(
		&jsonschema.Schema{Type: "null"},
		issue,
		&jsonschema.Schema{Type: "array", AnyOf: []*jsonschema.Schema{comments, subIssues}, Defs: arrayDefinitions},
		parent,
		labels,
	)
}

type SubIssueWriteOutput struct {
	Issue *SubIssueOutput
}

func (out SubIssueWriteOutput) MarshalJSON() ([]byte, error) {
	// Unlike the SDK's default nil-pointer handling, a null API response
	// must remain null rather than becoming an empty sub-issue object.
	return json.Marshal(out.Issue)
}

func subIssueWriteResult(result *mcp.CallToolResult, output *SubIssueOutput, err error) (*mcp.CallToolResult, *SubIssueWriteOutput, error) {
	return result, &SubIssueWriteOutput{Issue: output}, err
}

func subIssueWriteOutputSchema() *jsonschema.Schema {
	schema := subIssueSchema()
	schema.Type = ""
	schema.Types = []string{"object", "null"}
	return schema
}

func subIssueSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[SubIssueOutput]()
	replaceIssueFieldScalar(schema)
	return schema
}

func subIssueArraySchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[[]*SubIssueOutput]()
	replaceIssueFieldScalar(schema)
	return schema
}

func replaceIssueFieldScalar(schema *jsonschema.Schema) {
	// jsonschema's type override is not needed for the surrounding API
	// definitions; replace only the custom scalar's inferred properties.
	if schema == nil {
		return
	}
	if fields := schema.Properties["issue_field_values"]; fields != nil && fields.Items != nil {
		fields.Items.Properties["value"] = &jsonschema.Schema{Types: []string{"string", "number", "boolean", "null"}}
	}
	replaceIssueFieldScalar(schema.Items)
}

func normalizeConsolidatedIssueArguments(kind string) func(json.RawMessage) (json.RawMessage, error) {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, field := range []string{"method", "owner", "repo"} {
			if _, err := RequiredParam[string](args, field); err != nil {
				return nil, err
			}
		}
		requiredInts := []string{"issue_number"}
		switch kind {
		case "write":
			requiredInts = nil
		case "sub":
			requiredInts = append(requiredInts, "sub_issue_id")
		}
		for _, field := range requiredInts {
			value, err := RequiredInt(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		if kind == "read" {
			pagination, err := OptionalPaginationParams(args)
			if err != nil {
				return nil, err
			}
			args["page"], args["perPage"] = pagination.Page, pagination.PerPage
		}
		if kind == "sub" {
			if _, err := OptionalParam[bool](args, "replace_parent"); err != nil {
				return nil, err
			}
			for _, field := range []string{"after_id", "before_id"} {
				if _, exists := args[field]; exists {
					value, err := OptionalIntParam(args, field)
					if err != nil {
						return nil, err
					}
					args[field] = value
				}
			}
		}
		if kind == "write" {
			for _, field := range []string{"title", "body"} {
				if _, err := OptionalParam[string](args, field); err != nil {
					return nil, err
				}
			}
			for _, field := range []string{"assignees", "labels"} {
				if _, err := OptionalStringArrayParam(args, field); err != nil {
					return nil, err
				}
				if args[field] == nil {
					delete(args, field)
				}
			}
			milestone, err := OptionalIntParam(args, "milestone")
			if err != nil {
				return nil, err
			}
			if _, exists := args["milestone"]; exists {
				args["milestone"] = milestone
			}
			if _, _, err := OptionalNullableStringParam(args, "type"); err != nil {
				return nil, err
			}
			state, err := OptionalParam[string](args, "state")
			if err != nil {
				return nil, err
			}
			reason, err := OptionalParam[string](args, "state_reason")
			if err != nil {
				return nil, err
			}
			duplicate, err := OptionalIntParam(args, "duplicate_of")
			if err != nil {
				return nil, err
			}
			if _, exists := args["duplicate_of"]; exists {
				args["duplicate_of"] = duplicate
			}
			if duplicate != 0 && reason != "duplicate" {
				return nil, fmt.Errorf("duplicate_of can only be used when state_reason is 'duplicate'")
			}
			if err := validateDuplicateState(state, reason, duplicate); err != nil {
				return nil, err
			}
			parent, err := OptionalIntParam(args, "parent_issue_number")
			if err != nil {
				return nil, err
			}
			_, parentProvided := args["parent_issue_number"]
			if parentProvided {
				args["parent_issue_number"] = parent
				if parent < 1 {
					return nil, fmt.Errorf("parent_issue_number must be greater than 0")
				}
				if args["method"] != "create" {
					return nil, fmt.Errorf("parent_issue_number can only be used with the create method")
				}
			}
			parentOwner, err := OptionalParam[string](args, "parent_owner")
			if err != nil {
				return nil, err
			}
			parentRepo, err := OptionalParam[string](args, "parent_repo")
			if err != nil {
				return nil, err
			}
			if err := validateParentRepository(parentProvided, parentOwner, parentRepo); err != nil {
				return nil, err
			}
			fields, err := optionalIssueWriteFields(args)
			if err != nil {
				return nil, err
			}
			if parentProvided && len(fields) > 0 {
				return nil, fmt.Errorf("issue_fields cannot be used with parent_issue_number")
			}
			if args["method"] != "update" {
				delete(args, "issue_number")
			} else if _, exists := args["issue_number"]; exists {
				value, err := OptionalIntParam(args, "issue_number")
				if err != nil {
					return nil, err
				}
				args["issue_number"] = value
			}
		}
		return json.Marshal(args)
	}
}
