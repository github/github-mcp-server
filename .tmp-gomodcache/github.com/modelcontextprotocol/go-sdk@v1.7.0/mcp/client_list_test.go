// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"context"
	"encoding/json"
	"iter"
	"log"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestList(t *testing.T) {
	ctx := context.Background()
	server := mcp.NewServer(testImpl, nil)
	client := mcp.NewClient(testImpl, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer clientSession.Close()

	t.Run("tools", func(t *testing.T) {
		var wantTools []*mcp.Tool
		for _, name := range []string{"apple", "banana", "cherry"} {
			tt := &mcp.Tool{Name: name, Description: name + " tool"}
			mcp.AddTool(server, tt, SayHi)
			is, err := jsonschema.For[SayHiParams](nil)
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(is)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &tt.InputSchema); err != nil {
				t.Fatal(err)
			}
			wantTools = append(wantTools, tt)
		}
		t.Run("list", func(t *testing.T) {
			res, err := clientSession.ListTools(ctx, nil)
			if err != nil {
				t.Fatal("ListTools() failed:", err)
			}
			if diff := cmp.Diff(wantTools, res.Tools, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
				t.Fatalf("ListTools() mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("iterator", func(t *testing.T) {
			testIterator(t, clientSession.Tools(ctx, nil), wantTools)
		})
	})

	t.Run("resources", func(t *testing.T) {
		var wantResources []*mcp.Resource
		for _, name := range []string{"apple", "banana", "cherry"} {
			r := &mcp.Resource{URI: "http://" + name}
			wantResources = append(wantResources, r)
			server.AddResource(r, nil)
		}

		t.Run("list", func(t *testing.T) {
			res, err := clientSession.ListResources(ctx, nil)
			if err != nil {
				t.Fatal("ListResources() failed:", err)
			}
			if diff := cmp.Diff(wantResources, res.Resources, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
				t.Fatalf("ListResources() mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("iterator", func(t *testing.T) {
			testIterator(t, clientSession.Resources(ctx, nil), wantResources)
		})
	})

	t.Run("templates", func(t *testing.T) {
		var wantResourceTemplates []*mcp.ResourceTemplate
		for _, name := range []string{"apple", "banana", "cherry"} {
			rt := &mcp.ResourceTemplate{URITemplate: "http://" + name + "/{x}"}
			wantResourceTemplates = append(wantResourceTemplates, rt)
			server.AddResourceTemplate(rt, nil)
		}
		t.Run("list", func(t *testing.T) {
			res, err := clientSession.ListResourceTemplates(ctx, nil)
			if err != nil {
				t.Fatal("ListResourceTemplates() failed:", err)
			}
			if diff := cmp.Diff(wantResourceTemplates, res.ResourceTemplates, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
				t.Fatalf("ListResourceTemplates() mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("ResourceTemplatesIterator", func(t *testing.T) {
			testIterator(t, clientSession.ResourceTemplates(ctx, nil), wantResourceTemplates)
		})
	})

	t.Run("prompts", func(t *testing.T) {
		var wantPrompts []*mcp.Prompt
		for _, name := range []string{"apple", "banana", "cherry"} {
			p := &mcp.Prompt{Name: name, Description: name + " prompt"}
			wantPrompts = append(wantPrompts, p)
			server.AddPrompt(p, testPromptHandler)
		}
		t.Run("list", func(t *testing.T) {
			res, err := clientSession.ListPrompts(ctx, nil)
			if err != nil {
				t.Fatal("ListPrompts() failed:", err)
			}
			if diff := cmp.Diff(wantPrompts, res.Prompts, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
				t.Fatalf("ListPrompts() mismatch (-want +got):\n%s", diff)
			}
		})
		t.Run("iterator", func(t *testing.T) {
			testIterator(t, clientSession.Prompts(ctx, nil), wantPrompts)
		})
	})
}

func testIterator[T any](t *testing.T, seq iter.Seq2[*T, error], want []*T) {
	t.Helper()
	var got []*T
	for x, err := range seq {
		if err != nil {
			t.Fatalf("iteration failed: %v", err)
		}
		got = append(got, x)
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(jsonschema.Schema{})); diff != "" {
		t.Fatalf("mismatch (-want +got):\n%s", diff)
	}
}

func testPromptHandler(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	panic("not implemented")
}
