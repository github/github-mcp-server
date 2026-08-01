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
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var (
	host  = flag.String("host", "localhost", "host to connect to/listen on")
	port  = flag.Int("port", 8000, "port number to connect to/listen on")
	proto = flag.String("proto", "http", "if set, use as proto:// part of URL (ignored for server)")
)

func main() {
	out := flag.CommandLine.Output()
	flag.Usage = func() {
		fmt.Fprintf(out, "Usage: %s <client|server> [-proto <http|https>] [-port <port] [-host <host>]\n\n", os.Args[0])
		fmt.Fprintf(out, "This program demonstrates MCP over HTTP using the streamable transport.\n")
		fmt.Fprintf(out, "It can run as either a server or client.\n\n")
		fmt.Fprintf(out, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(out, "\nExamples:\n")
		fmt.Fprintf(out, "  Run as server:  %s server\n", os.Args[0])
		fmt.Fprintf(out, "  Run as client:  %s client\n", os.Args[0])
		fmt.Fprintf(out, "  Custom host/port: %s -port 9000 -host 0.0.0.0 server\n", os.Args[0])
		os.Exit(1)
	}
	flag.Parse()

	if flag.NArg() != 1 {
		fmt.Fprintf(out, "Error: Must specify 'client' or 'server' as first argument\n")
		flag.Usage()
	}
	mode := flag.Arg(0)

	switch mode {
	case "server":
		if *proto != "http" {
			log.Fatalf("Server only works with 'http' (you passed proto=%s)", *proto)
		}
		runServer(fmt.Sprintf("%s:%d", *host, *port))
	case "client":
		runClient(fmt.Sprintf("%s://%s:%d", *proto, *host, *port))
	default:
		fmt.Fprintf(os.Stderr, "Error: Invalid mode '%s'. Must be 'client' or 'server'\n\n", mode)
		flag.Usage()
	}
}

// GetTimeParams defines the parameters for the cityTime tool.
type GetTimeParams struct {
	City string `json:"city" jsonschema:"City to get time for (nyc, sf, or boston)"`
}

// getTime implements the tool that returns the current time for a given city.
func getTime(ctx context.Context, req *mcp.CallToolRequest, params *GetTimeParams) (*mcp.CallToolResult, any, error) {
	// Define time zones for each city
	locations := map[string]string{
		"nyc":    "America/New_York",
		"sf":     "America/Los_Angeles",
		"boston": "America/New_York",
	}

	city := params.City
	if city == "" {
		city = "nyc" // Default to NYC
	}

	// Get the timezone.
	tzName, ok := locations[city]
	if !ok {
		return nil, nil, fmt.Errorf("unknown city: %s", city)
	}

	// Load the location.
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load timezone: %w", err)
	}

	// Get current time in that location.
	now := time.Now().In(loc)

	// Format the response.
	cityNames := map[string]string{
		"nyc":    "New York City",
		"sf":     "San Francisco",
		"boston": "Boston",
	}

	response := fmt.Sprintf("The current time in %s is %s",
		cityNames[city],
		now.Format(time.RFC3339))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: response},
		},
	}, nil, nil
}

func runServer(url string) {
	// Create an MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "time-server",
		Version: "1.0.0",
	}, nil)

	// Add MCP-level logging middleware.
	server.AddReceivingMiddleware(createLoggingMiddleware())

	// Add the cityTime tool.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "cityTime",
		Description: "Get the current time in NYC, San Francisco, or Boston",
	}, getTime)

	// Create the streamable HTTP handler.
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)

	log.Printf("MCP server listening on %s", url)
	log.Printf("Available tool: cityTime (cities: nyc, sf, boston)")

	// Start the HTTP server.
	if err := http.ListenAndServe(url, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func runClient(url string) {
	ctx := context.Background()

	// Create the URL for the server.
	log.Printf("Connecting to MCP server at %s", url)

	// Create an MCP client.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)

	// Connect to the server.
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer session.Close()

	log.Printf("Connected to server (session ID: %s)", session.ID())

	// First, list available tools.
	log.Println("Listing available tools...")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	for _, tool := range toolsResult.Tools {
		log.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}

	// Call the cityTime tool for each city.
	cities := []string{"nyc", "sf", "boston"}

	log.Println("Getting time for each city...")
	for _, city := range cities {
		// Call the tool.
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "cityTime",
			Arguments: map[string]any{
				"city": city,
			},
		})
		if err != nil {
			log.Printf("Failed to get time for %s: %v\n", city, err)
			continue
		}

		// Print the result.
		for _, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok {
				log.Printf("  %s", textContent.Text)
			}
		}
	}

	log.Println("Client completed successfully")
}
