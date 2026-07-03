package github

import (
	"context"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test_FieldsParamVariants_MutuallyExclusive guards the dual-variant
// registration for the fields_param feature flag. The flag-enabled tools and
// their Legacy* counterparts share a tool name, so exactly one of each pair must
// survive inventory filtering for any flag state. If both ever leaked, a client
// could be offered two tools with the same name. This asserts that each gated
// tool is present exactly once, advertising the `fields` parameter only when
// fields_param is enabled.
func Test_FieldsParamVariants_MutuallyExclusive(t *testing.T) {
	gatedTools := []string{
		"search_code",
		"get_file_contents",
		"list_issues",
		"list_releases",
		"list_pull_requests",
		"search_issues",
		"search_pull_requests",
		"list_commits",
	}

	for _, tc := range []struct {
		name          string
		flagEnabled   bool
		expectFields  bool
		featureChecks func(context.Context, string) (bool, error)
	}{
		{
			name:          "flag off registers the legacy variant without fields",
			flagEnabled:   false,
			expectFields:  false,
			featureChecks: featureCheckerFor(), // fields_param disabled
		},
		{
			name:          "flag on registers the fields variant with fields",
			flagEnabled:   true,
			expectFields:  true,
			featureChecks: featureCheckerFor(FeatureFlagFieldsParam),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inv, err := NewInventory(translations.NullTranslationHelper).
				WithToolsets([]string{"all"}).
				WithFeatureChecker(tc.featureChecks).
				Build()
			require.NoError(t, err)

			available := inv.AvailableTools(context.Background())

			counts := make(map[string]int, len(available))
			for _, tool := range available {
				counts[tool.Tool.Name]++
			}

			// Each gated tool must be present exactly once (never both variants)
			// and advertise `fields` only when the flag is enabled.
			for _, name := range gatedTools {
				require.Equalf(t, 1, counts[name], "expected exactly one %q for flagEnabled=%v; dual variants must be mutually exclusive", name, tc.flagEnabled)

				tool := requireToolByName(t, available, name)
				schema, ok := tool.Tool.InputSchema.(*jsonschema.Schema)
				require.Truef(t, ok, "%q InputSchema should be *jsonschema.Schema", name)

				if tc.expectFields {
					assert.Containsf(t, schema.Properties, "fields", "%q should advertise fields when flag is on", name)
					assert.Equalf(t, FeatureFlagFieldsParam, tool.FeatureFlagEnable, "%q should be the flag-enabled variant", name)
				} else {
					assert.NotContainsf(t, schema.Properties, "fields", "%q must not advertise fields when flag is off", name)
					assert.Containsf(t, tool.FeatureFlagDisable, FeatureFlagFieldsParam, "%q should be the legacy (flag-disabled) variant", name)
				}
			}
		})
	}
}
