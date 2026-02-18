package headers

const (
	// AuthorizationHeader is a standard HTTP Header.
	AuthorizationHeader = "Authorization"
	// ContentTypeHeader is a standard HTTP Header.
	ContentTypeHeader = "Content-Type"
	// AcceptHeader is a standard HTTP Header.
	AcceptHeader = "Accept"
	// UserAgentHeader is a standard HTTP Header.
	UserAgentHeader = "User-Agent"

	// ContentTypeJSON is the standard MIME type for JSON.
	ContentTypeJSON = "application/json"
	// ContentTypeEventStream is the standard MIME type for Event Streams.
	ContentTypeEventStream = "text/event-stream"

	// ForwardedForHeader is a standard HTTP Header used to forward the originating IP address of a client.
	ForwardedForHeader = "X-Forwarded-For"

	// RealIPHeader is a standard HTTP Header used to indicate the real IP address of the client.
	RealIPHeader = "X-Real-IP"

	// ForwardedHostHeader is a standard HTTP Header for preserving the original Host header when proxying.
	ForwardedHostHeader = "X-Forwarded-Host"
	// ForwardedProtoHeader is a standard HTTP Header for preserving the original protocol when proxying.
	ForwardedProtoHeader = "X-Forwarded-Proto"

	// RequestHmacHeader is used to authenticate requests to the Raw API.
	RequestHmacHeader = "Request-Hmac"

	// MCP-specific headers.

	// MCPReadOnlyHeader indicates whether the MCP is in read-only mode.
	MCPReadOnlyHeader = "X-MCP-Readonly"
	// MCPToolsetsHeader is a comma-separated list of MCP toolsets that the request is for.
	MCPToolsetsHeader = "X-MCP-Toolsets"
	// MCPToolsHeader is a comma-separated list of MCP tools that the request is for.
	MCPToolsHeader = "X-MCP-Tools"
	// MCPLockdownHeader indicates whether lockdown mode is enabled.
	MCPLockdownHeader = "X-MCP-Lockdown"
	// MCPInsidersHeader indicates whether insiders mode is enabled for early access features.
	MCPInsidersHeader = "X-MCP-Insiders"
	// MCPDisallowedToolsHeader is a comma-separated list of MCP tools that should be
	// disabled regardless of other settings or header values.
	MCPDisallowedToolsHeader = "X-MCP-Disallowed-Tools"
	// MCPFeaturesHeader is a comma-separated list of feature flags to enable.
	MCPFeaturesHeader = "X-MCP-Features"

	// GitHub-specific headers.

	// GraphQLFeaturesHeader is a comma-separated list of GraphQL feature flags to enable for GraphQL requests.
	GraphQLFeaturesHeader = "GraphQL-Features"
	// GitHubAPIVersionHeader is the header used to specify the GitHub API version.
	GitHubAPIVersionHeader = "X-GitHub-Api-Version"
)
