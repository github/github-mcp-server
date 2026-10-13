package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

type IssueMetadataInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo,omitempty"`
}

type IssueReadInput struct {
	Method      string `json:"method"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	IssueNumber int    `json:"issue_number"`
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
}

type SubIssueWriteInput struct {
	Method        string `json:"method"`
	Owner         string `json:"owner"`
	Repo          string `json:"repo"`
	IssueNumber   int    `json:"issue_number"`
	SubIssueID    int    `json:"sub_issue_id"`
	ReplaceParent bool   `json:"replace_parent,omitempty"`
	AfterID       int    `json:"after_id,omitempty"`
	BeforeID      int    `json:"before_id,omitempty"`
}

type IssueTypeOutput struct {
	ID          *int64  `json:"id,omitempty"`
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Color       *string `json:"color,omitempty"`
	IsEnabled   *bool   `json:"is_enabled,omitempty"`
}

func issueTypeOutputs(types []*github.IssueType) []*IssueTypeOutput {
	if types == nil {
		return nil
	}
	output := make([]*IssueTypeOutput, len(types))
	for i, item := range types {
		if item != nil {
			output[i] = &IssueTypeOutput{
				ID: item.ID, Name: item.Name, Description: item.Description,
				Color: item.Color, IsEnabled: item.IsEnabled,
			}
		}
	}
	return output
}

type AddIssueCommentInput struct {
	Owner       string  `json:"owner"`
	Repo        string  `json:"repo"`
	IssueNumber int     `json:"issue_number"`
	CommentID   *int64  `json:"comment_id,omitempty"`
	Body        *string `json:"body,omitempty"`
	Reaction    *string `json:"reaction,omitempty"`
}

type UpdateIssueCommentInput struct {
	Owner     string  `json:"owner"`
	Repo      string  `json:"repo"`
	CommentID int64   `json:"comment_id"`
	Body      *string `json:"body"`
}

type IssueCommentOutput struct {
	ID      string `json:"id"`
	HTMLURL string `json:"html_url,omitempty"`
}

func (output IssueCommentOutput) MarshalJSON() ([]byte, error) {
	if output.ID == "" && output.HTMLURL == "" {
		return []byte("null"), nil
	}
	type wireOutput IssueCommentOutput
	return json.Marshal(wireOutput(output))
}

type IssueReactionOutput struct {
	ID string `json:"id"`
}

type IssueCommentAndReactionOutput struct {
	Comment  IssueCommentOutput  `json:"comment"`
	Reaction IssueReactionOutput `json:"reaction"`
}

type AddIssueCommentOutput struct {
	Single   *IssueCommentOutput
	Combined *IssueCommentAndReactionOutput
}

func (output AddIssueCommentOutput) MarshalJSON() ([]byte, error) {
	if output.Combined != nil {
		return json.Marshal(output.Combined)
	}
	return json.Marshal(output.Single)
}

type AddIssueCommentLegacyOutput struct {
	Single   *MinimalResponse
	Combined *IssueCommentAndReactionLegacyOutput
}

type IssueCommentAndReactionLegacyOutput struct {
	Comment  MinimalResponse `json:"comment"`
	Reaction MinimalResponse `json:"reaction"`
}

func (output AddIssueCommentLegacyOutput) MarshalJSON() ([]byte, error) {
	if output.Combined != nil {
		return json.Marshal(output.Combined)
	}
	return json.Marshal(output.Single)
}

func addIssueCommentOutputSchema() *jsonschema.Schema {
	comment := func() *jsonschema.Schema {
		return issueCommentOutputObjectSchema(true)
	}
	reaction := func() *jsonschema.Schema {
		return &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{
			"id": {Type: "string", Description: "The ID of the created reaction."},
		}, Required: []string{"id"}, AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}}}
	}
	return &jsonschema.Schema{
		Types: []string{"object", "null"},
		OneOf: []*jsonschema.Schema{
			{Type: "null"},
			comment(),
			{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"id": {Type: "string", Description: "The ID of the created reaction."},
				},
				Required:             []string{"id"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
			{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"comment": comment(), "reaction": reaction(),
				},
				Required:             []string{"comment", "reaction"},
				AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
			},
		},
	}
}

func issueCommentOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Types: []string{"object", "null"},
		OneOf: []*jsonschema.Schema{
			{Type: "null"},
			issueCommentOutputObjectSchema(true),
		},
	}
}

func issueCommentOutputObjectSchema(includeURL bool) *jsonschema.Schema {
	properties := map[string]*jsonschema.Schema{
		"id": {Type: "string", Description: "The ID of the created comment or reaction."},
	}
	required := []string{"id"}
	if includeURL {
		properties["html_url"] = &jsonschema.Schema{
			Type: "string", Description: "The web URL of the created issue comment.",
		}
		required = append(required, "html_url")
	}
	return &jsonschema.Schema{
		Type: "object", Properties: properties, Required: required,
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}

type IssueDependencyState string

type MinimalIssueDependencyRef struct {
	Number     int                  `json:"number"`
	Title      string               `json:"title"`
	State      IssueDependencyState `json:"state"`
	HTMLURL    string               `json:"html_url"`
	Repository string               `json:"repository,omitempty"`
}

func dependencyRefOutput(ref MinimalIssueRef) MinimalIssueDependencyRef {
	return MinimalIssueDependencyRef{
		Number: ref.Number, Title: ref.Title, State: IssueDependencyState(ref.State),
		HTMLURL: ref.URL, Repository: ref.Repository,
	}
}

type IssueDependencyReadInput struct {
	Method      string `json:"method"`
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	IssueNumber int    `json:"issue_number"`
	Page        int    `json:"page,omitempty"`
	PerPage     int    `json:"perPage,omitempty"`
}

type IssueDependencyPageInfo struct {
	HasNextPage bool `json:"hasNextPage"`
	NextPage    int  `json:"nextPage"`
}

type IssueDependencyReadOutput struct {
	Issues   []MinimalIssueDependencyRef `json:"issues"`
	PageInfo IssueDependencyPageInfo     `json:"pageInfo"`
}

type IssueDependencyReadLegacyOutput struct {
	Issues   []MinimalIssueRef       `json:"issues"`
	PageInfo IssueDependencyPageInfo `json:"pageInfo"`
}

type IssueDependencyWriteInput struct {
	Method             string `json:"method"`
	Type               string `json:"type"`
	Owner              string `json:"owner"`
	Repo               string `json:"repo"`
	IssueNumber        int    `json:"issue_number"`
	RelatedIssueNumber int    `json:"related_issue_number"`
	RelatedOwner       string `json:"related_owner,omitempty"`
	RelatedRepo        string `json:"related_repo,omitempty"`
}

type IssueDependencyWriteOutput struct {
	BlockedIssue  MinimalIssueDependencyRef `json:"blocked_issue"`
	BlockingIssue MinimalIssueDependencyRef `json:"blocking_issue"`
	Message       string                    `json:"message"`
}

type IssueDependencyWriteLegacyOutput struct {
	BlockedIssue  MinimalIssueRef `json:"blocked_issue"`
	BlockingIssue MinimalIssueRef `json:"blocking_issue"`
	Message       string          `json:"message"`
}

type DuplicateIssueState string

type MinimalDuplicateIssue struct {
	Number  int                 `json:"number"`
	Title   string              `json:"title"`
	State   DuplicateIssueState `json:"state"`
	HTMLURL string              `json:"html_url"`
}

type DuplicateConfidence string

type DuplicateCandidate struct {
	Issue           MinimalDuplicateIssue `json:"issue"`
	Score           *float64              `json:"score"`
	Confidence      DuplicateConfidence   `json:"confidence"`
	LikelyDuplicate bool                  `json:"likely_duplicate"`
}

type LegacyDuplicateCandidate struct {
	Issue           MinimalIssueRef     `json:"issue"`
	Score           *float64            `json:"score"`
	Confidence      DuplicateConfidence `json:"confidence"`
	LikelyDuplicate bool                `json:"likely_duplicate"`
}

type FindDuplicateInput struct {
	Owner               string   `json:"owner"`
	Repo                string   `json:"repo"`
	IssueNumber         int      `json:"issue_number"`
	ConfidenceThreshold *float64 `json:"confidence_threshold,omitempty"`
	Page                *int     `json:"page,omitempty"`
	PerPage             *int     `json:"perPage,omitempty"`
}

type IssueFieldOutput struct {
	ID          string                   `json:"id"`
	DatabaseID  int64                    `json:"full_database_id,omitempty"`
	Name        string                   `json:"name"`
	Description string                   `json:"description,omitempty"`
	DataType    string                   `json:"data_type"`
	Visibility  string                   `json:"visibility"`
	Options     []IssueFieldOptionOutput `json:"options,omitempty"`
}

type IssueFieldOptionOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Color       string `json:"color"`
	Priority    *int   `json:"priority,omitempty"`
}

func issueFieldOutputs(fields []IssueField) []IssueFieldOutput {
	if fields == nil {
		return nil
	}
	output := make([]IssueFieldOutput, len(fields))
	for i, field := range fields {
		output[i] = IssueFieldOutput{
			ID: field.ID, DatabaseID: field.DatabaseID, Name: field.Name,
			Description: field.Description, DataType: field.DataType,
			Visibility: field.Visibility,
		}
		if field.Options != nil {
			output[i].Options = make([]IssueFieldOptionOutput, len(field.Options))
			for j, option := range field.Options {
				output[i].Options[j] = IssueFieldOptionOutput(option)
			}
		}
	}
	return output
}

func issueOutputSchema[T any]() *jsonschema.Schema {
	schema, err := inventory.CachedSchemaFor[T](nil)
	if err != nil {
		panic(fmt.Sprintf("failed to generate issue output schema: %v", err))
	}
	return inventory.CloneSchema(schema)
}

func issuePaginationValidationSchema(advertised *jsonschema.Schema) *jsonschema.Schema {
	validation := inventory.CloneSchemaWithoutDefaults(advertised)
	for _, name := range []string{"page", "perPage"} {
		property := validation.Properties[name]
		property.Minimum = new(0.0)
	}
	return validation
}

func listIssueFieldsOutputSchema() *jsonschema.Schema {
	schema := issueOutputSchema[[]IssueFieldOutput]()
	schema.Items.Properties["id"].Description = "The node ID of the issue field."
	schema.Items.Properties["full_database_id"].Description = "The database ID of the issue field, when available."
	schema.Items.Properties["name"].Description = "The field name."
	schema.Items.Properties["description"].Description = "The field description, when available."
	schema.Items.Properties["data_type"].Description = "The field value type."
	schema.Items.Properties["visibility"].Description = "The visibility policy for the field."
	schema.Items.Properties["options"].Description = "Available values for single-select fields."
	schema.Items.Properties["data_type"].Enum = []any{"TEXT", "NUMBER", "DATE", "SINGLE_SELECT"}
	schema.Items.Properties["options"].Items.Properties["id"].Description = "The option node ID."
	schema.Items.Properties["options"].Items.Properties["name"].Description = "The option name."
	schema.Items.Properties["options"].Items.Properties["description"].Description = "The option description, when available."
	schema.Items.Properties["options"].Items.Properties["color"].Description = "The option color."
	schema.Items.Properties["options"].Items.Properties["priority"].Description = "Display order of this option, when set."
	return schema
}

func issueTypeOutputSchema() *jsonschema.Schema {
	schema := issueOutputSchema[[]*IssueTypeOutput]()
	schema.Items.Properties["id"].Description = "The issue type ID."
	schema.Items.Properties["name"].Description = "The issue type name."
	schema.Items.Properties["description"].Description = "The issue type description, when available."
	schema.Items.Properties["color"].Description = "The issue type color, when set."
	schema.Items.Properties["is_enabled"].Description = "Whether this issue type is enabled."
	return schema
}

func issueDependencyReadOutputSchema() *jsonschema.Schema {
	schema := issueOutputSchema[*IssueDependencyReadOutput]()
	ref := schema.Properties["issues"].Items.Properties
	ref["number"].Description = "The related issue number."
	ref["title"].Description = "The sanitized title of the related issue."
	ref["state"].Description = "The related issue state."
	ref["html_url"].Description = "The web URL of the related issue."
	ref["repository"].Description = "The related repository in owner/name form."
	schema.Properties["issues"].Description = "Related issues in the requested dependency direction."
	schema.Properties["pageInfo"].Properties["hasNextPage"].Description = "Whether another page of related issues is available."
	schema.Properties["pageInfo"].Properties["nextPage"].Description = "The next page number, or 0 when no next page exists."
	return schema
}

func issueDependencyWriteOutputSchema() *jsonschema.Schema {
	schema := issueOutputSchema[*IssueDependencyWriteOutput]()
	for _, field := range []string{"blocked_issue", "blocking_issue"} {
		ref := schema.Properties[field].Properties
		ref["number"].Description = "The issue number."
		ref["title"].Description = "The sanitized issue title."
		ref["state"].Description = "The issue state."
		ref["html_url"].Description = "The web URL of the issue."
		ref["repository"].Description = "The repository in owner/name form."
	}
	schema.Properties["message"].Description = "Whether the dependency relationship was added or removed."
	return schema
}

func findDuplicateOutputSchema() *jsonschema.Schema {
	schema := issueOutputSchema[[]DuplicateCandidate]()
	issue := schema.Items.Properties["issue"].Properties
	issue["number"].Description = "The candidate issue number."
	issue["title"].Description = "The sanitized candidate issue title."
	issue["state"].Description = "The candidate issue state."
	issue["html_url"].Description = "The web URL of the candidate issue."
	schema.Items.Properties["score"].Description = "Similarity score on the API-defined scale; null when the API does not return a score."
	schema.Items.Properties["confidence"].Description = "The API-provided confidence category."
	schema.Items.Properties["likely_duplicate"].Description = "Whether the API considers this candidate a likely duplicate."
	return schema
}

func validateIssueCoordinate(owner, repo string, number int) error {
	if owner == "" {
		return fmt.Errorf("missing required parameter: owner")
	}
	if repo == "" {
		return fmt.Errorf("missing required parameter: repo")
	}
	if number == 0 {
		return fmt.Errorf("missing required parameter: issue_number")
	}
	return nil
}

// Only the dependency writer historically accepted case-insensitive methods
// and directions. Do not broaden the dependency reader's method handling.
func normalizeIssueDependencyWriteArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	for _, field := range []string{"method", "type"} {
		var value string
		if err := json.Unmarshal(args[field], &value); err == nil {
			args[field], _ = json.Marshal(strings.ToLower(value))
		}
	}
	return json.Marshal(args)
}

func normalizeIssueCommentID(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if value, exists := args["comment_id"]; exists {
		id, err := toInt64(value)
		if err != nil {
			return nil, fmt.Errorf("parameter comment_id is not a valid number: %w", err)
		}
		args["comment_id"] = id
	}
	return json.Marshal(args)
}

func normalizeIssueStrings(required, optional []string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, field := range required {
			if _, err := RequiredParam[string](args, field); err != nil {
				return nil, err
			}
		}

		for _, field := range optional {
			if _, err := OptionalParam[string](args, field); err != nil {
				return nil, err
			}
		}
		return raw, nil
	}
}

func normalizeIssueIntegers(required, optional []string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		for _, field := range required {
			value, err := RequiredInt(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		for _, field := range optional {
			if _, exists := args[field]; !exists {
				continue
			}
			value, err := OptionalIntParam(args, field)
			if err != nil {
				return nil, err
			}
			args[field] = value
		}
		return json.Marshal(args)
	}
}

func normalizeDuplicateThreshold(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, err := OptionalParam[float64](args, "confidence_threshold"); err != nil {
		return nil, err
	}
	return raw, nil
}
