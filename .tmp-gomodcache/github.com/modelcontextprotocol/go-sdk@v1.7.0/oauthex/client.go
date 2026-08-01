// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import "errors"

// ClientCredentials holds client authentication credentials for OAuth token requests.
// It supports multiple authentication methods, but only one method should be set at a time.
// Use the Validate method to ensure proper configuration.
type ClientCredentials struct {
	// ClientID is the OAuth2 client identifier.
	// REQUIRED for all authentication methods.
	ClientID string

	// ClientSecretAuth configures client authentication using a client secret.
	// This is the most common authentication method for confidential clients.
	// OPTIONAL. If not provided, the client is treated as a public client.
	ClientSecretAuth *ClientSecretAuth

	// Issuer is the issuer identifier of the authorization server these
	// credentials are registered with. Pre-registered credentials are bound
	// to a specific authorization server; when set, an error is returned if
	// the discovered authorization server does not match, per SEP-2352.
	// The comparison ignores a single trailing slash, matching the
	// tolerance applied during RFC 8414 Section 3.3 metadata validation.
	// OPTIONAL.
	Issuer string
}

// ClientSecretAuth holds client secret authentication credentials.
// This authentication method supports both "client_secret_basic" and "client_secret_post"
// methods as defined in RFC 6749 Section 2.3.1.
type ClientSecretAuth struct {
	// ClientSecret is the OAuth2 client secret for confidential clients.
	// REQUIRED when using ClientSecretAuth.
	ClientSecret string
}

// Validate checks that the ClientCredentials are properly configured.
// It ensures that:
//   - ClientID is not empty.
//   - At most one authentication method is configured.
//   - If ClientSecretAuth is set, ClientSecret is not empty.
func (c *ClientCredentials) Validate() error {
	if c.ClientID == "" {
		return errors.New("ClientID is required")
	}

	// Count how many auth methods are configured.
	authMethodCount := 0
	if c.ClientSecretAuth != nil {
		authMethodCount++
		if c.ClientSecretAuth.ClientSecret == "" {
			return errors.New("ClientSecret is required when using ClientSecretAuth")
		}
	}

	// Allow zero auth methods (public client) or exactly one auth method.
	if authMethodCount > 1 {
		return errors.New("only one client authentication method can be configured")
	}

	return nil
}
