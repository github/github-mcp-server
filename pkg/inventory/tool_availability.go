package inventory

import (
	"context"
	"fmt"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ProtocolVersionMultiRoundTrip is the first MCP protocol version that supports
// multi-round-trip input requests.
const ProtocolVersionMultiRoundTrip = "2026-07-28"

// latestLegacyProtocolVersion is the newest protocol version the deprecated
// initialize handshake can negotiate.
const latestLegacyProtocolVersion = "2025-11-25"

// knownProtocolVersions lists the MCP protocol versions this server understands,
// oldest first. Availability is decided by comparing against these entries
// instead of comparing version strings lexically.
var knownProtocolVersions = []string{
	"2024-11-05",
	"2025-03-26",
	"2025-06-18",
	latestLegacyProtocolVersion,
	ProtocolVersionMultiRoundTrip,
}

// knownProtocolVersionRank returns the position of version in
// knownProtocolVersions, oldest first.
func knownProtocolVersionRank(version string) (int, bool) {
	for i, known := range knownProtocolVersions {
		if version == known {
			return i, true
		}
	}
	return 0, false
}

// availabilityProtocolVersionRank ranks a client-declared protocol version for
// the >= comparison. Versions the server does not know negotiate down to
// latestLegacyProtocolVersion, and an unset version ranks below every known one.
func availabilityProtocolVersionRank(version string) int {
	if version == "" {
		return -1
	}
	if rank, ok := knownProtocolVersionRank(version); ok {
		return rank
	}
	legacyRank, _ := knownProtocolVersionRank(latestLegacyProtocolVersion)
	return legacyRank
}

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

type toolFeatureDecision uint8

const (
	evaluateToolFeatureRule toolFeatureDecision = iota
	excludeToolBeforeFeatureRule
	includeToolWithoutFeatureRule
)

func (st *ServerTool) availability() toolAvailability {
	return toolAvailability{
		minimumProtocolVersion:  st.MinimumProtocolVersion,
		requiredElicitationMode: st.RequiredElicitationMode,
	}
}

func (a toolAvailability) unrestricted() bool {
	return a.minimumProtocolVersion == "" && a.requiredElicitationMode == ""
}

func featureDecisionForToolAvailability(ctx context.Context, availability toolAvailability) toolFeatureDecision {
	if availability.unrestricted() {
		return evaluateToolFeatureRule
	}
	info, ok := ghcontext.MCPMethod(ctx)
	if !ok || info == nil || (info.Method != MCPMethodToolsList && info.Method != MCPMethodToolsCall) {
		return evaluateToolFeatureRule
	}

	available, known := knownToolAvailability(info.ProtocolVersion, info.ClientCapabilities, availability)
	if !known || available {
		return evaluateToolFeatureRule
	}
	if info.Method == MCPMethodToolsCall {
		return includeToolWithoutFeatureRule
	}
	return excludeToolBeforeFeatureRule
}

func knownToolAvailability(protocolVersion string, capabilities *mcp.ClientCapabilities, availability toolAvailability) (bool, bool) {
	protocolKnown := availability.minimumProtocolVersion == "" || protocolVersion != ""
	if protocolKnown && !protocolVersionAllowed(protocolVersion, availability.minimumProtocolVersion) {
		return false, true
	}
	capabilitiesKnown := availability.requiredElicitationMode == "" || capabilities != nil
	if capabilitiesKnown && !elicitationModeSupported(capabilities, availability.requiredElicitationMode) {
		return false, true
	}
	if !protocolKnown || !capabilitiesKnown {
		return false, false
	}
	return true, true
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
			protocolVersion := protocolVersionForTools(requestMetaProtocolVersion(req.Params.Meta), req.ProtocolVersion())
			for _, tool := range list.Tools {
				if toolAvailable(protocolVersion, req.ClientCapabilities(), availabilityByName[tool.Name]) {
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
		protocolVersion := protocolVersionForTools(requestMetaProtocolVersion(req.Params.Meta), req.ProtocolVersion())
		if toolAvailable(protocolVersion, req.ClientCapabilities(), availability) {
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
	if minimum == "" {
		return true
	}
	minimumRank, ok := knownProtocolVersionRank(minimum)
	if !ok {
		// Keep comparing lexically when the requirement itself is not a
		// version this server knows about.
		return protocolVersion >= minimum
	}
	return availabilityProtocolVersionRank(protocolVersion) >= minimumRank
}

// protocolVersionForTools resolves the protocol version that gates tool
// availability for a request. The per-request _meta version is authoritative.
// Otherwise the request falls back to the session version established during
// initialize, which the server always negotiates down to
// latestLegacyProtocolVersion.
func protocolVersionForTools(metaVersion, sessionVersion string) string {
	if metaVersion != "" {
		return metaVersion
	}
	if sessionVersion == "" {
		return ""
	}
	legacyRank, _ := knownProtocolVersionRank(latestLegacyProtocolVersion)
	rank := availabilityProtocolVersionRank(sessionVersion)
	if rank > legacyRank {
		return latestLegacyProtocolVersion
	}
	if _, ok := knownProtocolVersionRank(sessionVersion); !ok {
		return latestLegacyProtocolVersion
	}
	return sessionVersion
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
	protocolVersion := protocolVersionForTools(requestMetaProtocolVersion(req.Params.Meta), req.ProtocolVersion())
	message := fmt.Sprintf("Tool %q is unavailable for this client.", name)
	switch {
	case !protocolVersionAllowed(protocolVersion, availability.minimumProtocolVersion):
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
