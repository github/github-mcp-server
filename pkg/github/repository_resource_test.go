package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/raw"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// errorTransport is a http.RoundTripper that always returns an error.
type errorTransport struct {
	err error
}

func (t *errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

type resourceResponseType int

const (
	resourceResponseTypeUnknown resourceResponseType = iota
	resourceResponseTypeBlob
	resourceResponseTypeText
)

func TestExpandRepoResourceURIUsesUTF8PercentEncoding(t *testing.T) {
	sha := strings.Repeat("a", 40)
	tests := []struct {
		name     string
		sha      string
		ref      string
		path     []string
		expected string
	}{
		{
			name: "sha path",
			sha:  sha,
			path: []string{"test", "Þfoo.go"},
			expected: "repo://owner/repo/sha/" + sha +
				"/contents/test/%C3%9Efoo.go",
		},
		{
			name: "branch path",
			ref:  "refs/heads/main",
			path: []string{"ملف.txt"},
			expected: "repo://owner/repo/refs/heads/main/contents/" +
				"%D9%85%D9%84%D9%81.txt",
		},
		{
			name: "unicode branch and path",
			ref:  "refs/heads/分支",
			path: []string{"日本語 ファイル.md"},
			expected: "repo://owner/repo/refs/heads/%E5%88%86%E6%94%AF/contents/" +
				"%E6%97%A5%E6%9C%AC%E8%AA%9E%20%E3%83%95%E3%82%A1%E3%82%A4%E3%83%AB.md",
		},
		{
			name:     "unicode tag",
			ref:      "refs/tags/v1-Þ",
			path:     []string{"README.md"},
			expected: "repo://owner/repo/refs/tags/v1-%C3%9E/contents/README.md",
		},
		{
			name:     "pull request ref",
			ref:      "refs/pull/42/head",
			path:     []string{"𝄞.txt"},
			expected: "repo://owner/repo/refs/pull/42/head/contents/%F0%9D%84%9E.txt",
		},
		{
			name:     "reserved path characters",
			path:     []string{"a%b", "query?value"},
			expected: "repo://owner/repo/contents/a%25b/query%3Fvalue",
		},
		{
			name:     "ascii path remains unchanged",
			path:     []string{"docs", "README.md"},
			expected: "repo://owner/repo/contents/docs/README.md",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := expandRepoResourceURI("owner", "repo", tc.sha, tc.ref, tc.path)
			require.NoError(t, err)
			require.Equal(t, tc.expected, actual)
		})
	}
}

func TestRepositoryResourceContentsDecodesUTF8Path(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	sha := strings.Repeat("a", 40)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		GetRawReposContentsByOwnerByRepoBySHAByPath: func(w http.ResponseWriter, r *http.Request) {
			require.True(t, strings.HasSuffix(r.URL.Path, "/test/Þfoo.go"))
			w.Header().Set("Content-Type", "text/plain")
			_, err := w.Write([]byte("package x"))
			require.NoError(t, err)
		},
	}))
	rawClient, err := raw.NewClient(client, base)
	require.NoError(t, err)
	ctx := ContextWithDeps(context.Background(), BaseDeps{Client: client, RawClient: rawClient})
	handler := RepositoryResourceContentsHandler(repositoryResourceCommitContentURITemplate)

	result, err := handler(ctx, &mcp.ReadResourceRequest{Params: &mcp.ReadResourceParams{
		URI: "repo://owner/repo/sha/" + sha + "/contents/test/%C3%9Efoo.go",
	}})
	require.NoError(t, err)
	require.Equal(t, "package x", result.Contents[0].Text)
}

func Test_repositoryResourceContents(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	tests := []struct {
		name                 string
		mockedClient         *http.Client
		uri                  string
		handlerFn            func() mcp.ResourceHandler
		expectedResponseType resourceResponseType
		expectError          string
		expectedResult       *mcp.ReadResourceResult
	}{
		{
			name: "missing owner",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo:///repo/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "owner is required",
		},
		{
			name: "missing repo",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByBranchByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner//refs/heads/main/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceBranchContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "repo is required",
		},
		{
			name: "successful blob content fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "image/png")
					_, err := w.Write([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01"))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/contents/data.png",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeBlob,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Blob:     []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01"),
					MIMEType: "image/png",
					URI:      "",
				}}},
		},
		{
			name: "successful text content fetch (HEAD)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Text:     "# Test Repository\n\nThis is a test repository.",
					MIMEType: "text/markdown",
					URI:      "",
				}}},
		},
		{
			name: "successful text content fetch (HEAD)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByPath: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")

					require.Contains(t, r.URL.Path, "pkg/github/actions.go")
					_, err := w.Write([]byte("package actions\n\nfunc main() {\n    // Sample Go file content\n}\n"))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/contents/pkg/github/actions.go",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Text:     "package actions\n\nfunc main() {\n    // Sample Go file content\n}\n",
					MIMEType: "text/plain",
					URI:      "",
				}}},
		},
		{
			name: "successful text content fetch (branch)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByBranchByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/refs/heads/main/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceBranchContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Text:     "# Test Repository\n\nThis is a test repository.",
					MIMEType: "text/markdown",
					URI:      "",
				}}},
		},
		{
			name: "successful text content fetch (tag)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByTagByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/refs/tags/v1.0.0/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceTagContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Text:     "# Test Repository\n\nThis is a test repository.",
					MIMEType: "text/markdown",
					URI:      "",
				}}},
		},
		{
			name: "successful text content fetch (sha)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoBySHAByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/sha/abc123/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceCommitContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Text:     "# Test Repository\n\nThis is a test repository.",
					MIMEType: "text/markdown",
					URI:      "",
				}}},
		},
		{
			name: "successful text content fetch (pr)",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposPullsByOwnerByRepoByPullNumber: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_, err := w.Write([]byte(`{"head": {"sha": "abc123"}}`))
					require.NoError(t, err)
				}),
				GetRawReposContentsByOwnerByRepoBySHAByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "text/markdown")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/refs/pull/42/head/contents/README.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourcePrContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Text:     "# Test Repository\n\nThis is a test repository.",
					MIMEType: "text/markdown",
					URI:      "",
				}}},
		},
		{
			name: "content fetch fails",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetReposContentsByOwnerByRepoByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"message": "Not Found"}`))
				}),
			}),
			uri: "repo://owner/repo/contents/nonexistent.md",
			handlerFn: func() mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "404 Not Found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := mustNewGHClient(t, tc.mockedClient)
			mockRawClient, err := raw.NewClient(client, base)
			require.NoError(t, err)
			deps := BaseDeps{
				Client:    client,
				RawClient: mockRawClient,
			}
			ctx := ContextWithDeps(context.Background(), deps)
			handler := tc.handlerFn()

			request := &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{
					URI: tc.uri,
				},
			}

			resp, err := handler(ctx, request)

			if tc.expectError != "" {
				require.ErrorContains(t, err, tc.expectError)
				return
			}

			require.NoError(t, err)

			content := resp.Contents[0]
			switch tc.expectedResponseType {
			case resourceResponseTypeBlob:
				require.Equal(t, tc.expectedResult.Contents[0].Blob, content.Blob)

				wireBytes, err := json.Marshal(resp)
				require.NoError(t, err)
				var wireResult struct {
					Contents []struct {
						Blob string `json:"blob"`
					} `json:"contents"`
				}
				require.NoError(t, json.Unmarshal(wireBytes, &wireResult))
				require.Len(t, wireResult.Contents, 1)
				decodedBlob, err := base64.StdEncoding.DecodeString(wireResult.Contents[0].Blob)
				require.NoError(t, err)
				require.Equal(t, tc.expectedResult.Contents[0].Blob, decodedBlob)
				require.True(t, strings.HasPrefix(string(decodedBlob), "\x89PNG\r\n\x1a\n"))
			case resourceResponseTypeText:
				require.Equal(t, tc.expectedResult.Contents[0].Text, content.Text)
			default:
				t.Fatalf("unknown expectedResponseType %v", tc.expectedResponseType)
			}
		})
	}
}

// Test_repositoryResourceContentsHandler_NetworkError tests that a network error
// during raw content fetch does not cause a panic (nil response body dereference).
func Test_repositoryResourceContentsHandler_NetworkError(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	networkErr := errors.New("network error: connection refused")

	httpClient := &http.Client{Transport: &errorTransport{err: networkErr}}
	client := mustNewGHClient(t, httpClient)
	mockRawClient, err := raw.NewClient(client, base)
	require.NoError(t, err)
	deps := BaseDeps{
		Client:    client,
		RawClient: mockRawClient,
	}
	ctx := ContextWithDeps(context.Background(), deps)

	handler := RepositoryResourceContentsHandler(repositoryResourceContentURITemplate)

	request := &mcp.ReadResourceRequest{
		Params: &mcp.ReadResourceParams{
			URI: "repo://owner/repo/contents/README.md",
		},
	}

	// This should not panic, even though the HTTP client returns an error
	resp, err := handler(ctx, request)
	require.Error(t, err)
	require.Nil(t, resp)
	require.ErrorContains(t, err, "failed to get raw content")
}
