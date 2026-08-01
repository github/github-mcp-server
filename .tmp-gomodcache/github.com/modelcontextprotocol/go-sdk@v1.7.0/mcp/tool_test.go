// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestApplySchema(t *testing.T) {
	schema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"x": {Type: "integer", Default: json.RawMessage("3")},
		},
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		t.Fatal(err)
	}

	type S struct {
		X int `json:"x"`
	}

	for _, tt := range []struct {
		data string
		v    any
		want any
	}{
		{`{"x": 1}`, new(S), &S{X: 1}},
		{`{}`, new(S), &S{X: 3}}, // default applied
		{`{"x": 0}`, new(S), &S{X: 0}},
		{`{"x": 1}`, new(map[string]any), &map[string]any{"x": 1.0}},
		{`{}`, new(map[string]any), &map[string]any{"x": 3.0}}, // default applied
		{`{"x": 0}`, new(map[string]any), &map[string]any{"x": 0.0}},
	} {
		raw := json.RawMessage(tt.data)
		raw, err = applySchema(raw, resolved, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &tt.v); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tt.v, tt.want) {
			t.Errorf("got %#v, want %#v", tt.v, tt.want)
		}
	}
}

func TestApplySchemaOutput(t *testing.T) {
	// SEP-2106: when forOutput is true, the schema may have a non-object root.
	for _, tc := range []struct {
		name    string
		schema  *jsonschema.Schema
		data    string
		want    string
		wantErr string
	}{
		{
			name:   "array root accepts array",
			schema: &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{Type: "integer"}},
			data:   `[1,2,3]`,
			want:   `[1,2,3]`,
		},
		{
			name:    "array root rejects wrong element type",
			schema:  &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{Type: "integer"}},
			data:    `[1,"two",3]`,
			wantErr: `"integer"`,
		},
		{
			name:   "primitive number root",
			schema: &jsonschema.Schema{Type: "number"},
			data:   `42`,
			want:   `42`,
		},
		{
			name:   "primitive string root",
			schema: &jsonschema.Schema{Type: "string"},
			data:   `"hello"`,
			want:   `"hello"`,
		},
		{
			name:   "object root with defaults",
			schema: &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"x": {Type: "integer", Default: json.RawMessage("3")}}},
			data:   `{}`,
			want:   `{"x":3}`,
		},
		{
			name:   "empty object root with defaults",
			schema: &jsonschema.Schema{Type: "object", Properties: map[string]*jsonschema.Schema{"x": {Type: "integer", Default: json.RawMessage("3")}}},
			data:   ``,
			want:   `{"x":3}`,
		},
		{
			name:   "object root accepts null as empty object",
			schema: &jsonschema.Schema{Type: "object"},
			data:   `null`,
			want:   `{}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := tc.schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
			if err != nil {
				t.Fatal(err)
			}
			got, err := applySchema(json.RawMessage(tc.data), resolved, true)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("got %s, want error containing %q", got, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("got error %q, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func TestToolErrorHandling(t *testing.T) {
	// Construct server and add both tools at the top level
	server := NewServer(testImpl, nil)

	// Create a tool that returns a structured error
	structuredErrorHandler := func(ctx context.Context, req *CallToolRequest, args map[string]any) (*CallToolResult, any, error) {
		return nil, nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: "internal server error",
		}
	}

	// Create a tool that returns a regular error
	regularErrorHandler := func(ctx context.Context, req *CallToolRequest, args map[string]any) (*CallToolResult, any, error) {
		return nil, nil, fmt.Errorf("tool execution failed")
	}

	AddTool(server, &Tool{Name: "error_tool", Description: "returns structured error"}, structuredErrorHandler)
	AddTool(server, &Tool{Name: "regular_error_tool", Description: "returns regular error"}, regularErrorHandler)

	// Connect server and client once
	ct, st := NewInMemoryTransports()
	_, err := server.Connect(context.Background(), st, nil)
	if err != nil {
		t.Fatal(err)
	}

	client := NewClient(testImpl, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Test that structured JSON-RPC errors are returned directly
	t.Run("structured_error", func(t *testing.T) {
		// Call the tool
		_, err = cs.CallTool(context.Background(), &CallToolParams{
			Name:      "error_tool",
			Arguments: map[string]any{},
		})

		// Should get the structured error directly
		if err == nil {
			t.Fatal("expected error, got nil")
		}

		var wireErr *jsonrpc.Error
		if !errors.As(err, &wireErr) {
			t.Fatalf("expected jsonrpc.Error, got %[1]T: %[1]v", err)
		}

		if wireErr.Code != jsonrpc.CodeInvalidParams {
			t.Errorf("expected error code %d, got %d", jsonrpc.CodeInvalidParams, wireErr.Code)
		}
	})

	// Test that regular errors are embedded in tool results
	t.Run("regular_error", func(t *testing.T) {
		// Call the tool
		result, err := cs.CallTool(context.Background(), &CallToolParams{
			Name:      "regular_error_tool",
			Arguments: map[string]any{},
		})
		// Should not get an error at the protocol level
		if err != nil {
			t.Fatalf("unexpected protocol error: %v", err)
		}

		// Should get a result with IsError=true
		if !result.IsError {
			t.Error("expected IsError=true, got false")
		}

		// Should have error message in content
		if len(result.Content) == 0 {
			t.Error("expected error content, got empty")
		}

		if textContent, ok := result.Content[0].(*TextContent); !ok {
			t.Error("expected TextContent")
		} else if !strings.Contains(textContent.Text, "tool execution failed") {
			t.Errorf("expected error message in content, got: %s", textContent.Text)
		}
	})
}

func TestValidateToolName(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		validTests := []struct {
			label    string
			toolName string
		}{
			{"simple alphanumeric names", "getUser"},
			{"names with underscores", "get_user_profile"},
			{"names with dashes", "user-profile-update"},
			{"names with dots", "admin.tools.list"},
			{"mixed character names", "DATA_EXPORT_v2.1"},
			{"single character names", "a"},
			{"128 character names", strings.Repeat("a", 128)},
		}
		for _, test := range validTests {
			t.Run(test.label, func(t *testing.T) {
				if err := validateToolName(test.toolName); err != nil {
					t.Errorf("validateToolName(%q) = %v, want nil", test.toolName, err)
				}
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		invalidTests := []struct {
			label             string
			toolName          string
			wantErrContaining string
		}{
			{"empty names", "", "tool name cannot be empty"},
			{"names longer than 128 characters", strings.Repeat("a", 129), "tool name exceeds maximum length of 128 characters (current: 129)"},
			{"names with spaces", "get user profile", `tool name contains invalid characters: " "`},
			{"names with commas", "get,user,profile", `tool name contains invalid characters: ","`},
			{"names with forward slashes", "user/profile/update", `tool name contains invalid characters: "/"`},
			{"names with other special chars", "user@domain.com", `tool name contains invalid characters: "@"`},
			{"names with multiple invalid chars", "user name@domain,com", `tool name contains invalid characters: " ", "@", ","`},
			{"names with unicode characters", "user-ñame", `tool name contains invalid characters: "ñ"`},
		}
		for _, test := range invalidTests {
			t.Run(test.label, func(t *testing.T) {
				if err := validateToolName(test.toolName); err == nil || !strings.Contains(err.Error(), test.wantErrContaining) {
					t.Errorf("validateToolName(%q) = %v, want error containing %q", test.toolName, err, test.wantErrContaining)
				}
			})
		}

	})

}
