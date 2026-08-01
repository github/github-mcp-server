// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The listfeatures command lists all features of a stdio MCP server.
//
// Usage: listfeatures <command> [<args>]
//
// For example:
//
//	listfeatures go run github.com/modelcontextprotocol/go-sdk/examples/server/hello
//
// or
//
//	listfeatures npx @modelcontextprotocol/server-everything
package main

import (
	"context"
	"flag"
	"fmt"
	"iter"
	"log"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	endpoint = flag.String("http", "", "if set, connect to this streamable endpoint rather than running a stdio server")
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 && *endpoint == "" {
		fmt.Fprintln(os.Stderr, "Usage: listfeatures <command> [<args>]")
		fmt.Fprintln(os.Stderr, "Usage: listfeatures --http=\"https://example.com/server/mcp\"")
		fmt.Fprintln(os.Stderr, "List all features for a stdio MCP server")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Example:\n\tlistfeatures go run github.com/modelcontextprotocol/go-sdk/examples/server/hello")
		os.Exit(2)
	}

	var (
		ctx       = context.Background()
		transport mcp.Transport
	)
	if *endpoint != "" {
		transport = &mcp.StreamableClientTransport{
			Endpoint: *endpoint,
		}
	} else {
		cmd := exec.Command(args[0], args[1:]...)
		transport = &mcp.CommandTransport{Command: cmd}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "mcp-client", Version: "v1.0.0"}, nil)
	cs, err := client.Connect(ctx, transport, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer cs.Close()

	if cs.InitializeResult().Capabilities.Tools != nil {
		printSection("tools", cs.Tools(ctx, nil), func(t *mcp.Tool) string { return t.Name })
	}
	if cs.InitializeResult().Capabilities.Resources != nil {
		printSection("resources", cs.Resources(ctx, nil), func(r *mcp.Resource) string { return r.Name })
		printSection("resource templates", cs.ResourceTemplates(ctx, nil), func(r *mcp.ResourceTemplate) string { return r.Name })
	}
	if cs.InitializeResult().Capabilities.Prompts != nil {
		printSection("prompts", cs.Prompts(ctx, nil), func(p *mcp.Prompt) string { return p.Name })
	}
}

func printSection[T any](name string, features iter.Seq2[T, error], featName func(T) string) {
	fmt.Printf("%s:\n", name)
	for feat, err := range features {
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("\t%s\n", featName(feat))
	}
	fmt.Println()
}
