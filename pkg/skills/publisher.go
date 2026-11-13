package skills

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ListTTL is the freshness hint published with a skills listing, in
// milliseconds. It says only how long a listing may be reused before
// re-fetching; it makes no claim about content, which digest verification
// governs regardless.
const ListTTL = 60_000

// Publisher serves the skills extension for one Source. A source must also
// implement ContentSource for Install to register the files its entries name.
//
// Declaring the extension commits a server to both skills/list and skills/get,
// so DeclareCapability and Install belong together: declare the capability on
// the options used to build the server, then install the methods on the server
// itself.
type Publisher struct {
	source Source
	// PageSize bounds how many entries one page of skills/list carries. Pages
	// are sliced by whole entries, so a skill's resource manifest is never
	// split across pages.
	PageSize int
}

// NewPublisher returns a publisher for a source.
func NewPublisher(source Source) *Publisher {
	return &Publisher{source: source, PageSize: DefaultPageSize}
}

// SupportsDirectoryRead reports whether the source can genuinely enumerate
// directories. The capability is declared only when it can, because a client
// that sees directoryRead: true is entitled to call the method for every
// directory in the namespaces the server serves.
func (p *Publisher) SupportsDirectoryRead() bool {
	_, ok := p.source.(DirectorySource)
	return ok
}

// DeclareCapability adds the extension to a server's declared capabilities.
//
// The settings object carries directoryRead only when the source implements it;
// an empty object declares the extension with no optional features, which is
// the correct declaration for a source that serves skills but cannot enumerate
// directories.
func (p *Publisher) DeclareCapability(caps *mcp.ServerCapabilities) {
	if caps == nil {
		return
	}
	settings := map[string]any{}
	if caps.Resources == nil {
		caps.Resources = &mcp.ResourceCapabilities{}
	}
	if p.SupportsDirectoryRead() {
		settings[SettingDirectoryRead] = true
	}
	if caps.Extensions == nil {
		caps.Extensions = map[string]any{}
	}
	caps.Extensions[ExtensionID] = settings
}

// Install registers the extension's methods on a server.
//
// skills/list and skills/get are always registered, because declaring the
// extension commits the server to both. resources/directory/read is registered
// only when the source implements DirectorySource, matching what
// DeclareCapability advertised. Installation requires a ContentSource and
// validates every listed manifest against its files before registering methods.
func (p *Publisher) Install(s *mcp.Server) error {
	content, ok := p.source.(ContentSource)
	if !ok {
		return fmt.Errorf("installing skills publisher requires a ContentSource")
	}
	if err := installResources(s, content, p.source); err != nil {
		return err
	}
	if err := mcp.AddReceivingCustomMethod(s, MethodListSkills, p.handleList); err != nil {
		return fmt.Errorf("registering %s: %w", MethodListSkills, err)
	}
	if err := mcp.AddReceivingCustomMethod(s, MethodGetSkill, p.handleGet); err != nil {
		return fmt.Errorf("registering %s: %w", MethodGetSkill, err)
	}
	if p.SupportsDirectoryRead() {
		if err := mcp.AddReceivingCustomMethod(s, MethodReadDirectory, p.handleReadDirectory); err != nil {
			return fmt.Errorf("registering %s: %w", MethodReadDirectory, err)
		}
	}
	return nil
}

// installResources registers each skill file as an ordinary MCP resource, so
// that skill content is read through the standard resources/read method. The
// extension defines no skill-specific read semantics, and reading a SKILL.md
// this way does not activate the skill.
func installResources(s *mcp.Server, source ContentSource, entriesSource Source) error {
	files, err := source.Files(context.Background())
	if err != nil {
		return fmt.Errorf("enumerating skill files: %w", err)
	}
	available := make(map[string]ResourceFile, len(files))
	for _, file := range files {
		available[file.URI] = file
	}
	entries, err := entriesSource.List(context.Background())
	if err != nil {
		return fmt.Errorf("validating skill content: listing skills: %w", err)
	}
	for _, entry := range entries {
		if err := entry.Validate(); err != nil {
			return fmt.Errorf("validating skill %q: %w", entry.URI, err)
		}
		if _, ok := available[entry.URI]; !ok {
			return fmt.Errorf("skill %q references resource %q, but the content source does not provide it", entry.URI, entry.URI)
		}
		if !entry.Dynamic {
			for _, resource := range entry.Resources {
				file, ok := available[resource.URI]
				if !ok {
					return fmt.Errorf("skill %q references resource %q, but the content source does not provide it", entry.URI, resource.URI)
				}
				if err := Verify(file.File.Content, resource); err != nil {
					return fmt.Errorf("validating skill %q content: %w", entry.URI, err)
				}
			}
		}
	}
	for _, f := range files {
		s.AddResource(&mcp.Resource{
			URI:         f.URI,
			Name:        f.Name,
			Description: f.Description,
			MIMEType:    f.File.MediaType(),
			Size:        int64(len(f.File.Content)),
		}, resourceHandler(f))
	}
	return nil
}

// resourceHandler serves one file's bytes. Text is sent as text; anything else
// is sent as a blob, which the SDK base64-encodes.
func resourceHandler(f ResourceFile) mcp.ResourceHandler {
	return func(_ context.Context, _ *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		contents := &mcp.ResourceContents{URI: f.URI, MIMEType: f.File.MediaType()}
		if IsText(f.File.Content) {
			contents.Text = string(f.File.Content)
		} else {
			contents.Blob = f.File.Content
		}
		result := &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{contents}}
		result.CacheScope = "private"
		return result, nil
	}
}

func (p *Publisher) handleList(ctx context.Context, ss *mcp.ServerSession, params *ListSkillsParams) (*ListSkillsResult, error) {
	version := NegotiatedProtocolVersion(ctx, params, ss)

	entries, err := p.source.List(ctx)
	if err != nil {
		return nil, err
	}
	cursor := ""
	if params != nil {
		cursor = params.Cursor
	}
	page, next, err := paginate(entries, cursor, p.PageSize)
	if err != nil {
		return nil, invalidParams(err)
	}
	if page == nil {
		page = []Entry{}
	}

	result := &ListSkillsResult{
		ResultType: resultTypeFor(version),
		Skills:     page,
		NextCursor: next,
	}
	// The list-caching attributes are defined from 2026-07-28 onwards; sending
	// them to an older client would be sending fields its protocol version
	// does not define.
	if SupportsRequestScopedContext(version) {
		ttl := ListTTL
		result.TTLMs = &ttl
		result.CacheScope = "private"
	}
	return result, nil
}

func (p *Publisher) handleGet(ctx context.Context, ss *mcp.ServerSession, params *GetSkillParams) (*GetSkillResult, error) {
	version := NegotiatedProtocolVersion(ctx, params, ss)
	if params == nil || params.URI == "" {
		return nil, invalidParams(fmt.Errorf("uri is required"))
	}

	entry, ok, err := p.source.Get(ctx, params.URI)
	if err != nil {
		return nil, err
	}
	if !ok {
		// An unknown skill is invalid params, not a resource-not-found error:
		// the extension specifies -32602 here, the same code resources/read
		// uses for unknown resources.
		return nil, invalidParams(fmt.Errorf("unknown skill %q", params.URI))
	}
	result := &GetSkillResult{ResultType: resultTypeFor(version), Skill: entry}
	if SupportsRequestScopedContext(version) {
		ttl := ListTTL
		result.TTLMs = &ttl
		result.CacheScope = "private"
	}
	return result, nil
}

func (p *Publisher) handleReadDirectory(ctx context.Context, ss *mcp.ServerSession, params *ReadDirectoryParams) (*ReadDirectoryResult, error) {
	version := NegotiatedProtocolVersion(ctx, params, ss)
	if params == nil || params.URI == "" {
		return nil, invalidParams(fmt.Errorf("uri is required"))
	}
	source, ok := p.source.(DirectorySource)
	if !ok {
		return nil, invalidParams(fmt.Errorf("directory reads are not supported"))
	}

	children, found, err := source.ReadDirectory(ctx, params.URI)
	if err != nil {
		return nil, err
	}
	if !found {
		// Both a URI that does not exist and one that exists but is not a
		// directory are invalid params. An existing but empty directory is not
		// an error: it yields an empty resources array.
		return nil, invalidParams(fmt.Errorf("%q is not a directory resource", params.URI))
	}

	children = slices.Clone(children)
	sort.Slice(children, func(i, j int) bool { return children[i].URI < children[j].URI })
	page, next, err := paginate(children, params.Cursor, p.PageSize)
	if err != nil {
		return nil, invalidParams(err)
	}

	resources := make([]*mcp.Resource, 0, len(page))
	for _, c := range page {
		r := &mcp.Resource{URI: c.URI, Name: c.Name, Description: c.Description, MIMEType: c.MIMEType}
		if !c.IsDir() {
			r.Size = c.Size
		}
		resources = append(resources, r)
	}
	result := &ReadDirectoryResult{ResultType: resultTypeFor(version), Resources: resources, NextCursor: next}
	if SupportsRequestScopedContext(version) {
		ttl := ListTTL
		result.TTLMs = &ttl
		result.CacheScope = "private"
	}
	return result, nil
}

// invalidParams wraps an error as JSON-RPC -32602.
func invalidParams(err error) error {
	return &jsonrpc.Error{Code: jsonrpc.CodeInvalidParams, Message: err.Error()}
}
