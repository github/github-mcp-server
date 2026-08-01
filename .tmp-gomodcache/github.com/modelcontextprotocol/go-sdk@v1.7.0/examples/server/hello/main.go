// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The hello server contains a single tool that says hi to the user.
//
// It runs over the stdio transport.
package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	// Create a server with a single tool that says "Hi".
	server := mcp.NewServer(&mcp.Implementation{Name: "greeter"}, nil)

	// Using the generic AddTool automatically populates the the input and output
	// schema of the tool.
	//
	// The schema considers 'json' and 'jsonschema' struct tags to get argument
	// names and descriptions.
	type args struct {
		Name string `json:"name" jsonschema:"the person to greet"`
	}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "say hi",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Hi " + args.Name},
			},
		}, nil, nil
	})

	// server.Run runs the server on the given transport.
	//
	// In this case, the server communicates over stdin/stdout.
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Printf("Server failed: %v", err)
	}
}
