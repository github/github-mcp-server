package transport

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserAgentTransportRequestIsolation(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _ = io.WriteString(w, req.UserAgent())
	}))
	t.Cleanup(server.Close)
	transport := &UserAgentTransport{
		Transport: http.DefaultTransport,
		Agent:     "github-mcp-server/remote-abcdef",
	}
	client := &http.Client{Transport: transport}

	for _, agent := range []string{"", "github-mcp-server/v1.2.3 (first/1)", "github-mcp-server/v1.2.3 (second/2)"} {
		t.Run(agent, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			want := transport.Agent
			if agent != "" {
				ctx = WithUserAgent(ctx, agent)
				want = agent
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			require.NoError(t, err)
			req.Header.Set("User-Agent", "original")
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, want, string(body))
			assert.Equal(t, "original", req.UserAgent())
			assert.Equal(t, "github-mcp-server/remote-abcdef", transport.Agent)
		})
	}
}
