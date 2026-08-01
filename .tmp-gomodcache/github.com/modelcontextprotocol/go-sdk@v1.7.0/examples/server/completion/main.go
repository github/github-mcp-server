// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This example demonstrates the minimal code to declare and assign
// a CompletionHandler to an MCP Server's options.
func main() {
	// Define your custom CompletionHandler logic.
	// !+completionhandler
	myCompletionHandler := func(_ context.Context, req *mcp.CompleteRequest) (*mcp.CompleteResult, error) {
		// In a real application, you'd implement actual completion logic here.
		// For this example, we return a fixed set of suggestions.
		var suggestions []string
		switch req.Params.Ref.Type {
		case "ref/prompt":
			suggestions = []string{"suggestion1", "suggestion2", "suggestion3"}
		case "ref/resource":
			suggestions = []string{"suggestion4", "suggestion5", "suggestion6"}
		default:
			return nil, fmt.Errorf("unrecognized content type %s", req.Params.Ref.Type)
		}

		return &mcp.CompleteResult{
			Completion: mcp.CompletionResultDetails{
				HasMore: false,
				Total:   len(suggestions),
				Values:  suggestions,
			},
		}, nil
	}

	// Create the MCP Server instance and assign the handler.
	// No server running, just showing the configuration.
	_ = mcp.NewServer(&mcp.Implementation{Name: "server"}, &mcp.ServerOptions{
		CompletionHandler: myCompletionHandler,
	})
	// !-completionhandler

	log.Println("MCP Server instance created with a CompletionHandler assigned (but not running).")
	log.Println("This example demonstrates configuration, not live interaction.")
}
