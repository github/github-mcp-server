// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestExtractName(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		params   json.RawMessage
		wantName string
		wantOK   bool
	}{
		{
			name:     "tools/call",
			method:   "tools/call",
			params:   mustMarshal(&CallToolParams{Name: "my-tool"}),
			wantName: "my-tool",
			wantOK:   true,
		},
		{
			name:     "prompts/get",
			method:   "prompts/get",
			params:   mustMarshal(&GetPromptParams{Name: "code_review"}),
			wantName: "code_review",
			wantOK:   true,
		},
		{
			name:     "resources/read",
			method:   "resources/read",
			params:   mustMarshal(&ReadResourceParams{URI: "file:///info.txt"}),
			wantName: "file:///info.txt",
			wantOK:   true,
		},
		{
			name:     "tools/call with empty name",
			method:   "tools/call",
			params:   mustMarshal(&CallToolParams{Name: ""}),
			wantName: "",
			wantOK:   true,
		},
		{
			name:     "tool name with hyphen",
			method:   "tools/call",
			params:   mustMarshal(&CallToolParams{Name: "my-tool-v2"}),
			wantName: "my-tool-v2",
			wantOK:   true,
		},
		{
			name:     "tool name with underscore",
			method:   "tools/call",
			params:   mustMarshal(&CallToolParams{Name: "my_tool_name"}),
			wantName: "my_tool_name",
			wantOK:   true,
		},
		{
			name:     "resource URI with special chars",
			method:   "resources/read",
			params:   mustMarshal(&ReadResourceParams{URI: "file:///path/to/file%20name.txt"}),
			wantName: "file:///path/to/file%20name.txt",
			wantOK:   true,
		},
		{
			name:     "resource URI with query string",
			method:   "resources/read",
			params:   mustMarshal(&ReadResourceParams{URI: "https://example.com/resource?id=123"}),
			wantName: "https://example.com/resource?id=123",
			wantOK:   true,
		},
		{
			name:     "unrelated method",
			method:   "initialize",
			params:   mustMarshal(&InitializeParams{ProtocolVersion: "2025-06-18"}),
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "notification method",
			method:   "notifications/initialized",
			params:   nil,
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "invalid JSON params",
			method:   "tools/call",
			params:   []byte("not json"),
			wantName: "",
			wantOK:   false,
		},
		{
			name:     "nil params",
			method:   "tools/call",
			params:   nil,
			wantName: "",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotOK := extractName(tt.method, tt.params)
			if gotName != tt.wantName || gotOK != tt.wantOK {
				t.Errorf("extractName(%q, ...) = (%q, %v), want (%q, %v)",
					tt.method, gotName, gotOK, tt.wantName, tt.wantOK)
			}
		})
	}
}

func TestSetStandardHeaders(t *testing.T) {
	tests := []struct {
		name             string
		protocolVersion  string
		msg              jsonrpc.Message
		wantMethodHeader string
		wantNameHeader   string
	}{
		{
			name:             "tools/call with future version",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantMethodHeader: "tools/call",
			wantNameHeader:   "my-tool",
		},
		{
			name:             "prompts/get with future version",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              &jsonrpc.Request{Method: "prompts/get", Params: mustMarshal(&GetPromptParams{Name: "code_review"})},
			wantMethodHeader: "prompts/get",
			wantNameHeader:   "code_review",
		},
		{
			name:             "resources/read with future version",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              &jsonrpc.Request{Method: "resources/read", Params: mustMarshal(&ReadResourceParams{URI: "file:///info.txt"})},
			wantMethodHeader: "resources/read",
			wantNameHeader:   "file:///info.txt",
		},
		{
			name:             "initialize sets method only",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              &jsonrpc.Request{Method: "initialize", Params: mustMarshal(&InitializeParams{ProtocolVersion: minVersionForStandardHeaders})},
			wantMethodHeader: "initialize",
			wantNameHeader:   "",
		},
		{
			name:             "notification sets method only",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              &jsonrpc.Request{Method: "notifications/initialized"},
			wantMethodHeader: "notifications/initialized",
			wantNameHeader:   "",
		},
		{
			name:             "old version skips all headers",
			protocolVersion:  protocolVersion20251125,
			msg:              &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantMethodHeader: "",
			wantNameHeader:   "",
		},
		{
			name:             "empty version skips all headers",
			protocolVersion:  "",
			msg:              &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantMethodHeader: "",
			wantNameHeader:   "",
		},
		{
			name:             "nil message is a no-op",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              nil,
			wantMethodHeader: "",
			wantNameHeader:   "",
		},
		{
			name:             "response message is ignored",
			protocolVersion:  minVersionForStandardHeaders,
			msg:              &jsonrpc.Response{},
			wantMethodHeader: "",
			wantNameHeader:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.protocolVersion != "" {
				header.Set(protocolVersionHeader, tt.protocolVersion)
			}

			setStandardHeaders(context.Background(), header, tt.msg)

			if got := header.Get(methodHeader); got != tt.wantMethodHeader {
				t.Errorf("MethodHeader = %q, want %q", got, tt.wantMethodHeader)
			}
			if got := header.Get(nameHeader); got != tt.wantNameHeader {
				t.Errorf("NameHeader = %q, want %q", got, tt.wantNameHeader)
			}
		})
	}
}

func TestValidateMcpHeaders(t *testing.T) {

	tests := []struct {
		name           string
		version        string
		methodHeader   string
		nameHeader     string
		msg            jsonrpc.Message
		wantErr        bool
		wantErrContain string
	}{
		{
			name:    "old version skips validation",
			version: protocolVersion20251125,
			msg:     &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantErr: false,
		},
		{
			name:    "empty version skips validation",
			version: "",
			msg:     &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantErr: false,
		},
		{
			name:           "missing Mcp-Method header",
			version:        minVersionForStandardHeaders,
			msg:            &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantErr:        true,
			wantErrContain: "missing required Mcp-Method header",
		},
		{
			name:           "missing Mcp-Name for tools/call",
			version:        minVersionForStandardHeaders,
			methodHeader:   "tools/call",
			msg:            &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantErr:        true,
			wantErrContain: "missing required Mcp-Name header",
		},
		{
			name:           "missing Mcp-Name for resources/read",
			version:        minVersionForStandardHeaders,
			methodHeader:   "resources/read",
			msg:            &jsonrpc.Request{Method: "resources/read", Params: mustMarshal(&ReadResourceParams{URI: "file:///info.txt"})},
			wantErr:        true,
			wantErrContain: "missing required Mcp-Name header",
		},
		{
			name:           "missing Mcp-Name for prompts/get",
			version:        minVersionForStandardHeaders,
			methodHeader:   "prompts/get",
			msg:            &jsonrpc.Request{Method: "prompts/get", Params: mustMarshal(&GetPromptParams{Name: "review"})},
			wantErr:        true,
			wantErrContain: "missing required Mcp-Name header",
		},
		{
			name:           "method mismatch",
			version:        minVersionForStandardHeaders,
			methodHeader:   "tools/call",
			msg:            &jsonrpc.Request{Method: "prompts/get", Params: mustMarshal(&GetPromptParams{Name: "review"})},
			wantErr:        true,
			wantErrContain: "Mcp-Method header value 'tools/call' does not match body value 'prompts/get'",
		},
		{
			name:           "tool name mismatch",
			version:        minVersionForStandardHeaders,
			methodHeader:   "tools/call",
			nameHeader:     "wrong-tool",
			msg:            &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "right-tool"})},
			wantErr:        true,
			wantErrContain: "Mcp-Name header value 'wrong-tool' does not match body value 'right-tool'",
		},
		{
			name:           "resource URI mismatch",
			version:        minVersionForStandardHeaders,
			methodHeader:   "resources/read",
			nameHeader:     "file:///wrong.txt",
			msg:            &jsonrpc.Request{Method: "resources/read", Params: mustMarshal(&ReadResourceParams{URI: "file:///right.txt"})},
			wantErr:        true,
			wantErrContain: "Mcp-Name header value 'file:///wrong.txt' does not match body value 'file:///right.txt'",
		},
		{
			name:           "prompt name mismatch",
			version:        minVersionForStandardHeaders,
			methodHeader:   "prompts/get",
			nameHeader:     "wrong-prompt",
			msg:            &jsonrpc.Request{Method: "prompts/get", Params: mustMarshal(&GetPromptParams{Name: "right-prompt"})},
			wantErr:        true,
			wantErrContain: "Mcp-Name header value 'wrong-prompt' does not match body value 'right-prompt'",
		},
		{
			name:           "method value is case-sensitive",
			version:        minVersionForStandardHeaders,
			methodHeader:   "TOOLS/CALL",
			nameHeader:     "my-tool",
			msg:            &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantErr:        true,
			wantErrContain: "Mcp-Method header value 'TOOLS/CALL' does not match body value 'tools/call'",
		},
		{
			name:         "valid tools/call",
			version:      minVersionForStandardHeaders,
			methodHeader: "tools/call",
			nameHeader:   "my-tool",
			msg:          &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool"})},
			wantErr:      false,
		},
		{
			name:         "valid resources/read",
			version:      minVersionForStandardHeaders,
			methodHeader: "resources/read",
			nameHeader:   "file:///info.txt",
			msg:          &jsonrpc.Request{Method: "resources/read", Params: mustMarshal(&ReadResourceParams{URI: "file:///info.txt"})},
			wantErr:      false,
		},
		{
			name:         "valid prompts/get",
			version:      minVersionForStandardHeaders,
			methodHeader: "prompts/get",
			nameHeader:   "code_review",
			msg:          &jsonrpc.Request{Method: "prompts/get", Params: mustMarshal(&GetPromptParams{Name: "code_review"})},
			wantErr:      false,
		},
		{
			name:         "valid initialize (no name needed)",
			version:      minVersionForStandardHeaders,
			methodHeader: "initialize",
			msg:          &jsonrpc.Request{Method: "initialize", Params: mustMarshal(&InitializeParams{ProtocolVersion: minVersionForStandardHeaders})},
			wantErr:      false,
		},
		{
			name:         "valid notification (no name needed)",
			version:      minVersionForStandardHeaders,
			methodHeader: "notifications/initialized",
			msg:          &jsonrpc.Request{Method: "notifications/initialized"},
			wantErr:      false,
		},
		{
			name:         "tool name with hyphen",
			version:      minVersionForStandardHeaders,
			methodHeader: "tools/call",
			nameHeader:   "my-tool-name",
			msg:          &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my-tool-name"})},
			wantErr:      false,
		},
		{
			name:         "tool name with underscore",
			version:      minVersionForStandardHeaders,
			methodHeader: "tools/call",
			nameHeader:   "my_tool_name",
			msg:          &jsonrpc.Request{Method: "tools/call", Params: mustMarshal(&CallToolParams{Name: "my_tool_name"})},
			wantErr:      false,
		},
		{
			name:         "resource URI with special chars",
			version:      minVersionForStandardHeaders,
			methodHeader: "resources/read",
			nameHeader:   "file:///path/to/file%20name.txt",
			msg:          &jsonrpc.Request{Method: "resources/read", Params: mustMarshal(&ReadResourceParams{URI: "file:///path/to/file%20name.txt"})},
			wantErr:      false,
		},
		{
			name:         "resource URI with query string",
			version:      minVersionForStandardHeaders,
			methodHeader: "resources/read",
			nameHeader:   "https://example.com/resource?id=123",
			msg:          &jsonrpc.Request{Method: "resources/read", Params: mustMarshal(&ReadResourceParams{URI: "https://example.com/resource?id=123"})},
			wantErr:      false,
		},
		{
			name:    "response message is ignored",
			version: minVersionForStandardHeaders,
			msg:     &jsonrpc.Response{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.version != "" {
				header.Set(protocolVersionHeader, tt.version)
			}
			if tt.methodHeader != "" {
				header.Set(methodHeader, tt.methodHeader)
			}
			if tt.nameHeader != "" {
				header.Set(nameHeader, tt.nameHeader)
			}

			err := validateMcpHeaders(header, tt.msg, nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("validateMcpHeaders() = nil, want error containing %q", tt.wantErrContain)
				}
				if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Errorf("validateMcpHeaders() error = %q, want substring %q", err.Error(), tt.wantErrContain)
				}
			} else if err != nil {
				t.Errorf("validateMcpHeaders() = %v, want nil", err)
			}
		})
	}
}

func TestValidateToolParamHeaders(t *testing.T) {
	tests := []struct {
		name       string
		tool       *Tool
		wantErr    bool
		wantErrSub string
	}{
		{
			name: "valid tool with x-mcp-header",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "Region",
						},
					},
				},
			},
		},
		{
			name: "tool with no x-mcp-header annotations",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{"type": "string"},
					},
				},
			},
		},
		{
			name: "empty x-mcp-header value",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "non-empty string",
		},
		{
			name: "x-mcp-header with space",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "My Region",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "invalid character",
		},
		{
			name: "x-mcp-header with colon",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "Region:Primary",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "invalid character",
		},
		{
			name: "x-mcp-header with non-ASCII",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "Région",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "invalid character",
		},
		{
			name: "x-mcp-header with separator char (parens) is invalid per RFC 9110",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "X-(Region)",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "invalid character",
		},
		{
			name: "x-mcp-header with equals sign is invalid per RFC 9110",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "Region=1",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "invalid character",
		},
		{
			name: "x-mcp-header with all tchar specials is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{
							"type":         "string",
							"x-mcp-header": "!#$%&'*+-.^_`|~aZ0",
						},
					},
				},
			},
		},
		{
			name: "duplicate header names same case",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"a": map[string]any{"type": "string", "x-mcp-header": "Region"},
						"b": map[string]any{"type": "string", "x-mcp-header": "Region"},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "duplicate",
		},
		{
			name: "duplicate header names different case",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"a": map[string]any{"type": "string", "x-mcp-header": "Region"},
						"b": map[string]any{"type": "string", "x-mcp-header": "REGION"},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "duplicate",
		},
		{
			name: "x-mcp-header on array type",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"items": map[string]any{
							"type":         "array",
							"x-mcp-header": "Items",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "primitive types",
		},
		{
			name: "x-mcp-header on object type",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"nested": map[string]any{
							"type":         "object",
							"x-mcp-header": "Nested",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "primitive types",
		},
		{
			name: "x-mcp-header on number type is invalid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"count": map[string]any{
							"type":         "number",
							"x-mcp-header": "Count",
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "primitive types",
		},
		{
			name: "x-mcp-header on integer type is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"count": map[string]any{
							"type":         "integer",
							"x-mcp-header": "Count",
						},
					},
				},
			},
		},
		{
			name: "x-mcp-header on boolean type is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"flag": map[string]any{
							"type":         "boolean",
							"x-mcp-header": "Flag",
						},
					},
				},
			},
		},
		{
			name: "x-mcp-header on nested property inside object is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"region": map[string]any{
									"type":         "string",
									"x-mcp-header": "Region",
								},
							},
						},
					},
				},
			},
		},
		{
			name: "x-mcp-header on deeply nested property is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"outer": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"inner": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"value": map[string]any{
											"type":         "string",
											"x-mcp-header": "Value",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "duplicate header names across nesting levels",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{"type": "string", "x-mcp-header": "Region"},
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"region": map[string]any{"type": "string", "x-mcp-header": "Region"},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "duplicate",
		},
		{
			name: "object property without nested x-mcp-header is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"region": map[string]any{
									"type": "string",
								},
							},
						},
						"flag": map[string]any{
							"type":         "boolean",
							"x-mcp-header": "Flag",
						},
					},
				},
			},
		},
		{
			name: "jsonschema.Schema valid x-mcp-header",
			tool: &Tool{
				Name: "test",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"region": {
							Type:  "string",
							Extra: map[string]any{"x-mcp-header": "Region"},
						},
					},
				},
			},
		},
		{
			name: "jsonschema.Schema x-mcp-header on array type",
			tool: &Tool{
				Name: "test",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"items": {
							Type:  "array",
							Extra: map[string]any{"x-mcp-header": "Items"},
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "primitive types",
		},
		{
			name: "jsonschema.Schema nested x-mcp-header is valid",
			tool: &Tool{
				Name: "test",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"config": {
							Type: "object",
							Properties: map[string]*jsonschema.Schema{
								"region": {
									Type:  "string",
									Extra: map[string]any{"x-mcp-header": "Region"},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "json.RawMessage valid x-mcp-header",
			tool: &Tool{
				Name:        "test",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"region":{"type":"string","x-mcp-header":"Region"}}}`),
			},
		},
		{
			name: "json.RawMessage x-mcp-header on object type",
			tool: &Tool{
				Name:        "test",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"nested":{"type":"object","x-mcp-header":"Nested"}}}`),
			},
			wantErr:    true,
			wantErrSub: "primitive types",
		},
		{
			name: "nested invalid header name is rejected",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"region": map[string]any{
									"type":         "string",
									"x-mcp-header": "Bad Header", // contains a space
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "invalid character",
		},
		{
			name: "nested x-mcp-header on number type is rejected",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"count": map[string]any{
									"type":         "number",
									"x-mcp-header": "Count",
								},
							},
						},
					},
				},
			},
			wantErr:    true,
			wantErrSub: "primitive types",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateParamHeaderAnnotations(tt.tool)
			if tt.wantErr {
				if err == nil {
					t.Fatal("validateToolParamHeaders() = nil, want error")
				}
				if !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErrSub)
				}
			} else if err != nil {
				t.Errorf("validateToolParamHeaders() = %v, want nil", err)
			}
		})
	}
}

func TestFilterValidTools(t *testing.T) {
	valid := &Tool{
		Name: "valid",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"region": map[string]any{"type": "string", "x-mcp-header": "Region"},
			},
		},
	}
	invalid := &Tool{
		Name: "invalid",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"region": map[string]any{"type": "string", "x-mcp-header": ""},
			},
		},
	}
	noAnnotation := &Tool{
		Name:        "plain",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"q": map[string]any{"type": "string"}}},
	}
	nestedValid := &Tool{
		Name: "nested-valid",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tenant": map[string]any{"type": "string", "x-mcp-header": "TenantId"},
					},
				},
			},
		},
	}

	validJsonSchema := &Tool{
		Name: "valid-jsonschema",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"region": {
					Type:  "string",
					Extra: map[string]any{"x-mcp-header": "Region"},
				},
			},
		},
	}
	invalidJsonSchema := &Tool{
		Name: "invalid-jsonschema",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"items": {
					Type:  "array",
					Extra: map[string]any{"x-mcp-header": "Items"},
				},
			},
		},
	}

	result := filterValidTools(nil, []*Tool{valid, invalid, noAnnotation, nestedValid, validJsonSchema, invalidJsonSchema})
	if len(result) != 4 {
		t.Fatalf("filterValidTools returned %d tools, want 4", len(result))
	}
	if result[0].Name != "valid" || result[1].Name != "plain" || result[2].Name != "nested-valid" || result[3].Name != "valid-jsonschema" {
		t.Errorf("filterValidTools returned [%s, %s, %s, %s], want [valid, plain, nested-valid, valid-jsonschema]",
			result[0].Name, result[1].Name, result[2].Name, result[3].Name)
	}
}

func TestSetStandardHeadersWithParamHeaders(t *testing.T) {
	toolSchema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"region": map[string]any{
				"type":         "string",
				"x-mcp-header": "Region",
			},
			"query": map[string]any{
				"type": "string",
			},
			"priority": map[string]any{
				"type":         "string",
				"x-mcp-header": "Priority",
			},
		},
	}
	tool := &Tool{Name: "execute_sql", InputSchema: toolSchema}

	tests := []struct {
		name        string
		tool        *Tool
		params      any
		wantHeaders map[string]string
	}{
		{
			name: "sets param headers from arguments",
			tool: tool,
			params: &CallToolParams{
				Name:      "execute_sql",
				Arguments: map[string]any{"region": "us-west1", "query": "SELECT 1", "priority": "high"},
			},
			wantHeaders: map[string]string{
				"Mcp-Param-Region":   "us-west1",
				"Mcp-Param-Priority": "high",
			},
		},
		{
			name: "omits header when argument is missing",
			tool: tool,
			params: &CallToolParams{
				Name:      "execute_sql",
				Arguments: map[string]any{"query": "SELECT 1"},
			},
			wantHeaders: nil,
		},
		{
			name: "omits header when argument is null",
			tool: tool,
			params: &CallToolParams{
				Name:      "execute_sql",
				Arguments: map[string]any{"region": nil, "query": "SELECT 1"},
			},
			wantHeaders: nil,
		},
		{
			name: "encodes non-ASCII value",
			tool: tool,
			params: &CallToolParams{
				Name:      "execute_sql",
				Arguments: map[string]any{"region": "日本", "query": "SELECT 1"},
			},
			wantHeaders: map[string]string{
				"Mcp-Param-Region": "=?base64?5pel5pys?=",
			},
		},
		{
			name: "handles boolean argument",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"flag": map[string]any{"type": "boolean", "x-mcp-header": "Flag"},
					},
				},
			},
			params: &CallToolParams{
				Name:      "test",
				Arguments: map[string]any{"flag": true},
			},
			wantHeaders: map[string]string{
				"Mcp-Param-Flag": "true",
			},
		},
		{
			name: "handles integer argument",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"count": map[string]any{"type": "integer", "x-mcp-header": "Count"},
					},
				},
			},
			params: &CallToolParams{
				Name:      "test",
				Arguments: map[string]any{"count": float64(42)},
			},
			wantHeaders: map[string]string{
				"Mcp-Param-Count": "42",
			},
		},
		{
			name: "out-of-range integer argument produces no header",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"count": map[string]any{"type": "integer", "x-mcp-header": "Count"},
					},
				},
			},
			params: &CallToolParams{
				Name:      "test",
				Arguments: map[string]any{"count": float64(maxSafeInteger) + 2},
			},
			wantHeaders: nil,
		},
		{
			name: "no tool in extra does not add param headers",
			tool: nil,
			params: &CallToolParams{
				Name:      "execute_sql",
				Arguments: map[string]any{"region": "us-west1"},
			},
			wantHeaders: nil,
		},
		{
			name: "nested arguments resolve via dotted path",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"region": map[string]any{"type": "string", "x-mcp-header": "Region"},
								"tenant": map[string]any{"type": "string", "x-mcp-header": "TenantId"},
							},
						},
					},
				},
			},
			params: &CallToolParams{
				Name: "test",
				Arguments: map[string]any{
					"config": map[string]any{
						"region": "us-west1",
						"tenant": "acme",
					},
				},
			},
			wantHeaders: map[string]string{
				"Mcp-Param-Region":   "us-west1",
				"Mcp-Param-TenantId": "acme",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set(protocolVersionHeader, minVersionForStandardHeaders)

			ctx := context.Background()
			if tt.tool != nil {
				ctx = context.WithValue(ctx, toolContextKey, tt.tool)
			}

			msg := &jsonrpc.Request{
				Method: "tools/call",
				Params: mustMarshal(tt.params),
			}

			setStandardHeaders(ctx, header, msg)

			if got := header.Get(methodHeader); got != "tools/call" {
				t.Errorf("MethodHeader = %q, want %q", got, "tools/call")
			}

			for h, want := range tt.wantHeaders {
				if got := header.Get(h); got != want {
					t.Errorf("%s = %q, want %q", h, got, want)
				}
			}

			if got := header.Get("Mcp-Param-query"); got != "" {
				t.Errorf("non-annotated param got header: Mcp-Param-query = %q", got)
			}
		})
	}
}

func TestExtractToolParamHeaders(t *testing.T) {
	tests := []struct {
		name string
		tool *Tool
		want map[string]string
	}{
		{
			name: "extracts x-mcp-header annotations",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region":    map[string]any{"type": "string", "x-mcp-header": "Region"},
						"query":     map[string]any{"type": "string"},
						"tenant_id": map[string]any{"type": "string", "x-mcp-header": "TenantId"},
					},
				},
			},
			want: map[string]string{"region": "Region", "tenant_id": "TenantId"},
		},
		{
			name: "returns nil for tool without properties",
			tool: &Tool{Name: "test", InputSchema: map[string]any{"type": "object"}},
			want: nil,
		},
		{
			name: "returns nil for non-map schema",
			tool: &Tool{Name: "test", InputSchema: "not a map"},
			want: nil,
		},
		{
			name: "returns nil when no annotations",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type":       "object",
					"properties": map[string]any{"q": map[string]any{"type": "string"}},
				},
			},
			want: nil,
		},
		{
			name: "jsonschema.Schema with x-mcp-header in Extra",
			tool: &Tool{
				Name: "test",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"region": {
							Type:  "string",
							Extra: map[string]any{"x-mcp-header": "Region"},
						},
						"query": {Type: "string"},
					},
				},
			},
			want: map[string]string{"region": "Region"},
		},
		{
			name: "json.RawMessage with x-mcp-header",
			tool: &Tool{
				Name:        "test",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"region":{"type":"string","x-mcp-header":"Region"},"query":{"type":"string"}}}`),
			},
			want: map[string]string{"region": "Region"},
		},
		{
			name: "jsonschema.Schema without x-mcp-header",
			tool: &Tool{
				Name: "test",
				InputSchema: &jsonschema.Schema{
					Type: "object",
					Properties: map[string]*jsonschema.Schema{
						"query": {Type: "string"},
					},
				},
			},
			want: nil,
		},
		{
			name: "nested x-mcp-header annotations produce path-slice bindings",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{"type": "string", "x-mcp-header": "Region"},
						"config": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"tenant": map[string]any{"type": "string", "x-mcp-header": "TenantId"},
								"deep": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"flag": map[string]any{"type": "boolean", "x-mcp-header": "DeepFlag"},
									},
								},
							},
						},
					},
				},
			},
			want: map[string]string{
				"region":           "Region",
				"config.tenant":    "TenantId",
				"config.deep.flag": "DeepFlag",
			},
		},
		{
			name: "property name containing a dot is preserved in path",
			tool: &Tool{
				Name: "test",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"user.id": map[string]any{"type": "string", "x-mcp-header": "UserId"},
					},
				},
			},
			want: map[string]string{"user.id": "UserId"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractParamHeaderAnnotations(tt.tool)
			if tt.want == nil {
				if got != nil {
					t.Errorf("extractToolParamHeaders() = %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("extractToolParamHeaders() returned %d entries, want %d", len(got), len(tt.want))
			}
			// Index returned bindings by joined-path for comparison; the
			// expected map uses dotted-path keys for readability.
			gotMap := make(map[string]string, len(got))
			for _, b := range got {
				gotMap[strings.Join(b.Path, ".")] = b.Header
			}
			for k, v := range tt.want {
				if gotMap[k] != v {
					t.Errorf("extractToolParamHeaders()[%q] = %q, want %q", k, gotMap[k], v)
				}
			}
		})
	}
}

func TestUnmarshalPrimitive(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want any
	}{
		{"string", `"hello"`, "hello"},
		{"true", `true`, true},
		{"false", `false`, false},

		// Integer JSON numbers are promoted to int64.
		{"integer", `42`, int64(42)},
		{"integer zero", `0`, int64(0)},
		{"integer negative", `-7`, int64(-7)},
		{"integer max safe", `9007199254740991`, int64(maxSafeInteger)},
		{"integer min safe", `-9007199254740991`, int64(minSafeInteger)},
		// JSON serialization of an integer-valued float (e.g. "42.0") is
		// still a valid integer at the value level and must be accepted.
		{"integer-valued float", `42.0`, int64(42)},

		// Non-integer numbers, out-of-range integers, and disallowed JSON
		// kinds are rejected (return nil).
		{"float with fraction", `3.14`, nil},
		{"integer above max safe", `9007199254740993`, nil},
		{"integer below min safe", `-9007199254740993`, nil},
		{"null", `null`, nil},
		{"array", `[1,2]`, nil},
		{"object", `{"a":1}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unmarshalPrimitive(json.RawMessage(tt.raw))
			if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", tt.want) {
				t.Errorf("unmarshalPrimitive(%s) = %v (%T), want %v (%T)", tt.raw, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestEncodeHeaderValue(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		want   string
		wantOK bool
	}{
		// Strings
		{"plain ASCII", "us-west1", "us-west1", true},
		{"empty string", "", "", true},
		{"string with internal spaces", "us west 1", "us west 1", true},
		{"string with leading space", " us-west1", "=?base64?IHVzLXdlc3Qx?=", true},
		{"string with trailing space", "us-west1 ", "=?base64?dXMtd2VzdDEg?=", true},
		{"string with both spaces", " us-west1 ", "=?base64?IHVzLXdlc3QxIA==?=", true},
		{"non-ASCII", "日本語", "=?base64?5pel5pys6Kqe?=", true},
		{"mixed ASCII and non-ASCII", "Hello, 世界", "=?base64?SGVsbG8sIOS4lueVjA==?=", true},
		{"string with newline", "line1\nline2", "=?base64?bGluZTEKbGluZTI=?=", true},
		{"string with carriage return", "line1\r\nline2", "=?base64?bGluZTENCmxpbmUy?=", true},
		{"string with leading tab", "\tindented", "=?base64?CWluZGVudGVk?=", true},

		// Sentinel pattern collisions: plain-ASCII values that match the base64
		// sentinel pattern must also be base64-encoded to avoid ambiguity.
		{"sentinel collision literal", "=?base64?literal?=", "=?base64?PT9iYXNlNjQ/bGl0ZXJhbD89?=", true},
		{"sentinel collision empty", "=?base64??=", "=?base64?PT9iYXNlNjQ/Pz0=?=", true},
		// Uppercase sentinel does NOT collide (case-sensitive markers).
		{"uppercase pseudo-sentinel passes through", "=?BASE64?abc?=", "=?BASE64?abc?=", true},

		{"integer", int64(42), "42", true},
		{"integer zero", int64(0), "0", true},
		{"integer negative", int64(-7), "-7", true},
		{"integer max safe", int64(maxSafeInteger), "9007199254740991", true},
		{"integer min safe", int64(minSafeInteger), "-9007199254740991", true},

		// Booleans
		{"true", true, "true", true},
		{"false", false, "false", true},

		{"raw float64 rejected", float64(42), "", false},
		{"nil", nil, "", false},
		{"slice", []string{"a"}, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := encodeHeaderValue(tt.value)
			if ok != tt.wantOK {
				t.Fatalf("encodeHeaderValue(%v) ok = %v, want %v", tt.value, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("encodeHeaderValue(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestDecodeHeaderValue(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		want   string
		wantOK bool
	}{
		{"plain value", "us-west1", "us-west1", true},
		{"empty value", "", "", true},
		{"valid base64", "=?base64?SGVsbG8=?=", "Hello", true},
		{"non-ASCII decoded", "=?base64?5pel5pys6Kqe?=", "日本語", true},
		{"leading space decoded", "=?base64?IHVzLXdlc3Qx?=", " us-west1", true},
		// Per SEP-2243, the base64 sentinel markers are case-sensitive: an
		// uppercase prefix is treated as a literal value, not a base64 marker.
		{"uppercase prefix is literal", "=?BASE64?SGVsbG8=?=", "=?BASE64?SGVsbG8=?=", true},
		{"mixed case prefix is literal", "=?Base64?SGVsbG8=?=", "=?Base64?SGVsbG8=?=", true},
		{"invalid base64 chars", "=?base64?SGVs!!!bG8=?=", "", false},
		// Missing prefix or suffix: treated as literal values, not base64
		{"missing prefix", "SGVsbG8=", "SGVsbG8=", true},
		{"missing suffix", "=?base64?SGVsbG8=", "=?base64?SGVsbG8=", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := decodeHeaderValue(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("decodeHeaderValue(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if got != tt.want {
				t.Errorf("decodeHeaderValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	values := []string{
		"us-west1",
		"",
		" leading",
		"trailing ",
		"Hello, 世界",
		"line1\nline2",
		"\ttab",
		"=?base64?literal?=", // sentinel-pattern collision (SEP-2243)
	}
	for _, v := range values {
		encoded, ok := encodeHeaderValue(v)
		if !ok {
			t.Fatalf("encodeHeaderValue(%q) failed", v)
		}
		decoded, ok := decodeHeaderValue(encoded)
		if !ok {
			t.Fatalf("decodeHeaderValue(%q) failed", encoded)
		}
		if decoded != v {
			t.Errorf("round-trip failed: %q -> %q -> %q", v, encoded, decoded)
		}
	}
}

// TestValidateParamHeaders_IntegerComparison verifies server-side validation
// of integer x-mcp-header parameters per SEP-2243.
func TestValidateParamHeaders_IntegerComparison(t *testing.T) {
	tool := &Tool{
		Name: "test",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer", "x-mcp-header": "Count"},
			},
		},
	}
	tests := []struct {
		name      string
		headerVal string
		bodyArg   any
		wantErr   bool
	}{
		// Canonical decimal form matches.
		{"integer matches integer", "42", float64(42), false},
		{"integer header matches integer-valued float body", "42", float64(42.0), false},
		{"negative integer matches", "-7", float64(-7), false},
		{"large safe integer matches", "1000000000000", float64(1e12), false},

		{"non-canonical '42.0' header matches integer body", "42.0", float64(42), false},
		{"scientific notation header matches integer body", "1e2", float64(100), false},
		{"negative non-canonical header matches", "-7.0", float64(-7), false},

		// Genuine mismatches and invalid forms are still rejected.
		{"different integers do not match", "42", float64(43), true},
		{"fractional header against integer body", "3.14", float64(3), true},
		{"non-numeric header fails", "not-a-number", float64(42), true},
		{"header outside safe range against integer body", "9007199254740993", float64(42), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			header.Set(paramHeaderPrefix+"Count", tt.headerVal)
			args := map[string]any{"count": tt.bodyArg}
			msg := &jsonrpc.Request{
				Method: "tools/call",
				Params: mustMarshal(&CallToolParams{Name: "test", Arguments: args}),
			}
			err := validateParamHeaders(header, msg, tool)
			if tt.wantErr && err == nil {
				t.Errorf("validateParamHeaders() = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateParamHeaders() = %v, want nil", err)
			}
		})
	}
}

// TestValidateParamHeaders_NestedArguments verifies that the server-side
// validation can look up nested arguments via the dotted path produced by
// extractParamHeaderAnnotations.
func TestValidateParamHeaders_NestedArguments(t *testing.T) {
	tool := &Tool{
		Name: "test",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"config": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"region": map[string]any{"type": "string", "x-mcp-header": "Region"},
					},
				},
			},
		},
	}
	header := http.Header{}
	header.Set(paramHeaderPrefix+"Region", "us-west1")
	args := map[string]any{
		"config": map[string]any{"region": "us-west1"},
	}
	msg := &jsonrpc.Request{
		Method: "tools/call",
		Params: mustMarshal(&CallToolParams{Name: "test", Arguments: args}),
	}
	if err := validateParamHeaders(header, msg, tool); err != nil {
		t.Errorf("validateParamHeaders() = %v, want nil", err)
	}

	// Mismatched nested value should fail.
	args["config"].(map[string]any)["region"] = "eu-west1"
	msg.Params = mustMarshal(&CallToolParams{Name: "test", Arguments: args})
	if err := validateParamHeaders(header, msg, tool); err == nil {
		t.Error("validateParamHeaders() = nil, want error for mismatched nested value")
	}
}
