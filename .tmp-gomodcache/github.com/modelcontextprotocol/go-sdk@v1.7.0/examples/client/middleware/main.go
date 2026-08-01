// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var nextProgressToken atomic.Int64

// This middleware function adds a progress token to every outgoing request
// from the client.
func main() {
	c := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil)
	c.AddSendingMiddleware(addProgressToken)
}

func addProgressToken(h mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (result mcp.Result, err error) {
		if rp, ok := req.GetParams().(mcp.RequestParams); ok {
			rp.SetProgressToken(nextProgressToken.Add(1))
		}
		return h(ctx, method, req)
	}
}
