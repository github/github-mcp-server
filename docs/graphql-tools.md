# GraphQL Tools

This document describes the GraphQL tools added to the GitHub MCP server that provide direct access to GitHub's GraphQL API.

## Tools

### execute_graphql_query

Executes a GraphQL query against GitHub's API and returns the results.

#### Parameters

- `query` (required): The GraphQL query string to execute
- `variables` (optional): Variables for the GraphQL query as a JSON object

#### Response

Returns a JSON object with:

- `query`: The original query string
- `variables`: The variables passed to the query
- `success`: Boolean indicating if the query executed successfully
- `data`: The GraphQL response data (if successful)
- `error`: Error message if execution failed
- `error_type`: Type of execution error (rate_limit, authentication, permission, not_found, execution_error)
- `graphql_errors`: Any GraphQL-specific errors from the response

#### Example

```json
{
  "query": "query { viewer { login } }",
  "variables": {},
  "success": true,
  "data": {
    "viewer": {
      "login": "username"
    }
  }
}
```

## Implementation Details

### Execution

The execution tool uses GitHub's REST client to make raw HTTP requests to the GraphQL endpoint (`/graphql`), allowing for arbitrary GraphQL query execution while maintaining proper authentication and error handling.

### Error Handling

The tool provides comprehensive error categorization:

- **Syntax errors**: Malformed GraphQL syntax
- **Field errors**: References to non-existent fields
- **Type errors**: Type-related validation issues
- **Client errors**: Authentication or connectivity issues
- **Rate limit errors**: API rate limiting
- **Permission errors**: Access denied to resources
- **Not found errors**: Referenced resources don't exist

## Usage with MCP

This tool is part of the "graphql" toolset and can be enabled through the dynamic toolset system:

1. Enable the graphql toolset: `enable_toolset` with name "graphql"
2. Use `execute_graphql_query` to run queries and get results

## Testing

The tool includes comprehensive tests covering:

- Tool definition validation
- Required parameter checking
- Response format validation
- Variable handling
- Error categorization

Run tests with: `go test -v ./pkg/github -run GraphQL`
