package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-github/v89/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
)

func Test_CustomPropertiesRead(t *testing.T) {
	toolDef := CustomPropertiesRead(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "custom_properties_read", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"level"})

	t.Run("repository level: returns property values", func(t *testing.T) {
		mockValues := []*github.CustomPropertyValue{{PropertyName: "environment", Value: "production"}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /repos/{owner}/{repo}/properties/values": mockResponse(t, http.StatusOK, mockValues),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned []*github.CustomPropertyValue
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "environment", returned[0].PropertyName)
	})

	t.Run("repository level: requires owner and repo", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "owner": "owner"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "repo")
	})

	t.Run("organization level: returns property definitions", func(t *testing.T) {
		mockProps := []*github.CustomProperty{{PropertyName: github.Ptr("environment"), ValueType: "single_select"}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/properties/schema": mockResponse(t, http.StatusOK, mockProps),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned []*github.CustomProperty
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "environment", returned[0].GetPropertyName())
	})

	t.Run("organization level: requires org", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "org")
	})

	t.Run("enterprise level: returns property definitions", func(t *testing.T) {
		mockProps := []*github.CustomProperty{{PropertyName: github.Ptr("compliance"), ValueType: "true_false"}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /enterprises/{enterprise}/properties/schema": mockResponse(t, http.StatusOK, mockProps),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "enterprise", "enterprise": "acme"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned []*github.CustomProperty
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "compliance", returned[0].GetPropertyName())
	})

	t.Run("unknown level returns an error", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "team"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown level")
	})
}

func Test_CustomPropertiesWrite(t *testing.T) {
	toolDef := CustomPropertiesWrite(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "custom_properties_write", toolDef.Tool.Name)
	assert.NotEmpty(t, toolDef.Tool.Description)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok, "InputSchema should be *jsonschema.Schema")
	assert.ElementsMatch(t, schema.Required, []string{"level", "properties"})

	t.Run("repository level: sets property values", func(t *testing.T) {
		var captured struct {
			Properties []*github.CustomPropertyValue `json:"properties"`
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"PATCH /repos/{owner}/{repo}/properties/values": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &captured)
				w.WriteHeader(http.StatusNoContent)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level": "repository",
			"owner": "owner",
			"repo":  "repo",
			"properties": []any{
				map[string]any{"property_name": "environment", "value": "production"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "updated successfully")

		require.Len(t, captured.Properties, 1)
		assert.Equal(t, "environment", captured.Properties[0].PropertyName)
		assert.Equal(t, "production", captured.Properties[0].Value)
	})

	t.Run("repository level: requires properties", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "repository", "owner": "owner", "repo": "repo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "properties parameter is required")
	})

	t.Run("organization level: defines property schema", func(t *testing.T) {
		var captured struct {
			Properties []*github.CustomProperty `json:"properties"`
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"PATCH /orgs/{org}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &captured)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[{"property_name":"environment","value_type":"single_select"}]`))
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level": "organization",
			"org":   "octo",
			"properties": []any{
				map[string]any{
					"property_name":  "environment",
					"value_type":     "single_select",
					"required":       true,
					"allowed_values": []any{"production", "staging"},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.Len(t, captured.Properties, 1)
		assert.Equal(t, "environment", captured.Properties[0].GetPropertyName())
		assert.Equal(t, github.PropertyValueType("single_select"), captured.Properties[0].ValueType)
		assert.ElementsMatch(t, []string{"production", "staging"}, captured.Properties[0].AllowedValues)
	})

	t.Run("enterprise level: defines property schema", func(t *testing.T) {
		var captured struct {
			Properties []*github.CustomProperty `json:"properties"`
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"PATCH /enterprises/{enterprise}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				_ = json.Unmarshal(body, &captured)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[{"property_name":"compliance","value_type":"true_false"}]`))
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":      "enterprise",
			"enterprise": "acme",
			"properties": []any{
				map[string]any{"property_name": "compliance", "value_type": "true_false"},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		require.Len(t, captured.Properties, 1)
		assert.Equal(t, "compliance", captured.Properties[0].GetPropertyName())
	})

	t.Run("unknown level returns an error", func(t *testing.T) {
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":      "team",
			"properties": []any{map[string]any{"property_name": "x"}},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.True(t, result.IsError)
		assert.Contains(t, getErrorResult(t, result).Text, "unknown level")
	})
}
