package github

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v82/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_CompareFileContents(t *testing.T) {
	// Verify tool definition and snapshot
	toolDef := CompareFileContents(translations.NullTranslationHelper)
	assert.Equal(t, "compare_file_contents", toolDef.Tool.Name)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)
	assert.Equal(t, "semantic_diff", toolDef.FeatureFlagEnable)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    bool
		expectedErrMsg string
		expectContains []string
	}{
		{
			name: "successful JSON semantic diff",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"": func(w http.ResponseWriter, r *http.Request) {
					path := r.URL.Path
					switch {
					case containsRef(path, "abc123"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"name":"Bob","age":30}`))
					case containsRef(path, "def456"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(`{"name":"Bobby","age":30}`))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				},
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.json",
				"base":  "abc123",
				"head":  "def456",
			},
			expectError: false,
			expectContains: []string{
				"Format: json (semantic diff)",
				`name: "Bob" → "Bobby"`,
			},
		},
		{
			name: "successful YAML semantic diff",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"": func(w http.ResponseWriter, r *http.Request) {
					path := r.URL.Path
					switch {
					case containsRef(path, "v1.0"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("name: Alice\nage: 30\n"))
					case containsRef(path, "v2.0"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("name: Alice\nage: 31\n"))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				},
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "config.yaml",
				"base":  "v1.0",
				"head":  "v2.0",
			},
			expectError: false,
			expectContains: []string{
				"Format: yaml (semantic diff)",
				"age: 30 → 31",
			},
		},
		{
			name: "fallback to unified diff for txt",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"": func(w http.ResponseWriter, r *http.Request) {
					path := r.URL.Path
					switch {
					case containsRef(path, "old"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("line1\nline2\nline3\n"))
					case containsRef(path, "new"):
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte("line1\nmodified\nline3\n"))
					default:
						w.WriteHeader(http.StatusNotFound)
					}
				},
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "readme.txt",
				"base":  "old",
				"head":  "new",
			},
			expectError: false,
			expectContains: []string{
				"unified diff",
				"-line2",
				"+modified",
			},
		},
		{
			name: "base file not found",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				"": func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				},
			}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "missing.json",
				"base":  "abc123",
				"head":  "def456",
			},
			expectError:    true,
			expectedErrMsg: "failed to fetch base file",
		},
		{
			name:         "missing required param owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"repo": "repo",
				"path": "file.json",
				"base": "abc",
				"head": "def",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: owner",
		},
		{
			name:         "missing required param base",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}),
			requestArgs: map[string]any{
				"owner": "owner",
				"repo":  "repo",
				"path":  "file.json",
				"head":  "def",
			},
			expectError:    true,
			expectedErrMsg: "missing required parameter: base",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			mockRawClient := raw.NewClient(client, &url.URL{Scheme: "https", Host: "raw.example.com", Path: "/"})
			deps := BaseDeps{
				Client:    client,
				RawClient: mockRawClient,
			}
			handler := toolDef.Handler(deps)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(ContextWithDeps(context.Background(), deps), &request)

			if tc.expectError {
				require.NoError(t, err)
				textContent := getErrorResult(t, result)
				assert.Contains(t, textContent.Text, tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			require.False(t, result.IsError)
			textContent := getTextResult(t, result)

			for _, expected := range tc.expectContains {
				assert.Contains(t, textContent.Text, expected)
			}
		})
	}
}

// containsRef checks if a URL path contains a specific ref segment.
func containsRef(path, ref string) bool {
	return strings.Contains(path, "/"+ref+"/")
}
