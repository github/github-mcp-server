package servercard

import (
	_ "embed"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/server-card.schema.json
var serverCardSchema []byte

// resolvedCardSchema parses the embedded experimental-ext-server-card schema and
// returns a resolver rooted at the ServerCard definition, so emitted cards can be
// validated against the canonical JSON Schema.
func resolvedCardSchema(t *testing.T) *jsonschema.Resolved {
	t.Helper()

	var schema jsonschema.Schema
	require.NoError(t, json.Unmarshal(serverCardSchema, &schema))

	root := &jsonschema.Schema{
		Schema: schema.Schema,
		Ref:    "#/$defs/ServerCard",
		Defs:   schema.Defs,
	}

	resolved, err := root.Resolve(nil)
	require.NoError(t, err)
	return resolved
}

// assertSchemaValid marshals card and validates it against the ServerCard schema.
func assertSchemaValid(t *testing.T, resolved *jsonschema.Resolved, card *ServerCard) {
	t.Helper()

	raw, err := json.Marshal(card)
	require.NoError(t, err)

	var instance any
	require.NoError(t, json.Unmarshal(raw, &instance))

	require.NoError(t, resolved.Validate(instance), "card must conform to the Server Card schema")
}

func TestNewServerCard(t *testing.T) {
	t.Parallel()

	resolved := resolvedCardSchema(t)

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

			assert.Equal(t, SchemaURL, card.Schema)
			assert.Equal(t, "io.github.github/github-mcp-server", card.Name)
			assert.Equal(t, "GitHub", card.Title)
			assert.Equal(t, tc.expectedVersion, card.Version)
			assert.LessOrEqual(t, len(card.Description), 100, "description must respect the schema maxLength")

			require.NotNil(t, card.Repository)
			assert.Equal(t, "https://github.com/github/github-mcp-server", card.Repository.URL)
			assert.Equal(t, "github", card.Repository.Source)
			assert.Equal(t, "942771284", card.Repository.ID)

			require.Len(t, card.Remotes, 1)
			assert.Equal(t, "streamable-http", card.Remotes[0].Type)
			assert.Equal(t, tc.expectedRemoteURL, card.Remotes[0].URL)
			require.Len(t, card.Remotes[0].Headers, 1)
			assert.Equal(t, "Authorization", card.Remotes[0].Headers[0].Name)
			assert.True(t, card.Remotes[0].Headers[0].IsSecret)

			assertSchemaValid(t, resolved, card)
		})
	}
}

// TestServerCardIsRemoteOnly guards the SEP-2127 requirement that a Server Card
// never enumerates installable packages — those stay in the registry server.json.
func TestServerCardIsRemoteOnly(t *testing.T) {
	t.Parallel()

	raw, err := json.Marshal(NewServerCard(Config{}))
	require.NoError(t, err)

	var fields map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &fields))

	_, hasPackages := fields["packages"]
	assert.False(t, hasPackages, "Server Card must be remote-only and omit packages")
	assert.Contains(t, fields, "remotes")
}

// TestServerCardIdentityMatchesRegistry keeps the card's identity fields aligned
// with the static registry document (server.json).
func TestServerCardIdentityMatchesRegistry(t *testing.T) {
	t.Parallel()

	card := NewServerCard(Config{})

	assert.Equal(t, "io.github.github/github-mcp-server", card.Name)
	assert.Equal(t, "GitHub", card.Title)
	assert.True(t, strings.HasPrefix(card.Description, "Connect AI assistants to GitHub"))
}
