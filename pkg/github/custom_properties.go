package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const customPropertiesLevelDescription = "The level at which custom properties are managed:\n" +
	"- 'repository': The custom property VALUES assigned to a repository (requires 'owner' and 'repo').\n" +
	"- 'organization': The custom property DEFINITIONS (schema) for an organization (requires 'org').\n" +
	"- 'enterprise': The custom property DEFINITIONS (schema) for an enterprise (requires 'enterprise')."

// CustomPropertiesRead creates the custom properties read tool.
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

func customPropertiesReadOrganization(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	properties, errResult := getCustomPropertyDefinitions(ctx, client, fmt.Sprintf("orgs/%s/properties/schema", url.PathEscape(org)), "failed to get organization custom properties")
	if errResult != nil {
		return errResult, nil, nil
	}

	return MarshalledTextResult(properties), nil, nil
}

func customPropertiesReadEnterprise(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	enterprise, err := RequiredParam[string](args, "enterprise")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}

	properties, errResult := getCustomPropertyDefinitions(ctx, client, fmt.Sprintf("enterprises/%s/properties/schema", url.PathEscape(enterprise)), "failed to get enterprise custom properties")
	if errResult != nil {
		return errResult, nil, nil
	}

	return MarshalledTextResult(properties), nil, nil
}

// CustomPropertiesWrite creates the custom properties write tool.
func CustomPropertiesWrite(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "custom_properties_write",
			Description: t("TOOL_CUSTOM_PROPERTIES_WRITE_DESCRIPTION", "Create or update custom properties at the repository, organization, or enterprise level. At the repository level this sets the property values on a repository (the properties must already be defined for the organization). Organization and enterprise definition writes preserve omitted writable fields by reading current definitions immediately before updating; concurrent definition updates remain last-write-wins. Select the level with the 'level' parameter."),
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
						Items: &jsonschema.Schema{
							Type: "object",
							OneOf: []*jsonschema.Schema{
								customPropertyValueSchema(),
								customPropertyDefinitionSchema(),
							},
						},
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

func customPropertiesWriteRepository(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	owner, err := RequiredParam[string](args, "owner")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	repo, err := RequiredParam[string](args, "repo")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	values, errResult := parseCustomProperties[*github.CustomPropertyValue](args, customPropertyValueSchema())
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

func customPropertiesWriteOrganization(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	org, err := RequiredParam[string](args, "org")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	properties, errResult := parseCustomProperties[map[string]any](args, customPropertyDefinitionSchema())
	if errResult != nil {
		return errResult, nil, nil
	}

	return customPropertiesWriteDefinitions(ctx, client, fmt.Sprintf("orgs/%s/properties/schema", url.PathEscape(org)), "organization", properties)
}

func customPropertiesWriteEnterprise(ctx context.Context, client *github.Client, args map[string]any) (*mcp.CallToolResult, any, error) {
	enterprise, err := RequiredParam[string](args, "enterprise")
	if err != nil {
		return utils.NewToolResultError(err.Error()), nil, nil
	}
	properties, errResult := parseCustomProperties[map[string]any](args, customPropertyDefinitionSchema())
	if errResult != nil {
		return errResult, nil, nil
	}

	return customPropertiesWriteDefinitions(ctx, client, fmt.Sprintf("enterprises/%s/properties/schema", url.PathEscape(enterprise)), "enterprise", properties)
}

func customPropertyValueSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		Description:          "A repository-level custom property value.",
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		Properties: map[string]*jsonschema.Schema{
			"property_name": {
				Type:        "string",
				Description: "The name of the custom property.",
			},
			"value": {
				Description: "Repository level only: the value to assign. A string, an array of strings, or null to clear the value.",
				OneOf: []*jsonschema.Schema{
					{Type: "string"},
					{
						Type:  "array",
						Items: &jsonschema.Schema{Type: "string"},
					},
					{Type: "null"},
				},
			},
		},
		Required: []string{"property_name", "value"},
	}
}

func customPropertyDefinitionSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		Description:          "An organization- or enterprise-level custom property definition.",
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
		Properties: map[string]*jsonschema.Schema{
			"property_name": {
				Type:        "string",
				Description: "The name of the custom property.",
			},
			"value_type": {
				Type:        "string",
				Enum:        []any{"string", "single_select", "multi_select", "true_false", "url"},
				Description: "The data type of the property. Required for new definitions; omit when updating to preserve the current type.",
			},
			"required": {
				Type:        "boolean",
				Description: "Whether the property must be set on every repository. Omit when updating to preserve the current setting.",
			},
			"default_value": {
				Description: "The value applied when a repository does not set the property. Omit when updating to preserve the current value; use null to clear it.",
				OneOf: []*jsonschema.Schema{
					{Type: "string"},
					{
						Type:  "array",
						Items: &jsonschema.Schema{Type: "string"},
					},
					{Type: "null"},
				},
			},
			"description": {
				Description: "A short description of the property. Omit when updating to preserve the current description; use null to clear it.",
				AnyOf: []*jsonschema.Schema{
					{Type: "string"},
					{Type: "null"},
				},
			},
			"allowed_values": {
				Description: "The ordered list of allowed values for single_select and multi_select properties. Omit when updating to preserve the current list; use null or an empty array to clear it.",
				AnyOf: []*jsonschema.Schema{
					{
						Type:  "array",
						Items: &jsonschema.Schema{Type: "string"},
					},
					{Type: "null"},
				},
			},
			"values_editable_by": {
				Description: "Who can edit the values of the property. Omit when updating to preserve the current setting; use null to restore the default.",
				AnyOf: []*jsonschema.Schema{
					{
						Type: "string",
						Enum: []any{"org_actors", "org_and_repo_actors"},
					},
					{Type: "null"},
				},
			},
			"require_explicit_values": {
				Type:        "boolean",
				Description: "Whether repositories must explicitly set a value for the property. Omit when updating to preserve the current setting.",
			},
		},
		Required: []string{"property_name"},
	}
}

var customPropertyDefinitionFields = []string{
	"value_type",
	"required",
	"default_value",
	"description",
	"allowed_values",
	"values_editable_by",
	"require_explicit_values",
}

func customPropertiesWriteDefinitions(ctx context.Context, client *github.Client, apiURL, sourceType string, requested []map[string]any) (*mcp.CallToolResult, any, error) {
	if errResult := validateUniqueCustomPropertyNames(requested); errResult != nil {
		return errResult, nil, nil
	}

	current, errResult := getCustomPropertyDefinitions(ctx, client, apiURL, fmt.Sprintf("failed to get %s custom properties before updating", sourceType))
	if errResult != nil {
		return errResult, nil, nil
	}
	merged, errResult := mergeCustomPropertyDefinitions(current, requested, sourceType)
	if errResult != nil {
		return errResult, nil, nil
	}

	req, err := client.NewRequest(ctx, http.MethodPatch, apiURL, map[string]any{"properties": merged})
	if err != nil {
		return utils.NewToolResultErrorFromErr("failed to create custom properties update request", err), nil, nil
	}

	var updated []map[string]any
	resp, err := client.Do(req, &updated)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return ghErrors.NewGitHubAPIErrorResponse(ctx, fmt.Sprintf("failed to update %s custom properties", sourceType), resp, err), nil, nil
	}
	return MarshalledTextResult(updated), nil, nil
}

func getCustomPropertyDefinitions(ctx context.Context, client *github.Client, apiURL, errorMessage string) ([]map[string]any, *mcp.CallToolResult) {
	req, err := client.NewRequest(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to create custom properties request", err)
	}

	var properties []map[string]any
	resp, err := client.Do(req, &properties)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return nil, ghErrors.NewGitHubAPIErrorResponse(ctx, errorMessage, resp, err)
	}
	return properties, nil
}

func validateUniqueCustomPropertyNames(properties []map[string]any) *mcp.CallToolResult {
	seen := make(map[string]struct{}, len(properties))
	for i, property := range properties {
		name := property["property_name"].(string)
		if _, ok := seen[name]; ok {
			return utils.NewToolResultError(fmt.Sprintf("properties[%d].property_name duplicates %q", i, name))
		}
		seen[name] = struct{}{}
	}
	return nil
}

func mergeCustomPropertyDefinitions(current, requested []map[string]any, sourceType string) ([]map[string]any, *mcp.CallToolResult) {
	currentByName := make(map[string]map[string]any, len(current))
	for _, property := range current {
		name, _ := property["property_name"].(string)
		if name != "" {
			currentByName[name] = property
		}
	}

	merged := make([]map[string]any, 0, len(requested))
	for i, update := range requested {
		name := update["property_name"].(string)
		existing, exists := currentByName[name]
		if !exists {
			if _, ok := update["value_type"]; !ok {
				return nil, utils.NewToolResultError(fmt.Sprintf("properties[%d].value_type is required for new property %q", i, name))
			}
			merged = append(merged, update)
			continue
		}
		if existingSource, _ := existing["source_type"].(string); existingSource != "" && existingSource != sourceType {
			return nil, utils.NewToolResultError(fmt.Sprintf("property %q is inherited from %s and cannot be updated at the %s level", name, existingSource, sourceType))
		}

		property := map[string]any{"property_name": name}
		for _, field := range customPropertyDefinitionFields {
			if value, ok := existing[field]; ok {
				property[field] = value
			}
			if value, ok := update[field]; ok {
				property[field] = value
			}
		}
		merged = append(merged, property)
	}
	return merged, nil
}

func parseCustomProperties[T any](args map[string]any, itemSchema *jsonschema.Schema) ([]T, *mcp.CallToolResult) {
	raw, ok := args["properties"]
	if !ok || raw == nil {
		return nil, utils.NewToolResultError("properties parameter is required")
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, utils.NewToolResultError("properties parameter must be an array")
	}
	resolved, err := itemSchema.Resolve(nil)
	if err != nil {
		return nil, utils.NewToolResultErrorFromErr("failed to resolve properties schema", err)
	}
	for i, property := range arr {
		if err := resolved.Validate(property); err != nil {
			return nil, utils.NewToolResultErrorFromErr(fmt.Sprintf("properties[%d] is invalid", i), err)
		}
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
