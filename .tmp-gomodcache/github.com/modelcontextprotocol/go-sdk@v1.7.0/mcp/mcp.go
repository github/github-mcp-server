// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The mcp package provides an SDK for writing model context protocol clients
// and servers.
//
// To get started, create either a [Client] or [Server], add features to it
// using `AddXXX` functions, and connect it to a peer using a [Transport].
//
// For example, to run a simple server on the [StdioTransport]:
//
//	server := mcp.NewServer(&mcp.Implementation{Name: "greeter"}, nil)
//
//	// Using the generic AddTool automatically populates the the input and output
//	// schema of the tool.
//	type args struct {
//		Name string `json:"name" jsonschema:"the person to greet"`
//	}
//	mcp.AddTool(server, &mcp.Tool{
//		Name:        "greet",
//		Description: "say hi",
//	}, func(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
//		return &mcp.CallToolResult{
//			Content: []mcp.Content{
//				&mcp.TextContent{Text: "Hi " + args.Name},
//			},
//		}, nil, nil
//	})
//
//	// Run the server on the stdio transport.
//	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
//		log.Printf("Server failed: %v", err)
//	}
//
// To connect to this server, use the [CommandTransport]:
//
//	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
//	transport := &mcp.CommandTransport{Command: exec.Command("myserver")}
//	session, err := client.Connect(ctx, transport, nil)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer session.Close()
//
//	params := &mcp.CallToolParams{
//		Name:      "greet",
//		Arguments: map[string]any{"name": "you"},
//	}
//	res, err := session.CallTool(ctx, params)
//	if err != nil {
//		log.Fatalf("CallTool failed: %v", err)
//	}
//
// # Clients, servers, and sessions
//
// In this SDK, both a [Client] and [Server] may handle many concurrent
// connections. Each time a client or server is connected to a peer using a
// [Transport], it creates a new session (either a [ClientSession] or
// [ServerSession]):
//
//	Client                                                   Server
//	 ⇅                          (jsonrpc2)                     ⇅
//	ClientSession ⇄ Client Transport ⇄ Server Transport ⇄ ServerSession
//
// The session types expose an API to interact with its peer. For example,
// [ClientSession.CallTool] or [ServerSession.ListRoots].
//
// # Adding features
//
// Add MCP servers to your Client or Server using AddXXX methods (for example
// [Client.AddRoot] or [Server.AddPrompt]). If any peers are connected when
// AddXXX is called, they will receive a corresponding change notification
// (for example notifications/roots/list_changed).
//
// Adding tools is special: tools may be bound to ordinary Go functions by
// using the top-level generic [AddTool] function, which allows specifying an
// input and output type. When AddTool is used, the tool's input schema and
// output schema are automatically populated, and inputs are automatically
// validated. As a special case, if the output type is 'any', no output schema
// is generated.
//
//	func double(_ context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error) {
//		return nil, Out{Answer: 2*in.Number}, nil
//	}
//	...
//	mcp.AddTool(server, &mcp.Tool{Name: "double"}, double)
package mcp
