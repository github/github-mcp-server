package oauth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{"empty defaults to github.com", "", "https://github.com"},
		{"bare host", "github.com", "https://github.com"},
		{"https scheme preserved", "https://github.com", "https://github.com"},
		{"http scheme preserved", "http://localhost:3000", "http://localhost:3000"},
		{"api subdomain stripped", "api.github.com", "https://github.com"},
		{"whitespace trimmed", "  github.com  ", "https://github.com"},
		{"path and api stripped", "https://api.github.com/api/v3", "https://github.com"},
		{"ghes host", "ghe.example.com", "https://ghe.example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeHost(tt.host))
		})
	}
}

func TestGitHubEndpoint(t *testing.T) {
	ep := GitHubEndpoint("")
	assert.Equal(t, "https://github.com/login/oauth/authorize", ep.AuthURL)
	assert.Equal(t, "https://github.com/login/oauth/access_token", ep.TokenURL)
	assert.Equal(t, "https://github.com/login/device/code", ep.DeviceAuthURL)

	ghes := GitHubEndpoint("https://ghe.example.com")
	assert.Equal(t, "https://ghe.example.com/login/oauth/authorize", ghes.AuthURL)
}

func TestNewGitHubConfig(t *testing.T) {
	cfg := NewGitHubConfig("client", "secret", []string{"repo", "read:org"}, "", 8085)
	assert.Equal(t, "client", cfg.ClientID)
	assert.Equal(t, "secret", cfg.ClientSecret)
	assert.Equal(t, []string{"repo", "read:org"}, cfg.Scopes)
	assert.Equal(t, 8085, cfg.CallbackPort)
	assert.Equal(t, GitHubEndpoint(""), cfg.Endpoint)
}

func TestRandomState(t *testing.T) {
	s1, err := randomState()
	require.NoError(t, err)
	s2, err := randomState()
	require.NoError(t, err)

	assert.NotEqual(t, s1, s2, "state must be unique per call")
	assert.NotContains(t, s1, "=", "state must be URL-safe without padding")
	assert.NotContains(t, s1, "+")
	assert.NotContains(t, s1, "/")
	assert.GreaterOrEqual(t, len(s1), 22, "16 random bytes encode to 22 base64url chars")
	assert.False(t, strings.ContainsAny(s1, " \t\n"))
}
