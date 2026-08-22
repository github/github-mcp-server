package inventory

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolVersionMultiRoundTrip is the first MCP protocol version that supports
// multi-round-trip input requests.
const ProtocolVersionMultiRoundTrip = "2026-07-28"

// ElicitationMode identifies a client-supported elicitation interaction mode.
type ElicitationMode string

const (
	// ElicitationModeForm collects structured user input through the client.
	ElicitationModeForm ElicitationMode = "form"
	// ElicitationModeURL directs the user to an external URL.
	ElicitationModeURL ElicitationMode = "url"
)

type toolAvailability struct {
	minimumProtocolVersion  string
	requiredElicitationMode ElicitationMode
}

func (st *ServerTool) availability() toolAvailability {
	return toolAvailability{
		minimumProtocolVersion:  st.MinimumProtocolVersion,
		requiredElicitationMode: st.RequiredElicitationMode,
	}
}

func (a toolAvailability) unrestricted() bool {
	return a.minimumProtocolVersion == "" && a.requiredElicitationMode == ""
}

func addToolAvailabilityMiddleware(server *mcp.Server, tools []ServerTool) {
	availabilityByName := make(map[string]toolAvailability)
	for _, tool := range tools {
		availability := tool.availability()
		if availability.unrestricted() {
			delete(availabilityByName, tool.Tool.Name)
		} else {
			// AddTool replaces an existing tool with the same name, so preserve
			// the availability metadata from the last registered definition too.
			availabilityByName[tool.Tool.Name] = availability
		}
	}
	if len(availabilityByName) == 0 {
		return
	}

	server.AddReceivingMiddleware(toolAvailabilityMiddleware(availabilityByName))
}

func toolAvailabilityMiddleware(availabilityByName map[string]toolAvailability) mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			req, ok := request.(*mcp.ListToolsRequest)
			if !ok {
				return next(ctx, method, request)
			}
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
				if toolAvailable(req.ProtocolVersion(), req.ClientCapabilities(), availabilityByName[tool.Name]) {
					tools = append(tools, tool)
				}
			}
			list.Tools = tools
			return list, nil
		}
	}
}

func (st *ServerTool) wrapAvailabilityCheck(next mcp.ToolHandler) mcp.ToolHandler {
	availability := st.availability()
	if availability.unrestricted() {
		return next
	}
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if toolAvailable(req.ProtocolVersion(), req.ClientCapabilities(), availability) {
			return next(ctx, req)
		}
		return toolUnavailableResult(st.Tool.Name, req, availability), nil
	}
}

func toolAvailable(protocolVersion string, capabilities *mcp.ClientCapabilities, availability toolAvailability) bool {
	return protocolVersionAllowed(protocolVersion, availability.minimumProtocolVersion) &&
		elicitationModeSupported(capabilities, availability.requiredElicitationMode)
}

func protocolVersionAllowed(protocolVersion, minimum string) bool {
	// MCP protocol versions use ISO dates, so lexical ordering is chronological.
	return minimum == "" || protocolVersion >= minimum
}

func elicitationModeSupported(capabilities *mcp.ClientCapabilities, requiredMode ElicitationMode) bool {
	if requiredMode == "" {
		return true
	}
	if capabilities == nil || capabilities.Elicitation == nil {
		return false
	}
	elicitation := capabilities.Elicitation
	switch requiredMode {
	case ElicitationModeForm:
		// An empty elicitation capability means form-only for compatibility.
		return elicitation.Form != nil || elicitation.URL == nil
	case ElicitationModeURL:
		return elicitation.URL != nil
	default:
		return false
	}
}

func toolUnavailableResult(name string, req *mcp.CallToolRequest, availability toolAvailability) *mcp.CallToolResult {
	message := fmt.Sprintf("Tool %q is unavailable for this client.", name)
	switch {
	case !protocolVersionAllowed(req.ProtocolVersion(), availability.minimumProtocolVersion):
		message = fmt.Sprintf(
			"Tool %q requires MCP protocol version %s or later.",
			name,
			availability.minimumProtocolVersion,
		)
	case !elicitationModeSupported(req.ClientCapabilities(), availability.requiredElicitationMode):
		message = fmt.Sprintf(
			"Tool %q requires client support for %s elicitation.",
			name,
			availability.requiredElicitationMode,
		)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
		IsError: true,
	}
}
