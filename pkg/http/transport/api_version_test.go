package transport

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/github/github-mcp-server/pkg/http/headers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIVersionTransport(t *testing.T) {
	t.Parallel()

	var gotVersion string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get(headers.GitHubAPIVersionHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	require.NoError(t, err)
	req.Header.Set(headers.GitHubAPIVersionHeader, "2022-11-28")

	resp, err := (&APIVersionTransport{}).RoundTrip(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, headers.GitHubAPIVersion, gotVersion)
	assert.Equal(t, "2022-11-28", req.Header.Get(headers.GitHubAPIVersionHeader))
}
