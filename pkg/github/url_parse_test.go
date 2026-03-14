package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitHubURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		url     string
		want    *ParsedGitHubURL
		wantErr string
	}{
		// Repo URLs
		{
			name: "github.com repo",
			url:  "https://github.com/facebook/react",
			want: &ParsedGitHubURL{Owner: "facebook", Repo: "react", Type: URLTypeRepo},
		},
		{
			name: "GHE repo",
			url:  "https://github.mycompany.com/myorg/myrepo",
			want: &ParsedGitHubURL{Owner: "myorg", Repo: "myrepo", Type: URLTypeRepo},
		},
		{
			name: "repo URL with trailing slash",
			url:  "https://github.com/facebook/react/",
			want: &ParsedGitHubURL{Owner: "facebook", Repo: "react", Type: URLTypeRepo},
		},

		// Issue URLs
		{
			name: "issue URL",
			url:  "https://github.com/facebook/react/issues/30072",
			want: &ParsedGitHubURL{Owner: "facebook", Repo: "react", Type: URLTypeIssue, Number: 30072},
		},
		{
			name: "GHE issue URL",
			url:  "https://github.mycompany.com/myorg/myrepo/issues/5",
			want: &ParsedGitHubURL{Owner: "myorg", Repo: "myrepo", Type: URLTypeIssue, Number: 5},
		},
		{
			name:    "issue URL missing number",
			url:     "https://github.com/facebook/react/issues/",
			wantErr: "issue URL missing number",
		},
		{
			name:    "issue URL with non-numeric number",
			url:     "https://github.com/facebook/react/issues/abc",
			wantErr: "issue URL has invalid number",
		},

		// PR URLs
		{
			name: "PR URL",
			url:  "https://github.com/facebook/react/pull/31000",
			want: &ParsedGitHubURL{Owner: "facebook", Repo: "react", Type: URLTypePR, Number: 31000},
		},
		{
			name: "GHE PR URL",
			url:  "https://github.mycompany.com/myorg/myrepo/pull/42",
			want: &ParsedGitHubURL{Owner: "myorg", Repo: "myrepo", Type: URLTypePR, Number: 42},
		},
		{
			name:    "PR URL missing number",
			url:     "https://github.com/facebook/react/pull/",
			wantErr: "pull request URL missing number",
		},
		{
			name:    "PR URL with zero number",
			url:     "https://github.com/facebook/react/pull/0",
			wantErr: "pull request URL has invalid number",
		},

		// File blob URLs
		{
			name: "file blob URL",
			url:  "https://github.com/facebook/react/blob/main/packages/react/index.js",
			want: &ParsedGitHubURL{
				Owner: "facebook",
				Repo:  "react",
				Type:  URLTypeFile,
				Ref:   "main",
				Path:  "packages/react/index.js",
			},
		},
		{
			name: "file blob URL with deeply nested path",
			url:  "https://github.com/facebook/react/blob/v18.0.0/packages/react/src/ReactHooks.js",
			want: &ParsedGitHubURL{
				Owner: "facebook",
				Repo:  "react",
				Type:  URLTypeFile,
				Ref:   "v18.0.0",
				Path:  "packages/react/src/ReactHooks.js",
			},
		},
		{
			name: "file blob URL with SHA ref",
			url:  "https://github.com/facebook/react/blob/abc1234def5678/README.md",
			want: &ParsedGitHubURL{
				Owner: "facebook",
				Repo:  "react",
				Type:  URLTypeFile,
				Ref:   "abc1234def5678",
				Path:  "README.md",
			},
		},
		{
			name:    "file blob URL missing path",
			url:     "https://github.com/facebook/react/blob/main",
			wantErr: "file URL must contain",
		},

		// Error cases
		{
			name:    "unsupported URL type",
			url:     "https://github.com/facebook/react/tree/main",
			wantErr: `unsupported GitHub URL type "tree"`,
		},
		{
			name:    "URL missing repo",
			url:     "https://github.com/facebook",
			wantErr: "URL must contain at least /{owner}/{repo}",
		},
		{
			name:    "non-http scheme",
			url:     "ftp://github.com/facebook/react",
			wantErr: "unsupported URL scheme",
		},
		{
			name:    "invalid URL",
			url:     "://bad-url",
			wantErr: "invalid URL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseGitHubURL(tc.url)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestApplyURLParam(t *testing.T) {
	t.Parallel()

	t.Run("no url param — args unchanged", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"owner": "myorg"}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, map[string]any{"owner": "myorg"}, args)
	})

	t.Run("repo URL populates owner and repo", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": "https://github.com/facebook/react"}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "facebook", args["owner"])
		assert.Equal(t, "react", args["repo"])
	})

	t.Run("issue URL populates owner, repo, and number", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": "https://github.com/facebook/react/issues/30072"}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "facebook", args["owner"])
		assert.Equal(t, "react", args["repo"])
		assert.Equal(t, float64(30072), args["number"])
	})

	t.Run("PR URL populates owner, repo, and number", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": "https://github.com/facebook/react/pull/31000"}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "facebook", args["owner"])
		assert.Equal(t, "react", args["repo"])
		assert.Equal(t, float64(31000), args["number"])
	})

	t.Run("file URL populates owner, repo, ref, and path", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": "https://github.com/facebook/react/blob/main/packages/react/index.js"}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "facebook", args["owner"])
		assert.Equal(t, "react", args["repo"])
		assert.Equal(t, "main", args["ref"])
		assert.Equal(t, "packages/react/index.js", args["path"])
	})

	t.Run("explicit owner takes precedence over URL", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"url":   "https://github.com/facebook/react",
			"owner": "override-owner",
		}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "override-owner", args["owner"])
		assert.Equal(t, "react", args["repo"])
	})

	t.Run("explicit repo takes precedence over URL", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"url":  "https://github.com/facebook/react",
			"repo": "override-repo",
		}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "facebook", args["owner"])
		assert.Equal(t, "override-repo", args["repo"])
	})

	t.Run("empty string owner is treated as absent", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{
			"url":   "https://github.com/facebook/react",
			"owner": "",
		}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "facebook", args["owner"])
	})

	t.Run("invalid URL returns error", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": "not-a-url"}
		err := ApplyURLParam(args)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid GitHub URL")
	})

	t.Run("non-string url param is ignored", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": 12345}
		require.NoError(t, ApplyURLParam(args))
		// no fields populated, no error
		_, hasOwner := args["owner"]
		assert.False(t, hasOwner)
	})

	t.Run("GHE URL is parsed correctly", func(t *testing.T) {
		t.Parallel()
		args := map[string]any{"url": "https://github.mycompany.com/myorg/myrepo/pull/7"}
		require.NoError(t, ApplyURLParam(args))
		assert.Equal(t, "myorg", args["owner"])
		assert.Equal(t, "myrepo", args["repo"])
		assert.Equal(t, float64(7), args["number"])
	})
}
