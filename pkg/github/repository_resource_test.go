package github

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/github/github-mcp-server/pkg/raw"
	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/require"
)

func Test_repositoryResourceContentsHandler(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]any
		expectError    string
		expectedResult any
	}{
		{
			name: "missing owner",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "image/png")
						// as this is given as a png, it will return the content as a blob
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			requestArgs: map[string]any{},
			expectError: "owner is required",
		},
		{
			name: "missing repo",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					raw.GetRawReposContentsByOwnerByRepoByBranchByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "image/png")
						// as this is given as a png, it will return the content as a blob
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner": []string{"owner"},
			},
			expectError: "repo is required",
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
			requestArgs: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"data.png"},
			},
			expectedResult: []mcp.BlobResourceContents{{
				Blob:     "IyBUZXN0IFJlcG9zaXRvcnkKClRoaXMgaXMgYSB0ZXN0IHJlcG9zaXRvcnku",
				MIMEType: "image/png",
				URI:      "",
			}},
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
			requestArgs: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"README.md"},
			},
			expectedResult: []mcp.TextResourceContents{{
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
				URI:      "",
			}},
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
			requestArgs: map[string]any{
				"owner":  []string{"owner"},
				"repo":   []string{"repo"},
				"path":   []string{"README.md"},
				"branch": []string{"main"},
			},
			expectedResult: []mcp.TextResourceContents{{
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
				URI:      "",
			}},
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
			requestArgs: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"README.md"},
				"tag":   []string{"v1.0.0"},
			},
			expectedResult: []mcp.TextResourceContents{{
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
				URI:      "",
			}},
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
			requestArgs: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"README.md"},
				"sha":   []string{"abc123"},
			},
			expectedResult: []mcp.TextResourceContents{{
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
				URI:      "",
			}},
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
			requestArgs: map[string]any{
				"owner":    []string{"owner"},
				"repo":     []string{"repo"},
				"path":     []string{"README.md"},
				"prNumber": []string{"42"},
			},
			expectedResult: []mcp.TextResourceContents{{
				Text:     "# Test Repository\n\nThis is a test repository.",
				MIMEType: "text/markdown",
				URI:      "",
			}},
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
			requestArgs: map[string]any{
				"owner":  []string{"owner"},
				"repo":   []string{"repo"},
				"path":   []string{"nonexistent.md"},
				"branch": []string{"main"},
			},
			expectError: "404 Not Found",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			mockRawClient := raw.NewClient(client, base)
			handler := RepositoryResourceContentsHandler((stubGetClientFn(client)), stubGetRawClientFn(mockRawClient))

			request := mcp.ReadResourceRequest{
				Params: struct {
					URI       string         `json:"uri"`
					Arguments map[string]any `json:"arguments,omitempty"`
				}{
					Arguments: tc.requestArgs,
				},
			}

			resp, err := handler(context.TODO(), request)

			if tc.expectError != "" {
				require.ErrorContains(t, err, tc.expectError)
				return
			}

			require.NoError(t, err)
			require.ElementsMatch(t, resp, tc.expectedResult)
		})
	}
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
