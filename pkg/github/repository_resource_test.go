package github

import (
	"context"
	"errors"
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
				_, handler := GetRepositoryResourceContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceBranchContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceBranchContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceTagContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceCommitContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourcePrContent(getClient, getRawClient, t)
				return handler
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
				_, handler := GetRepositoryResourceContent(getClient, getRawClient, t)
				return handler
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

// Test_repositoryResourceContentsHandler_NetworkError tests that a network error
// during raw content fetch does not cause a panic (nil response body dereference).
func Test_repositoryResourceContentsHandler_NetworkError(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	networkErr := errors.New("network error: connection refused")

	httpClient := &http.Client{Transport: &errorTransport{err: networkErr}}
	client := github.NewClient(httpClient)
	mockRawClient := raw.NewClient(client, base)
	handler := RepositoryResourceContentsHandler(stubGetClientFn(client), stubGetRawClientFn(mockRawClient))

	request := mcp.ReadResourceRequest{
		Params: struct {
			URI       string         `json:"uri"`
			Arguments map[string]any `json:"arguments,omitempty"`
		}{
			Arguments: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"README.md"},
			},
		},
	}

	// This should not panic, even though the HTTP client returns an error
	resp, err := handler(context.TODO(), request)
	require.Error(t, err)
	require.Nil(t, resp)
	require.ErrorContains(t, err, "failed to get raw content")
}

func Test_GetRepositoryResourceContent(t *testing.T) {
	mockRawClient := raw.NewClient(github.NewClient(nil), &url.URL{})
	tmpl, _ := GetRepositoryResourceContent(nil, stubGetRawClientFn(mockRawClient), translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/contents{/path*}", tmpl.URITemplate.Raw())
}

func Test_GetRepositoryResourceBranchContent(t *testing.T) {
	mockRawClient := raw.NewClient(github.NewClient(nil), &url.URL{})
	tmpl, _ := GetRepositoryResourceBranchContent(nil, stubGetRawClientFn(mockRawClient), translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}", tmpl.URITemplate.Raw())
}
func Test_GetRepositoryResourceCommitContent(t *testing.T) {
	mockRawClient := raw.NewClient(github.NewClient(nil), &url.URL{})
	tmpl, _ := GetRepositoryResourceCommitContent(nil, stubGetRawClientFn(mockRawClient), translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/sha/{sha}/contents{/path*}", tmpl.URITemplate.Raw())
}

func Test_GetRepositoryResourceTagContent(t *testing.T) {
	mockRawClient := raw.NewClient(github.NewClient(nil), &url.URL{})
	tmpl, _ := GetRepositoryResourceTagContent(nil, stubGetRawClientFn(mockRawClient), translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/refs/tags/{tag}/contents{/path*}", tmpl.URITemplate.Raw())
}

