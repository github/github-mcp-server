package github

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
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
	require.NoError(nil, err)

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
func getTextResourceResult(t *testing.T, result *mcp.CallToolResult) *mcp.ResourceContents {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 2)
	content := result.Content[1]
	require.IsType(t, mcp.EmbeddedResource{}, content)
	resource, ok := content.(*mcp.EmbeddedResource)
	require.True(t, ok, "expected content to be of type EmbeddedResource")

	require.IsType(t, mcp.ResourceContents{}, resource.Resource)
	require.NotEmpty(t, resource.Resource.Text)
	return resource.Resource
}

// getBlobResourceResult is a helper function that returns a blob result from a tool call.
func getBlobResourceResult(t *testing.T, result *mcp.CallToolResult) *mcp.ResourceContents {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 2)
	content := result.Content[1]
	require.IsType(t, mcp.EmbeddedResource{}, content)

	resource := content.(*mcp.EmbeddedResource)
	require.IsType(t, mcp.ResourceContents{}, resource.Resource)
	require.NotEmpty(t, resource.Resource.Blob)
	return resource.Resource
}

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
