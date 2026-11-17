package github

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/github/github-mcp-server/v2/pkg/inventory"
	"github.com/github/github-mcp-server/v2/pkg/scopes"
	"github.com/github/github-mcp-server/v2/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var graphqlToolset = inventory.ToolsetMetadata{
	ID:          "graphql",
	Description: "Execute operations against the GitHub GraphQL API",
	Icon:        "code",
}

type graphqlOperationInput struct {
	Operation string         `json:"operation" jsonschema:"Operation body starting with optional variable definitions then the selection set. Omit the query or mutation keyword and operation name. Fragment definitions may follow."`
	Variables map[string]any `json:"variables,omitempty" jsonschema:"Values for the operation variables."`
}

type graphqlOperationPayload struct {
	Query         string         `json:"query"`
	OperationName string         `json:"operationName"`
	Variables     map[string]any `json:"variables,omitempty"`
}

// Set both the operation type and the selected name on the server. Even a
// document containing other operations can only execute the named operation.
func graphqlPayload(input graphqlOperationInput, write bool) graphqlOperationPayload {
	kind := "query"
	if write {
		kind = "mutation"
	}
	return graphqlOperationPayload{
		Query:         kind + " MCPRequest " + input.Operation,
		OperationName: "MCPRequest",
		Variables:     input.Variables,
	}
}

func graphqlOperationTool(write bool) inventory.ServerTool {
	name, description := "github_graphql_query", "Read GitHub data with a GraphQL query using the current GitHub credential."
	if write {
		name, description = "github_graphql_mutation", "Change GitHub data with a GraphQL mutation using the current GitHub credential."
	}
	return NewTool(graphqlToolset, mcp.Tool{
		Name:        name,
		Description: description,
		InputSchema: contextToolInputSchema[graphqlOperationInput](nil),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: !write, DestructiveHint: ToBoolPtr(write)},
	}, scopes.NoScopes(), func(ctx context.Context, deps ToolDependencies, _ *mcp.CallToolRequest, input graphqlOperationInput) (*mcp.CallToolResult, any, error) {
		if deps.GetFlags(ctx).LockdownMode || deps.IsFeatureEnabled(ctx, FeatureFlagIFCLabels) {
			return utils.NewToolResultError("Direct GraphQL is unavailable with lockdown or IFC labels enabled"), nil, nil
		}
		if strings.TrimSpace(input.Operation) == "" {
			return utils.NewToolResultError("A GraphQL operation body is required"), nil, nil
		}
		client, err := deps.GetClient(ctx)
		if err != nil {
			return utils.NewToolResultErrorFromErr("Get GitHub client", err), nil, nil
		}
		endpoint := "graphql"
		if strings.HasSuffix(client.BaseURL(), "/api/v3/") {
			endpoint = "/api/graphql"
		}
		req, err := client.NewRequest(ctx, http.MethodPost, endpoint, graphqlPayload(input, write))
		if err != nil {
			return utils.NewToolResultErrorFromErr("Build GitHub GraphQL request", err), nil, nil
		}
		var response struct {
			Data   json.RawMessage   `json:"data,omitempty"`
			Errors []json.RawMessage `json:"errors,omitempty"`
		}
		if _, err := client.Do(req, &response); err != nil {
			return utils.NewToolResultErrorFromErr("GitHub GraphQL request failed", err), nil, nil
		}
		result := MarshalledTextResult(response)
		result.IsError = len(response.Errors) > 0
		return result, nil, nil
	})
}
