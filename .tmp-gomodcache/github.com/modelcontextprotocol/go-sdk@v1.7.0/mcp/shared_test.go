// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestValidateRequestMeta(t *testing.T) {
	tests := []struct {
		name            string
		method          string
		params          any
		wantUsesNew     bool
		wantLogLevel    LoggingLevel
		wantErrContains string
	}{
		{
			name:        "no params: old protocol",
			method:      methodListTools,
			params:      nil,
			wantUsesNew: false,
		},
		{
			name:        "no _meta: old protocol",
			method:      methodCallTool,
			params:      map[string]any{"name": "x"},
			wantUsesNew: false,
		},
		{
			name:   "_meta without protocolVersion: old protocol",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{"otherKey": "v"},
				"name":  "x",
			},
			wantUsesNew: false,
		},
		{
			name:   "new protocol with all required fields",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
					MetaKeyClientCapabilities: map[string]any{},
				},
				"name": "x",
			},
			wantUsesNew: true,
		},
		{
			name:   "new protocol missing clientInfo",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientCapabilities: map[string]any{},
				},
				"name": "x",
			},
			wantUsesNew: true,
		},
		{
			name:   "new protocol invalid clientInfo",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientInfo:         "not an object",
					MetaKeyClientCapabilities: map[string]any{},
				},
				"name": "x",
			},
			wantUsesNew:     false,
			wantErrContains: MetaKeyClientInfo,
		},
		{
			name:   "new protocol missing clientCapabilities",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion: protocolVersion20260728,
					MetaKeyClientInfo:      map[string]any{"name": "c", "version": "1"},
				},
				"name": "x",
			},
			wantUsesNew:     false,
			wantErrContains: MetaKeyClientCapabilities,
		},
		{
			name:        "malformed _meta is ignored",
			method:      methodCallTool,
			params:      json.RawMessage(`{"_meta": "not an object", "name": "x"}`),
			wantUsesNew: false,
		},
		{
			name:   "new protocol with logLevel",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
					MetaKeyClientCapabilities: map[string]any{},
					MetaKeyLogLevel:           "warning",
				},
				"name": "x",
			},
			wantUsesNew:  true,
			wantLogLevel: "warning",
		},
		{
			name:   "new protocol without logLevel",
			method: methodCallTool,
			params: map[string]any{
				"_meta": map[string]any{
					MetaKeyProtocolVersion:    protocolVersion20260728,
					MetaKeyClientInfo:         map[string]any{"name": "c", "version": "1"},
					MetaKeyClientCapabilities: map[string]any{},
				},
				"name": "x",
			},
			wantUsesNew:  true,
			wantLogLevel: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var raw json.RawMessage
			switch p := tc.params.(type) {
			case json.RawMessage:
				raw = p
			default:
				raw = mustMarshal(tc.params)
			}
			id, err := jsonrpc.MakeID("test")
			if err != nil {
				t.Fatal(err)
			}
			req := &jsonrpc.Request{Method: tc.method, Params: raw, ID: id}

			vmeta, err := validateRequestMeta(req)
			usesNew := vmeta != nil && vmeta.usesNewProtocol
			if usesNew != tc.wantUsesNew {
				t.Errorf("usesNewProtocol = %v, want %v", usesNew, tc.wantUsesNew)
			}
			if vmeta != nil && vmeta.logLevel != tc.wantLogLevel {
				t.Errorf("logLevel = %q, want %q", vmeta.logLevel, tc.wantLogLevel)
			}
			if tc.wantErrContains == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrContains)
			}
			var jerr *jsonrpc.Error
			if !errors.As(err, &jerr) {
				t.Fatalf("expected *jsonrpc.Error, got %T: %v", err, err)
			}
			if jerr.Code != jsonrpc.CodeInvalidParams {
				t.Errorf("error code = %d, want %d", jerr.Code, jsonrpc.CodeInvalidParams)
			}
			if !strings.Contains(jerr.Message, tc.wantErrContains) {
				t.Errorf("error message %q does not contain %q", jerr.Message, tc.wantErrContains)
			}
		})
	}
}

func TestServerRequest_PerRequestAccessors(t *testing.T) {
	// A request carrying the new-protocol _meta fields populates the
	// accessors with values from _meta.
	caps := &ClientCapabilities{Sampling: &SamplingCapabilities{}}
	info := &Implementation{Name: "c", Version: "1"}
	params := &CallToolParamsRaw{
		Meta: Meta{
			MetaKeyProtocolVersion:    protocolVersion20260728,
			MetaKeyClientInfo:         info,
			MetaKeyClientCapabilities: caps,
		},
		Name: "x",
	}
	req := &ServerRequest[*CallToolParamsRaw]{Params: params}
	if got := req.ProtocolVersion(); got != protocolVersion20260728 {
		t.Errorf("ProtocolVersion = %q, want %q", got, protocolVersion20260728)
	}
	if got := req.ClientInfo(); got == nil || got.Name != "c" {
		t.Errorf("ClientInfo = %+v, want Name=c", got)
	}
	if got := req.ClientCapabilities(); got == nil || got.Sampling == nil {
		t.Errorf("ClientCapabilities = %+v, want non-nil Sampling", got)
	}
}

func TestServerRequest_PerRequestAccessors_FromJSON(t *testing.T) {
	// Values arriving over the wire are JSON maps; the accessors should
	// re-decode them into typed Go values.
	raw := json.RawMessage(`{
		"_meta": {
			"io.modelcontextprotocol/protocolVersion": "2026-07-28",
			"io.modelcontextprotocol/clientInfo": {"name": "wire-client", "version": "9"},
			"io.modelcontextprotocol/clientCapabilities": {"sampling": {}}
		},
		"name": "tool"
	}`)
	var params CallToolParamsRaw
	if err := json.Unmarshal(raw, &params); err != nil {
		t.Fatal(err)
	}
	req := &ServerRequest[*CallToolParamsRaw]{Params: &params}
	if got, want := req.ProtocolVersion(), protocolVersion20260728; got != want {
		t.Errorf("ProtocolVersion = %q, want %q", got, want)
	}
	gotInfo := req.ClientInfo()
	wantInfo := &Implementation{Name: "wire-client", Version: "9"}
	if diff := cmp.Diff(wantInfo, gotInfo); diff != "" {
		t.Errorf("ClientInfo mismatch (-want +got):\n%s", diff)
	}
	gotCaps := req.ClientCapabilities()
	if gotCaps == nil || gotCaps.Sampling == nil {
		t.Errorf("ClientCapabilities = %+v, want non-nil Sampling", gotCaps)
	}
}

func TestServerRequest_PerRequestAccessors_FallbackToInitializeParams(t *testing.T) {
	// With no _meta on the request, accessors must fall back to the
	// session's InitializeParams (the old-protocol path).
	ss := &ServerSession{}
	ss.state.InitializeParams = &InitializeParams{
		ProtocolVersion: protocolVersion20251125,
		ClientInfo:      &Implementation{Name: "old", Version: "0"},
		Capabilities:    &ClientCapabilities{Elicitation: &ElicitationCapabilities{}},
	}
	req := &ServerRequest[*CallToolParamsRaw]{
		Session: ss,
		Params:  &CallToolParamsRaw{Name: "x"},
	}
	if got, want := req.ProtocolVersion(), protocolVersion20251125; got != want {
		t.Errorf("ProtocolVersion fallback = %q, want %q", got, want)
	}
	if got := req.ClientInfo(); got == nil || got.Name != "old" {
		t.Errorf("ClientInfo fallback = %+v, want Name=old", got)
	}
	if got := req.ClientCapabilities(); got == nil || got.Elicitation == nil {
		t.Errorf("ClientCapabilities fallback = %+v, want non-nil Elicitation", got)
	}
}

func TestServerRequest_PerRequestAccessors_Empty(t *testing.T) {
	// With no _meta and no session, accessors return zero values.
	req := &ServerRequest[*CallToolParamsRaw]{
		Params: &CallToolParamsRaw{Name: "x"},
	}
	if got := req.ProtocolVersion(); got != "" {
		t.Errorf("ProtocolVersion = %q, want empty", got)
	}
	if got := req.ClientInfo(); got != nil {
		t.Errorf("ClientInfo = %+v, want nil", got)
	}
	if got := req.ClientCapabilities(); got != nil {
		t.Errorf("ClientCapabilities = %+v, want nil", got)
	}
}

func TestImplementationDescriptionJSON(t *testing.T) {
	impl := &Implementation{
		Name:        "greeter",
		Title:       "Greeter",
		Description: "Example server for greeting tools",
		Version:     "v1.0.0",
	}
	got, err := json.Marshal(impl)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"greeter","title":"Greeter","description":"Example server for greeting tools","version":"v1.0.0"}`
	if string(got) != want {
		t.Fatalf("Implementation JSON = %s, want %s", got, want)
	}

	var roundTrip Implementation
	if err := json.Unmarshal(got, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(impl, &roundTrip); diff != "" {
		t.Fatalf("Implementation round trip mismatch (-want +got):\n%s", diff)
	}

	got, err = json.Marshal(&Implementation{Name: "greeter", Version: "v1.0.0"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "description") {
		t.Fatalf("empty description should be omitted, got %s", got)
	}
}

// TODO(v0.3.0): rewrite this test.
// func TestToolValidate(t *testing.T) {
// 	// Check that the tool returned from NewServerTool properly validates its input schema.

// 	type req struct {
// 		I int
// 		B bool
// 		S string `json:",omitempty"`
// 		P *int   `json:",omitempty"`
// 	}

// 	dummyHandler := func(context.Context, *CallToolRequest, req) (*CallToolResultFor[any], error) {
// 		return nil, nil
// 	}

// 	st, err := newServerTool(&Tool{Name: "test", Description: "test"}, dummyHandler)
// 	if err != nil {
// 		t.Fatal(err)
// 	}

// 	for _, tt := range []struct {
// 		desc string
// 		args map[string]any
// 		want string // error should contain this string; empty for success
// 	}{
// 		{
// 			"both required",
// 			map[string]any{"I": 1, "B": true},
// 			"",
// 		},
// 		{
// 			"optional",
// 			map[string]any{"I": 1, "B": true, "S": "foo"},
// 			"",
// 		},
// 		{
// 			"wrong type",
// 			map[string]any{"I": 1.5, "B": true},
// 			"cannot unmarshal",
// 		},
// 		{
// 			"extra property",
// 			map[string]any{"I": 1, "B": true, "C": 2},
// 			"unknown field",
// 		},
// 		{
// 			"value for pointer",
// 			map[string]any{"I": 1, "B": true, "P": 3},
// 			"",
// 		},
// 		{
// 			"null for pointer",
// 			map[string]any{"I": 1, "B": true, "P": nil},
// 			"",
// 		},
// 	} {
// 		t.Run(tt.desc, func(t *testing.T) {
// 			raw, err := json.Marshal(tt.args)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			_, err = st.handler(context.Background(), &ServerRequest[*CallToolParamsFor[json.RawMessage]]{
// 				Params: &CallToolParamsFor[json.RawMessage]{Arguments: json.RawMessage(raw)},
// 			})
// 			if err == nil && tt.want != "" {
// 				t.Error("got success, wanted failure")
// 			}
// 			if err != nil {
// 				if tt.want == "" {
// 					t.Fatalf("failed with:\n%s\nwanted success", err)
// 				}
// 				if !strings.Contains(err.Error(), tt.want) {
// 					t.Fatalf("got:\n%s\nwanted to contain %q", err, tt.want)
// 				}
// 			}
// 		})
// 	}
// }

// TestNilParamsHandling tests that nil parameters don't cause panic in unmarshalParams.
// This addresses a vulnerability where missing or null parameters could crash the server.
// func TestNilParamsHandling(t *testing.T) {
// 	// Define test types for clarity
// 	type TestArgs struct {
// 		Name  string `json:"name"`
// 		Value int    `json:"value"`
// 	}

// 	// Simple test handler
// 	testHandler := func(ctx context.Context, req *ServerRequest[**GetPromptParams]) (*GetPromptResult, error) {
// 		result := "processed: " + req.Params.Arguments.Name
// 		return &CallToolResultFor[string]{StructuredContent: result}, nil
// 	}

// 	methodInfo := newServerMethodInfo(testHandler, missingParamsOK)

// 	// Helper function to test that unmarshalParams doesn't panic and handles nil gracefully
// 	mustNotPanic := func(t *testing.T, rawMsg json.RawMessage, expectNil bool) Params {
// 		t.Helper()

// 		defer func() {
// 			if r := recover(); r != nil {
// 				t.Fatalf("unmarshalParams panicked: %v", r)
// 			}
// 		}()

// 		params, err := methodInfo.unmarshalParams(rawMsg)
// 		if err != nil {
// 			t.Fatalf("unmarshalParams failed: %v", err)
// 		}

// 		if expectNil {
// 			if params != nil {
// 				t.Fatalf("Expected nil params, got %v", params)
// 			}
// 			return params
// 		}

// 		if params == nil {
// 			t.Fatal("unmarshalParams returned unexpected nil")
// 		}

// 		// Verify the result can be used safely
// 		typedParams := params.(TestParams)
// 		_ = typedParams.Name
// 		_ = typedParams.Arguments.Name
// 		_ = typedParams.Arguments.Value

// 		return params
// 	}

// 	// Test different nil parameter scenarios - with missingParamsOK flag, nil/null should return nil
// 	t.Run("missing_params", func(t *testing.T) {
// 		mustNotPanic(t, nil, true) // Expect nil with missingParamsOK flag
// 	})

// 	t.Run("explicit_null", func(t *testing.T) {
// 		mustNotPanic(t, json.RawMessage(`null`), true) // Expect nil with missingParamsOK flag
// 	})

// 	t.Run("empty_object", func(t *testing.T) {
// 		mustNotPanic(t, json.RawMessage(`{}`), false) // Empty object should create valid params
// 	})

// 	t.Run("valid_params", func(t *testing.T) {
// 		rawMsg := json.RawMessage(`{"name":"test","arguments":{"name":"hello","value":42}}`)
// 		params := mustNotPanic(t, rawMsg, false)

// 		// For valid params, also verify the values are parsed correctly
// 		typedParams := params.(TestParams)
// 		if typedParams.Name != "test" {
// 			t.Errorf("Expected name 'test', got %q", typedParams.Name)
// 		}
// 		if typedParams.Arguments.Name != "hello" {
// 			t.Errorf("Expected argument name 'hello', got %q", typedParams.Arguments.Name)
// 		}
// 		if typedParams.Arguments.Value != 42 {
// 			t.Errorf("Expected argument value 42, got %d", typedParams.Arguments.Value)
// 		}
// 	})
// }

// TestNilParamsEdgeCases tests edge cases to ensure we don't over-fix
// func TestNilParamsEdgeCases(t *testing.T) {
// 	type TestArgs struct {
// 		Name  string `json:"name"`
// 		Value int    `json:"value"`
// 	}
// 	type TestParams = *CallToolParamsFor[TestArgs]

// 	testHandler := func(context.Context, *ServerRequest[TestParams]) (*CallToolResultFor[string], error) {
// 		return &CallToolResultFor[string]{StructuredContent: "test"}, nil
// 	}

// 	methodInfo := newServerMethodInfo(testHandler, missingParamsOK)

// 	// These should fail normally, not be treated as nil params
// 	invalidCases := []json.RawMessage{
// 		json.RawMessage(""),       // empty string - should error
// 		json.RawMessage("[]"),     // array - should error
// 		json.RawMessage(`"null"`), // string "null" - should error
// 		json.RawMessage("0"),      // number - should error
// 		json.RawMessage("false"),  // boolean - should error
// 	}

// 	for i, rawMsg := range invalidCases {
// 		t.Run(fmt.Sprintf("invalid_case_%d", i), func(t *testing.T) {
// 			params, err := methodInfo.unmarshalParams(rawMsg)
// 			if err == nil && params == nil {
// 				t.Error("Should not return nil params without error")
// 			}
// 		})
// 	}

// 	// Test that methods without missingParamsOK flag properly reject nil params
// 	t.Run("reject_when_params_required", func(t *testing.T) {
// 		methodInfoStrict := newServerMethodInfo(testHandler, 0) // No missingParamsOK flag

// 		testCases := []struct {
// 			name   string
// 			params json.RawMessage
// 		}{
// 			{"nil_params", nil},
// 			{"null_params", json.RawMessage(`null`)},
// 		}

// 		for _, tc := range testCases {
// 			t.Run(tc.name, func(t *testing.T) {
// 				_, err := methodInfoStrict.unmarshalParams(tc.params)
// 				if err == nil {
// 					t.Error("Expected error for required params, got nil")
// 				}
// 				if !strings.Contains(err.Error(), "missing required \"params\"") {
// 					t.Errorf("Expected 'missing required params' error, got: %v", err)
// 				}
// 			})
// 		}
// 	})
// }

// TestUnmarshalParamsInvalidParamsCode is a regression test for
// https://github.com/modelcontextprotocol/go-sdk/issues/976#issuecomment-4829124838.
// Malformed params for a registered method must surface as a JSON-RPC error
// whose code is CodeInvalidParams (-32602) rather than the zero-value 0.
func TestUnmarshalParamsInvalidParamsCode(t *testing.T) {
	info, ok := serverMethodInfos[methodPing]
	if !ok {
		t.Fatalf("no methodInfo for %q", methodPing)
	}
	// Array where a struct is expected.
	_, err := info.unmarshalParams(json.RawMessage(`["a","b"]`))
	if err == nil {
		t.Fatal("unmarshalParams returned nil error for malformed params")
	}
	var jerr *jsonrpc.Error
	if !errors.As(err, &jerr) {
		t.Fatalf("expected *jsonrpc.Error in chain, got %T: %v", err, err)
	}
	if jerr.Code != jsonrpc.CodeInvalidParams {
		t.Errorf("error code = %d, want %d (CodeInvalidParams)", jerr.Code, jsonrpc.CodeInvalidParams)
	}
	if !strings.Contains(err.Error(), "unmarshaling") {
		t.Errorf("error message = %q, want it to mention unmarshaling", err.Error())
	}
}
