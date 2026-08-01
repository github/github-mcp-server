// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSSEServer(t *testing.T) {
	for _, closeServerFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("closeServerFirst=%t", closeServerFirst), func(t *testing.T) {
			ctx := context.Background()
			server := NewServer(testImpl, nil)
			AddTool(server, &Tool{Name: "greet"}, sayHi)

			sseHandler := NewSSEHandler(func(*http.Request) *Server { return server }, nil)

			serverSessions := make(chan *ServerSession, 1)
			sseHandler.onConnection = func(ss *ServerSession) {
				select {
				case serverSessions <- ss:
				default:
				}
			}
			httpServer := httptest.NewServer(sseHandler)
			defer httpServer.Close()

			var customClientUsed int64
			customClient := &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					atomic.AddInt64(&customClientUsed, 1)
					return http.DefaultTransport.RoundTrip(req)
				}),
			}

			clientTransport := &SSEClientTransport{
				Endpoint:   httpServer.URL,
				HTTPClient: customClient,
			}

			c := NewClient(testImpl, nil)
			cs, err := c.Connect(ctx, clientTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := cs.Ping(ctx, nil); err != nil {
				t.Fatal(err)
			}
			ss := <-serverSessions
			gotHi, err := cs.CallTool(ctx, &CallToolParams{
				Name:      "greet",
				Arguments: map[string]any{"Name": "user"},
			})
			if err != nil {
				t.Fatal(err)
			}
			wantHi := &CallToolResult{
				Content: []Content{
					&TextContent{Text: "hi user"},
				},
			}
			if diff := cmp.Diff(wantHi, gotHi, ctrCmpOpts...); diff != "" {
				t.Errorf("tools/call 'greet' mismatch (-want +got):\n%s", diff)
			}

			// Verify that customClient was used
			if atomic.LoadInt64(&customClientUsed) == 0 {
				t.Error("Expected custom HTTP client to be used, but it wasn't")
			}

			t.Run("badrequests", func(t *testing.T) {
				msgEndpoint := cs.mcpConn.(*sseClientConn).msgEndpoint.String()

				// Test some invalid data, and verify that we get 400s.
				badRequests := []struct {
					name             string
					body             string
					responseContains string
				}{
					{"not a method", `{"jsonrpc":"2.0", "method":"notamethod"}`, "not handled"},
					{"missing ID", `{"jsonrpc":"2.0", "method":"ping"}`, "missing id"},
				}
				for _, r := range badRequests {
					t.Run(r.name, func(t *testing.T) {
						resp, err := http.Post(msgEndpoint, "application/json", bytes.NewReader([]byte(r.body)))
						if err != nil {
							t.Fatal(err)
						}
						defer resp.Body.Close()
						if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
							t.Errorf("Sending bad request %q: got status %d, want %d", r.body, got, want)
						}
						result, err := io.ReadAll(resp.Body)
						if err != nil {
							t.Fatalf("Reading response: %v", err)
						}
						if !bytes.Contains(result, []byte(r.responseContains)) {
							t.Errorf("Response body does not contain %q:\n%s", r.responseContains, string(result))
						}
					})
				}
			})

			// Test that closing either end of the connection terminates the other
			// end.
			if closeServerFirst {
				cs.Close()
				ss.Wait()
			} else {
				ss.Close()
				cs.Wait()
			}
		})
	}
}

// roundTripperFunc is a helper to create a custom RoundTripper
type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestSSEClientTransport_HTTPErrors(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		wantErrContain string
	}{
		{
			name:           "401 Unauthorized",
			statusCode:     http.StatusUnauthorized,
			wantErrContain: "Unauthorized",
		},
		{
			name:           "403 Forbidden",
			statusCode:     http.StatusForbidden,
			wantErrContain: "Forbidden",
		},
		{
			name:           "404 Not Found",
			statusCode:     http.StatusNotFound,
			wantErrContain: "Not Found",
		},
		{
			name:           "500 Internal Server Error",
			statusCode:     http.StatusInternalServerError,
			wantErrContain: "Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server that returns the specified status code
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(tt.statusCode), tt.statusCode)
			}))
			defer httpServer.Close()

			clientTransport := &SSEClientTransport{
				Endpoint: httpServer.URL,
			}

			c := NewClient(testImpl, nil)
			_, err := c.Connect(context.Background(), clientTransport, nil)

			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			errStr := err.Error()
			if !bytes.Contains([]byte(errStr), []byte(tt.wantErrContain)) {
				t.Errorf("error message %q does not contain %q", errStr, tt.wantErrContain)
			}
		})
	}
}

// TestSSE405AllowHeader verifies RFC 9110 §15.5.6 compliance:
// 405 Method Not Allowed responses MUST include an Allow header.
func TestSSE405AllowHeader(t *testing.T) {
	server := NewServer(testImpl, nil)

	handler := NewSSEHandler(func(req *http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	methods := []string{"PUT", "PATCH", "DELETE", "OPTIONS"}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req, err := http.NewRequest(method, httpServer.URL, nil)
			if err != nil {
				t.Fatal(err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if got, want := resp.StatusCode, http.StatusMethodNotAllowed; got != want {
				t.Errorf("status code: got %d, want %d", got, want)
			}

			allow := resp.Header.Get("Allow")
			if allow != "GET, POST" {
				t.Errorf("Allow header: got %q, want %q", allow, "GET, POST")
			}
		})
	}
}

// TestSSELocalhostProtection verifies that DNS rebinding protection
// is automatically enabled for localhost servers.
func TestSSELocalhostProtection(t *testing.T) {
	server := NewServer(testImpl, nil)

	tests := []struct {
		name              string
		listenAddr        string
		hostHeader        string
		disableProtection bool
		wantStatus        int
	}{
		{
			name:       "127.0.0.1 accepts 127.0.0.1",
			listenAddr: "127.0.0.1:0",
			hostHeader: "127.0.0.1:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "127.0.0.1 accepts localhost",
			listenAddr: "127.0.0.1:0",
			hostHeader: "localhost:1234",
			wantStatus: http.StatusOK,
		},
		{
			name:       "127.0.0.1 rejects evil.com",
			listenAddr: "127.0.0.1:0",
			hostHeader: "evil.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "127.0.0.1 rejects evil.com:80",
			listenAddr: "127.0.0.1:0",
			hostHeader: "evil.com:80",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "127.0.0.1 rejects localhost.evil.com",
			listenAddr: "127.0.0.1:0",
			hostHeader: "localhost.evil.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "0.0.0.0 via localhost rejects evil.com",
			listenAddr: "0.0.0.0:0",
			hostHeader: "evil.com",
			wantStatus: http.StatusForbidden,
		},
		{
			name:              "disabled accepts evil.com",
			listenAddr:        "127.0.0.1:0",
			hostHeader:        "evil.com",
			disableProtection: true,
			wantStatus:        http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &SSEOptions{
				DisableLocalhostProtection: tt.disableProtection,
			}
			handler := NewSSEHandler(func(req *http.Request) *Server { return server }, opts)

			listener, err := net.Listen("tcp", tt.listenAddr)
			if err != nil {
				t.Fatalf("Failed to listen on %s: %v", tt.listenAddr, err)
			}
			defer listener.Close()

			srv := &http.Server{Handler: handler}
			go srv.Serve(listener)
			defer srv.Close()

			// Use a GET request since it's the entry point for SSE sessions.
			// For accepted requests, the response will be a hanging SSE stream,
			// but we only need to check the initial status code.
			req, err := http.NewRequest("GET", fmt.Sprintf("http://%s", listener.Addr().String()), nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tt.hostHeader
			req.Header.Set("Accept", "text/event-stream")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if got := resp.StatusCode; got != tt.wantStatus {
				t.Errorf("Status code: got %d, want %d", got, tt.wantStatus)
			}
		})
	}
}
