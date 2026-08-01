// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp_test

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// !+streamablehandler

// TODO: Until we have a way to clean up abandoned sessions, this test will leak goroutines (see #499)
func ExampleStreamableHTTPHandler() {
	// Create a new streamable handler, using the same MCP server for every request.
	//
	// Here, we configure it to serves application/json responses rather than
	// text/event-stream, just so the output below doesn't use random event ids.
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.1.0"}, nil)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{JSONResponse: true})
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	// The SDK is currently permissive of some missing keys in "params".
	resp := mustPostMessage(`{"jsonrpc": "2.0", "id": 1, "method":"initialize", "params": {"protocolVersion":"2025-11-25"}}`, httpServer.URL)
	fmt.Println(resp)
	// Output:
	// {"jsonrpc":"2.0","id":1,"result":{"capabilities":{"logging":{}},"protocolVersion":"2025-11-25","serverInfo":{"name":"server","version":"v0.1.0"}}}
}

// !-streamablehandler

// !+httpmiddleware

func ExampleStreamableHTTPHandler_middleware() {
	server := mcp.NewServer(&mcp.Implementation{Name: "server", Version: "v0.1.0"}, nil)
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})
	loggingHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Example debugging; you could also capture the response.
		body, err := io.ReadAll(req.Body)
		if err != nil {
			log.Fatal(err)
		}
		req.Body.Close() // ignore error
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		fmt.Println(req.Method, string(body))
		handler.ServeHTTP(w, req)
	})
	httpServer := httptest.NewServer(loggingHandler)
	defer httpServer.Close()

	// The SDK is currently permissive of some missing keys in "params".
	mustPostMessage(`{"jsonrpc": "2.0", "id": 1, "method":"initialize", "params": {}}`, httpServer.URL)
	// Output:
	// POST {"jsonrpc": "2.0", "id": 1, "method":"initialize", "params": {}}
}

// !-httpmiddleware

func mustPostMessage(msg, url string) string {
	req := orFatal(http.NewRequest("POST", url, strings.NewReader(msg)))
	req.Header["Content-Type"] = []string{"application/json"}
	req.Header["Accept"] = []string{"application/json", "text/event-stream"}
	resp := orFatal(http.DefaultClient.Do(req))
	defer resp.Body.Close()
	body := orFatal(io.ReadAll(resp.Body))
	return string(body)
}

func orFatal[T any](t T, err error) T {
	if err != nil {
		log.Fatal(err)
	}
	return t
}
