// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

//lint:file-ignore SA1019 tests exercise deprecated SEP-2577 APIs (roots, sampling, logging).

package mcp

import (
	"context"
	"fmt"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/jsonschema-go/jsonschema"
)

func TestMultiRoundTrip_ManualRetry(t *testing.T) {
	type deployResult struct {
		Deployed bool   `json:"deployed"`
		Reason   string `json:"reason,omitempty"`
	}

	ctx := context.Background()

	srv := NewServer(testImpl, nil)
	AddTool(srv, &Tool{Name: "deploy"}, func(ctx context.Context, req *CallToolRequest, input struct{}) (*CallToolResult, *deployResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &CallToolResult{
				InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Deploy to production?"}},
				RequestState:  "deployment-123",
			}, nil, nil
		}

		resp, ok := req.Params.InputResponses["confirm"]
		if !ok {
			return &CallToolResult{
				InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Please confirm (retry)"}},
			}, nil, nil
		}

		if req.Params.RequestState == "" {
			return &CallToolResult{}, &deployResult{Deployed: false, Reason: "no_state"}, nil
		}
		if elicitResult := resp.(*ElicitResult); elicitResult != nil && elicitResult.Action != "accept" {
			return &CallToolResult{}, &deployResult{Deployed: false, Reason: "cancelled"}, nil
		}

		return &CallToolResult{}, &deployResult{Deployed: true}, nil
	})

	conn := mustConnect(t, srv, &ClientOptions{
		MultiRoundTrip: &MultiRoundTripOptions{Disabled: true},
	})

	// Round 1: initiate deployment
	res, err := conn.CallTool(ctx, &CallToolParams{Name: "deploy"})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if !res.NeedsInput() {
		t.Fatal("NeedsInput() = false, want true")
	}
	if got := len(res.InputRequests); got != 1 {
		t.Fatalf("len(res.InputRequests) = %d, want 1", got)
	}
	if _, ok := res.InputRequests["confirm"].(*ElicitParams); !ok {
		t.Fatalf("res.InputRequests[confirm] type = %T, want *ElicitParams", res.InputRequests["confirm"])
	}

	// Round 2: retry with confirmation
	res, err = conn.CallTool(ctx, &CallToolParams{
		Name: "deploy",
		InputResponses: InputResponseMap{
			"confirm": &ElicitResult{Action: "accept", Content: map[string]any{"ok": true}},
		},
		RequestState: res.RequestState,
	})
	if err != nil {
		t.Fatalf("CallTool() follow-up error = %v", err)
	}
	if res.NeedsInput() {
		t.Fatal("NeedsInput() = true after follow-up, want false")
	}

	if diff := cmp.Diff(map[string]any{"deployed": true}, res.StructuredContent, ctrCmpOpts...); diff != "" {
		t.Errorf("result mismatch (-want +got):\n%s", diff)
	}
}

func TestMultiRoundTrip_AutoRetry(t *testing.T) {

	tests := []struct {
		name          string
		inputRequests InputRequestMap
		wantResult    map[string]any
	}{
		{
			name: "elicit",
			inputRequests: InputRequestMap{
				"confirm": &ElicitParams{Message: "Deploy?"},
			},
			wantResult: map[string]any{"ids": []any{"confirm"}},
		},
		{
			name: "createMessage",
			inputRequests: InputRequestMap{
				"summarize": &CreateMessageParams{
					Messages:  []*SamplingMessage{{Role: "user", Content: &TextContent{Text: "summarize"}}},
					MaxTokens: 100,
				},
			},
			wantResult: map[string]any{"ids": []any{"summarize"}},
		},
		{
			name: "listRoots",
			inputRequests: InputRequestMap{
				"roots": &ListRootsParams{},
			},
			wantResult: map[string]any{"ids": []any{"roots"}},
		},
		{
			name: "all three",
			inputRequests: InputRequestMap{
				"confirm": &ElicitParams{Message: "OK?"},
				"draft": &CreateMessageParams{
					Messages:  []*SamplingMessage{{Role: "user", Content: &TextContent{Text: "write"}}},
					MaxTokens: 50,
				},
				"roots": &ListRootsParams{},
			},
			wantResult: map[string]any{"ids": []any{"confirm", "draft", "roots"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			srv := NewServer(testImpl, nil)
			inputRequests := tt.inputRequests
			AddTool(srv, &Tool{Name: "act"}, func(ctx context.Context, req *CallToolRequest, input struct{}) (*CallToolResult, any, error) {
				if len(req.Params.InputResponses) == 0 {
					return &CallToolResult{
						InputRequests: inputRequests,
						RequestState:  "state-1",
					}, nil, nil
				}
				// Collect the IDs of fulfilled responses.
				var ids []string
				for id := range req.Params.InputResponses {
					ids = append(ids, id)
				}
				slices.Sort(ids)
				return &CallToolResult{}, map[string]any{"ids": ids}, nil
			})

			conn := mustConnect(t, srv, &ClientOptions{
				ElicitationHandler: func(_ context.Context, req *ElicitRequest) (*ElicitResult, error) {
					return &ElicitResult{Action: "accept"}, nil
				},
				CreateMessageHandler: func(_ context.Context, req *CreateMessageRequest) (*CreateMessageResult, error) {
					return &CreateMessageResult{
						Model:   "test-model",
						Role:    "assistant",
						Content: &TextContent{Text: "response"},
					}, nil
				},
			})
			conn.client.AddRoots(&Root{URI: "file:///workspace", Name: "workspace"})

			res, err := conn.CallTool(ctx, &CallToolParams{Name: "act"})
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if res.NeedsInput() {
				t.Fatal("NeedsInput() = true after auto-retry, want false")
			}

			// Sort the expected IDs for stable comparison.
			if wantIDs, ok := tt.wantResult["ids"].([]any); ok {
				slices.SortFunc(wantIDs, func(a, b any) int {
					if a.(string) < b.(string) {
						return -1
					}
					return 1
				})
			}

			if diff := cmp.Diff(tt.wantResult, res.StructuredContent, ctrCmpOpts...); diff != "" {
				t.Errorf("result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestMultiRoundTrip_MaxRetries(t *testing.T) {
	testCases := []struct {
		name        string
		requests    InputRequestMap
		wantRetries int
	}{
		{
			name:        "load shedding",
			requests:    InputRequestMap{},
			wantRetries: maxLoadSheddingMultiRoundTripRetries,
		},
		{
			name:        "input request",
			requests:    InputRequestMap{"confirm": &ElicitParams{Message: "Again?"}},
			wantRetries: maxMultiRoundTripRetries,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			var serverCalls atomic.Int32
			srv := NewServer(testImpl, nil)
			AddTool(srv, &Tool{Name: "loop"}, func(ctx context.Context, req *CallToolRequest, input struct{}) (*CallToolResult, any, error) {
				serverCalls.Add(1)
				return &CallToolResult{InputRequests: tc.requests, RequestState: "loop-state"}, nil, nil
			})

			conn := mustConnect(t, srv, &ClientOptions{
				ElicitationHandler: func(_ context.Context, req *ElicitRequest) (*ElicitResult, error) {
					return &ElicitResult{Action: "accept"}, nil
				},
			})

			_, err := conn.CallTool(ctx, &CallToolParams{Name: "loop"})
			if err == nil {
				t.Fatal("CallTool() err = nil, want error for exceeded max retries")
			}
			if serverCalls.Load() != int32(tc.wantRetries) {
				t.Errorf("serverCalls = %d, want %d", serverCalls.Load(), tc.wantRetries)
			}
		})
	}
}

func TestMultiRoundTrip_ServerMiddleware(t *testing.T) {
	// multiRoundTripToolHandler returns a ToolHandler (plain, non-generic) that requests
	// the given inputRequests on the first call and returns the fulfilled
	// response IDs on the second.
	multiRoundTripToolHandler := func(inputRequests InputRequestMap) ToolHandler {
		return func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			if len(req.Params.InputResponses) == 0 {
				return &CallToolResult{
					InputRequests: inputRequests,
					RequestState:  "state-1",
				}, nil
			}
			var ids []string
			for id := range req.Params.InputResponses {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			content := &TextContent{Text: fmt.Sprintf("%v", ids)}
			return &CallToolResult{Content: []Content{content}}, nil
		}
	}

	tests := []struct {
		name          string
		inputRequests InputRequestMap
		wantText      string
	}{
		{
			name: "elicit via ToolHandler",
			inputRequests: InputRequestMap{
				"confirm": &ElicitParams{Message: "Sure?"},
			},
			wantText: "[confirm]",
		},
		{
			name: "elicit and listRoots via ToolHandler",
			inputRequests: InputRequestMap{
				"confirm": &ElicitParams{Message: "OK?"},
				"roots":   &ListRootsParams{},
			},
			wantText: "[confirm roots]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			srv := NewServer(testImpl, nil)
			srv.AddTool(
				&Tool{Name: "act", InputSchema: &jsonschema.Schema{Type: "object"}},
				multiRoundTripToolHandler(tt.inputRequests),
			)

			// Connect with an OLD protocol version where multi-round-trip is not supported.
			// The server middleware should handle it transparently.
			st, ct := NewInMemoryTransports()
			ss, err := srv.Connect(t.Context(), st, nil)
			if err != nil {
				t.Fatalf("server.Connect() error = %v", err)
			}
			t.Cleanup(func() { _ = ss.Close() })

			c := NewClient(testImpl, &ClientOptions{
				MultiRoundTrip: &MultiRoundTripOptions{Disabled: true},
				ElicitationHandler: func(_ context.Context, req *ElicitRequest) (*ElicitResult, error) {
					return &ElicitResult{Action: "accept"}, nil
				},
			})
			c.AddRoots(&Root{URI: "file:///workspace", Name: "workspace"})
			cs, err := c.Connect(t.Context(), ct, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
			if err != nil {
				t.Fatalf("client.Connect() error = %v", err)
			}
			t.Cleanup(func() { _ = cs.Close() })

			res, err := cs.CallTool(ctx, &CallToolParams{Name: "act"})
			if err != nil {
				t.Fatalf("CallTool() error = %v", err)
			}
			if got := res.Content[0].(*TextContent).Text; got != tt.wantText {
				t.Errorf("result text = %q, want %q", got, tt.wantText)
			}
		})
	}
}

func TestMultiRoundTrip_GetPrompt_AutoRetry(t *testing.T) {

	ctx := context.Background()

	srv := NewServer(testImpl, nil)
	srv.AddPrompt(&Prompt{Name: "review"}, func(_ context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &GetPromptResult{
				InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Include sensitive data?"}},
				RequestState:  "prompt-state",
			}, nil
		}
		return &GetPromptResult{
			Description: "Code review prompt",
			Messages:    []*PromptMessage{{Role: "user", Content: &TextContent{Text: "review this code"}}},
		}, nil
	})

	conn := mustConnect(t, srv, &ClientOptions{
		ElicitationHandler: func(_ context.Context, _ *ElicitRequest) (*ElicitResult, error) {
			return &ElicitResult{Action: "accept"}, nil
		},
	})

	res, err := conn.GetPrompt(ctx, &GetPromptParams{Name: "review"})
	if err != nil {
		t.Fatalf("GetPrompt() error = %v", err)
	}
	if res.NeedsInput() {
		t.Fatal("NeedsInput() = true after auto-retry, want false")
	}
	if len(res.Messages) != 1 {
		t.Fatalf("len(res.Messages) = %d, want 1", len(res.Messages))
	}
	if got := res.Messages[0].Content.(*TextContent).Text; got != "review this code" {
		t.Errorf("message text = %q, want %q", got, "review this code")
	}
}

func TestMultiRoundTrip_GetPrompt_ManualRetry(t *testing.T) {

	ctx := context.Background()

	srv := NewServer(testImpl, nil)
	srv.AddPrompt(&Prompt{Name: "review"}, func(_ context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &GetPromptResult{
				InputRequests: InputRequestMap{"confirm": &ElicitParams{Message: "Include sensitive data?"}},
				RequestState:  "prompt-state",
			}, nil
		}
		return &GetPromptResult{
			Description: "Code review prompt",
			Messages:    []*PromptMessage{{Role: "user", Content: &TextContent{Text: "review this code"}}},
		}, nil
	})

	conn := mustConnect(t, srv, &ClientOptions{
		MultiRoundTrip: &MultiRoundTripOptions{Disabled: true},
	})

	res, err := conn.GetPrompt(ctx, &GetPromptParams{Name: "review"})
	if err != nil {
		t.Fatalf("GetPrompt() error = %v", err)
	}
	if !res.NeedsInput() {
		t.Fatal("NeedsInput() = false, want true")
	}
	if _, ok := res.InputRequests["confirm"].(*ElicitParams); !ok {
		t.Fatalf("InputRequests[confirm] type = %T, want *ElicitParams", res.InputRequests["confirm"])
	}

	res, err = conn.GetPrompt(ctx, &GetPromptParams{
		Name:           "review",
		InputResponses: InputResponseMap{"confirm": &ElicitResult{Action: "accept"}},
		RequestState:   res.RequestState,
	})
	if err != nil {
		t.Fatalf("GetPrompt() follow-up error = %v", err)
	}
	if res.NeedsInput() {
		t.Fatal("NeedsInput() = true after follow-up, want false")
	}
	if len(res.Messages) != 1 {
		t.Fatalf("len(res.Messages) = %d, want 1", len(res.Messages))
	}
}

func TestMultiRoundTrip_ReadResource_AutoRetry(t *testing.T) {

	ctx := context.Background()

	srv := NewServer(testImpl, nil)
	srv.AddResource(&Resource{URI: "test://data", Name: "data"}, func(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &ReadResourceResult{
				InputRequests: InputRequestMap{"auth": &ElicitParams{Message: "Authenticate?"}},
				RequestState:  "resource-state",
			}, nil
		}
		return &ReadResourceResult{
			Contents: []*ResourceContents{{URI: "test://data", Text: "resource data"}},
		}, nil
	})

	conn := mustConnect(t, srv, &ClientOptions{
		ElicitationHandler: func(_ context.Context, _ *ElicitRequest) (*ElicitResult, error) {
			return &ElicitResult{Action: "accept"}, nil
		},
	})

	res, err := conn.ReadResource(ctx, &ReadResourceParams{URI: "test://data"})
	if err != nil {
		t.Fatalf("ReadResource() error = %v", err)
	}
	if res.NeedsInput() {
		t.Fatal("NeedsInput() = true after auto-retry, want false")
	}
	if len(res.Contents) != 1 {
		t.Fatalf("len(res.Contents) = %d, want 1", len(res.Contents))
	}
	if got := res.Contents[0].Text; got != "resource data" {
		t.Errorf("resource text = %q, want %q", got, "resource data")
	}
}

func TestMultiRoundTrip_ReadResource_ManualRetry(t *testing.T) {

	ctx := context.Background()

	srv := NewServer(testImpl, nil)
	srv.AddResource(&Resource{URI: "test://data", Name: "data"}, func(_ context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
		if len(req.Params.InputResponses) == 0 {
			return &ReadResourceResult{
				InputRequests: InputRequestMap{"auth": &ElicitParams{Message: "Authenticate?"}},
				RequestState:  "resource-state",
			}, nil
		}
		return &ReadResourceResult{
			Contents: []*ResourceContents{{URI: "test://data", Text: "resource data"}},
		}, nil
	})

	conn := mustConnect(t, srv, &ClientOptions{
		MultiRoundTrip: &MultiRoundTripOptions{Disabled: true},
	})

	res, err := conn.ReadResource(ctx, &ReadResourceParams{URI: "test://data"})
	if err != nil {
		t.Fatalf("ReadResource() error = %v", err)
	}
	if !res.NeedsInput() {
		t.Fatal("NeedsInput() = false, want true")
	}
	if _, ok := res.InputRequests["auth"].(*ElicitParams); !ok {
		t.Fatalf("InputRequests[auth] type = %T, want *ElicitParams", res.InputRequests["auth"])
	}

	res, err = conn.ReadResource(ctx, &ReadResourceParams{
		URI:            "test://data",
		InputResponses: InputResponseMap{"auth": &ElicitResult{Action: "accept"}},
		RequestState:   res.RequestState,
	})
	if err != nil {
		t.Fatalf("ReadResource() follow-up error = %v", err)
	}
	if res.NeedsInput() {
		t.Fatal("NeedsInput() = true after follow-up, want false")
	}
	if len(res.Contents) != 1 {
		t.Fatalf("len(res.Contents) = %d, want 1", len(res.Contents))
	}
}

func mustConnect(t *testing.T, s *Server, clientOpts *ClientOptions) *ClientSession {
	t.Helper()

	st, ct := NewInMemoryTransports()
	ss, err := s.Connect(t.Context(), st, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		_ = ss.Close()
	})

	c := NewClient(testImpl, clientOpts)
	cs, err := c.Connect(t.Context(), ct, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
	})
	return cs
}
