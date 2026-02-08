package raw

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v79/github"
	"github.com/stretchr/testify/require"
)

// mockRawTransport is a custom HTTP transport for testing raw content API
type mockRawTransport struct {
	statusCode  int
	contentType string
	body        string
}

func (m *mockRawTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Create a response with the configured status and body
	resp := &http.Response{
		StatusCode: m.statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(m.body)),
		Request:    req,
	}
	if m.contentType != "" {
		resp.Header.Set("Content-Type", m.contentType)
	}
	return resp, nil
}

func TestGetRawContent(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")

	tests := []struct {
		name              string
		opts              *ContentOpts
		owner, repo, path string
		statusCode        int
		contentType       string
		body              string
		expectError       string
	}{
		{
			name:        "HEAD fetch success",
			opts:        nil,
			owner:       "octocat",
			repo:        "hello",
			path:        "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:        "branch fetch success",
			opts:        &ContentOpts{Ref: "refs/heads/main"},
			owner:       "octocat",
			repo:        "hello",
			path:        "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:        "tag fetch success",
			opts:        &ContentOpts{Ref: "refs/tags/v1.0.0"},
			owner:       "octocat",
			repo:        "hello",
			path:        "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:        "sha fetch success",
			opts:        &ContentOpts{SHA: "abc123"},
			owner:       "octocat",
			repo:        "hello",
			path:        "README.md",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:        "not found",
			opts:        nil,
			owner:       "octocat",
			repo:        "hello",
			path:        "notfound.txt",
			statusCode:  404,
			contentType: "application/json",
			body:        `{"message": "Not Found"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create mock HTTP client with custom transport
			mockedClient := &http.Client{
				Transport: &mockRawTransport{
					statusCode:  tc.statusCode,
					contentType: tc.contentType,
					body:        tc.body,
				},
			}
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

			// Verify the URL was constructed correctly
			actualURL := client.URLFromOpts(tc.opts, tc.owner, tc.repo, tc.path)
			require.True(t, strings.Contains(actualURL, tc.owner))
			require.True(t, strings.Contains(actualURL, tc.repo))
			require.True(t, strings.Contains(actualURL, tc.path))
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
