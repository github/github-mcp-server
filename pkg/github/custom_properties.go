package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// customPropertiesLevelDescription documents the "level" parameter shared by
// the custom properties read and write tools. The repository level operates on
// the property VALUES assigned to a repository, while the organization and
// enterprise levels operate on the property DEFINITIONS (schema).
const customPropertiesLevelDescription = "The level at which custom properties are managed:\n" +
	"- 'repository': The custom property VALUES assigned to a repository (requires 'owner' and 'repo').\n" +
	"- 'organization': The custom property DEFINITIONS (schema) for an organization (requires 'org').\n" +
	"- 'enterprise': The custom property DEFINITIONS (schema) for an enterprise (requires 'enterprise')."

// CustomPropertiesRead creates a tool for read operations on custom properties
// at the repository, organization, or enterprise level. The level is selected
// with the "level" parameter. Repository reads return property values;
// organization and enterprise reads return property definitions.
func CustomPropertiesRead(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "custom_properties_read",
			Description: t("TOOL_CUSTOM_PROPERTIES_READ_DESCRIPTION", "Read custom properties at the repository, organization, or enterprise level. At the repository level this returns the property values assigned to a repository; at the organization and enterprise levels it returns the property definitions (schema). Select the level with the 'level' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CUSTOM_PROPERTIES_READ_USER_TITLE", "Read custom properties"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"level": {
						Type:        "string",
						Enum:        []any{"repository", "organization", "enterprise"},
						Description: customPropertiesLevelDescription,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner. Required when level is 'repository'.",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Required when level is 'repository'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization name. Required when level is 'organization'.",
					},
					"enterprise": {
						Type:        "string",
						Description: "Enterprise slug. Required when level is 'enterprise'.",
					},
				},
				Required: []string{"level"},
			},
		},
		rulesetReadScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			level, err := RequiredParam[string](args, "level")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch level {
			case "repository":
				return customPropertiesReadRepository(ctx, client, args)
			case "organization":
				return customPropertiesReadOrganization(ctx, client, args)
			case "enterprise":
				return customPropertiesReadEnterprise(ctx, client, args)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown level: %q (expected 'repository', 'organization', or 'enterprise')", level)), nil, nil
			}
		},
	)
}

// customPropertiesReadRepository handles custom_properties_read calls with level="repository".
func customPropertiesReadRepository(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	properties, resp, err := client.Repositories.GetAllCustomPropertyValues(ctx, owner, repo)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository custom property values", resp, err), nil, nil
	}

	return MarshalledTextResult(properties), nil, nil
}

// customPropertiesReadOrganization handles custom_properties_read calls with level="organization".
func customPropertiesReadOrganization(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	properties, resp, err := client.Organizations.GetAllCustomProperties(ctx, org)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get organization custom properties", resp, err), nil, nil
	}

	return MarshalledTextResult(properties), nil, nil
}

// customPropertiesReadEnterprise handles custom_properties_read calls with level="enterprise".
func customPropertiesReadEnterprise(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	enterprise, err := RequiredParam[string](args, "enterprise")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	properties, resp, err := client.Enterprise.GetAllCustomProperties(ctx, enterprise)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get enterprise custom properties", resp, err), nil, nil
	}

	return MarshalledTextResult(properties), nil, nil
}

// CustomPropertiesWrite creates a tool for create-or-update operations on
// custom properties at the repository, organization, or enterprise level. The
// level is selected with the "level" parameter. Repository writes set property
// values; organization and enterprise writes define property schemas.
func CustomPropertiesWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "custom_properties_write",
			Description: t("TOOL_CUSTOM_PROPERTIES_WRITE_DESCRIPTION", "Create or update custom properties at the repository, organization, or enterprise level. At the repository level this sets the property values on a repository (the properties must already be defined for the organization); at the organization and enterprise levels it creates or updates the property definitions (schema). Select the level with the 'level' parameter."),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CUSTOM_PROPERTIES_WRITE_USER_TITLE", "Set custom properties"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"level": {
						Type:        "string",
						Enum:        []any{"repository", "organization", "enterprise"},
						Description: customPropertiesLevelDescription,
					},
					"owner": {
						Type:        "string",
						Description: "Repository owner. Required when level is 'repository'.",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name. Required when level is 'repository'.",
					},
					"org": {
						Type:        "string",
						Description: "Organization name. Required when level is 'organization'.",
					},
					"enterprise": {
						Type:        "string",
						Description: "Enterprise slug. Required when level is 'enterprise'.",
					},
					"properties": {
						Type:        "array",
						Description: "The custom properties to create or update. At the repository level each item assigns a value ('property_name' and 'value'); at the organization and enterprise levels each item defines the schema ('property_name' and 'value_type', plus optional definition fields).",
						Items:       customPropertyItemSchema(),
					},
				},
				Required: []string{"level", "properties"},
			},
		},
		rulesetWriteScopeAccess(),
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			level, err := RequiredParam[string](args, "level")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			switch level {
			case "repository":
				return customPropertiesWriteRepository(ctx, client, args)
			case "organization":
				return customPropertiesWriteOrganization(ctx, client, args)
			case "enterprise":
				return customPropertiesWriteEnterprise(ctx, client, args)
			default:
				return utils.NewToolResultError(fmt.Sprintf("unknown level: %q (expected 'repository', 'organization', or 'enterprise')", level)), nil, nil
			}
		},
	)
}

// customPropertiesWriteRepository handles custom_properties_write calls with level="repository".
func customPropertiesWriteRepository(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	values, errResult := parseCustomProperties[*github.CustomPropertyValue](args)
	if errResult != nil {
		return errResult, nil, nil
	}

	resp, err := client.Repositories.CreateOrUpdateCustomProperties(ctx, owner, repo, values)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update repository custom property values", resp, err), nil, nil
	}

	return utils.NewToolResultText("Repository custom property values updated successfully"), nil, nil
}

// customPropertiesWriteOrganization handles custom_properties_write calls with level="organization".
func customPropertiesWriteOrganization(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	properties, errResult := parseCustomProperties[*github.CustomProperty](args)
	if errResult != nil {
		return errResult, nil, nil
	}

	updated, resp, err := client.Organizations.CreateOrUpdateCustomProperties(ctx, org, properties)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update organization custom properties", resp, err), nil, nil
	}

	return MarshalledTextResult(updated), nil, nil
}

// customPropertiesWriteEnterprise handles custom_properties_write calls with level="enterprise".
func customPropertiesWriteEnterprise(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	enterprise, err := RequiredParam[string](args, "enterprise")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	properties, errResult := parseCustomProperties[*github.CustomProperty](args)
	if errResult != nil {
		return errResult, nil, nil
	}

	updated, resp, err := client.Enterprise.CreateOrUpdateCustomProperties(ctx, enterprise, properties)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update enterprise custom properties", resp, err), nil, nil
	}

	return MarshalledTextResult(updated), nil, nil
}

// customPropertyItemSchema describes a single item of the "properties" array
// for custom_properties_write. It covers both a repository property value and
// an organization or enterprise property definition; which fields apply
// depends on the "level" argument.
func customPropertyItemSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"property_name": {
				Type:        "string",
				Description: "The name of the custom property.",
			},
			"value": {
				Description: "Repository level only: the value to assign. A string, an array of strings, or null to clear the value.",
			},
			"value_type": {
				Type:        "string",
				Enum:        []any{"string", "single_select", "multi_select", "true_false", "url"},
				Description: "Organization and enterprise levels only: the data type of the property. Required when defining a property.",
			},
			"required": {
				Type:        "boolean",
				Description: "Organization and enterprise levels only: whether the property must be set on every repository.",
			},
			"default_value": {
				Description: "Organization and enterprise levels only: the value applied when a repository does not set the property. A string or an array of strings.",
			},
			"description": {
				Type:        "string",
				Description: "Organization and enterprise levels only: a short description of the property.",
			},
			"allowed_values": {
				Type:        "array",
				Description: "Organization and enterprise levels only: the ordered list of allowed values for single_select and multi_select properties.",
				Items:       &jsonschema.Schema{Type: "string"},
			},
			"values_editable_by": {
				Type:        "string",
				Enum:        []any{"org_actors", "org_and_repo_actors"},
				Description: "Organization and enterprise levels only: who can edit the values of the property.",
			},
		},
		Required: []string{"property_name"},
	}
}

// parseCustomProperties reads the "properties" array argument and decodes it into
// the requested go-github type. It returns a non-nil *mcp.CallToolResult
// describing the problem when the argument is missing or malformed.
func parseCustomProperties[T any](args map[string]any) ([]T, *mcp.CallToolResult) {
	raw, ok := args["properties"]
	if !ok || raw == nil {
		return nil, utils.NewToolResultError("properties parameter is required")
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, utils.NewToolResultError("properties parameter must be an array")
	}

	encoded, err := json.Marshal(arr)
	if err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to encode properties", err)
	}
	var out []T
	if err := json.Unmarshal(encoded, &out); err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to parse properties", err)
	}
	return out, nil
}
