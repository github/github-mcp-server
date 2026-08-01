// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//lint:file-ignore SA1019 tests exercise deprecated SEP-2577 sampling APIs.

package mcp

// TODO: move other sampling-related tests to this file.

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSamplingWithTools_ToolUse(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	// Track what the client received
	var gotParams *CreateMessageWithToolsParams
	result := &CreateMessageWithToolsResult{
		Model: "test-model",
		Role:  "assistant",
		Content: []Content{
			&ToolUseContent{
				ID:    "tool_call_1",
				Name:  "calculator",
				Input: map[string]any{"x": 1.0, "y": 2.0},
			},
		},
		StopReason: "toolUse",
	}

	// Client with tools capability, using CreateMessageWithToolsHandler
	client := NewClient(testImpl, &ClientOptions{
		CreateMessageWithToolsHandler: func(_ context.Context, req *CreateMessageWithToolsRequest) (*CreateMessageWithToolsResult, error) {
			gotParams = req.Params
			return result, nil
		},
		Capabilities: &ClientCapabilities{
			Sampling: &SamplingCapabilities{Tools: &SamplingToolsCapabilities{}},
		},
	})

	server := NewServer(testImpl, nil)
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Server sends CreateMessageWithTools
	params := &CreateMessageWithToolsParams{
		MaxTokens: 1000,
		Messages: []*SamplingMessageV2{
			{Role: "user", Content: []Content{&TextContent{Text: "Calculate 1+2"}}},
		},
		Tools: []*Tool{
			{
				Name:        "calculator",
				Description: "A calculator",
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
	gotResult, err := ss.CreateMessageWithTools(ctx, params)
	if err != nil {
		t.Fatalf("CreateMessageWithTools() error = %v", err)
	}

	// Verify client received the params
	if diff := cmp.Diff(params, gotParams); diff != "" {
		t.Errorf("CreateMessageWithToolsParams mismatch (-want +got):\n%s", diff)
	}

	// Verify server received the tool use response
	if diff := cmp.Diff(result, gotResult); diff != "" {
		t.Errorf("CreateMessageWithToolsResult mismatch (-want +got):\n%s", diff)
	}
}

func TestSamplingWithTools_ToolResult(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	// Track messages received by client
	var gotParams *CreateMessageParams

	client := NewClient(testImpl, &ClientOptions{
		CreateMessageHandler: func(_ context.Context, req *CreateMessageRequest) (*CreateMessageResult, error) {
			gotParams = req.Params
			return &CreateMessageResult{
				Model:   "test-model",
				Role:    "assistant",
				Content: &TextContent{Text: "The result is 3"},
			}, nil
		},
		Capabilities: &ClientCapabilities{
			Sampling: &SamplingCapabilities{Tools: &SamplingToolsCapabilities{}},
		},
	})

	server := NewServer(testImpl, nil)
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	params := &CreateMessageParams{
		MaxTokens: 1000,
		Messages: []*SamplingMessage{
			{Role: "user", Content: &TextContent{Text: "Calculate 1+2"}},
			{Role: "assistant", Content: &ToolUseContent{
				ID:    "tool_1",
				Name:  "calculator",
				Input: map[string]any{"x": 1.0, "y": 2.0},
			}},
			{Role: "user", Content: &ToolResultContent{
				ToolUseID: "tool_1",
				Content:   []Content{&TextContent{Text: "3"}},
			}},
		},
	}
	_, err = ss.CreateMessage(ctx, params)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if diff := cmp.Diff(params, gotParams); diff != "" {
		t.Errorf("CreateMessageParams mismatch (-want +got):\n%s", diff)
	}
}

func TestSamplingToolsCapabilities(t *testing.T) {
	ctx := context.Background()

	t.Run("client with explicit tools capability", func(t *testing.T) {
		ct, st := NewInMemoryTransports()

		client := NewClient(testImpl, &ClientOptions{
			CreateMessageHandler: func(_ context.Context, _ *CreateMessageRequest) (*CreateMessageResult, error) {
				return &CreateMessageResult{Model: "m", Content: &TextContent{}}, nil
			},
			Capabilities: &ClientCapabilities{
				Sampling: &SamplingCapabilities{
					Tools:   &SamplingToolsCapabilities{},
					Context: &SamplingContextCapabilities{},
				},
			},
		})

		server := NewServer(testImpl, nil)
		ss, err := server.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Check server sees client capabilities
		caps := ss.InitializeParams().Capabilities
		want := &SamplingCapabilities{
			Tools:   &SamplingToolsCapabilities{},
			Context: &SamplingContextCapabilities{},
		}
		if diff := cmp.Diff(want, caps.Sampling); diff != "" {
			t.Errorf("SamplingCapabilities mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("client without tools capability", func(t *testing.T) {
		ct, st := NewInMemoryTransports()

		client := NewClient(testImpl, &ClientOptions{
			CreateMessageHandler: func(_ context.Context, _ *CreateMessageRequest) (*CreateMessageResult, error) {
				return &CreateMessageResult{Model: "m", Content: &TextContent{}}, nil
			},
			// No Capabilities.Sampling.Tools set
		})

		server := NewServer(testImpl, nil)
		ss, err := server.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		// Check server sees client capabilities
		caps := ss.InitializeParams().Capabilities
		want := &SamplingCapabilities{}
		if diff := cmp.Diff(want, caps.Sampling); diff != "" {
			t.Errorf("SamplingCapabilities mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("client with inferred tools capability", func(t *testing.T) {
		ct, st := NewInMemoryTransports()

		client := NewClient(testImpl, &ClientOptions{
			CreateMessageWithToolsHandler: func(_ context.Context, _ *CreateMessageWithToolsRequest) (*CreateMessageWithToolsResult, error) {
				return &CreateMessageWithToolsResult{Model: "m", Content: []Content{&TextContent{}}}, nil
			},
			// No explicit Capabilities set — tools should be inferred.
		})

		server := NewServer(testImpl, nil)
		ss, err := server.Connect(ctx, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer ss.Close()

		cs, err := client.Connect(ctx, ct, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer cs.Close()

		caps := ss.InitializeParams().Capabilities
		want := &SamplingCapabilities{
			Tools: &SamplingToolsCapabilities{},
		}
		if diff := cmp.Diff(want, caps.Sampling); diff != "" {
			t.Errorf("SamplingCapabilities mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestSamplingWithTools_ToolResultWithError(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	var gotParams *CreateMessageParams

	client := NewClient(testImpl, &ClientOptions{
		CreateMessageHandler: func(_ context.Context, req *CreateMessageRequest) (*CreateMessageResult, error) {
			gotParams = req.Params
			return &CreateMessageResult{
				Model:   "test-model",
				Role:    "assistant",
				Content: &TextContent{Text: "I see the tool failed"},
			}, nil
		},
		Capabilities: &ClientCapabilities{
			Sampling: &SamplingCapabilities{Tools: &SamplingToolsCapabilities{}},
		},
	})

	server := NewServer(testImpl, nil)
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	params := &CreateMessageParams{
		MaxTokens: 1000,
		Messages: []*SamplingMessage{
			{Role: "user", Content: &TextContent{Text: "Divide 1 by 0"}},
			{Role: "assistant", Content: &ToolUseContent{
				ID:    "tool_1",
				Name:  "calculator",
				Input: map[string]any{"op": "div", "x": 1.0, "y": 0.0},
			}},
			{Role: "user", Content: &ToolResultContent{
				ToolUseID: "tool_1",
				Content:   []Content{&TextContent{Text: "division by zero"}},
				IsError:   true,
			}},
		},
	}
	_, err = ss.CreateMessage(ctx, params)
	if err != nil {
		t.Fatalf("CreateMessage() error = %v", err)
	}

	if diff := cmp.Diff(params, gotParams); diff != "" {
		t.Errorf("CreateMessageParams mismatch (-want +got):\n%s", diff)
	}
}

func TestSamplingWithTools_ParallelToolCalls(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	result := &CreateMessageWithToolsResult{
		Model: "test-model",
		Role:  "assistant",
		Content: []Content{
			&ToolUseContent{ID: "call_1", Name: "weather", Input: map[string]any{"city": "SF"}},
			&ToolUseContent{ID: "call_2", Name: "weather", Input: map[string]any{"city": "NY"}},
		},
		StopReason: "toolUse",
	}
	// Client returns parallel tool use results
	client := NewClient(testImpl, &ClientOptions{
		CreateMessageWithToolsHandler: func(_ context.Context, req *CreateMessageWithToolsRequest) (*CreateMessageWithToolsResult, error) {
			return result, nil
		},
		Capabilities: &ClientCapabilities{
			Sampling: &SamplingCapabilities{Tools: &SamplingToolsCapabilities{}},
		},
	})

	server := NewServer(testImpl, nil)
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	gotResult, err := ss.CreateMessageWithTools(ctx, &CreateMessageWithToolsParams{
		MaxTokens: 1000,
		Messages: []*SamplingMessageV2{
			{Role: "user", Content: []Content{&TextContent{Text: "Weather in SF and NY"}}},
		},
		Tools: []*Tool{
			{Name: "weather", InputSchema: map[string]any{"type": "object"}},
		},
	})
	if err != nil {
		t.Fatalf("CreateMessageWithTools() error = %v", err)
	}

	if diff := cmp.Diff(result, gotResult); diff != "" {
		t.Errorf("CreateMessageWithToolsResult mismatch (-want +got):\n%s", diff)
	}
}

func TestNewClient_BothHandlersPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when both handlers set")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "CreateMessageHandler") {
			t.Errorf("unexpected panic: %v", r)
		}
	}()
	NewClient(testImpl, &ClientOptions{
		CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
			return nil, nil
		},
		CreateMessageWithToolsHandler: func(context.Context, *CreateMessageWithToolsRequest) (*CreateMessageWithToolsResult, error) {
			return nil, nil
		},
	})
}

func TestCreateMessage_MultipleContentError(t *testing.T) {
	ctx := context.Background()
	ct, st := NewInMemoryTransports()

	// Client returns multiple content blocks via CreateMessageWithToolsHandler
	client := NewClient(testImpl, &ClientOptions{
		CreateMessageWithToolsHandler: func(_ context.Context, _ *CreateMessageWithToolsRequest) (*CreateMessageWithToolsResult, error) {
			return &CreateMessageWithToolsResult{
				Model: "test",
				Role:  "assistant",
				Content: []Content{
					&TextContent{Text: "a"},
					&TextContent{Text: "b"},
				},
			}, nil
		},
	})

	server := NewServer(testImpl, nil)
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ss.Close()

	cs, err := client.Connect(ctx, ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()

	// Server calls CreateMessage (singular), should get error
	_, err = ss.CreateMessage(ctx, &CreateMessageParams{
		MaxTokens: 100,
		Messages:  []*SamplingMessage{{Role: "user", Content: &TextContent{Text: "hi"}}},
	})
	if err == nil {
		t.Fatal("expected error for multiple content blocks")
	}
	if !strings.Contains(err.Error(), "CreateMessageWithTools") {
		t.Errorf("error should mention CreateMessageWithTools, got: %v", err)
	}
}

func TestClientCapabilities_CloneSampling(t *testing.T) {
	caps := &ClientCapabilities{
		Sampling: &SamplingCapabilities{
			Tools:   &SamplingToolsCapabilities{},
			Context: &SamplingContextCapabilities{},
		},
	}
	cloned := caps.clone()

	// Verify deep copy — Sampling pointer should differ.
	// (Tools and Context are empty structs, so Go may reuse the same address;
	// we just check they're non-nil and that mutating Sampling doesn't alias.)
	if cloned.Sampling == caps.Sampling {
		t.Error("Sampling pointer should differ after clone")
	}
	if cloned.Sampling.Tools == nil {
		t.Error("cloned Sampling.Tools should not be nil")
	}
	if cloned.Sampling.Context == nil {
		t.Error("cloned Sampling.Context should not be nil")
	}
	// Verify mutation doesn't affect original.
	cloned.Sampling.Tools = nil
	if caps.Sampling.Tools == nil {
		t.Error("modifying cloned Sampling.Tools should not affect original")
	}
}
