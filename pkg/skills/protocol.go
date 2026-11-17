package skills

import (
	"context"

	ghcontext "github.com/github/github-mcp-server/v2/pkg/context"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Method names defined by the extension.
const (
	MethodListSkills    = "skills/list"
	MethodGetSkill      = "skills/get"
	MethodReadDirectory = "resources/directory/read"
)

// ProtocolVersionRequestScoped is the first protocol version whose requests
// carry their own negotiated context, and the version from which the extension's
// results carry list-caching attributes.
const ProtocolVersionRequestScoped = "2026-07-28"

// ResultTypeComplete marks a custom-method result as whole rather than a
// fragment of a multi-part response. It is emitted only for protocol versions
// that define it.
const ResultTypeComplete = "complete"

// ListSkillsParams are the parameters of skills/list.
type ListSkillsParams struct {
	mcp.ParamsBase
	// Cursor is an opaque pagination cursor from a previous result's
	// nextCursor.
	Cursor string `json:"cursor,omitempty"`
}

// ListSkillsResult is the result of skills/list.
//
// Skills is always rendered, as an empty array when the server publishes
// nothing. An empty listing is a valid answer and is not proof that the server
// has no skills: a large, generated, or otherwise unenumerable catalog may
// decline to enumerate itself while still answering skills/get for every skill
// it serves.
type ListSkillsResult struct {
	mcp.ResultBase
	// ResultType marks the result complete. It is omitted for protocol
	// versions that predate the attribute.
	ResultType string  `json:"resultType,omitempty"`
	Skills     []Entry `json:"skills"`
	// NextCursor is present only when further pages remain.
	NextCursor string `json:"nextCursor,omitempty"`
	// TTLMs and CacheScope are the base protocol's list-caching attributes,
	// carried from protocol version 2026-07-28 and omitted before it. They are
	// a freshness hint for the listing, not an integrity property: content is
	// governed by digest verification however fresh a cached listing is.
	TTLMs      *int   `json:"ttlMs,omitempty"`
	CacheScope string `json:"cacheScope,omitempty"`
}

// GetSkillParams are the parameters of skills/get.
type GetSkillParams struct {
	mcp.ParamsBase
	// URI is the resource URI of a skill's SKILL.md.
	URI string `json:"uri"`
}

// GetSkillResult is the result of skills/get. It carries no pagination cursor:
// a single entry is not a list.
type GetSkillResult struct {
	mcp.ResultBase
	ResultType string `json:"resultType,omitempty"`
	Skill      Entry  `json:"skill"`
	TTLMs      *int   `json:"ttlMs,omitempty"`
	CacheScope string `json:"cacheScope,omitempty"`
}

// ReadDirectoryParams are the parameters of resources/directory/read.
type ReadDirectoryParams struct {
	mcp.ParamsBase
	// URI is the directory resource URI, written without a trailing slash.
	URI string `json:"uri"`
	// Cursor is an opaque pagination cursor from a previous result's
	// nextCursor.
	Cursor string `json:"cursor,omitempty"`
}

// ReadDirectoryResult is the result of resources/directory/read. It carries the
// direct children of one directory; the listing is not recursive, and clients
// descend by calling the method again on a child directory.
type ReadDirectoryResult struct {
	mcp.ResultBase
	ResultType string          `json:"resultType,omitempty"`
	Resources  []*mcp.Resource `json:"resources"`
	NextCursor string          `json:"nextCursor,omitempty"`
	TTLMs      *int            `json:"ttlMs,omitempty"`
	CacheScope string          `json:"cacheScope,omitempty"`
}

// NegotiatedProtocolVersion resolves the protocol version in force for a single
// request.
//
// Resolution is request-scoped and never inferred from cached session state.
// The context is consulted first, because a stateless HTTP server establishes
// per-request values there and its session carries none. Failing that, the
// request's own _meta is consulted, which is where a 2026-07-28 client puts the
// negotiated version. Only then does the session's initialize params apply,
// which is how a stdio client that negotiated once and stays connected is
// answered.
//
// The empty string means the version is unknown, which is treated throughout as
// "older than 2026-07-28" — the conservative reading, since an older client must
// not be sent fields its protocol version does not define.
func NegotiatedProtocolVersion(ctx context.Context, params mcp.Params, ss *mcp.ServerSession) string {
	if info, ok := ghcontext.MCPMethod(ctx); ok && info != nil && info.ProtocolVersion != "" {
		return info.ProtocolVersion
	}
	if params != nil && !isNilParams(params) {
		if meta := params.GetMeta(); meta != nil {
			if v, ok := meta[mcp.MetaKeyProtocolVersion].(string); ok && v != "" {
				return v
			}
		}
	}
	if ss != nil {
		if init := ss.InitializeParams(); init != nil {
			return init.ProtocolVersion
		}
	}
	return ""
}

// SupportsRequestScopedContext reports whether a protocol version is 2026-07-28
// or later. MCP versions are ISO dates, so lexical ordering is chronological.
func SupportsRequestScopedContext(version string) bool {
	return version >= ProtocolVersionRequestScoped
}

// resultTypeFor returns the resultType value appropriate to a protocol version:
// "complete" from 2026-07-28, and empty before it so the field is omitted.
func resultTypeFor(version string) string {
	if SupportsRequestScopedContext(version) {
		return ResultTypeComplete
	}
	return ""
}

// isNilParams reports whether a Params interface holds a nil pointer, which is
// how the SDK represents a request that omitted params entirely.
func isNilParams(p mcp.Params) bool {
	switch v := p.(type) {
	case *ListSkillsParams:
		return v == nil
	case *GetSkillParams:
		return v == nil
	case *ReadDirectoryParams:
		return v == nil
	}
	return false
}
