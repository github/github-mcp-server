// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package authutil

import "strings"

// IssuersEqual reports whether two OAuth 2.0 authorization server issuer
// identifiers refer to the same server comparing them without the final trailing slash.
func IssuersEqual(a, b string) bool {
	return strings.TrimSuffix(a, "/") == strings.TrimSuffix(b, "/")
}
