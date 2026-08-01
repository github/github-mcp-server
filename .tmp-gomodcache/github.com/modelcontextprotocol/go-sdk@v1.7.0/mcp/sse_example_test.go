// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type AddParams struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func Add(ctx context.Context, req *mcp.CallToolRequest, args AddParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("%d", args.X+args.Y)},
		},
	}, nil, nil
}

func ExampleSSEHandler() {
	server := mcp.NewServer(&mcp.Implementation{Name: "adder", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "add", Description: "add two numbers"}, Add)

	handler := mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	ctx := context.Background()
	transport := &mcp.SSEClientTransport{Endpoint: httpServer.URL}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v1.0.0"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add",
		Arguments: map[string]any{"x": 1, "y": 2},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Content[0].(*mcp.TextContent).Text)

	// Output: 3
}
