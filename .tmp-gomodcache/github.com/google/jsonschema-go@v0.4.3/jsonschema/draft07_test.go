// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jsonschema

import (
	"encoding/json"
	"testing"
)

// TestDraft07Schema tests draft-07 specific schema behaviors
func TestDraft07Schema(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		data   string
		valid  bool
	}{
		{
			name:   "draft-07 schema version",
			schema: `{"$schema": "http://json-schema.org/draft-07/schema#", "type": "string"}`,
			data:   `"hello"`,
			valid:  true,
		},
		{
			name:   "draft-07 schema version with https",
			schema: `{"$schema": "https://json-schema.org/draft-07/schema#", "type": "string"}`,
			data:   `"hello"`,
			valid:  true,
		},
		{
			name:   "invalid data against draft-07 schema",
			schema: `{"$schema": "http://json-schema.org/draft-07/schema#", "type": "string"}`,
			data:   `123`,
			valid:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.schema), &schema); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}

			var data interface{}
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatalf("failed to unmarshal data: %v", err)
			}

			rs, err := schema.Resolve(nil)
			if err != nil {
				t.Fatalf("failed to resolve schema: %v", err)
			}

			err = rs.Validate(data)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			} else if !tt.valid && err == nil {
				t.Errorf("expected invalid, got no error")
			}
		})
	}
}

// TestDraft07Dependencies tests draft-07 specific dependencies behavior
func TestDraft07Dependencies(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		data   string
		valid  bool
	}{
		{
			name: "draft-07 property dependencies",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"dependencies": {
					"billing_address": ["credit_card"]
				}
			}`,
			data:  `{"billing_address": "123 Main St"}`,
			valid: false,
		},
		{
			name: "draft-07 property dependencies satisfied",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"dependencies": {
					"billing_address": ["credit_card"]
				}
			}`,
			data:  `{"billing_address": "123 Main St", "credit_card": "1234"}`,
			valid: true,
		},
		{
			name: "draft-07 schema dependencies",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"dependencies": {
					"billing_address": {
						"properties": {
							"credit_card": {"type": "string"}
						},
						"required": ["credit_card"]
					}
				}
			}`,
			data:  `{"billing_address": "123 Main St"}`,
			valid: false,
		},
		{
			name: "draft-07 schema dependencies satisfied",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"dependencies": {
					"billing_address": {
						"properties": {
							"credit_card": {"type": "string"}
						},
						"required": ["credit_card"]
					}
				}
			}`,
			data:  `{"billing_address": "123 Main St", "credit_card": "1234"}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.schema), &schema); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}

			var data interface{}
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatalf("failed to unmarshal data: %v", err)
			}

			rs, err := schema.Resolve(nil)
			if err != nil {
				t.Fatalf("failed to resolve schema: %v", err)
			}

			err = rs.Validate(data)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			} else if !tt.valid && err == nil {
				t.Errorf("expected invalid, got no error")
			}
		})
	}
}

// TestDraft07ItemsArray tests draft-07 items array behavior (tuple validation)
func TestDraft07ItemsArray(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		data   string
		valid  bool
	}{
		{
			name: "draft-07 items array - valid tuple",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				]
			}`,
			data:  `["hello", 42]`,
			valid: true,
		},
		{
			name: "draft-07 items array - invalid first element",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				]
			}`,
			data:  `[123, 42]`,
			valid: false,
		},
		{
			name: "draft-07 items array - invalid second element",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				]
			}`,
			data:  `["hello", "world"]`,
			valid: false,
		},
		{
			name: "draft-07 items array with additionalItems",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				],
				"additionalItems": {"type": "boolean"}
			}`,
			data:  `["hello", 42, true]`,
			valid: true,
		},
		{
			name: "draft-07 items array with invalid additionalItems",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				],
				"additionalItems": {"type": "boolean"}
			}`,
			data:  `["hello", 42, "extra"]`,
			valid: false,
		},
		{
			name: "draft-07 items array with additionalItems false",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "array",
				"items": [
					{"type": "string"},
					{"type": "number"}
				],
				"additionalItems": false
			}`,
			data:  `["hello", 42, true]`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.schema), &schema); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}

			var data interface{}
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatalf("failed to unmarshal data: %v", err)
			}

			rs, err := schema.Resolve(nil)
			if err != nil {
				t.Fatalf("failed to resolve schema: %v", err)
			}

			err = rs.Validate(data)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			} else if !tt.valid && err == nil {
				t.Errorf("expected invalid, got no error")
			}
		})
	}
}

// TestDraft07Definitions tests draft-07 definitions keyword behavior
func TestDraft07Definitions(t *testing.T) {
	schema := `{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"type": "object",
		"definitions": {
			"address": {
				"type": "object",
				"properties": {
					"street": {"type": "string"},
					"city": {"type": "string"}
				},
				"required": ["street", "city"]
			}
		},
		"properties": {
			"home": {"$ref": "#/definitions/address"},
			"work": {"$ref": "#/definitions/address"}
		}
	}`

	tests := []struct {
		name  string
		data  string
		valid bool
	}{
		{
			name: "valid addresses",
			data: `{
				"home": {"street": "123 Main St", "city": "Anytown"},
				"work": {"street": "456 Oak Ave", "city": "Other City"}
			}`,
			valid: true,
		},
		{
			name: "invalid home address",
			data: `{
				"home": {"street": "123 Main St"},
				"work": {"street": "456 Oak Ave", "city": "Other City"}
			}`,
			valid: false,
		},
		{
			name: "invalid work address",
			data: `{
				"home": {"street": "123 Main St", "city": "Anytown"},
				"work": {"street": "456 Oak Ave"}
			}`,
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schemaObj Schema
			if err := json.Unmarshal([]byte(schema), &schemaObj); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}

			var data interface{}
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatalf("failed to unmarshal data: %v", err)
			}

			rs, err := schemaObj.Resolve(nil)
			if err != nil {
				t.Fatalf("failed to resolve schema: %v", err)
			}

			err = rs.Validate(data)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			} else if !tt.valid && err == nil {
				t.Errorf("expected invalid, got no error")
			}
		})
	}
}

// TestDraft07BooleanSchemas tests draft-07 boolean schema behavior
func TestDraft07BooleanSchemas(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		data   string
		valid  bool
	}{
		{
			name:   "true schema allows everything",
			schema: `true`,
			data:   `"anything"`,
			valid:  true,
		},
		{
			name:   "true schema allows numbers",
			schema: `true`,
			data:   `42`,
			valid:  true,
		},
		{
			name:   "true schema allows objects",
			schema: `true`,
			data:   `{"key": "value"}`,
			valid:  true,
		},
		{
			name:   "false schema rejects everything",
			schema: `false`,
			data:   `"anything"`,
			valid:  false,
		},
		{
			name:   "false schema rejects numbers",
			schema: `false`,
			data:   `42`,
			valid:  false,
		},
		{
			name:   "false schema rejects objects",
			schema: `false`,
			data:   `{"key": "value"}`,
			valid:  false,
		},
		{
			name: "boolean schema in object properties",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"always_valid": true,
					"never_valid": false
				}
			}`,
			data:  `{"always_valid": "anything", "never_valid": "something"}`,
			valid: false,
		},
		{
			name: "boolean schema in object properties - valid case",
			schema: `{
				"$schema": "http://json-schema.org/draft-07/schema#",
				"type": "object",
				"properties": {
					"always_valid": true
				}
			}`,
			data:  `{"always_valid": "anything"}`,
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.schema), &schema); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}

			var data interface{}
			if err := json.Unmarshal([]byte(tt.data), &data); err != nil {
				t.Fatalf("failed to unmarshal data: %v", err)
			}

			rs, err := schema.Resolve(nil)
			if err != nil {
				t.Fatalf("failed to resolve schema: %v", err)
			}

			err = rs.Validate(data)
			if tt.valid && err != nil {
				t.Errorf("expected valid, got error: %v", err)
			} else if !tt.valid && err == nil {
				t.Errorf("expected invalid, got no error")
			}
		})
	}
}

// TestDraft07Marshalling tests that draft-07 specific schemas marshal correctly
func TestDraft07Marshalling(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "draft-07 items array marshalling",
			input:    `{"items": [{"type": "string"}, {"type": "number"}]}`,
			expected: `{"items":[{"type":"string"},{"type":"number"}]}`,
		},
		{
			name:     "draft-07 dependencies marshalling",
			input:    `{"dependencies": {"name": ["first", "last"]}}`,
			expected: `{"dependencies":{"name":["first","last"]}}`,
		},
		{
			name:     "draft-07 dependencies marshalling complex",
			input:    `{"dependencies": {"name": ["first", "last"],"billing_address": {"required": ["shipping_address"],"properties": {"user_role": { "enum": ["preferred", "standard"] }}}}}`,
			expected: `{"dependencies":{"billing_address":{"properties":{"user_role":{"enum":["preferred","standard"]}},"required":["shipping_address"]},"name":["first","last"]}}`,
		},
		{
			name:     "draft-07 definitions marshalling",
			input:    `{"definitions": {"person": {"type": "object"}}}`,
			expected: `{"definitions":{"person":{"type":"object"}}}`,
		},
		{
			name:     "draft-07 schema with $schema",
			input:    `{"$schema": "http://json-schema.org/draft-07/schema#", "type": "string"}`,
			expected: `{"type":"string","$schema":"http://json-schema.org/draft-07/schema#"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var schema Schema
			if err := json.Unmarshal([]byte(tt.input), &schema); err != nil {
				t.Fatalf("failed to unmarshal schema: %v", err)
			}

			data, err := json.Marshal(&schema)
			if err != nil {
				t.Fatalf("failed to marshal schema: %v", err)
			}

			if string(data) != tt.expected {
				t.Errorf("marshalling mismatch:\ngot:  %s\nwant: %s", string(data), tt.expected)
			}
		})
	}
}
