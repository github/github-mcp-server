// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import (
	"reflect"
	"strings"
	"testing"
)

func TestClientCredentials_Validate(t *testing.T) {
	tests := []struct {
		name    string
		creds   ClientCredentials
		wantErr string
	}{
		{
			name:  "valid public client",
			creds: ClientCredentials{ClientID: "my-client"},
		},
		{
			name: "valid confidential client",
			creds: ClientCredentials{
				ClientID: "my-client",
				ClientSecretAuth: &ClientSecretAuth{
					ClientSecret: "my-secret",
				},
			},
		},
		{
			name:    "empty client ID",
			creds:   ClientCredentials{},
			wantErr: "ClientID is required",
		},
		{
			name: "empty secret in ClientSecretAuth",
			creds: ClientCredentials{
				ClientID:         "my-client",
				ClientSecretAuth: &ClientSecretAuth{},
			},
			wantErr: "ClientSecret is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.creds.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestClientCredentials_ValidateCoversAllAuthFields uses reflection to detect
// when new authentication method fields are added to ClientCredentials without
// updating Validate. If this test fails, update Validate() to handle the new
// field and increment knownAuthMethods.
func TestClientCredentials_ValidateCoversAllAuthFields(t *testing.T) {
	const knownAuthMethods = 1 // ClientSecretAuth

	typ := reflect.TypeOf(ClientCredentials{})
	var pointerFields int
	for i := range typ.NumField() {
		f := typ.Field(i)
		if f.Name == "ClientID" || f.Name == "Issuer" {
			continue
		}
		if f.Type.Kind() != reflect.Ptr {
			t.Errorf("field %q is %v, expected a pointer to an auth method struct", f.Name, f.Type.Kind())
		}
		pointerFields++
	}

	if pointerFields != knownAuthMethods {
		t.Fatalf("ClientCredentials has %d auth method fields but Validate only knows about %d -- update Validate() and this test", pointerFields, knownAuthMethods)
	}
}
