// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file implements Authorization Server Metadata.
// See https://www.rfc-editor.org/rfc/rfc8414.html.

package oauthex

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/internal/authutil"
)

// AuthServerMeta represents the metadata for an OAuth 2.0 authorization server,
// as defined in [RFC 8414].
//
// Not supported:
// - signed metadata
//
// Note: URL fields in this struct are validated by validateAuthServerMetaURLs to
// prevent XSS attacks. If you add a new URL field, you must also add it to that
// function.
//
// [RFC 8414]: https://tools.ietf.org/html/rfc8414)
type AuthServerMeta struct {
	// Issuer is the REQUIRED URL identifying the authorization server.
	Issuer string `json:"issuer"`

	// AuthorizationEndpoint is the REQUIRED URL of the server's OAuth 2.0 authorization endpoint.
	AuthorizationEndpoint string `json:"authorization_endpoint"`

	// TokenEndpoint is the REQUIRED URL of the server's OAuth 2.0 token endpoint.
	TokenEndpoint string `json:"token_endpoint"`

	// JWKSURI is the REQUIRED URL of the server's JSON Web Key Set [JWK] document.
	JWKSURI string `json:"jwks_uri"`

	// RegistrationEndpoint is the RECOMMENDED URL of the server's OAuth 2.0 Dynamic Client Registration endpoint.
	RegistrationEndpoint string `json:"registration_endpoint,omitempty"`

	// ScopesSupported is a RECOMMENDED JSON array of strings containing a list of the OAuth 2.0
	// "scope" values that this server supports.
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// ResponseTypesSupported is a REQUIRED JSON array of strings containing a list of the OAuth 2.0
	// "response_type" values that this server supports.
	ResponseTypesSupported []string `json:"response_types_supported"`

	// ResponseModesSupported is a RECOMMENDED JSON array of strings containing a list of the OAuth 2.0
	// "response_mode" values that this server supports.
	ResponseModesSupported []string `json:"response_modes_supported,omitempty"`

	// GrantTypesSupported is a RECOMMENDED JSON array of strings containing a list of the OAuth 2.0
	// grant type values that this server supports.
	GrantTypesSupported []string `json:"grant_types_supported,omitempty"`

	// TokenEndpointAuthMethodsSupported is a RECOMMENDED JSON array of strings containing a list of
	// client authentication methods supported by this token endpoint.
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`

	// TokenEndpointAuthSigningAlgValuesSupported is a RECOMMENDED JSON array of strings containing
	// a list of the JWS signing algorithms ("alg" values) supported by the token endpoint for
	// the signature on the JWT used to authenticate the client.
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported,omitempty"`

	// ServiceDocumentation is a RECOMMENDED URL of a page containing human-readable documentation
	// for the service.
	ServiceDocumentation string `json:"service_documentation,omitempty"`

	// UILocalesSupported is a RECOMMENDED JSON array of strings representing supported
	// BCP47 [RFC5646] language tag values for display in the user interface.
	UILocalesSupported []string `json:"ui_locales_supported,omitempty"`

	// OpPolicyURI is a RECOMMENDED URL that the server provides to the person registering
	// the client to read about the server's operator policies.
	OpPolicyURI string `json:"op_policy_uri,omitempty"`

	// OpTOSURI is a RECOMMENDED URL that the server provides to the person registering the
	// client to read about the server's terms of service.
	OpTOSURI string `json:"op_tos_uri,omitempty"`

	// RevocationEndpoint is a RECOMMENDED URL of the server's OAuth 2.0 revocation endpoint.
	RevocationEndpoint string `json:"revocation_endpoint,omitempty"`

	// RevocationEndpointAuthMethodsSupported is a RECOMMENDED JSON array of strings containing
	// a list of client authentication methods supported by this revocation endpoint.
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported,omitempty"`

	// RevocationEndpointAuthSigningAlgValuesSupported is a RECOMMENDED JSON array of strings
	// containing a list of the JWS signing algorithms ("alg" values) supported by the revocation
	// endpoint for the signature on the JWT used to authenticate the client.
	RevocationEndpointAuthSigningAlgValuesSupported []string `json:"revocation_endpoint_auth_signing_alg_values_supported,omitempty"`

	// IntrospectionEndpoint is a RECOMMENDED URL of the server's OAuth 2.0 introspection endpoint.
	IntrospectionEndpoint string `json:"introspection_endpoint,omitempty"`

	// IntrospectionEndpointAuthMethodsSupported is a RECOMMENDED JSON array of strings containing
	// a list of client authentication methods supported by this introspection endpoint.
	IntrospectionEndpointAuthMethodsSupported []string `json:"introspection_endpoint_auth_methods_supported,omitempty"`

	// IntrospectionEndpointAuthSigningAlgValuesSupported is a RECOMMENDED JSON array of strings
	// containing a list of the JWS signing algorithms ("alg" values) supported by the introspection
	// endpoint for the signature on the JWT used to authenticate the client.
	IntrospectionEndpointAuthSigningAlgValuesSupported []string `json:"introspection_endpoint_auth_signing_alg_values_supported,omitempty"`

	// CodeChallengeMethodsSupported is a RECOMMENDED JSON array of strings containing a list of
	// PKCE code challenge methods supported by this authorization server.
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`

	// ClientIDMetadataDocumentSupported is a boolean indicating whether the authorization server
	// supports client ID metadata documents.
	ClientIDMetadataDocumentSupported bool `json:"client_id_metadata_document_supported,omitempty"`

	// AuthorizationResponseIssParameterSupported indicates whether the authorization server
	// provides the "iss" parameter in authorization responses per [RFC 9207].
	// When true, clients must verify the "iss" parameter is present and matches the Issuer field.
	//
	// [RFC 9207]: https://www.rfc-editor.org/rfc/rfc9207
	AuthorizationResponseIssParameterSupported bool `json:"authorization_response_iss_parameter_supported,omitempty"`
}

// GetAuthServerMeta issues a GET request to retrieve authorization server metadata
// from an OAuth authorization server with the given metadataURL.
//
// It follows [RFC 8414]:
//   - The metadataURL must use HTTPS or be a local address.
//   - The Issuer field is checked against metadataURL.Issuer.
//
// It also verifies that the authorization server supports PKCE and that the URLs
// in the metadata don't use dangerous schemes.
//
// It returns an error if the request fails with a non-4xx status code or the fetched
// metadata doesn't pass security validations.
// It returns nil if the request fails with a 4xx status code.
//
// [RFC 8414]: https://tools.ietf.org/html/rfc8414
func GetAuthServerMeta(ctx context.Context, metadataURL, issuer string, c *http.Client) (*AuthServerMeta, error) {
	// Only allow HTTP for local addresses (testing or development purposes).
	if err := checkHTTPSOrLoopback(metadataURL); err != nil {
		return nil, fmt.Errorf("metadataURL: %v", err)
	}
	asm, err := getJSON[AuthServerMeta](ctx, c, metadataURL, 1<<20)
	if err != nil {
		var httpErr *httpStatusError
		if errors.As(err, &httpErr) {
			if 400 <= httpErr.StatusCode && httpErr.StatusCode < 500 {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("%v", err) // Do not expose error types.
	}
	if !authutil.IssuersEqual(asm.Issuer, issuer) {
		return nil, fmt.Errorf("metadata issuer %q does not match issuer URL %q", asm.Issuer, issuer)
	}

	if len(asm.CodeChallengeMethodsSupported) == 0 {
		return nil, fmt.Errorf("authorization server at %s does not implement PKCE", issuer)
	}

	// Validate endpoint URLs to prevent XSS attacks (see #526).
	if err := validateAuthServerMetaURLs(asm); err != nil {
		return nil, err
	}

	return asm, nil
}

// validateAuthServerMetaURLs validates all URL fields in AuthServerMeta
// to ensure they don't use dangerous schemes that could enable XSS attacks.
// It also validates that URLs likely to be called by the client use
// HTTPS or are loopback addresses.
func validateAuthServerMetaURLs(asm *AuthServerMeta) error {
	urls := []struct {
		name  string
		value string
	}{
		{"authorization_endpoint", asm.AuthorizationEndpoint},
		{"token_endpoint", asm.TokenEndpoint},
		{"jwks_uri", asm.JWKSURI},
		{"registration_endpoint", asm.RegistrationEndpoint},
		{"service_documentation", asm.ServiceDocumentation},
		{"op_policy_uri", asm.OpPolicyURI},
		{"op_tos_uri", asm.OpTOSURI},
		{"revocation_endpoint", asm.RevocationEndpoint},
		{"introspection_endpoint", asm.IntrospectionEndpoint},
	}

	for _, u := range urls {
		if err := checkURLScheme(u.value); err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
	}

	urls = []struct {
		name  string
		value string
	}{
		{"authorization_endpoint", asm.AuthorizationEndpoint},
		{"token_endpoint", asm.TokenEndpoint},
		{"registration_endpoint", asm.RegistrationEndpoint},
		{"introspection_endpoint", asm.IntrospectionEndpoint},
	}

	for _, u := range urls {
		if err := checkHTTPSOrLoopback(u.value); err != nil {
			return fmt.Errorf("%s: %w", u.name, err)
		}
	}

	return nil
}
