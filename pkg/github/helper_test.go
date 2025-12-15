package github

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type expectations struct {
	path        string
	queryParams map[string]string
	requestBody any
}

// expect is a helper function to create a partial mock that expects various
// request behaviors, such as path, query parameters, and request body.
func expect(t *testing.T, e expectations) *partialMock {
	return &partialMock{
		t:                   t,
		expectedPath:        e.path,
		expectedQueryParams: e.queryParams,
		expectedRequestBody: e.requestBody,
	}
}

// expectPath is a helper function to create a partial mock that expects a
// request with the given path, with the ability to chain a response handler.
func expectPath(t *testing.T, expectedPath string) *partialMock {
	return &partialMock{
		t:            t,
		expectedPath: expectedPath,
	}
}

// expectQueryParams is a helper function to create a partial mock that expects a
// request with the given query parameters, with the ability to chain a response handler.
func expectQueryParams(t *testing.T, expectedQueryParams map[string]string) *partialMock {
	return &partialMock{
		t:                   t,
		expectedQueryParams: expectedQueryParams,
	}
}

// expectRequestBody is a helper function to create a partial mock that expects a
// request with the given body, with the ability to chain a response handler.
func expectRequestBody(t *testing.T, expectedRequestBody any) *partialMock {
	return &partialMock{
		t:                   t,
		expectedRequestBody: expectedRequestBody,
	}
}

type partialMock struct {
	t *testing.T

	expectedPath        string
	expectedQueryParams map[string]string
	expectedRequestBody any
}

func (p *partialMock) andThen(responseHandler http.HandlerFunc) http.HandlerFunc {
	p.t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if p.expectedPath != "" {
			require.Equal(p.t, p.expectedPath, r.URL.Path)
		}

		if p.expectedQueryParams != nil {
			require.Equal(p.t, len(p.expectedQueryParams), len(r.URL.Query()))
			for k, v := range p.expectedQueryParams {
				require.Equal(p.t, v, r.URL.Query().Get(k))
			}
		}

		if p.expectedRequestBody != nil {
			var unmarshaledRequestBody any
			err := json.NewDecoder(r.Body).Decode(&unmarshaledRequestBody)
			require.NoError(p.t, err)

			require.Equal(p.t, p.expectedRequestBody, unmarshaledRequestBody)
		}

		responseHandler(w, r)
	}
}

// mockResponse is a helper function to create a mock HTTP response handler
// that returns a specified status code and marshaled body.
func mockResponse(t *testing.T, code int, body interface{}) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
		// Some tests do not expect to return a JSON object, such as fetching a raw pull request diff,
		// so allow strings to be returned directly.
		s, ok := body.(string)
		if ok {
			_, _ = w.Write([]byte(s))
			return
		}

		b, err := json.Marshal(body)
		require.NoError(t, err)
		_, _ = w.Write(b)
	}
}

// createMCPRequest is a helper function to create a MCP request with the given arguments.
func createMCPRequest(args any) mcp.CallToolRequest {
	// convert args to map[string]interface{} and serialize to JSON
	argsMap, ok := args.(map[string]interface{})
	if !ok {
		argsMap = make(map[string]interface{})
	}

	argsJSON, err := json.Marshal(argsMap)
	if err != nil {
		return mcp.CallToolRequest{}
	}

	jsonRawMessage := json.RawMessage(argsJSON)

	return mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: jsonRawMessage,
		},
	}
}

// getTextResult is a helper function that returns a text result from a tool call.
func getTextResult(t *testing.T, result *mcp.CallToolResult) *mcp.TextContent {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 1)
	textContent, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected content to be of type TextContent")
	return textContent
}

func getErrorResult(t *testing.T, result *mcp.CallToolResult) *mcp.TextContent {
	res := getTextResult(t, result)
	require.True(t, result.IsError, "expected tool call result to be an error")
	return res
}

// getTextResourceResult is a helper function that returns a text result from a tool call.

// getBlobResourceResult is a helper function that returns a blob result from a tool call.

func TestOptionalParamOK(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]interface{}
		paramName   string
		expectedVal interface{}
		expectedOk  bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "present and correct type (string)",
			args:        map[string]interface{}{"myParam": "hello"},
			paramName:   "myParam",
			expectedVal: "hello",
			expectedOk:  true,
			expectError: false,
		},
		{
			name:        "present and correct type (bool)",
			args:        map[string]interface{}{"myParam": true},
			paramName:   "myParam",
			expectedVal: true,
			expectedOk:  true,
			expectError: false,
		},
		{
			name:        "present and correct type (number)",
			args:        map[string]interface{}{"myParam": float64(123)},
			paramName:   "myParam",
			expectedVal: float64(123),
			expectedOk:  true,
			expectError: false,
		},
		{
			name:        "present but wrong type (string expected, got bool)",
			args:        map[string]interface{}{"myParam": true},
			paramName:   "myParam",
			expectedVal: "",   // Zero value for string
			expectedOk:  true, // ok is true because param exists
			expectError: true,
			errorMsg:    "parameter myParam is not of type string, is bool",
		},
		{
			name:        "present but wrong type (bool expected, got string)",
			args:        map[string]interface{}{"myParam": "true"},
			paramName:   "myParam",
			expectedVal: false, // Zero value for bool
			expectedOk:  true,  // ok is true because param exists
			expectError: true,
			errorMsg:    "parameter myParam is not of type bool, is string",
		},
		{
			name:        "parameter not present",
			args:        map[string]interface{}{"anotherParam": "value"},
			paramName:   "myParam",
			expectedVal: "", // Zero value for string
			expectedOk:  false,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Test with string type assertion
			if _, isString := tc.expectedVal.(string); isString || tc.errorMsg == "parameter myParam is not of type string, is bool" {
				val, ok, err := OptionalParamOK[string, map[string]any](tc.args, tc.paramName)
				if tc.expectError {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.errorMsg)
					assert.Equal(t, tc.expectedOk, ok)   // Check ok even on error
					assert.Equal(t, tc.expectedVal, val) // Check zero value on error
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedOk, ok)
					assert.Equal(t, tc.expectedVal, val)
				}
			}

			// Test with bool type assertion
			if _, isBool := tc.expectedVal.(bool); isBool || tc.errorMsg == "parameter myParam is not of type bool, is string" {
				val, ok, err := OptionalParamOK[bool, map[string]any](tc.args, tc.paramName)
				if tc.expectError {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.errorMsg)
					assert.Equal(t, tc.expectedOk, ok)   // Check ok even on error
					assert.Equal(t, tc.expectedVal, val) // Check zero value on error
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedOk, ok)
					assert.Equal(t, tc.expectedVal, val)
				}
			}

			// Test with float64 type assertion (for number case)
			if _, isFloat := tc.expectedVal.(float64); isFloat {
				val, ok, err := OptionalParamOK[float64, map[string]any](tc.args, tc.paramName)
				if tc.expectError {
					// This case shouldn't happen for float64 in the defined tests
					require.Fail(t, "Unexpected error case for float64")
				} else {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedOk, ok)
					assert.Equal(t, tc.expectedVal, val)
				}
			}
		})
	}
}

func getResourceResult(t *testing.T, result *mcp.CallToolResult) *mcp.ResourceContents {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 2)
	content := result.Content[1]
	require.IsType(t, &mcp.EmbeddedResource{}, content)
	resource, ok := content.(*mcp.EmbeddedResource)
	require.True(t, ok, "expected content to be of type EmbeddedResource")

	require.IsType(t, &mcp.ResourceContents{}, resource.Resource)
	return resource.Resource
}

// MockRoundTripper is a mock HTTP transport using testify/mock
type MockRoundTripper struct {
	mock.Mock
	handlers map[string]http.HandlerFunc
}

// NewMockRoundTripper creates a new mock round tripper
func NewMockRoundTripper() *MockRoundTripper {
	return &MockRoundTripper{
		handlers: make(map[string]http.HandlerFunc),
	}
}

// RoundTrip implements the http.RoundTripper interface
func (m *MockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Normalize the request path and method for matching
	key := req.Method + " " + req.URL.Path

	// Check if we have a specific handler for this request
	if handler, ok := m.handlers[key]; ok {
		// Use httptest.ResponseRecorder to capture the handler's response
		recorder := &responseRecorder{
			header: make(http.Header),
			body:   &bytes.Buffer{},
		}
		handler(recorder, req)

		return &http.Response{
			StatusCode: recorder.statusCode,
			Header:     recorder.header,
			Body:       io.NopCloser(bytes.NewReader(recorder.body.Bytes())),
			Request:    req,
		}, nil
	}

	// Fall back to mock.Mock assertions if defined
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// On registers an expectation using testify/mock
func (m *MockRoundTripper) OnRequest(method, path string, handler http.HandlerFunc) *MockRoundTripper {
	key := method + " " + path
	m.handlers[key] = handler
	return m
}

// NewMockHTTPClient creates an HTTP client with a mock transport
func NewMockHTTPClient() (*http.Client, *MockRoundTripper) {
	transport := NewMockRoundTripper()
	client := &http.Client{Transport: transport}
	return client, transport
}

// responseRecorder is a simple response recorder for the mock transport
type responseRecorder struct {
	statusCode int
	header     http.Header
	body       *bytes.Buffer
}

func (r *responseRecorder) Header() http.Header {
	return r.header
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	if r.statusCode == 0 {
		r.statusCode = http.StatusOK
	}
	return r.body.Write(data)
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
}

// matchPath checks if a request path matches a pattern (supports simple wildcards)
func matchPath(pattern, path string) bool {
	// Simple exact match for now
	if pattern == path {
		return true
	}

	// Support for path parameters like /repos/{owner}/{repo}/issues/{issue_number}
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return false
	}

	for i := range patternParts {
		// Check if this is a path parameter (enclosed in {})
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			continue // Path parameters match anything
		}
		if patternParts[i] != pathParts[i] {
			return false
		}
	}

	return true
}

// executeHandler executes an HTTP handler and returns the response
func executeHandler(handler http.HandlerFunc, req *http.Request) *http.Response {
	recorder := &responseRecorder{
		header: make(http.Header),
		body:   &bytes.Buffer{},
	}
	handler(recorder, req)

	return &http.Response{
		StatusCode: recorder.statusCode,
		Header:     recorder.header,
		Body:       io.NopCloser(bytes.NewReader(recorder.body.Bytes())),
		Request:    req,
	}
}

// MockHTTPClientWithHandler creates an HTTP client with a single handler function
func MockHTTPClientWithHandler(handler http.HandlerFunc) *http.Client {
	handlers := map[string]http.HandlerFunc{
		"": handler, // Empty key acts as catch-all
	}
	return MockHTTPClientWithHandlers(handlers)
}

// MockHTTPClientWithHandlers creates an HTTP client with multiple handlers for different paths
func MockHTTPClientWithHandlers(handlers map[string]http.HandlerFunc) *http.Client {
	transport := &multiHandlerTransport{handlers: handlers}
	return &http.Client{Transport: transport}
}

type multiHandlerTransport struct {
	handlers map[string]http.HandlerFunc
}

func (m *multiHandlerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Check for catch-all handler
	if handler, ok := m.handlers[""]; ok {
		return executeHandler(handler, req), nil
	}

	// Try to find a handler for this request
	key := req.Method + " " + req.URL.Path

	// First try exact match
	if handler, ok := m.handlers[key]; ok {
		return executeHandler(handler, req), nil
	}

	// Then try pattern matching
	for pattern, handler := range m.handlers {
		if pattern == "" {
			continue // Skip catch-all
		}
		parts := strings.SplitN(pattern, " ", 2)
		if len(parts) == 2 {
			method, pathPattern := parts[0], parts[1]
			if req.Method == method && matchPath(pathPattern, req.URL.Path) {
				return executeHandler(handler, req), nil
			}
		}
	}

	// No handler found
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(bytes.NewReader([]byte("not found"))),
		Request:    req,
	}, nil
}

// extractPathParams extracts path parameters from a URL path given a pattern
func extractPathParams(pattern, path string) map[string]string {
	params := make(map[string]string)
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	pathParts := strings.Split(strings.Trim(path, "/"), "/")

	if len(patternParts) != len(pathParts) {
		return params
	}

	for i := range patternParts {
		if strings.HasPrefix(patternParts[i], "{") && strings.HasSuffix(patternParts[i], "}") {
			paramName := strings.Trim(patternParts[i], "{}")
			params[paramName] = pathParts[i]
		}
	}

	return params
}

// ParseRequestPath is a helper to extract path parameters
func ParseRequestPath(t *testing.T, req *http.Request, pattern string) url.Values {
	t.Helper()
	params := extractPathParams(pattern, req.URL.Path)
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values
}
