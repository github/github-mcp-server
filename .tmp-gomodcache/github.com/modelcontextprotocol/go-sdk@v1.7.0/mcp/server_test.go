// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

type testItem struct {
	Name  string
	Value string
}

type testListParams struct {
	Cursor string
}

func (p *testListParams) cursorPtr() *string {
	return &p.Cursor
}

type testListResult struct {
	Items      []*testItem
	NextCursor string
}

func (r *testListResult) nextCursorPtr() *string {
	return &r.NextCursor
}

var allTestItems = []*testItem{
	{"alpha", "val-A"},
	{"bravo", "val-B"},
	{"charlie", "val-C"},
	{"delta", "val-D"},
	{"echo", "val-E"},
	{"foxtrot", "val-F"},
	{"golf", "val-G"},
	{"hotel", "val-H"},
	{"india", "val-I"},
	{"juliet", "val-J"},
	{"kilo", "val-K"},
}

// getCursor encodes a string input into a URL-safe base64 cursor,
// fatally logging any encoding errors.
func getCursor(input string) string {
	cursor, err := encodeCursor(input)
	if err != nil {
		log.Fatalf("encodeCursor(%s) error = %v", input, err)
	}
	return cursor
}

func TestServerPaginateBasic(t *testing.T) {
	testCases := []struct {
		name           string
		initialItems   []*testItem
		inputCursor    string
		inputPageSize  int
		wantFeatures   []*testItem
		wantNextCursor string
		wantErr        bool
	}{
		{
			name:           "FirstPage_DefaultSize_Full",
			initialItems:   allTestItems,
			inputCursor:    "",
			inputPageSize:  5,
			wantFeatures:   allTestItems[0:5],
			wantNextCursor: getCursor("echo"), // Based on last item of first page
			wantErr:        false,
		},
		{
			name:           "SecondPage_DefaultSize_Full",
			initialItems:   allTestItems,
			inputCursor:    getCursor("echo"),
			inputPageSize:  5,
			wantFeatures:   allTestItems[5:10],
			wantNextCursor: getCursor("juliet"), // Based on last item of second page
			wantErr:        false,
		},
		{
			name:           "SecondPage_DefaultSize_Full_OutOfOrder",
			initialItems:   append(allTestItems[5:], allTestItems[0:5]...),
			inputCursor:    getCursor("echo"),
			inputPageSize:  5,
			wantFeatures:   allTestItems[5:10],
			wantNextCursor: getCursor("juliet"), // Based on last item of second page
			wantErr:        false,
		},
		{
			name:           "SecondPage_DefaultSize_Full_Duplicates",
			initialItems:   append(allTestItems, allTestItems[0:5]...),
			inputCursor:    getCursor("echo"),
			inputPageSize:  5,
			wantFeatures:   allTestItems[5:10],
			wantNextCursor: getCursor("juliet"), // Based on last item of second page
			wantErr:        false,
		},
		{
			name:           "LastPage_Remaining",
			initialItems:   allTestItems,
			inputCursor:    getCursor("juliet"),
			inputPageSize:  5,
			wantFeatures:   allTestItems[10:11], // Only 1 item left
			wantNextCursor: "",                  // No more pages
			wantErr:        false,
		},
		{
			name:           "PageSize_1",
			initialItems:   allTestItems,
			inputCursor:    "",
			inputPageSize:  1,
			wantFeatures:   allTestItems[0:1],
			wantNextCursor: getCursor("alpha"),
			wantErr:        false,
		},
		{
			name:           "PageSize_All",
			initialItems:   allTestItems,
			inputCursor:    "",
			inputPageSize:  len(allTestItems), // Page size equals total
			wantFeatures:   allTestItems,
			wantNextCursor: "", // No more pages
			wantErr:        false,
		},
		{
			name:           "PageSize_LargerThanAll",
			initialItems:   allTestItems,
			inputCursor:    "",
			inputPageSize:  len(allTestItems) + 5, // Page size larger than total
			wantFeatures:   allTestItems,
			wantNextCursor: "",
			wantErr:        false,
		},
		{
			name:           "EmptySet",
			initialItems:   nil,
			inputCursor:    "",
			inputPageSize:  5,
			wantFeatures:   nil,
			wantNextCursor: "",
			wantErr:        false,
		},
		{
			name:           "InvalidCursor",
			initialItems:   allTestItems,
			inputCursor:    "not-a-valid-gob-base64-cursor",
			inputPageSize:  5,
			wantFeatures:   nil, // Should be nil for error cases
			wantNextCursor: "",
			wantErr:        true,
		},
		{
			name:           "AboveNonExistentID",
			initialItems:   allTestItems,
			inputCursor:    getCursor("dne"), // A UID that doesn't exist
			inputPageSize:  5,
			wantFeatures:   allTestItems[4:9], // Should return elements above UID.
			wantNextCursor: getCursor("india"),
			wantErr:        false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFeatureSet(func(t *testItem) string { return t.Name })
			fs.add(tc.initialItems...)
			params := &testListParams{Cursor: tc.inputCursor}
			gotResult, err := paginateList(fs, tc.inputPageSize, params, &testListResult{}, func(res *testListResult, items []*testItem) {
				res.Items = items
			})
			if (err != nil) != tc.wantErr {
				t.Errorf("paginateList(%s) error, got %v, wantErr %v", tc.name, err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if diff := cmp.Diff(tc.wantFeatures, gotResult.Items); diff != "" {
				t.Errorf("paginateList(%s) mismatch (-want +got):\n%s", tc.name, diff)
			}
			if tc.wantNextCursor != gotResult.NextCursor {
				t.Errorf("paginateList(%s) nextCursor, got %v, want %v", tc.name, gotResult.NextCursor, tc.wantNextCursor)
			}
		})
	}
}

func TestServerPaginateVariousPageSizes(t *testing.T) {
	fs := newFeatureSet(func(t *testItem) string { return t.Name })
	fs.add(allTestItems...)
	// Try all possible page sizes, ensuring we get the correct list of items.
	for pageSize := 1; pageSize < len(allTestItems)+1; pageSize++ {
		var gotItems []*testItem
		var nextCursor string
		wantChunks := slices.Collect(slices.Chunk(allTestItems, pageSize))
		index := 0
		// Iterate through all pages, comparing sub-slices to the paginated list.
		for {
			params := &testListParams{Cursor: nextCursor}
			gotResult, err := paginateList(fs, pageSize, params, &testListResult{}, func(res *testListResult, items []*testItem) {
				res.Items = items
			})
			if err != nil {
				t.Fatalf("paginateList() unexpected error for pageSize %d, cursor %q: %v", pageSize, nextCursor, err)
			}
			if diff := cmp.Diff(wantChunks[index], gotResult.Items); diff != "" {
				t.Errorf("paginateList mismatch (-want +got):\n%s", diff)
			}
			gotItems = append(gotItems, gotResult.Items...)
			nextCursor = gotResult.NextCursor
			if nextCursor == "" {
				break
			}
			index++
		}

		if len(gotItems) != len(allTestItems) {
			t.Fatalf("paginateList() returned %d items, want %d", len(allTestItems), len(gotItems))
		}
	}
}

func TestServerCapabilities(t *testing.T) {
	tool := &Tool{Name: "t", InputSchema: &jsonschema.Schema{Type: "object"}}
	testCases := []struct {
		name             string
		configureServer  func(s *Server)
		serverOpts       ServerOptions
		wantCapabilities *ServerCapabilities
	}{
		{
			name:            "no capabilities",
			configureServer: func(s *Server) {},
			wantCapabilities: &ServerCapabilities{
				Logging: &LoggingCapabilities{},
			},
		},
		{
			name: "with prompts",
			configureServer: func(s *Server) {
				s.AddPrompt(&Prompt{Name: "p"}, nil)
			},
			wantCapabilities: &ServerCapabilities{
				Logging: &LoggingCapabilities{},
				Prompts: &PromptCapabilities{ListChanged: true},
			},
		},
		{
			name: "with resources",
			configureServer: func(s *Server) {
				s.AddResource(&Resource{URI: "file:///r"}, nil)
			},
			wantCapabilities: &ServerCapabilities{
				Logging:   &LoggingCapabilities{},
				Resources: &ResourceCapabilities{ListChanged: true},
			},
		},
		{
			name: "with resource templates",
			configureServer: func(s *Server) {
				s.AddResourceTemplate(&ResourceTemplate{URITemplate: "file:///rt"}, nil)
			},
			wantCapabilities: &ServerCapabilities{
				Logging:   &LoggingCapabilities{},
				Resources: &ResourceCapabilities{ListChanged: true},
			},
		},
		{
			name: "with resource subscriptions",
			configureServer: func(s *Server) {
				s.AddResourceTemplate(&ResourceTemplate{URITemplate: "file:///rt"}, nil)
			},
			serverOpts: ServerOptions{
				SubscribeHandler: func(context.Context, *SubscribeRequest) error {
					return nil
				},
				UnsubscribeHandler: func(context.Context, *UnsubscribeRequest) error {
					return nil
				},
			},
			wantCapabilities: &ServerCapabilities{
				Logging:   &LoggingCapabilities{},
				Resources: &ResourceCapabilities{ListChanged: true, Subscribe: true},
			},
		},
		{
			name: "with tools",
			configureServer: func(s *Server) {
				s.AddTool(tool, nil)
			},
			wantCapabilities: &ServerCapabilities{
				Logging: &LoggingCapabilities{},
				Tools:   &ToolCapabilities{ListChanged: true},
			},
		},
		{
			name:            "with completions",
			configureServer: func(s *Server) {},
			serverOpts: ServerOptions{
				CompletionHandler: func(context.Context, *CompleteRequest) (*CompleteResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ServerCapabilities{
				Logging:     &LoggingCapabilities{},
				Completions: &CompletionCapabilities{},
			},
		},
		{
			name: "all capabilities",
			configureServer: func(s *Server) {
				s.AddPrompt(&Prompt{Name: "p"}, nil)
				s.AddResource(&Resource{URI: "file:///r"}, nil)
				s.AddResourceTemplate(&ResourceTemplate{URITemplate: "file:///rt"}, nil)
				s.AddTool(tool, nil)
			},
			serverOpts: ServerOptions{
				SubscribeHandler: func(context.Context, *SubscribeRequest) error {
					return nil
				},
				UnsubscribeHandler: func(context.Context, *UnsubscribeRequest) error {
					return nil
				},
				CompletionHandler: func(context.Context, *CompleteRequest) (*CompleteResult, error) {
					return nil, nil
				},
			},
			wantCapabilities: &ServerCapabilities{
				Completions: &CompletionCapabilities{},
				Logging:     &LoggingCapabilities{},
				Prompts:     &PromptCapabilities{ListChanged: true},
				Resources:   &ResourceCapabilities{ListChanged: true, Subscribe: true},
				Tools:       &ToolCapabilities{ListChanged: true},
			},
		},
		{
			name:            "has features",
			configureServer: func(s *Server) {},
			serverOpts: ServerOptions{
				HasPrompts:   true,
				HasResources: true,
				HasTools:     true,
			},
			wantCapabilities: &ServerCapabilities{
				Logging:   &LoggingCapabilities{},
				Prompts:   &PromptCapabilities{ListChanged: true},
				Resources: &ResourceCapabilities{ListChanged: true},
				Tools:     &ToolCapabilities{ListChanged: true},
			},
		},
		{
			name:            "empty capabilities",
			configureServer: func(s *Server) {},
			serverOpts: ServerOptions{
				Capabilities: &ServerCapabilities{},
			},
			wantCapabilities: &ServerCapabilities{},
		},
		{
			name:            "no logging",
			configureServer: func(s *Server) {},
			serverOpts: ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools: &ToolCapabilities{ListChanged: true},
				},
			},
			wantCapabilities: &ServerCapabilities{
				Tools: &ToolCapabilities{ListChanged: true},
			},
		},
		{
			name:            "no list",
			configureServer: func(s *Server) {},
			serverOpts: ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools:   &ToolCapabilities{ListChanged: false},
					Prompts: &PromptCapabilities{ListChanged: false},
				},
			},
			wantCapabilities: &ServerCapabilities{
				Tools:   &ToolCapabilities{ListChanged: false},
				Prompts: &PromptCapabilities{ListChanged: false},
			},
		},
		{
			name: "adding tools-list",
			configureServer: func(s *Server) {
				s.AddTool(tool, nil)
			},
			serverOpts: ServerOptions{
				Capabilities: &ServerCapabilities{
					Logging: &LoggingCapabilities{},
				},
			},
			wantCapabilities: &ServerCapabilities{
				Logging: &LoggingCapabilities{},
				Tools:   &ToolCapabilities{ListChanged: true},
			},
		},
		{
			name: "adding tools-no list",
			configureServer: func(s *Server) {
				s.AddTool(tool, nil)
			},
			serverOpts: ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools: &ToolCapabilities{ListChanged: false},
				},
			},
			wantCapabilities: &ServerCapabilities{
				Tools: &ToolCapabilities{ListChanged: false},
			},
		},
		{
			name:            "experimental preserved",
			configureServer: func(s *Server) {},
			serverOpts: ServerOptions{
				Capabilities: &ServerCapabilities{
					Experimental: map[string]any{"custom": "value"},
					Logging:      &LoggingCapabilities{},
				},
			},
			wantCapabilities: &ServerCapabilities{
				Experimental: map[string]any{"custom": "value"},
				Logging:      &LoggingCapabilities{},
			},
		},
		{
			name:            "extensions preserved",
			configureServer: func(s *Server) {},
			serverOpts: func() ServerOptions {
				caps := &ServerCapabilities{
					Logging: &LoggingCapabilities{},
				}
				caps.AddExtension("io.example/ext1", map[string]any{"key": "value"})
				caps.AddExtension("io.example/ext2", nil)
				return ServerOptions{Capabilities: caps}
			}(),
			wantCapabilities: &ServerCapabilities{
				Extensions: map[string]any{
					"io.example/ext1": map[string]any{"key": "value"},
					"io.example/ext2": map[string]any{},
				},
				Logging: &LoggingCapabilities{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(testImpl, &tc.serverOpts)
			tc.configureServer(server)
			gotCapabilities := server.capabilities()
			if diff := cmp.Diff(tc.wantCapabilities, gotCapabilities); diff != "" {
				t.Errorf("capabilities() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestServerAddResourceTemplate(t *testing.T) {
	tests := []struct {
		name        string
		template    string
		expectPanic bool
	}{
		{"ValidFileTemplate", "file:///{a}/{b}", false},
		{"ValidCustomScheme", "myproto:///{a}", false},
		{"EmptyVariable", "file:///{}/{b}", true},
		{"UnclosedVariable", "file:///{a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := ResourceTemplate{URITemplate: tt.template}

			defer func() {
				if r := recover(); r != nil {
					if !tt.expectPanic {
						t.Errorf("%s: unexpected panic: %v", tt.name, r)
					}
				} else {
					if tt.expectPanic {
						t.Errorf("%s: expected panic but did not panic", tt.name)
					}
				}
			}()

			s := NewServer(testImpl, nil)
			s.AddResourceTemplate(&rt, nil)
		})
	}
}

// panicks reports whether f() panics.
func panics(f func()) (b bool) {
	defer func() {
		b = recover() != nil
	}()
	f()
	return false
}

func TestAddTool(t *testing.T) {
	// AddTool should panic if In is not a JSON object.
	// Out may be of any type whose inferred JSON Schema is valid.
	s := NewServer(testImpl, nil)
	if !panics(func() {
		AddTool(s, &Tool{Name: "T1"}, func(context.Context, *CallToolRequest, string) (*CallToolResult, any, error) { return nil, nil, nil })
	}) {
		t.Error("bad In: expected panic")
	}
	if panics(func() {
		AddTool(s, &Tool{Name: "T2"}, func(context.Context, *CallToolRequest, map[string]any) (*CallToolResult, any, error) {
			return nil, nil, nil
		})
	}) {
		t.Error("good In: expected no panic")
	}
	if panics(func() {
		AddTool(s, &Tool{Name: "T3"}, func(context.Context, *CallToolRequest, map[string]any) (*CallToolResult, int, error) {
			return nil, 0, nil
		})
	}) {
		t.Error("primitive Out: expected no panic")
	}
}

func TestAddToolNameValidation(t *testing.T) {
	tests := []struct {
		label             string
		name              string
		wantLogContaining string
	}{
		{
			label:             "empty name",
			name:              "",
			wantLogContaining: `tool name cannot be empty`,
		},
		{
			label:             "long name",
			name:              strings.Repeat("a", 129),
			wantLogContaining: "exceeds maximum length of 128 characters",
		},
		{
			label:             "name with spaces",
			name:              "get user profile",
			wantLogContaining: `tool name contains invalid characters: \" \"`,
		},
		{
			label:             "name with multiple invalid chars",
			name:              "user name@domain,com",
			wantLogContaining: `tool name contains invalid characters: \" \", \"@\", \",\"`,
		},
		{
			label:             "name with unicode",
			name:              "tool-ñame",
			wantLogContaining: `tool name contains invalid characters: \"ñ\"`,
		},
		{
			label:             "valid name",
			name:              "valid-tool_name.123",
			wantLogContaining: "", // No log expected
		},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			var buf bytes.Buffer
			s := NewServer(testImpl, &ServerOptions{
				Logger: slog.New(slog.NewTextHandler(&buf, nil)),
			})

			// Use the generic AddTool as it also calls validateToolName.
			AddTool(s, &Tool{Name: test.name}, func(context.Context, *CallToolRequest, any) (*CallToolResult, any, error) {
				return nil, nil, nil
			})

			logOutput := buf.String()
			if test.wantLogContaining != "" {
				if !strings.Contains(logOutput, test.wantLogContaining) {
					t.Errorf("log output =\n%s\nwant containing %q", logOutput, test.wantLogContaining)
				}
			} else {
				if logOutput != "" {
					t.Errorf("expected empty log output, got %q", logOutput)
				}
			}
		})
	}
}

func TestAddToolNilSchema(t *testing.T) {
	var nilSchema *jsonschema.Schema

	panicMsg := func(f func()) (msg string) {
		defer func() {
			if r := recover(); r != nil {
				msg = fmt.Sprintf("%v", r)
			}
		}()
		f()
		return msg
	}

	// Call s.AddTool directly to exercise the typed-nil checks added to that method.
	tests := []struct {
		name        string
		tool        *Tool
		wantContain string
	}{
		{
			name:        "typed nil InputSchema",
			tool:        &Tool{Name: "T", InputSchema: nilSchema},
			wantContain: "input schema is nil",
		},
		{
			name:        "typed nil OutputSchema",
			tool:        &Tool{Name: "T", InputSchema: &jsonschema.Schema{Type: "object"}, OutputSchema: nilSchema},
			wantContain: "output schema is nil",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := NewServer(testImpl, nil)

			msg := panicMsg(func() {
				s.AddTool(tc.tool, nil)
			})
			if msg == "" {
				t.Error("expected panic")
			} else if !strings.Contains(msg, tc.wantContain) {
				t.Errorf("panic message %q does not contain %q", msg, tc.wantContain)
			}
		})
	}
}

// TestAddToolNonObjectOutputSchema verifies the low-level
// Server.AddTool accepts an output schema whose root type is not "object"
// (array or primitive) and structured content of the matching shape
// round-trips end-to-end.
func TestAddToolNonObjectOutputSchema(t *testing.T) {
	ctx := context.Background()

	for _, tc := range []struct {
		name         string
		outputSchema any
		content      any
		want         any
	}{
		{
			name:         "array of objects",
			outputSchema: &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{Type: "object"}},
			content:      []any{map[string]any{"id": "u1"}, map[string]any{"id": "u2"}},
			want:         []any{map[string]any{"id": "u1"}, map[string]any{"id": "u2"}},
		},
		{
			name:         "primitive number (map-based schema)",
			outputSchema: map[string]any{"type": "number"},
			content:      42.0,
			want:         42.0,
		},
		{
			name:         "primitive string (RawMessage schema)",
			outputSchema: json.RawMessage(`{"type":"string"}`),
			content:      "hello",
			want:         "hello",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(testImpl, nil)
			handler := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
				return &CallToolResult{StructuredContent: tc.content}, nil
			}
			tool := &Tool{
				Name:         "t",
				InputSchema:  &jsonschema.Schema{Type: "object"},
				OutputSchema: tc.outputSchema,
			}
			// Must not panic.
			server.AddTool(tool, handler)

			ct, st := NewInMemoryTransports()
			if _, err := server.Connect(ctx, st, nil); err != nil {
				t.Fatal(err)
			}
			client := NewClient(testImpl, nil)
			cs, err := client.Connect(ctx, ct, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()

			res, err := cs.CallTool(ctx, &CallToolParams{Name: "t"})
			if err != nil {
				t.Fatal(err)
			}
			if res.IsError {
				t.Fatalf("unexpected tool error: %v", res.Content)
			}
			if diff := cmp.Diff(tc.want, res.StructuredContent); diff != "" {
				t.Errorf("structured content mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestAddToolGenericNonObjectOutput verifies SEP-2106 for the generic
// AddTool[In, Out] helper: Out may be a slice or primitive whose inferred
// JSON Schema has a non-object root.
func TestAddToolGenericNonObjectOutput(t *testing.T) {
	ctx := context.Background()

	type item struct {
		ID string `json:"id"`
	}

	t.Run("slice output", func(t *testing.T) {
		server := NewServer(testImpl, nil)
		AddTool(server, &Tool{Name: "list"},
			func(ctx context.Context, req *CallToolRequest, _ struct{}) (*CallToolResult, []item, error) {
				return nil, []item{{ID: "u1"}, {ID: "u2"}}, nil
			})

		ct, st := NewInMemoryTransports()
		if _, err := server.Connect(ctx, st, nil); err != nil {
			t.Fatal(err)
		}
		client := NewClient(testImpl, nil)
		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Verify tools/list reports an array-rooted output schema.
		// jsonschema-go infers ["null","array"] for slices since nil slices
		// are valid; accept either form.
		lt, err := cs.ListTools(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(lt.Tools) != 1 {
			t.Fatalf("got %d tools, want 1", len(lt.Tools))
		}
		gotSchema, _ := lt.Tools[0].OutputSchema.(map[string]any)
		typeOK := false
		switch typ := gotSchema["type"].(type) {
		case string:
			typeOK = typ == "array"
		case []any:
			for _, e := range typ {
				if e == "array" {
					typeOK = true
				}
			}
		}
		if !typeOK {
			t.Errorf("output schema type = %v, want to include %q", gotSchema["type"], "array")
		}

		res, err := cs.CallTool(ctx, &CallToolParams{Name: "list"})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected tool error: %v", res.Content)
		}
		want := []any{map[string]any{"id": "u1"}, map[string]any{"id": "u2"}}
		if diff := cmp.Diff(want, res.StructuredContent); diff != "" {
			t.Errorf("structured content mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("primitive output", func(t *testing.T) {
		server := NewServer(testImpl, nil)
		AddTool(server, &Tool{Name: "count"},
			func(ctx context.Context, req *CallToolRequest, _ struct{}) (*CallToolResult, int, error) {
				return nil, 42, nil
			})

		ct, st := NewInMemoryTransports()
		if _, err := server.Connect(ctx, st, nil); err != nil {
			t.Fatal(err)
		}
		client := NewClient(testImpl, nil)
		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		res, err := cs.CallTool(ctx, &CallToolParams{Name: "count"})
		if err != nil {
			t.Fatal(err)
		}
		if res.IsError {
			t.Fatalf("unexpected tool error: %v", res.Content)
		}
		if diff := cmp.Diff(float64(42), res.StructuredContent); diff != "" {
			t.Errorf("structured content mismatch (-want +got):\n%s", diff)
		}
	})
}

// TestAddToolInputSchemaComposition verifies SEP-2106 (input side): composition
// keywords such as oneOf are allowed on the input schema alongside
// type:"object".
func TestAddToolInputSchemaComposition(t *testing.T) {
	server := NewServer(testImpl, nil)
	tool := &Tool{
		Name: "lookup",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			OneOf: []*jsonschema.Schema{
				{Required: []string{"id"}, Properties: map[string]*jsonschema.Schema{"id": {Type: "string"}}},
				{Required: []string{"name"}, Properties: map[string]*jsonschema.Schema{"name": {Type: "string"}}},
			},
		},
	}
	// Must not panic.
	server.AddTool(tool, func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{}, nil
	})
}

type schema = jsonschema.Schema

func testToolForSchema[In, Out any](t *testing.T, tool *Tool, in string, out Out, wantIn, wantOut any, wantErrContaining string) {
	t.Helper()
	th := func(context.Context, *CallToolRequest, In) (*CallToolResult, Out, error) {
		return nil, out, nil
	}
	gott, goth, err := toolForErr(tool, th, nil)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantIn, gott.InputSchema); diff != "" {
		t.Errorf("input: mismatch (-want, +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantOut, gott.OutputSchema); diff != "" {
		t.Errorf("output: mismatch (-want, +got):\n%s", diff)
	}
	ctr := &CallToolRequest{
		Params: &CallToolParamsRaw{
			Arguments: json.RawMessage(in),
		},
	}
	result, err := goth(context.Background(), ctr)
	if wantErrContaining != "" {
		// Input validation errors are returned as tool results with IsError=true,
		// not as Go errors. Check both possibilities.
		if err != nil {
			if !strings.Contains(err.Error(), wantErrContaining) {
				t.Errorf("got error %q, want containing %q", err, wantErrContaining)
			}
		} else if result != nil && result.IsError {
			text := result.Content[0].(*TextContent).Text
			if !strings.Contains(text, wantErrContaining) {
				t.Errorf("got tool error %q, want containing %q", text, wantErrContaining)
			}
		} else {
			t.Errorf("got no error, want error containing %q", wantErrContaining)
		}
	} else if err != nil {
		t.Errorf("got error %v, want no error", err)
	}

	if gott.OutputSchema != nil && err == nil && !result.IsError {
		// Check that structured content matches exactly.
		unstructured := result.Content[0].(*TextContent).Text
		structured := string(result.StructuredContent.(json.RawMessage))
		if diff := cmp.Diff(unstructured, structured); diff != "" {
			t.Errorf("Unstructured content does not match structured content exactly (-unstructured +structured):\n%s", diff)
		}
	}
}

// TestClientRootCapabilities verifies that the server correctly observes
// RootsV2 for various client capability configurations. This tests the fix
// for #607.
func TestClientRootCapabilities(t *testing.T) {
	testCases := []struct {
		name         string
		capabilities *string // JSON for the capabilities field; nil means omit the field
		wantRootsV2  *RootCapabilities
	}{
		{
			name:         "capabilities field omitted",
			capabilities: nil,
			wantRootsV2:  nil,
		},
		{
			name:         "empty capabilities",
			capabilities: ptr(`{}`),
			wantRootsV2:  nil,
		},
		{
			name:         "capabilities with no roots",
			capabilities: ptr(`{"sampling": {}}`),
			wantRootsV2:  nil,
		},
		{
			name:         "capabilities with empty roots",
			capabilities: ptr(`{"roots": {}}`),
			wantRootsV2:  &RootCapabilities{ListChanged: false},
		},
		{
			name:         "capabilities with roots without listChanged",
			capabilities: ptr(`{"roots": {"listChanged": false}}`),
			wantRootsV2:  &RootCapabilities{ListChanged: false},
		},
		{
			name:         "capabilities with roots with listChanged",
			capabilities: ptr(`{"roots": {"listChanged": true}}`),
			wantRootsV2:  &RootCapabilities{ListChanged: true},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Create a minimal server.
			impl := &Implementation{Name: "testServer", Version: "v1.0.0"}
			s := NewServer(impl, nil)

			// Connect the server.
			cTransport, sTransport := NewInMemoryTransports()
			ss, err := s.Connect(ctx, sTransport, nil)
			if err != nil {
				t.Fatal(err)
			}

			// Connect the client JSON-RPC connection (raw, no client).
			cConn, err := cTransport.Connect(ctx)
			if err != nil {
				t.Fatal(err)
			}

			// Build initialize params, optionally including capabilities.
			var initParams json.RawMessage
			if tc.capabilities != nil {
				initParams = json.RawMessage(`{
					"protocolVersion": "2025-11-25",
					"capabilities": ` + *tc.capabilities + `,
					"clientInfo": {"name": "TestClient", "version": "1.0.0"}
				}`)
			} else {
				initParams = json.RawMessage(`{
					"protocolVersion": "2025-11-25",
					"clientInfo": {"name": "TestClient", "version": "1.0.0"}
				}`)
			}

			initReq, err := jsonrpc2.NewCall(jsonrpc2.Int64ID(1), "initialize", initParams)
			if err != nil {
				t.Fatal(err)
			}

			if err := cConn.Write(ctx, initReq); err != nil {
				t.Fatalf("Write failed: %v", err)
			}

			// Read the initialize response.
			msg, err := cConn.Read(ctx)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			resp, ok := msg.(*jsonrpc2.Response)
			if !ok {
				t.Fatalf("expected Response, got %T", msg)
			}
			if resp.Error != nil {
				t.Fatalf("initialize failed: %v", resp.Error)
			}

			// Verify that the server session has the correct RootsV2 value.
			params := ss.InitializeParams()
			if params == nil {
				t.Fatal("InitializeParams is nil")
			}

			var gotRootsV2 *RootCapabilities
			if params.Capabilities != nil {
				gotRootsV2 = params.Capabilities.RootsV2
			}
			if diff := cmp.Diff(tc.wantRootsV2, gotRootsV2); diff != "" {
				t.Errorf("RootsV2 mismatch (-want +got):\n%s", diff)
			}

			// Close the client connection.
			if err := cConn.Close(); err != nil {
				t.Fatalf("Stream.Close failed: %v", err)
			}
			ss.Wait()
		})
	}
}

func TestServerRejectsDuplicateInitialize(t *testing.T) {
	ctx := context.Background()

	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	cTransport, sTransport := NewInMemoryTransports()
	ss, err := server.Connect(ctx, sTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	cConn, err := cTransport.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer cConn.Close()

	firstParams := json.RawMessage(`{
		"protocolVersion": "2025-11-25",
		"clientInfo": {"name": "first-client", "version": "1.0.0"}
	}`)
	firstReq, err := jsonrpc2.NewCall(jsonrpc2.Int64ID(1), methodInitialize, firstParams)
	if err != nil {
		t.Fatal(err)
	}
	if err := cConn.Write(ctx, firstReq); err != nil {
		t.Fatalf("first initialize write failed: %v", err)
	}
	msg, err := cConn.Read(ctx)
	if err != nil {
		t.Fatalf("first initialize read failed: %v", err)
	}
	resp, ok := msg.(*jsonrpc2.Response)
	if !ok {
		t.Fatalf("expected Response, got %T", msg)
	}
	if resp.Error != nil {
		t.Fatalf("first initialize failed: %v", resp.Error)
	}

	initializedReq, err := jsonrpc2.NewNotification(notificationInitialized, &InitializedParams{})
	if err != nil {
		t.Fatal(err)
	}
	if err := cConn.Write(ctx, initializedReq); err != nil {
		t.Fatalf("initialized notification write failed: %v", err)
	}

	secondParams := json.RawMessage(`{
		"protocolVersion": "2024-11-05",
		"clientInfo": {"name": "second-client", "version": "2.0.0"}
	}`)
	secondReq, err := jsonrpc2.NewCall(jsonrpc2.Int64ID(2), methodInitialize, secondParams)
	if err != nil {
		t.Fatal(err)
	}
	if err := cConn.Write(ctx, secondReq); err != nil {
		t.Fatalf("second initialize write failed: %v", err)
	}
	msg, err = cConn.Read(ctx)
	if err != nil {
		t.Fatalf("second initialize read failed: %v", err)
	}
	resp, ok = msg.(*jsonrpc2.Response)
	if !ok {
		t.Fatalf("expected Response, got %T", msg)
	}
	if resp.Error == nil {
		t.Fatal("second initialize unexpectedly succeeded")
	}
	if !strings.Contains(resp.Error.Error(), `duplicate "initialize" received`) {
		t.Fatalf("second initialize error = %v, want duplicate initialize", resp.Error)
	}

	got := ss.InitializeParams()
	if got == nil {
		t.Fatal("InitializeParams is nil")
	}
	if got.ProtocolVersion != "2025-11-25" {
		t.Fatalf("ProtocolVersion = %q, want first initialize value", got.ProtocolVersion)
	}
	if got.ClientInfo == nil || got.ClientInfo.Name != "first-client" {
		t.Fatalf("ClientInfo = %#v, want first initialize value", got.ClientInfo)
	}
}

// TODO: move this to tool_test.go
func TestToolForSchemas(t *testing.T) {
	// Validate that toolForErr handles schemas properly.
	type in struct {
		P int `json:"p,omitempty"`
	}
	type out struct {
		B bool `json:"b,omitempty"`
	}

	var (
		falseSchema = &schema{Not: &schema{}}
		inSchema    = &schema{
			Type:                 "object",
			AdditionalProperties: falseSchema,
			Properties:           map[string]*schema{"p": {Type: "integer"}},
			PropertyOrder:        []string{"p"},
		}
		inSchema2 = &schema{
			Type:                 "object",
			AdditionalProperties: falseSchema,
			Properties:           map[string]*schema{"p": {Type: "string"}},
		}
		inSchema3 = &schema{
			Type:                 "object",
			AdditionalProperties: falseSchema,
			Properties:           map[string]*schema{}, // empty map is preserved
		}
		outSchema = &schema{
			Type:                 "object",
			AdditionalProperties: falseSchema,
			Properties:           map[string]*schema{"b": {Type: "boolean"}},
			PropertyOrder:        []string{"b"},
		}
		outSchema2 = &schema{
			Type:                 "object",
			AdditionalProperties: falseSchema,
			Properties:           map[string]*schema{"b": {Type: "integer"}},
			PropertyOrder:        []string{"b"},
		}
	)

	// Infer both schemas.
	testToolForSchema[in](t, &Tool{}, `{"p":3}`, out{true}, inSchema, outSchema, "")
	// Validate the input schema: expect an error if it's wrong.
	// We can't test that the output schema is validated, because it's typed.
	testToolForSchema[in](t, &Tool{}, `{"p":"x"}`, out{true}, inSchema, outSchema, `want "integer"`)
	// Ignore type any for output.
	testToolForSchema[in, any](t, &Tool{}, `{"p":3}`, 0, inSchema, nil, "")
	// Input is still validated.
	testToolForSchema[in, any](t, &Tool{}, `{"p":"x"}`, 0, inSchema, nil, `want "integer"`)
	// Tool sets input schema: that is what's used.
	testToolForSchema[in, any](t, &Tool{InputSchema: inSchema2}, `{"p":3}`, 0, inSchema2, nil, `want "string"`)
	// Tool sets input schema, empty properties map.
	testToolForSchema[in, any](t, &Tool{InputSchema: inSchema3}, `{}`, 0, inSchema3, nil, "")
	// Tool sets output schema: that is what's used, and validation happens.
	testToolForSchema[in, any](t, &Tool{OutputSchema: outSchema2}, `{"p":3}`, out{true},
		inSchema, outSchema2, `want "integer"`)

	// Check a slightly more complicated case.
	type weatherOutput struct {
		Summary string
		AsOf    time.Time
		Source  string
	}
	testToolForSchema[any](t, &Tool{}, `{}`, weatherOutput{},
		&schema{Type: "object"},
		&schema{
			Type:                 "object",
			Required:             []string{"Summary", "AsOf", "Source"},
			AdditionalProperties: falseSchema,
			Properties: map[string]*schema{
				"Summary": {Type: "string"},
				"AsOf":    {Type: "string"},
				"Source":  {Type: "string"},
			},
			PropertyOrder: []string{"Summary", "AsOf", "Source"},
		},
		"")
}

// TestServerCapabilitiesOverWire verifies that server capabilities are
// correctly sent over the wire during initialization.
func TestServerCapabilitiesOverWire(t *testing.T) {
	tool := &Tool{Name: "test-tool", InputSchema: &jsonschema.Schema{Type: "object"}}

	testCases := []struct {
		name             string
		serverOpts       *ServerOptions
		configureServer  func(s *Server)
		wantCapabilities *ServerCapabilities
	}{
		{
			name:            "Default capabilities",
			serverOpts:      nil,
			configureServer: func(s *Server) {},
			wantCapabilities: &ServerCapabilities{
				Logging: &LoggingCapabilities{},
			},
		},
		{
			name: "Custom Capabilities with tools",
			serverOpts: &ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools: &ToolCapabilities{ListChanged: false},
				},
			},
			configureServer: func(s *Server) {},
			wantCapabilities: &ServerCapabilities{
				Tools: &ToolCapabilities{ListChanged: false},
			},
		},
		{
			name: "Dynamic tool capability",
			serverOpts: &ServerOptions{
				Capabilities: &ServerCapabilities{
					Logging: &LoggingCapabilities{},
				},
			},
			configureServer: func(s *Server) {
				s.AddTool(tool, nil)
			},
			wantCapabilities: &ServerCapabilities{
				Logging: &LoggingCapabilities{},
				Tools:   &ToolCapabilities{ListChanged: true},
			},
		},
		{
			name: "Extensions over wire",
			serverOpts: func() *ServerOptions {
				caps := &ServerCapabilities{
					Logging: &LoggingCapabilities{},
				}
				caps.AddExtension("io.example/ext", map[string]any{"key": "value"})
				return &ServerOptions{Capabilities: caps}
			}(),
			configureServer: func(s *Server) {},
			wantCapabilities: &ServerCapabilities{
				Extensions: map[string]any{
					"io.example/ext": map[string]any{"key": "value"},
				},
				Logging: &LoggingCapabilities{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			// Create server.
			impl := &Implementation{Name: "testServer", Version: "v1.0.0"}
			server := NewServer(impl, tc.serverOpts)
			tc.configureServer(server)

			// Connect client and server.
			cTransport, sTransport := NewInMemoryTransports()
			ss, err := server.Connect(ctx, sTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ss.Close()

			client := NewClient(&Implementation{Name: "testClient", Version: "v1.0.0"}, nil)
			cs, err := client.Connect(ctx, cTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer cs.Close()

			// Check that the client received the expected capabilities.
			initResult := cs.InitializeResult()
			if initResult == nil {
				t.Fatal("InitializeResult is nil")
			}

			if diff := cmp.Diff(tc.wantCapabilities, initResult.Capabilities); diff != "" {
				t.Errorf("Capabilities mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// SEP-2575 removes the initialization handshake. An `initialize` request
// that opts into the new protocol via `_meta.protocolVersion` must be
// rejected with `Method not found` (-32601).
func TestServerSessionHandle_RejectsInitializeOnNewProtocol(t *testing.T) {

	tests := []struct {
		name       string
		params     any
		wantReject bool
	}{
		{
			name: "initialize with new-protocol _meta is rejected",
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
					MetaKeyClientCapabilities: map[string]any{},
				},
				"protocolVersion": protocolVersion20260728,
			},
			wantReject: true,
		},
		{
			name: "initialize without _meta is allowed (old protocol)",
			params: map[string]any{
				"protocolVersion": protocolVersion20251125,
			},
			wantReject: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ss := &ServerSession{server: NewServer(testImpl, nil)}
			id, err := jsonrpc.MakeID("test")
			if err != nil {
				t.Fatal(err)
			}
			req := &jsonrpc.Request{
				ID:     id,
				Method: methodInitialize,
				Params: mustMarshal(tc.params),
			}
			_, err = ss.handle(context.Background(), req)
			if tc.wantReject {
				if err == nil {
					t.Fatal("expected error rejecting initialize, got nil")
				}
				var jerr *jsonrpc.Error
				if !errors.As(err, &jerr) {
					t.Fatalf("error type = %T, want *jsonrpc.Error so the wire returns the right code", err)
				}
				if jerr.Code != jsonrpc.CodeMethodNotFound {
					t.Errorf("error code = %d, want %d (CodeMethodNotFound = -32601)", jerr.Code, jsonrpc.CodeMethodNotFound)
				}
				if !strings.Contains(jerr.Message, "initialize") {
					t.Errorf("error message %q does not mention %q", jerr.Message, "initialize")
				}
			} else {
				// Old-protocol initialize should be dispatched normally; any
				// CodeMethodNotFound here would mean the rejection branch
				// fired incorrectly.
				var jerr *jsonrpc.Error
				if errors.As(err, &jerr) && jerr.Code == jsonrpc.CodeMethodNotFound {
					t.Errorf("old-protocol initialize was incorrectly rejected: %v", err)
				}
			}
		})
	}

	t.Run("rejection error encodes to wire as code -32601", func(t *testing.T) {
		// Belt-and-braces check that the error type produced by handle()
		// actually serializes to JSON-RPC code -32601, not a bare 0.
		ss := &ServerSession{server: NewServer(testImpl, nil)}
		id, err := jsonrpc.MakeID("test")
		if err != nil {
			t.Fatal(err)
		}
		req := &jsonrpc.Request{
			ID:     id,
			Method: methodInitialize,
			Params: mustMarshal(map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
					MetaKeyClientCapabilities: map[string]any{},
				},
				"protocolVersion": protocolVersion20260728,
			}),
		}
		_, handleErr := ss.handle(context.Background(), req)
		if handleErr == nil {
			t.Fatal("expected rejection error, got nil")
		}
		data, encErr := jsonrpc.EncodeMessage(&jsonrpc.Response{ID: id, Error: handleErr.(*jsonrpc.Error)})
		if encErr != nil {
			t.Fatal(encErr)
		}
		var wire struct {
			Error struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(data, &wire); err != nil {
			t.Fatal(err)
		}
		if wire.Error.Code != jsonrpc.CodeMethodNotFound {
			t.Errorf("wire error code = %d, want %d; full response = %s", wire.Error.Code, jsonrpc.CodeMethodNotFound, data)
		}
	})
}

func TestServerSessionHandle_SetsResultTypeOnNewProtocol(t *testing.T) {
	server := NewServer(testImpl, &ServerOptions{
		CompletionHandler: func(context.Context, *CompleteRequest) (*CompleteResult, error) {
			return &CompleteResult{
				Completion: CompletionResultDetails{
					Values: []string{"go"},
				},
			}, nil
		},
	})
	AddTool(server, &Tool{Name: "tool"}, func(context.Context, *CallToolRequest, struct{}) (*CallToolResult, any, error) {
		return &CallToolResult{
			InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Continue?"}},
		}, nil, nil
	})
	server.AddPrompt(&Prompt{Name: "prompt"}, func(context.Context, *GetPromptRequest) (*GetPromptResult, error) {
		return &GetPromptResult{
			InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Continue?"}},
		}, nil
	})
	server.AddResource(&Resource{URI: "test://resource", Name: "resource"}, func(context.Context, *ReadResourceRequest) (*ReadResourceResult, error) {
		return &ReadResourceResult{
			InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Continue?"}},
		}, nil
	})
	newProtocolParams := func(fields map[string]any) map[string]any {
		params := map[string]any{
			"_meta": map[string]any{
				MetaKeyProtocolVersion:    protocolVersion20260728,
				MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
				MetaKeyClientCapabilities: map[string]any{},
			},
		}
		for k, v := range fields {
			params[k] = v
		}
		return params
	}

	tests := []struct {
		name   string
		method string
		params map[string]any
		want   resultType
	}{
		{
			name:   "discover",
			method: methodDiscover,
			params: newProtocolParams(nil),
			want:   resultTypeComplete,
		},
		{
			name:   "tools list",
			method: methodListTools,
			params: newProtocolParams(nil),
			want:   resultTypeComplete,
		},
		{
			name:   "prompts list",
			method: methodListPrompts,
			params: newProtocolParams(nil),
			want:   resultTypeComplete,
		},
		{
			name:   "resources list",
			method: methodListResources,
			params: newProtocolParams(nil),
			want:   resultTypeComplete,
		},
		{
			name:   "resource templates list",
			method: methodListResourceTemplates,
			params: newProtocolParams(nil),
			want:   resultTypeComplete,
		},
		{
			name:   "complete",
			method: methodComplete,
			params: newProtocolParams(map[string]any{
				"argument": map[string]any{
					"name":  "language",
					"value": "g",
				},
				"ref": map[string]any{
					"type": "ref/prompt",
					"name": "code_review",
				},
			}),
			want: resultTypeComplete,
		},
		{
			name:   "tool input required",
			method: methodCallTool,
			params: newProtocolParams(map[string]any{"name": "tool", "arguments": map[string]any{}}),
			want:   resultTypeInputRequired,
		},
		{
			name:   "prompt input required",
			method: methodGetPrompt,
			params: newProtocolParams(map[string]any{"name": "prompt"}),
			want:   resultTypeInputRequired,
		},
		{
			name:   "resource input required",
			method: methodReadResource,
			params: newProtocolParams(map[string]any{"uri": "test://resource"}),
			want:   resultTypeInputRequired,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ss := &ServerSession{server: server}
			id, err := jsonrpc.MakeID("test")
			if err != nil {
				t.Fatal(err)
			}
			result, err := ss.handle(context.Background(), &jsonrpc.Request{
				ID:     id,
				Method: tc.method,
				Params: mustMarshal(tc.params),
			})
			if err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				ResultType string `json:"resultType"`
			}
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if got.ResultType != string(tc.want) {
				t.Fatalf("resultType = %q, want %q; response = %s", got.ResultType, tc.want, data)
			}
		})
	}
}

// TestServerSessionHandle_RejectsRemovedMethodsOnNewProtocol verifies that
// the methods removed by SEP-2575 (`initialize`, `notifications/initialized`,
// `ping`) all return Method not found when the request opts into the new
// protocol via `_meta.protocolVersion`.
func TestServerSessionHandle_RejectsRemovedMethodsOnNewProtocol(t *testing.T) {

	newProtoMeta := map[string]any{
		"_meta": map[string]any{
			MetaKeyProtocolVersion:    protocolVersion20260728,
			MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
			MetaKeyClientCapabilities: map[string]any{},
		},
	}

	tests := []struct {
		name   string
		method string
	}{
		{"initialize", methodInitialize},
		{"ping", methodPing},
		{"notifications/initialized", notificationInitialized},
	}

	for _, tc := range tests {
		t.Run(tc.name+" rejected on new protocol", func(t *testing.T) {
			ss := &ServerSession{server: NewServer(testImpl, nil)}
			id, err := jsonrpc.MakeID("test")
			if err != nil {
				t.Fatal(err)
			}
			req := &jsonrpc.Request{
				ID:     id,
				Method: tc.method,
				Params: mustMarshal(newProtoMeta),
			}
			_, err = ss.handle(context.Background(), req)
			if err == nil {
				t.Fatalf("method %q on new protocol: got nil error, want CodeMethodNotFound", tc.method)
			}
			var jerr *jsonrpc.Error
			if !errors.As(err, &jerr) {
				t.Fatalf("error type = %T, want *jsonrpc.Error", err)
			}
			if jerr.Code != jsonrpc.CodeMethodNotFound {
				t.Errorf("method %q: code = %d, want %d", tc.method, jerr.Code, jsonrpc.CodeMethodNotFound)
			}
			if !strings.Contains(jerr.Message, tc.method) {
				t.Errorf("method %q: message %q does not mention method name", tc.method, jerr.Message)
			}
		})
	}
}

// TestServerSession_RejectsServerInitiatedRequests verifies that
// SEP-2322 / SEP-2575 is enforced at the API surface: [ServerSession.Elicit],
// [ServerSession.CreateMessage], [ServerSession.CreateMessageWithTools], and
// [ServerSession.ListRoots] must refuse to send a server-to-client request
// when the session is negotiated at protocol version >= 2026-07-28, and must
// remain functional on pre-2026-07-28 sessions.
func TestServerSession_RejectsServerInitiated(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name string
		call func(*ServerSession) error
	}{
		{
			name: "Elicit",
			call: func(ss *ServerSession) error {
				_, err := ss.Elicit(ctx, &ElicitParams{Message: "hi"})
				return err
			},
		},
		{
			name: "CreateMessage",
			call: func(ss *ServerSession) error {
				_, err := ss.CreateMessage(ctx, &CreateMessageParams{})
				return err
			},
		},
		{
			name: "CreateMessageWithTools",
			call: func(ss *ServerSession) error {
				_, err := ss.CreateMessageWithTools(ctx, &CreateMessageWithToolsParams{})
				return err
			},
		},
		{
			name: "ListRoots",
			call: func(ss *ServerSession) error {
				_, err := ss.ListRoots(ctx, nil)
				return err
			},
		},
	}

	// Cover both branches of the era gate: modern must reject, legacy must let
	// the call proceed to the wire (where it either succeeds or fails on
	// something unrelated to the guard, such as missing client capabilities).
	for _, protoVer := range []string{protocolVersion20260728, protocolVersion20251125} {
		for _, tc := range tests {
			t.Run(fmt.Sprintf("%s/%s", protoVer, tc.name), func(t *testing.T) {
				ct, st := NewInMemoryTransports()
				s := NewServer(testImpl, nil)
				ss, err := s.Connect(ctx, st, nil)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = ss.Close() })

				// Client advertises every server-to-client capability so a
				// legacy-era call is not rejected for capability reasons
				// instead of era reasons.
				c := NewClient(testImpl, &ClientOptions{
					ElicitationHandler: func(context.Context, *ElicitRequest) (*ElicitResult, error) {
						return &ElicitResult{Action: "cancel"}, nil
					},
					CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
						return &CreateMessageResult{Model: "m", Role: "assistant", Content: &TextContent{Text: "ok"}}, nil
					},
				})
				c.AddRoots(&Root{URI: "file:///workspace"})
				cs, err := c.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protoVer})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = cs.Close() })

				gotErr := tc.call(ss)
				modern := protoVer >= protocolVersion20260728
				if modern {
					if gotErr == nil {
						t.Fatalf("%s on %s: got nil error, want era-gate rejection", tc.name, protoVer)
					}
					wantSubstr := "cannot be sent while serving a request on protocol version " + protoVer
					if !strings.Contains(gotErr.Error(), wantSubstr) {
						t.Errorf("%s on %s: error %q does not contain %q", tc.name, protoVer, gotErr.Error(), wantSubstr)
					}
					if !strings.Contains(gotErr.Error(), "multi round-trip requests") {
						t.Errorf("%s on %s: error %q does not steer to MRTR", tc.name, protoVer, gotErr.Error())
					}
				} else {
					if gotErr != nil {
						t.Errorf("%s on %s: got error %v, want nil (era gate must not fire on legacy)", tc.name, protoVer, gotErr)
					}
				}
			})
		}
	}
}
