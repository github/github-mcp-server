// Copyright 2026 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// Flags.
var (
	port = flag.Int("port", 8000, "Port to listen on")
)

// Configuration required for this example.
var (
	// Authorization server to return in the protected resource metadata.
	authorizationServer = ""
	// Introspection endpoint for verifying tokens.
	introspectionEndpoint = ""
	// Client credentials used in the introspection request.
	clientID     = ""
	clientSecret = ""
)

func verifyToken(ctx context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
	data := url.Values{}
	data.Set("token", token)
	data.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, "POST", introspectionEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		dump, _ := httputil.DumpResponse(resp, true)
		log.Printf("Introspection failed: %s", dump)
		return nil, fmt.Errorf("introspection failed with status %d", resp.StatusCode)
	}

	var result struct {
		Active bool   `json:"active"`
		Scope  string `json:"scope"`
		Exp    int64  `json:"exp"`
		Sub    string `json:"sub"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if !result.Active {
		return nil, auth.ErrInvalidToken
	}

	return &auth.TokenInfo{
		Scopes:     strings.Fields(result.Scope),
		Expiration: time.Unix(result.Exp, 0),
		UserID:     result.Sub,
	}, nil
}

type args struct {
	Input string `json:"input"`
}

func echo(ctx context.Context, req *mcp.CallToolRequest, args args) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: args.Input},
		},
	}, nil, nil
}

func main() {
	flag.Parse()
	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             fmt.Sprintf("http://localhost:%d/mcp", *port),
		AuthorizationServers: []string{authorizationServer},
		ScopesSupported:      []string{"read"},
	}
	http.Handle("/.well-known/oauth-protected-resource", auth.ProtectedResourceMetadataHandler(metadata))

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-server",
		Version: "1.0.0",
	}, nil)
	server.AddReceivingMiddleware(createLoggingMiddleware())
	mcp.AddTool(server, &mcp.Tool{Name: "echo"}, echo)

	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)

	authMiddleware := auth.RequireBearerToken(verifyToken, &auth.RequireBearerTokenOptions{
		Scopes:              []string{"read"},
		ResourceMetadataURL: fmt.Sprintf("http://localhost:%d/.well-known/oauth-protected-resource", *port),
	})

	http.Handle("/mcp", authMiddleware(handler))

	log.Printf("Starting server on http://localhost:%d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf("localhost:%d", *port), nil))
}

// createLoggingMiddleware creates an MCP middleware that logs method calls.
func createLoggingMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			start := time.Now()
			sessionID := req.GetSession().ID()

			// Log request details.
			log.Printf("[REQUEST] Session: %s | Method: %s",
				sessionID,
				method)

			// Call the actual handler.
			result, err := next(ctx, method, req)

			// Log response details.
			duration := time.Since(start)

			if err != nil {
				log.Printf("[RESPONSE] Session: %s | Method: %s | Status: ERROR | Duration: %v | Error: %v",
					sessionID,
					method,
					duration,
					err)
			} else {
				log.Printf("[RESPONSE] Session: %s | Method: %s | Status: OK | Duration: %v",
					sessionID,
					method,
					duration)
			}

			return result, err
		}
	}
}
