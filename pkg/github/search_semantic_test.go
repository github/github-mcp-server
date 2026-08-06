package github

import (
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_stripFreeTextQuotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "leaves an unquoted query alone",
			query:    "is:issue sticky sidebar",
			expected: "is:issue sticky sidebar",
		},
		{
			name:     "strips quotes around free text",
			query:    `is:issue "sticky sidebar"`,
			expected: "is:issue sticky sidebar",
		},
		{
			name:     "preserves quotes around a multi-word qualifier value",
			query:    `is:issue label:"needs triage"`,
			expected: `is:issue label:"needs triage"`,
		},
		{
			name:     "strips free text but preserves the qualifier alongside it",
			query:    `is:issue label:"needs triage" "sticky sidebar"`,
			expected: `is:issue label:"needs triage" sticky sidebar`,
		},
		{
			name:     "preserves quotes on a hyphenated qualifier",
			query:    `is:issue state-reason:"not planned"`,
			expected: `is:issue state-reason:"not planned"`,
		},
		{
			name:     "preserves quotes on a dotted custom field qualifier",
			query:    `is:issue field.priority:"P1 urgent"`,
			expected: `is:issue field.priority:"P1 urgent"`,
		},
		{
			name:     "preserves quotes on a negated qualifier",
			query:    `is:issue -label:"wont fix"`,
			expected: `is:issue -label:"wont fix"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, stripFreeTextQuotes(tt.query))
		})
	}
}

func Test_searchIssuesTool_descriptionMatchesEngine(t *testing.T) {
	t.Parallel()

	// The description has to describe the engine the host will actually use.
	// Steering a lexical-only host toward paraphrased natural language is actively misleading.
	semantic := SearchIssues(translations.NullTranslationHelper, WithHost(utils.HostTypeDotcom))
	lexical := SearchIssues(translations.NullTranslationHelper, WithHost(utils.HostTypeGHES))

	require.Equal(t, "search_issues", semantic.Tool.Name)
	require.Equal(t, "search_issues", lexical.Tool.Name)

	assert.Equal(t, searchIssuesSemanticDescription, semantic.Tool.Description)
	assert.Equal(t, searchIssuesLexicalDescription, lexical.Tool.Description)

	semanticSchema, ok := semantic.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	lexicalSchema, ok := lexical.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)

	assert.Equal(t, searchIssuesSemanticQueryDescription, semanticSchema.Properties["query"].Description)
	assert.Equal(t, searchIssuesLexicalQueryDescription, lexicalSchema.Properties["query"].Description)
}
