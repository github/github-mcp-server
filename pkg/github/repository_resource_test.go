package github

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
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
		handlerFn            func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler
		expectedResponseType resourceResponseType
		expectError          string
		expectedResult       *mcp.ReadResourceResult
	}{
		{
			name: "missing owner",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo:///repo/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceContent(getClient, getRawClient, t).Handler
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "owner is required",
		},
		{
			name: "missing repo",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner//refs/heads/main/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceBranchContent(getClient, getRawClient, t).Handler
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "repo is required",
		},
		{
			name: "successful blob content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "image/png")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/contents/data.png",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Set("Content-Type", "text/plain")

						require.Contains(t, r.URL.Path, "pkg/github/actions.go")
						_, err := w.Write([]byte("package actions\n\nfunc main() {\n    // Sample Go file content\n}\n"))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/contents/pkg/github/actions.go",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/refs/heads/main/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceBranchContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByTagByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/refs/tags/v1.0.0/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceTagContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoBySHAByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/sha/abc123/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceCommitContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposPullsByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						_, err := w.Write([]byte(`{"head": {"sha": "abc123"}}`))
						require.NoError(t, err)
					}),
				),
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoBySHAByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "text/markdown")
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			uri: "repo://owner/repo/refs/pull/42/head/contents/README.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourcePrContent(getClient, getRawClient, t).Handler
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
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			uri: "repo://owner/repo/contents/nonexistent.md",
			handlerFn: func(getClient GetClientFn, getRawClient raw.GetRawClientFn, t translations.TranslationHelperFunc) mcp.ResourceHandler {
				return GetRepositoryResourceContent(getClient, getRawClient, t).Handler
			},
			expectedResponseType: resourceResponseTypeText, // Ignored as error is expected
			expectError:          "404 Not Found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			mockRawClient := raw.NewClient(client, base)
			handler := tc.handlerFn(stubGetClientFn(client), stubGetRawClientFn(mockRawClient), translations.NullTranslationHelper)

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
