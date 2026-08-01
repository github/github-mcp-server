// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	host = flag.String("host", "localhost", "host to listen on")
	port = flag.String("port", "8080", "port to listen on")
)

type SayHiParams struct {
	Name string `json:"name"`
}

func SayHi(ctx context.Context, req *mcp.CallToolRequest, args SayHiParams) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: "Hi " + args.Name},
		},
	}, nil, nil
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "This program runs MCP servers over SSE HTTP.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEndpoints:\n")
		fmt.Fprintf(os.Stderr, "  /greeter1 - Greeter 1 service\n")
		fmt.Fprintf(os.Stderr, "  /greeter2 - Greeter 2 service\n")
		os.Exit(1)
	}
	flag.Parse()

	addr := fmt.Sprintf("%s:%s", *host, *port)

	server1 := mcp.NewServer(&mcp.Implementation{Name: "greeter1"}, nil)
	mcp.AddTool(server1, &mcp.Tool{Name: "greet1", Description: "say hi"}, SayHi)

	server2 := mcp.NewServer(&mcp.Implementation{Name: "greeter2"}, nil)
	mcp.AddTool(server2, &mcp.Tool{Name: "greet2", Description: "say hello"}, SayHi)

	log.Printf("MCP servers serving at %s", addr)
	handler := mcp.NewSSEHandler(func(request *http.Request) *mcp.Server {
		url := request.URL.Path
		log.Printf("Handling request for URL %s\n", url)
		switch url {
		case "/greeter1":
			return server1
		case "/greeter2":
			return server2
		default:
			return nil
		}
	}, nil)
	log.Fatal(http.ListenAndServe(addr, handler))
}
