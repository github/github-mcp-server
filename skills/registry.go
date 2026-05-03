package skills

import (
	"context"
	"encoding/json"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Well-known identifiers from the skills-over-MCP SEP (SEP-2133) and the
// Agent Skills discovery index schema (agentskills.io).
const (
	// IndexURI is the well-known URI for the per-server discovery index.
	IndexURI = "skill://index.json"
	// ExtensionKey is the MCP capability extension identifier that a server
	// MUST declare when it publishes skill:// resources.
	ExtensionKey = "io.modelcontextprotocol/skills"
	// IndexSchema is the $schema value servers MUST emit in their index.
	IndexSchema = "https://schemas.agentskills.io/discovery/0.2.0/schema.json"
)

// Bundled describes a single server-bundled Agent Skill — a SKILL.md the
// server ships in its binary and serves at a stable skill:// URI.
type Bundled struct {
	// Name is the skill name. Must match the SKILL.md frontmatter `name`
	// and the final segment of the skill-path in the URI.
	Name string
	// Description is the text shown to the agent in the discovery index.
	// Should describe both what the skill does and when to use it.
	Description string
	// Content is the SKILL.md body (typically a //go:embed string).
	Content string
	// Icons, if non-empty, are attached to the SKILL.md MCP resource so
	// hosts that render icons in their resource list can show one.
	Icons []mcp.Icon
	// Enabled, if set, is called to determine whether this skill should
	// be published on the current server instance. Leave nil for "always
	// publish". Useful for gating on a toolset, feature flag, or request
	// context in per-request server builds.
	Enabled func() bool
}

// URI returns the skill's canonical SKILL.md URI: skill://github/<name>/SKILL.md.
// The "github/" segment is the SEP's organizational prefix for this server's
// bundled skills; the final path segment is the skill name.
func (b Bundled) URI() string { return "skill://github/" + b.Name + "/SKILL.md" }

func (b Bundled) enabled() bool { return b.Enabled == nil || b.Enabled() }

// Registry is the set of bundled skills a server publishes. Build one at
// server-construction time with New().Add(...).Add(...); then call
// DeclareCapability before mcp.NewServer and Install after.
type Registry struct {
	entries []Bundled
}

// New returns an empty registry.
func New() *Registry { return &Registry{} }

// Add appends a bundled skill and returns the registry for chaining.
func (r *Registry) Add(b Bundled) *Registry {
	r.entries = append(r.entries, b)
	return r
}

// Enabled returns the subset of entries currently enabled.
func (r *Registry) Enabled() []Bundled {
	var out []Bundled
	for _, e := range r.entries {
		if e.enabled() {
			out = append(out, e)
		}
	}
	return out
}

// DeclareCapability adds the skills-over-MCP extension to the provided
// ServerOptions.Capabilities if any entry is currently enabled. Must be
// called BEFORE mcp.NewServer since capabilities are captured at
// construction.
func (r *Registry) DeclareCapability(opts *mcp.ServerOptions) {
	if opts == nil || len(r.Enabled()) == 0 {
		return
	}
	if opts.Capabilities == nil {
		opts.Capabilities = &mcp.ServerCapabilities{}
	}
	opts.Capabilities.AddExtension(ExtensionKey, nil)
}

// Install registers each enabled skill's SKILL.md as an MCP resource and
// publishes the skill://index.json discovery document.
func (r *Registry) Install(s *mcp.Server) {
	enabled := r.Enabled()
	if len(enabled) == 0 {
		return
	}

	for _, e := range enabled {
		e := e
		s.AddResource(
			&mcp.Resource{
				URI:         e.URI(),
				Name:        e.Name + "_skill",
				Description: e.Description,
				MIMEType:    "text/markdown",
				Icons:       e.Icons,
			},
			func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
				return &mcp.ReadResourceResult{
					Contents: []*mcp.ResourceContents{{
						URI:      e.URI(),
						MIMEType: "text/markdown",
						Text:     e.Content,
					}},
				}, nil
			},
		)
	}

	indexJSON := buildIndex(enabled)
	s.AddResource(
		&mcp.Resource{
			URI:         IndexURI,
			Name:        "skills_index",
			Description: "Agent Skill discovery index for this server.",
			MIMEType:    "application/json",
		},
		func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					URI:      IndexURI,
					MIMEType: "application/json",
					Text:     indexJSON,
				}},
			}, nil
		},
	)
}

// IndexEntry matches the agentskills.io discovery schema, with MCP-specific
// fields: `url` holds the MCP resource URI; `digest` is omitted because
// integrity is handled by the authenticated MCP connection.
type IndexEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// IndexDoc is the top-level shape of skill://index.json.
type IndexDoc struct {
	Schema string       `json:"$schema"`
	Skills []IndexEntry `json:"skills"`
}

func buildIndex(entries []Bundled) string {
	doc := IndexDoc{Schema: IndexSchema, Skills: make([]IndexEntry, len(entries))}
	for i, e := range entries {
		doc.Skills[i] = IndexEntry{Name: e.Name, Type: "skill-md", Description: e.Description, URL: e.URI()}
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic("skills: failed to marshal index: " + err.Error())
	}
	return string(b)
}
