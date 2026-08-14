package inventory

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolVersionMultiRoundTrip is the first MCP protocol version that supports
// multi-round-trip input requests.
const ProtocolVersionMultiRoundTrip = "2026-07-28"

func addToolProtocolVersionMiddleware(server *mcp.Server, tools []ServerTool) {
	minimumVersions := make(map[string]string)
	for _, tool := range tools {
		minimum := tool.MinimumProtocolVersion
		if minimum == "" || minimum <= minimumVersions[tool.Tool.Name] {
			continue
		}
		minimumVersions[tool.Tool.Name] = minimum
	}
	if len(minimumVersions) == 0 {
		return
	}

	server.AddReceivingMiddleware(toolProtocolVersionMiddleware(minimumVersions))
}

func toolProtocolVersionMiddleware(minimumVersions map[string]string) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			switch req := request.(type) {
			case *mcp.CallToolRequest:
				if req.Params != nil {
					if minimum := minimumVersions[req.Params.Name]; !protocolVersionAllowed(req.ProtocolVersion(), minimum) {
						return &mcp.CallToolResult{
							Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
								"Tool %q requires MCP protocol version %s or later.",
								req.Params.Name,
								minimum,
							)}},
							IsError: true,
						}, nil
					}
				}
			case *mcp.ListToolsRequest:
				result, err := next(ctx, method, request)
				if err != nil {
					return nil, err
				}
				list, ok := result.(*mcp.ListToolsResult)
				if !ok {
					return result, nil
				}

				tools := make([]*mcp.Tool, 0, len(list.Tools))
				for _, tool := range list.Tools {
					if protocolVersionAllowed(req.ProtocolVersion(), minimumVersions[tool.Name]) {
						tools = append(tools, tool)
					}
				}
				list.Tools = tools
				return list, nil
			}

			return next(ctx, method, request)
		}
	}
}

func protocolVersionAllowed(protocolVersion, minimum string) bool {
	return minimum == "" || protocolVersion >= minimum
}
