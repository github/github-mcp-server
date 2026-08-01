// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestUnmarshalCaseSensitivity(t *testing.T) {
	type Nested struct {
		Field string `json:"field"`
	}
	type Target struct {
		Field       string
		TaggedField string `json:"custom_tag"`
		Nested      *Nested
	}

	tests := []struct {
		name string
		json string
		want Target
	}{
		{
			name: "exact match",
			json: `{"Field": "value", "custom_tag": "tagged", "Nested": {"field": "nested"}}`,
			want: Target{
				Field:       "value",
				TaggedField: "tagged",
				Nested: &Nested{
					Field: "nested",
				},
			},
		},
		{
			name: "case mismatch",
			json: `{"field": "value", "Custom_tag": "tagged", "Nested": {"Field": "nested"}}`,
			want: Target{
				Field:       "",
				TaggedField: "",
				Nested: &Nested{
					Field: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Target
			if err := Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Unmarshal mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewDecoderCaseSensitivity(t *testing.T) {
	type Target struct {
		Field       string `json:"field"`
		TaggedField string `json:"custom_tag"`
	}

	tests := []struct {
		name string
		json string
		want Target
	}{
		{
			name: "exact match",
			json: `{"field": "value", "custom_tag": "tagged"}`,
			want: Target{Field: "value", TaggedField: "tagged"},
		},
		{
			name: "case mismatch",
			json: `{"Field": "value", "Custom_tag": "tagged"}`,
			want: Target{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Target
			dec := NewDecoder(strings.NewReader(tt.json))
			if err := dec.Decode(&got); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Decode mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestUnmarshalNullCharacter(t *testing.T) {
	type Target struct {
		Field string `json:"field"`
	}

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "null char in middle",
			json: `{"fi\u0000eld": "value"}`,
		},
		{
			name: "null char at end",
			json: `{"field\u0000": "value"}`,
		},
		{
			name: "null char at start",
			json: `{"\u0000field": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Target
			if err := Unmarshal([]byte(tt.json), &got); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
			if got.Field != "" {
				t.Errorf("Field was set: %s", tt.json)
			}
		})
	}
}

func FuzzUnmarshalKeyComparison(f *testing.F) {
	// Add seed corpus for expected valid key and the known problematic cases.
	f.Add("field")
	f.Add("FIELD")
	f.Add("field\x00")

	f.Fuzz(func(t *testing.T, key string) {
		// Generate valid JSON with the fuzzed key.
		// json.Encoder ensures the key is properly escaped (e.g. \x00 becomes \u0000).
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(key); err != nil {
			return
		}
		encodedKey := buf.Bytes()
		jsonData := fmt.Sprintf(`{%s: "value"}`, encodedKey)

		type Target struct {
			Field string `json:"field"`
		}

		var got Target
		if err := Unmarshal([]byte(jsonData), &got); err != nil {
			return
		}

		if got.Field == "value" && key != "field" {
			t.Errorf("Unmarshal matched key %q (JSON: %s) to field 'field'", key, encodedKey)
		}
	})
}

func FuzzUnmarshalFieldIsolation(f *testing.F) {
	// Add seed corpus.
	f.Add("other", "value")
	f.Add("safe", "overwrite")
	f.Add("safe\x00", "attack")

	f.Fuzz(func(t *testing.T, key, val string) {
		// Generate valid JSON with the fuzzed key and value.
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(key); err != nil {
			return
		}
		encodedKey := buf.Bytes()
		buf.Reset()
		if err := enc.Encode(val); err != nil {
			return
		}
		encodedVal := buf.Bytes()

		jsonData := fmt.Sprintf(`{"safe": "value", %s: %s}`, encodedKey, encodedVal)

		type Target struct {
			Safe string `json:"safe"`
		}

		var got Target
		if err := Unmarshal([]byte(jsonData), &got); err != nil {
			return
		}

		if got.Safe != "value" && key != "safe" {
			t.Errorf("Field 'safe' improperly modified by key %q (JSON: %s). Got %q, want %q", key, encodedKey, got.Safe, "value")
		}
	})
}
