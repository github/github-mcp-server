// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// TestServerErrors validates that the server returns appropriate error codes
// for various invalid requests.
func TestServerErrors(t *testing.T) {
	ctx := context.Background()

	// Set up a server with tools, prompts, and resources for testing
	cs, _, cleanup := basicConnection(t, func(s *Server) {
		// Add a tool with required parameters
		type RequiredParams struct {
			Name string `json:"name" jsonschema:"the name is required"`
		}
		handler := func(ctx context.Context, req *CallToolRequest, args RequiredParams) (*CallToolResult, any, error) {
			return &CallToolResult{
				Content: []Content{&TextContent{Text: "success"}},
			}, nil, nil
		}
		AddTool(s, &Tool{Name: "validate", Description: "validates params"}, handler)

		// Add a prompt
		s.AddPrompt(codeReviewPrompt, codReviewPromptHandler)

		// Add a resource that returns ResourceNotFoundError
		s.AddResource(
			&Resource{URI: "file:///test.txt", Name: "test", MIMEType: "text/plain"},
			func(ctx context.Context, req *ReadResourceRequest) (*ReadResourceResult, error) {
				return nil, ResourceNotFoundError(req.Params.URI)
			},
		)
	})
	defer cleanup()

	testCases := []struct {
		name         string
		executeCall  func() error
		expectedCode int64
	}{
		// Note: "missing required param" is tested separately below, because
		// input validation errors are returned as tool results with IsError=true
		// rather than JSON-RPC errors (see #450).
		{
			name: "unknown tool",
			executeCall: func() error {
				_, err := cs.CallTool(ctx, &CallToolParams{
					Name:      "nonexistent_tool",
					Arguments: map[string]any{},
				})
				return err
			},
			expectedCode: jsonrpc.CodeInvalidParams,
		},
		{
			name: "unknown prompt",
			executeCall: func() error {
				_, err := cs.GetPrompt(ctx, &GetPromptParams{
					Name:      "nonexistent_prompt",
					Arguments: map[string]string{},
				})
				return err
			},
			expectedCode: jsonrpc.CodeInvalidParams,
		},
		{
			name: "resource not found",
			executeCall: func() error {
				_, err := cs.ReadResource(ctx, &ReadResourceParams{
					URI: "file:///test.txt",
				})
				return err
			},
			expectedCode: CodeResourceNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.executeCall()
			if err == nil {
				t.Fatal("got nil error, want non-nil")
			}

			var rpcErr *jsonrpc.Error
			if !errors.As(err, &rpcErr) {
				t.Fatalf("got error type %T, want jsonrpc.Error: %v", err, err)
			}

			if rpcErr.Code != tc.expectedCode {
				t.Errorf("got error code %d, want %d", rpcErr.Code, tc.expectedCode)
			}

			if rpcErr.Message == "" {
				t.Error("got empty error message, want non-empty")
			}
		})
	}
}

// TestResourceNotFoundErrorCode verifies ResourceNotFoundError code and
// CodeResourceNotFound are -32602 (InvalidParams) per SEP-2164.
func TestResourceNotFoundErrorCode(t *testing.T) {
	err := ResourceNotFoundError("file:///test.txt")
	var rpcErr *jsonrpc.Error
	if !errors.As(err, &rpcErr) {
		t.Fatalf("got error type %T, want *jsonrpc.Error", err)
	}
	if rpcErr.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("got error code %d, want %d (InvalidParams)", rpcErr.Code, jsonrpc.CodeInvalidParams)
	}
	if CodeResourceNotFound != jsonrpc.CodeInvalidParams {
		t.Errorf("CodeResourceNotFound = %d, want %d", CodeResourceNotFound, jsonrpc.CodeInvalidParams)
	}
}

// TestInputValidationToolError validates that input validation errors (missing
// required params, wrong types) are returned as tool results with IsError=true,
// not as JSON-RPC errors. This allows LLMs to see the error and self-correct.
// See #450.
func TestInputValidationToolError(t *testing.T) {
	ctx := context.Background()

	type RequiredParams struct {
		Name string `json:"name" jsonschema:"the name is required"`
	}
	handler := func(ctx context.Context, req *CallToolRequest, args RequiredParams) (*CallToolResult, any, error) {
		return &CallToolResult{
			Content: []Content{&TextContent{Text: "success"}},
		}, nil, nil
	}

	cs, _, cleanup := basicConnection(t, func(s *Server) {
		AddTool(s, &Tool{Name: "validate", Description: "validates params"}, handler)
	})
	defer cleanup()

	// Call the tool with missing required "name" field.
	result, err := cs.CallTool(ctx, &CallToolParams{
		Name:      "validate",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("CallTool returned error: %v; want tool result with IsError", err)
	}
	if !result.IsError {
		t.Fatal("got IsError=false, want IsError=true for missing required param")
	}
	text := result.Content[0].(*TextContent).Text
	if !strings.Contains(text, "name") {
		t.Errorf("error text %q does not mention missing field \"name\"", text)
	}
}

// TestURLElicitationRequired validates that URL elicitation required errors
// are properly created and handled by the client.
func TestURLElicitationRequired(t *testing.T) {
	ctx := context.Background()

	t.Run("error creation", func(t *testing.T) {
		elicitations := []*ElicitParams{
			{
				Mode:          "url",
				Message:       "Please authorize",
				URL:           "https://example.com/auth",
				ElicitationID: "auth-123",
			},
		}

		err := URLElicitationRequiredError(elicitations)

		var rpcErr *jsonrpc.Error
		if !errors.As(err, &rpcErr) {
			t.Fatalf("got error type %T, want jsonrpc.Error", err)
		}

		if rpcErr.Code != CodeURLElicitationRequired {
			t.Errorf("got error code %d, want %d", rpcErr.Code, CodeURLElicitationRequired)
		}

		if rpcErr.Message != "URL elicitation required" {
			t.Errorf("got message %q, want 'URL elicitation required'", rpcErr.Message)
		}

		if rpcErr.Data == nil {
			t.Fatal("got nil error data, want non-nil")
		}

		// Verify the elicitations can be unmarshaled from the error data
		var errorData struct {
			Elicitations []*ElicitParams `json:"elicitations"`
		}
		if err := json.Unmarshal(rpcErr.Data, &errorData); err != nil {
			t.Fatalf("failed to unmarshal error data: %v", err)
		}

		if len(errorData.Elicitations) != 1 {
			t.Fatalf("got %d elicitations, want 1", len(errorData.Elicitations))
		}

		if errorData.Elicitations[0].URL != "https://example.com/auth" {
			t.Errorf("got URL %q, want 'https://example.com/auth'", errorData.Elicitations[0].URL)
		}
	})

	t.Run("error creation with non-URL mode panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("got no panic when creating URLElicitationRequiredError with non-URL mode, want panic")
			}
		}()

		// This should panic because mode is "form"
		URLElicitationRequiredError([]*ElicitParams{
			{
				Mode:          "form",
				Message:       "This should panic",
				ElicitationID: "bad-123",
			},
		})
	})

	t.Run("error creation with empty mode panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("got no panic when creating URLElicitationRequiredError with empty mode (defaults to form), want panic")
			}
		}()

		// This should panic because empty mode defaults to "form"
		URLElicitationRequiredError([]*ElicitParams{
			{
				Message:       "This should panic",
				ElicitationID: "bad-123",
			},
		})
	})

	t.Run("client middleware", func(t *testing.T) {
		// Declare ss outside so it can be captured in handlers.
		var ss *ServerSession

		elicitCalled := false
		elicitURL := ""
		elicitID := "form-123"

		// Create client with elicitation handler and middleware.
		client := NewClient(testImpl, &ClientOptions{
			Capabilities: &ClientCapabilities{
				Roots:   RootCapabilities{ListChanged: true},
				RootsV2: &RootCapabilities{ListChanged: true},
				Elicitation: &ElicitationCapabilities{
					URL: &URLElicitationCapabilities{},
				},
			},
			ElicitationHandler: func(ctx context.Context, req *ElicitRequest) (*ElicitResult, error) {
				elicitCalled = true
				elicitURL = req.Params.URL

				// Simulate the server sending elicitation complete notification.
				// In a real scenario, this would happen out-of-band after the user
				// completes the form submission.
				go func() {
					err := handleNotify(ctx, notificationElicitationComplete,
						newServerRequest(ss, &ElicitationCompleteParams{
							ElicitationID: elicitID,
						}))
					if err != nil {
						t.Errorf("failed to send elicitation complete notification: %v", err)
					}
				}()

				return &ElicitResult{Action: "accept"}, nil
			},
		})
		// Add URL elicitation middleware for automatic retry.
		client.AddSendingMiddleware(urlElicitationMiddleware())

		var callCount atomic.Int32

		cs, serverSession, cleanup := basicClientServerConnection(t,
			client,
			nil,
			func(s *Server) {
				// Tool that requires form submission on first call, succeeds on second.
				handler := func(ctx context.Context, req *CallToolRequest, args map[string]any) (*CallToolResult, any, error) {
					if callCount.Add(1) == 1 {
						// First call: require elicitation.
						return nil, nil, URLElicitationRequiredError([]*ElicitParams{
							{
								Mode:          "url",
								Message:       "Please complete the form",
								URL:           "https://example.com/form",
								ElicitationID: elicitID,
							},
						})
					}
					// Second call (after retry): return success.
					return &CallToolResult{
						Content: []Content{&TextContent{Text: "form submitted"}},
					}, nil, nil
				}
				AddTool(s, &Tool{Name: "submit_form", Description: "requires form submission"}, handler)

				// Tool that returns invalid elicitation mode (form instead of URL).
				badHandler := func(ctx context.Context, req *CallToolRequest, args map[string]any) (*CallToolResult, any, error) {
					// Manually construct an error with form mode (bypassing validation).
					data, _ := json.Marshal(map[string]any{
						"elicitations": []*ElicitParams{
							{
								Mode:          "form",
								Message:       "Invalid mode",
								ElicitationID: "bad-form",
							},
						},
					})
					return nil, nil, &jsonrpc.Error{
						Code:    CodeURLElicitationRequired,
						Message: "URL elicitation required",
						Data:    json.RawMessage(data),
					}
				}
				AddTool(s, &Tool{Name: "bad_tool", Description: "returns invalid elicitation"}, badHandler)
			},
		)
		ss = serverSession
		defer cleanup()

		t.Run("auto-retry after elicitation", func(t *testing.T) {
			// Reset state for this subtest.
			elicitCalled = false
			elicitURL = ""
			callCount.Store(0)

			// Call the tool that requires URL elicitation.
			result, err := cs.CallTool(ctx, &CallToolParams{
				Name:      "submit_form",
				Arguments: map[string]any{},
			})

			// After automatic retry, the operation should succeed.
			if err != nil {
				t.Fatalf("CallTool failed: %v", err)
			}

			// Verify the elicitation handler was called.
			if !elicitCalled {
				t.Error("elicitation handler not called")
			}

			if elicitURL != "https://example.com/form" {
				t.Errorf("got elicit URL %q, want 'https://example.com/form'", elicitURL)
			}

			// Verify the tool was called twice (first attempt + retry).
			if got, want := callCount.Load(), int32(2); got != want {
				t.Errorf("CallTool(): with retry, got %d tool calls, want %d", got, want)
			}

			// Verify we got the successful result.
			if len(result.Content) != 1 {
				t.Fatalf("CallTool(): got %d content items, want 1", len(result.Content))
			}

			textContent, ok := result.Content[0].(*TextContent)
			if !ok {
				t.Fatalf("CallTool(): got content type %T, want TextContent", result.Content[0])
			}

			if textContent.Text != "form submitted" {
				t.Errorf("CallTool(): got text %q, want 'form submitted'", textContent.Text)
			}
		})

		t.Run("reject non-URL mode", func(t *testing.T) {
			// Call the tool that returns invalid elicitation mode.
			_, err := cs.CallTool(ctx, &CallToolParams{
				Name:      "bad_tool",
				Arguments: map[string]any{},
			})

			// Should get an error about invalid mode.
			if err == nil {
				t.Fatal("got nil error for non-URL mode elicitation, want error")
			}

			if !strings.Contains(err.Error(), "URL mode") {
				t.Errorf("got error %v, want mention of URL mode", err)
			}
		})
	})
}
