package github

import (
	"context"
	"encoding/json"
	"fmt"

	ghErrors "github.com/github/github-mcp-server/pkg/errors"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/github/github-mcp-server/pkg/scopes"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetRepositoryCustomProperties creates a tool to get the custom property values assigned to a repository.
func GetRepositoryCustomProperties(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "get_repository_custom_properties",
			Description: t("TOOL_GET_REPOSITORY_CUSTOM_PROPERTIES_DESCRIPTION", "Get the custom property values that are assigned to a repository"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_REPOSITORY_CUSTOM_PROPERTIES_USER_TITLE", "Get repository custom property values"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
				},
				Required: []string{"owner", "repo"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			owner, err := RequiredParam[string](args, "owner")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			repo, err := RequiredParam[string](args, "repo")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			properties, resp, err := client.Repositories.GetAllCustomPropertyValues(ctx, owner, repo)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get repository custom properties", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(properties), nil, nil
		},
	)
}

// CreateOrUpdateRepositoryCustomProperties creates a tool to set custom property values on a repository.
func CreateOrUpdateRepositoryCustomProperties(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_or_update_repository_custom_properties",
			Description: t("TOOL_CREATE_OR_UPDATE_REPOSITORY_CUSTOM_PROPERTIES_DESCRIPTION", "Create or update the custom property values assigned to a repository. The properties must already be defined at the organization level"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_OR_UPDATE_REPOSITORY_CUSTOM_PROPERTIES_USER_TITLE", "Set repository custom property values"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"owner": {
						Type:        "string",
						Description: "Repository owner",
					},
					"repo": {
						Type:        "string",
						Description: "Repository name",
					},
					"properties": {
						Type:        "array",
						Description: "The custom property values to assign to the repository",
						Items:       customPropertyValueItemSchema(),
					},
				},
				Required: []string{"owner", "repo", "properties"},
			},
		},
		[]scopes.Scope{scopes.Repo},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
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

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			resp, err := client.Repositories.CreateOrUpdateCustomProperties(ctx, owner, repo, values)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update repository custom properties", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return utils.NewToolResultText("Repository custom property values updated successfully"), nil, nil
		},
	)
}

// GetOrganizationCustomProperties creates a tool to get the custom property definitions for an organization.
func GetOrganizationCustomProperties(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "get_organization_custom_properties",
			Description: t("TOOL_GET_ORGANIZATION_CUSTOM_PROPERTIES_DESCRIPTION", "Get all custom property definitions (schema) for an organization"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_ORGANIZATION_CUSTOM_PROPERTIES_USER_TITLE", "Get organization custom properties"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "Organization name",
					},
				},
				Required: []string{"org"},
			},
		},
		[]scopes.Scope{scopes.ReadOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			properties, resp, err := client.Organizations.GetAllCustomProperties(ctx, org)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get organization custom properties", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(properties), nil, nil
		},
	)
}

// CreateOrUpdateOrganizationCustomProperties creates a tool to define custom properties for an organization.
func CreateOrUpdateOrganizationCustomProperties(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_or_update_organization_custom_properties",
			Description: t("TOOL_CREATE_OR_UPDATE_ORGANIZATION_CUSTOM_PROPERTIES_DESCRIPTION", "Create or update custom property definitions (schema) for an organization"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_OR_UPDATE_ORGANIZATION_CUSTOM_PROPERTIES_USER_TITLE", "Define organization custom properties"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"org": {
						Type:        "string",
						Description: "Organization name",
					},
					"properties": {
						Type:        "array",
						Description: "The custom property definitions to create or update",
						Items:       customPropertyDefinitionItemSchema(),
					},
				},
				Required: []string{"org", "properties"},
			},
		},
		[]scopes.Scope{scopes.AdminOrg},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			org, err := RequiredParam[string](args, "org")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			properties, errResult := parseCustomProperties[*github.CustomProperty](args)
			if errResult != nil {
				return errResult, nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			updated, resp, err := client.Organizations.CreateOrUpdateCustomProperties(ctx, org, properties)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update organization custom properties", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(updated), nil, nil
		},
	)
}

// GetEnterpriseCustomProperties creates a tool to get the custom property definitions for an enterprise.
func GetEnterpriseCustomProperties(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "get_enterprise_custom_properties",
			Description: t("TOOL_GET_ENTERPRISE_CUSTOM_PROPERTIES_DESCRIPTION", "Get all custom property definitions (schema) for an enterprise"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_GET_ENTERPRISE_CUSTOM_PROPERTIES_USER_TITLE", "Get enterprise custom properties"),
				ReadOnlyHint: true,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enterprise": {
						Type:        "string",
						Description: "Enterprise slug",
					},
				},
				Required: []string{"enterprise"},
			},
		},
		[]scopes.Scope{scopes.ReadEnterprise},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			enterprise, err := RequiredParam[string](args, "enterprise")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			properties, resp, err := client.Enterprise.GetAllCustomProperties(ctx, enterprise)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to get enterprise custom properties", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(properties), nil, nil
		},
	)
}

// CreateOrUpdateEnterpriseCustomProperties creates a tool to define custom properties for an enterprise.
func CreateOrUpdateEnterpriseCustomProperties(t translations.TranslationHelperFunc) inventory.ServerTool {
	return NewTool(
		ToolsetMetadataGovernance,
		mcp.Tool{
			Name:        "create_or_update_enterprise_custom_properties",
			Description: t("TOOL_CREATE_OR_UPDATE_ENTERPRISE_CUSTOM_PROPERTIES_DESCRIPTION", "Create or update custom property definitions (schema) for an enterprise"),
			Annotations: &mcp.ToolAnnotations{
				Title:        t("TOOL_CREATE_OR_UPDATE_ENTERPRISE_CUSTOM_PROPERTIES_USER_TITLE", "Define enterprise custom properties"),
				ReadOnlyHint: false,
			},
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"enterprise": {
						Type:        "string",
						Description: "Enterprise slug",
					},
					"properties": {
						Type:        "array",
						Description: "The custom property definitions to create or update",
						Items:       customPropertyDefinitionItemSchema(),
					},
				},
				Required: []string{"enterprise", "properties"},
			},
		},
		[]scopes.Scope{scopes.AdminEnterprise},
		func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, args map[string]any) (*mcp.CallToolResult, any, error) {
			enterprise, err := RequiredParam[string](args, "enterprise")
			if err != nil {
				return utils.NewToolResultError(err.Error()), nil, nil
			}
			properties, errResult := parseCustomProperties[*github.CustomProperty](args)
			if errResult != nil {
				return errResult, nil, nil
			}

			client, err := deps.GetClient(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			updated, resp, err := client.Enterprise.CreateOrUpdateCustomProperties(ctx, enterprise, properties)
			if err != nil {
				return ghErrors.NewGitHubAPIErrorResponse(ctx, "failed to update enterprise custom properties", resp, err), nil, nil
			}
			defer func() { _ = resp.Body.Close() }()

			return MarshalledTextResult(updated), nil, nil
		},
	)
}

// customPropertyValueItemSchema describes a single custom property value assigned
// to a repository.
func customPropertyValueItemSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"property_name": {
				Type:        "string",
				Description: "The name of the custom property",
			},
			"value": {
				Description: "The value to assign. A string, an array of strings, or null to clear the value",
			},
		},
		Required: []string{"property_name"},
	}
}

// customPropertyDefinitionItemSchema describes a single custom property definition
// in an organization or enterprise schema.
func customPropertyDefinitionItemSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"property_name": {
				Type:        "string",
				Description: "The name of the custom property",
			},
			"value_type": {
				Type:        "string",
				Enum:        []any{"string", "single_select", "multi_select", "true_false", "url"},
				Description: "The type of the value for the property",
			},
			"required": {
				Type:        "boolean",
				Description: "Whether the property is required",
			},
			"default_value": {
				Description: "Default value of the property. A string or an array of strings",
			},
			"description": {
				Type:        "string",
				Description: "Short description of the property",
			},
			"allowed_values": {
				Type:        "array",
				Description: "An ordered list of the allowed values of the property (for single_select and multi_select)",
				Items:       &jsonschema.Schema{Type: "string"},
			},
			"values_editable_by": {
				Type:        "string",
				Enum:        []any{"org_actors", "org_and_repo_actors"},
				Description: "Who can edit the values of the property",
			},
		},
		Required: []string{"property_name", "value_type"},
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
