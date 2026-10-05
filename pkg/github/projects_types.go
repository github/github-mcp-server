package github

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProjectParameter retains the original JSON for the legacy method-specific
// validators. In particular, invalid or unused arguments must not fail before
// client acquisition, and numeric strings must retain their original coercion.
type ProjectParameter[T any] struct {
	Value T
	raw   json.RawMessage
}

func (p *ProjectParameter[T]) UnmarshalJSON(raw []byte) error {
	p.raw = append(p.raw[:0], raw...)
	// A value of the wrong type is reported by the selected method, not here.
	var value T
	if err := json.Unmarshal(raw, &value); err == nil {
		p.Value = value
	}
	return nil
}

func (p ProjectParameter[T]) MarshalJSON() ([]byte, error) {
	if p.raw != nil {
		return p.raw, nil
	}
	return json.Marshal(p.Value)
}

type ProjectsListInput struct {
	Method        ProjectParameter[string]   `json:"method"`
	Owner         ProjectParameter[string]   `json:"owner"`
	OwnerType     ProjectParameter[string]   `json:"owner_type"`
	ProjectNumber ProjectParameter[int]      `json:"project_number"`
	Query         ProjectParameter[string]   `json:"query"`
	Fields        ProjectParameter[[]string] `json:"fields"`
	FieldNames    ProjectParameter[[]string] `json:"field_names"`
	PerPage       ProjectParameter[int]      `json:"perPage"`
	LegacyPerPage ProjectParameter[int]      `json:"per_page"`
	After         ProjectParameter[string]   `json:"after"`
	Before        ProjectParameter[string]   `json:"before"`
}

type ProjectsGetInput struct {
	Method         ProjectParameter[string]   `json:"method"`
	Owner          ProjectParameter[string]   `json:"owner"`
	OwnerType      ProjectParameter[string]   `json:"owner_type"`
	ProjectNumber  ProjectParameter[int]      `json:"project_number"`
	FieldID        ProjectParameter[int64]    `json:"field_id"`
	ItemID         ProjectParameter[int64]    `json:"item_id"`
	Fields         ProjectParameter[[]string] `json:"fields"`
	FieldNames     ProjectParameter[[]string] `json:"field_names"`
	StatusUpdateID ProjectParameter[string]   `json:"status_update_id"`
	ViewID         ProjectParameter[string]   `json:"view_id"`
}

type ProjectsWriteInput struct {
	Method            ProjectParameter[string]                   `json:"method"`
	Owner             ProjectParameter[string]                   `json:"owner"`
	OwnerType         ProjectParameter[string]                   `json:"owner_type"`
	ProjectNumber     ProjectParameter[int]                      `json:"project_number"`
	Title             ProjectParameter[string]                   `json:"title"`
	ViewID            ProjectParameter[string]                   `json:"view_id"`
	Name              ProjectParameter[string]                   `json:"name"`
	Layout            ProjectParameter[string]                   `json:"layout"`
	Filter            ProjectParameter[*string]                  `json:"filter"`
	VisibleFields     ProjectParameter[[]string]                 `json:"visible_fields"`
	VisibleFieldNames ProjectParameter[[]string]                 `json:"visible_field_names"`
	ItemID            ProjectParameter[int64]                    `json:"item_id"`
	ItemType          ProjectParameter[string]                   `json:"item_type"`
	ItemOwner         ProjectParameter[string]                   `json:"item_owner"`
	ItemRepo          ProjectParameter[string]                   `json:"item_repo"`
	IssueNumber       ProjectParameter[int]                      `json:"issue_number"`
	PullRequestNumber ProjectParameter[int]                      `json:"pull_request_number"`
	UpdatedField      ProjectParameter[ProjectUpdatedFieldInput] `json:"updated_field"`
	Items             ProjectParameter[[]ProjectItemReference]   `json:"items"`
	Body              ProjectParameter[string]                   `json:"body"`
	Status            ProjectParameter[string]                   `json:"status"`
	StartDate         ProjectParameter[string]                   `json:"start_date"`
	TargetDate        ProjectParameter[string]                   `json:"target_date"`
	FieldName         ProjectParameter[string]                   `json:"field_name"`
	IterationDuration ProjectParameter[int]                      `json:"iteration_duration"`
	Iterations        ProjectParameter[[]ProjectIterationInput]  `json:"iterations"`
}

func (input ProjectsListInput) MarshalJSON() ([]byte, error) {
	return marshalProjectInput(input)
}

func (input ProjectsGetInput) MarshalJSON() ([]byte, error) {
	return marshalProjectInput(input)
}

func (input ProjectsWriteInput) MarshalJSON() ([]byte, error) {
	return marshalProjectInput(input)
}

func marshalProjectInput(input any) ([]byte, error) {
	args, err := projectArguments(input)
	if err != nil {
		return nil, err
	}
	return json.Marshal(args)
}

func (input *ProjectsListInput) UnmarshalJSON(raw []byte) error {
	return unmarshalProjectInput(raw, input)
}

func (input *ProjectsGetInput) UnmarshalJSON(raw []byte) error {
	return unmarshalProjectInput(raw, input)
}

func (input *ProjectsWriteInput) UnmarshalJSON(raw []byte) error {
	return unmarshalProjectInput(raw, input)
}

// Legacy map handlers recognize only exact JSON keys; encoding/json's default
// case-insensitive struct matching must not turn ignored keys into write targets.
func unmarshalProjectInput(raw []byte, input any) error {
	var args map[string]json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	value := reflect.ValueOf(input).Elem()
	value.SetZero()
	inputType := value.Type()
	for i := range value.NumField() {
		if argument, ok := args[inputType.Field(i).Tag.Get("json")]; ok {
			if err := json.Unmarshal(argument, value.Field(i).Addr().Interface()); err != nil {
				return err
			}
		}
	}
	return nil
}

type ProjectUpdatedFieldInput struct {
	ID    *int64          `json:"id,omitempty"`
	Name  *string         `json:"name,omitempty"`
	Value json.RawMessage `json:"value"`
}

type ProjectItemReference struct {
	NodeID      string `json:"node_id,omitempty"`
	ItemID      int64  `json:"item_id,omitempty"`
	ItemOwner   string `json:"item_owner,omitempty"`
	ItemRepo    string `json:"item_repo,omitempty"`
	IssueNumber int    `json:"issue_number,omitempty"`
}

type ProjectIterationInput struct {
	Title     string  `json:"title"`
	StartDate string  `json:"start_date"`
	Duration  float64 `json:"duration"`
}

// The raw map is confined to the compatibility boundary with the existing
// validators and API helpers. Omitted parameters must stay omitted.
func projectArguments(input any) (map[string]any, error) {
	value := reflect.ValueOf(input)
	inputType := value.Type()
	args := make(map[string]any, value.NumField())
	for i := range value.NumField() {
		parameter := value.Field(i).Interface().(interface {
			argumentJSON() (json.RawMessage, error)
		})
		raw, err := parameter.argumentJSON()
		if err != nil {
			return nil, err
		}
		if raw == nil {
			continue
		}
		var argument any
		if err := json.Unmarshal(raw, &argument); err != nil {
			return nil, err
		}
		args[inputType.Field(i).Tag.Get("json")] = argument
	}
	return args, nil
}

func (p ProjectParameter[T]) argumentJSON() (json.RawMessage, error) {
	if p.raw != nil {
		return p.raw, nil
	}
	if reflect.ValueOf(&p.Value).Elem().IsZero() {
		return nil, nil
	}
	return json.Marshal(p.Value)
}

type ProjectsListOutput struct {
	Projects      *ProjectListOutput
	Fields        *ProjectFieldListOutput
	Items         *ProjectItemListOutput
	StatusUpdates *ProjectStatusUpdateListOutput
	Views         *ProjectViewListOutput
}

type ProjectListOutput struct {
	Projects []MinimalProject `json:"projects"`
	PageInfo *pageInfo        `json:"pageInfo,omitempty"`
	Note     string           `json:"note,omitempty"`
}

type ProjectFieldListOutput struct {
	Fields   []*github.ProjectV2Field `json:"fields"`
	PageInfo pageInfo                 `json:"pageInfo"`
}

type ProjectItemListOutput struct {
	Items    []ProjectItemOutput `json:"items"`
	PageInfo pageInfo            `json:"pageInfo"`
}

type ProjectStatusUpdateListOutput struct {
	StatusUpdates []MinimalProjectStatusUpdate `json:"statusUpdates"`
	PageInfo      ProjectGraphQLPageInfo       `json:"pageInfo"`
}

type ProjectViewListOutput struct {
	Views    []MinimalProjectView   `json:"views"`
	PageInfo ProjectGraphQLPageInfo `json:"pageInfo"`
}

type ProjectGraphQLPageInfo struct {
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	NextCursor      string `json:"nextCursor"`
	PrevCursor      string `json:"prevCursor"`
}

type ProjectsGetOutput struct {
	Project      *MinimalProject
	Field        *github.ProjectV2Field
	Item         *ProjectItemOutput
	StatusUpdate *MinimalProjectStatusUpdate
	View         *MinimalProjectView
}

type ProjectItemOutput struct {
	ID          int64                      `json:"id"`
	NodeID      string                     `json:"node_id,omitempty"`
	ContentType string                     `json:"content_type,omitempty"`
	Content     *MinimalProjectItemContent `json:"content,omitempty"`
	Fields      []ProjectFieldValueOutput  `json:"fields,omitempty"`
	ArchivedAt  string                     `json:"archived_at,omitempty"`
	CreatedAt   string                     `json:"created_at,omitempty"`
	UpdatedAt   string                     `json:"updated_at,omitempty"`
	Creator     string                     `json:"creator,omitempty"`
}

type ProjectFieldValueOutput struct {
	ID       int64  `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	DataType string `json:"data_type,omitempty"`
	// Unknown field types are recursively compacted by the legacy projection,
	// including arbitrary object keys. A closed value union would lose data.
	Value json.RawMessage `json:"value,omitempty"`
}

type ProjectsWriteOutput struct {
	Added          *ProjectAddedItemOutput
	Item           *ProjectItemOutput
	IssueFields    *MinimalResponse
	Batch          *ProjectBatchOutput
	DeletedItem    *RepositoryMessageOutput
	StatusUpdate   *MinimalProjectStatusUpdate
	View           *MinimalProjectView
	DeletedView    *ProjectDeletedViewOutput
	Project        *ProjectCreatedOutput
	IterationField *ProjectIterationFieldOutput
}

type ProjectAddedItemOutput struct {
	ID             *string `json:"id"`
	Message        string  `json:"message"`
	FullDatabaseID string  `json:"full_database_id,omitempty"`
	ItemID         *int64  `json:"item_id,omitempty"`
}

type ProjectDeletedViewOutput struct {
	DeletedViewID string `json:"deleted_view_id"`
}

type ProjectCreatedOutput struct {
	ID     string `json:"id"`
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

type ProjectIterationFieldOutput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Configuration struct {
		Iterations []ProjectIterationOutput `json:"iterations"`
	} `json:"configuration"`
}

type ProjectIterationOutput struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	StartDate string `json:"start_date"`
	Duration  int    `json:"duration"`
}

type ProjectBatchOutput struct {
	Total     int                      `json:"total"`
	Succeeded int                      `json:"succeeded"`
	Failed    int                      `json:"failed"`
	Unknown   int                      `json:"unknown"`
	Results   []ProjectBatchItemOutput `json:"results"`
}

type ProjectBatchItemOutput struct {
	Index  int                `json:"index"`
	Status batchItemStatus    `json:"status"`
	Item   *batchItemIdentity `json:"item,omitempty"`
	Error  *ProjectBatchError `json:"error,omitempty"`
	// Batch failures echo even malformed reference objects verbatim.
	Ref json.RawMessage `json:"ref,omitempty"`
}

type ProjectBatchError struct {
	Code       string          `json:"code"`
	Message    string          `json:"message"`
	Candidates json.RawMessage `json:"candidates,omitempty"`
	Hint       string          `json:"hint,omitempty"`
}

func (out ProjectsListOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.Projects != nil:
		return json.Marshal(out.Projects)
	case out.Fields != nil:
		return json.Marshal(out.Fields)
	case out.Items != nil:
		return json.Marshal(out.Items)
	case out.StatusUpdates != nil:
		return json.Marshal(out.StatusUpdates)
	default:
		return json.Marshal(out.Views)
	}
}

func (out ProjectsGetOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.Project != nil:
		return json.Marshal(out.Project)
	case out.Field != nil:
		return json.Marshal(out.Field)
	case out.Item != nil:
		return json.Marshal(out.Item)
	case out.StatusUpdate != nil:
		return json.Marshal(out.StatusUpdate)
	default:
		return json.Marshal(out.View)
	}
}

func (out ProjectsWriteOutput) MarshalJSON() ([]byte, error) {
	switch {
	case out.Added != nil:
		return json.Marshal(out.Added)
	case out.Item != nil:
		return json.Marshal(out.Item)
	case out.IssueFields != nil:
		return json.Marshal(out.IssueFields)
	case out.Batch != nil:
		return json.Marshal(out.Batch)
	case out.DeletedItem != nil:
		return json.Marshal(out.DeletedItem)
	case out.StatusUpdate != nil:
		return json.Marshal(out.StatusUpdate)
	case out.View != nil:
		return json.Marshal(out.View)
	case out.DeletedView != nil:
		return json.Marshal(out.DeletedView)
	case out.Project != nil:
		return json.Marshal(out.Project)
	default:
		return json.Marshal(out.IterationField)
	}
}

func projectsListOutputSchema() *jsonschema.Schema {
	return actionsUnionSchema(
		&jsonschema.Schema{Type: "null"},
		projectOutputSchema[ProjectListOutput](),
		projectOutputSchema[ProjectFieldListOutput](),
		projectOutputSchema[ProjectItemListOutput](),
		projectOutputSchema[ProjectStatusUpdateListOutput](),
		projectOutputSchema[ProjectViewListOutput](),
	)
}

func projectsGetOutputSchema() *jsonschema.Schema {
	return actionsUnionSchema(
		&jsonschema.Schema{Type: "null"},
		projectOutputSchema[MinimalProject](),
		projectOutputSchema[github.ProjectV2Field](),
		projectOutputSchema[ProjectItemOutput](),
		projectOutputSchema[MinimalProjectStatusUpdate](),
		projectOutputSchema[MinimalProjectView](),
	)
}

func projectsWriteOutputSchema() *jsonschema.Schema {
	return actionsUnionSchema(
		&jsonschema.Schema{Type: "null"},
		projectOutputSchema[ProjectAddedItemOutput](),
		projectOutputSchema[ProjectItemOutput](),
		projectOutputSchema[MinimalResponse](),
		projectOutputSchema[ProjectBatchOutput](),
		projectOutputSchema[RepositoryMessageOutput](),
		projectOutputSchema[MinimalProjectStatusUpdate](),
		projectOutputSchema[MinimalProjectView](),
		projectOutputSchema[ProjectDeletedViewOutput](),
		projectOutputSchema[ProjectCreatedOutput](),
		projectOutputSchema[ProjectIterationFieldOutput](),
	)
}

func projectOutputSchema[T any]() *jsonschema.Schema {
	schema := forbidOmittedNulls(repositoryOutputSchema[T]())
	var patch func(*jsonschema.Schema)
	patch = func(schema *jsonschema.Schema) {
		if schema == nil {
			return
		}
		for name, property := range schema.Properties {
			switch name {
			case "value":
				schema.Properties[name] = &jsonschema.Schema{Description: "Recursively compacted JSON field value; unknown field types retain arbitrary object keys."}
			case "ref":
				schema.Properties[name] = &jsonschema.Schema{Type: "object", Description: "Original batch reference, including malformed or extra fields.", AdditionalProperties: &jsonschema.Schema{}}
			case "candidates":
				schema.Properties[name] = &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{}, Description: "Resolution candidates supplied by the selected field or item resolver."}
			case "visible_fields":
				property.Type, property.Types = "array", nil
				patch(property)
			default:
				patch(property)
			}
		}
		patch(schema.Items)
		for _, definition := range schema.Defs {
			patch(definition)
		}
	}
	patch(schema)
	return schema
}

func decodeProjectsListOutput(method string, raw []byte) (*ProjectsListOutput, error) {
	out := &ProjectsListOutput{}
	switch method {
	case projectsMethodListProjects:
		return out, json.Unmarshal(raw, &out.Projects)
	case projectsMethodListProjectFields:
		return out, json.Unmarshal(raw, &out.Fields)
	case projectsMethodListProjectItems:
		return out, json.Unmarshal(raw, &out.Items)
	case projectsMethodListProjectStatusUpdates:
		return out, json.Unmarshal(raw, &out.StatusUpdates)
	case projectsMethodListProjectViews:
		return out, json.Unmarshal(raw, &out.Views)
	default:
		return nil, fmt.Errorf("unexpected Projects list output method %q", method)
	}
}

func decodeProjectsGetOutput(method string, raw []byte) (*ProjectsGetOutput, error) {
	out := &ProjectsGetOutput{}
	switch method {
	case projectsMethodGetProject:
		return out, json.Unmarshal(raw, &out.Project)
	case projectsMethodGetProjectField:
		return out, json.Unmarshal(raw, &out.Field)
	case projectsMethodGetProjectItem:
		return out, json.Unmarshal(raw, &out.Item)
	case projectsMethodGetProjectStatusUpdate:
		return out, json.Unmarshal(raw, &out.StatusUpdate)
	case projectsMethodGetProjectView:
		return out, json.Unmarshal(raw, &out.View)
	default:
		return nil, fmt.Errorf("unexpected Projects get output method %q", method)
	}
}

func decodeProjectsWriteOutput(method string, raw []byte) (*ProjectsWriteOutput, error) {
	out := &ProjectsWriteOutput{}
	switch method {
	case projectsMethodAddProjectItem:
		return out, json.Unmarshal(raw, &out.Added)
	case projectsMethodUpdateProjectItem:
		var discriminator struct {
			URL *string `json:"url"`
		}
		if err := json.Unmarshal(raw, &discriminator); err != nil {
			return nil, err
		}
		if discriminator.URL != nil {
			return out, json.Unmarshal(raw, &out.IssueFields)
		}
		return out, json.Unmarshal(raw, &out.Item)
	case projectsMethodUpdateProjectItems:
		return out, json.Unmarshal(raw, &out.Batch)
	case projectsMethodDeleteProjectItem:
		out.DeletedItem = &RepositoryMessageOutput{Message: string(raw)}
		return out, nil
	case projectsMethodCreateProjectStatusUpdate:
		return out, json.Unmarshal(raw, &out.StatusUpdate)
	case projectsMethodCreateProjectView, projectsMethodUpdateProjectView:
		return out, json.Unmarshal(raw, &out.View)
	case projectsMethodDeleteProjectView:
		return out, json.Unmarshal(raw, &out.DeletedView)
	case projectsMethodCreateProject:
		return out, json.Unmarshal(raw, &out.Project)
	case projectsMethodCreateIterationField:
		return out, json.Unmarshal(raw, &out.IterationField)
	default:
		return nil, fmt.Errorf("unexpected Projects write output method %q", method)
	}
}

func projectsTypedHandler[In, Out any](
	handler func(context.Context, ToolDependencies, *mcp.CallToolRequest, map[string]any) (*mcp.CallToolResult, any, error),
	decode func(string, []byte) (Out, error),
) func(context.Context, ToolDependencies, *mcp.CallToolRequest, In) (*mcp.CallToolResult, Out, error) {
	return func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, input In) (*mcp.CallToolResult, Out, error) {
		var zero Out
		args, err := projectArguments(input)
		if err != nil {
			return nil, zero, fmt.Errorf("decode Projects arguments: %w", err)
		}
		result, _, err := handler(ctx, deps, req, args)
		if err != nil || result == nil || result.IsError {
			return result, zero, err
		}
		if len(result.Content) != 1 {
			return nil, zero, fmt.Errorf("expected one Projects text result")
		}
		text, ok := result.Content[0].(*mcp.TextContent)
		if !ok {
			return nil, zero, fmt.Errorf("expected Projects text content")
		}
		method, err := RequiredParam[string](args, "method")
		if err != nil {
			return nil, zero, err
		}
		output, err := decode(method, []byte(text.Text))
		if err != nil {
			return nil, zero, fmt.Errorf("decode %s output: %w", method, err)
		}
		return result, output, nil
	}
}

// Projects' raw handlers historically validated only parameters consumed by
// the selected method. Keep the suggested shape discoverable, but permit other
// JSON values so the same method-specific errors and precedence are preserved.
func projectsInputSchema(tool inventory.ServerTool) inventory.ServerTool {
	schema := tool.Tool.InputSchema.(*jsonschema.Schema)
	schema.Required = nil
	for name, property := range schema.Properties {
		if name == "owner" {
			continue
		}
		schema.Properties[name] = &jsonschema.Schema{
			Description: property.Description + " Validated by the selected method; otherwise ignored.",
			AnyOf: []*jsonschema.Schema{property, {
				Description: "Other JSON values are handled by the method-specific validator, preserving its errors and validation order.",
			}},
		}
	}
	return tool
}

func normalizeProjectsRouting(kind string) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: " + err.Error()}
		}
		method, err := RequiredParam[string](args, "method")
		if err != nil {
			return nil, &inventory.ToolInputError{Message: err.Error()}
		}
		if kind == "get" && (method == projectsMethodGetProjectStatusUpdate || method == projectsMethodGetProjectView) {
			delete(args, "owner")
		} else if _, err := RequiredParam[string](args, "owner"); err != nil {
			return nil, &inventory.ToolInputError{Message: err.Error()}
		}
		return json.Marshal(args)
	}
}
