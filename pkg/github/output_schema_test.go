package github

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type outputSchemaTestPayload struct {
	Message string `json:"message"`
}

func TestMustOutputSchema(t *testing.T) {
	schema := MustOutputSchema[outputSchemaTestPayload]()

	require.NotNil(t, schema)
	assert.Equal(t, "object", schema.Type)
	assert.Contains(t, schema.Properties, "message")
}

func TestMustOutputSchemaPanicsForNonObject(t *testing.T) {
	require.Panics(t, func() {
		MustOutputSchema[string]()
	})
}

func TestStructuredTextResultStructuredContentFeatureFlag(t *testing.T) {
	textValue := map[string]string{"message": "text"}
	structuredContent := outputSchemaTestPayload{Message: "structured"}

	tests := []struct {
		name             string
		featureEnabled   bool
		wantStructured   bool
		wantFeatureCheck bool
	}{
		{
			name:             "omits structured content by default",
			wantStructured:   false,
			wantFeatureCheck: true,
		},
		{
			name:             "includes structured content when output schemas are enabled",
			featureEnabled:   true,
			wantStructured:   true,
			wantFeatureCheck: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var checkedFeature string
			deps := BaseDeps{
				featureChecker: func(_ context.Context, flag string) (bool, error) {
					checkedFeature = flag
					return tt.featureEnabled, nil
				},
			}

			result, err := structuredTextResult(context.Background(), deps, textValue, structuredContent)
			require.NoError(t, err)
			require.NotNil(t, result)

			textContent := getTextResult(t, result)
			var gotText map[string]string
			require.NoError(t, json.Unmarshal([]byte(textContent.Text), &gotText))
			assert.Equal(t, textValue, gotText)

			if tt.wantStructured {
				assert.Equal(t, structuredContent, result.StructuredContent)
			} else {
				assert.Nil(t, result.StructuredContent)
			}
			if tt.wantFeatureCheck {
				assert.Equal(t, FeatureFlagOutputSchemas, checkedFeature)
			}
		})
	}
}

func TestStructuredTextResultMarshalError(t *testing.T) {
	_, err := structuredTextResult(context.Background(), BaseDeps{}, map[string]any{
		"invalid": func() {},
	}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal response")
}
