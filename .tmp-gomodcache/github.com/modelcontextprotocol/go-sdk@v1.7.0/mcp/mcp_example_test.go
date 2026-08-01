// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// !+lifecycle

func Example_lifecycle() {
	ctx := context.Background()

	// Create a client and server. Under protocol version 2026-07-28, there
	// is no dedicated initialize/initialized handshake: the session is
	// implicitly live the moment the client issues its first request.
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)

	// Connect the server and client using in-memory transports.
	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		log.Fatal(err)
	}
	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Now shut down the session by closing the client, and waiting for the
	// server session to end.
	if err := clientSession.Close(); err != nil {
		log.Fatal(err)
	}
	if err := serverSession.Wait(); err != nil {
		log.Fatal(err)
	}
	// Output:
}

// !-lifecycle

// !+progress

func Example_progress() {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "makeProgress"}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		if token := req.Params.GetProgressToken(); token != nil {
			for i := range 3 {
				params := &mcp.ProgressNotificationParams{
					Message:       "frobbing widgets",
					ProgressToken: token,
					Progress:      float64(i),
					Total:         2,
				}
				req.Session.NotifyProgress(ctx, params) // ignore error
			}
		}
		return &mcp.CallToolResult{}, nil, nil
	})
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, &mcp.ClientOptions{
		ProgressNotificationHandler: func(_ context.Context, req *mcp.ProgressNotificationClientRequest) {
			fmt.Printf("%s %.0f/%.0f\n", req.Params.Message, req.Params.Progress, req.Params.Total)
		},
	})
	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		log.Fatal(err)
	}

	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()
	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "makeProgress",
		Meta: mcp.Meta{"progressToken": "abc123"},
	}); err != nil {
		log.Fatal(err)
	}
	// Output:
	// frobbing widgets 0/2
	// frobbing widgets 1/2
	// frobbing widgets 2/2
}

// !-progress

// !+cancellation

func Example_cancellation() {
	// For this example, we're going to be collecting observations from the
	// server and client.
	var clientResult, serverResult string
	var wg sync.WaitGroup
	wg.Add(2)

	// Create a server with a single slow tool.
	// When the client cancels its request, the server should observe
	// cancellation.
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.0.1"}, nil)
	started := make(chan struct{}, 1) // signals that the server started handling the tool call
	mcp.AddTool(server, &mcp.Tool{Name: "slow"}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		started <- struct{}{}
		defer wg.Done()
		select {
		case <-time.After(5 * time.Second):
			serverResult = "tool done"
		case <-ctx.Done():
			serverResult = "tool canceled"
		}
		return &mcp.CallToolResult{}, nil, nil
	})

	// Connect a client to the server.
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "v0.0.1"}, nil)
	ctx := context.Background()
	t1, t2 := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, t1, nil); err != nil {
		log.Fatal(err)
	}
	session, err := client.Connect(ctx, t2, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	// Make a tool call, asynchronously.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer wg.Done()
		_, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "slow"})
		clientResult = fmt.Sprintf("%v", err)
	}()

	// As soon as the server has started handling the call, cancel it from the
	// client side.
	<-started
	cancel()
	wg.Wait()

	fmt.Println(clientResult)
	fmt.Println(serverResult)
	// Output:
	// context canceled
	// tool canceled
}

// !-cancellation
