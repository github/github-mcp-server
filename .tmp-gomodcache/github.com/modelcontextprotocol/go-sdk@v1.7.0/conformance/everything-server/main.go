// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The conformance server implements features required for MCP conformance testing.
// It mirrors the functionality of the TypeScript conformance server at
// https://github.com/modelcontextprotocol/conformance/blob/main/examples/servers/typescript/everything-server.ts
//
//lint:file-ignore SA1019 conformance server exercises deprecated SEP-2577 APIs (roots, sampling, logging).
package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/yosida95/uritemplate/v3"
)

var (
	httpAddr  = flag.String("http", "", "if set, use streamable HTTP at this address, instead of stdin/stdout")
	stateless = flag.Bool("stateless", true, "use stateless streamable HTTP mode")
)

const watchedResourceURI = "test://watched-resource"

func main() {
	flag.Parse()

	opts := &mcp.ServerOptions{
		CompletionHandler:  completionHandler,
		SubscribeHandler:   subscribeHandler,
		UnsubscribeHandler: unsubscribeHandler,
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "mcp-conformance-test-server",
		Version: "1.0.0",
	}, opts)

	// Register server features.
	registerTools(server)
	registerResources(server)
	registerPrompts(server)

	// Start the watched resource auto-update goroutine.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watchedResourceUpdater(ctx, server)

	// Serve over stdio, or streamable HTTP if -http is set.
	if *httpAddr != "" {
		handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, &mcp.StreamableHTTPOptions{Stateless: *stateless})
		log.Printf("Conformance server listening at %s", *httpAddr)
		log.Fatal(http.ListenAndServe(*httpAddr, handler))
	} else {
		t := &mcp.StdioTransport{}
		if err := server.Run(ctx, t); err != nil {
			log.Printf("Server failed: %v", err)
			os.Exit(1)
		}
	}
}

// watchedResourceUpdater sends resource update notifications every 3 seconds.
func watchedResourceUpdater(ctx context.Context, server *mcp.Server) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			server.ResourceUpdated(ctx, &mcp.ResourceUpdatedNotificationParams{
				URI: watchedResourceURI,
			})
		}
	}
}

// =============================================================================
// Tools
// =============================================================================

func registerTools(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_simple_text",
		Description: "Tests simple text content response",
	}, testSimpleTextHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_image_content",
		Description: "Tests image content response",
	}, testImageContentHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_audio_content",
		Description: "Tests audio content response",
	}, testAudioContentHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_embedded_resource",
		Description: "Tests embedded resource content response",
	}, testEmbeddedResourceHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_multiple_content_types",
		Description: "Tests response with multiple content types (text, image, resource)",
	}, testMultipleContentTypesHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_tool_with_logging",
		Description: "Tests tool that emits log messages during execution",
	}, testToolWithLoggingHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_tool_with_progress",
		Description: "Tests tool that reports progress notifications",
	}, testToolWithProgressHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_error_handling",
		Description: "Tests error response handling",
	}, testErrorHandlingHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_sampling",
		Description: "Tests server-initiated sampling (LLM completion request)",
	}, testSamplingHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_elicitation",
		Description: "Tests server-initiated elicitation (user input request)",
	}, testElicitationHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_elicitation_sep1034_defaults",
		Description: "Tests elicitation with default values per SEP-1034",
	}, testElicitationDefaultsHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_elicitation_sep1330_enums",
		Description: "Tests elicitation with enum schema improvements per SEP-1330",
	}, testElicitationEnumsHandler)

	// SEP-1613 / SEP-2106: JSON Schema 2020-12 conformance test tool.
	// The scenario verifies that $schema/$defs/additionalProperties (SEP-1613)
	// and the broader 2020-12 vocabulary — $anchor, allOf/anyOf, if/then/else —
	// (SEP-2106) survive tools/list verbatim.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "json_schema_2020_12_tool",
		Description: "Tool with JSON Schema 2020-12 features for conformance testing (SEP-1613, SEP-2106)",
		InputSchema: json.RawMessage(`{
			"$schema": "https://json-schema.org/draft/2020-12/schema",
			"type": "object",
			"$defs": {
				"address": {
					"$anchor": "addressDef",
					"type": "object",
					"properties": {
						"street": { "type": "string" },
						"city": { "type": "string" }
					}
				}
			},
			"properties": {
				"name": { "type": "string" },
				"address": { "$ref": "#/$defs/address" },
				"contactMethod": { "type": "string", "enum": ["phone", "email"] },
				"phone": { "type": "string" },
				"email": { "type": "string" }
			},
			"allOf": [
				{ "anyOf": [{ "required": ["phone"] }, { "required": ["email"] }] }
			],
			"if": {
				"properties": { "contactMethod": { "const": "phone" } },
				"required": ["contactMethod"]
			},
			"then": { "required": ["phone"] },
			"else": { "required": ["email"] },
			"additionalProperties": false
		}`),
	}, jsonSchema202012Handler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_reconnection",
		Description: "Tests SSE stream disconnection and client reconnection (SEP-1699). Server will close the stream mid-call and send the result after client reconnects.",
	}, testReconnectionHandler)

	// SEP-2243 x-mcp-header tool — arms the http-custom-header-server-validation
	// conformance scenario (which skips when no tool with an x-mcp-header
	// annotation is found).
	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_x_mcp_header",
		Description: "Tests SEP-2243 Mcp-Param-* server-side validation",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"region": { "type": "string", "description": "mirrored into Mcp-Param-Region", "x-mcp-header": "Region" },
				"level": { "type": "integer", "description": "non-mirrored argument" }
			}
		}`),
	}, testXMcpHeaderHandler)

	// SEP-2575 diagnostic tools used by the stateless conformance scenario.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_missing_capability",
		Description: "Requires the sampling client capability; used to verify MissingRequiredClientCapabilityError (-32021) (SEP-2575)",
	}, testMissingCapabilityHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_streaming_elicitation",
		Description: "Streams progress notifications while a call is in flight; used to verify response streams carry only notifications, never independent JSON-RPC requests (SEP-2575)",
	}, testStreamingElicitationHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_logging_tool",
		Description: "Attempts to emit a log message; the framework must drop it when the client did not set _meta.../logLevel (SEP-2575)",
	}, testLoggingToolHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_trigger_tool_change",
		Description: "Mutates the tool list to trigger a notifications/tools/list_changed on active subscription streams (SEP-2575)",
	}, makeTriggerToolChangeHandler(server))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_trigger_prompt_change",
		Description: "Mutates the prompt list to trigger a notifications/prompts/list_changed on active subscription streams (SEP-2575)",
	}, makeTriggerPromptChangeHandler(server))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_elicitation",
		Description: "MRTR (SEP-2322): asks for the caller name via an in-band elicitation request",
	}, testInputRequiredElicitationHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_sampling",
		Description: "MRTR (SEP-2322): asks for an LLM completion via an in-band sampling request",
	}, testInputRequiredSamplingHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_list_roots",
		Description: "MRTR (SEP-2322): asks for the client roots via an in-band roots/list request",
	}, testInputRequiredListRootsHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_request_state",
		Description: "MRTR (SEP-2322): round-trips integrity-protected requestState alongside an elicitation request",
	}, testInputRequiredRequestStateHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_multiple_inputs",
		Description: "MRTR (SEP-2322): asks for elicitation, sampling and roots input in a single round",
	}, testInputRequiredMultipleInputsHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_multi_round",
		Description: "MRTR (SEP-2322): two elicitation rounds with evolving requestState before completing",
	}, testInputRequiredMultiRoundHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_tampered_state",
		Description: "MRTR (SEP-2322): rejects retries whose requestState fails integrity verification",
	}, testInputRequiredTamperedStateHandler)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "test_input_required_result_capabilities",
		Description: "MRTR (SEP-2322): only requests input kinds the declared client capabilities cover",
	}, testInputRequiredCapabilitiesHandler)
}

// Tool handlers

func testSimpleTextHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "This is a simple text response for testing."},
		},
	}, nil, nil
}

func testImageContentHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.ImageContent{
				Data:     imageData(),
				MIMEType: "image/png",
			},
		},
	}, nil, nil
}

func testAudioContentHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.AudioContent{
				Data:     audioData(),
				MIMEType: "audio/wav",
			},
		},
	}, nil, nil
}

func testEmbeddedResourceHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "test://embedded-resource",
					MIMEType: "text/plain",
					Text:     "This is an embedded resource",
				},
			},
		},
	}, nil, nil
}

func testMultipleContentTypesHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "This is text content"},
			&mcp.ImageContent{
				Data:     imageData(),
				MIMEType: "image/png",
			},
			&mcp.EmbeddedResource{
				Resource: &mcp.ResourceContents{
					URI:      "test://embedded-in-multiple",
					MIMEType: "text/plain",
					Text:     "This is an embedded resource",
				},
			},
		},
	}, nil, nil
}

func testToolWithLoggingHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	// Emit three info-level log messages
	req.Session.Log(ctx, &mcp.LoggingMessageParams{
		Level: "info",
		Data:  "Tool execution started",
	})
	time.Sleep(50 * time.Millisecond)
	req.Session.Log(ctx, &mcp.LoggingMessageParams{
		Level: "info",
		Data:  "Tool processing data",
	})
	time.Sleep(50 * time.Millisecond)
	req.Session.Log(ctx, &mcp.LoggingMessageParams{
		Level: "info",
		Data:  "Tool execution completed",
	})

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Tool with logging executed successfully"},
		},
	}, nil, nil
}

func testToolWithProgressHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	// Get progress token from the request if provided
	progressToken := req.Params.GetProgressToken()

	// Send three progress notifications (0%, 50%, 100%)
	total := 100.0
	steps := []float64{0, 50, 100}
	for _, progress := range steps {
		req.Session.NotifyProgress(ctx, &mcp.ProgressNotificationParams{
			ProgressToken: progressToken,
			Progress:      progress,
			Total:         total,
			Message:       fmt.Sprintf("Completed step %.0f of %.0f", progress, total),
		})
		time.Sleep(50 * time.Millisecond)
	}

	// Return the progress token value as the response (matching TypeScript behavior)
	tokenStr := fmt.Sprintf("%v", progressToken)
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: tokenStr},
		},
	}, nil, nil
}

func testErrorHandlingHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return nil, nil, errors.New("this tool intentionally returns an error for testing")
}

type samplingInput struct {
	Prompt string `json:"prompt" jsonschema:"The prompt to send to the LLM"`
}

func testSamplingHandler(ctx context.Context, req *mcp.CallToolRequest, input samplingInput) (*mcp.CallToolResult, any, error) {
	// Request LLM completion from the client
	result, err := req.Session.CreateMessage(ctx, &mcp.CreateMessageParams{
		Messages: []*mcp.SamplingMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: input.Prompt,
				},
			},
		},
		MaxTokens: 100,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("sampling failed: %w", err)
	}

	// Extract the text response from the result
	var responseText string
	if tc, ok := result.Content.(*mcp.TextContent); ok {
		responseText = tc.Text
	} else {
		responseText = "(non-text response)"
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("LLM response: %s", responseText)},
		},
	}, nil, nil
}

type elicitationInput struct {
	Message string `json:"message" jsonschema:"The message to show the user"`
}

func testElicitationHandler(ctx context.Context, req *mcp.CallToolRequest, input elicitationInput) (*mcp.CallToolResult, any, error) {
	result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message: input.Message,
		RequestedSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"username": {
					Type:        "string",
					Description: "Your preferred username",
				},
			},
			Required: []string{"username"},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("elicitation failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Elicitation result: action=%s, content=%v", result.Action, result.Content)},
		},
	}, nil, nil
}

func testElicitationDefaultsHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message: "Test defaults for primitives",
		RequestedSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "User name",
					"default":     "John Doe",
				},
				"age": map[string]any{
					"type":        "integer",
					"description": "User age",
					"default":     30,
				},
				"score": map[string]any{
					"type":        "number",
					"description": "User score",
					"default":     95.5,
				},
				"status": map[string]any{
					"type":        "string",
					"description": "User status",
					"enum":        []string{"active", "inactive", "pending"},
					"default":     "active",
				},
				"verified": map[string]any{
					"type":        "boolean",
					"description": "Verification status",
					"default":     true,
				},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("elicitation failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Elicitation result: action=%s, content=%v", result.Action, result.Content)},
		},
	}, nil, nil
}

func testElicitationEnumsHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	result, err := req.Session.Elicit(ctx, &mcp.ElicitParams{
		Message: "Test enum schemas",
		RequestedSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				// Basic enum without titles
				"untitledSingle": map[string]any{
					"type": "string",
					"enum": []string{"option1", "option2", "option3"},
				},
				// Enum with titles using oneOf
				"titledSingle": map[string]any{
					"type": "string",
					"oneOf": []map[string]any{
						{"const": "value1", "title": "First Option"},
						{"const": "value2", "title": "Second Option"},
						{"const": "value3", "title": "Third Option"},
					},
				},
				// Legacy enum with enumNames
				"legacyEnum": map[string]any{
					"type":      "string",
					"enum":      []string{"opt1", "opt2", "opt3"},
					"enumNames": []string{"Option One", "Option Two", "Option Three"},
				},
				// Multi-select without titles
				"untitledMulti": map[string]any{
					"type":     "array",
					"minItems": 1,
					"maxItems": 3,
					"items": map[string]any{
						"type": "string",
						"enum": []string{"option1", "option2", "option3"},
					},
				},
				// Multi-select with titles using anyOf
				"titledMulti": map[string]any{
					"type":     "array",
					"minItems": 1,
					"maxItems": 3,
					"items": map[string]any{
						"type": "string",
						"anyOf": []map[string]any{
							{"const": "value1", "title": "First Option"},
							{"const": "value2", "title": "Second Option"},
							{"const": "value3", "title": "Third Option"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("elicitation failed: %w", err)
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Elicitation result: action=%s, content=%v", result.Action, result.Content)},
		},
	}, nil, nil
}

func jsonSchema202012Handler(ctx context.Context, req *mcp.CallToolRequest, input json.RawMessage) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: fmt.Sprintf("JSON Schema 2020-12 tool called with: %s", input),
			},
		},
	}, nil, nil
}

type testXMcpHeaderInput struct {
	Region string `json:"region,omitempty"`
	Level  int    `json:"level,omitempty"`
}

func testXMcpHeaderHandler(ctx context.Context, req *mcp.CallToolRequest, input testXMcpHeaderInput) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("region=%s", input.Region)},
		},
	}, nil, nil
}

// testMissingCapabilityHandler requires the client to have declared the
// sampling capability. When absent, it returns a MissingRequiredClientCapabilityError
// with data.requiredCapabilities = {"sampling": {}} per SEP-2575.
func testMissingCapabilityHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	caps := req.ClientCapabilities()
	if caps != nil && caps.Sampling != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Client declared the sampling capability; tool executed."},
			},
		}, nil, nil
	}
	missingCapabilityData := mcp.MissingRequiredClientCapabilityData{
		RequiredCapabilities: &mcp.ClientCapabilities{
			Sampling: &mcp.SamplingCapabilities{},
		},
	}
	dataBytes, err := json.Marshal(missingCapabilityData)
	if err != nil {
		return nil, nil, err
	}
	return nil, nil, &jsonrpc.Error{
		Code:    mcp.CodeMissingRequiredClientCapabilities,
		Message: "sampling capability required but not declared by client",
		Data:    dataBytes,
	}
}

// testStreamingElicitationHandler returns a plain result. Per SEP-2575, the
// response stream must carry no independent top-level JSON-RPC requests; a
// plain response trivially satisfies this. The scenario declares no
// `elicitation` capability, so this handler must not call req.Session.Elicit.
func testStreamingElicitationHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "stream observed: result frames only, no top-level requests"},
		},
	}, nil, nil
}

// testLoggingToolHandler attempts to emit a log message. The SDK's
// ServerSession.Log gates on the per-request _meta.../logLevel, so this is a
// no-op notification-wise when the client did not opt in.
func testLoggingToolHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	req.Session.Log(ctx, &mcp.LoggingMessageParams{
		Level: "info",
		Data:  "test_logging_tool executed",
	})
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Log attempted; framework gates on _meta.../logLevel."},
		},
	}, nil, nil
}

// makeTriggerToolChangeHandler returns a handler that (re-)registers a
// no-op transient tool. Every call to Server.AddTool dispatches a
// notifications/tools/list_changed to active subscription streams (even when
// it replaces a tool with the same name), which is what the SEP-2575
// subscription checks assert.
func makeTriggerToolChangeHandler(server *mcp.Server) mcp.ToolHandlerFor[any, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		mcp.AddTool(server, &mcp.Tool{
			Name:        "__transient_tool_for_list_changed",
			Description: "Transient tool used to trigger tools/list_changed",
		}, func(context.Context, *mcp.CallToolRequest, any) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{}, nil, nil
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "tools_list_changed published"},
			},
		}, nil, nil
	}
}

// makeTriggerPromptChangeHandler is the prompt analogue of
// makeTriggerToolChangeHandler.
func makeTriggerPromptChangeHandler(server *mcp.Server) mcp.ToolHandlerFor[any, any] {
	return func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		server.AddPrompt(&mcp.Prompt{
			Name:        "__transient_prompt_for_list_changed",
			Description: "Transient prompt used to trigger prompts/list_changed",
		}, func(context.Context, *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
			return &mcp.GetPromptResult{}, nil
		})
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "prompts_list_changed published"},
			},
		}, nil, nil
	}
}

// =============================================================================
// SEP-2322 Multi-Round-Trip helpers
// =============================================================================

// requestStateTamperedSuffix is what the tampered-state conformance scenario
// appends to a valid requestState. Detecting this suffix is sufficient to
// pass the check; a production server would use a real integrity mechanism
// (HMAC / AEAD, per SEP-2322 § "Server Requirements").
const requestStateTamperedSuffix = "-TAMPERED"

// requestStateValid reports whether the state looks like one this server
// minted (i.e. the harness did not append the tampered marker).
func requestStateValid(state string) bool {
	return state != "" && !strings.HasSuffix(state, requestStateTamperedSuffix)
}

// acceptedContent returns the "content" field of an ElicitResult only when the
// user accepted the elicitation. Returns nil if the response is missing, of the
// wrong type, or the action is not "accept".
func acceptedContent(responses mcp.InputResponseMap, key string) map[string]any {
	if responses == nil {
		return nil
	}
	resp, ok := responses[key]
	if !ok {
		return nil
	}
	elicit, ok := resp.(*mcp.ElicitResult)
	if !ok || elicit == nil || elicit.Action != "accept" {
		return nil
	}
	return elicit.Content
}

// =============================================================================
// SEP-2322 Multi-Round-Trip tool handlers
// =============================================================================

// nameElicitSchema is the requestedSchema used by tools that elicit a name.
func nameElicitSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"name": {Type: "string"},
		},
		Required: []string{"name"},
	}
}

// testInputRequiredElicitationHandler asks for the caller name via elicitation.
// Reused by the result-type, missing-input-response, ignore-extra-params, and
// validate-input scenarios: anything that does not carry an accepted
// "user_name" response is answered with a fresh InputRequiredResult.
func testInputRequiredElicitationHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if content := acceptedContent(req.Params.InputResponses, "user_name"); content != nil {
		name, _ := content["name"].(string)
		if name != "" {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Hello, %s!", name)}},
			}, nil, nil
		}
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			"user_name": &mcp.ElicitParams{
				Message:         "What is your name?",
				RequestedSchema: nameElicitSchema(),
			},
		},
	}, nil, nil
}

// samplingResponseText extracts the first text content from a sampling
// response. The Go SDK unmarshals responses with a "role" field into
// *CreateMessageWithToolsResult (a superset of the legacy single-content
// CreateMessageResult).
func samplingResponseText(responses mcp.InputResponseMap, key string) (string, bool) {
	resp, ok := responses[key].(*mcp.CreateMessageWithToolsResult)
	if !ok || resp == nil {
		return "", false
	}
	for _, c := range resp.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text, true
		}
	}
	return "(non-text response)", true
}

// testInputRequiredSamplingHandler asks for an LLM completion via sampling.
func testInputRequiredSamplingHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if text, ok := samplingResponseText(req.Params.InputResponses, "capital_question"); ok {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("Sampling response: %s", text)}},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			"capital_question": &mcp.CreateMessageParams{
				Messages: []*mcp.SamplingMessage{
					{Role: "user", Content: &mcp.TextContent{Text: "What is the capital of France?"}},
				},
				MaxTokens: 100,
			},
		},
	}, nil, nil
}

// testInputRequiredListRootsHandler asks for the client roots.
func testInputRequiredListRootsHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if resp, ok := req.Params.InputResponses["client_roots"].(*mcp.ListRootsResult); ok && resp != nil {
		var uris []string
		for _, r := range resp.Roots {
			uris = append(uris, r.URI)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Client exposed %d root(s): %s", len(resp.Roots), strings.Join(uris, ", "))},
			},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			"client_roots": &mcp.ListRootsParams{},
		},
	}, nil, nil
}

// testInputRequiredRequestStateHandler round-trips an opaque requestState
// alongside an elicitation request.
func testInputRequiredRequestStateHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if content := acceptedContent(req.Params.InputResponses, "confirm"); content != nil {
		if requestStateValid(req.Params.RequestState) {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "state-ok: requestState received and confirmation accepted"}},
			}, nil, nil
		}
		return nil, nil, &jsonrpc.Error{
			Code:    jsonrpc.CodeInvalidParams,
			Message: "invalid requestState",
		}
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			"confirm": &mcp.ElicitParams{
				Message: "Please confirm",
				RequestedSchema: &jsonschema.Schema{
					Type:       "object",
					Properties: map[string]*jsonschema.Schema{"ok": {Type: "boolean"}},
					Required:   []string{"ok"},
				},
			},
		},
		RequestState: "request_state",
	}, nil, nil
}

// testInputRequiredMultipleInputsHandler asks for elicitation, sampling, and
// roots input in a single round.
func testInputRequiredMultipleInputsHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	nameContent := acceptedContent(req.Params.InputResponses, "user_name")
	name, _ := nameContent["name"].(string)

	greetingText, hasGreeting := samplingResponseText(req.Params.InputResponses, "greeting")

	var rootsCount int
	rootsResp, rootsOK := req.Params.InputResponses["client_roots"].(*mcp.ListRootsResult)
	if rootsOK && rootsResp != nil {
		rootsCount = len(rootsResp.Roots)
	}

	if name != "" && hasGreeting && rootsOK {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("%s %s — %d root(s) visible", greetingText, name, rootsCount)},
			},
		}, nil, nil
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			"user_name": &mcp.ElicitParams{
				Message:         "What is your name?",
				RequestedSchema: nameElicitSchema(),
			},
			"greeting": &mcp.CreateMessageParams{
				Messages: []*mcp.SamplingMessage{
					{Role: "user", Content: &mcp.TextContent{Text: "Generate a greeting"}},
				},
				MaxTokens: 50,
			},
			"client_roots": &mcp.ListRootsParams{},
		},
		RequestState: "multiple_inputs",
	}, nil, nil
}

// testInputRequiredMultiRoundHandler runs two elicitation rounds with
// evolving requestState before completing. The round number (and the name
// elicited in round 1) lives in the state, since the stateless server keeps
// no per-session memory.
func testInputRequiredMultiRoundHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	state := req.Params.RequestState
	// State format: "round=1" or "round=2;name=<name>".
	switch {
	case state == "":
		return &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{
				"step1": &mcp.ElicitParams{
					Message:         "Step 1: What is your name?",
					RequestedSchema: nameElicitSchema(),
				},
			},
			RequestState: "round=1",
		}, nil, nil
	case state == "round=1":
		name, _ := acceptedContent(req.Params.InputResponses, "step1")["name"].(string)
		if name == "" {
			name = "unknown"
		}
		return &mcp.CallToolResult{
			InputRequests: mcp.InputRequestMap{
				"step2": &mcp.ElicitParams{
					Message: "Step 2: What is your favorite color?",
					RequestedSchema: &jsonschema.Schema{
						Type:       "object",
						Properties: map[string]*jsonschema.Schema{"color": {Type: "string"}},
						Required:   []string{"color"},
					},
				},
			},
			RequestState: "round=2;name=" + name,
		}, nil, nil
	default:
		color, _ := acceptedContent(req.Params.InputResponses, "step2")["color"].(string)
		if color == "" {
			color = "unknown"
		}
		name := strings.TrimPrefix(state, "round=2;name=")
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Multi-round complete: %s likes %s", name, color)},
			},
		}, nil, nil
	}
}

// testInputRequiredTamperedStateHandler rejects retries whose requestState
// looks tampered. The conformance harness tampers by appending "-TAMPERED";
// production servers should use a real integrity mechanism (HMAC / AEAD).
func testInputRequiredTamperedStateHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if req.Params.RequestState != "" {
		if !requestStateValid(req.Params.RequestState) {
			return nil, nil, &jsonrpc.Error{
				Code:    jsonrpc.CodeInvalidParams,
				Message: "invalid requestState: tampered",
			}
		}
		if acceptedContent(req.Params.InputResponses, "confirm") != nil {
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "state-ok: requestState received and confirmation accepted"}},
			}, nil, nil
		}
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{
			"confirm": &mcp.ElicitParams{
				Message: "Please confirm",
				RequestedSchema: &jsonschema.Schema{
					Type:       "object",
					Properties: map[string]*jsonschema.Schema{"ok": {Type: "boolean"}},
					Required:   []string{"ok"},
				},
			},
		},
		RequestState: "tampered_state",
	}, nil, nil
}

// testInputRequiredCapabilitiesHandler only requests input kinds the client
// declared capabilities cover. Reads capabilities from the per-request
// _meta["io.modelcontextprotocol/clientCapabilities"] envelope.
func testInputRequiredCapabilitiesHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	if len(req.Params.InputResponses) > 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "Capability-aware input requests fulfilled"}},
		}, nil, nil
	}
	caps := req.ClientCapabilities()
	reqs := mcp.InputRequestMap{}
	if caps != nil && caps.Elicitation != nil {
		reqs["user_name"] = &mcp.ElicitParams{
			Message:         "What is your name?",
			RequestedSchema: nameElicitSchema(),
		}
	}
	if caps != nil && caps.Sampling != nil {
		reqs["greeting"] = &mcp.CreateMessageParams{
			Messages: []*mcp.SamplingMessage{
				{Role: "user", Content: &mcp.TextContent{Text: "Generate a short greeting"}},
			},
			MaxTokens: 50,
		}
	}
	if caps != nil && (caps.RootsV2 != nil || caps.Roots.ListChanged) {
		reqs["client_roots"] = &mcp.ListRootsParams{}
	}
	if len(reqs) == 0 {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "No declared client capability supports an in-band input request"}},
		}, nil, nil
	}
	return &mcp.CallToolResult{InputRequests: reqs}, nil, nil
}

// testInputRequiredPromptHandler is the prompts/get analogue: elicits context
// on the first call and returns a GetPromptResult with messages on retry.
func testInputRequiredPromptHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	if content := acceptedContent(req.Params.InputResponses, "user_context"); content != nil {
		contextStr, _ := content["context"].(string)
		return &mcp.GetPromptResult{
			Description: "A prompt with elicited context",
			Messages: []*mcp.PromptMessage{
				{
					Role:    "user",
					Content: &mcp.TextContent{Text: fmt.Sprintf("Context: %s", contextStr)},
				},
			},
		}, nil
	}
	return &mcp.GetPromptResult{
		InputRequests: mcp.InputRequestMap{
			"user_context": &mcp.ElicitParams{
				Message: "Please provide context for the prompt",
				RequestedSchema: &jsonschema.Schema{
					Type:       "object",
					Properties: map[string]*jsonschema.Schema{"context": {Type: "string"}},
					Required:   []string{"context"},
				},
			},
		},
	}, nil
}

func testReconnectionHandler(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
	// Close the SSE stream to trigger client reconnection (SEP-1699)
	if req.Extra != nil && req.Extra.CloseSSEStream != nil {
		req.Extra.CloseSSEStream(mcp.CloseSSEStreamArgs{RetryAfter: 10 * time.Millisecond})
	}

	// Wait for client to reconnect
	time.Sleep(100 * time.Millisecond)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{
				Text: "Reconnection test completed successfully. If you received this, the client properly reconnected after stream closure.",
			},
		},
	}, nil, nil
}

// =============================================================================
// Resources
// =============================================================================

func registerResources(server *mcp.Server) {
	server.AddResource(&mcp.Resource{
		Name:        "static-text",
		Description: "A static text resource for testing",
		MIMEType:    "text/plain",
		URI:         "test://static-text",
	}, staticTextHandler)

	server.AddResource(&mcp.Resource{
		Name:        "static-binary",
		Description: "A static binary resource (image) for testing",
		MIMEType:    "image/png",
		URI:         "test://static-binary",
	}, staticBinaryHandler)

	server.AddResourceTemplate(&mcp.ResourceTemplate{
		Name:        "template",
		Description: "A resource template with parameter substitution",
		MIMEType:    "application/json",
		URITemplate: "test://template/{id}/data",
	}, templateResourceHandler)

	server.AddResource(&mcp.Resource{
		Name:        "watched-resource",
		Description: "A resource that auto-updates every 3 seconds",
		MIMEType:    "text/plain",
		URI:         watchedResourceURI,
	}, watchedResourceHandler)
}

func staticTextHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     "This is the content of the static text resource.",
			},
		},
	}, nil
}

func staticBinaryHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "image/png",
				Blob:     imageData(),
			},
		},
	}, nil
}

// templatePattern is the compiled URI template for the template resource.
var templatePattern = uritemplate.MustNew("test://template/{id}/data")

func templateResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// Extract the ID from the URI using the template pattern
	uri := req.Params.URI
	match := templatePattern.Regexp().FindStringSubmatch(uri)
	id := ""
	if len(match) > 1 {
		id = match[1]
	}

	jsonContent := fmt.Sprintf(`{"id": "%s", "templateTest": true, "data": "Data for ID: %s"}`, id, id)
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      uri,
				MIMEType: "application/json",
				Text:     jsonContent,
			},
		},
	}, nil
}

func watchedResourceHandler(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     "Watched resource content",
			},
		},
	}, nil
}

// =============================================================================
// Prompts
// =============================================================================

func registerPrompts(server *mcp.Server) {
	server.AddPrompt(&mcp.Prompt{
		Name:        "test_simple_prompt",
		Title:       "Simple Test Prompt",
		Description: "A simple prompt without arguments",
	}, simplePromptHandler)

	server.AddPrompt(&mcp.Prompt{
		Name:        "test_prompt_with_arguments",
		Title:       "Prompt With Arguments",
		Description: "A prompt with required arguments",
		Arguments: []*mcp.PromptArgument{
			{Name: "arg1", Description: "First test argument", Required: true},
			{Name: "arg2", Description: "Second test argument", Required: true},
		},
	}, promptWithArgumentsHandler)

	server.AddPrompt(&mcp.Prompt{
		Name:        "test_prompt_with_embedded_resource",
		Title:       "Prompt With Embedded Resource",
		Description: "A prompt that includes an embedded resource",
		Arguments: []*mcp.PromptArgument{
			{Name: "resourceUri", Description: "URI of the resource to embed", Required: true},
		},
	}, promptWithEmbeddedResourceHandler)

	server.AddPrompt(&mcp.Prompt{
		Name:        "test_prompt_with_image",
		Title:       "Prompt With Image",
		Description: "A prompt that includes image content",
	}, promptWithImageHandler)

	server.AddPrompt(&mcp.Prompt{
		Name:        "test_input_required_result_prompt",
		Description: "MRTR (SEP-2322): asks for context via an in-band elicitation request",
	}, testInputRequiredPromptHandler)
}

func simplePromptHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "A simple test prompt",
		Messages: []*mcp.PromptMessage{
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: "This is a simple prompt for testing."},
			},
		},
	}, nil
}

func promptWithArgumentsHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	arg1 := req.Params.Arguments["arg1"]
	arg2 := req.Params.Arguments["arg2"]

	return &mcp.GetPromptResult{
		Description: "A prompt with arguments",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: fmt.Sprintf("Prompt with arguments: arg1='%s', arg2='%s'", arg1, arg2),
				},
			},
		},
	}, nil
}

func promptWithEmbeddedResourceHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	resourceUri := req.Params.Arguments["resourceUri"]

	return &mcp.GetPromptResult{
		Description: "A prompt with an embedded resource",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.EmbeddedResource{
					Resource: &mcp.ResourceContents{
						URI:      resourceUri,
						MIMEType: "text/plain",
						Text:     "Embedded resource content for testing.",
					},
				},
			},
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: "Please process the embedded resource above."},
			},
		},
	}, nil
}

func promptWithImageHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return &mcp.GetPromptResult{
		Description: "A prompt with an image",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.ImageContent{
					Data:     imageData(),
					MIMEType: "image/png",
				},
			},
			{
				Role:    "user",
				Content: &mcp.TextContent{Text: "Please analyze the image above."},
			},
		},
	}, nil
}

// =============================================================================
// Server handlers
// =============================================================================

func completionHandler(ctx context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
	// Return empty completion - just acknowledging the capability
	return &mcp.CompleteResult{
		Completion: mcp.CompletionResultDetails{
			Values: []string{},
			Total:  0,
		},
	}, nil
}

func subscribeHandler(ctx context.Context, req *mcp.SubscribeRequest) error {
	// The SDK handles subscription tracking internally via Server.ResourceUpdated()
	return nil
}

func unsubscribeHandler(ctx context.Context, req *mcp.UnsubscribeRequest) error {
	// The SDK handles subscription tracking internally
	return nil
}

// =============================================================================
// Helper functions
// =============================================================================

// Base64-encoded minimal test files, copied from the typescript conformance example.
const (
	// Minimal 1x1 red PNG image
	testImageBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="
	// Minimal WAV audio file (silence)
	testAudioBase64 = "UklGRiYAAABXQVZFZm10IBAAAAABAAEAQB8AAAB9AAACABAAZGF0YQIAAAA="
)

func imageData() []byte {
	data, err := base64.StdEncoding.DecodeString(testImageBase64)
	if err != nil {
		panic("invalid testImageBase64: " + err.Error())
	}
	return data
}

func audioData() []byte {
	data, err := base64.StdEncoding.DecodeString(testAudioBase64)
	if err != nil {
		panic("invalid testAudioBase64: " + err.Error())
	}
	return data
}
