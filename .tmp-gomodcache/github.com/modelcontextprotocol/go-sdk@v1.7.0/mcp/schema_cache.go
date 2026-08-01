// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"reflect"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

// A SchemaCache caches JSON schemas to avoid repeated reflection and resolution.
//
// This is useful for stateless server deployments (one [Server] per request)
// where tools are re-registered on every request. Without caching, each
// [AddTool] call triggers expensive reflection-based schema generation.
//
// A SchemaCache is safe for concurrent use by multiple goroutines.
//
// # Trade-offs
//
// The cache is unbounded: it stores one entry per unique Go type or schema
// pointer. For typical MCP servers with a fixed set of tools, memory usage
// is negligible. However, if tool input types are generated dynamically,
// the cache will grow without bound.
//
// The cache uses pointer identity for pre-defined schemas. If a schema's
// contents change but the pointer remains the same, stale resolved schemas
// may be returned. In practice, this is not an issue because tool schemas
// are typically defined once at startup.
type SchemaCache struct {
	byType   sync.Map // reflect.Type -> *cachedSchema
	bySchema sync.Map // *jsonschema.Schema -> *jsonschema.Resolved
}

type cachedSchema struct {
	schema   *jsonschema.Schema
	resolved *jsonschema.Resolved
}

// NewSchemaCache creates a new [SchemaCache].
func NewSchemaCache() *SchemaCache {
	return &SchemaCache{}
}

func (c *SchemaCache) getByType(t reflect.Type) (*jsonschema.Schema, *jsonschema.Resolved, bool) {
	if v, ok := c.byType.Load(t); ok {
		cs := v.(*cachedSchema)
		return cs.schema, cs.resolved, true
	}
	return nil, nil, false
}

func (c *SchemaCache) setByType(t reflect.Type, schema *jsonschema.Schema, resolved *jsonschema.Resolved) {
	c.byType.Store(t, &cachedSchema{schema: schema, resolved: resolved})
}

func (c *SchemaCache) getBySchema(schema *jsonschema.Schema) (*jsonschema.Resolved, bool) {
	if v, ok := c.bySchema.Load(schema); ok {
		return v.(*jsonschema.Resolved), true
	}
	return nil, false
}

func (c *SchemaCache) setBySchema(schema *jsonschema.Schema, resolved *jsonschema.Resolved) {
	c.bySchema.Store(schema, resolved)
}
