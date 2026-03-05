package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureHandler is a test MethodHandler that captures the context it was called with.
type captureHandler struct {
	capturedCtx context.Context
}

func (c *captureHandler) handle(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
	c.capturedCtx = ctx
	return nil, nil
}

// makeCallToolRequest builds a *mcp.CallToolRequest with the given JSON arguments.
func makeCallToolRequest(t *testing.T, args map[string]any) *mcp.CallToolRequest {
	t.Helper()
	raw, err := json.Marshal(args)
	require.NoError(t, err)
	return &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test_tool",
			Arguments: json.RawMessage(raw),
		},
	}
}

func TestOwnerExtractMiddleware_WithOwner(t *testing.T) {
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"owner": "my-org", "repo": "my-repo"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "my-org", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_WithoutOwner(t *testing.T) {
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"repo": "my-repo"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_NonToolsCall(t *testing.T) {
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	// Use a non-tools/call method; req can be nil since we won't inspect it
	_, err := handler(context.Background(), "resources/list", nil)
	require.NoError(t, err)

	// Context should have no owner
	assert.Equal(t, "", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_InvalidJSON(t *testing.T) {
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	// Construct a request with invalid JSON arguments
	req := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Name:      "test_tool",
			Arguments: json.RawMessage(`{invalid json`),
		},
	}
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	// Should pass through without error and without owner
	assert.Equal(t, "", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerFromContext_FreshContext(t *testing.T) {
	owner := OwnerFromContext(context.Background())
	assert.Equal(t, "", owner)
}

func TestContextWithOwner_RoundTrip(t *testing.T) {
	ctx := ContextWithOwner(context.Background(), "block-xyz")
	assert.Equal(t, "block-xyz", OwnerFromContext(ctx))
}

func TestOwnerExtractMiddleware_WithOrgParam(t *testing.T) {
	// Tools like get_team_members and get_teams use "org" instead of "owner".
	// The middleware should fall back to "org" when "owner" is absent.
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"org": "my-org"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "my-org", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_OwnerTakesPrecedenceOverOrg(t *testing.T) {
	// When both "owner" and "org" are present, "owner" wins.
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"owner": "owner-val", "org": "org-val"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "owner-val", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_WithOrganizationParam(t *testing.T) {
	// create_repository and fork_repository use "organization" instead of "owner"/"org".
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"organization": "target-org", "name": "new-repo"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "target-org", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_OwnerTakesPrecedenceOverOrganization(t *testing.T) {
	// "owner" should take precedence over "organization".
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"owner": "owner-val", "organization": "org-val"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "owner-val", OwnerFromContext(capture.capturedCtx))
}

func TestOwnerExtractMiddleware_OrgTakesPrecedenceOverOrganization(t *testing.T) {
	// "org" should take precedence over "organization".
	capture := &captureHandler{}
	middleware := OwnerExtractMiddleware()
	handler := middleware(capture.handle)

	req := makeCallToolRequest(t, map[string]any{"org": "org-val", "organization": "organization-val"})
	_, err := handler(context.Background(), "tools/call", req)
	require.NoError(t, err)

	assert.Equal(t, "org-val", OwnerFromContext(capture.capturedCtx))
}
