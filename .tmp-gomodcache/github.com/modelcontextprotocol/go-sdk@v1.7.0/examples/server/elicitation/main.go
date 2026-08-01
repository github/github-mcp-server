// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	ctx := context.Background()
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	// Create server
	server := mcp.NewServer(&mcp.Implementation{Name: "config-server", Version: "v1.0.0"}, nil)

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Create client with elicitation handler
	// Note: Never use elicitation for sensitive data like API keys or passwords
	client := mcp.NewClient(&mcp.Implementation{Name: "config-client", Version: "v1.0.0"}, &mcp.ClientOptions{
		ElicitationHandler: func(ctx context.Context, request *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
			fmt.Printf("Server requests: %s\n", request.Params.Message)

			// In a real application, this would prompt the user for input
			// Here we simulate user providing configuration data
			return &mcp.ElicitResult{
				Action: "accept",
				Content: map[string]any{
					"serverEndpoint": "https://api.example.com",
					"maxRetries":     float64(3),
					"enableLogs":     true,
				},
			}, nil
		},
	})

	_, err = client.Connect(ctx, clientTransport, nil)
	if err != nil {
		log.Fatal(err)
	}

	// Server requests user configuration via elicitation
	configSchema := &jsonschema.Schema{
		Type: "object",
		Properties: map[string]*jsonschema.Schema{
			"serverEndpoint": {Type: "string", Description: "Server endpoint URL"},
			"maxRetries":     {Type: "number", Minimum: ptr(1.0), Maximum: ptr(10.0)},
			"enableLogs":     {Type: "boolean", Description: "Enable debug logging"},
		},
		Required: []string{"serverEndpoint"},
	}

	result, err := serverSession.Elicit(ctx, &mcp.ElicitParams{
		Message:         "Please provide your configuration settings",
		RequestedSchema: configSchema,
	})
	if err != nil {
		log.Fatal(err)
	}

	if result.Action == "accept" {
		fmt.Printf("Configuration received: Endpoint: %v, Max Retries: %.0f, Logs: %v\n",
			result.Content["serverEndpoint"],
			result.Content["maxRetries"],
			result.Content["enableLogs"])
	}

	// Output:
	// Server requests: Please provide your configuration settings
	// Configuration received: Endpoint: https://api.example.com, Max Retries: 3, Logs: true
}

// ptr is a helper function to create pointers for schema constraints
func ptr[T any](v T) *T {
	return &v
}
