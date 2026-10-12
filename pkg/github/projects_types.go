package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
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
	Method        string                         `json:"method"`
	Projects      *ProjectListOutput             `json:"projects,omitempty"`
	Fields        *ProjectFieldListOutput        `json:"fields,omitempty"`
	Items         *ProjectItemListOutput         `json:"items,omitempty"`
	StatusUpdates *ProjectStatusUpdateListOutput `json:"status_updates,omitempty"`
	Views         *ProjectViewListOutput         `json:"views,omitempty"`
}

type ProjectListOutput struct {
	Projects []MinimalProject `json:"projects"`
	PageInfo *pageInfo        `json:"pageInfo,omitempty"`
	Note     string           `json:"note,omitempty"`
}

type ProjectFieldListOutput struct {
	Fields   []ProjectFieldOutput `json:"fields"`
	PageInfo pageInfo             `json:"pageInfo"`
}

// ProjectFieldOutput omits the derivable project API URL from REST fields.
type ProjectFieldOutput struct {
	ID            *int64                           `json:"id,omitempty"`
	NodeID        *string                          `json:"node_id,omitempty"`
	Name          *string                          `json:"name,omitempty"`
	DataType      *string                          `json:"data_type,omitempty"`
	Options       []ProjectFieldOptionOutput       `json:"options,omitempty"`
	Configuration *ProjectFieldConfigurationOutput `json:"configuration,omitempty"`
	CreatedAt     *github.Timestamp                `json:"created_at,omitempty"`
	UpdatedAt     *github.Timestamp                `json:"updated_at,omitempty"`
}

type ProjectFieldOptionOutput struct {
	ID          *string                 `json:"id,omitempty"`
	Color       *string                 `json:"color,omitempty"`
	Description *ProjectFieldTextOutput `json:"description,omitempty"`
	Name        *ProjectFieldTextOutput `json:"name,omitempty"`
}

type ProjectFieldTextOutput struct {
	HTML *string `json:"html,omitempty"`
	Raw  *string `json:"raw,omitempty"`
}

type ProjectFieldConfigurationOutput struct {
	Duration   *int                          `json:"duration,omitempty"`
	StartDay   *int                          `json:"start_day,omitempty"`
	Iterations []ProjectFieldIterationOutput `json:"iterations,omitempty"`
}

type ProjectFieldIterationOutput struct {
	ID        *string                 `json:"id,omitempty"`
	Title     *ProjectFieldTextOutput `json:"title,omitempty"`
	StartDate *string                 `json:"start_date,omitempty"`
	Duration  *int                    `json:"duration,omitempty"`
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
	Method       string                      `json:"method"`
	Project      *MinimalProject             `json:"project,omitempty"`
	Field        *ProjectFieldOutput         `json:"field,omitempty"`
	Item         *ProjectItemOutput          `json:"item,omitempty"`
	StatusUpdate *MinimalProjectStatusUpdate `json:"status_update,omitempty"`
	View         *MinimalProjectView         `json:"view,omitempty"`
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
	Method         string                       `json:"method"`
	Added          *ProjectAddedItemOutput      `json:"added,omitempty"`
	Item           *ProjectItemOutput           `json:"item,omitempty"`
	IssueFields    *ProjectIssueOutput          `json:"issue_fields,omitempty"`
	Batch          *ProjectBatchOutput          `json:"batch,omitempty"`
	DeletedItem    *RepositoryMessageOutput     `json:"deleted_item,omitempty"`
	StatusUpdate   *MinimalProjectStatusUpdate  `json:"status_update,omitempty"`
	View           *MinimalProjectView          `json:"view,omitempty"`
	DeletedView    *ProjectDeletedViewOutput    `json:"deleted_view,omitempty"`
	Project        *ProjectCreatedOutput        `json:"project,omitempty"`
	IterationField *ProjectIterationFieldOutput `json:"iteration_field,omitempty"`
}

type ProjectIssueOutput struct {
	ID      string `json:"id"`
	HTMLURL string `json:"html_url"`
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
	ID      string `json:"id"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	HTMLURL string `json:"html_url"`
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
	// Batch entries echo user-supplied reference objects, including malformed
	// references and extra keys, verbatim.
	Ref json.RawMessage `json:"ref,omitempty"`
}

type ProjectBatchError struct {
	Code       string                       `json:"code"`
	Message    string                       `json:"message"`
	Candidates []ProjectResolutionCandidate `json:"candidates,omitempty"`
	Hint       string                       `json:"hint,omitempty"`
}

type ProjectResolutionCandidate struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	DataType string `json:"data_type,omitempty"`
}

func projectsListOutputSchema() *jsonschema.Schema {
	schema := projectOutputSchema[ProjectsListOutput]()
	schema.Properties["method"].Enum = []any{projectsMethodListProjects, projectsMethodListProjectFields, projectsMethodListProjectItems, projectsMethodListProjectStatusUpdates, projectsMethodListProjectViews}
	return schema
}

func projectsGetOutputSchema() *jsonschema.Schema {
	schema := projectOutputSchema[ProjectsGetOutput]()
	schema.Properties["method"].Enum = []any{projectsMethodGetProject, projectsMethodGetProjectField, projectsMethodGetProjectItem, projectsMethodGetProjectStatusUpdate, projectsMethodGetProjectView}
	return schema
}

func projectsWriteOutputSchema() *jsonschema.Schema {
	schema := projectOutputSchema[ProjectsWriteOutput]()
	schema.Properties["method"].Enum = []any{projectsMethodAddProjectItem, projectsMethodUpdateProjectItem, projectsMethodUpdateProjectItems, projectsMethodDeleteProjectItem, projectsMethodCreateProjectStatusUpdate, projectsMethodCreateProjectView, projectsMethodUpdateProjectView, projectsMethodDeleteProjectView, projectsMethodCreateProject, projectsMethodCreateIterationField}
	return schema
}

func projectOutputSchema[T any]() *jsonschema.Schema {
	user := repositoryOutputSchema[MinimalUser]()
	delete(user.Properties, "details")
	generated, err := inventory.CachedSchemaFor[T](&jsonschema.ForOptions{
		TypeSchemas: map[reflect.Type]*jsonschema.Schema{
			reflect.TypeFor[github.Timestamp](): {Type: "string", Format: "date-time"},
			reflect.TypeFor[MinimalUser]():      user,
		},
	})
	if err != nil {
		panic(fmt.Sprintf("failed to generate project output schema: %v", err))
	}
	schema := forbidOmittedNulls(inventory.CloneSchema(generated))
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
			case "visible_fields":
				property.Type, property.Types = "array", nil
				patch(property)
			case "created_at", "updated_at", "closed_at", "deleted_at", "archived_at":
				property.Description = "RFC3339 timestamp."
			case "start_date", "target_date":
				property.Description = "Date in YYYY-MM-DD format."
			case "duration":
				property.Description = "Iteration duration in days."
			case "layout":
				property.Enum = []any{"board", "table", "roadmap"}
			case "status":
				if _, batch := schema.Properties["index"]; batch {
					property.Enum = []any{"succeeded", "failed", "unknown"}
				} else {
					property.Enum = []any{"INACTIVE", "ON_TRACK", "AT_RISK", "OFF_TRACK", "COMPLETE"}
				}
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
	out := &ProjectsListOutput{Method: method}
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
	out := &ProjectsGetOutput{Method: method}
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
	out := &ProjectsWriteOutput{Method: method}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return out, nil
	}
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
			var issue MinimalResponse
			if err := json.Unmarshal(raw, &issue); err != nil {
				return nil, err
			}
			out.IssueFields = &ProjectIssueOutput{ID: issue.ID, HTMLURL: issue.URL}
			return out, nil
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
		var project struct {
			ID     string `json:"id"`
			Number int    `json:"number"`
			Title  string `json:"title"`
			URL    string `json:"url"`
		}
		if err := json.Unmarshal(raw, &project); err != nil {
			return nil, err
		}
		out.Project = &ProjectCreatedOutput{ID: project.ID, Number: project.Number, Title: project.Title, HTMLURL: project.URL}
		return out, nil
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

func projectsSchemaOptions() inventory.TypedSchemaOptions {
	// Legacy Projects validators inspect only arguments consumed by the selected
	// method, after routing and dependency acquisition. Discovery stays strict.
	return inventory.TypedSchemaOptions{
		ValidationInputSchema: &jsonschema.Schema{Type: "object"},
	}
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
