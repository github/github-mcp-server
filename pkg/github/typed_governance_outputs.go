package github

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/google/go-github/v92/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type LabelOutput struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

type ListLabelsOutput struct {
	Labels     []LabelOutput `json:"labels"`
	TotalCount int           `json:"totalCount"`
}

type LabelWriteOutput struct {
	Message string `json:"message"`
}

func labelOutputSchema() *jsonschema.Schema {
	return typedGovernanceSchema[LabelOutput]()
}

func listLabelsOutputSchema() *jsonschema.Schema {
	return typedGovernanceSchema[ListLabelsOutput]()
}

func labelWriteOutputSchema() *jsonschema.Schema {
	return typedGovernanceSchema[LabelWriteOutput]()
}

func typedGovernanceSchema[T any]() *jsonschema.Schema {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		panic(err)
	}
	return forbidOmittedNulls(schema)
}

type rawGovernanceOutput json.RawMessage

func (out rawGovernanceOutput) MarshalJSON() ([]byte, error) {
	if len(out) == 0 {
		return []byte("null"), nil
	}
	return out, nil
}

func customPropertiesReadOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{
				Type: "array",
				Items: &jsonschema.Schema{
					Type:                 "object",
					Description:          "A custom property value or definition returned by the selected level.",
					AdditionalProperties: &jsonschema.Schema{},
				},
			},
			{Type: "null"},
		},
	}
}

func customPropertiesWriteOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{Type: "string", Description: "Status message for repository custom property writes."},
			customPropertiesReadOutputSchema(),
		},
	}
}

func rulesetReadOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			rulesetItemOutputSchema(),
			{
				Type:  "array",
				Items: rulesetItemOutputSchema(),
			},
			{Type: "array", Items: &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}}},
			{
				Type:                 "object",
				Description:          "An enterprise ruleset listing response or rule suite.",
				AdditionalProperties: &jsonschema.Schema{},
				Properties: map[string]*jsonschema.Schema{
					"rulesets": {
						AnyOf: []*jsonschema.Schema{
							{Type: "array", Items: rulesetItemOutputSchema()},
							{Type: "null"},
						},
					},
					"total_count": {Type: "integer"},
				},
			},
			{Type: "object", AdditionalProperties: &jsonschema.Schema{}},
			{Type: "null"},
		},
	}
}

func rulesetItemOutputSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		Description:          "A repository, organization, or enterprise ruleset.",
		AdditionalProperties: &jsonschema.Schema{},
		Properties: map[string]*jsonschema.Schema{
			"id":          {AnyOf: []*jsonschema.Schema{{Type: "integer"}, {Type: "null"}}},
			"name":        {Type: "string"},
			"target":      {Type: "string"},
			"source":      {Type: "string"},
			"enforcement": {Type: "string"},
			"rules": {
				AnyOf: []*jsonschema.Schema{
					{Type: "array", Items: &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}}},
					{Type: "null"},
				},
			},
			"bypass_actors": {
				AnyOf: []*jsonschema.Schema{
					{Type: "array", Items: &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}}},
					{Type: "null"},
				},
			},
			"conditions": {
				AnyOf: []*jsonschema.Schema{
					{Type: "object", AdditionalProperties: &jsonschema.Schema{}},
					{Type: "null"},
				},
			},
			"created_at": {AnyOf: []*jsonschema.Schema{{Type: "string", Format: "date-time"}, {Type: "null"}}},
			"updated_at": {AnyOf: []*jsonschema.Schema{{Type: "string", Format: "date-time"}, {Type: "null"}}},
		},
	}
}

func createdRulesetOutputSchema() *jsonschema.Schema {
	schema := repositoryOutputSchema[*github.RepositoryRuleset]()
	schema.Properties["rules"] = &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "object", AdditionalProperties: &jsonschema.Schema{}},
			},
			{Type: "null"},
		},
	}
	return schema
}

func decodeGovernanceJSON[T any](raw []byte) (T, error) {
	var output T
	if err := json.Unmarshal(raw, &output); err != nil {
		return output, err
	}
	return output, nil
}

func decodeRawGovernanceJSON(raw []byte) (rawGovernanceOutput, error) {
	if !json.Valid(raw) {
		return nil, fmt.Errorf("legacy response is not valid JSON")
	}
	return rawGovernanceOutput(append(json.RawMessage(nil), raw...)), nil
}

func decodeCustomPropertiesWrite(raw []byte) (rawGovernanceOutput, error) {
	if json.Valid(raw) {
		return rawGovernanceOutput(append(json.RawMessage(nil), raw...)), nil
	}
	encoded, err := json.Marshal(string(raw))
	if err != nil {
		return nil, err
	}
	return rawGovernanceOutput(encoded), nil
}

func typedGovernanceTool[Out any](
	legacy inventory.ServerTool,
	outputSchema *jsonschema.Schema,
	decode func([]byte) (Out, error),
) inventory.ServerTool {
	tool := legacy.Tool
	tool.InputSchema = permissiveLegacyInputSchema(tool.InputSchema)
	tool.OutputSchema = outputSchema
	typed := NewTool[map[string]json.RawMessage, Out](
		legacy.Toolset,
		tool,
		legacy.ScopeAccess,
		func(ctx context.Context, deps ToolDependencies, req *mcp.CallToolRequest, _ map[string]json.RawMessage) (*mcp.CallToolResult, Out, error) {
			var zero Out
			result, err := legacy.Handler(deps)(ctx, req)
			if err != nil || result == nil || result.IsError {
				return result, zero, err
			}
			if len(result.Content) != 1 {
				return nil, zero, fmt.Errorf("expected one legacy %s text result", tool.Name)
			}
			content, ok := result.Content[0].(*mcp.TextContent)
			if !ok {
				return nil, zero, fmt.Errorf("expected legacy %s text content", tool.Name)
			}
			output, err := decode([]byte(content.Text))
			if err != nil {
				return nil, zero, fmt.Errorf("decode %s output: %w", tool.Name, err)
			}
			return result, output, nil
		},
	)
	typed.FeatureRule = legacy.FeatureRule
	typed.Enabled = legacy.Enabled
	typed.MinimumProtocolVersion = legacy.MinimumProtocolVersion
	typed.RequiredElicitationMode = legacy.RequiredElicitationMode
	return typed
}

func permissiveLegacyInputSchema(inputSchema any) any {
	schema, ok := inputSchema.(*jsonschema.Schema)
	if !ok || schema == nil {
		return inputSchema
	}
	// Typed registration validates inputs before invoking handlers. Keep the
	// declared shape discoverable without moving legacy validation ahead of
	// method routing or changing the handler's error precedence.
	encoded, err := json.Marshal(schema)
	if err != nil {
		panic(fmt.Sprintf("marshal legacy input schema: %v", err))
	}
	var compatible jsonschema.Schema
	if err := json.Unmarshal(encoded, &compatible); err != nil {
		panic(fmt.Sprintf("clone legacy input schema: %v", err))
	}
	required := make(map[string]struct{}, len(compatible.Required))
	for _, name := range compatible.Required {
		required[name] = struct{}{}
	}
	compatible.Required = nil
	compatible.AdditionalProperties = &jsonschema.Schema{}
	for name, property := range maps.Clone(compatible.Properties) {
		// These fields are used for automatic request routing, which requires
		// their schema type to remain a direct primitive.
		if name == "owner" || name == "repo" {
			continue
		}
		description := property.Description
		if _, isRequired := required[name]; isRequired {
			description += " Required by the legacy handler."
		}
		compatible.Properties[name] = &jsonschema.Schema{
			Description: fmt.Sprintf("%s Validated by the legacy handler to preserve its error behavior.", description),
			AnyOf:       []*jsonschema.Schema{property, {Description: "Other JSON values are validated by the legacy handler."}},
		}
	}
	return &compatible
}
