// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"reflect"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func TestSchemaCacheByType(t *testing.T) {
	cache := NewSchemaCache()

	type TestInput struct {
		Name string `json:"name"`
	}

	rt := reflect.TypeFor[TestInput]()

	if _, _, ok := cache.getByType(rt); ok {
		t.Error("expected cache miss for new type")
	}

	schema := &jsonschema.Schema{Type: "object"}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("failed to resolve schema: %v", err)
	}
	cache.setByType(rt, schema, resolved)

	gotSchema, gotResolved, ok := cache.getByType(rt)
	if !ok {
		t.Error("expected cache hit after set")
	}
	if gotSchema != schema {
		t.Error("schema mismatch")
	}
	if gotResolved != resolved {
		t.Error("resolved schema mismatch")
	}
}

func TestSchemaCacheBySchema(t *testing.T) {
	cache := NewSchemaCache()

	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {Type: "string"},
		},
	}

	if _, ok := cache.getBySchema(schema); ok {
		t.Error("expected cache miss for new schema")
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("failed to resolve schema: %v", err)
	}
	cache.setBySchema(schema, resolved)

	gotResolved, ok := cache.getBySchema(schema)
	if !ok {
		t.Error("expected cache hit after set")
	}
	if gotResolved != resolved {
		t.Error("resolved schema mismatch")
	}

	// Different pointer should miss (cache uses pointer identity).
	schema2 := &jsonschema.Schema{Type: "object"}
	if _, ok = cache.getBySchema(schema2); ok {
		t.Error("expected cache miss for different schema pointer")
	}
}

func TestSetSchemaCachesGeneratedSchemas(t *testing.T) {
	cache := NewSchemaCache()

	type TestInput struct {
		Query string `json:"query"`
	}

	rt := reflect.TypeFor[TestInput]()

	var sfield1 any
	var rfield1 *jsonschema.Resolved
	if _, err := setSchema[TestInput](&sfield1, &rfield1, cache); err != nil {
		t.Fatalf("setSchema failed: %v", err)
	}

	cachedSchema, cachedResolved, ok := cache.getByType(rt)
	if !ok {
		t.Fatal("schema not cached after first setSchema call")
	}

	var sfield2 any
	var rfield2 *jsonschema.Resolved
	if _, err := setSchema[TestInput](&sfield2, &rfield2, cache); err != nil {
		t.Fatalf("setSchema failed on second call: %v", err)
	}

	if sfield2.(*jsonschema.Schema) != cachedSchema {
		t.Error("expected cached schema to be returned")
	}
	if rfield2 != cachedResolved {
		t.Error("expected cached resolved schema to be returned")
	}
}

func TestSetSchemaCachesProvidedSchemas(t *testing.T) {
	cache := NewSchemaCache()

	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"query": {Type: "string"},
		},
	}

	var sfield1 any = schema
	var rfield1 *jsonschema.Resolved
	if _, err := setSchema[map[string]any](&sfield1, &rfield1, cache); err != nil {
		t.Fatalf("setSchema failed: %v", err)
	}

	cachedResolved, ok := cache.getBySchema(schema)
	if !ok {
		t.Fatal("resolved schema not cached after first setSchema call")
	}
	if rfield1 != cachedResolved {
		t.Error("expected same resolved schema")
	}

	var sfield2 any = schema
	var rfield2 *jsonschema.Resolved
	if _, err := setSchema[map[string]any](&sfield2, &rfield2, cache); err != nil {
		t.Fatalf("setSchema failed on second call: %v", err)
	}

	if rfield2 != cachedResolved {
		t.Error("expected cached resolved schema to be returned")
	}
}

func TestSetSchemaNilCache(t *testing.T) {
	type TestInput struct {
		Query string `json:"query"`
	}

	var sfield1 any
	var rfield1 *jsonschema.Resolved
	if _, err := setSchema[TestInput](&sfield1, &rfield1, nil); err != nil {
		t.Fatalf("setSchema failed: %v", err)
	}

	var sfield2 any
	var rfield2 *jsonschema.Resolved
	if _, err := setSchema[TestInput](&sfield2, &rfield2, nil); err != nil {
		t.Fatalf("setSchema failed on second call: %v", err)
	}

	if sfield1 == nil || sfield2 == nil {
		t.Error("expected schemas to be generated")
	}
	if rfield1 == nil || rfield2 == nil {
		t.Error("expected resolved schemas to be generated")
	}
}

func TestAddToolWithSharedCache(t *testing.T) {
	cache := NewSchemaCache()

	type GreetInput struct {
		Name string `json:"name" jsonschema:"the name to greet"`
	}

	type GreetOutput struct {
		Message string `json:"message"`
	}

	handler := func(ctx context.Context, req *CallToolRequest, in GreetInput) (*CallToolResult, GreetOutput, error) {
		return &CallToolResult{}, GreetOutput{Message: "Hello, " + in.Name}, nil
	}

	tool := &Tool{
		Name:        "greet",
		Description: "Greet someone",
	}

	// Simulate stateless server pattern: new server per request, shared cache.
	for range 3 {
		s := NewServer(&Implementation{Name: "test", Version: "1.0"}, &ServerOptions{
			SchemaCache: cache,
		})
		AddTool(s, tool, handler)
	}

	rt := reflect.TypeFor[GreetInput]()
	if _, _, ok := cache.getByType(rt); !ok {
		t.Error("expected schema to be cached by type after multiple AddTool calls")
	}
}
