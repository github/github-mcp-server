package transport

import (
	"net/http"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/github/github-mcp-server/pkg/utils"
)

// APIVersionTransport sets the GitHub REST API version on requests to
// GitHub.com and GitHub Enterprise Cloud.
type APIVersionTransport struct {
	Transport http.RoundTripper
}

// SetGitHubAPIVersionHeader selects the REST API version supported by the
// target deployment. GitHub Enterprise Server releases support API versions
// independently, so they retain the established compatibility version.
func SetGitHubAPIVersionHeader(req *http.Request) {
	if req == nil || req.URL == nil {
		return
	}

	hostType, err := utils.ParseHostType(req.URL.String())
	if err != nil {
		return
	}

	if req.Header == nil {
		req.Header = make(http.Header)
	}
	version := headers.GitHubAPIVersion
	if hostType == utils.HostTypeGHES {
		version = headers.GitHubEnterpriseServerAPIVersion
	}
	req.Header.Set(headers.GitHubAPIVersionHeader, version)
}

// RoundTrip implements http.RoundTripper.
func (t *APIVersionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	underlying := t.Transport
	if underlying == nil {
		underlying = http.DefaultTransport
	}

	req = req.Clone(req.Context())
	SetGitHubAPIVersionHeader(req)
	return underlying.RoundTrip(req)
}
