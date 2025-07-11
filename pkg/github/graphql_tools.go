package github

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ExecuteGraphQLQuery creates a tool to execute a GraphQL query and return results
func ExecuteGraphQLQuery(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("execute_graphql_query",
			mcp.WithDescription(t("TOOL_EXECUTE_GRAPHQL_QUERY_DESCRIPTION", "Execute a GraphQL query against GitHub's API and return the results.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_EXECUTE_GRAPHQL_QUERY_USER_TITLE", "Execute GraphQL query"),
				ReadOnlyHint: ToBoolPtr(false),
			}),
			mcp.WithString("query",
				mcp.Required(),
				mcp.Description("The GraphQL query string to execute"),
			),
			mcp.WithObject("variables",
				mcp.Description("Variables for the GraphQL query (optional)"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			queryStr, err := RequiredParam[string](request, "query")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			variables, _ := OptionalParam[map[string]interface{}](request, "variables")
			if variables == nil {
				variables = make(map[string]interface{})
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Create a GraphQL request payload
			graphqlPayload := map[string]interface{}{
				"query":     queryStr,
				"variables": variables,
			}

			// Use the underlying HTTP client to make a raw GraphQL request
			req, err := client.NewRequest("POST", "graphql", graphqlPayload)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to create request: %v", err)), nil
			}

			// Execute the request
			var response map[string]interface{}
			_, err = client.Do(ctx, req, &response)

			result := map[string]interface{}{
				"query":     queryStr,
				"variables": variables,
			}

			if err != nil {
				// Query execution failed
				result["success"] = false
				result["error"] = err.Error()

				// Try to categorize the error
				errorStr := err.Error()
				switch {
				case strings.Contains(errorStr, "rate limit"):
					result["error_type"] = "rate_limit"
				case strings.Contains(errorStr, "unauthorized") || strings.Contains(errorStr, "authentication"):
					result["error_type"] = "authentication"
				case strings.Contains(errorStr, "permission") || strings.Contains(errorStr, "forbidden"):
					result["error_type"] = "permission"
				case strings.Contains(errorStr, "not found") || strings.Contains(errorStr, "Could not resolve") || strings.Contains(errorStr, "not exist"):
					result["error_type"] = "not_found"
				default:
					result["error_type"] = "execution_error"
				}
			} else {
				// Query executed successfully
				result["success"] = true
				result["data"] = response["data"]

				// Include any errors from the GraphQL response
				if errors, ok := response["errors"]; ok {
					result["graphql_errors"] = errors
				}
			}

			r, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal response: %w", err)
			}

			return mcp.NewToolResultText(string(r)), nil
		}
}
