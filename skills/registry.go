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

// BundledTemplate describes a parameterized Agent Skill *family* —
// a `mcp-resource-template` entry per SEP-2640's discovery schema. Use this
// when the server exposes skills via an RFC 6570 MCP resource template (e.g.
// repo-discovered skills like `skill://{owner}/{repo}/{skill_name}/SKILL.md`)
// rather than as a single fixed `skill-md` resource.
//
// The actual MCP resource template (and its handler) must be registered
// elsewhere — typically through the same inventory that registers the
// server's other resource templates. This type only carries the metadata
// needed to advertise the family in `skill://index.json`.
type BundledTemplate struct {
	// Description is the text shown to the agent in the discovery index.
	// Should explain what the parameterized skill family covers.
	Description string
	// URL is the canonical discovery URL with RFC 6570 placeholders intact,
	// e.g. `skill://{owner}/{repo}/{skill_name}/SKILL.md`. By SEP convention
	// the URL anchors on `SKILL.md` so hosts know where to start reading;
	// per-file resolution then follows by extending the URI suffix.
	URL string
	// Enabled, if set, is called to determine whether this template should
	// be advertised on the current server instance. Leave nil for "always
	// publish".
	Enabled func() bool
}

func (t BundledTemplate) enabled() bool { return t.Enabled == nil || t.Enabled() }

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
	entries   []Bundled
	templates []BundledTemplate
}

// New returns an empty registry.
func New() *Registry { return &Registry{} }

// Add appends a bundled skill and returns the registry for chaining.
func (r *Registry) Add(b Bundled) *Registry {
	r.entries = append(r.entries, b)
	return r
}

// AddTemplate appends a parameterized skill template entry (advertised in
// the discovery index as `type: "mcp-resource-template"`) and returns the
// registry for chaining. The corresponding MCP resource template + handler
// must be registered separately (typically via the server's inventory).
func (r *Registry) AddTemplate(t BundledTemplate) *Registry {
	r.templates = append(r.templates, t)
	return r
}

// Enabled returns the subset of bundled-skill entries currently enabled.
func (r *Registry) Enabled() []Bundled {
	var out []Bundled
	for _, e := range r.entries {
		if e.enabled() {
			out = append(out, e)
		}
	}
	return out
}

// EnabledTemplates returns the subset of template entries currently enabled.
func (r *Registry) EnabledTemplates() []BundledTemplate {
	var out []BundledTemplate
	for _, t := range r.templates {
		if t.enabled() {
			out = append(out, t)
		}
	}
	return out
}

// hasAnyEnabled returns true when at least one bundled skill or template
// entry is enabled and would appear in the discovery index.
func (r *Registry) hasAnyEnabled() bool {
	return len(r.Enabled()) > 0 || len(r.EnabledTemplates()) > 0
}

// DeclareCapability adds the skills-over-MCP extension to the provided
// ServerOptions.Capabilities if any entry (skill or template) is currently
// enabled. Must be called BEFORE mcp.NewServer since capabilities are
// captured at construction.
func (r *Registry) DeclareCapability(opts *mcp.ServerOptions) {
	if opts == nil || !r.hasAnyEnabled() {
		return
	}
	if opts.Capabilities == nil {
		opts.Capabilities = &mcp.ServerCapabilities{}
	}
	opts.Capabilities.AddExtension(ExtensionKey, nil)
}

// Install registers each enabled skill's SKILL.md as an MCP resource and
// publishes the skill://index.json discovery document. Template entries
// don't get resource handlers installed here — only their metadata in the
// index. The corresponding MCP resource template handlers must be wired
// through the server's regular resource-template registration path.
func (r *Registry) Install(s *mcp.Server) {
	enabled := r.Enabled()
	templates := r.EnabledTemplates()
	if len(enabled) == 0 && len(templates) == 0 {
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

	indexJSON := buildIndex(enabled, templates)
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
// fields: `url` holds the MCP resource URI (or RFC 6570 template); `digest`
// is omitted because integrity is handled by the authenticated MCP
// connection. `name` is omitted for `mcp-resource-template` entries since
// the SEP doesn't require a stable name for parameterized families.
type IndexEntry struct {
	Name        string `json:"name,omitempty"`
	Type        string `json:"type"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// IndexDoc is the top-level shape of skill://index.json.
type IndexDoc struct {
	Schema string       `json:"$schema"`
	Skills []IndexEntry `json:"skills"`
}

func buildIndex(entries []Bundled, templates []BundledTemplate) string {
	doc := IndexDoc{Schema: IndexSchema, Skills: make([]IndexEntry, 0, len(entries)+len(templates))}
	for _, e := range entries {
		doc.Skills = append(doc.Skills, IndexEntry{Name: e.Name, Type: "skill-md", Description: e.Description, URL: e.URI()})
	}
	for _, t := range templates {
		doc.Skills = append(doc.Skills, IndexEntry{Type: "mcp-resource-template", Description: t.Description, URL: t.URL})
	}
	b, err := json.Marshal(doc)
	if err != nil {
		panic("skills: failed to marshal index: " + err.Error())
	}
	return string(b)
}
