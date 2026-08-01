// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestParamsMeta(t *testing.T) {
	// Verify some properties of the Meta field of Params structs.
	// We use CallToolParams for the test, but the Meta setup of all params types
	// is identical so they should all behave the same.

	toJSON := func(x any) string {
		data, err := json.Marshal(x)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	meta := map[string]any{"m": 1}

	// You can set the embedded Meta field to a literal map.
	p := &CallToolParams{
		Meta: meta,
		Name: "name",
	}

	// The Meta field marshals properly when it's present.
	if g, w := toJSON(p), `{"_meta":{"m":1},"name":"name"}`; g != w {
		t.Errorf("got %s, want %s", g, w)
	}
	// ... and when it's absent.
	p2 := &CallToolParams{Name: "n"}
	if g, w := toJSON(p2), `{"name":"n"}`; g != w {
		t.Errorf("got %s, want %s", g, w)
	}

	// The GetMeta and SetMeta functions work as expected.
	if g := p.GetMeta(); !maps.Equal(g, meta) {
		t.Errorf("got %+v, want %+v", g, meta)
	}

	meta2 := map[string]any{"x": 2}
	p.SetMeta(meta2)
	if g := p.GetMeta(); !maps.Equal(g, meta2) {
		t.Errorf("got %+v, want %+v", g, meta2)
	}

	// The GetProgressToken and SetProgressToken methods work as expected.
	if g := p.GetProgressToken(); g != nil {
		t.Errorf("got %v, want nil", g)
	}

	p.SetProgressToken("t")
	if g := p.GetProgressToken(); g != "t" {
		t.Errorf("got %v, want `t`", g)
	}

	// The GetProgressToken and SetProgressToken methods work on a params struct that doesn't have a Meta field.
	if g := p2.GetProgressToken(); g != nil {
		t.Errorf("got %v, want nil", g)
	}

	p2.SetProgressToken("t")
	if g := p2.GetProgressToken(); g != "t" {
		t.Errorf("got %v, want `t`", g)
	}

	// You can set a progress token to an int, int32 or int64.
	p.SetProgressToken(int(1))
	p.SetProgressToken(int32(1))
	p.SetProgressToken(int64(1))
}

func TestCompleteReference(t *testing.T) {
	marshalTests := []struct {
		name    string
		in      CompleteReference // The Go struct to marshal
		want    string            // The expected JSON string output
		wantErr bool              // True if json.Marshal is expected to return an error
	}{
		{
			name:    "ValidPrompt",
			in:      CompleteReference{Type: "ref/prompt", Name: "my_prompt"},
			want:    `{"type":"ref/prompt","name":"my_prompt"}`,
			wantErr: false,
		},
		{
			name:    "ValidResource",
			in:      CompleteReference{Type: "ref/resource", URI: "file:///path/to/resource.txt"},
			want:    `{"type":"ref/resource","uri":"file:///path/to/resource.txt"}`,
			wantErr: false,
		},
		{
			name:    "ValidPromptEmptyName",
			in:      CompleteReference{Type: "ref/prompt", Name: ""},
			want:    `{"type":"ref/prompt"}`,
			wantErr: false,
		},
		{
			name:    "ValidResourceEmptyURI",
			in:      CompleteReference{Type: "ref/resource", URI: ""},
			want:    `{"type":"ref/resource"}`,
			wantErr: false,
		},
		// Error cases for MarshalJSON
		{
			name:    "InvalidType",
			in:      CompleteReference{Type: "ref/unknown", Name: "something"},
			wantErr: true,
		},
		{
			name:    "PromptWithURI",
			in:      CompleteReference{Type: "ref/prompt", Name: "my_prompt", URI: "unexpected_uri"},
			wantErr: true,
		},
		{
			name:    "ResourceWithName",
			in:      CompleteReference{Type: "ref/resource", URI: "my_uri", Name: "unexpected_name"},
			wantErr: true,
		},
		{
			name:    "MissingTypeField",
			in:      CompleteReference{Name: "missing"}, // Type is ""
			wantErr: true,
		},
	}

	// Define test cases specifically for Unmarshalling
	unmarshalTests := []struct {
		name    string
		in      string            // The JSON string input
		want    CompleteReference // The expected Go struct output
		wantErr bool              // True if json.Unmarshal is expected to return an error
	}{
		{
			name:    "ValidPrompt",
			in:      `{"type":"ref/prompt","name":"my_prompt"}`,
			want:    CompleteReference{Type: "ref/prompt", Name: "my_prompt"},
			wantErr: false,
		},
		{
			name:    "ValidResource",
			in:      `{"type":"ref/resource","uri":"file:///path/to/resource.txt"}`,
			want:    CompleteReference{Type: "ref/resource", URI: "file:///path/to/resource.txt"},
			wantErr: false,
		},
		// Error cases for UnmarshalJSON
		{
			name:    "UnrecognizedType",
			in:      `{"type":"ref/unknown","name":"something"}`,
			want:    CompleteReference{}, // placeholder, as unmarshal will fail
			wantErr: true,
		},
		{
			name:    "PromptWithURI",
			in:      `{"type":"ref/prompt","name":"my_prompt","uri":"unexpected_uri"}`,
			want:    CompleteReference{}, // placeholder
			wantErr: true,
		},
		{
			name:    "ResourceWithName",
			in:      `{"type":"ref/resource","uri":"my_uri","name":"unexpected_name"}`,
			want:    CompleteReference{}, // placeholder
			wantErr: true,
		},
		{
			name:    "MissingType",
			in:      `{"name":"missing"}`,
			want:    CompleteReference{}, // placeholder
			wantErr: true,
		},
		{
			name:    "InvalidJSON",
			in:      `invalid json`,
			want:    CompleteReference{}, // placeholder
			wantErr: true,                // json.Unmarshal will fail natively
		},
	}

	// Run Marshal Tests
	for _, test := range marshalTests {
		t.Run("Marshal/"+test.name, func(t *testing.T) {
			gotBytes, err := json.Marshal(&test.in)
			if (err != nil) != test.wantErr {
				t.Errorf("json.Marshal(%v) got error %v (want error %t)", test.in, err, test.wantErr)
			}
			if !test.wantErr { // Only check JSON output if marshal was expected to succeed
				if diff := cmp.Diff(test.want, string(gotBytes)); diff != "" {
					t.Errorf("json.Marshal(%v) mismatch (-want +got):\n%s", test.in, diff)
				}
			}
		})
	}

	// Run Unmarshal Tests
	for _, test := range unmarshalTests {
		t.Run("Unmarshal/"+test.name, func(t *testing.T) {
			var got CompleteReference
			err := json.Unmarshal([]byte(test.in), &got)

			if (err != nil) != test.wantErr {
				t.Errorf("json.Unmarshal(%q) got error %v (want error %t)", test.in, err, test.wantErr)
			}
			if !test.wantErr { // Only check content if unmarshal was expected to succeed
				if diff := cmp.Diff(test.want, got); diff != "" {
					t.Errorf("json.Unmarshal(%q) mismatch (-want +got):\n%s", test.in, diff)
				}
			}
		})
	}
}

func TestCompleteParams(t *testing.T) {
	// Define test cases specifically for Marshalling
	marshalTests := []struct {
		name string
		in   CompleteParams
		want string // Expected JSON output
	}{
		{
			name: "BasicPromptCompletion",
			in: CompleteParams{
				Ref: &CompleteReference{
					Type: "ref/prompt",
					Name: "my_prompt",
				},
				Argument: CompleteParamsArgument{
					Name:  "language",
					Value: "go",
				},
			},
			want: `{"argument":{"name":"language","value":"go"},"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
		},
		{
			name: "ResourceCompletionRequest",
			in: CompleteParams{
				Ref: &CompleteReference{
					Type: "ref/resource",
					URI:  "file:///src/main.java",
				},
				Argument: CompleteParamsArgument{
					Name:  "class",
					Value: "MyClas",
				},
			},
			want: `{"argument":{"name":"class","value":"MyClas"},"ref":{"type":"ref/resource","uri":"file:///src/main.java"}}`,
		},
		{
			name: "PromptCompletionEmptyArgumentValue",
			in: CompleteParams{
				Ref: &CompleteReference{
					Type: "ref/prompt",
					Name: "another_prompt",
				},
				Argument: CompleteParamsArgument{
					Name:  "query",
					Value: "",
				},
			},
			want: `{"argument":{"name":"query","value":""},"ref":{"type":"ref/prompt","name":"another_prompt"}}`,
		},
		{
			name: "PromptCompletionWithContext",
			in: CompleteParams{
				Ref: &CompleteReference{
					Type: "ref/prompt",
					Name: "my_prompt",
				},
				Argument: CompleteParamsArgument{
					Name:  "language",
					Value: "go",
				},
				Context: &CompleteContext{
					Arguments: map[string]string{
						"framework": "mcp",
						"language":  "python",
					},
				},
			},
			want: `{"argument":{"name":"language","value":"go"},"context":{"arguments":{"framework":"mcp","language":"python"}},"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
		},
		{
			name: "PromptCompletionEmptyContextArguments",
			in: CompleteParams{
				Ref: &CompleteReference{
					Type: "ref/prompt",
					Name: "my_prompt",
				},
				Argument: CompleteParamsArgument{
					Name:  "language",
					Value: "go",
				},
				Context: &CompleteContext{
					Arguments: map[string]string{},
				},
			},
			want: `{"argument":{"name":"language","value":"go"},"context":{},"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
		},
	}

	// Define test cases specifically for Unmarshalling
	unmarshalTests := []struct {
		name string
		in   string         // JSON string input
		want CompleteParams // Expected Go struct output
	}{
		{
			name: "BasicPromptCompletion",
			in:   `{"argument":{"name":"language","value":"go"},"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
			want: CompleteParams{
				Ref:      &CompleteReference{Type: "ref/prompt", Name: "my_prompt"},
				Argument: CompleteParamsArgument{Name: "language", Value: "go"},
			},
		},
		{
			name: "ResourceCompletionRequest",
			in:   `{"argument":{"name":"class","value":"MyClas"},"ref":{"type":"ref/resource","uri":"file:///src/main.java"}}`,
			want: CompleteParams{
				Ref:      &CompleteReference{Type: "ref/resource", URI: "file:///src/main.java"},
				Argument: CompleteParamsArgument{Name: "class", Value: "MyClas"},
			},
		},
		{
			name: "PromptCompletionWithContext",
			in:   `{"argument":{"name":"language","value":"go"},"context":{"arguments":{"framework":"mcp","language":"python"}},"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
			want: CompleteParams{
				Ref:      &CompleteReference{Type: "ref/prompt", Name: "my_prompt"},
				Argument: CompleteParamsArgument{Name: "language", Value: "go"},
				Context: &CompleteContext{Arguments: map[string]string{
					"framework": "mcp",
					"language":  "python",
				}},
			},
		},
		{
			name: "PromptCompletionEmptyContextArguments",
			in:   `{"argument":{"name":"language","value":"go"},"context":{"arguments":{}},"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
			want: CompleteParams{
				Ref:      &CompleteReference{Type: "ref/prompt", Name: "my_prompt"},
				Argument: CompleteParamsArgument{Name: "language", Value: "go"},
				Context:  &CompleteContext{Arguments: map[string]string{}},
			},
		},
		{
			name: "PromptCompletionNilContext", // JSON `null` for context
			in:   `{"argument":{"name":"language","value":"go"},"context":null,"ref":{"type":"ref/prompt","name":"my_prompt"}}`,
			want: CompleteParams{
				Ref:      &CompleteReference{Type: "ref/prompt", Name: "my_prompt"},
				Argument: CompleteParamsArgument{Name: "language", Value: "go"},
				Context:  nil, // Should unmarshal to nil pointer
			},
		},
	}

	// Run Marshal Tests
	for _, test := range marshalTests {
		t.Run("Marshal/"+test.name, func(t *testing.T) {
			got, err := json.Marshal(&test.in) // Marshal takes a pointer
			if err != nil {
				t.Fatalf("json.Marshal(CompleteParams) failed: %v", err)
			}
			if diff := cmp.Diff(test.want, string(got)); diff != "" {
				t.Errorf("CompleteParams marshal mismatch (-want +got):\n%s", diff)
			}
		})
	}

	// Run Unmarshal Tests
	for _, test := range unmarshalTests {
		t.Run("Unmarshal/"+test.name, func(t *testing.T) {
			var got CompleteParams
			if err := json.Unmarshal([]byte(test.in), &got); err != nil {
				t.Fatalf("json.Unmarshal(CompleteParams) failed: %v", err)
			}
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("CompleteParams unmarshal mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCompleteResult(t *testing.T) {
	// Define test cases specifically for Marshalling
	marshalTests := []struct {
		name string
		in   CompleteResult
		want string // Expected JSON output
	}{
		{
			name: "BasicCompletionResult",
			in: CompleteResult{
				Completion: CompletionResultDetails{
					Values:  []string{"golang", "google", "goroutine"},
					Total:   10,
					HasMore: true,
				},
			},
			want: `{"completion":{"hasMore":true,"total":10,"values":["golang","google","goroutine"]}}`,
		},
		{
			name: "CompletionResultNoTotalNoHasMore",
			in: CompleteResult{
				Completion: CompletionResultDetails{
					Values:  []string{"only"},
					HasMore: false,
					Total:   0,
				},
			},
			want: `{"completion":{"values":["only"]}}`,
		},
		{
			name: "CompletionResultEmptyValues",
			in: CompleteResult{
				Completion: CompletionResultDetails{
					Values:  []string{},
					Total:   0,
					HasMore: false,
				},
			},
			want: `{"completion":{"values":[]}}`,
		},
	}

	// Define test cases specifically for Unmarshalling
	unmarshalTests := []struct {
		name string
		in   string         // JSON string input
		want CompleteResult // Expected Go struct output
	}{
		{
			name: "BasicCompletionResult",
			in:   `{"completion":{"hasMore":true,"total":10,"values":["golang","google","goroutine"]}}`,
			want: CompleteResult{
				Completion: CompletionResultDetails{
					Values:  []string{"golang", "google", "goroutine"},
					Total:   10,
					HasMore: true,
				},
			},
		},
		{
			name: "CompletionResultNoTotalNoHasMore",
			in:   `{"completion":{"values":["only"]}}`,
			want: CompleteResult{
				Completion: CompletionResultDetails{
					Values:  []string{"only"},
					HasMore: false,
					Total:   0,
				},
			},
		},
		{
			name: "CompletionResultEmptyValues",
			in:   `{"completion":{"hasMore":false,"total":0,"values":[]}}`,
			want: CompleteResult{
				Completion: CompletionResultDetails{
					Values:  []string{},
					Total:   0,
					HasMore: false,
				},
			},
		},
	}

	// Run Marshal Tests
	for _, test := range marshalTests {
		t.Run("Marshal/"+test.name, func(t *testing.T) {
			got, err := json.Marshal(&test.in) // Marshal takes a pointer
			if err != nil {
				t.Fatalf("json.Marshal(CompleteResult) failed: %v", err)
			}
			if diff := cmp.Diff(test.want, string(got)); diff != "" {
				t.Errorf("CompleteResult marshal mismatch (-want +got):\n%s", diff)
			}
		})
	}

	// Run Unmarshal Tests
	for _, test := range unmarshalTests {
		t.Run("Unmarshal/"+test.name, func(t *testing.T) {
			var got CompleteResult
			if err := json.Unmarshal([]byte(test.in), &got); err != nil {
				t.Fatalf("json.Unmarshal(CompleteResult) failed: %v", err)
			}
			if diff := cmp.Diff(test.want, got, cmpopts.IgnoreUnexported(CompleteResult{})); diff != "" {
				t.Errorf("CompleteResult unmarshal mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TODO: merge the following 4 tests into content_test.go.
func TestToolUseContent_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		content *ToolUseContent
		want    string
	}{
		{
			name: "basic tool use",
			content: &ToolUseContent{
				ID:   "tool_123",
				Name: "calculator",
				Input: map[string]any{
					"operation": "add",
					"x":         1.0,
					"y":         2.0,
				},
			},
			want: `{"type":"tool_use","id":"tool_123","name":"calculator","input":{"operation":"add","x":1,"y":2}}`,
		},
		{
			name: "nil input marshals as empty object",
			content: &ToolUseContent{
				ID:    "tool_456",
				Name:  "no_args_tool",
				Input: nil,
			},
			want: `{"type":"tool_use","id":"tool_456","name":"no_args_tool","input":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.content.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestToolUseContent_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want *ToolUseContent
	}{
		{
			name: "basic tool use",
			json: `{"type":"tool_use","id":"tool_123","name":"calculator","input":{"x":1,"y":2}}`,
			want: &ToolUseContent{
				ID:   "tool_123",
				Name: "calculator",
				Input: map[string]any{
					"x": 1.0,
					"y": 2.0,
				},
			},
		},
		{
			name: "with meta",
			json: `{"type":"tool_use","id":"t1","name":"calc","input":{"x":1},"_meta":{"requestId":"req-123"}}`,
			want: &ToolUseContent{
				ID:    "t1",
				Name:  "calc",
				Input: map[string]any{"x": 1.0},
				Meta:  Meta{"requestId": "req-123"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := &wireContent{}
			if err := json.Unmarshal([]byte(tt.json), wire); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			got, err := contentFromWire(wire, map[string]bool{"tool_use": true})
			if err != nil {
				t.Fatalf("contentFromWire() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestToolResultContent_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		content *ToolResultContent
		want    string
	}{
		{
			name: "basic tool result",
			content: &ToolResultContent{
				ToolUseID: "tool_123",
				Content:   []Content{&TextContent{Text: "42"}},
			},
			want: `{"type":"tool_result","toolUseId":"tool_123","content":[{"type":"text","text":"42"}]}`,
		},
		{
			name: "tool result with error",
			content: &ToolResultContent{
				ToolUseID: "tool_456",
				Content:   []Content{&TextContent{Text: "division by zero"}},
				IsError:   true,
			},
			want: `{"type":"tool_result","toolUseId":"tool_456","content":[{"type":"text","text":"division by zero"}],"isError":true}`,
		},
		{
			name: "tool result with structured content",
			content: &ToolResultContent{
				ToolUseID:         "tool_789",
				Content:           []Content{&TextContent{Text: `{"result": 42}`}},
				StructuredContent: map[string]any{"result": 42.0},
			},
			want: `{"type":"tool_result","toolUseId":"tool_789","content":[{"type":"text","text":"{\"result\": 42}"}],"structuredContent":{"result":42}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.content.MarshalJSON()
			if err != nil {
				t.Fatalf("MarshalJSON() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestToolResultContent_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want *ToolResultContent
	}{
		{
			name: "basic tool result",
			json: `{"type":"tool_result","toolUseId":"tool_123","content":[{"type":"text","text":"42"}],"isError":false}`,
			want: &ToolResultContent{
				ToolUseID: "tool_123",
				Content:   []Content{&TextContent{Text: "42"}},
				IsError:   false,
			},
		},
		{
			name: "image nested content",
			json: `{"type":"tool_result","toolUseId":"t1","content":[{"type":"image","mimeType":"image/png","data":"YWJj"}]}`,
			want: &ToolResultContent{
				ToolUseID: "t1",
				Content:   []Content{&ImageContent{MIMEType: "image/png", Data: []byte("abc")}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wire := &wireContent{}
			if err := json.Unmarshal([]byte(tt.json), wire); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			got, err := contentFromWire(wire, map[string]bool{"tool_result": true})
			if err != nil {
				t.Fatalf("contentFromWire() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSamplingMessage_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want *SamplingMessage
	}{
		{
			name: "tool_use content",
			json: `{"content":{"type":"tool_use","id":"tool_1","name":"calc","input":{}},"role":"assistant"}`,
			want: &SamplingMessage{
				Role: "assistant",
				Content: &ToolUseContent{
					ID:    "tool_1",
					Name:  "calc",
					Input: map[string]any{},
				},
			},
		},
		{
			name: "tool_result content",
			json: `{"content":{"type":"tool_result","toolUseId":"tool_1","content":[{"type":"text","text":"42"}]},"role":"user"}`,
			want: &SamplingMessage{
				Role: "user",
				Content: &ToolResultContent{
					ToolUseID: "tool_1",
					Content:   []Content{&TextContent{Text: "42"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got SamplingMessage
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, &got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSamplingCapabilities_MarshalUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		caps *SamplingCapabilities
		json string
	}{
		{
			name: "WithCapabilities",
			caps: &SamplingCapabilities{
				Tools:   &SamplingToolsCapabilities{},
				Context: &SamplingContextCapabilities{},
			},
			json: `{"context":{},"tools":{}}`,
		},
		{
			name: "EmptyCapabilities",
			caps: &SamplingCapabilities{},
			json: `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJson, err := json.Marshal(tt.caps)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.json, string(gotJson)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
			var gotCaps SamplingCapabilities
			if err := json.Unmarshal([]byte(tt.json), &gotCaps); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.caps, &gotCaps); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateMessageWithToolsParams_MarshalUnmarshalJSON(t *testing.T) {
	params := &CreateMessageWithToolsParams{
		MaxTokens: 1000,
		Messages: []*SamplingMessageV2{
			{
				Role:    "user",
				Content: []Content{&TextContent{Text: "Calculate 1+1"}},
			},
		},
		Tools: []*Tool{
			{
				Name:        "calculator",
				Description: "A calculator tool",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"x": map[string]any{"type": "number"},
						"y": map[string]any{"type": "number"},
					},
				},
			},
		},
		ToolChoice: &ToolChoice{Mode: "auto"},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var got CreateMessageWithToolsParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if diff := cmp.Diff(params, &got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestToolChoice_MarshalUnmarshalJSON(t *testing.T) {
	choices := []ToolChoice{
		{Mode: "auto"},
		{Mode: "required"},
		{Mode: "none"},
	}

	for _, tc := range choices {
		data, err := json.Marshal(tc)
		if err != nil {
			t.Fatalf("Marshal(%v) error = %v", tc.Mode, err)
		}
		var got ToolChoice
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}
		if diff := cmp.Diff(tc, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	}
}

func TestCreateMessageWithToolsResult_MarshalJSON(t *testing.T) {
	tests := []struct {
		name   string
		result *CreateMessageWithToolsResult
		want   string
	}{
		{
			name: "single element content",
			result: &CreateMessageWithToolsResult{
				Model:   "test",
				Role:    "assistant",
				Content: []Content{&TextContent{Text: "hello"}},
			},
			// Single-element Content marshals as object (not array) for backward compat.
			want: `{"content":{"type":"text","text":"hello"},"model":"test","role":"assistant"}`,
		},
		{
			name: "multiple elements content",
			result: &CreateMessageWithToolsResult{
				Model: "test",
				Role:  "assistant",
				Content: []Content{
					&TextContent{Text: "thinking..."},
					&ToolUseContent{
						ID:   "call_1",
						Name: "calculator",
						Input: map[string]any{
							"a": 1.0,
							"b": 2.0,
						},
					},
				},
			},
			// Multiple elements marshal as array.
			want: `{"content":[{"type":"text","text":"thinking..."},{"type":"tool_use","id":"call_1","name":"calculator","input":{"a":1,"b":2}}],"model":"test","role":"assistant"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.result)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCreateMessageWithToolsResult_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    *CreateMessageWithToolsResult
		wantErr bool
	}{
		{
			name: "single tool_use content",
			json: `{"content":{"type":"tool_use","id":"tool_1","name":"calculator","input":{"x":1}},"model":"test-model","role":"assistant","stopReason":"toolUse"}`,
			want: &CreateMessageWithToolsResult{
				Model:      "test-model",
				Role:       "assistant",
				StopReason: "toolUse",
				Content: []Content{
					&ToolUseContent{
						ID:   "tool_1",
						Name: "calculator",
						Input: map[string]any{
							"x": 1.0,
						},
					},
				},
			},
		},
		{
			name: "array of tool_use content",
			json: `{"content":[{"type":"tool_use","id":"t1","name":"calc","input":{"x":1}},{"type":"tool_use","id":"t2","name":"search","input":{"q":"hi"}}],"model":"test","role":"assistant","stopReason":"toolUse"}`,
			want: &CreateMessageWithToolsResult{
				Model:      "test",
				Role:       "assistant",
				StopReason: "toolUse",
				Content: []Content{
					&ToolUseContent{
						ID:    "t1",
						Name:  "calc",
						Input: map[string]any{"x": 1.0},
					},
					&ToolUseContent{
						ID:    "t2",
						Name:  "search",
						Input: map[string]any{"q": "hi"},
					},
				},
			},
		},
		{
			name: "empty array",
			json: `{"content":[],"model":"m","role":"assistant"}`,
			want: &CreateMessageWithToolsResult{
				Model:   "m",
				Role:    "assistant",
				Content: []Content{},
			},
		},
		{
			name:    "null content",
			json:    `{"content":null,"model":"m","role":"assistant"}`,
			wantErr: true,
		},
		{
			name:    "rejects tool_result",
			json:    `{"content":{"type":"tool_result","toolUseId":"t1","content":[]},"model":"m","role":"assistant"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got CreateMessageWithToolsResult
			err := json.Unmarshal([]byte(tt.json), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, &got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSamplingMessageV2_MarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		msg  *SamplingMessageV2
		want string
	}{
		{
			name: "empty content",
			msg: &SamplingMessageV2{
				Role:    "user",
				Content: []Content{},
			},
			want: `{"content":[],"role":"user"}`,
		},
		{
			name: "single content",
			msg: &SamplingMessageV2{
				Role:    "assistant",
				Content: []Content{&TextContent{Text: "hello"}},
			},
			want: `{"content":{"type":"text","text":"hello"},"role":"assistant"}`,
		},
		{
			name: "multiple content",
			msg: &SamplingMessageV2{
				Role: "assistant",
				// Text + tool_use in the same message (valid per spec for assistant).
				Content: []Content{
					&TextContent{Text: "checking weather"},
					&ToolUseContent{ID: "c1", Name: "weather", Input: map[string]any{"city": "SF"}},
				},
			},
			want: `{"content":[{"type":"text","text":"checking weather"},{"type":"tool_use","id":"c1","name":"weather","input":{"city":"SF"}}],"role":"assistant"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSamplingMessageV2_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		json string
		want *SamplingMessageV2
	}{
		{
			name: "single content object",
			json: `{"role":"user","content":{"type":"text","text":"hello"}}`,
			want: &SamplingMessageV2{
				Role: "user",
				Content: []Content{
					&TextContent{Text: "hello"},
				},
			},
		},
		{
			name: "multiple content",
			json: `{"role":"assistant","content":[{"type":"text","text":"Let me check the weather."},{"type":"tool_use","id":"c1","name":"weather","input":{"city":"SF"}}]}`,
			want: &SamplingMessageV2{
				Role: "assistant",
				// Text + tool_use in the same message (valid per spec for assistant).
				Content: []Content{
					&TextContent{Text: "Let me check the weather."},
					&ToolUseContent{ID: "c1", Name: "weather", Input: map[string]any{"city": "SF"}},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got SamplingMessageV2
			if err := json.Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, &got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestToBase_Conversion(t *testing.T) {
	tests := []struct {
		name    string
		params  *CreateMessageWithToolsParams
		want    *CreateMessageParams
		wantErr bool
	}{
		{
			name: "Single content",
			params: &CreateMessageWithToolsParams{
				MaxTokens: 1000,
				Messages: []*SamplingMessageV2{
					{Role: "user", Content: []Content{&TextContent{Text: "hello"}}},
				},
				Tools:      []*Tool{{Name: "calc"}},
				ToolChoice: &ToolChoice{Mode: "auto"},
			},
			want: &CreateMessageParams{
				MaxTokens: 1000,
				Messages: []*SamplingMessage{
					{Role: "user", Content: &TextContent{Text: "hello"}},
				},
			},
		},
		{
			name: "Multi content",
			params: &CreateMessageWithToolsParams{
				MaxTokens: 1000,
				Messages: []*SamplingMessageV2{
					{Role: "assistant", Content: []Content{
						&ToolUseContent{ID: "c1", Name: "calc", Input: map[string]any{}},
						&ToolUseContent{ID: "c2", Name: "search", Input: map[string]any{}},
					}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.params.toBase()
			if (err != nil) != tt.wantErr {
				t.Fatalf("toBase() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestToWithTools_Conversion(t *testing.T) {
	tests := []struct {
		name   string
		result *CreateMessageResult
		want   *CreateMessageWithToolsResult
	}{
		{
			name: "with content",
			result: &CreateMessageResult{
				Model:      "test",
				Role:       "assistant",
				Content:    &TextContent{Text: "hello"},
				StopReason: "endTurn",
			},
			want: &CreateMessageWithToolsResult{
				Model:      "test",
				Role:       "assistant",
				Content:    []Content{&TextContent{Text: "hello"}},
				StopReason: "endTurn",
			},
		},
		{
			name: "nil content",
			result: &CreateMessageResult{
				Model: "test",
				Role:  "assistant",
			},
			want: &CreateMessageWithToolsResult{
				Model: "test",
				Role:  "assistant",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wt := tt.result.toWithTools()
			if diff := cmp.Diff(tt.want, wt); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInputRequestMapJSON(t *testing.T) {
	t.Run("nil is omitted from JSON", func(t *testing.T) {
		result := CallToolResult{Content: []Content{&TextContent{Text: "ok"}}}
		data, err := json.Marshal(&result)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		if _, ok := raw["inputRequests"]; ok {
			t.Errorf("nil InputRequests should be omitted, got %s", raw["inputRequests"])
		}
	})

	t.Run("non-nil empty round-trips", func(t *testing.T) {
		result := CallToolResult{
			Content:       []Content{&TextContent{Text: "ok"}},
			InputRequests: InputRequestMap{},
		}
		data, err := json.Marshal(&result)
		if err != nil {
			t.Fatal(err)
		}
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		if string(raw["inputRequests"]) != "{}" {
			t.Errorf("empty InputRequests should marshal to {}, got %s", raw["inputRequests"])
		}
		var got CallToolResult
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.InputRequests == nil {
			t.Error("empty InputRequests should round-trip as non-nil")
		}
	})

	t.Run("populated round-trips", func(t *testing.T) {
		result := CallToolResult{
			Content: []Content{&TextContent{Text: "ok"}},
			InputRequests: InputRequestMap{
				"r1": &ElicitParams{Message: "confirm?"},
			},
		}
		data, err := json.Marshal(&result)
		if err != nil {
			t.Fatal(err)
		}
		var got CallToolResult
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.InputRequests == nil {
			t.Fatal("InputRequests should not be nil after round-trip")
		}
		if _, ok := got.InputRequests["r1"]; !ok {
			t.Error("expected key r1 in InputRequests")
		}
	})
}

func TestInputResponseMapJSON(t *testing.T) {
	tests := []struct {
		name  string
		value InputResponse
		check func(t *testing.T, got InputResponse)
	}{
		{
			name:  "elicit",
			value: &ElicitResult{Action: "accept", Content: map[string]any{"ok": true}},
		},
		{
			name:  "sampling",
			value: &CreateMessageWithToolsResult{Role: "assistant", Model: "test-model", Content: []Content{&TextContent{Text: "hello"}}},
		},
		{
			name:  "list-roots",
			value: &ListRootsResult{Roots: []*Root{{URI: "file:///tmp", Name: "tmp"}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := tt.name
			data, err := json.Marshal(InputResponseMap{key: tt.value})
			if err != nil {
				t.Fatal(err)
			}
			var got InputResponseMap
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.value, got[key], ctrCmpOpts...); diff != "" {
				t.Errorf("mismatch (-want, +got):\n%s", diff)
			}
		})
	}
}

func TestContentUnmarshal(t *testing.T) {
	// Verify that types with a Content field round-trip properly.
	roundtrip := func(in, out any) {
		t.Helper()
		data, err := json.Marshal(in)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(in, out, ctrCmpOpts...); diff != "" {
			t.Errorf("mismatch (-want, +got):\n%s", diff)
		}
	}

	content := []Content{&TextContent{Text: "t"}}

	ctr := &CallToolResult{
		Meta:              Meta{"m": true},
		Content:           content,
		IsError:           true,
		StructuredContent: map[string]any{"s": "x"},
	}
	var got CallToolResult
	roundtrip(ctr, &got)

	ctrf := &CallToolResult{
		Meta:              Meta{"m": true},
		Content:           content,
		IsError:           true,
		StructuredContent: 3.0,
	}
	var gotf CallToolResult
	roundtrip(ctrf, &gotf)

	pm := &PromptMessage{
		Content: content[0],
		Role:    "",
	}
	var gotpm PromptMessage
	roundtrip(pm, &gotpm)
}

func TestToolAnnotations_MarshalJSON(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	tests := []struct {
		name string
		in   ToolAnnotations
		want string
	}{
		{
			name: "ZeroValue",
			in:   ToolAnnotations{},
			want: `{"idempotentHint":false,"readOnlyHint":false}`,
		},
		{
			name: "AllFalse",
			in: ToolAnnotations{
				DestructiveHint: boolPtr(false),
				IdempotentHint:  false,
				OpenWorldHint:   boolPtr(false),
				ReadOnlyHint:    false,
			},
			want: `{"destructiveHint":false,"idempotentHint":false,"openWorldHint":false,"readOnlyHint":false}`,
		},
		{
			name: "AllTrue",
			in: ToolAnnotations{
				DestructiveHint: boolPtr(true),
				IdempotentHint:  true,
				OpenWorldHint:   boolPtr(true),
				ReadOnlyHint:    true,
				Title:           "my tool",
			},
			want: `{"destructiveHint":true,"idempotentHint":true,"openWorldHint":true,"readOnlyHint":true,"title":"my tool"}`,
		},
		{
			name: "MixedValues",
			in: ToolAnnotations{
				ReadOnlyHint: true,
				Title:        "read tool",
			},
			want: `{"idempotentHint":false,"readOnlyHint":true,"title":"read tool"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("json.Marshal(%v) failed: %v", tt.in, err)
			}
			if diff := cmp.Diff(tt.want, string(got)); diff != "" {
				t.Errorf("json.Marshal() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
