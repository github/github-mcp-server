// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
)

// OAuthHandler is an interface for handling OAuth flows.
//
// If a transport wishes to support OAuth 2 authorization, it should support
// being configured with an OAuthHandler. It should call the handler's
// TokenSource method whenever it sends an HTTP request to set the
// Authorization header. If a request fails with a 401 or 403, it should call
// Authorize, and if that returns nil, it should retry the request. It should
// not call Authorize after the second failure. See
// [github.com/modelcontextprotocol/go-sdk/mcp.StreamableClientTransport]
// for an example.
type OAuthHandler interface {
	// TokenSource returns a token source to be used for outgoing requests.
	// Returned token source might be nil. In that case, the transport will not
	// add any authorization headers to the request.
	TokenSource(context.Context) (oauth2.TokenSource, error)

	// Authorize is called when an HTTP request results in an error that may
	// be addressed by the authorization flow (currently 401 Unauthorized and 403 Forbidden).
	// It is responsible for performing the OAuth flow to obtain an access token.
	// The arguments are the request that failed and the response that was received for it.
	// The headers of the request are available, but the body will have already been consumed
	// when Authorize is called.
	// If the returned error is nil, TokenSource is expected to return a non-nil token source.
	// After a successful call to Authorize, the HTTP request will be retried by the transport.
	// The function is responsible for closing the response body.
	Authorize(context.Context, *http.Request, *http.Response) error
}
