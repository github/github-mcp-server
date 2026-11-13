package transport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/github/github-mcp-server/v2/pkg/http/headers"
	"github.com/google/go-github/v92/github"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type apiVersionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f apiVersionRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestAPIVersionTransportWithGoGitHub(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		host    string
		version string
	}{
		{"api.github.com", headers.GitHubAPIVersion},
		{"api.example.ghe.com", headers.GitHubAPIVersion},
		{"API.EXAMPLE.GHE.COM", headers.GitHubAPIVersion},
		{"github.example.com", headers.GitHubEnterpriseServerAPIVersion},
		{"api.github.com.example.org", headers.GitHubEnterpriseServerAPIVersion},
		{"api.example.ghe.com.example.org", headers.GitHubEnterpriseServerAPIVersion},
	} {
		t.Run(tt.host, func(t *testing.T) {
			t.Parallel()

			client, err := github.NewClient(
				github.WithEnterpriseURLs("https://"+tt.host, "https://"+tt.host),
				github.WithTransport(&APIVersionTransport{Transport: apiVersionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
					assert.Equal(t, tt.version, req.Header.Get(headers.GitHubAPIVersionHeader))
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"resources":{"core":{"limit":5000,"remaining":4999}}}`)),
						Request:    req,
					}, nil
				})}),
			)
			require.NoError(t, err)

			// Exercise the SDK's version guard as well as the transport override.
			req, err := client.NewRequest(context.Background(), http.MethodGet, "rate_limit", nil, github.WithVersion(headers.GitHubAPIVersion))
			require.NoError(t, err)
			resp, err := client.Do(req, nil)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			limits, resp, err := client.RateLimit.Get(context.Background())
			require.NoError(t, err)
			defer resp.Body.Close()
			require.NotNil(t, limits.Core)
			assert.Equal(t, 4999, limits.Core.Remaining)
		})
	}
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
			underlying := apiVersionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
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
