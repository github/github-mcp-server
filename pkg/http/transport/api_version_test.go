package transport

import (
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAPIVersionTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		url             string
		existingVersion string
		wantVersion     string
	}{
		{
			name:            "GitHub.com overrides the default version",
			url:             "https://api.github.com/repos/octo-org/octo-repo",
			existingVersion: headers.GitHubEnterpriseServerAPIVersion,
			wantVersion:     headers.GitHubAPIVersion,
		},
		{
			name:        "GitHub Enterprise Cloud sets the new version",
			url:         "https://api.example.ghe.com/repos/octo-org/octo-repo",
			wantVersion: headers.GitHubAPIVersion,
		},
		{
			name:            "GitHub Enterprise Server pins the compatibility version",
			url:             "https://github.example.com/api/v3/repos/octo-org/octo-repo",
			existingVersion: headers.GitHubAPIVersion,
			wantVersion:     headers.GitHubEnterpriseServerAPIVersion,
		},
		{
			name:        "GitHub Enterprise Server sets the compatibility version",
			url:         "https://github.example.com/api/v3/repos/octo-org/octo-repo",
			wantVersion: headers.GitHubEnterpriseServerAPIVersion,
		},
		{
			name:        "host classification is case insensitive",
			url:         "https://API.GITHUB.COM/repos/octo-org/octo-repo",
			wantVersion: headers.GitHubAPIVersion,
		},
		{
			name:        "lookalike domain is treated as GitHub Enterprise Server",
			url:         "https://api.github.com.example.org/api/v3/",
			wantVersion: headers.GitHubEnterpriseServerAPIVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var gotVersion string
			underlying := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				gotVersion = req.Header.Get(headers.GitHubAPIVersionHeader)
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       http.NoBody,
					Request:    req,
				}, nil
			})

			req, err := http.NewRequest(http.MethodGet, tt.url, nil)
			require.NoError(t, err)
			if tt.existingVersion != "" {
				req.Header.Set(headers.GitHubAPIVersionHeader, tt.existingVersion)
			} else {
				req.Header = nil
			}

			resp, err := (&APIVersionTransport{Transport: underlying}).RoundTrip(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantVersion, gotVersion)
			assert.Equal(t, tt.existingVersion, req.Header.Get(headers.GitHubAPIVersionHeader), "the original request must not be mutated")
		})
	}
}
