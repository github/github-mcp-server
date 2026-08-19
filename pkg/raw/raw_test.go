package raw

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-github/v89/github"
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
		{
			name:        "benign double-dot filename",
			opts:        nil,
			owner:       "octocat",
			repo:        "hello",
			path:        "file..txt",
			statusCode:  200,
			contentType: "text/plain",
			body:        "# Test file",
		},
		{
			name:        "literal path traversal rejected",
			opts:        nil,
			owner:       "octocat",
			repo:        "hello",
			path:        "../evilowner/evilrepo/HEAD/secret.txt",
			expectError: "path traversal",
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
			ghClient, err := github.NewClient(github.WithHTTPClient(mockedClient))
			require.NoError(t, err)
			client, err := NewClient(ghClient, base)
			require.NoError(t, err)
			resp, err := client.GetRawContent(context.Background(), tc.owner, tc.repo, tc.path, tc.opts)
			if resp != nil {
				defer func() {
					_ = resp.Body.Close()
				}()
			}

			if tc.expectError != "" {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.statusCode, resp.StatusCode)

			// Verify the URL was constructed correctly
			actualURL, err := client.URLFromOpts(tc.opts, tc.owner, tc.repo, tc.path)
			require.NoError(t, err)
			require.True(t, strings.Contains(actualURL, tc.owner))
			require.True(t, strings.Contains(actualURL, tc.repo))
			require.True(t, strings.Contains(actualURL, tc.path))
		})
	}
}

func TestUrlFromOpts(t *testing.T) {
	base, _ := url.Parse("https://raw.example.com/")
	ghClient, err := github.NewClient(github.WithHTTPClient(&http.Client{}))
	require.NoError(t, err)
	client, err := NewClient(ghClient, base)
	require.NoError(t, err)

	tests := []struct {
		name    string
		opts    *ContentOpts
		owner   string
		repo    string
		path    string
		want    string
		wantErr bool
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
		{
			name:  "nested path",
			opts:  &ContentOpts{Ref: "refs/heads/main"},
			owner: "octocat", repo: "hello", path: "src/pkg/deep/file.go",
			want: "https://raw.example.com/octocat/hello/refs/heads/main/src/pkg/deep/file.go",
		},
		{
			name:  "benign double-dot filename",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "file..txt",
			want: "https://raw.example.com/octocat/hello/HEAD/file..txt",
		},
		{
			name:  "benign leading double-dot filename",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "..hidden",
			want: "https://raw.example.com/octocat/hello/HEAD/..hidden",
		},
		{
			name:  "literal dot-dot segment rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "../secret.txt",
			wantErr: true,
		},
		{
			name:  "nested literal dot-dot segment rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "docs/../../secret.txt",
			wantErr: true,
		},
		{
			name:  "rebinding traversal rejected",
			opts:  &ContentOpts{Ref: "main"},
			owner: "octocat", repo: "hello", path: "../../evilowner/evilrepo/main/secret.txt",
			wantErr: true,
		},
		{
			name:  "percent-encoded dot-dot segment rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%2e%2e/secret.txt",
			wantErr: true,
		},
		{
			name:  "uppercase percent-encoded dot-dot segment rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%2E%2E/secret.txt",
			wantErr: true,
		},
		{
			name:  "partially percent-encoded dot-dot segment rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%2e./secret.txt",
			wantErr: true,
		},
		{
			name:  "dot-dot in ref rejected",
			opts:  &ContentOpts{Ref: "../evilref"},
			owner: "octocat", repo: "hello", path: "README.md",
			wantErr: true,
		},
		{
			name:  "encoded separator revealing dot-dot rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%2e%2e%2fsecret.txt",
			wantErr: true,
		},
		{
			name:  "encoded separator revealing dot-dot mid-path rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "docs%2f..%2f..%2fsecret.txt",
			wantErr: true,
		},
		{
			name:  "encoded separator in owner rejected",
			opts:  nil,
			owner: "octocat%2f..", repo: "hello", path: "README.md",
			wantErr: true,
		},
		{
			name:  "double percent-encoded dot-dot rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%252e%252e/secret.txt",
			wantErr: true,
		},
		{
			name:  "double percent-encoded separator revealing dot-dot rejected",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%252e%252e%252fsecret.txt",
			wantErr: true,
		},
		{
			name:  "benign percent-encoded filename allowed",
			opts:  nil,
			owner: "octocat", repo: "hello", path: "%2ehidden",
			want: "https://raw.example.com/octocat/hello/HEAD/%2ehidden",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.URLFromOpts(tt.opts, tt.owner, tt.repo, tt.path)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if got != tt.want {
				t.Errorf("UrlFromOpts() = %q, want %q", got, tt.want)
			}
		})
	}
}
