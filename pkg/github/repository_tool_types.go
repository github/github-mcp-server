package github

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
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
	PublishedAt *string      `json:"published_at,omitempty" jsonschema:"Publication time in RFC3339 format."`
	Prerelease  *bool        `json:"prerelease,omitempty" jsonschema:"Whether the release is a prerelease."`
	Draft       *bool        `json:"draft,omitempty" jsonschema:"Whether the release is unpublished."`
	Author      *MinimalUser `json:"author,omitempty"`
}

type RepositoryGitObjectOutput struct {
	Type string `json:"type" jsonschema:"Git object kind: commit, tag, tree, or blob."`
	SHA  string `json:"sha"`
}

type RepositoryReferenceOutput struct {
	Ref    string                    `json:"ref" jsonschema:"Fully qualified reference, such as refs/heads/main."`
	Object RepositoryGitObjectOutput `json:"object"`
}

type RepositoryAnnotatedTagOutput struct {
	Tag     string                     `json:"tag"`
	SHA     string                     `json:"sha"`
	Message string                     `json:"message,omitempty"`
	Tagger  *MinimalCommitAuthor       `json:"tagger,omitempty"`
	Object  *RepositoryGitObjectOutput `json:"object,omitempty"`
}

type RepositoryGitCommitOutput struct {
	SHA       string               `json:"sha"`
	Message   string               `json:"message,omitempty"`
	HTMLURL   string               `json:"html_url,omitempty"`
	Author    *MinimalCommitAuthor `json:"author,omitempty"`
	Committer *MinimalCommitAuthor `json:"committer,omitempty"`
	TreeSHA   string               `json:"tree_sha,omitempty"`
	Parents   []string             `json:"parents,omitempty" jsonschema:"Parent commit SHAs in Git parent order."`
}

type DeleteFileOutput struct {
	Commit  *RepositoryGitCommitOutput `json:"commit"`
	Content *struct{}                  `json:"content"`
}

type RepositoryTagOutput struct {
	Reference *RepositoryReferenceOutput
	Tag       *RepositoryAnnotatedTagOutput
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
	Directory []*RepositoryDirectoryEntryOutput
	Content   *RepositoryContentOutput
}

// Optional pointer fields retain projected zero values without reinstating
// omitted fields. API URLs are included only when explicitly projected.
type RepositoryDirectoryEntryOutput struct {
	Type            *string `json:"type,omitempty"`
	Name            *string `json:"name,omitempty"`
	Path            *string `json:"path,omitempty"`
	SHA             *string `json:"sha,omitempty"`
	Size            *int    `json:"size,omitempty" jsonschema:"File size in bytes."`
	HTMLURL         *string `json:"html_url,omitempty"`
	DownloadURL     *string `json:"download_url,omitempty"`
	URL             *string `json:"url,omitempty"`
	GitURL          *string `json:"git_url,omitempty"`
	Target          *string `json:"target,omitempty"`
	SubmoduleGitURL *string `json:"submodule_git_url,omitempty" jsonschema:"Git remote URL for a submodule."`
}

func repositoryReferenceOutput(ref *github.Reference) *RepositoryReferenceOutput {
	if ref == nil {
		return nil
	}
	return &RepositoryReferenceOutput{
		Ref: ref.GetRef(),
		Object: RepositoryGitObjectOutput{
			Type: ref.GetObject().GetType(), SHA: ref.GetObject().GetSHA(),
		},
	}
}

func repositoryCommitAuthorOutput(author *github.CommitAuthor) *MinimalCommitAuthor {
	if author == nil {
		return nil
	}
	out := &MinimalCommitAuthor{Name: author.GetName(), Email: author.GetEmail()}
	if author.Date != nil {
		out.Date = author.Date.Format("2006-01-02T15:04:05Z07:00")
	}
	return out
}

func repositoryGitCommitOutput(commit *github.Commit) *RepositoryGitCommitOutput {
	if commit == nil {
		return nil
	}
	out := &RepositoryGitCommitOutput{
		SHA: commit.GetSHA(), Message: commit.GetMessage(), HTMLURL: commit.GetHTMLURL(),
		Author: repositoryCommitAuthorOutput(commit.Author), Committer: repositoryCommitAuthorOutput(commit.Committer),
		TreeSHA: commit.GetTree().GetSHA(),
	}
	for _, parent := range commit.Parents {
		if parent != nil {
			out.Parents = append(out.Parents, parent.GetSHA())
		}
	}
	return out
}

func repositoryAnnotatedTagOutput(tag *github.Tag) *RepositoryAnnotatedTagOutput {
	if tag == nil {
		return nil
	}
	out := &RepositoryAnnotatedTagOutput{
		Tag: tag.GetTag(), SHA: tag.GetSHA(), Message: tag.GetMessage(),
		Tagger: repositoryCommitAuthorOutput(tag.Tagger),
	}
	if tag.Object != nil {
		out.Object = &RepositoryGitObjectOutput{Type: tag.Object.GetType(), SHA: tag.Object.GetSHA()}
	}
	return out
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
	Blob     *string `json:"blob,omitempty" jsonschema:"Base64-encoded file bytes."`
}

type RepositoryLinkBlock struct {
	Type  string `json:"type"`
	URI   string `json:"uri"`
	Name  string `json:"name"`
	Title string `json:"title,omitempty"`
	Size  *int64 `json:"size,omitempty" jsonschema:"File size in bytes."`
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

func repositoryOutputSchema[T any]() *jsonschema.Schema {
	schema, err := inventory.CachedSchemaFor[T](nil)
	if err != nil {
		panic(fmt.Sprintf("failed to generate repository output schema: %v", err))
	}
	return inventory.CloneSchema(schema)
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
	ref := repositoryOutputSchema[RepositoryReferenceOutput]()
	tag := repositoryOutputSchema[RepositoryAnnotatedTagOutput]()
	tag.Properties["tagger"].Properties["date"].Description = "Tagger time in RFC3339 format."
	return repositoryUnionSchema(&jsonschema.Schema{Type: "null"}, ref, tag)
}

func repositoryReleaseOutputSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[*MinimalRelease]()
	schema.Properties["published_at"].Description = "Publication time in RFC3339 format."
	schema.Properties["prerelease"].Description = "Whether the release is a prerelease."
	schema.Properties["draft"].Description = "Whether the release is unpublished."
	return schema
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
	for _, field := range []string{"author", "committer"} {
		schema.Properties["commit"].Properties[field].Properties["date"].Description = "Commit timestamp in RFC3339 format."
	}
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
	directory := repositoryOutputSchema[[]*RepositoryDirectoryEntryOutput]()
	directory.Type, directory.Types = "array", nil
	return repositoryUnionSchema(&jsonschema.Schema{Type: "null"}, directory, content)
}
