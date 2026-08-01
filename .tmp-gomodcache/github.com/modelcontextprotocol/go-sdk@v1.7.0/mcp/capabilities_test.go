// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//lint:file-ignore SA1019 tests exercise deprecated SEP-2577 APIs (roots, sampling, logging).

package mcp

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
)

// TestServerListChangedNotifications verifies that listChanged notifications
// are correctly sent or suppressed based on capability configuration.
func TestServerListChangedNotifications(t *testing.T) {
	tool := &Tool{Name: "test-tool", InputSchema: &jsonschema.Schema{Type: "object"}}

	testCases := []struct {
		name            string
		serverOpts      *ServerOptions
		wantNotifyCount int64
	}{
		{
			name: "Default: notification sent",
			serverOpts: &ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools: &ToolCapabilities{ListChanged: true},
				},
			},
			wantNotifyCount: 1,
		},
		{
			name: "ListChanged false: notification suppressed",
			serverOpts: &ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools: &ToolCapabilities{ListChanged: false},
				},
			},
			wantNotifyCount: 0,
		},
		{
			name: "ListChanged true: notification sent",
			serverOpts: &ServerOptions{
				Capabilities: &ServerCapabilities{
					Tools: &ToolCapabilities{ListChanged: true},
				},
			},
			wantNotifyCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := context.Background()

				// Create server.
				impl := &Implementation{Name: "testServer", Version: "v1.0.0"}
				server := NewServer(impl, tc.serverOpts)

				// Track notifications.
				var notifyCount atomic.Int64

				// Connect client and server.
				cTransport, sTransport := NewInMemoryTransports()
				ss, err := server.Connect(ctx, sTransport, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer ss.Close()

				client := NewClient(&Implementation{Name: "testClient", Version: "v1.0.0"}, &ClientOptions{
					ToolListChangedHandler: func(ctx context.Context, req *ToolListChangedRequest) {
						notifyCount.Add(1)
					},
				})
				cs, err := client.Connect(ctx, cTransport, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer cs.Close()

				// Add a tool, which may or may not trigger notification.
				server.AddTool(tool, nil)

				// Sleep an arbitrary time longer than the debounce delay (synctest
				// makes this practical).
				time.Sleep(1 * time.Second)

				// Wait for all goroutines to be blocked (notification delivered).
				synctest.Wait()

				if got, want := notifyCount.Load(), tc.wantNotifyCount; got != want {
					t.Errorf("notification count: got %d, want %d", got, want)
				}
			})
		})
	}
}

// TestClientListChangedNotifications verifies that roots listChanged notifications
// are correctly sent or suppressed based on client capability configuration.
func TestClientListChangedNotifications(t *testing.T) {
	root := &Root{URI: "file:///test"}

	testCases := []struct {
		name            string
		clientOpts      *ClientOptions
		wantNotifyCount int64
	}{
		{
			name:            "Default: notification sent",
			clientOpts:      nil,
			wantNotifyCount: 1,
		},
		{
			name: "ListChanged false: notification suppressed",
			clientOpts: &ClientOptions{
				Capabilities: &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: false},
				},
			},
			wantNotifyCount: 0,
		},
		{
			name: "ListChanged true: notification sent",
			clientOpts: &ClientOptions{
				Capabilities: &ClientCapabilities{
					RootsV2: &RootCapabilities{ListChanged: true},
				},
			},
			wantNotifyCount: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx := context.Background()

				// Track notifications.
				var notifyCount atomic.Int64

				// Create server with roots list changed handler.
				server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, &ServerOptions{
					RootsListChangedHandler: func(ctx context.Context, req *RootsListChangedRequest) {
						notifyCount.Add(1)
					},
				})

				// Connect client and server.
				cTransport, sTransport := NewInMemoryTransports()
				ss, err := server.Connect(ctx, sTransport, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer ss.Close()

				client := NewClient(&Implementation{Name: "testClient", Version: "v1.0.0"}, tc.clientOpts)
				cs, err := client.Connect(ctx, cTransport, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer cs.Close()

				// Add a root, which may or may not trigger notification.
				client.AddRoots(root)

				// Wait for all goroutines to be blocked (notification delivered).
				synctest.Wait()

				if got, want := notifyCount.Load(), tc.wantNotifyCount; got != want {
					t.Errorf("notification count: got %d, want %d", got, want)
				}
			})
		})
	}
}
