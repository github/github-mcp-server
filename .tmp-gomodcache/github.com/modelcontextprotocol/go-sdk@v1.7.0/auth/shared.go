// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// This file contains shared utilities for OAuth handlers.

package auth

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// GetAuthServerMetadata fetches authorization server metadata for the given issuer URL.
// It tries standard well-known endpoints (OAuth 2.0 and OIDC) and returns the first successful result.
//
// Returns (nil, nil) when no metadata endpoints respond (404s), allowing callers to implement
// fallback logic. Returns an error for any non-client error (network failures, invalid JSON, etc.).
func GetAuthServerMetadata(ctx context.Context, issuerURL string, httpClient *http.Client) (*oauthex.AuthServerMeta, error) {
	for _, metadataURL := range authorizationServerMetadataURLs(issuerURL) {
		asm, err := oauthex.GetAuthServerMeta(ctx, metadataURL, issuerURL, httpClient)
		if err != nil {
			return nil, err
		}
		if asm != nil {
			return asm, nil
		}
	}
	return nil, nil
}

// authorizationServerMetadataURLs returns a list of URLs to try when looking for
// authorization server metadata as mandated by the MCP specification:
// https://modelcontextprotocol.io/specification/2025-11-25/basic/authorization#authorization-server-metadata-discovery.
func authorizationServerMetadataURLs(issuerURL string) []string {
	var urls []string

	baseURL, err := url.Parse(issuerURL)
	if err != nil {
		return nil
	}

	if baseURL.Path == "" {
		// "OAuth 2.0 Authorization Server Metadata".
		baseURL.Path = "/.well-known/oauth-authorization-server"
		urls = append(urls, baseURL.String())
		// "OpenID Connect Discovery 1.0".
		baseURL.Path = "/.well-known/openid-configuration"
		urls = append(urls, baseURL.String())
		return urls
	}

	originalPath := baseURL.Path
	// "OAuth 2.0 Authorization Server Metadata with path insertion".
	baseURL.Path = "/.well-known/oauth-authorization-server/" + strings.TrimLeft(originalPath, "/")
	urls = append(urls, baseURL.String())
	// "OpenID Connect Discovery 1.0 with path insertion".
	baseURL.Path = "/.well-known/openid-configuration/" + strings.TrimLeft(originalPath, "/")
	urls = append(urls, baseURL.String())
	// "OpenID Connect Discovery 1.0 with path appending".
	baseURL.Path = "/" + strings.Trim(originalPath, "/") + "/.well-known/openid-configuration"
	urls = append(urls, baseURL.String())

	return urls
}
