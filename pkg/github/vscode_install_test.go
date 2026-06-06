package github

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVSCodeInstallRedirectURLHTTP(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		insiders bool
		wantSub  []string
		notWant  []string
	}{
		{
			name:     "stable uses vscode protocol redirect",
			insiders: false,
			wantSub: []string{
				"https://insiders.vscode.dev/redirect?url=",
				"vscode%3Amcp%2Finstall%3F",
				"%2522name%2522%253A%2522github%2522",
				"%2522type%2522%253A%2522http%2522",
				"api.githubcopilot.com%252Fmcp%252F",
			},
			notWant: []string{"quality=insiders", "vscode-insiders", "/redirect/mcp/install"},
		},
		{
			name:     "insiders uses vscode-insiders protocol redirect",
			insiders: true,
			wantSub: []string{
				"https://insiders.vscode.dev/redirect?url=",
				"vscode-insiders%3Amcp%2Finstall%3F",
				"%2522name%2522%253A%2522github%2522",
				"%2522config%2522%253A%257B",
				"%2522type%2522%253A%2522http%2522",
				"api.githubcopilot.com%252Fmcp%252F",
			},
			notWant: []string{"quality=insiders", "/redirect/mcp/install"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := VSCodeInstallRedirectURLHTTP("github", "https://api.githubcopilot.com/mcp/", tt.insiders)
			require.NoError(t, err)

			for _, sub := range tt.wantSub {
				assert.Contains(t, got, sub)
			}
			for _, sub := range tt.notWant {
				assert.NotContains(t, got, sub)
			}
		})
	}
}

func TestVSCodeInstallRedirectURL_withInputs(t *testing.T) {
	t.Parallel()

	inputs := []map[string]any{
		{
			"id":          "github_token",
			"type":        "promptString",
			"description": "GitHub Personal Access Token",
			"password":    true,
		},
	}
	config := map[string]any{
		"command": "docker",
		"args":    []string{"run", "-i", "--rm", "ghcr.io/github/github-mcp-server"},
	}

	got, err := VSCodeInstallRedirectURL("github", config, inputs, true)
	require.NoError(t, err)

	assert.Contains(t, got, "vscode-insiders%3Amcp%2Finstall%3F")
	assert.Contains(t, got, "%2522inputs%2522%253A%255B")
	assert.NotContains(t, got, "quality=insiders")
}

func TestVSCodeInstallRedirectURL_decodesToNestedConfig(t *testing.T) {
	t.Parallel()

	installURL, err := VSCodeInstallRedirectURLHTTP("github", "https://api.githubcopilot.com/mcp/", true)
	require.NoError(t, err)

	parsed, err := url.Parse(installURL)
	require.NoError(t, err)

	redirectTarget, err := url.QueryUnescape(parsed.Query().Get("url"))
	require.NoError(t, err)

	_, payload, ok := strings.Cut(redirectTarget, "mcp/install?")
	require.True(t, ok)

	decodedPayload, err := url.QueryUnescape(payload)
	require.NoError(t, err)

	assert.JSONEq(t, `{"name":"github","config":{"type":"http","url":"https://api.githubcopilot.com/mcp/"}}`, decodedPayload)
}

func TestVSCodeHTTPToolsetInstallLink(t *testing.T) {
	t.Parallel()

	link, err := VSCodeHTTPToolsetInstallLink("gh-issues", "https://api.githubcopilot.com/mcp/x/issues")
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(link, "[Install](https://insiders.vscode.dev/redirect?url="))
	assert.Contains(t, link, "%2522name%2522%253A%2522gh-issues%2522")
}
