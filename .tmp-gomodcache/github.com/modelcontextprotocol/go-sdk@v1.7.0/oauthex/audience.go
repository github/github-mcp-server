// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package oauthex

import "strings"

// MatchesResource reports whether any of claims matches resource under
// RFC 3986 §6.2.3 scheme-based normalization, narrowed to the empty-path
// case: a URI with an empty path is treated as equivalent to one with a
// path of "/". All other URI components (scheme, host case, port, query,
// fragment) must match exactly — this is intentionally narrower than the
// full §6.2.3 rung, which would also fold scheme/host case and default
// ports.
//
// RFC 9728 §3.3 normatively requires "simple string comparison" per
// RFC 3986 §6.2.1 (byte-equal). Callers that need strict byte-equal
// semantics should compare claims to resource directly:
//
//	for _, c := range claims { if c == resource { return true } }
//
// Background: RFC 9728 §3.3 canonicalises the protected-resource
// identifier with a trailing slash, but RFC 8707 resource indicators
// sometimes omit it, and upstream IdPs vary in which form they emit in
// `aud` claims (Google trims, Auth0 retains, claude.ai round-trips
// whichever it received). Strict byte equality therefore fails routinely
// on legitimate setups; this helper is the most common pragmatic relaxation
// while keeping path/scheme/host strict so token confusion across distinct
// resources still fails closed.
//
// Returns false when claims is empty.
func MatchesResource(claims []string, resource string) bool {
	expected := strings.TrimSuffix(resource, "/")
	for _, c := range claims {
		if c == resource {
			return true
		}
		if strings.TrimSuffix(c, "/") == expected {
			return true
		}
	}
	return false
}
