package github

import (
	"context"
	"testing"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithOperationID_PreservesRequestIDAndAddsOperationID(t *testing.T) {
	t.Parallel()

	var capturedRequestID string
	var capturedOperationID string
	handler := withOperationID(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		var ok bool
		capturedRequestID, ok = ghcontext.RequestID(ctx)
		require.True(t, ok)

		capturedOperationID, ok = ghcontext.OperationID(ctx)
		require.True(t, ok)
		return nil, nil
	})

	_, err := handler(ghcontext.WithRequestID(context.Background(), "req_client"), "tools/call", nil)
	require.NoError(t, err)

	assert.Equal(t, "req_client", capturedRequestID)
	assert.Regexp(t, `^op_[0-9a-f]+$`, capturedOperationID)
}

func TestWithOperationID_GeneratesUniqueOperationIDs(t *testing.T) {
	t.Parallel()

	var operationIDs []string
	handler := withOperationID(func(ctx context.Context, _ string, _ mcp.Request) (mcp.Result, error) {
		operationID, ok := ghcontext.OperationID(ctx)
		require.True(t, ok)
		operationIDs = append(operationIDs, operationID)
		return nil, nil
	})

	_, err := handler(context.Background(), "tools/call", nil)
	require.NoError(t, err)
	_, err = handler(context.Background(), "tools/call", nil)
	require.NoError(t, err)

	require.Len(t, operationIDs, 2)
	assert.NotEqual(t, operationIDs[0], operationIDs[1])
}
