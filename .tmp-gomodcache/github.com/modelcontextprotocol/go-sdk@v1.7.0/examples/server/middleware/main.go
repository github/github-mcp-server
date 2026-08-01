// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// This example demonstrates server side logging using the mcp.Middleware system.
func main() {
	// Create a logger for demonstration purposes.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			// Simplify timestamp format for consistent output.
			if a.Key == slog.TimeKey {
				return slog.String("time", "2025-01-01T00:00:00Z")
			}
			return a
		},
	}))

	loggingMiddleware := func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			logger.Info("MCP method started",
				"method", method,
				"session_id", req.GetSession().ID(),
				"has_params", req.GetParams() != nil,
			)
			// Log more for tool calls.
			if ctr, ok := req.(*mcp.CallToolRequest); ok {
				logger.Info("Calling tool",
					"name", ctr.Params.Name,
					"args", ctr.Params.Arguments)
			}

			start := time.Now()
			result, err := next(ctx, method, req)
			duration := time.Since(start)
			if err != nil {
				logger.Error("MCP method failed",
					"method", method,
					"session_id", req.GetSession().ID(),
					"duration_ms", duration.Milliseconds(),
					"err", err,
				)
			} else {
				logger.Info("MCP method completed",
					"method", method,
					"session_id", req.GetSession().ID(),
					"duration_ms", duration.Milliseconds(),
					"has_result", result != nil,
				)
				// Log more for tool results.
				if ctr, ok := result.(*mcp.CallToolResult); ok {
					logger.Info("tool result",
						"isError", ctr.IsError,
						"structuredContent", ctr.StructuredContent)
				}
			}
			return result, err
		}
	}

	// Create server with middleware
	server := mcp.NewServer(&mcp.Implementation{Name: "logging-example"}, nil)
	server.AddReceivingMiddleware(loggingMiddleware)

	// Add a simple tool
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "greet",
			Description: "Greet someone with logging.",
			InputSchema: &jsonschema.Schema{
				Type: "object",
				Properties: map[string]*jsonschema.Schema{
					"name": {
						Type:        "string",
						Description: "Name to greet",
					},
				},
				Required: []string{"name"},
			},
		},
		func(
			ctx context.Context,
			req *mcp.CallToolRequest, args map[string]any,
		) (*mcp.CallToolResult, any, error) {
			name, ok := args["name"].(string)
			if !ok {
				return nil, nil, fmt.Errorf("name parameter is required and must be a string")
			}

			message := fmt.Sprintf("Hello, %s!", name)
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: message},
				},
			}, message, nil
		},
	)

	// Create client-server connection for demonstration
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client"}, nil)
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	ctx := context.Background()

	// Connect server and client
	serverSession, _ := server.Connect(ctx, serverTransport, nil)
	defer serverSession.Close()

	clientSession, _ := client.Connect(ctx, clientTransport, nil)
	defer clientSession.Close()

	// Call the tool to demonstrate logging
	result, _ := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "greet",
		Arguments: map[string]any{
			"name": "World",
		},
	})

	fmt.Printf("Tool result: %s\n", result.Content[0].(*mcp.TextContent).Text)

	// Output:
	// time=2025-01-01T00:00:00Z level=INFO msg="MCP method started" method=initialize session_id="" has_params=true
	// time=2025-01-01T00:00:00Z level=INFO msg="MCP method completed" method=initialize session_id="" duration_ms=0 has_result=true
	// time=2025-01-01T00:00:00Z level=INFO msg="MCP method started" method=notifications/initialized session_id="" has_params=true
	// time=2025-01-01T00:00:00Z level=INFO msg="MCP method completed" method=notifications/initialized session_id="" duration_ms=0 has_result=false
	// time=2025-01-01T00:00:00Z level=INFO msg="MCP method started" method=tools/call session_id="" has_params=true
	// time=2025-01-01T00:00:00Z level=INFO msg="Calling tool" name=greet args="{\"name\":\"World\"}"
	// time=2025-01-01T00:00:00Z level=INFO msg="MCP method completed" method=tools/call session_id="" duration_ms=0 has_result=true
	// Tool result: Hello, World!
}
