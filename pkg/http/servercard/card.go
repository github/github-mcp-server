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
	// It is stable across renames but changes if the repository is recreated.
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
	// Remotes lists the HTTP-based endpoints for connecting to the server.
	Remotes []Remote `json:"remotes,omitempty"`
}

// Repository describes the MCP server's source code location.
type Repository struct {
	// URL is the repository URL for browsing source and cloning.
	URL string `json:"url"`
	// Source is the hosting service identifier (e.g. "github").
	Source string `json:"source"`
	// ID is the optional repository identifier owned by the hosting service.
	ID string `json:"id,omitempty"`
}

// Remote describes a remote (HTTP-based) MCP server endpoint. Authentication is
// intentionally not described here: the hosted server advertises its auth
// requirements via OAuth protected-resource-metadata discovery, so duplicating
// them on the card would risk drift and cannot capture every accepted mode.
type Remote struct {
	// Type is the transport type ("streamable-http" or "sse").
	Type string `json:"type"`
	// URL is the endpoint URL.
	URL string `json:"url"`
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

	// supportedProtocolVersions is intentionally omitted: the go-sdk does not
	// export the versions it negotiates, so advertising a hand-maintained list
	// here would risk drifting from what the server actually serves.
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
			{Type: "streamable-http", URL: remoteURL},
		},
	}
}
