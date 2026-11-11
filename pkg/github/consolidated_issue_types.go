package github

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

type IssueReadOutput struct {
	Method     string                    `json:"method"`
	Issue      *IssueDetailsOutput       `json:"issue,omitempty"`
	Comments   *[]IssueReadCommentOutput `json:"comments,omitempty"`
	SubIssues  *[]*SubIssueOutput        `json:"sub_issues,omitempty"`
	Parent     *IssueParentRef           `json:"parent,omitempty"`
	Labels     *[]IssueLabelOutput       `json:"labels,omitempty"`
	TotalCount *int                      `json:"totalCount,omitempty"`
}

func (out IssueReadOutput) MarshalJSON() ([]byte, error) {
	if out.Method == "" {
		// SDK v1.8 validates this zero value even for errors before the
		// protocol middleware removes it from the wire result.
		return []byte(`{"method":"get","issue":null}`), nil
	}
	if out.Method == "get_parent" {
		return json.Marshal(struct {
			Method string          `json:"method"`
			Parent *IssueParentRef `json:"parent"`
		}{out.Method, out.Parent})
	}
	type plain IssueReadOutput
	return json.Marshal(plain(out))
}

// IssueDetailsOutput excludes REST issue_field_values: get always drops them
// and replaces them with the GraphQL field_values representation.
type IssueDetailsOutput struct {
	Number               int                         `json:"number"`
	Title                string                      `json:"title"`
	Body                 string                      `json:"body,omitempty"`
	State                string                      `json:"state" jsonschema:"Issue state (lowercase REST value)."`
	StateReason          string                      `json:"state_reason,omitempty"`
	Draft                bool                        `json:"draft,omitempty"`
	Locked               bool                        `json:"locked,omitempty"`
	HTMLURL              string                      `json:"html_url,omitempty" jsonschema:"Human-readable issue link."`
	User                 *IssueUserOutput            `json:"user,omitempty"`
	AuthorAssociation    string                      `json:"author_association,omitempty"`
	Labels               []string                    `json:"labels,omitempty"`
	Assignees            []string                    `json:"assignees"`
	Milestone            string                      `json:"milestone,omitempty"`
	Comments             int                         `json:"comments,omitempty" jsonschema:"Number of comments."`
	Reactions            *MinimalReactions           `json:"reactions,omitempty"`
	CreatedAt            string                      `json:"created_at,omitempty" jsonschema:"Creation time (RFC 3339)."`
	UpdatedAt            string                      `json:"updated_at,omitempty" jsonschema:"Last update time (RFC 3339)."`
	ClosedAt             string                      `json:"closed_at,omitempty" jsonschema:"Closure time (RFC 3339), when closed."`
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
		HTMLURL: issue.HTMLURL, User: issueUserOutput(issue.User), AuthorAssociation: issue.AuthorAssociation,
		Labels: issue.Labels, Assignees: issue.Assignees, Milestone: issue.Milestone,
		Comments: issue.Comments, Reactions: issue.Reactions, CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt, ClosedAt: issue.ClosedAt, ClosedBy: issue.ClosedBy,
		IssueType: issue.IssueType, FieldValues: issue.FieldValues, HasParent: issue.HasParent,
		HasChildren: issue.HasChildren, Parent: issue.Parent, SubIssuesSummary: issue.SubIssuesSummary,
		ClosedByPullRequests: issue.ClosedByPullRequests,
	}
}

type IssueUserOutput struct {
	Login string `json:"login"`
	ID    int64  `json:"id,omitempty"`
}

func issueUserOutput(user *MinimalUser) *IssueUserOutput {
	if user == nil {
		return nil
	}
	return &IssueUserOutput{Login: user.Login, ID: user.ID}
}

type IssueReadCommentOutput struct {
	ID                int64             `json:"id"`
	Body              string            `json:"body,omitempty"`
	HTMLURL           string            `json:"html_url" jsonschema:"Human-readable comment link."`
	User              *IssueUserOutput  `json:"user,omitempty"`
	AuthorAssociation string            `json:"author_association,omitempty"`
	Reactions         *MinimalReactions `json:"reactions,omitempty"`
	CreatedAt         string            `json:"created_at,omitempty" jsonschema:"Creation time (RFC 3339)."`
	UpdatedAt         string            `json:"updated_at,omitempty" jsonschema:"Last update time (RFC 3339)."`
}

func issueCommentOutputs(comments []MinimalIssueComment) []IssueReadCommentOutput {
	output := make([]IssueReadCommentOutput, 0, len(comments))
	for _, comment := range comments {
		output = append(output, IssueReadCommentOutput{
			ID: comment.ID, Body: comment.Body, HTMLURL: comment.HTMLURL,
			User: issueUserOutput(comment.User), AuthorAssociation: comment.AuthorAssociation,
			Reactions: comment.Reactions, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt,
		})
	}
	return output
}

type IssueParentOutput struct {
	Parent *IssueParentRef `json:"parent"`
}

// These fields keep the original map's lexicographic JSON ordering.
type IssueParentRef struct {
	Number     int    `json:"number"`
	Repository string `json:"repository"`
	State      string `json:"state" jsonschema:"Parent issue state (uppercase GraphQL value)."`
	Title      string `json:"title"`
	URL        string `json:"url" jsonschema:"Human-readable parent issue link."`
}

type IssueLabelsOutput struct {
	Labels     []IssueLabelOutput `json:"labels"`
	TotalCount int                `json:"totalCount"`
}

type IssueWriteOutput struct {
	Method   string                    `json:"method"`
	Issue    *MinimalResponse          `json:"issue,omitempty"`
	Awaiting *IssueWriteAwaitingOutput `json:"awaiting,omitempty"`
}

type IssueWriteAwaitingOutput struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (out IssueWriteOutput) MarshalJSON() ([]byte, error) {
	if out.Awaiting != nil {
		return json.Marshal(out.Awaiting)
	}
	if out.Method == "" {
		return []byte(`{"method":"create","issue":{"id":"","url":""}}`), nil
	}
	type plain IssueWriteOutput
	return json.Marshal(plain(out))
}

var issueWriteOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := issueDTOschema[IssueWriteOutput]()
	awaiting := issueDTOschema[IssueWriteAwaitingOutput]()
	awaiting.Properties["status"].Enum = []any{"awaiting_user_submission"}
	delete(schema.Properties, "awaiting")
	schema.Properties["status"] = awaiting.Properties["status"]
	schema.Properties["reason"] = awaiting.Properties["reason"]
	schema.Required = nil
	schema.Properties["method"].Enum = []any{"create", "update"}
	schema.OneOf = []*jsonschema.Schema{
		{Required: []string{"method", "issue"}, Properties: map[string]*jsonschema.Schema{
			"status": forbiddenIssueVariantProperty(), "reason": forbiddenIssueVariantProperty(),
		}},
		{Required: []string{"status", "reason"}, Properties: map[string]*jsonschema.Schema{
			"method": forbiddenIssueVariantProperty(), "issue": forbiddenIssueVariantProperty(),
		}},
	}
	return schema
})

func issueWriteResult(method string) func(*mcp.CallToolResult, *MinimalResponse, error) (*mcp.CallToolResult, *IssueWriteOutput, error) {
	return func(result *mcp.CallToolResult, output *MinimalResponse, err error) (*mcp.CallToolResult, *IssueWriteOutput, error) {
		if output == nil {
			return result, nil, err
		}
		return result, &IssueWriteOutput{Method: method, Issue: output}, err
	}
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
				"value":             forbiddenIssueVariantProperty(), "delete": {Type: "boolean", Enum: []any{false}},
			},
		},
		{
			Required: []string{"delete"},
			Properties: map[string]*jsonschema.Schema{
				"delete": {Type: "boolean", Enum: []any{true}}, "value": forbiddenIssueVariantProperty(), "field_option_name": {Type: "string", Enum: []any{""}},
			},
		},
	}
}

func forbiddenIssueVariantProperty() *jsonschema.Schema {
	// An annotation keeps jsonschema-go from simplifying the universal
	// schema to a boolean, which schema-object-only clients cannot decode.
	return &jsonschema.Schema{Not: &jsonschema.Schema{Description: "Any value."}}
}

type IssueLabelOutput struct {
	Color       string `json:"color"`
	Description string `json:"description"`
	ID          string `json:"id"`
	Name        string `json:"name"`
}

type SubIssueOutput struct {
	ID               *int64                      `json:"id,omitempty"`
	Number           *int                        `json:"number,omitempty"`
	Title            *string                     `json:"title,omitempty"`
	Body             *string                     `json:"body,omitempty"`
	State            *string                     `json:"state,omitempty" jsonschema:"Issue state (lowercase REST value)."`
	StateReason      *string                     `json:"state_reason,omitempty"`
	HTMLURL          *string                     `json:"html_url,omitempty" jsonschema:"Human-readable issue link."`
	User             *IssueUserOutput            `json:"user,omitempty"`
	Labels           []string                    `json:"labels,omitempty"`
	Assignees        []string                    `json:"assignees"`
	Milestone        string                      `json:"milestone,omitempty"`
	Comments         *int                        `json:"comments,omitempty" jsonschema:"Number of comments."`
	CreatedAt        string                      `json:"created_at,omitempty" jsonschema:"Creation time (RFC 3339)."`
	UpdatedAt        string                      `json:"updated_at,omitempty" jsonschema:"Last update time (RFC 3339)."`
	ClosedAt         string                      `json:"closed_at,omitempty" jsonschema:"Closure time (RFC 3339), when closed."`
	IssueType        string                      `json:"issue_type,omitempty"`
	SubIssuesSummary *MinimalSubIssuesSummary    `json:"sub_issues_summary,omitempty"`
	FieldValues      []*SubIssueFieldValueOutput `json:"field_values,omitempty"`
}

type SubIssueFieldValueOutput struct {
	IssueFieldID int64            `json:"issue_field_id"`
	DataType     string           `json:"data_type" jsonschema:"REST issue field type."`
	Value        *IssueFieldValue `json:"value" jsonschema:"Scalar field value; dates use YYYY-MM-DD."`
}

func subIssueOutput(issue *github.SubIssue) (*SubIssueOutput, error) {
	if issue == nil {
		return nil, nil
	}
	minimal := convertToMinimalIssue((*github.Issue)(issue))
	out := &SubIssueOutput{
		ID: issue.ID, Number: issue.Number, State: issue.State, StateReason: issue.StateReason,
		Title: issue.Title, Body: issue.Body, HTMLURL: issue.HTMLURL,
		User: issueUserOutput(minimal.User), Labels: minimal.Labels, Assignees: minimal.Assignees,
		Milestone: minimal.Milestone, Comments: issue.Comments,
		CreatedAt: minimal.CreatedAt, UpdatedAt: minimal.UpdatedAt, ClosedAt: minimal.ClosedAt,
		IssueType: minimal.IssueType,
	}
	if summary := issue.SubIssuesSummary; summary != nil {
		out.SubIssuesSummary = &MinimalSubIssuesSummary{
			Total: summary.GetTotal(), Completed: summary.GetCompleted(), PercentCompleted: summary.GetPercentCompleted(),
		}
	}
	for _, field := range issue.IssueFieldValues {
		if field == nil {
			out.FieldValues = append(out.FieldValues, nil)
			continue
		}
		value, err := issueFieldValue(field.Value)
		if err != nil {
			return nil, err
		}
		out.FieldValues = append(out.FieldValues, &SubIssueFieldValueOutput{
			IssueFieldID: field.IssueFieldID, DataType: field.DataType, Value: value,
		})
	}
	return out, nil
}

var issueReadOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := issueDTOschema[IssueReadOutput]()
	issue := schema.Properties["issue"]
	issue.Properties["state"].Enum = []any{"", "open", "closed"}
	issue.Properties["state_reason"].Enum = []any{"completed", "not_planned", "reopened", "duplicate"}
	describeIssueSummarySchema(issue.Properties["sub_issues_summary"])
	schema.Properties["sub_issues"] = subIssueArraySchema()
	schema.Properties["parent"].Properties["state"].Enum = []any{"", "OPEN", "CLOSED"}
	schema.Properties["method"].Enum = []any{"get", "get_comments", "get_sub_issues", "get_parent", "get_labels"}
	methods := []string{"get", "get_comments", "get_sub_issues", "get_parent", "get_labels"}
	fields := []string{"issue", "comments", "sub_issues", "parent", "labels"}
	for i, method := range methods {
		variant := &jsonschema.Schema{Required: []string{fields[i]}, Properties: map[string]*jsonschema.Schema{
			"method": {Enum: []any{method}},
		}}
		for j, field := range fields {
			if i != j {
				variant.Properties[field] = forbiddenIssueVariantProperty()
			}
		}
		if method == "get_labels" {
			variant.Required = append(variant.Required, "totalCount")
		} else {
			variant.Properties["totalCount"] = forbiddenIssueVariantProperty()
		}
		schema.OneOf = append(schema.OneOf, variant)
	}
	return schema
})

type SubIssueWriteOutput struct {
	Method string          `json:"method"`
	Issue  *SubIssueOutput `json:"issue"`
}

func (out SubIssueWriteOutput) MarshalJSON() ([]byte, error) {
	// Unlike the SDK's default nil-pointer handling, a null API response
	// must remain null rather than becoming an empty sub-issue object.
	if out.Method == "" {
		return []byte(`{"method":"add","issue":null}`), nil
	}
	type plain SubIssueWriteOutput
	return json.Marshal(plain(out))
}

func subIssueWriteResult(method string) func(*mcp.CallToolResult, *SubIssueOutput, error) (*mcp.CallToolResult, *SubIssueWriteOutput, error) {
	return func(result *mcp.CallToolResult, output *SubIssueOutput, err error) (*mcp.CallToolResult, *SubIssueWriteOutput, error) {
		return result, &SubIssueWriteOutput{Method: method, Issue: output}, err
	}
}

var subIssueWriteOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := issueDTOschema[SubIssueWriteOutput]()
	schema.Properties["method"].Enum = []any{"add", "remove", "reprioritize"}
	schema.Properties["issue"] = subIssueSchema()
	schema.Properties["issue"].Type = ""
	schema.Properties["issue"].Types = []string{"object", "null"}
	return schema
})

func subIssueSchema() *jsonschema.Schema {
	schema := issueDTOschema[SubIssueOutput]()
	schema.Properties["state"].Enum = []any{"open", "closed"}
	schema.Properties["state_reason"].Enum = []any{"completed", "not_planned", "reopened", "duplicate"}
	schema.Properties["field_values"].Items.Properties["data_type"].Enum = []any{"text", "number", "date", "single_select"}
	describeIssueSummarySchema(schema.Properties["sub_issues_summary"])
	replaceIssueFieldScalar(schema)
	return schema
}

func describeIssueSummarySchema(schema *jsonschema.Schema) {
	schema.Properties["total"].Description = "Total number of sub-issues."
	schema.Properties["completed"].Description = "Number of completed sub-issues."
	schema.Properties["percent_completed"].Description = "Completed sub-issues as a percentage (0-100)."
}

func subIssueArraySchema() *jsonschema.Schema {
	items := subIssueSchema()
	items.Type = ""
	items.Types = []string{"object", "null"}
	return &jsonschema.Schema{Types: []string{"array", "null"}, Items: items}
}

func replaceIssueFieldScalar(schema *jsonschema.Schema) {
	// jsonschema's type override is not needed for the surrounding API
	// definitions; replace only the custom scalar's inferred properties.
	if schema == nil {
		return
	}
	if fields := schema.Properties["field_values"]; fields != nil && fields.Items != nil {
		fields.Items.Properties["value"] = &jsonschema.Schema{Types: []string{"string", "number", "boolean", "null"}}
	}

	replaceIssueFieldScalar(schema.Items)
}

func issueDTOschema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	return schema
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
