package raw

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/require"
)

func TestGetRawContent(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")

	tests := []struct {
		name              string
		pattern           mock.EndpointPattern
		opts              *ContentOpts
		owner, repo, path string
		statusCode        int
		contentType       string
		body              string
		expectError       string
	}{
		{
			name:    "HEAD fetch success",
			pattern: GetRawReposContentsByOwnerByRepoByPath,
			opts:    nil,
			owner:   "octocat", repo: "hello", path: "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:    "branch fetch success",
			pattern: GetRawReposContentsByOwnerByRepoByBranchByPath,
			opts:    &ContentOpts{Ref: "refs/heads/main"},
			owner:   "octocat", repo: "hello", path: "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:    "tag fetch success",
			pattern: GetRawReposContentsByOwnerByRepoByTagByPath,
			opts:    &ContentOpts{Ref: "refs/tags/v1.0.0"},
			owner:   "octocat", repo: "hello", path: "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:    "sha fetch success",
			pattern: GetRawReposContentsByOwnerByRepoBySHAByPath,
			opts:    &ContentOpts{SHA: "abc123"},
			owner:   "octocat", repo: "hello", path: "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:    "not found",
			pattern: GetRawReposContentsByOwnerByRepoByPath,
			opts:    nil,
			owner:   "octocat", repo: "hello", path: "notfound.txt",
			statusCode:  404,
			contentType: "application/json",
			body:        `{"message": "Not Found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockedClient := mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					tc.pattern,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", tc.contentType)
						w.WriteHeader(tc.statusCode)
						_, err := w.Write([]byte(tc.body))
						require.NoError(t, err)
					}),
				),
			)
			ghClient := github.NewClient(mockedClient)
			client := NewClient(ghClient, base)
			resp, err := client.GetRawContent(context.Background(), tc.owner, tc.repo, tc.path, tc.opts)
			defer func() {
				_ = resp.Body.Close()
			}()
			if tc.expectError != "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, resp.StatusCode)
		})
	}
}

func TestUrlFromOpts(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	ghClient := github.NewClient(nil)
	client := NewClient(ghClient, base)

	tests := []struct {
		name  string
		opts  *ContentOpts
		owner string
		repo  string
		path  string
		want  string
	}{
		{
			name:  "no opts (HEAD)",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "README.md",
			want: "https://raw.example.com/octocat/hello/HEAD/README.md",
		},
		{
			name:  "ref branch",
			opts:  &ContentOpts{Ref: "refs/heads/main"},
			owner: "octocat", repo: "hello", path: "README.md",
			want: "https://raw.example.com/octocat/hello/refs/heads/main/README.md",
		},
		{
			name:  "ref tag",
			opts:  &ContentOpts{Ref: "refs/tags/v1.0.0"},
			owner: "octocat", repo: "hello", path: "README.md",
			want: "https://raw.example.com/octocat/hello/refs/tags/v1.0.0/README.md",
		},
		{
			name:  "sha",
			opts:  &ContentOpts{SHA: "abc123"},
			owner: "octocat", repo: "hello", path: "README.md",
			want: "https://raw.example.com/octocat/hello/abc123/README.md",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.URLFromOpts(tt.opts, tt.owner, tt.repo, tt.path)
			if got != tt.want {
				t.Errorf("UrlFromOpts() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewRequestError(t *testing.T) {
	// Test error path when NewRequest fails due to invalid URL
	base, _ := url.Parse("https://raw.example.com/")
	ghClient := github.NewClient(nil)
	client := NewClient(ghClient, base)

	// Call newRequest with a URL string containing control characters that will fail to parse
	// The newline character in the URL will cause url.Parse to fail
	req, err := client.newRequest(context.Background(), "GET", "http://example.com/path\nwith\nnewlines", nil)

	require.Error(t, err)
	require.Nil(t, req)
}

func TestGetRawContentError(t *testing.T) {
	// Test error path when GetRawContent fails due to newRequest error
	// We'll use a base URL that causes issues when joined with paths
	base, _ := url.Parse("http://")
	ghClient := github.NewClient(nil)

	// Set a malformed BaseURL that will cause NewRequest to fail
	ghClient.BaseURL = &url.URL{Scheme: "http", Host: "example.com", Path: "/%"}

	client := &Client{client: ghClient, url: base}

	// Call GetRawContent which will fail when calling newRequest
	resp, err := client.GetRawContent(context.Background(), "owner", "repo", "path", nil)

	require.Error(t, err)
	require.Nil(t, resp)
}
