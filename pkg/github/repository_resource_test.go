package github

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/google/go-github/v79/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type resourceResponseType int

const (
	resourceResponseTypeUnknown resourceResponseType = iota
	resourceResponseTypeBlob
	resourceResponseTypeText
)

func Test_repositoryResourceContents(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	tests := []struct {
		name                 string
		mockedClient         *http.Client
		uri                  string
		handlerFn            func(deps ToolDependencies) mcp.ResourceHandler
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceBranchContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "repo is required",
		},
		{
			name: "successful blob content fetch",
			mockedClient: MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
				GetRawReposContentsByOwnerByRepoByPath: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Type", "image/png")
					_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
					require.NoError(t, err)
				}),
			}),
			uri: "repo://owner/repo/contents/data.png",
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeBlob,
			expectedResult: &mcp.ReadResourceResult{
				Contents: []*mcp.ResourceContents{{
					Blob:     []byte("IyBUZXN0IFJlcG9zaXRvcnkKClRoaXMgaXMgYSB0ZXN0IHJlcG9zaXRvcnku"),
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceBranchContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceTagContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceCommitContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourcePrContentURITemplate)
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
			handlerFn: func(deps ToolDependencies) mcp.ResourceHandler {
				return RepositoryResourceContentsHandler(deps, repositoryResourceContentURITemplate)
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "404 Not Found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			mockRawClient := raw.NewClient(client, base)
			deps := BaseDeps{
				Client:    client,
				RawClient: mockRawClient,
			}
			handler := tc.handlerFn(deps)

			request := &mcp.ReadResourceRequest{
				Params: &mcp.ReadResourceParams{
					URI: tc.uri,
				},
			}

			resp, err := handler(context.TODO(), request)

			if tc.expectError != "" {
				require.ErrorContains(t, err, tc.expectError)
				return
			}

			require.NoError(t, err)

			content := resp.Contents[0]
			switch tc.expectedResponseType {
			case resourceResponseTypeBlob:
				require.Equal(t, tc.expectedResult.Contents[0].Blob, content.Blob)
			case resourceResponseTypeText:
				require.Equal(t, tc.expectedResult.Contents[0].Text, content.Text)
			default:
				t.Fatalf("unknown expectedResponseType %v", tc.expectedResponseType)
			}
		})
	}
}
