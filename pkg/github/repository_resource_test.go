package github

import (
	"context"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v72/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/require"
)

var GetRawReposContentsByOwnerByRepoByPath mock.EndpointPattern = mock.EndpointPattern{
	Pattern: "/{owner}/{repo}/main/{path:.+}",
	Method:  "GET",
}

func Test_repositoryResourceContentsHandler(t *testing.T) {
	mockDirContent := []*github.RepositoryContent{
		{
			Type:        github.Ptr("file"),
			Name:        github.Ptr("README.md"),
			Path:        github.Ptr("README.md"),
			SHA:         github.Ptr("abc123"),
			Size:        github.Ptr(42),
			HTMLURL:     github.Ptr("https://github.com/owner/repo/blob/main/README.md"),
			DownloadURL: github.Ptr("https://raw.githubusercontent.com/owner/repo/main/README.md"),
		},
		{
			Type:        github.Ptr("dir"),
			Name:        github.Ptr("src"),
			Path:        github.Ptr("src"),
			SHA:         github.Ptr("def456"),
			HTMLURL:     github.Ptr("https://github.com/owner/repo/tree/main/src"),
			DownloadURL: github.Ptr("https://raw.githubusercontent.com/owner/repo/main/src"),
		},
	}
	expectedDirContent := []mcp.TextResourceContents{
		{
			URI:      "https://github.com/owner/repo/blob/main/README.md",
			MIMEType: "text/markdown",
			Text:     "README.md",
		},
		{
			URI:      "https://github.com/owner/repo/tree/main/src",
			MIMEType: "text/directory",
			Text:     "src",
		},
	}

	mockTextContent := &github.RepositoryContent{
		Type:        github.Ptr("file"),
		Name:        github.Ptr("README.md"),
		Path:        github.Ptr("README.md"),
		Content:     github.Ptr("# Test Repository\n\nThis is a test repository."),
		SHA:         github.Ptr("abc123"),
		Size:        github.Ptr(42),
		HTMLURL:     github.Ptr("https://github.com/owner/repo/blob/main/README.md"),
		DownloadURL: github.Ptr("https://raw.githubusercontent.com/owner/repo/main/README.md"),
	}

	mockFileContent := &github.RepositoryContent{
		Type:        github.Ptr("file"),
		Name:        github.Ptr("data.png"),
		Path:        github.Ptr("data.png"),
		Content:     github.Ptr("IyBUZXN0IFJlcG9zaXRvcnkKClRoaXMgaXMgYSB0ZXN0IHJlcG9zaXRvcnku"), // Base64 encoded "# Test Repository\n\nThis is a test repository."
		SHA:         github.Ptr("abc123"),
		Size:        github.Ptr(42),
		HTMLURL:     github.Ptr("https://github.com/owner/repo/blob/main/data.png"),
		DownloadURL: github.Ptr("https://raw.githubusercontent.com/owner/repo/main/data.png"),
	}

	expectedFileContent := []mcp.BlobResourceContents{
		{
			Blob:     "IyBUZXN0IFJlcG9zaXRvcnkKClRoaXMgaXMgYSB0ZXN0IHJlcG9zaXRvcnku",
			MIMEType: "image/png",
			URI:      "",
		},
	}

	expectedTextContent := []mcp.TextResourceContents{
		{
			Text:     "# Test Repository\n\nThis is a test repository.",
			MIMEType: "text/markdown",
			URI:      "",
		},
	}

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
				mock.WithRequestMatch(
					mock.GetReposContentsByOwnerByRepoByPath,
					mockFileContent,
				),
			),
			requestArgs: map[string]any{},
			expectError: "owner is required",
		},
		{
			name: "missing repo",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposContentsByOwnerByRepoByPath,
					mockFileContent,
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
				mock.WithRequestMatch(
					mock.GetReposContentsByOwnerByRepoByPath,
					mockFileContent,
				),
				mock.WithRequestMatchHandler(
					GetRawReposContentsByOwnerByRepoByPath,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "image/png")
						// as this is given as a png, it will return the content as a blob
						_, err := w.Write([]byte("# Test Repository\n\nThis is a test repository."))
						require.NoError(t, err)
					}),
				),
			),
			requestArgs: map[string]any{
				"owner":  []string{"owner"},
				"repo":   []string{"repo"},
				"path":   []string{"data.png"},
				"branch": []string{"main"},
			},
			expectedResult: expectedFileContent,
		},
		{
			name: "successful text content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposContentsByOwnerByRepoByPath,
					mockTextContent,
				),
				mock.WithRequestMatch(
					GetRawReposContentsByOwnerByRepoByPath,
					[]byte("# Test Repository\n\nThis is a test repository."),
				),
			),
			requestArgs: map[string]any{
				"owner":  []string{"owner"},
				"repo":   []string{"repo"},
				"path":   []string{"README.md"},
				"branch": []string{"main"},
			},
			expectedResult: expectedTextContent,
		},
		{
			name: "successful directory content fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposContentsByOwnerByRepoByPath,
					mockDirContent,
				),
			),
			requestArgs: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"src"},
			},
			expectedResult: expectedDirContent,
		},
		{
			name: "empty data",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposContentsByOwnerByRepoByPath,
					[]*github.RepositoryContent{},
				),
			),
			requestArgs: map[string]any{
				"owner": []string{"owner"},
				"repo":  []string{"repo"},
				"path":  []string{"src"},
			},
			expectedResult: nil,
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
			handler := RepositoryResourceContentsHandler((stubGetClientFn(client)))

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
	tmpl, _ := GetRepositoryResourceContent(nil, translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/contents{/path*}", tmpl.URITemplate.Raw())
}

func Test_GetRepositoryResourceBranchContent(t *testing.T) {
	tmpl, _ := GetRepositoryResourceBranchContent(nil, translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/refs/heads/{branch}/contents{/path*}", tmpl.URITemplate.Raw())
}
func Test_GetRepositoryResourceCommitContent(t *testing.T) {
	tmpl, _ := GetRepositoryResourceCommitContent(nil, translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/sha/{sha}/contents{/path*}", tmpl.URITemplate.Raw())
}

func Test_GetRepositoryResourceTagContent(t *testing.T) {
	tmpl, _ := GetRepositoryResourceTagContent(nil, translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/refs/tags/{tag}/contents{/path*}", tmpl.URITemplate.Raw())
}

func Test_GetRepositoryResourcePrContent(t *testing.T) {
	tmpl, _ := GetRepositoryResourcePrContent(nil, translations.NullTranslationHelper)
	require.Equal(t, "repo://{owner}/{repo}/refs/pull/{prNumber}/head/contents{/path*}", tmpl.URITemplate.Raw())
}
