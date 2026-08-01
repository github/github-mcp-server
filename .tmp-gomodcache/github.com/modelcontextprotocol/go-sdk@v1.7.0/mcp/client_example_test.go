// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"context"
	"fmt"
	"log"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// !+roots

func Example_roots() {
	ctx := context.Background()

	// Create a client with two roots.
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	c.AddRoots(&mcp.Root{URI: "file://a"}, &mcp.Root{URI: "file://b"})

	// Create a server with a tool that requests roots via the multi round-trip
	// pattern (SEP-2322): server-to-client requests are no longer sent as
	// standalone JSON-RPC calls on protocol version >= 2026-07-28.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "roots"}, func(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if len(req.Params.InputResponses) == 0 {
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{"roots": &mcp.ListRootsParams{}},
			}, nil, nil
		}
		rootList := req.Params.InputResponses["roots"].(*mcp.ListRootsResult)
		var roots []string
		for _, root := range rootList.Roots {
			roots = append(roots, root.URI)
		}
		fmt.Println(roots)
		return &mcp.CallToolResult{}, nil, nil
	})

	// Connect the server and client...
	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := s.Connect(ctx, t1, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer serverSession.Close()

	clientSession, err := c.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer clientSession.Close()

	// ...and call the tool. The client's multi round-trip driver fulfils the
	// embedded roots/list request and retries the call.
	if _, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "roots"}); err != nil {
		log.Fatal(err)
	}
	// Output: [file://a file://b]
}

// !-roots

// !+sampling

func Example_sampling() {
	ctx := context.Background()

	// Create a client with a sampling handler.
	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, &mcp.ClientOptions{
		CreateMessageHandler: func(_ context.Context, req *mcp.CreateMessageRequest) (*mcp.CreateMessageResult, error) {
			return &mcp.CreateMessageResult{
				Content: &mcp.TextContent{
					Text: "would have created a message",
				},
			}, nil
		},
	})

	// Connect the server and client...
	ct, st := mcp.NewInMemoryTransports()
	// Create a server with a tool that requests sampling via the multi
	// round-trip pattern (SEP-2322): server-to-client requests are no longer
	// sent as standalone JSON-RPC calls on protocol version >= 2026-07-28.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "sample"}, func(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if len(req.Params.InputResponses) == 0 {
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{"msg": &mcp.CreateMessageParams{}},
			}, nil, nil
		}
		msg := req.Params.InputResponses["msg"].(*mcp.CreateMessageWithToolsResult)
		return &mcp.CallToolResult{Content: msg.Content}, nil, nil
	})
	session, err := s.Connect(ctx, st, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	clientSession, err := c.Connect(ctx, ct, nil)
	if err != nil {
		log.Fatal(err)
	}

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "sample"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Content[0].(*mcp.TextContent).Text)
	// Output: would have created a message
}

// !-sampling

// !+elicitation

func Example_elicitation() {
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()

	// Create a server with a tool that requests elicitation via the multi
	// round-trip pattern (SEP-2322): server-to-client requests are no longer
	// sent as standalone JSON-RPC calls on protocol version >= 2026-07-28.
	s := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "ask"}, func(_ context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		if len(req.Params.InputResponses) == 0 {
			return &mcp.CallToolResult{
				InputRequests: mcp.InputRequestMap{"input": &mcp.ElicitParams{
					Message: "This should fail",
					RequestedSchema: &jsonschema.Schema{
						Type: "object",
						Properties: map[string]*jsonschema.Schema{
							"test": {Type: "string"},
						},
					},
				}},
			}, nil, nil
		}
		res := req.Params.InputResponses["input"].(*mcp.ElicitResult)
		fmt.Println(res.Content["test"])
		return &mcp.CallToolResult{}, nil, nil
	})
	ss, err := s.Connect(ctx, st, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer ss.Close()

	c := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ElicitationHandler: func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			return &mcp.ElicitResult{Action: "accept", Content: map[string]any{"test": "value"}}, nil
		},
	})
	clientSession, err := c.Connect(ctx, ct, nil)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: "ask"}); err != nil {
		log.Fatal(err)
	}
	// Output: value
}

// !-elicitation
