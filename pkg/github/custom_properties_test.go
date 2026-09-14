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
		mockProps := []map[string]any{{
			"property_name":           "environment",
			"value_type":              "single_select",
			"require_explicit_values": true,
		}}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/properties/schema": mockResponse(t, http.StatusOK, mockProps),
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{"level": "organization", "org": "octo"})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		var returned []map[string]any
		require.NoError(t, json.Unmarshal([]byte(getTextResult(t, result).Text), &returned))
		require.Len(t, returned, 1)
		assert.Equal(t, "environment", returned[0]["property_name"])
		assert.Equal(t, true, returned[0]["require_explicit_values"])
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

	t.Run("property items use level-specific schemas", func(t *testing.T) {
		itemSchema := schema.Properties["properties"].Items
		assert.Equal(t, "object", itemSchema.Type)
		require.Len(t, itemSchema.OneOf, 2)
		valueItemSchema := itemSchema.OneOf[0]
		definitionItemSchema := itemSchema.OneOf[1]
		require.NotNil(t, valueItemSchema.AdditionalProperties.Not)
		require.NotNil(t, definitionItemSchema.AdditionalProperties.Not)
		assert.ElementsMatch(t, []string{"property_name", "value"}, valueItemSchema.Required)
		assert.ElementsMatch(t, []string{"property_name"}, definitionItemSchema.Required)
		assert.Equal(t, "boolean", definitionItemSchema.Properties["require_explicit_values"].Type)

		resolvedValue, err := valueItemSchema.Resolve(nil)
		require.NoError(t, err)
		resolvedDefinition, err := definitionItemSchema.Resolve(nil)
		require.NoError(t, err)

		valueSchema := valueItemSchema.Properties["value"]
		require.Len(t, valueSchema.OneOf, 3)
		assert.Equal(t, "string", valueSchema.OneOf[0].Type)
		assert.Equal(t, "array", valueSchema.OneOf[1].Type)
		assert.Equal(t, "string", valueSchema.OneOf[1].Items.Type)
		assert.Equal(t, "null", valueSchema.OneOf[2].Type)

		defaultValueSchema := definitionItemSchema.Properties["default_value"]
		require.Len(t, defaultValueSchema.OneOf, 3)
		assert.Equal(t, "string", defaultValueSchema.OneOf[0].Type)
		assert.Equal(t, "array", defaultValueSchema.OneOf[1].Type)
		assert.Equal(t, "string", defaultValueSchema.OneOf[1].Items.Type)
		assert.Equal(t, "null", defaultValueSchema.OneOf[2].Type)

		tests := []struct {
			name       string
			definition bool
			property   map[string]any
			shouldPass bool
		}{
			{name: "repository string", property: map[string]any{"property_name": "environment", "value": "production"}, shouldPass: true},
			{name: "repository string array", property: map[string]any{"property_name": "environment", "value": []any{"production", "staging"}}, shouldPass: true},
			{name: "repository null", property: map[string]any{"property_name": "environment", "value": nil}, shouldPass: true},
			{name: "repository number", property: map[string]any{"property_name": "environment", "value": 1}},
			{name: "repository boolean", property: map[string]any{"property_name": "environment", "value": true}},
			{name: "repository object", property: map[string]any{"property_name": "environment", "value": map[string]any{"name": "production"}}},
			{name: "repository definition field", property: map[string]any{"property_name": "environment", "value": "production", "required": true}},
			{
				name:       "definition all fields",
				definition: true,
				property: map[string]any{
					"property_name":           "environment",
					"value_type":              "single_select",
					"required":                true,
					"default_value":           "production",
					"description":             "Deployment environment",
					"allowed_values":          []any{"production", "staging"},
					"values_editable_by":      "org_and_repo_actors",
					"require_explicit_values": true,
				},
				shouldPass: true,
			},
			{name: "partial definition", definition: true, property: map[string]any{"property_name": "environment", "required": false}, shouldPass: true},
			{name: "definition array default", definition: true, property: map[string]any{"property_name": "compliance", "value_type": "multi_select", "default_value": []any{"soc2", "fedramp"}}, shouldPass: true},
			{name: "definition explicit nulls", definition: true, property: map[string]any{"property_name": "environment", "default_value": nil, "description": nil, "allowed_values": nil, "values_editable_by": nil}, shouldPass: true},
			{name: "definition number default", definition: true, property: map[string]any{"property_name": "environment", "value_type": "string", "default_value": 1}},
			{name: "definition boolean default", definition: true, property: map[string]any{"property_name": "environment", "value_type": "string", "default_value": true}},
			{name: "definition object default", definition: true, property: map[string]any{"property_name": "environment", "value_type": "string", "default_value": map[string]any{"name": "production"}}},
			{name: "definition repository field", definition: true, property: map[string]any{"property_name": "environment", "value_type": "string", "value": "production"}},
			{name: "definition misspelled field", definition: true, property: map[string]any{"property_name": "environment", "value_type": "string", "require": true}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resolved := resolvedValue
				if tt.definition {
					resolved = resolvedDefinition
				}
				err := resolved.Validate(tt.property)
				if tt.shouldPass {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			})
		}
	})

	t.Run("repository level: sets property values with one request", func(t *testing.T) {
		requests := 0
		var captured struct {
			Properties []*github.CustomPropertyValue `json:"properties"`
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"PATCH /repos/{owner}/{repo}/properties/values": func(w http.ResponseWriter, r *http.Request) {
				requests++
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
				map[string]any{"property_name": "deprecated", "value": nil},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Contains(t, getTextResult(t, result).Text, "updated successfully")
		assert.Equal(t, 1, requests)

		require.Len(t, captured.Properties, 2)
		assert.Equal(t, "environment", captured.Properties[0].PropertyName)
		assert.Equal(t, "production", captured.Properties[0].Value)
		assert.Equal(t, "deprecated", captured.Properties[1].PropertyName)
		assert.Nil(t, captured.Properties[1].Value)
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

	t.Run("organization level: merges mixed create and update in two requests", func(t *testing.T) {
		requests := 0
		var captured struct {
			Properties []map[string]any `json:"properties"`
		}
		current := []map[string]any{
			{
				"property_name":           "environment",
				"source_type":             "organization",
				"url":                     "https://api.github.com/orgs/octo/properties/schema/environment",
				"value_type":              "single_select",
				"required":                true,
				"default_value":           "production",
				"description":             "Deployment environment",
				"allowed_values":          []string{"production", "staging"},
				"values_editable_by":      "org_and_repo_actors",
				"require_explicit_values": true,
			},
			{
				"property_name":           "legacy",
				"source_type":             "organization",
				"value_type":              "string",
				"required":                false,
				"default_value":           nil,
				"description":             "",
				"allowed_values":          []string{},
				"values_editable_by":      nil,
				"require_explicit_values": false,
			},
			{
				"property_name": "untouched",
				"source_type":   "organization",
				"value_type":    "string",
			},
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /orgs/{org}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
				requests++
				mockResponse(t, http.StatusOK, current)(w, r)
			},
			"PATCH /orgs/{org}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
				requests++
				body, _ := io.ReadAll(r.Body)
				require.NoError(t, json.Unmarshal(body, &captured))
				mockResponse(t, http.StatusOK, captured.Properties)(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level": "organization",
			"org":   "octo",
			"properties": []any{
				map[string]any{
					"property_name": "environment",
					"required":      false,
				},
				map[string]any{
					"property_name":           "legacy",
					"description":             nil,
					"allowed_values":          []any{},
					"require_explicit_values": false,
				},
				map[string]any{
					"property_name":           "service",
					"value_type":              "string",
					"required":                false,
					"default_value":           nil,
					"description":             "",
					"allowed_values":          nil,
					"values_editable_by":      nil,
					"require_explicit_values": false,
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, 2, requests)
		require.Len(t, captured.Properties, 3)

		environment := captured.Properties[0]
		assert.Equal(t, "environment", environment["property_name"])
		assert.Equal(t, "single_select", environment["value_type"])
		assert.Equal(t, false, environment["required"])
		assert.Equal(t, "production", environment["default_value"])
		assert.Equal(t, "Deployment environment", environment["description"])
		assert.Equal(t, []any{"production", "staging"}, environment["allowed_values"])
		assert.Equal(t, "org_and_repo_actors", environment["values_editable_by"])
		assert.Equal(t, true, environment["require_explicit_values"])
		assert.NotContains(t, environment, "source_type")
		assert.NotContains(t, environment, "url")

		legacy := captured.Properties[1]
		assert.Equal(t, "string", legacy["value_type"])
		assert.Equal(t, false, legacy["required"])
		assert.Contains(t, legacy, "default_value")
		assert.Nil(t, legacy["default_value"])
		assert.Contains(t, legacy, "description")
		assert.Nil(t, legacy["description"])
		assert.Contains(t, legacy, "allowed_values")
		assert.Empty(t, legacy["allowed_values"])
		assert.Contains(t, legacy, "values_editable_by")
		assert.Nil(t, legacy["values_editable_by"])
		assert.Equal(t, false, legacy["require_explicit_values"])

		service := captured.Properties[2]
		assert.Equal(t, "string", service["value_type"])
		assert.Equal(t, false, service["required"])
		assert.Contains(t, service, "default_value")
		assert.Nil(t, service["default_value"])
		assert.Equal(t, "", service["description"])
		assert.Contains(t, service, "allowed_values")
		assert.Nil(t, service["allowed_values"])
		assert.Contains(t, service, "values_editable_by")
		assert.Nil(t, service["values_editable_by"])
		assert.Equal(t, false, service["require_explicit_values"])
	})

	t.Run("enterprise level: merges an existing definition", func(t *testing.T) {
		requests := 0
		var captured struct {
			Properties []map[string]any `json:"properties"`
		}
		client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
			"GET /enterprises/{enterprise}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
				requests++
				mockResponse(t, http.StatusOK, []map[string]any{{
					"property_name":           "compliance",
					"source_type":             "enterprise",
					"value_type":              "multi_select",
					"required":                true,
					"default_value":           []string{"soc2"},
					"description":             "Compliance frameworks",
					"allowed_values":          []string{"soc2", "fedramp"},
					"values_editable_by":      "org_actors",
					"require_explicit_values": true,
				}})(w, r)
			},
			"PATCH /enterprises/{enterprise}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
				requests++
				body, _ := io.ReadAll(r.Body)
				require.NoError(t, json.Unmarshal(body, &captured))
				mockResponse(t, http.StatusOK, captured.Properties)(w, r)
			},
		}))
		deps := BaseDeps{Client: client}
		handler := toolDef.Handler(deps)
		request := createMCPRequest(map[string]any{
			"level":      "enterprise",
			"enterprise": "acme",
			"properties": []any{
				map[string]any{
					"property_name": "compliance",
					"default_value": []any{"soc2", "fedramp"},
				},
			},
		})

		result, err := handler(ContextWithDeps(context.Background(), deps), &request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		assert.Equal(t, 2, requests)
		require.Len(t, captured.Properties, 1)
		assert.Equal(t, "multi_select", captured.Properties[0]["value_type"])
		assert.Equal(t, []any{"soc2", "fedramp"}, captured.Properties[0]["default_value"])
		assert.Equal(t, true, captured.Properties[0]["require_explicit_values"])
	})

	t.Run("rejects invalid definition writes before patch", func(t *testing.T) {
		tests := []struct {
			name             string
			args             map[string]any
			current          []map[string]any
			getStatus        int
			expectedError    string
			expectedRequests int
		}{
			{
				name: "organization definition without value_type",
				args: map[string]any{
					"level":      "organization",
					"org":        "octo",
					"properties": []any{map[string]any{"property_name": "environment"}},
				},
				expectedError:    "value_type is required for new property",
				expectedRequests: 1,
			},
			{
				name: "duplicate property names",
				args: map[string]any{
					"level": "organization",
					"org":   "octo",
					"properties": []any{
						map[string]any{"property_name": "environment"},
						map[string]any{"property_name": "environment"},
					},
				},
				expectedError: "duplicates",
			},
			{
				name: "inherited enterprise definition",
				args: map[string]any{
					"level":      "organization",
					"org":        "octo",
					"properties": []any{map[string]any{"property_name": "environment"}},
				},
				current: []map[string]any{{
					"property_name": "environment",
					"source_type":   "enterprise",
					"value_type":    "string",
				}},
				expectedError:    "inherited from enterprise",
				expectedRequests: 1,
			},
			{
				name: "GET failure",
				args: map[string]any{
					"level":      "organization",
					"org":        "octo",
					"properties": []any{map[string]any{"property_name": "environment"}},
				},
				getStatus:        http.StatusInternalServerError,
				expectedError:    "failed to get organization custom properties before updating",
				expectedRequests: 1,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				requests := 0
				status := tt.getStatus
				if status == 0 {
					status = http.StatusOK
				}
				client := mustNewGHClient(t, MockHTTPClientWithHandlers(map[string]http.HandlerFunc{
					"GET /orgs/{org}/properties/schema": func(w http.ResponseWriter, r *http.Request) {
						requests++
						mockResponse(t, status, tt.current)(w, r)
					},
					"PATCH /orgs/{org}/properties/schema": func(w http.ResponseWriter, _ *http.Request) {
						requests++
						w.WriteHeader(http.StatusOK)
					},
				}))
				deps := BaseDeps{Client: client}
				handler := toolDef.Handler(deps)
				request := createMCPRequest(tt.args)

				result, err := handler(ContextWithDeps(context.Background(), deps), &request)
				require.NoError(t, err)
				require.True(t, result.IsError)
				assert.Contains(t, getErrorResult(t, result).Text, tt.expectedError)
				assert.Equal(t, tt.expectedRequests, requests)
			})
		}
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
