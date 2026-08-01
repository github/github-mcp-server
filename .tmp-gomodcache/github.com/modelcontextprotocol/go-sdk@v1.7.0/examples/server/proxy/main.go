// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Define a context key for passing headers.
type contextKey string

const headerContextKey contextKey = "ctx-headers"

// HeaderForwardingTransport is an http.RoundTripper that injects headers
// from the context into the outgoing request.
type HeaderForwardingTransport struct{}

// RoundTrip executes a single HTTP transaction, adding headers from the context.
func (h *HeaderForwardingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original one.
	newReq := req.Clone(req.Context())

	// Retrieve headers from the context.
	if headers, ok := req.Context().Value(headerContextKey).(http.Header); ok {
		for key, values := range headers {
			// Copy all values for each header.
			for _, value := range values {
				newReq.Header.Add(key, value)
			}
		}
	}

	return http.DefaultTransport.RoundTrip(newReq)
}

// NewHeaderForwardingClient creates a new http.Client that uses HeaderForwardingTransport.
func NewHeaderForwardingClient() *http.Client {
	return &http.Client{
		Transport: new(HeaderForwardingTransport),
	}
}

// Backend Server: Echoes received headers to verify propagation.
func runBackendServer() {
	server := mcp.NewServer(&mcp.Implementation{Name: "backend-server", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo_headers",
		Description: "Returns the headers received by the server",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		return nil, req.Extra.Header, nil
	})

	// Start the backend server on port 8082.
	log.Println("Starting Backend Server on :8082")
	if err := http.ListenAndServe(":8082", mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)); err != nil {
		log.Fatal(err)
	}
}

// Proxy Server: Forwards requests to the backend with headers.
func runProxyServer(ctx context.Context) {
	// Connect to the backend server (acting as a client).
	client := mcp.NewClient(&mcp.Implementation{Name: "proxy-client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:   "http://localhost:8082/mcp",
		HTTPClient: NewHeaderForwardingClient(),
	}, nil)
	if err != nil {
		log.Fatalf("Failed to connect to backend: %v", err)
	}
	defer clientSession.Close()

	server := mcp.NewServer(&mcp.Implementation{Name: "proxy-server", Version: "1.0.0"}, nil)
	// Add a tool that forwards the call to the backend.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "forward_headers",
		Description: "Calls the backend server, ensuring headers are propagated",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		incomingHeaders := req.Extra.Header
		log.Printf("Gateway received headers: %v", incomingHeaders)
		propagateCtx := context.WithValue(ctx, headerContextKey, incomingHeaders)
		result, err := clientSession.CallTool(propagateCtx, &mcp.CallToolParams{
			Name: "echo_headers",
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to call backend: %w", err)
		}

		return result, nil, nil
	})

	// Start the gateway server on port 8081.
	log.Println("Starting Gateway Server on :8081")
	if err := http.ListenAndServe(":8081", mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)); err != nil {
		log.Fatal(err)
	}
}

func runClient(ctx context.Context) {
	// Connect to the proxy server (acting as a client).
	client := mcp.NewClient(&mcp.Implementation{Name: "client", Version: "1.0.0"}, nil)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: "http://localhost:8081/mcp",
	}, nil)
	if err != nil {
		log.Fatalf("Failed to connect to proxy: %v", err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "forward_headers",
	})
	if err != nil {
		log.Fatalf("Failed to call proxy: %v", err)
	}
	log.Printf("Client received result: %v", result.StructuredContent)
}

func main() {
	ctx := context.Background()
	go runBackendServer()
	// Give the backend a moment to start.
	time.Sleep(100 * time.Millisecond)
	go runProxyServer(ctx)
	// Give the proxy a moment to start.
	time.Sleep(100 * time.Millisecond)
	runClient(ctx)
}
