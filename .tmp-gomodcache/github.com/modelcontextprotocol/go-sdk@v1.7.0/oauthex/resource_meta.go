// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file implements Protected Resource Metadata.
// See https://www.rfc-editor.org/rfc/rfc9728.html.

package oauthex

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/internal/util"
)

// ProtectedResourceMetadata is the metadata for an OAuth 2.0 protected resource,
// as defined in section 2 of https://www.rfc-editor.org/rfc/rfc9728.html.
//
// The following features are not supported:
// - additional keys (§2, last sentence)
// - human-readable metadata (§2.1)
// - signed metadata (§2.2)
type ProtectedResourceMetadata struct {
	// Resource (resource) is the protected resource's resource identifier.
	// Required.
	Resource string `json:"resource"`

	// AuthorizationServers (authorization_servers) is an optional slice containing a list of
	// OAuth authorization server issuer identifiers (as defined in RFC 8414) that can be
	// used with this protected resource.
	AuthorizationServers []string `json:"authorization_servers,omitempty"`

	// JWKSURI (jwks_uri) is an optional URL of the protected resource's JSON Web Key (JWK) Set
	// document. This contains public keys belonging to the protected resource, such as
	// signing key(s) that the resource server uses to sign resource responses.
	JWKSURI string `json:"jwks_uri,omitempty"`

	// ScopesSupported (scopes_supported) is a recommended slice containing a list of scope
	// values (as defined in RFC 6749) used in authorization requests to request access
	// to this protected resource.
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// BearerMethodsSupported (bearer_methods_supported) is an optional slice containing
	// a list of the supported methods of sending an OAuth 2.0 bearer token to the
	// protected resource. Defined values are "header", "body", and "query".
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`

	// ResourceSigningAlgValuesSupported (resource_signing_alg_values_supported) is an optional
	// slice of JWS signing algorithms (alg values) supported by the protected
	// resource for signing resource responses.
	ResourceSigningAlgValuesSupported []string `json:"resource_signing_alg_values_supported,omitempty"`

	// ResourceName (resource_name) is a human-readable name of the protected resource
	// intended for display to the end user. It is RECOMMENDED that this field be included.
	// This value may be internationalized.
	ResourceName string `json:"resource_name,omitempty"`

	// ResourceDocumentation (resource_documentation) is an optional URL of a page containing
	// human-readable information for developers using the protected resource.
	// This value may be internationalized.
	ResourceDocumentation string `json:"resource_documentation,omitempty"`

	// ResourcePolicyURI (resource_policy_uri) is an optional URL of a page containing
	// human-readable policy information on how a client can use the data provided.
	// This value may be internationalized.
	ResourcePolicyURI string `json:"resource_policy_uri,omitempty"`

	// ResourceTOSURI (resource_tos_uri) is an optional URL of a page containing the protected
	// resource's human-readable terms of service. This value may be internationalized.
	ResourceTOSURI string `json:"resource_tos_uri,omitempty"`

	// TLSClientCertificateBoundAccessTokens (tls_client_certificate_bound_access_tokens) is an
	// optional boolean indicating support for mutual-TLS client certificate-bound
	// access tokens (RFC 8705). Defaults to false if omitted.
	TLSClientCertificateBoundAccessTokens bool `json:"tls_client_certificate_bound_access_tokens,omitempty"`

	// AuthorizationDetailsTypesSupported (authorization_details_types_supported) is an optional
	// slice of 'type' values supported by the resource server for the
	// 'authorization_details' parameter (RFC 9396).
	AuthorizationDetailsTypesSupported []string `json:"authorization_details_types_supported,omitempty"`

	// DPOPSigningAlgValuesSupported (dpop_signing_alg_values_supported) is an optional
	// slice of JWS signing algorithms supported by the resource server for validating
	// DPoP proof JWTs (RFC 9449).
	DPOPSigningAlgValuesSupported []string `json:"dpop_signing_alg_values_supported,omitempty"`

	// DPOPBoundAccessTokensRequired (dpop_bound_access_tokens_required) is an optional boolean
	// specifying whether the protected resource always requires the use of DPoP-bound
	// access tokens (RFC 9449). Defaults to false if omitted.
	DPOPBoundAccessTokensRequired bool `json:"dpop_bound_access_tokens_required,omitempty"`

	// SignedMetadata (signed_metadata) is an optional JWT containing metadata parameters
	// about the protected resource as claims. If present, these values take precedence
	// over values conveyed in plain JSON.
	// TODO:implement.
	// Note that §2.2 says it's okay to ignore this.
	// SignedMetadata string `json:"signed_metadata,omitempty"`
}

// Challenge represents a single authentication challenge from a WWW-Authenticate header.
// As per RFC 9110, Section 11.6.1, a challenge consists of a scheme and optional parameters.
type Challenge struct {
	// Scheme is the authentication scheme (e.g., "Bearer", "Basic").
	// It is case-insensitive. A parsed value will always be lower-case.
	Scheme string
	// Params is a map of authentication parameters.
	// Keys are case-insensitive. Parsed keys are always lower-case.
	Params map[string]string
}

// GetProtectedResourceMetadata issues a GET request to retrieve protected resource
// metadata from a resource server.
// The metadataURL is typically a URL with a host:port and possibly a path.
// The resourceURL is the resource URI the metadataURL is for.
// The following checks are performed:
//   - The metadataURL must use HTTPS or be a local address.
//   - The resource field of the resulting metadata must match the resourceURL.
//   - The authorization_servers field of the resulting metadata is checked for dangerous URL schemes.
func GetProtectedResourceMetadata(ctx context.Context, metadataURL, resourceURL string, c *http.Client) (_ *ProtectedResourceMetadata, err error) {
	defer util.Wrapf(&err, "GetProtectedResourceMetadata(%q)", metadataURL)
	// Only allow HTTP for local addresses (testing or development purposes).
	if err := checkHTTPSOrLoopback(metadataURL); err != nil {
		return nil, fmt.Errorf("metadataURL: %v", err)
	}
	prm, err := getJSON[ProtectedResourceMetadata](ctx, c, metadataURL, 1<<20)
	if err != nil {
		return nil, err
	}
	// Validate the Resource field (see RFC 9728, section 3.3).
	if prm.Resource != resourceURL {
		return nil, fmt.Errorf("got metadata resource %q, want %q", prm.Resource, resourceURL)
	}
	// Validate the authorization server URLs to prevent XSS attacks (see #526).
	for i, u := range prm.AuthorizationServers {
		if err := checkURLScheme(u); err != nil {
			return nil, fmt.Errorf("authorization_servers[%d]: %v", i, err)
		}
		if err := checkHTTPSOrLoopback(u); err != nil {
			return nil, fmt.Errorf("authorization_servers[%d]: %v", i, err)
		}
	}
	return prm, nil
}

// ParseWWWAuthenticate parses a WWW-Authenticate header string.
// The header format is defined in RFC 9110, Section 11.6.1, and can contain
// one or more challenges, separated by commas.
// It returns a slice of challenges or an error if one of the headers is malformed.
func ParseWWWAuthenticate(headers []string) ([]Challenge, error) {
	var challenges []Challenge
	for _, h := range headers {
		challengeStrings, err := splitChallenges(h)
		if err != nil {
			return nil, err
		}
		for _, cs := range challengeStrings {
			if strings.TrimSpace(cs) == "" {
				continue
			}
			challenge, err := parseSingleChallenge(cs)
			if err != nil {
				return nil, fmt.Errorf("failed to parse challenge %q: %w", cs, err)
			}
			challenges = append(challenges, challenge)
		}
	}
	return challenges, nil
}

// splitChallenges splits a header value containing one or more challenges.
// It correctly handles commas within quoted strings and distinguishes between
// commas separating auth-params and commas separating challenges.
func splitChallenges(header string) ([]string, error) {
	var challenges []string
	inQuotes := false
	start := 0
	for i, r := range header {
		if r == '"' {
			if i > 0 && header[i-1] != '\\' {
				inQuotes = !inQuotes
			} else if i == 0 {
				// A challenge begins with an auth-scheme, which is a token, which cannot contain
				// a quote.
				return nil, errors.New(`challenge begins with '"'`)
			}
		} else if r == ',' && !inQuotes {
			// This is a potential challenge separator.
			// A new challenge does not start with `key=value`.
			// We check if the part after the comma looks like a parameter.
			lookahead := strings.TrimSpace(header[i+1:])
			eqPos := strings.Index(lookahead, "=")

			isParam := false
			if eqPos > 0 {
				// Check if the part before '=' is a single token (no spaces).
				token := lookahead[:eqPos]
				if strings.IndexFunc(token, unicode.IsSpace) == -1 {
					isParam = true
				}
			}

			if !isParam {
				// The part after the comma does not look like a parameter,
				// so this comma separates challenges.
				challenges = append(challenges, header[start:i])
				start = i + 1
			}
		}
	}
	// Add the last (or only) challenge to the list.
	challenges = append(challenges, header[start:])
	return challenges, nil
}

// parseSingleChallenge parses a string containing exactly one challenge.
// challenge   = auth-scheme [ 1*SP ( token68 / #auth-param ) ]
func parseSingleChallenge(s string) (Challenge, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Challenge{}, errors.New("empty challenge string")
	}

	scheme, paramsStr, found := strings.Cut(s, " ")
	c := Challenge{Scheme: strings.ToLower(scheme)}
	if !found {
		return c, nil
	}

	params := make(map[string]string)

	// Parse the key-value parameters.
	for paramsStr != "" {
		// Find the end of the parameter key.
		keyEnd := strings.Index(paramsStr, "=")
		if keyEnd <= 0 {
			return Challenge{}, fmt.Errorf("malformed auth parameter: expected key=value, but got %q", paramsStr)
		}
		key := strings.TrimSpace(paramsStr[:keyEnd])

		// Move the string past the key and the '='.
		paramsStr = strings.TrimSpace(paramsStr[keyEnd+1:])

		var value string
		if strings.HasPrefix(paramsStr, "\"") {
			// The value is a quoted string.
			paramsStr = paramsStr[1:] // Consume the opening quote.
			var valBuilder strings.Builder
			i := 0
			for ; i < len(paramsStr); i++ {
				// Handle escaped characters.
				if paramsStr[i] == '\\' && i+1 < len(paramsStr) {
					valBuilder.WriteByte(paramsStr[i+1])
					i++ // We've consumed two characters.
				} else if paramsStr[i] == '"' {
					// End of the quoted string.
					break
				} else {
					valBuilder.WriteByte(paramsStr[i])
				}
			}

			// A quoted string must be terminated.
			if i == len(paramsStr) {
				return Challenge{}, fmt.Errorf("unterminated quoted string in auth parameter")
			}

			value = valBuilder.String()
			// Move the string past the value and the closing quote.
			paramsStr = strings.TrimSpace(paramsStr[i+1:])
		} else {
			// The value is a token. It ends at the next comma or the end of the string.
			commaPos := strings.Index(paramsStr, ",")
			if commaPos == -1 {
				value = paramsStr
				paramsStr = ""
			} else {
				value = strings.TrimSpace(paramsStr[:commaPos])
				paramsStr = strings.TrimSpace(paramsStr[commaPos:]) // Keep comma for next check
			}
		}
		if value == "" {
			return Challenge{}, fmt.Errorf("no value for auth param %q", key)
		}

		// Per RFC 9110, parameter keys are case-insensitive.
		params[strings.ToLower(key)] = value

		// If there is a comma, consume it and continue to the next parameter.
		if strings.HasPrefix(paramsStr, ",") {
			paramsStr = strings.TrimSpace(paramsStr[1:])
		} else if paramsStr != "" {
			// If there's content but it's not a new parameter, the format is wrong.
			return Challenge{}, fmt.Errorf("malformed auth parameter: expected comma after value, but got %q", paramsStr)
		}
	}

	// Per RFC 9110, the scheme is case-insensitive.
	return Challenge{Scheme: strings.ToLower(scheme), Params: params}, nil
}
