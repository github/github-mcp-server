// Package servercard provides the GitHub MCP Server's MCP Server Card
// (SEP-2127) types and a public, no-auth HTTP handler that serves it.
//
// A Server Card is a static metadata document that describes a remote MCP
// server — its identity, repository, and HTTP transport — so clients can
// discover and connect to it before the protocol handshake. It is remote-only
// and deliberately does NOT enumerate primitives (tools, resources, prompts)
// or installable packages; those remain in the MCP Registry document
// (server.json) and runtime listing.
//
// See:
//   - https://github.com/modelcontextprotocol/experimental-ext-server-card
//   - https://github.com/modelcontextprotocol/modelcontextprotocol/pull/2127
package servercard

import "net/http"

const (
	// SchemaURL is the v1 Server Card JSON Schema URI that emitted cards
	// conform to. The schema is versioned by its `vN` path segment.
	SchemaURL = "https://static.modelcontextprotocol.io/schemas/v1/server-card.schema.json"

	// MediaType is the media type used to serve and request a Server Card.
	MediaType = "application/mcp-server-card+json"

	// Path is the suffix, relative to a server's streamable-HTTP URL, at which
	// MCP reserves the recommended Server Card location. A server hosted at
	// `https://host/mcp` therefore serves its card at `https://host/mcp/server-card`.
	Path = "/server-card"

	// DefaultRemoteURL is the streamable-HTTP endpoint of the hosted GitHub MCP
	// Server on github.com. The remote repository overrides this per environment.
	DefaultRemoteURL = "https://api.githubcopilot.com/mcp/"
)

// Identity fields reused from the MCP Registry document (server.json) so the
// Server Card and the registry entry describe the same server.
const (
	serverName        = "io.github.github/github-mcp-server"
	serverTitle       = "GitHub"
	serverDescription = "Connect AI assistants to GitHub - manage repos, issues, PRs, and workflows through natural language."
	repositoryURL     = "https://github.com/github/github-mcp-server"
	repositorySource  = "github"
	// repositoryID is the github.com repository ID for github/github-mcp-server.
	// It is stable across renames and changes if the repository is recreated.
	repositoryID = "942771284"
)

// ServerCard is a static metadata document describing a remote MCP server,
// suitable for pre-connection discovery. It mirrors the ServerCard interface in
// modelcontextprotocol/experimental-ext-server-card. Server Cards are
// remote-only and never carry installable packages.
type ServerCard struct {
	// Schema is the Server Card JSON Schema URI this document conforms to.
	Schema string `json:"$schema"`
	// Name is the server name in reverse-DNS format with exactly one slash.
	Name string `json:"name"`
	// Version is the server version, equivalent to Implementation.version.
	Version string `json:"version"`
	// Description is a short, human-readable explanation of server functionality.
	Description string `json:"description"`
	// Title is an optional human-readable display name.
	Title string `json:"title,omitempty"`
	// WebsiteURL optionally links to the server's homepage or documentation.
	WebsiteURL string `json:"websiteUrl,omitempty"`
	// Repository optionally describes the server's source code for inspection.
	Repository *Repository `json:"repository,omitempty"`
	// Icons optionally lists sized icons a client may render.
	Icons []Icon `json:"icons,omitempty"`
	// Remotes lists the HTTP-based endpoints for connecting to the server.
	Remotes []Remote `json:"remotes,omitempty"`
	// Meta carries vendor-specific metadata using reverse-DNS namespacing.
	Meta map[string]any `json:"_meta,omitempty"`
}

// Repository describes the MCP server's source code location.
type Repository struct {
	// URL is the repository URL for browsing source and cloning.
	URL string `json:"url"`
	// Source is the hosting service identifier (e.g. "github").
	Source string `json:"source"`
	// Subfolder is an optional clean relative path within a monorepo.
	Subfolder string `json:"subfolder,omitempty"`
	// ID is the optional repository identifier owned by the hosting service.
	ID string `json:"id,omitempty"`
}

// Remote describes a remote (HTTP-based) MCP server endpoint.
type Remote struct {
	// Type is the transport type ("streamable-http" or "sse").
	Type string `json:"type"`
	// URL is the endpoint URL template. Variables in {curly_braces} are
	// substituted from Variables before the client connects.
	URL string `json:"url"`
	// Headers describes HTTP headers required or accepted when connecting.
	Headers []KeyValueInput `json:"headers,omitempty"`
	// Variables defines values referenced by {curly_braces} in URL and headers.
	Variables map[string]Input `json:"variables,omitempty"`
	// SupportedProtocolVersions lists MCP protocol versions this endpoint serves.
	SupportedProtocolVersions []string `json:"supportedProtocolVersions,omitempty"`
}

// Input is a user-supplied or pre-set value for a remote URL variable or
// header value.
type Input struct {
	// Description is a human-readable explanation of the input.
	Description string `json:"description,omitempty"`
	// IsRequired indicates the input must be supplied to connect.
	IsRequired bool `json:"isRequired,omitempty"`
	// IsSecret indicates the value is sensitive and must be handled securely.
	IsSecret bool `json:"isSecret,omitempty"`
	// Format specifies the input format ("string", "number", "boolean", "filepath").
	Format string `json:"format,omitempty"`
	// Default is the default value for the input.
	Default string `json:"default,omitempty"`
	// Placeholder is example guidance shown during configuration.
	Placeholder string `json:"placeholder,omitempty"`
	// Value is a pre-set value that end users should not configure.
	Value string `json:"value,omitempty"`
	// Choices, when set, constrains the input to one of the listed values.
	Choices []string `json:"choices,omitempty"`
}

// KeyValueInput is a named Input used to describe an HTTP header.
type KeyValueInput struct {
	Input
	// Name is the header name.
	Name string `json:"name"`
	// Variables defines values referenced by {curly_braces} in Value.
	Variables map[string]Input `json:"variables,omitempty"`
}

// Icon is an optionally-sized icon a client may display.
type Icon struct {
	// Src is a URI (HTTP(S) or data:) pointing to an icon resource.
	Src string `json:"src"`
	// MimeType optionally overrides the source MIME type.
	MimeType string `json:"mimeType,omitempty"`
	// Sizes optionally lists sizes (e.g. "48x48" or "any") the icon supports.
	Sizes []string `json:"sizes,omitempty"`
	// Theme optionally indicates the theme ("light" or "dark") the icon suits.
	Theme string `json:"theme,omitempty"`
}

// Config controls how the GitHub MCP Server card is built and served.
type Config struct {
	// Version is advertised as the card's version and SHOULD match the
	// runtime serverInfo version. When empty, "0.0.0-dev" is used.
	Version string

	// RemoteURL is the absolute streamable-HTTP endpoint advertised in the
	// card's single remote. When empty, DefaultRemoteURL is used. The remote
	// repository supplies a per-environment URL here.
	RemoteURL string

	// RemoteURLFunc, when set, derives the streamable-HTTP remote URL from the
	// incoming request, taking precedence over RemoteURL whenever it returns a
	// non-empty value. This supports multi-tenant deployments (e.g. proxima)
	// where the absolute URL varies per request (e.g. from X-Forwarded-Host).
	//
	// It is consumed by the Handler when serving a card; NewServerCard ignores
	// it, since the card constructor is not request-aware.
	RemoteURLFunc func(*http.Request) string
}

// NewServerCard builds the GitHub MCP Server's Server Card from cfg.
func NewServerCard(cfg Config) *ServerCard {
	version := cfg.Version
	if version == "" {
		version = "0.0.0-dev"
	}

	remoteURL := cfg.RemoteURL
	if remoteURL == "" {
		remoteURL = DefaultRemoteURL
	}

	return &ServerCard{
		Schema:      SchemaURL,
		Name:        serverName,
		Version:     version,
		Description: serverDescription,
		Title:       serverTitle,
		WebsiteURL:  repositoryURL,
		Repository: &Repository{
			URL:    repositoryURL,
			Source: repositorySource,
			ID:     repositoryID,
		},
		Remotes: []Remote{
			{
				Type: "streamable-http",
				URL:  remoteURL,
				Headers: []KeyValueInput{
					{
						Input: Input{
							Description: "Authorization header with authentication token (PAT or App token)",
							IsRequired:  true,
							IsSecret:    true,
						},
						Name: "Authorization",
					},
				},
			},
		},
	}
}
