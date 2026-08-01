// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file implements Authorization Server Metadata.
// See https://www.rfc-editor.org/rfc/rfc8414.html.

package oauthex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	internaljson "github.com/modelcontextprotocol/go-sdk/internal/json"
)

// ClientRegistrationMetadata represents the client metadata fields for the DCR POST request (RFC 7591).
//
// Note: URL fields in this struct are validated by validateClientRegistrationURLs
// to prevent XSS attacks. If you add a new URL field, you must also add it to
// that function.
type ClientRegistrationMetadata struct {
	// RedirectURIs is a REQUIRED JSON array of redirection URI strings for use in
	// redirect-based flows (such as the authorization code grant).
	RedirectURIs []string `json:"redirect_uris"`

	// TokenEndpointAuthMethod is an OPTIONAL string indicator of the requested
	// authentication method for the token endpoint.
	// If omitted, the default is "client_secret_basic".
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method,omitempty"`

	// GrantTypes is an OPTIONAL JSON array of OAuth 2.0 grant type strings
	// that the client will restrict itself to using.
	// If omitted, the default is ["authorization_code"].
	GrantTypes []string `json:"grant_types,omitempty"`

	// ResponseTypes is an OPTIONAL JSON array of OAuth 2.0 response type strings
	// that the client will restrict itself to using.
	// If omitted, the default is ["code"].
	ResponseTypes []string `json:"response_types,omitempty"`

	// ClientName is a RECOMMENDED human-readable name of the client to be presented
	// to the end-user.
	ClientName string `json:"client_name,omitempty"`

	// ClientURI is a RECOMMENDED URL of a web page providing information about the client.
	ClientURI string `json:"client_uri,omitempty"`

	// LogoURI is an OPTIONAL URL of a logo for the client, which may be displayed
	// to the end-user.
	LogoURI string `json:"logo_uri,omitempty"`

	// Scope is an OPTIONAL string containing a space-separated list of scope values
	// that the client will restrict itself to using.
	Scope string `json:"scope,omitempty"`

	// Contacts is an OPTIONAL JSON array of strings representing ways to contact
	// people responsible for this client (e.g., email addresses).
	Contacts []string `json:"contacts,omitempty"`

	// TOSURI is an OPTIONAL URL that the client provides to the end-user
	// to read about the client's terms of service.
	TOSURI string `json:"tos_uri,omitempty"`

	// PolicyURI is an OPTIONAL URL that the client provides to the end-user
	// to read about the client's privacy policy.
	PolicyURI string `json:"policy_uri,omitempty"`

	// JWKSURI is an OPTIONAL URL for the client's JSON Web Key Set [JWK] document.
	// This is preferred over the 'jwks' parameter.
	JWKSURI string `json:"jwks_uri,omitempty"`

	// JWKS is an OPTIONAL client's JSON Web Key Set [JWK] document, passed by value.
	// This is an alternative to providing a JWKSURI.
	JWKS string `json:"jwks,omitempty"`

	// SoftwareID is an OPTIONAL unique identifier string for the client software,
	// constant across all instances and versions.
	SoftwareID string `json:"software_id,omitempty"`

	// SoftwareVersion is an OPTIONAL version identifier string for the client software.
	SoftwareVersion string `json:"software_version,omitempty"`

	// SoftwareStatement is an OPTIONAL JWT that asserts client metadata values.
	// Values in the software statement take precedence over other metadata values.
	SoftwareStatement string `json:"software_statement,omitempty"`

	// ApplicationType is an OPTIONAL string that indicates the type of application.
	// Valid values are "native" and "web".
	// If omitted, OIDC-compliant authorization servers default to "web".
	ApplicationType string `json:"application_type,omitempty"`
}

// ClientRegistrationResponse represents the fields returned by the Authorization Server
// (RFC 7591, Section 3.2.1 and 3.2.2).
type ClientRegistrationResponse struct {
	// ClientRegistrationMetadata contains all registered client metadata, returned by the
	// server on success, potentially with modified or defaulted values.
	ClientRegistrationMetadata

	// ClientID is the REQUIRED newly issued OAuth 2.0 client identifier.
	ClientID string `json:"client_id"`

	// ClientSecret is an OPTIONAL client secret string.
	ClientSecret string `json:"client_secret,omitempty"`

	// ClientIDIssuedAt is an OPTIONAL Unix timestamp when the ClientID was issued.
	ClientIDIssuedAt time.Time `json:"client_id_issued_at,omitempty"`

	// ClientSecretExpiresAt is the REQUIRED (if client_secret is issued) Unix
	// timestamp when the secret expires, or 0 if it never expires.
	ClientSecretExpiresAt time.Time `json:"client_secret_expires_at,omitempty"`
}

func (r *ClientRegistrationResponse) MarshalJSON() ([]byte, error) {
	type alias ClientRegistrationResponse
	var clientIDIssuedAt int64
	var clientSecretExpiresAt int64

	if !r.ClientIDIssuedAt.IsZero() {
		clientIDIssuedAt = r.ClientIDIssuedAt.Unix()
	}
	if !r.ClientSecretExpiresAt.IsZero() {
		clientSecretExpiresAt = r.ClientSecretExpiresAt.Unix()
	}

	return json.Marshal(&struct {
		ClientIDIssuedAt      int64 `json:"client_id_issued_at,omitempty"`
		ClientSecretExpiresAt int64 `json:"client_secret_expires_at,omitempty"`
		*alias
	}{
		ClientIDIssuedAt:      clientIDIssuedAt,
		ClientSecretExpiresAt: clientSecretExpiresAt,
		alias:                 (*alias)(r),
	})
}

func (r *ClientRegistrationResponse) UnmarshalJSON(data []byte) error {
	type alias ClientRegistrationResponse
	aux := &struct {
		ClientIDIssuedAt      int64 `json:"client_id_issued_at,omitempty"`
		ClientSecretExpiresAt int64 `json:"client_secret_expires_at,omitempty"`
		*alias
	}{
		alias: (*alias)(r),
	}
	if err := internaljson.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ClientIDIssuedAt != 0 {
		r.ClientIDIssuedAt = time.Unix(aux.ClientIDIssuedAt, 0)
	}
	if aux.ClientSecretExpiresAt != 0 {
		r.ClientSecretExpiresAt = time.Unix(aux.ClientSecretExpiresAt, 0)
	}
	return nil
}

// ClientRegistrationError is the error response from the Authorization Server
// for a failed registration attempt (RFC 7591, Section 3.2.2).
type ClientRegistrationError struct {
	// ErrorCode is the REQUIRED error code if registration failed (RFC 7591, 3.2.2).
	ErrorCode string `json:"error"`

	// ErrorDescription is an OPTIONAL human-readable error message.
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e *ClientRegistrationError) Error() string {
	return fmt.Sprintf("registration failed: %s (%s)", e.ErrorCode, e.ErrorDescription)
}

// RegisterClient performs Dynamic Client Registration according to RFC 7591.
func RegisterClient(ctx context.Context, registrationEndpoint string, clientMeta *ClientRegistrationMetadata, c *http.Client) (*ClientRegistrationResponse, error) {
	if registrationEndpoint == "" {
		return nil, fmt.Errorf("registration_endpoint is required")
	}

	if c == nil {
		c = http.DefaultClient
	}

	payload, err := json.Marshal(clientMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal client metadata: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", registrationEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read registration response body: %w", err)
	}

	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		var regResponse ClientRegistrationResponse
		if err := internaljson.Unmarshal(body, &regResponse); err != nil {
			return nil, fmt.Errorf("failed to decode successful registration response: %w (%s)", err, string(body))
		}
		if regResponse.ClientID == "" {
			return nil, fmt.Errorf("registration response is missing required 'client_id' field")
		}
		// Validate URL fields to prevent XSS attacks (see #526).
		if err := validateClientRegistrationURLs(&regResponse.ClientRegistrationMetadata); err != nil {
			return nil, err
		}
		return &regResponse, nil
	}

	if resp.StatusCode == http.StatusBadRequest {
		var regError ClientRegistrationError
		if err := internaljson.Unmarshal(body, &regError); err != nil {
			return nil, fmt.Errorf("failed to decode registration error response: %w (%s)", err, string(body))
		}
		return nil, &regError
	}

	return nil, fmt.Errorf("registration failed with status %s: %s", resp.Status, string(body))
}

// validateClientRegistrationURLs validates all URL fields in ClientRegistrationMetadata
// to ensure they don't use dangerous schemes that could enable XSS attacks.
func validateClientRegistrationURLs(meta *ClientRegistrationMetadata) error {
	// Validate redirect URIs
	for i, uri := range meta.RedirectURIs {
		if err := checkURLScheme(uri); err != nil {
			return fmt.Errorf("redirect_uris[%d]: %w", i, err)
		}
	}

	// Validate other URL fields
	urls := []struct {
		name  string
		value string
	}{
		{"client_uri", meta.ClientURI},
		{"logo_uri", meta.LogoURI},
		{"tos_uri", meta.TOSURI},
		{"policy_uri", meta.PolicyURI},
		{"jwks_uri", meta.JWKSURI},
	}

	for _, u := range urls {
		if err := checkURLScheme(u.value); err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
	}
	return nil
}
