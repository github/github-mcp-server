package github

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ownerContextKey is the context key for the repository owner.
type ownerContextKey struct{}

// ContextWithOwner returns a new context with the owner stored in it.
func ContextWithOwner(ctx context.Context, owner string) context.Context {
	return context.WithValue(ctx, ownerContextKey{}, owner)
}

// OwnerFromContext retrieves the owner from the context.
// Returns empty string if not present.
func OwnerFromContext(ctx context.Context) string {
	owner, _ := ctx.Value(ownerContextKey{}).(string)
	return owner
}

// OwnerExtractMiddleware creates MCP middleware that extracts the "owner" parameter
// from tools/call requests and stores it in context. This allows MultiOrgDeps to
// route API calls to the correct org's GitHub App installation.
//
// For non-tools/call methods (resources, prompts, etc.), the request passes through unchanged.
// For tools/call requests without an "owner" parameter, the request passes through with no
// owner in context (MultiOrgDeps will use the default installation).
func OwnerExtractMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			// CallToolRequest = ServerRequest[*CallToolParamsRaw]
			// Params.Arguments is json.RawMessage, not map[string]any
			toolReq, ok := req.(*mcp.CallToolRequest)
			if !ok {
				return next(ctx, method, req)
			}

			var args map[string]any
			if err := json.Unmarshal(toolReq.Params.Arguments, &args); err != nil {
				// Can't parse args — pass through without owner
				return next(ctx, method, req)
			}

			if owner, ok := args["owner"].(string); ok && owner != "" {
				ctx = ContextWithOwner(ctx, owner)
			}

			return next(ctx, method, req)
		}
	}
}
