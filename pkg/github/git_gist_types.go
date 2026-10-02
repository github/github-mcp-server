package github

import (
	"encoding/json"
	"slices"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
)

// gitGistNormalizer applies the legacy argument validation, in legacy order and
// with legacy messages, before SDK schema validation can reject the request.
// The optional canonicalize hook rewrites accepted legacy values (such as
// numeric strings) into schema-valid ones.
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

func treeOutputSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[TreeResponse]()
	// The SDK validates the zero output on error results, so tree stays nullable.
	// Size is omitted, never null, for entries without a size.
	size := schema.Properties["tree"].Items.Properties["size"]
	size.Types, size.Type = nil, "integer"
	return schema
}

func gistOutputSchema() *jsonschema.Schema {
	return forbidOmittedNulls(repositoryOutputSchema[github.Gist]())
}

func gistListOutputSchema() *jsonschema.Schema {
	return forbidOmittedNulls(repositoryOutputSchema[[]*github.Gist]())
}

// forbidOmittedNulls removes null from every optional property. go-github
// marks such fields omitempty, so absent values are omitted rather than null.
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

func gistMutationOutputSchema() *jsonschema.Schema {
	return repositoryOutputSchema[MinimalResponse]()
}
