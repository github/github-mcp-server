package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	ghcontext "github.com/github/github-mcp-server/pkg/context"
)

// mcpJSONRPCRequest represents the structure of an MCP JSON-RPC request.
// We only parse the fields needed for routing and optimization.
type mcpJSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  struct {
		// For tools/call
		Name      string          `json:"name,omitempty"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
		// For prompts/get
		// Name is shared with tools/call
		// For resources/read
		URI string `json:"uri,omitempty"`
	} `json:"params"`
}

// WithMCPParse creates a middleware that parses MCP JSON-RPC requests early in the
// request lifecycle and stores the parsed information in the request context.
// This enables:
//   - Registry filtering via ForMCPRequest (only register needed tools/resources/prompts)
//   - Avoiding duplicate JSON parsing in downstream middlewares
//   - Access to owner/repo for secret-scanning middleware
//
// The middleware reads the request body, parses it, restores the body for downstream
// handlers, and stores the parsed MCPMethodInfo in the request context.
func WithMCPParse() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Skip health check endpoints
			if r.URL.Path == "/_ping" {
				next.ServeHTTP(w, r)
				return
			}

			// Only parse POST requests (MCP uses JSON-RPC over POST)
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			// Read the request body
			body, err := io.ReadAll(r.Body)
			if err != nil {
				// Log but continue - don't block requests on parse errors
				next.ServeHTTP(w, r)
				return
			}

			// Restore the body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Skip empty bodies
			if len(body) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			// Parse the JSON-RPC request
			var mcpReq mcpJSONRPCRequest
			err = json.Unmarshal(body, &mcpReq)
			if err != nil {
				// Log but continue - could be a non-MCP request or malformed JSON
				next.ServeHTTP(w, r)
				return
			}

			// Skip if not a valid JSON-RPC 2.0 request
			if mcpReq.JSONRPC != "2.0" || mcpReq.Method == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Build the MCPMethodInfo
			methodInfo := &ghcontext.MCPMethodInfo{
				Method: mcpReq.Method,
			}

			// Extract item name based on method type

			switch mcpReq.Method {
			case "tools/call":
				methodInfo.ItemName = mcpReq.Params.Name
				// Parse arguments if present
				if len(mcpReq.Params.Arguments) > 0 {
					var args map[string]any
					err := json.Unmarshal(mcpReq.Params.Arguments, &args)
					if err == nil {
						methodInfo.Arguments = args
						// Extract owner and repo if present
						if owner, ok := args["owner"].(string); ok {
							methodInfo.Owner = owner
						}
						if repo, ok := args["repo"].(string); ok {
							methodInfo.Repo = repo
						}
					}
				}
			case "prompts/get":
				methodInfo.ItemName = mcpReq.Params.Name
			case "resources/read":
				methodInfo.ItemName = mcpReq.Params.URI
			default:
				// Whatever
			}

			// Store the parsed info in context
			ctx = ghcontext.WithMCPMethodInfo(ctx, methodInfo)

			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}
