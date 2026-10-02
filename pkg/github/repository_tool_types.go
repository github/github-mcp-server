package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type RepositoryInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

type CreateOrUpdateFileInput struct {
	Owner             string `json:"owner"`
	Repo              string `json:"repo"`
	Path              string `json:"path"`
	Content           string `json:"content"`
	Message           string `json:"message"`
	Branch            string `json:"branch"`
	SHA               string `json:"sha,omitempty"`
	AllowSymlinkWrite bool   `json:"allow_symlink_write,omitempty"`
}

type CreateRepositoryInput struct {
	Name         string `json:"name"`
	Description  string `json:"description,omitempty"`
	Organization string `json:"organization,omitempty"`
	Private      *bool  `json:"private,omitempty"`
	AutoInit     bool   `json:"autoInit,omitempty"`
}

type GetFileContentsInput struct {
	Owner  string   `json:"owner"`
	Repo   string   `json:"repo"`
	Path   string   `json:"path,omitempty"`
	Ref    string   `json:"ref,omitempty"`
	SHA    string   `json:"sha,omitempty"`
	Fields []string `json:"fields,omitempty"`
}

type ForkRepositoryInput struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Organization string `json:"organization,omitempty"`
}

type DeleteFileInput struct {
	Owner   string `json:"owner"`
	Repo    string `json:"repo"`
	Path    string `json:"path"`
	Message string `json:"message"`
	Branch  string `json:"branch"`
}

type CreateBranchInput struct {
	Owner      string `json:"owner"`
	Repo       string `json:"repo"`
	Branch     string `json:"branch"`
	FromBranch string `json:"from_branch,omitempty"`
}

type PushFileInput struct {
	Path    string  `json:"path"`
	Content *string `json:"content"`
}

type PushFilesInput struct {
	Owner   string          `json:"owner"`
	Repo    string          `json:"repo"`
	Branch  string          `json:"branch"`
	Message string          `json:"message"`
	Files   []PushFileInput `json:"files"`
}

type RepositoryTagInput struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	Tag   string `json:"tag"`
}

type ListReleasesInput struct {
	Owner   string   `json:"owner"`
	Repo    string   `json:"repo"`
	Fields  []string `json:"fields,omitempty"`
	Page    *int     `json:"page,omitempty"`
	PerPage *int     `json:"perPage,omitempty"`
}

type ListStarredRepositoriesInput struct {
	Username  string `json:"username,omitempty"`
	Sort      string `json:"sort,omitempty"`
	Direction string `json:"direction,omitempty"`
	Page      *int   `json:"page,omitempty"`
	PerPage   *int   `json:"perPage,omitempty"`
}

type GetFileBlameInput struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Path      string `json:"path"`
	Ref       string `json:"ref,omitempty"`
	StartLine *int   `json:"start_line,omitempty"`
	EndLine   *int   `json:"end_line,omitempty"`
	PerPage   *int   `json:"perPage,omitempty"`
	After     string `json:"after,omitempty"`
}

type ListRepositoryCollaboratorsInput struct {
	Owner       string `json:"owner"`
	Repo        string `json:"repo"`
	Affiliation string `json:"affiliation,omitempty"`
	Page        *int   `json:"page,omitempty"`
	PerPage     *int   `json:"perPage,omitempty"`
}

type RepositoryMessageOutput struct {
	Message string `json:"message"`
}

type RepositoryCollaboratorsOutput struct {
	FirstPage int                   `json:"firstPage"`
	Items     []MinimalCollaborator `json:"items"`
	LastPage  int                   `json:"lastPage"`
	NextPage  int                   `json:"nextPage"`
	PrevPage  int                   `json:"prevPage"`
}

type RepositoryBlameOutput struct {
	Result *BlameResult
}

func (out RepositoryBlameOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(out.Result)
}

// Pointer fields preserve both a complete compact release and a requested
// projection, including explicit false/zero values.
type ReleaseListOutput struct {
	ID          *int64       `json:"id,omitempty"`
	TagName     *string      `json:"tag_name,omitempty"`
	Name        *string      `json:"name,omitempty"`
	Body        *string      `json:"body,omitempty"`
	HTMLURL     *string      `json:"html_url,omitempty"`
	PublishedAt *string      `json:"published_at,omitempty"`
	Prerelease  *bool        `json:"prerelease,omitempty"`
	Draft       *bool        `json:"draft,omitempty"`
	Author      *MinimalUser `json:"author,omitempty"`
}

// Git commits contain recursive parent objects. Keep that real wire contract
// rather than truncating parents or substituting a permissive object schema.
type DeleteFileOutput struct {
	Commit  *github.Commit `json:"commit"`
	Content *struct{}      `json:"content"`
}

type RepositoryTagOutput struct {
	Reference *github.Reference
	Tag       *github.Tag
}

func (out RepositoryTagOutput) MarshalJSON() ([]byte, error) {
	if out.Reference != nil {
		return json.Marshal(out.Reference)
	}
	return json.Marshal(out.Tag)
}

type ForkRepositoryOutput struct {
	Repository *MinimalResponse
	Message    *RepositoryMessageOutput
}

func (out ForkRepositoryOutput) MarshalJSON() ([]byte, error) {
	if out.Repository != nil {
		return json.Marshal(out.Repository)
	}
	return json.Marshal(out.Message)
}

// Repository contents can be a projected directory listing or a heterogeneous
// MCP content response. The latter is represented losslessly as typed content
// blocks, including the human-readable status and embedded text/blob or link.
type RepositoryContentsOutput struct {
	Directory []*github.RepositoryContent
	Content   *RepositoryContentOutput
}

func (out RepositoryContentsOutput) MarshalJSON() ([]byte, error) {
	if out.Content != nil {
		return json.Marshal(out.Content)
	}
	return json.Marshal(out.Directory)
}

type RepositoryContentOutput struct {
	Content []RepositoryContentBlock `json:"content"`
}

type RepositoryContentBlock struct {
	Text     *RepositoryTextBlock
	Resource *RepositoryResourceBlock
	Link     *RepositoryLinkBlock
}

func (out RepositoryContentBlock) MarshalJSON() ([]byte, error) {
	switch {
	case out.Text != nil:
		return json.Marshal(out.Text)
	case out.Resource != nil:
		return json.Marshal(out.Resource)
	default:
		return json.Marshal(out.Link)
	}
}

type RepositoryTextBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type RepositoryResourceBlock struct {
	Type     string                   `json:"type"`
	Resource RepositoryResourceOutput `json:"resource"`
}

type RepositoryResourceOutput struct {
	URI      string  `json:"uri"`
	MIMEType string  `json:"mimeType,omitempty"`
	Text     *string `json:"text,omitempty"`
	Blob     *string `json:"blob,omitempty"`
}

type RepositoryLinkBlock struct {
	Type  string `json:"type"`
	URI   string `json:"uri"`
	Name  string `json:"name"`
	Title string `json:"title,omitempty"`
	Size  *int64 `json:"size,omitempty"`
}

func repositoryContentOutput(result *mcp.CallToolResult) (*RepositoryContentsOutput, error) {
	if result == nil || result.IsError {
		return nil, nil
	}
	out := &RepositoryContentOutput{Content: make([]RepositoryContentBlock, 0, len(result.Content))}
	for _, content := range result.Content {
		switch block := content.(type) {
		case *mcp.TextContent:
			out.Content = append(out.Content, RepositoryContentBlock{Text: &RepositoryTextBlock{Type: "text", Text: block.Text}})
		case *mcp.EmbeddedResource:
			resource := RepositoryResourceOutput{URI: block.Resource.URI, MIMEType: block.Resource.MIMEType}
			if block.Resource.Blob != nil {
				blob := base64.StdEncoding.EncodeToString(block.Resource.Blob)
				resource.Blob = &blob
			} else if block.Resource.Text != "" {
				resource.Text = &block.Resource.Text
			}
			out.Content = append(out.Content, RepositoryContentBlock{Resource: &RepositoryResourceBlock{Type: "resource", Resource: resource}})
		case *mcp.ResourceLink:
			out.Content = append(out.Content, RepositoryContentBlock{Link: &RepositoryLinkBlock{
				Type: "resource_link", URI: block.URI, Name: block.Name, Title: block.Title, Size: block.Size,
			}})
		default:
			return nil, fmt.Errorf("unsupported repository content block %T", content)
		}
	}
	return &RepositoryContentsOutput{Content: out}, nil
}

func repositoryContentResult(result *mcp.CallToolResult) (*mcp.CallToolResult, *RepositoryContentsOutput, error) {
	out, err := repositoryContentOutput(result)
	return result, out, err
}

func normalizeRepositoryArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	for _, field := range []string{"page", "perPage"} {
		if value, present := args[field]; present {
			number, err := toInt(value)
			if err != nil {
				return nil, fmt.Errorf("parameter %s is not a valid number: %w", field, err)
			}
			if number == 0 {
				delete(args, field)
			} else {
				args[field] = number
			}
		}
	}
	return json.Marshal(args)
}

func normalizeRepositoryFieldsArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if fields, present := args["fields"]; present && fields == nil {
		delete(args, "fields")
	}
	if _, err := OptionalStringArrayParam(args, "fields"); err != nil {
		return nil, err
	}
	return json.Marshal(args)
}

func normalizeRepositoryBlameArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if _, present := args["page"]; present {
		return nil, fmt.Errorf("This tool uses cursor-based pagination. Use the 'after' parameter with the 'endCursor' value from the previous response instead of 'page'.") //nolint:revive,staticcheck // Preserve the legacy validation message.
	}
	for _, field := range []string{"start_line", "end_line", "perPage"} {
		if value, present := args[field]; present {
			number, err := toInt(value)
			if err != nil {
				return nil, fmt.Errorf("parameter %s is not a valid number: %w", field, err)
			}
			args[field] = number
		}
	}
	return json.Marshal(args)
}

func normalizePushFilesArguments(raw json.RawMessage) (json.RawMessage, error) {
	var args map[string]any
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	files, ok := args["files"].([]any)
	if !ok {
		return nil, fmt.Errorf("files parameter must be an array of objects with path and content")
	}
	for _, file := range files {
		object, ok := file.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("each file must be an object with path and content")
		}
		path, ok := object["path"].(string)
		if !ok || path == "" {
			return nil, fmt.Errorf("each file must have a path")
		}
		if _, ok := object["content"].(string); !ok {
			return nil, fmt.Errorf("each file must have content")
		}
	}
	return raw, nil
}

func repositoryPagination(page, perPage *int) PaginationParams {
	out := PaginationParams{Page: 1, PerPage: 30}
	if page != nil && *page != 0 {
		out.Page = *page
	}
	if perPage != nil && *perPage != 0 {
		out.PerPage = *perPage
	}
	return out
}

// The SDK timestamp embeds time.Time but marshals as a string. Recursive Git
// and user/team types also need named references, which SDK inference cannot
// discover by itself. All other fields are inferred from the actual API DTOs.
func repositoryOutputSchema[T any]() *jsonschema.Schema {
	options := &jsonschema.ForOptions{TypeSchemas: map[reflect.Type]*jsonschema.Schema{
		reflect.TypeFor[github.Timestamp]():  {Type: "string", Format: "date-time"},
		reflect.TypeFor[github.Commit]():     {Type: "object", Ref: "#/$defs/commit"},
		reflect.TypeFor[github.Team]():       {Type: "object", Ref: "#/$defs/team"},
		reflect.TypeFor[github.Repository](): {Type: "object", Ref: "#/$defs/repository"},
	}}
	schema, err := jsonschema.For[T](options)
	if err != nil {
		panic(err)
	}
	schema.Defs = make(map[string]*jsonschema.Schema)
	type commitSchema github.Commit
	type teamSchema github.Team
	type repoSchema github.Repository
	definitions := map[string]reflect.Type{
		"commit":     reflect.TypeFor[commitSchema](),
		"team":       reflect.TypeFor[teamSchema](),
		"repository": reflect.TypeFor[repoSchema](),
	}
	for {
		added := false
		encoded, err := json.Marshal(schema)
		if err != nil {
			panic(err)
		}
		for name, typ := range definitions {
			if schema.Defs[name] != nil || !strings.Contains(string(encoded), `#/$defs/`+name) {
				continue
			}
			definition, err := jsonschema.ForType(typ, options)
			if err != nil {
				panic(err)
			}
			definition.Type = ""
			definition.Types = []string{"object", "null"}
			schema.Defs[name] = definition
			added = true
		}
		if !added {
			break
		}
	}
	return schema
}

func repositoryUnionSchema(variants ...*jsonschema.Schema) *jsonschema.Schema {
	out := &jsonschema.Schema{OneOf: variants, Defs: make(map[string]*jsonschema.Schema)}
	for _, variant := range variants {
		maps.Copy(out.Defs, variant.Defs)
		variant.Defs = nil
	}
	return out
}

func repositoryTagOutputSchema() *jsonschema.Schema {
	ref := repositoryOutputSchema[github.Reference]()
	tag := repositoryOutputSchema[github.Tag]()
	// The reference keys are always serialized, even for sparse API responses;
	// requiring an actual tag key would reject an empty annotated-tag response.
	tag.Not = &jsonschema.Schema{Required: []string{"ref"}}
	return repositoryUnionSchema(&jsonschema.Schema{Type: "null"}, ref, tag)
}

func forkRepositoryOutputSchema() *jsonschema.Schema {
	return repositoryUnionSchema(
		&jsonschema.Schema{Type: "null"},
		repositoryOutputSchema[MinimalResponse](),
		repositoryOutputSchema[RepositoryMessageOutput](),
	)
}

func deleteFileOutputSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[*DeleteFileOutput]()
	schema.Properties["content"] = &jsonschema.Schema{Type: "null"}
	return schema
}

func repositoryContentsOutputSchema() *jsonschema.Schema {
	text := repositoryOutputSchema[RepositoryTextBlock]()
	text.Properties["type"].Const = new(any("text"))
	resource := repositoryOutputSchema[RepositoryResourceBlock]()
	resource.Properties["type"].Const = new(any("resource"))
	resource.Properties["resource"].OneOf = []*jsonschema.Schema{
		{Required: []string{"text"}, Not: &jsonschema.Schema{Required: []string{"blob"}}},
		{Required: []string{"blob"}, Not: &jsonschema.Schema{Required: []string{"text"}}},
		{Not: &jsonschema.Schema{AnyOf: []*jsonschema.Schema{{Required: []string{"text"}}, {Required: []string{"blob"}}}}},
	}
	link := repositoryOutputSchema[RepositoryLinkBlock]()
	link.Properties["type"].Const = new(any("resource_link"))
	content := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"content": {Type: "array", Items: repositoryUnionSchema(text, resource, link)},
		},
		Required:             []string{"content"},
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
	return repositoryUnionSchema(&jsonschema.Schema{Type: "null"}, repositoryOutputSchema[[]*github.RepositoryContent](), content)
}
