package servercard

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertCardContract checks the required Server Card fields defined by the
// experimental-ext-server-card v1 schema, plus the remote-only invariant. The
// card is small and stable, so asserting its required fields is a focused
// stand-in for vendoring the upstream JSON schema.
func assertCardContract(t *testing.T, card *ServerCard) {
	t.Helper()

	raw, err := json.Marshal(card)
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))

	// Required by the schema: $schema, name, version, description.
	assert.Equal(t, SchemaURL, card.Schema)
	assert.NotEmpty(t, card.Name)
	assert.NotEmpty(t, card.Version)
	require.NotEmpty(t, card.Description)
	assert.LessOrEqual(t, len(card.Description), 100, "description must respect the schema maxLength")
	for _, key := range []string{"$schema", "name", "version", "description"} {
		assert.Contains(t, fields, key, "required field %q must be serialized", key)
	}

	// Remote-only: a Server Card never enumerates installable packages — those
	// stay in the registry server.json.
	assert.NotContains(t, fields, "packages", "Server Card must be remote-only and omit packages")
	require.Len(t, card.Remotes, 1)
	assert.Equal(t, "streamable-http", card.Remotes[0].Type)
}

func TestNewServerCard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		cfg               Config
		expectedVersion   string
		expectedRemoteURL string
	}{
		{
			name:              "defaults",
			cfg:               Config{},
			expectedVersion:   "0.0.0-dev",
			expectedRemoteURL: DefaultRemoteURL,
		},
		{
			name:              "explicit version",
			cfg:               Config{Version: "1.2.3"},
			expectedVersion:   "1.2.3",
			expectedRemoteURL: DefaultRemoteURL,
		},
		{
			name:              "per-environment remote URL",
			cfg:               Config{Version: "1.2.3", RemoteURL: "https://api.example.test/mcp/"},
			expectedVersion:   "1.2.3",
			expectedRemoteURL: "https://api.example.test/mcp/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			card := NewServerCard(tc.cfg)

			// Identity is reused from the registry document (server.json): it is
			// the stable Server Card / registry server name. The AI Catalog
			// identifier is assigned independently and is not derived from it.
			assert.Equal(t, "io.github.github/github-mcp-server", card.Name)
			assert.Equal(t, "GitHub", card.Title)
			assert.True(t, strings.HasPrefix(card.Description, "Connect AI assistants to GitHub"))
			assert.Equal(t, tc.expectedVersion, card.Version)
			assert.Equal(t, "https://github.com/github/github-mcp-server", card.WebsiteURL)

			require.NotNil(t, card.Repository)
			assert.Equal(t, "https://github.com/github/github-mcp-server", card.Repository.URL)
			assert.Equal(t, "github", card.Repository.Source)
			assert.Equal(t, "942771284", card.Repository.ID)

			assert.Equal(t, tc.expectedRemoteURL, card.Remotes[0].URL)

			assertCardContract(t, card)
		})
	}
}
