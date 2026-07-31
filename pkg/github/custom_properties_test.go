package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/google/go-github/v87/github"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/github/github-mcp-server/internal/toolsnaps"
	"github.com/github/github-mcp-server/pkg/translations"
)

func Test_GetRepositoryCustomProperties(t *testing.T) {
	toolDef := GetRepositoryCustomProperties(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_repository_custom_properties", toolDef.Tool.Name)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo"})

	mockValues := []*github.CustomPropertyValue{{PropertyName: "environment", Value: "production"}}
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		"GET /repos/{owner}/{repo}/properties/values": mockResponse(t, http.StatusOK, mockValues),
	}))
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var returned []*github.CustomPropertyValue
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
	require.Len(t, returned, 1)
	assert.Equal(t, "environment", returned[0].PropertyName)
}

func Test_CreateOrUpdateRepositoryCustomProperties(t *testing.T) {
	toolDef := CreateOrUpdateRepositoryCustomProperties(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_or_update_repository_custom_properties", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"owner", "repo", "properties"})

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
		"owner": "owner",
		"repo":  "repo",
		"properties": []any{
			map[string]any{"property_name": "environment", "value": "production"},
		},
	})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	require.Len(t, captured.Properties, 1)
	assert.Equal(t, "environment", captured.Properties[0].PropertyName)
	assert.Equal(t, "production", captured.Properties[0].Value)
}

func Test_CreateOrUpdateRepositoryCustomProperties_MissingProperties(t *testing.T) {
	toolDef := CreateOrUpdateRepositoryCustomProperties(translations.NullTranslationHelper)
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{}))
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{"owner": "owner", "repo": "repo"})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.True(t, result.IsError)
	assert.Contains(t, getErrorResult(t, result).Text, "properties parameter is required")
}

func Test_GetOrganizationCustomProperties(t *testing.T) {
	toolDef := GetOrganizationCustomProperties(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_organization_custom_properties", toolDef.Tool.Name)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"org"})

	mockProps := []*github.CustomProperty{{PropertyName: github.Ptr("environment"), ValueType: "single_select"}}
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		"GET /orgs/{org}/properties/schema": mockResponse(t, http.StatusOK, mockProps),
	}))
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{"org": "octo"})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var returned []*github.CustomProperty
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
	require.Len(t, returned, 1)
	assert.Equal(t, "environment", returned[0].GetPropertyName())
}

func Test_CreateOrUpdateOrganizationCustomProperties(t *testing.T) {
	toolDef := CreateOrUpdateOrganizationCustomProperties(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_or_update_organization_custom_properties", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"org", "properties"})

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
		"org": "octo",
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
}

func Test_GetEnterpriseCustomProperties(t *testing.T) {
	toolDef := GetEnterpriseCustomProperties(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "get_enterprise_custom_properties", toolDef.Tool.Name)
	assert.True(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"enterprise"})

	mockProps := []*github.CustomProperty{{PropertyName: github.Ptr("compliance"), ValueType: "true_false"}}
	client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
		"GET /enterprises/{enterprise}/properties/schema": mockResponse(t, http.StatusOK, mockProps),
	}))
	deps := BaseDeps{Client: client}
	handler := toolDef.Handler(deps)
	request := createMCPRequest(map[string]any{"enterprise": "acme"})

	result, err := handler(ContextWithDeps(context.Background(), deps), &request)
	require.NoError(t, err)
	require.False(t, result.IsError)

	var returned []*github.CustomProperty
	require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
	require.Len(t, returned, 1)
	assert.Equal(t, "compliance", returned[0].GetPropertyName())
}

func Test_CreateOrUpdateEnterpriseCustomProperties(t *testing.T) {
	toolDef := CreateOrUpdateEnterpriseCustomProperties(translations.NullTranslationHelper)
	require.NoError(t, toolsnaps.Test(toolDef.Tool.Name, toolDef.Tool))

	assert.Equal(t, "create_or_update_enterprise_custom_properties", toolDef.Tool.Name)
	assert.False(t, toolDef.Tool.Annotations.ReadOnlyHint)

	schema, ok := toolDef.Tool.InputSchema.(*jsonschema.Schema)
	require.True(t, ok)
	assert.ElementsMatch(t, schema.Required, []string{"enterprise", "properties"})

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
}
