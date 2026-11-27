package github

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestGraphQLOperationSelection(t *testing.T) {
	body := "($id: ID!) { node(id: $id) { id } }"
	variables := map[string]any{"id": "example"}
	in := graphqlOperationInput{Operation: body, Variables: variables}
	read := graphqlPayload(in, false)
	require.Equal(t, "query MCPRequest "+body, read.Query)
	require.Equal(t, "MCPRequest", read.OperationName)
	require.Equal(t, variables, read.Variables)
	write := graphqlPayload(in, true)
	require.Equal(t, "mutation MCPRequest "+body, write.Query)
	require.Equal(t, read.OperationName, write.OperationName)

	// Only the server-selected read operation can run, even if the caller
	// includes an additional, differently named mutation in the document.
	in.Operation = "{ viewer { login } } mutation Other { __typename }"
	read = graphqlPayload(in, false)
	require.Equal(t, "MCPRequest", read.OperationName)
	require.Equal(t, "query MCPRequest "+in.Operation, read.Query)
}

func TestGraphQLOperationRegistration(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "discovery-test", Version: "1"}, nil)
	for _, write := range []bool{false, true} {
		tool := graphqlOperationTool(write)
		require.NotNil(t, tool.Tool.InputSchema)
		tool.RegisterFunc(server, nil)
	}
}
