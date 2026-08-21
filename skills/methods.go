package skills

import (
	"context"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Method names defined by the io.modelcontextprotocol/skills extension.
const (
	// MethodSkillsList enumerates the skills a server serves.
	MethodSkillsList = "skills/list"
	// MethodSkillsGet returns the entry for a single skill by URI.
	MethodSkillsGet = "skills/get"
	// MethodDirectoryRead lists the direct children of a directory resource.
	// Gated behind the extension's directoryRead capability setting.
	MethodDirectoryRead = "resources/directory/read"
)

// listTTLMs is the freshness hint attached to skills/list results (SEP-2549
// list-caching attributes). Bundled skills are immutable for the lifetime of
// the server binary, so a long TTL is safe; it is a hint, not an integrity
// property — digest verification governs content regardless.
const listTTLMs = 3_600_000

// resultTypeComplete is the base protocol's multi-round-trip result marker
// (SEP-2322): results on 2026-07-28+ sessions declare themselves "complete".
// The SDK stamps its own result types automatically but not custom methods',
// so the extension's handlers set it themselves.
const (
	resultTypeComplete      = "complete"
	protocolVersion20260728 = "2026-07-28"
)

// completeResultType returns the resultType value for a session: "complete"
// when the session negotiated 2026-07-28 or later, empty (omitted) for older
// clients, for which the field does not exist.
func completeResultType(ss *mcp.ServerSession) string {
	if ss != nil {
		if p := ss.InitializeParams(); p != nil && p.ProtocolVersion < protocolVersion20260728 {
			return ""
		}
	}
	return resultTypeComplete
}

// ListSkillsParams is the skills/list request payload.
type ListSkillsParams struct {
	mcp.ParamsBase
	// Cursor is an opaque pagination token from a prior result's nextCursor.
	Cursor string `json:"cursor,omitempty"`
}

// ListSkillsResult is the skills/list result payload.
type ListSkillsResult struct {
	mcp.ResultBase
	mcp.Cacheable
	// ResultType is the base protocol's completion marker on 2026-07-28+
	// sessions; see completeResultType.
	ResultType string  `json:"resultType,omitempty"`
	Skills     []Entry `json:"skills"`
	// NextCursor, when present, indicates more entries are available.
	NextCursor string `json:"nextCursor,omitempty"`
}

// GetSkillParams is the skills/get request payload.
type GetSkillParams struct {
	mcp.ParamsBase
	// URI is the resource URI of the skill's SKILL.md.
	URI string `json:"uri"`
}

// GetSkillResult is the skills/get result payload. Skill is identical in
// shape and meaning to an entry of skills/list.
type GetSkillResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType,omitempty"`
	Skill      Entry  `json:"skill"`
}

// DirectoryReadParams is the resources/directory/read request payload.
type DirectoryReadParams struct {
	mcp.ParamsBase
	// URI is the directory resource's URI (no trailing slash).
	URI string `json:"uri"`
	// Cursor is an opaque pagination token from a prior result's nextCursor.
	Cursor string `json:"cursor,omitempty"`
}

// DirectoryReadResult is the resources/directory/read result payload: the
// resource metadata of the directory's direct children, exactly as
// resources/list would carry them, with subdirectories marked by
// mimeType "inode/directory".
type DirectoryReadResult struct {
	mcp.ResultBase
	ResultType string          `json:"resultType,omitempty"`
	Resources  []*mcp.Resource `json:"resources"`
	// NextCursor, when present, indicates more children are available.
	NextCursor string `json:"nextCursor,omitempty"`
}

// Publisher wires a Registry into an mcp.Server: it declares the extension
// capability, registers each skill file as a resource, and implements the
// extension's protocol methods. The optional Dynamic* hooks let the server
// answer for skills outside the registry (e.g. repo-hosted skills reachable
// only by URI), mirroring the SEP's allowance for unenumerable catalogs.
type Publisher struct {
	Registry *Registry

	// DynamicGet, if set, is consulted by skills/get for URIs not present in
	// the registry. Returning a nil Entry means the URI does not identify a
	// skill this server serves.
	DynamicGet func(ctx context.Context, uri string) (*Entry, error)

	// DynamicDirectoryRead, if set, is consulted by resources/directory/read
	// for URIs outside the registry's directory tree. Returning nil resources
	// with a nil error means the URI is not a directory this server serves.
	DynamicDirectoryRead func(ctx context.Context, uri string) ([]*mcp.Resource, error)
}

// DeclareCapability adds the skills extension, with directoryRead support, to
// the options' capabilities. Must run before mcp.NewServer, which captures
// capabilities at construction.
func (p *Publisher) DeclareCapability(opts *mcp.ServerOptions) {
	if opts.Capabilities == nil {
		opts.Capabilities = &mcp.ServerCapabilities{}
	}
	opts.Capabilities.AddExtension(ExtensionID, map[string]any{"directoryRead": true})
}

// Install registers every skill file as an MCP resource and the extension's
// methods on the server. Custom methods run through the server's receiving
// middleware chain like any standard method.
func (p *Publisher) Install(s *mcp.Server) error {
	for _, sk := range p.Registry.Skills() {
		for _, f := range sk.Files {
			s.AddResource(resourceMeta(sk, f), serveFile(sk.FileURI(f.Path), f))
		}
	}
	if err := mcp.AddReceivingCustomMethod(s, MethodSkillsList, p.listSkills); err != nil {
		return err
	}
	if err := mcp.AddReceivingCustomMethod(s, MethodSkillsGet, p.getSkill); err != nil {
		return err
	}
	return mcp.AddReceivingCustomMethod(s, MethodDirectoryRead, p.readDirectory)
}

func serveFile(uri string, f File) mcp.ResourceHandler {
	return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		contents := &mcp.ResourceContents{URI: uri, MIMEType: f.MIMEType}
		if IsText(f.Content) {
			contents.Text = string(f.Content)
		} else {
			contents.Blob = f.Content
		}
		return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}, nil
	}
}

// listSkills implements skills/list. The bundled catalog is small and static,
// so it always fits one page: any cursor is accepted and the full listing
// returned with no nextCursor, which terminates every conforming pagination
// loop.
func (p *Publisher) listSkills(_ context.Context, ss *mcp.ServerSession, _ *ListSkillsParams) (*ListSkillsResult, error) {
	res := &ListSkillsResult{Skills: p.Registry.Entries(), ResultType: completeResultType(ss)}
	res.TTLMs = listTTLMs
	res.CacheScope = "public"
	return res, nil
}

// getSkill implements skills/get. Per the SEP it answers for every skill the
// server serves — listed or not — and returns -32602 (the same code
// resources/read uses for unknown resources) otherwise.
func (p *Publisher) getSkill(ctx context.Context, ss *mcp.ServerSession, params *GetSkillParams) (*GetSkillResult, error) {
	if params == nil || params.URI == "" {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "missing required parameter: uri"}
	}
	if s, ok := p.Registry.Get(params.URI); ok {
		return &GetSkillResult{Skill: s.Entry(), ResultType: completeResultType(ss)}, nil
	}
	if p.DynamicGet != nil {
		entry, err := p.DynamicGet(ctx, params.URI)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			return &GetSkillResult{Skill: *entry, ResultType: completeResultType(ss)}, nil
		}
	}
	return nil, mcp.ResourceNotFoundError(params.URI)
}

// readDirectory implements resources/directory/read. Like listSkills, bundled
// directories are small enough to always fit one page.
func (p *Publisher) readDirectory(ctx context.Context, ss *mcp.ServerSession, params *DirectoryReadParams) (*DirectoryReadResult, error) {
	if params == nil || params.URI == "" {
		return nil, &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: "missing required parameter: uri"}
	}
	uri := strings.TrimSuffix(params.URI, "/")
	if children, ok := p.Registry.Directory(uri); ok {
		return &DirectoryReadResult{Resources: children, ResultType: completeResultType(ss)}, nil
	}
	if p.DynamicDirectoryRead != nil {
		children, err := p.DynamicDirectoryRead(ctx, uri)
		if err != nil {
			return nil, err
		}
		if children != nil {
			return &DirectoryReadResult{Resources: children, ResultType: completeResultType(ss)}, nil
		}
	}
	return nil, mcp.ResourceNotFoundError(params.URI)
}
