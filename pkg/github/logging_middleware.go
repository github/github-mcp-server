package github

import (
	"context"
	"log/slog"
	"time"

	"github.com/github/github-mcp-server/pkg/observability"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPMethodCallTool is the JSON-RPC method name for MCP tool invocations.
// The SDK keeps its equivalent constant unexported, so we mirror it here.
const MCPMethodCallTool = "tools/call"

// ToolLoggingMiddleware returns an MCP middleware that uniformly logs every
// tool invocation with its name, duration, and outcome, and exposes a
// request-scoped *slog.Logger via observability.ContextWithLogger so tool
// handlers can retrieve an enriched logger from deps.Logger(ctx).
//
// Logging policy:
//   - tools/call success: logged at Debug with tool name and duration.
//   - tools/call failure (error return or IsError result): logged at Error
//     with tool name, duration, and the error when present.
//   - Non-tool methods pass through without emitting any log line; the
//     middleware only attaches an enriched logger so downstream code can
//     still benefit from the method tag if it chooses to log.
//
// The base logger comes from ToolDependencies on the context (populated by
// InjectDepsMiddleware), so this middleware must be registered AFTER
// InjectDepsMiddleware in the receiving middleware chain.
func ToolLoggingMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			deps, ok := DepsFromContext(ctx)
			if !ok {
				// Deps not injected yet; nothing we can do but pass through.
				return next(ctx, method, req)
			}

			base := deps.Logger(ctx)
			if base == nil {
				return next(ctx, method, req)
			}

			logger := base.With(slog.String("mcp.method", method))
			toolName := toolNameFromRequest(method, req)
			if toolName != "" {
				logger = logger.With(slog.String("mcp.tool", toolName))
			}

			ctx = observability.ContextWithLogger(ctx, logger)

			// Only time+log for tool calls. Other methods (initialize,
			// resources/list, etc.) are infrastructure chatter we leave
			// to the SDK unless a handler chooses to log explicitly.
			if method != MCPMethodCallTool {
				return next(ctx, method, req)
			}

			start := time.Now()
			result, err := next(ctx, method, req)
			duration := time.Since(start)

			switch {
			case err != nil:
				logger.LogAttrs(ctx, slog.LevelError, "tool call failed",
					slog.Duration("duration", duration),
					slog.String("error", err.Error()),
				)
			case isErrorResult(result):
				logger.LogAttrs(ctx, slog.LevelError, "tool call returned error result",
					slog.Duration("duration", duration),
				)
			default:
				logger.LogAttrs(ctx, slog.LevelDebug, "tool call succeeded",
					slog.Duration("duration", duration),
				)
			}

			return result, err
		}
	}
}

// toolNameFromRequest extracts the tool name from a tools/call request.
// Returns "" for other methods or when the name cannot be determined.
func toolNameFromRequest(method string, req mcp.Request) string {
	if method != MCPMethodCallTool || req == nil {
		return ""
	}
	switch p := req.GetParams().(type) {
	case *mcp.CallToolParams:
		if p != nil {
			return p.Name
		}
	case *mcp.CallToolParamsRaw:
		if p != nil {
			return p.Name
		}
	}
	return ""
}

// isErrorResult reports whether the MCP result represents a tool-reported
// error (CallToolResult.IsError == true). A returned Go error is handled
// separately by the caller.
func isErrorResult(r mcp.Result) bool {
	if r == nil {
		return false
	}
	if ctr, ok := r.(*mcp.CallToolResult); ok && ctr != nil {
		return ctr.IsError
	}
	return false
}
