// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file contains tests to verify that UnmarshalJSON methods for Content types
// don't panic when unmarshaling onto nil pointers, as requested in GitHub issue #205.
//
// NOTE: The contentFromWire function has been fixed to handle nil wire.Content
// gracefully by returning an error instead of panicking.

package mcp_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestContentUnmarshalNil(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		content any
		want    any
	}{
		{
			name:    "CallToolResult nil Content",
			json:    `{"content":[{"type":"text","text":"hello"}]}`,
			content: &mcp.CallToolResult{},
			want:    &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}},
		},
		{
			name:    "CreateMessageResult nil Content",
			json:    `{"content":{"type":"text","text":"hello"},"model":"test","role":"user"}`,
			content: &mcp.CreateMessageResult{},
			want:    &mcp.CreateMessageResult{Content: &mcp.TextContent{Text: "hello"}, Model: "test", Role: "user"},
		},
		{
			name:    "PromptMessage nil Content",
			json:    `{"content":{"type":"text","text":"hello"},"role":"user"}`,
			content: &mcp.PromptMessage{},
			want:    &mcp.PromptMessage{Content: &mcp.TextContent{Text: "hello"}, Role: "user"},
		},
		{
			name:    "SamplingMessage nil Content",
			json:    `{"content":{"type":"text","text":"hello"},"role":"user"}`,
			content: &mcp.SamplingMessage{},
			want:    &mcp.SamplingMessage{Content: &mcp.TextContent{Text: "hello"}, Role: "user"},
		},
		{
			name:    "CallToolResultFor nil Content",
			json:    `{"content":[{"type":"text","text":"hello"}]}`,
			content: &mcp.CallToolResult{},
			want:    &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that unmarshaling doesn't panic on nil Content fields
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("UnmarshalJSON panicked: %v", r)
				}
			}()

			err := json.Unmarshal([]byte(tt.json), tt.content)
			if err != nil {
				t.Errorf("UnmarshalJSON failed: %v", err)
			}

			// Verify that the Content field was properly populated
			if cmp.Diff(tt.want, tt.content, ctrCmpOpts...) != "" {
				t.Errorf("Content is not equal: %v", cmp.Diff(tt.content, tt.content))
			}
		})
	}
}

func TestContentUnmarshalNilWithDifferentTypes(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		content     any
		expectError bool
	}{
		{
			name:        "ImageContent",
			json:        `{"content":{"type":"image","mimeType":"image/png","data":"YTFiMmMz"}}`,
			content:     &mcp.CreateMessageResult{},
			expectError: false,
		},
		{
			name:        "AudioContent",
			json:        `{"content":{"type":"audio","mimeType":"audio/wav","data":"YTFiMmMz"}}`,
			content:     &mcp.CreateMessageResult{},
			expectError: false,
		},
		{
			name:        "ResourceLink",
			json:        `{"content":{"type":"resource_link","uri":"file:///test","name":"test"}}`,
			content:     &mcp.CreateMessageResult{},
			expectError: true, // CreateMessageResult only allows text, image, audio
		},
		{
			name:        "EmbeddedResource",
			json:        `{"content":{"type":"resource","resource":{"uri":"file://test","text":"test"}}}`,
			content:     &mcp.CreateMessageResult{},
			expectError: true, // CreateMessageResult only allows text, image, audio
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that unmarshaling doesn't panic on nil Content fields
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("UnmarshalJSON panicked: %v", r)
				}
			}()

			err := json.Unmarshal([]byte(tt.json), tt.content)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			// Verify that the Content field was properly populated for successful cases
			if !tt.expectError {
				if result, ok := tt.content.(*mcp.CreateMessageResult); ok {
					if result.Content == nil {
						t.Error("CreateMessageResult.Content was not populated")
					}
				}
			}
		})
	}
}

func TestContentUnmarshalNilWithEmptyContent(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		content     any
		expectError bool
	}{
		{
			name:        "Empty Content array",
			json:        `{"content":[]}`,
			content:     &mcp.CallToolResult{},
			expectError: false,
		},
		{
			name:        "Missing Content field",
			json:        `{"model":"test","role":"user"}`,
			content:     &mcp.CreateMessageResult{},
			expectError: true, // Content field is required for CreateMessageResult
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that unmarshaling doesn't panic on nil Content fields
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("UnmarshalJSON panicked: %v", r)
				}
			}()

			err := json.Unmarshal([]byte(tt.json), tt.content)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestContentUnmarshalNilWithInvalidContent(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		content     any
		expectError bool
	}{
		{
			name:        "Invalid content type",
			json:        `{"content":{"type":"invalid","text":"hello"}}`,
			content:     &mcp.CreateMessageResult{},
			expectError: true,
		},
		{
			name:        "Missing type field",
			json:        `{"content":{"text":"hello"}}`,
			content:     &mcp.CreateMessageResult{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that unmarshaling doesn't panic on nil Content fields
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("UnmarshalJSON panicked: %v", r)
				}
			}()

			err := json.Unmarshal([]byte(tt.json), tt.content)
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

var ctrCmpOpts = []cmp.Option{
	cmpopts.IgnoreUnexported(mcp.CallToolResult{}, mcp.GetPromptResult{}, mcp.ReadResourceResult{}),
	// Server responses under the >= 2026-07-28 protocol carry an auto-populated
	// [mcp.MetaKeyServerInfo] entry; tests that compare result bodies against
	// hand-crafted expected values should ignore it.
	cmpopts.IgnoreFields(mcp.CallToolResult{}, "Meta"),
	cmpopts.IgnoreFields(mcp.GetPromptResult{}, "Meta"),
	cmpopts.IgnoreFields(mcp.ReadResourceResult{}, "Meta"),
}
