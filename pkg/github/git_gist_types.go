package github

import (
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

// gitGistNormalizer applies the legacy argument validation, in legacy order and
// with legacy messages, before SDK schema validation can reject the request.
// Validators may also canonicalize accepted values by updating args.
func gitGistNormalizer(validate func(args map[string]any) error) inventory.InputNormalizer {
	return func(raw json.RawMessage) (json.RawMessage, error) {
		var args map[string]any
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: " + err.Error()}
		}
		if args == nil {
			return nil, &inventory.ToolInputError{Message: "invalid arguments: arguments must be a JSON object"}
		}
		if err := validate(args); err != nil {
			return nil, &inventory.ToolInputError{Message: err.Error()}
		}
		return json.Marshal(args)
	}
}

func validateTreeArguments(args map[string]any) error {
	for _, field := range []string{"owner", "repo"} {
		if _, err := RequiredParam[string](args, field); err != nil {
			return err
		}
	}
	if _, err := OptionalParam[string](args, "tree_sha"); err != nil {
		return err
	}
	if _, err := OptionalBoolParamWithDefault(args, "recursive", false); err != nil {
		return err
	}
	_, err := OptionalParam[string](args, "path_filter")
	return err
}

func validateListGistsArguments(args map[string]any) error {
	if _, err := OptionalParam[string](args, "username"); err != nil {
		return err
	}
	if _, err := OptionalParam[string](args, "since"); err != nil {
		return err
	}
	pagination, err := OptionalPaginationParams(args)
	if err != nil {
		return err
	}
	args["page"], args["perPage"] = pagination.Page, pagination.PerPage
	return nil
}

func validateGetGistArguments(args map[string]any) error {
	_, err := RequiredParam[string](args, "gist_id")
	return err
}

func validateCreateGistArguments(args map[string]any) error {
	if _, err := OptionalParam[string](args, "description"); err != nil {
		return err
	}
	for _, field := range []string{"filename", "content"} {
		if _, err := RequiredParam[string](args, field); err != nil {
			return err
		}
	}
	_, err := OptionalParam[bool](args, "public")
	return err
}

func validateUpdateGistArguments(args map[string]any) error {
	if _, err := RequiredParam[string](args, "gist_id"); err != nil {
		return err
	}
	if _, err := OptionalParam[string](args, "description"); err != nil {
		return err
	}
	for _, field := range []string{"filename", "content"} {
		if _, err := RequiredParam[string](args, field); err != nil {
			return err
		}
	}
	return nil
}

var treeOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := repositoryOutputSchema[RepositoryTreeOutput]()
	entry := schema.Properties["tree"].Items
	entry.Properties["type"].Enum = []any{"blob", "tree", "commit", ""}
	entry.Properties["mode"].Enum = []any{"100644", "100755", "040000", "160000", "120000", ""}
	entry.Properties["type"].Description = "Git object type; empty if absent in the API response."
	entry.Properties["mode"].Description = "Git file mode in octal; empty if absent in the API response."
	entry.Properties["size"].Description = "Blob size in bytes, when available."
	// The SDK validates the zero output on error results, so tree stays nullable.
	// Size is omitted, never null, for entries without a size.
	size := entry.Properties["size"]
	size.Types, size.Type = nil, "integer"
	return schema
})

type RepositoryTreeOutput struct {
	SHA       string                      `json:"sha"`
	Truncated bool                        `json:"truncated"`
	Tree      []RepositoryTreeEntryOutput `json:"tree"`
	TreeSHA   string                      `json:"tree_sha"`
	Owner     string                      `json:"owner"`
	Repo      string                      `json:"repo"`
	Recursive bool                        `json:"recursive"`
	Count     int                         `json:"count"`
}

type RepositoryTreeEntryOutput struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size *int   `json:"size,omitempty"`
	Mode string `json:"mode"`
	SHA  string `json:"sha"`
}

func projectRepositoryTree(tree TreeResponse) *RepositoryTreeOutput {
	entries := make([]RepositoryTreeEntryOutput, len(tree.Tree))
	for i, entry := range tree.Tree {
		entries[i] = RepositoryTreeEntryOutput{
			Path: entry.Path, Type: entry.Type, Size: entry.Size, Mode: entry.Mode, SHA: entry.SHA,
		}
	}
	return &RepositoryTreeOutput{
		SHA: tree.SHA, Truncated: tree.Truncated, Tree: entries,
		TreeSHA: tree.TreeSHA, Owner: tree.Owner, Repo: tree.Repo,
		Recursive: tree.Recursive, Count: tree.Count,
	}
}

type GistOutput struct {
	ID          *string                   `json:"id,omitempty"`
	Description *string                   `json:"description,omitempty"`
	Public      *bool                     `json:"public,omitempty"`
	Owner       *MinimalUser              `json:"owner,omitempty"`
	Files       map[string]GistFileOutput `json:"files,omitempty"`
	Comments    *int                      `json:"comments,omitempty"`
	HTMLURL     *string                   `json:"html_url,omitempty"`
	GitPullURL  *string                   `json:"git_pull_url,omitempty"`
	CreatedAt   string                    `json:"created_at,omitempty"`
	UpdatedAt   string                    `json:"updated_at,omitempty"`
}

type GistReadOutput struct {
	Gist *GistOutput
}

func (output GistReadOutput) MarshalJSON() ([]byte, error) {
	return json.Marshal(output.Gist)
}

type GistFileOutput struct {
	Filename *string `json:"filename,omitempty"`
	Content  *string `json:"content,omitempty"`
	Size     *int    `json:"size,omitempty"`
	Language *string `json:"language,omitempty"`
	Type     *string `json:"type,omitempty"`
	RawURL   *string `json:"raw_url,omitempty"`
}

func projectGist(gist *github.Gist) *GistOutput {
	if gist == nil {
		return nil
	}
	output := &GistOutput{
		ID: gist.ID, Description: gist.Description, Public: gist.Public,
		Owner: convertToMinimalUser(gist.Owner), Comments: gist.Comments, HTMLURL: gist.HTMLURL, GitPullURL: gist.GitPullURL,
	}
	if len(gist.Files) != 0 {
		output.Files = make(map[string]GistFileOutput, len(gist.Files))
		for name, file := range gist.Files {
			output.Files[string(name)] = GistFileOutput{
				Filename: file.Filename, Content: file.Content, Size: file.Size, Language: file.Language, Type: file.Type, RawURL: file.RawURL,
			}
		}
	}
	if gist.CreatedAt != nil {
		output.CreatedAt = gist.CreatedAt.Format(time.RFC3339Nano)
	}
	if gist.UpdatedAt != nil {
		output.UpdatedAt = gist.UpdatedAt.Format(time.RFC3339Nano)
	}
	return output
}

func projectGists(gists []*github.Gist) []*GistOutput {
	if gists == nil {
		return nil
	}
	output := make([]*GistOutput, len(gists))
	for i, gist := range gists {
		output[i] = projectGist(gist)
	}
	return output
}

var gistOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	return repositoryUnionSchema(&jsonschema.Schema{Type: "null"}, gistObjectSchema())
})

var gistObjectSchema = sync.OnceValue(func() *jsonschema.Schema {
	schema := forbidOmittedNulls(repositoryOutputSchema[GistOutput]())
	schema.Properties["created_at"].Format = "date-time"
	schema.Properties["updated_at"].Format = "date-time"
	schema.Properties["created_at"].Description = "Creation time in RFC3339 format."
	schema.Properties["updated_at"].Description = "Last update time in RFC3339 format."
	schema.Properties["public"].Description = "True for a public gist; false for a secret, URL-accessible gist."
	schema.Properties["files"].AdditionalProperties.Properties["size"].Description = "File size in bytes."
	schema.Properties["files"].AdditionalProperties.Properties["type"].Description = "File MIME type, when available."
	// convertToMinimalUser never populates Details.
	delete(schema.Properties["owner"].Properties, "details")
	return schema
})

var gistListOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	return &jsonschema.Schema{
		Types: []string{"array", "null"},
		Items: gistOutputSchema(),
	}
})

// relaxedPaginationValidationSchema clones a WithPagination-derived input
// schema and clears the page/perPage bounds in the clone only. The advertised
// InputSchema (passed in) is left untouched, so tools/list still documents the
// min/max metadata; only the runtime ValidationInputSchema is relaxed, so the
// SDK continues accepting legacy out-of-range values (e.g. page=-1,
// perPage=101) that the handler has always forwarded unmodified to the
// GitHub API.
func relaxedPaginationValidationSchema(schema *jsonschema.Schema) *jsonschema.Schema {
	cloned := inventory.CloneSchema(schema)
	if page := cloned.Properties["page"]; page != nil {
		page.Minimum = nil
	}
	if perPage := cloned.Properties["perPage"]; perPage != nil {
		perPage.Minimum = nil
		perPage.Maximum = nil
	}
	return cloned
}

// Optional pointer fields are omitted rather than serialized as null.
func forbidOmittedNulls(schema *jsonschema.Schema) *jsonschema.Schema {
	if schema == nil {
		return nil
	}
	required := make(map[string]bool, len(schema.Required))
	for _, name := range schema.Required {
		required[name] = true
	}
	for name, property := range schema.Properties {
		if !required[name] {
			property.Types = slices.DeleteFunc(slices.Clone(property.Types), func(t string) bool { return t == "null" })
			if len(property.Types) == 1 {
				property.Type, property.Types = property.Types[0], nil
			}
		}
		forbidOmittedNulls(property)
	}
	forbidOmittedNulls(schema.Items)
	forbidOmittedNulls(schema.AdditionalProperties)
	for _, definition := range schema.Defs {
		forbidOmittedNulls(definition)
	}
	return schema
}

var gistMutationOutputSchema = sync.OnceValue(func() *jsonschema.Schema {
	return repositoryOutputSchema[MinimalResponse]()
})
