// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGreet(t *testing.T) {
	manual, err := newManualGreeter()
	if err != nil {
		t.Fatal(err)
	}
	res, err := manual.greet(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(`{"name": "Bob"}`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError {
		t.Fatalf("tool error: %q", res.Content[0].(*mcp.TextContent).Text)
	}
}
