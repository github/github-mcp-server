package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newLockdownMockGQLHTTPClient(t *testing.T, isPrivate bool, permissions map[string]string) *http.Client {
	t.Helper()

	lowerPermissions := make(map[string]string, len(permissions))
	for user, perm := range permissions {
		lowerPermissions[strings.ToLower(user)] = perm
	}

	return &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/graphql", req.URL.Path)

			bodyBytes, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			_ = req.Body.Close()

			var gqlRequest struct {
				Variables map[string]any `json:"variables"`
			}
			require.NoError(t, json.Unmarshal(bodyBytes, &gqlRequest))

			rawUsername, ok := gqlRequest.Variables["username"]
			require.True(t, ok, "expected username variable in GraphQL request")

			username := fmt.Sprint(rawUsername)
			permission := lowerPermissions[strings.ToLower(username)]

			edges := []any{}
			if permission != "" {
				edges = append(edges, map[string]any{
					"permission": permission,
					"node": map[string]any{
						"login": username,
					},
				})
			}

			response := map[string]any{
				"data": map[string]any{
					"repository": map[string]any{
						"isPrivate": isPrivate,
						"collaborators": map[string]any{
							"edges": edges,
						},
					},
				},
			}

			respBytes, err := json.Marshal(response)
			require.NoError(t, err)

			res := &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(respBytes)),
			}
			res.Header.Set("Content-Type", "application/json")
			return res, nil
		}),
	}
}

// createMCPRequest is a helper function to create a MCP request with the given arguments.
func createMCPRequest(args any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: struct {
			Name      string    `json:"name"`
			Arguments any       `json:"arguments,omitempty"`
			Meta      *mcp.Meta `json:"_meta,omitempty"`
		}{
			Arguments: args,
		},
	}
}

// getTextResult is a helper function that returns a text result from a tool call.
func getTextResult(t *testing.T, result *mcp.CallToolResult) mcp.TextContent {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 1)
	require.IsType(t, mcp.TextContent{}, result.Content[0])
	textContent := result.Content[0].(mcp.TextContent)
	assert.Equal(t, "text", textContent.Type)
	return textContent
}

func getErrorResult(t *testing.T, result *mcp.CallToolResult) mcp.TextContent {
	res := getTextResult(t, result)
	require.True(t, result.IsError, "expected tool call result to be an error")
	return res
}

// getTextResourceResult is a helper function that returns a text result from a tool call.
func getTextResourceResult(t *testing.T, result *mcp.CallToolResult) mcp.TextResourceContents {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 2)
	content := result.Content[1]
	require.IsType(t, mcp.EmbeddedResource{}, content)
	resource := content.(mcp.EmbeddedResource)
	require.IsType(t, mcp.TextResourceContents{}, resource.Resource)
	return resource.Resource.(mcp.TextResourceContents)
}

// getBlobResourceResult is a helper function that returns a blob result from a tool call.
func getBlobResourceResult(t *testing.T, result *mcp.CallToolResult) mcp.BlobResourceContents {
	t.Helper()
	assert.NotNil(t, result)
	require.Len(t, result.Content, 2)
	content := result.Content[1]
	require.IsType(t, mcp.EmbeddedResource{}, content)
	resource := content.(mcp.EmbeddedResource)
	require.IsType(t, mcp.BlobResourceContents{}, resource.Resource)
	return resource.Resource.(mcp.BlobResourceContents)
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
			request := createMCPRequest(tc.args)

			// Test with string type assertion
			if _, isString := tc.expectedVal.(string); isString || tc.errorMsg == "parameter myParam is not of type string, is bool" {
				val, ok, err := OptionalParamOK[string](request, tc.paramName)
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
				val, ok, err := OptionalParamOK[bool](request, tc.paramName)
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
				val, ok, err := OptionalParamOK[float64](request, tc.paramName)
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
