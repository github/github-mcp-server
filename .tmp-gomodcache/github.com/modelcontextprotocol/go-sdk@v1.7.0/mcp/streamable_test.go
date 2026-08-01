// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

//lint:file-ignore SA1019 tests exercise deprecated SEP-2577 APIs (roots, sampling, logging).

package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

func TestStreamableTransports(t *testing.T) {
	// This test checks that the streamable server and client transports can
	// communicate.

	ctx := context.Background()

	tests := []struct {
		useJSON bool
		replay  bool
	}{
		{false, false},
		{false, true},
		{true, false},
		{true, true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("JSONResponse=%v;replay=%v", test.useJSON, test.replay), func(t *testing.T) {
			// Create a server with some simple tools.
			server := NewServer(testImpl, nil)
			AddTool(server, &Tool{Name: "greet", Description: "say hi"}, sayHi)
			// The "hang" tool checks that context cancellation is propagated.
			// It hangs until the context is cancelled.
			var (
				start     = make(chan struct{})
				cancelled = make(chan struct{}, 1) // don't block the request
			)
			hang := func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, any, error) {
				start <- struct{}{}
				select {
				case <-ctx.Done():
					cancelled <- struct{}{}
				case <-time.After(5 * time.Second):
					return nil, nil, nil
				}
				return nil, nil, nil
			}
			AddTool(server, &Tool{Name: "hang"}, hang)
			// We use sampling to test server->client requests, both before and after
			// the related client->server request completes.
			sampleDone := make(chan struct{})
			var sampleWG sync.WaitGroup
			AddTool(server, &Tool{Name: "sample"}, func(ctx context.Context, req *CallToolRequest, args any) (*CallToolResult, any, error) {
				type testCase struct {
					label       string
					ctx         context.Context
					wantSuccess bool
				}
				testSample := func(tc testCase) {
					res, err := req.Session.CreateMessage(tc.ctx, &CreateMessageParams{})
					if gotSuccess := err == nil; gotSuccess != tc.wantSuccess {
						t.Errorf("%s: CreateMessage success=%v, want %v", tc.label, gotSuccess, tc.wantSuccess)
					}
					if err != nil {
						return
					}
					if g, w := res.Model, "aModel"; g != w {
						t.Errorf("%s: got model %q, want %q", tc.label, g, w)
					}
				}
				// Test that we can make sampling requests during tool handling.
				//
				// Try this on both the request context and a background context, so
				// that messages may be delivered on either the POST or GET connection.
				for _, test := range []testCase{
					{"request context", ctx, true},
					{"background context", context.Background(), true},
				} {
					testSample(test)
				}
				// Now, spin off a goroutine that runs after the sampling request, to
				// check behavior when the client request has completed.
				sampleWG.Go(func() {
					<-sampleDone
					// Test that sampling requests in the tool context fail outside of
					// tool handling, but succeed on the background context.
					for _, test := range []testCase{
						{"request context", ctx, false},
						{"background context", context.Background(), true},
					} {
						testSample(test)
					}
				})
				return &CallToolResult{}, nil, nil
			})

			// Start an httptest.Server with the StreamableHTTPHandler, wrapped in a
			// cookie-checking middleware.
			opts := &StreamableHTTPOptions{
				JSONResponse: test.useJSON,
			}
			if test.replay {
				opts.EventStore = NewMemoryEventStore(nil)
			}
			handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, opts)

			var (
				headerMu   sync.Mutex
				lastHeader http.Header
			)
			httpServer := httptest.NewServer(mustNotPanic(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				headerMu.Lock()
				lastHeader = r.Header
				headerMu.Unlock()
				cookie, err := r.Cookie("test-cookie")
				if err != nil {
					t.Errorf("missing cookie: %v", err)
				} else if cookie.Value != "test-value" {
					t.Errorf("got cookie %q, want %q", cookie.Value, "test-value")
				}
				handler.ServeHTTP(w, r)
			})))
			defer httpServer.Close()

			// Create a client and connect it to the server using our StreamableClientTransport.
			// Check that all requests honor a custom client.
			jar, err := cookiejar.New(nil)
			if err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(httpServer.URL)
			if err != nil {
				t.Fatal(err)
			}
			jar.SetCookies(u, []*http.Cookie{{Name: "test-cookie", Value: "test-value"}})
			httpClient := &http.Client{Jar: jar}
			transport := &StreamableClientTransport{
				Endpoint:   httpServer.URL,
				HTTPClient: httpClient,
			}
			client := NewClient(testImpl, &ClientOptions{
				CreateMessageHandler: func(context.Context, *CreateMessageRequest) (*CreateMessageResult, error) {
					return &CreateMessageResult{Model: "aModel", Content: &TextContent{}}, nil
				},
			})
			session, err := client.Connect(ctx, transport, nil)
			if err != nil {
				t.Fatalf("client.Connect() failed: %v", err)
			}
			defer session.Close()
			sid := session.ID()
			if sid == "" {
				t.Fatalf("empty session ID")
			}
			// Stateful HTTP servers reject 2026-07-28 (which requires
			// StreamableHTTPOptions.Stateless = true), so the client falls
			// back to the legacy initialize handshake which explicitly
			// requests 2025-11-25.
			if g, w := session.mcpConn.(*streamableClientConn).initializedResult.ProtocolVersion, protocolVersion20251125; g != w {
				t.Fatalf("got protocol version %q, want %q", g, w)
			}

			// Verify the behavior of various tools.

			// The "greet" tool should just work.
			params := &CallToolParams{
				Name:      "greet",
				Arguments: map[string]any{"Name": "foo"},
			}
			got, err := session.CallTool(ctx, params)
			if err != nil {
				t.Fatalf("CallTool() failed: %v", err)
			}
			if g := session.ID(); g != sid {
				t.Errorf("session ID: got %q, want %q", g, sid)
			}
			// Stateful HTTP requires the legacy protocol (see the earlier
			// initializeResult assertion); the client sends the matching
			// MCP-Protocol-Version header.
			if g, w := lastHeader.Get(protocolVersionHeader), protocolVersion20251125; g != w {
				t.Errorf("got protocol version header %q, want %q", g, w)
			}
			want := &CallToolResult{
				Content: []Content{&TextContent{Text: "hi foo"}},
			}
			if diff := cmp.Diff(want, got, ctrCmpOpts...); diff != "" {
				t.Errorf("CallTool() returned unexpected content (-want +got):\n%s", diff)
			}

			// The "hang" tool should be cancellable.
			ctx2, cancel := context.WithCancel(context.Background())
			go session.CallTool(ctx2, &CallToolParams{Name: "hang"})
			<-start
			cancel()
			select {
			case <-cancelled:
			case <-time.After(5 * time.Second):
				t.Fatal("timeout waiting for cancellation")
			}

			// The "sampling" tool checks the validity of server->client requests
			// both within and without the tool context.
			result, err := session.CallTool(ctx, &CallToolParams{
				Name:      "sample",
				Arguments: map[string]any{},
			})
			if err != nil {
				t.Fatal(err)
			}
			// Run the out-of-band checks.
			close(sampleDone)
			sampleWG.Wait()

			if result.IsError {
				t.Fatalf("tool failed: %s", result.Content[0].(*TextContent).Text)
			}
		})
	}
}

func TestStreamableConcurrentHandling(t *testing.T) {
	// This test checks that the streamable server and client transports can
	// communicate.
	type count struct {
		Count int
	}

	var mu sync.Mutex
	counts := make(map[string]int)

	server := NewServer(testImpl, nil)
	AddTool(server, &Tool{Name: "inc"}, func(ctx context.Context, req *CallToolRequest, _ any) (*CallToolResult, count, error) {
		id := req.Session.ID()
		mu.Lock()
		defer mu.Unlock()
		c := counts[id]
		counts[id] = c + 1
		return nil, count{c}, nil
	})
	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	ctx := context.Background()
	client := NewClient(testImpl, nil)
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			clientSession, err := client.Connect(ctx, &StreamableClientTransport{Endpoint: httpServer.URL}, nil)
			if err != nil {
				t.Errorf("Connect failed: %v", err)
				return
			}
			defer clientSession.Close()
			for i := range 10 {
				res, err := clientSession.CallTool(ctx, &CallToolParams{Name: "inc"})
				if err != nil {
					t.Errorf("CallTool failed: %v", err)
					return
				}
				if got := int(res.StructuredContent.(map[string]any)["Count"].(float64)); got != i {
					t.Errorf("got count %d, want %d", got, i)
				}
			}
		})
	}
	wg.Wait()
}

func TestStreamableServerShutdown(t *testing.T) {
	ctx := context.Background()

	// This test checks that closing the streamable HTTP server actually results
	// in client session termination, provided one of following holds:
	//  1. The server is stateful, and therefore the hanging GET fails the connection.
	//  2. The server is stateless, and the client uses a KeepAlive.
	tests := []struct {
		name                 string
		stateless, keepalive bool
	}{
		{"stateful", false, false},
		{"stateless with keepalive", true, true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := NewServer(testImpl, nil)
			// Add a tool, just so we can check things are working.
			AddTool(server, &Tool{Name: "greet"}, sayHi)

			handler := NewStreamableHTTPHandler(
				func(req *http.Request) *Server { return server },
				&StreamableHTTPOptions{Stateless: test.stateless})

			// When we shut down the server, we need to explicitly close ongoing
			// connections. Otherwise, the hanging GET may never terminate.
			httpServer := httptest.NewUnstartedServer(handler)
			httpServer.Config.RegisterOnShutdown(func() {
				for session := range server.Sessions() {
					session.Close()
				}
			})
			httpServer.Start()
			defer httpServer.Close()

			// Connect and run a tool.
			var opts ClientOptions
			if test.keepalive {
				opts.KeepAlive = 50 * time.Millisecond
			}
			client := NewClient(testImpl, &opts)
			// Pin to 2025-11-25: this test relies on the legacy hanging GET
			// (stateful case) or Ping-based keepalive (stateless case) for
			// the client to notice the server shutdown. Both mechanisms are
			// removed in 2026-07-28 (SEP-2575), so the new protocol would
			// leave the client waiting forever.
			clientSession, err := client.Connect(ctx, &StreamableClientTransport{
				Endpoint:   httpServer.URL,
				MaxRetries: -1, // avoid slow tests during exponential retries
			}, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
			if err != nil {
				t.Fatal(err)
			}
			defer clientSession.Close()

			params := &CallToolParams{
				Name:      "greet",
				Arguments: map[string]any{"Name": "foo"},
			}
			// Verify that we can call a tool.
			if _, err := clientSession.CallTool(ctx, params); err != nil {
				t.Fatalf("CallTool() failed: %v", err)
			}

			// Shut down the server. Sessions should terminate.
			go func() {
				if err := httpServer.Config.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
					t.Errorf("closing http server: %v", err)
				}
			}()

			// Wait may return an error (after all, the connection failed), but it
			// should not hang.
			t.Log("Client waiting")
			_ = clientSession.Wait()
		})
	}
}

// TestStreamableStatelessKeepaliveRace verifies that there is no data race between
// ServerSession.startKeepalive and ServerSession.Close in stateless servers.
func TestStreamableStatelessKeepaliveRace(t *testing.T) {
	ctx := context.Background()
	server := NewServer(testImpl, &ServerOptions{KeepAlive: time.Hour})
	AddTool(server, &Tool{Name: "greet"}, sayHi)
	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return server },
		&StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	for range 50 {
		cs, err := NewClient(testImpl, nil).Connect(ctx, &StreamableClientTransport{
			Endpoint: httpServer.URL,
		}, nil)
		if err != nil {
			t.Fatalf("NewClient() failed: %v", err)
		}
		_, _ = cs.CallTool(ctx, &CallToolParams{
			Name:      "greet",
			Arguments: map[string]any{"Name": "world"},
		})
		_ = cs.Close()
	}
}

// TestClientReplay verifies that the client can recover from a mid-stream
// network failure and receive replayed messages (if replay is configured). It
// uses a proxy that is killed and restarted to simulate a recoverable network
// outage.
//
// TODO: Until we have a way to clean up abandoned sessions, this test will leak goroutines (see #499)
func TestClientReplay(t *testing.T) {
	for _, test := range []clientReplayTest{
		{"default", 0, true},
		{"no retries", -1, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			testClientReplay(t, test)
		})
	}
}

type clientReplayTest struct {
	name          string
	maxRetries    int
	wantRecovered bool
}

func testClientReplay(t *testing.T, test clientReplayTest) {
	notifications := make(chan string)
	// Configure the real MCP server.
	server := NewServer(testImpl, nil)

	// Use a channel to synchronize the server's message sending with the test's
	// proxy-killing action.
	serverReadyToKillProxy := make(chan struct{})
	serverClosed := make(chan struct{})
	AddTool(server, &Tool{Name: "multiMessageTool", InputSchema: &jsonschema.Schema{Type: "object"}},
		func(ctx context.Context, req *CallToolRequest, args map[string]any) (*CallToolResult, any, error) {
			// Send one message to the request context, and another to a background
			// context (which will end up on the hanging GET).

			bgCtx := context.Background()
			req.Session.NotifyProgress(ctx, &ProgressNotificationParams{Message: "msg1"})
			req.Session.NotifyProgress(bgCtx, &ProgressNotificationParams{Message: "msg2"})

			// Signal the test that it can now kill the proxy.
			close(serverReadyToKillProxy)
			<-serverClosed

			// These messages should be queued for replay by the server after
			// the client's connection drops.
			req.Session.NotifyProgress(ctx, &ProgressNotificationParams{Message: "msg3"})
			req.Session.NotifyProgress(bgCtx, &ProgressNotificationParams{Message: "msg4"})
			return new(CallToolResult), nil, nil
		})

	realServer := httptest.NewServer(mustNotPanic(t, NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, &StreamableHTTPOptions{
		EventStore: NewMemoryEventStore(nil), // necessary for replay
	})))
	t.Cleanup(func() {
		t.Log("Closing real HTTP server")
		realServer.Close()
	})
	realServerURL, err := url.Parse(realServer.URL)
	if err != nil {
		t.Fatalf("Failed to parse real server URL: %v", err)
	}

	// Configure a proxy that sits between the client and the real server.
	proxyHandler := httputil.NewSingleHostReverseProxy(realServerURL)
	// note: don't use mustNotPanic here as the proxy WILL panic when killed.
	proxy := httptest.NewServer(proxyHandler)
	proxyAddr := proxy.Listener.Addr().String() // Get the address to restart it later.

	// Configure the client to connect to the proxy with default options.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := NewClient(testImpl, &ClientOptions{
		ProgressNotificationHandler: func(ctx context.Context, req *ProgressNotificationClientRequest) {
			notifications <- req.Params.Message
		},
	})
	clientSession, err := client.Connect(ctx, &StreamableClientTransport{
		Endpoint:   proxy.URL,
		MaxRetries: test.maxRetries,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() failed: %v", err)
	}
	t.Cleanup(func() {
		t.Log("Closing clientSession")
		clientSession.Close()
	})

	toolCallResult := make(chan error, 1)
	go func() {
		_, callErr := clientSession.CallTool(ctx, &CallToolParams{Name: "multiMessageTool"})
		toolCallResult <- callErr
	}()

	select {
	case <-serverReadyToKillProxy:
		t.Log("Server has sent the first two messages and is paused.")
	case <-ctx.Done():
		t.Fatalf("Context timed out before server was ready to kill proxy")
	}

	// We should always get the first two notifications.
	msgs := readNotifications(t, ctx, notifications, 2)
	sort.Strings(msgs) // notifications may arrive in either order
	want := []string{"msg1", "msg2"}
	if diff := cmp.Diff(want, msgs); diff != "" {
		t.Errorf("Recovered notifications mismatch (-want +got):\n%s", diff)
	}

	// Simulate a total network failure by closing the proxy.
	t.Log("--- Killing proxy to simulate network failure ---")
	proxy.CloseClientConnections()
	proxy.Close()
	close(serverClosed)

	// Simulate network recovery by restarting the proxy on the same address.
	t.Logf("--- Restarting proxy on %s ---", proxyAddr)
	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		t.Fatalf("Failed to listen on proxy address: %v", err)
	}

	restartedProxy := &http.Server{Handler: proxyHandler}
	go restartedProxy.Serve(listener)
	t.Cleanup(func() { restartedProxy.Close() })

	callErr := <-toolCallResult

	if test.wantRecovered {
		// If we've recovered, we should get all 4 notifications and the tool call
		// should have succeeded.
		msgs := readNotifications(t, ctx, notifications, 2)
		sort.Strings(msgs)
		want := []string{"msg3", "msg4"}
		if diff := cmp.Diff(want, msgs); diff != "" {
			t.Errorf("Recovered notifications mismatch (-want +got):\n%s", diff)
		}
		if callErr != nil {
			t.Errorf("CallTool failed unexpectedly: %v", err)
		}
	} else {
		// Otherwise, the call should fail.
		if callErr == nil {
			t.Errorf("CallTool succeeded unexpectedly")
		}
	}
}

func TestStreamableServerDisconnect(t *testing.T) {
	server := NewServer(testImpl, nil)

	// Test that client replayability allows the server to terminate incoming
	// requests immediately, and have the client replay them.

	// testStream exercises stream resumption by interleaving stream termination
	// with progress notifications.
	testStream := func(ctx context.Context, session *ServerSession, extra *RequestExtra) {
		// Close the stream before the first message. We should have sent an
		// initial priming message already, so the client will be able to replay
		extra.CloseSSEStream(CloseSSEStreamArgs{RetryAfter: 10 * time.Millisecond})
		session.NotifyProgress(ctx, &ProgressNotificationParams{Message: "msg1"})
		time.Sleep(20 * time.Millisecond)
		extra.CloseSSEStream(CloseSSEStreamArgs{RetryAfter: 10 * time.Millisecond}) // Closing twice should still be supported.
		session.NotifyProgress(ctx, &ProgressNotificationParams{Message: "msg2"})
	}

	AddTool(server, &Tool{Name: "disconnect"},
		func(ctx context.Context, req *CallToolRequest, args map[string]any) (*CallToolResult, map[string]any, error) {
			testStream(ctx, req.Session, req.Extra)
			return new(CallToolResult), nil, nil
		})

	server.AddPrompt(&Prompt{Name: "disconnect"}, func(ctx context.Context, req *GetPromptRequest) (*GetPromptResult, error) {
		testStream(ctx, req.Session, req.Extra)
		return nil, nil
	})

	tests := []struct {
		name   string
		doCall func(context.Context, *ClientSession) error
	}{
		{
			"tool",
			func(ctx context.Context, cs *ClientSession) error {
				_, err := cs.CallTool(ctx, &CallToolParams{Name: "disconnect"})
				return err
			},
		},
		{
			"prompt",
			func(ctx context.Context, cs *ClientSession) error {
				_, err := cs.GetPrompt(ctx, &GetPromptParams{Name: "disconnect"})
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notifications := make(chan string, 2)
			handler := NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, &StreamableHTTPOptions{
				EventStore: NewMemoryEventStore(nil), // support replayability
			})
			httpServer := httptest.NewServer(mustNotPanic(t, handler))
			defer httpServer.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			client := NewClient(testImpl, &ClientOptions{
				ProgressNotificationHandler: func(ctx context.Context, req *ProgressNotificationClientRequest) {
					notifications <- req.Params.Message
				},
			})
			clientSession, err := client.Connect(ctx, &StreamableClientTransport{
				Endpoint: httpServer.URL,
			}, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
			if err != nil {
				t.Fatalf("client.Connect() failed: %v", err)
			}
			defer clientSession.Close()

			if err = test.doCall(ctx, clientSession); err != nil {
				t.Fatalf("CallTool failed: %v", err)
			}

			got := readNotifications(t, ctx, notifications, 2)
			want := []string{"msg1", "msg2"}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("got unexpected notifications (-want +got):\n%s", diff)
			}
		})
	}
}

func TestServerTransportCleanup(t *testing.T) {
	nClient := 3

	var mu sync.Mutex
	var id int = -1 // session id starting from "0", "1", "2"...
	chans := make(map[string]chan struct{}, nClient)

	server := NewServer(testImpl, &ServerOptions{
		KeepAlive: 10 * time.Millisecond,
		GetSessionID: func() string {
			mu.Lock()
			defer mu.Unlock()
			id++
			if id == nClient {
				t.Errorf("creating more than %v session", nClient)
			}
			chans[fmt.Sprint(id)] = make(chan struct{}, 1)
			return fmt.Sprint(id)
		},
	})

	handler := NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, nil)
	handler.onTransportDeletion = func(sessionID string) {
		mu.Lock()
		ch := chans[sessionID]
		mu.Unlock()
		ch <- struct{}{}
	}

	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Spin up clients connect to the same server but refuse to ping request.
	for range nClient {
		client := NewClient(testImpl, nil)
		pingMiddleware := func(next MethodHandler) MethodHandler {
			return func(
				ctx context.Context,
				method string,
				req Request,
			) (Result, error) {
				if method == "ping" {
					return &emptyResult{}, errors.New("ping error")
				}
				return next(ctx, method, req)
			}
		}
		client.AddReceivingMiddleware(pingMiddleware)
		// Pin to 2025-11-25: this test counts server-side sessions, but
		// the 2026-07-28 client probes server/discover first against a
		// stateful HTTP server (which rejects the new protocol), opening
		// an extra HTTP connection and therefore an extra session. Pinning
		// to 2025-11-25 skips the discover probe.
		clientSession, err := client.Connect(ctx, &StreamableClientTransport{Endpoint: httpServer.URL}, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
		if err != nil {
			t.Fatalf("client.Connect() failed: %v", err)
		}
		t.Cleanup(func() { _ = clientSession.Close() })
	}

	mu.Lock()
	channels := slices.Collect(maps.Values(chans))
	mu.Unlock()

	for _, ch := range channels {
		select {
		case <-ctx.Done():
			t.Errorf("did not capture transport deletion event from all session in 10 seconds")
		case <-ch:
			t.Log("Received session transport deletion signal")
		}
	}

	handler.mu.Lock()
	if len(handler.sessions) != 0 {
		t.Errorf("want empty transports map, find %v entries from handler's transports map", len(handler.sessions))
	}
	handler.mu.Unlock()
}

// TestServerInitiatedSSE verifies that the persistent SSE connection remains
// open and can receive server-initiated events.
func TestServerInitiatedSSE(t *testing.T) {
	notifications := make(chan string)
	server := NewServer(testImpl, nil)

	httpServer := httptest.NewServer(mustNotPanic(t, NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, nil)))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := NewClient(testImpl, &ClientOptions{
		ToolListChangedHandler: func(context.Context, *ToolListChangedRequest) {
			notifications <- "toolListChanged"
		},
	})
	clientSession, err := client.Connect(ctx, &StreamableClientTransport{Endpoint: httpServer.URL}, nil)
	if err != nil {
		t.Fatalf("client.Connect() failed: %v", err)
	}
	defer clientSession.Close()
	AddTool(server, &Tool{Name: "testTool", InputSchema: &jsonschema.Schema{Type: "object"}},
		func(context.Context, *CallToolRequest, map[string]any) (*CallToolResult, any, error) {
			return &CallToolResult{}, nil, nil
		})
	receivedNotifications := readNotifications(t, ctx, notifications, 1)
	wantReceived := []string{"toolListChanged"}
	if diff := cmp.Diff(wantReceived, receivedNotifications); diff != "" {
		t.Errorf("Received notifications mismatch (-want +got):\n%s", diff)
	}
}

// Helper to read a specific number of notifications.
func readNotifications(t *testing.T, ctx context.Context, notifications chan string, count int) []string {
	t.Helper()
	var collectedNotifications []string
	for {
		select {
		case n := <-notifications:
			collectedNotifications = append(collectedNotifications, n)
			if len(collectedNotifications) == count {
				return collectedNotifications
			}
		case <-ctx.Done():
			if len(collectedNotifications) != count {
				t.Fatalf("readProgressNotifications(): did not receive expected notifications, got %d, want %d", len(collectedNotifications), count)
			}
			return collectedNotifications
		}
	}
}

// JSON-RPC message constructors.
func req(id int64, method string, params any) *jsonrpc.Request {
	r := &jsonrpc.Request{
		Method: method,
		Params: mustMarshal(params),
	}
	if id > 0 {
		r.ID = jsonrpc2.Int64ID(id)
	}
	return r
}

func completeCallToolResult() *CallToolResult {
	r := &CallToolResult{Content: []Content{}}
	r.resultType = resultTypeComplete
	return r
}

// completeCallToolResultWithServerInfo returns a completeCallToolResult
// annotated with the auto-populated [MetaKeyServerInfo] entry that servers
// add to every new-protocol response.
func completeCallToolResultWithServerInfo(impl *Implementation) *CallToolResult {
	r := completeCallToolResult()
	r.Meta = Meta{MetaKeyServerInfo: impl}
	return r
}

func resp(id int64, result any, err error) *jsonrpc.Response {
	return &jsonrpc.Response{
		ID:     jsonrpc2.Int64ID(id),
		Result: mustMarshal(result),
		Error:  err,
	}
}

func TestStreamableServerTransport(t *testing.T) {
	// This test checks detailed behavior of the streamable server transport, by
	// faking the behavior of a streamable client using a sequence of HTTP
	// requests.

	// Predefined steps, to avoid repetition below.
	initReq := req(1, methodInitialize, &InitializeParams{ProtocolVersion: protocolVersion20250618})
	initResp := resp(1, &InitializeResult{
		Capabilities: &ServerCapabilities{
			Logging: &LoggingCapabilities{},
			Tools:   &ToolCapabilities{ListChanged: true},
		},
		ProtocolVersion: protocolVersion20250618,
		ServerInfo:      &Implementation{Name: "testServer", Version: "v1.0.0"},
	}, nil)
	initializedMsg := req(0, notificationInitialized, &InitializedParams{})
	initialize := streamableRequest{
		method:         "POST",
		messages:       []jsonrpc.Message{initReq},
		wantStatusCode: http.StatusOK,
		wantMessages:   []jsonrpc.Message{initResp},
		wantSessionID:  true,
	}
	initialized := streamableRequest{
		method:         "POST",
		messages:       []jsonrpc.Message{initializedMsg},
		wantStatusCode: http.StatusAccepted,
	}

	// Protocol version 2025-11-25 variants, for testing prime/close events (SEP-1699).
	initReq20251125 := req(1, methodInitialize, &InitializeParams{ProtocolVersion: protocolVersion20251125})
	initResp20251125 := resp(1, &InitializeResult{
		Capabilities: &ServerCapabilities{
			Logging: &LoggingCapabilities{},
			Tools:   &ToolCapabilities{ListChanged: true},
		},
		ProtocolVersion: protocolVersion20251125,
		ServerInfo:      &Implementation{Name: "testServer", Version: "v1.0.0"},
	}, nil)
	initialize20251125 := streamableRequest{
		method:         "POST",
		messages:       []jsonrpc.Message{initReq20251125},
		wantStatusCode: http.StatusOK,
		wantMessages:   []jsonrpc.Message{initResp20251125},
		wantSessionID:  true,
	}
	initialized20251125 := streamableRequest{
		method:         "POST",
		headers:        http.Header{protocolVersionHeader: {protocolVersion20251125}},
		messages:       []jsonrpc.Message{initializedMsg},
		wantStatusCode: http.StatusAccepted,
	}

	tests := []struct {
		name         string
		replay       bool                                                // if set, use a MemoryEventStore to enable replay
		tool         func(*testing.T, context.Context, *CallToolRequest) // if set, called during execution
		requests     []streamableRequest
		wantSessions int // number of sessions expected after the test
	}{
		{
			name: "basic",
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:         "POST",
					messages:       []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
				},
			},
			wantSessions: 1,
		},
		{
			name: "uninitialized",
			requests: []streamableRequest{
				{
					method:             "POST",
					messages:           []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:     http.StatusOK,
					wantBodyContaining: "invalid during session initialization",
				},
			},
			wantSessions: 0,
		},
		{
			name: "content type headers",
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					// Request with incorrect Content-Type should be rejected.
					method:         "POST",
					headers:        http.Header{"Content-Type": {"text/plain"}},
					messages:       []jsonrpc.Message{req(3, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusUnsupportedMediaType,
				},
				{
					// Request with empty Content-Type should be rejected.
					method:         "POST",
					headers:        http.Header{"Content-Type": {""}},
					messages:       []jsonrpc.Message{req(4, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusUnsupportedMediaType,
				},
				{
					// Correct Content-Type should pass.
					method:         "POST",
					headers:        http.Header{"Content-Type": {"application/json"}},
					messages:       []jsonrpc.Message{req(5, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(5, &CallToolResult{Content: []Content{}}, nil)},
				},
				{
					// Correct Content-Type with parameters should pass.
					method:         "POST",
					headers:        http.Header{"Content-Type": {"application/json; charset=utf-8"}},
					messages:       []jsonrpc.Message{req(5, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(5, &CallToolResult{Content: []Content{}}, nil)},
				},
			},
			wantSessions: 1,
		},
		{
			name: "accept headers",
			requests: []streamableRequest{
				initialize,
				initialized,
				// Test various accept headers.
				{
					method:         "POST",
					headers:        http.Header{"Accept": {"text/plain", "application/*"}},
					messages:       []jsonrpc.Message{req(3, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusBadRequest, // missing text/event-stream
				},
				{
					method:         "POST",
					headers:        http.Header{"Accept": {"text/event-stream"}},
					messages:       []jsonrpc.Message{req(3, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusBadRequest, // missing application/json
				},
				{
					method:         "POST",
					headers:        http.Header{"Accept": {"text/plain", "*/*"}},
					messages:       []jsonrpc.Message{req(4, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(4, &CallToolResult{Content: []Content{}}, nil)},
				},
				{
					method:         "POST",
					headers:        http.Header{"Accept": {"application/json;charset=utf-8, text/event-stream"}},
					messages:       []jsonrpc.Message{req(5, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(5, &CallToolResult{Content: []Content{}}, nil)},
				},
				{
					method:         "POST",
					headers:        http.Header{"Accept": {"application/json;charset=utf-8", "text/event-stream"}},
					messages:       []jsonrpc.Message{req(6, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(6, &CallToolResult{Content: []Content{}}, nil)},
				},
				{
					method:         "POST",
					headers:        http.Header{"Accept": {"text/*, application/*"}},
					messages:       []jsonrpc.Message{req(7, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages:   []jsonrpc.Message{resp(7, &CallToolResult{Content: []Content{}}, nil)},
				},
			},
			wantSessions: 1,
		},
		{
			name: "protocol version headers",
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:             "POST",
					headers:            http.Header{"mcp-protocol-version": {"2025-01-01"}}, // an invalid protocol version
					messages:           []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:     http.StatusBadRequest,
					wantBodyContaining: "2025-03-26", // a supported version
					wantSessionID:      false,        // could be true, but shouldn't matter
				},
			},
			wantSessions: 1,
		},
		{
			name: "batch rejected on 2025-06-18",
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method: "POST",
					// Explicitly set the protocol version header
					headers: http.Header{"MCP-Protocol-Version": {"2025-06-18"}},
					// Two messages => batch. Expect reject.
					messages: []jsonrpc.Message{
						req(101, "tools/call", &CallToolParams{Name: "tool"}),
						req(102, "tools/call", &CallToolParams{Name: "tool"}),
					},
					wantStatusCode:     http.StatusBadRequest,
					wantBodyContaining: "batch",
				},
			},
			wantSessions: 1,
		},
		{
			name: "batch accepted on 2025-03-26",
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:  "POST",
					headers: http.Header{"MCP-Protocol-Version": {"2025-03-26"}},
					// Two messages => batch. Expect OK with two responses in order.
					messages: []jsonrpc.Message{
						// Note: only include one request here, because responses are not
						// necessarily sorted.
						req(201, "tools/call", &CallToolParams{Name: "tool"}),
						req(0, "notifications/roots/list_changed", &RootsListChangedParams{}),
					},
					wantStatusCode: http.StatusOK,
					wantMessages: []jsonrpc.Message{
						resp(201, &CallToolResult{Content: []Content{}}, nil),
					},
				},
			},
			wantSessions: 1,
		},
		{
			name: "tool notification",
			tool: func(t *testing.T, ctx context.Context, req *CallToolRequest) {
				// Send an arbitrary notification.
				if err := req.Session.NotifyProgress(ctx, &ProgressNotificationParams{}); err != nil {
					t.Errorf("Notify failed: %v", err)
				}
			},
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method: "POST",
					messages: []jsonrpc.Message{
						req(2, "tools/call", &CallToolParams{Name: "tool"}),
					},
					wantStatusCode: http.StatusOK,
					wantMessages: []jsonrpc.Message{
						req(0, "notifications/progress", &ProgressNotificationParams{}),
						resp(2, &CallToolResult{Content: []Content{}}, nil),
					},
				},
			},
			wantSessions: 1,
		},
		{
			name: "tool upcall",
			tool: func(t *testing.T, ctx context.Context, req *CallToolRequest) {
				// Make an arbitrary call.
				if _, err := req.Session.ListRoots(ctx, &ListRootsParams{}); err != nil {
					t.Errorf("Call failed: %v", err)
				}
			},
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:    "POST",
					onRequest: 1,
					messages: []jsonrpc.Message{
						resp(1, &ListRootsResult{}, nil),
					},
					wantStatusCode: http.StatusAccepted,
				},
				{
					method: "POST",
					messages: []jsonrpc.Message{
						req(2, "tools/call", &CallToolParams{Name: "tool"}),
					},
					wantStatusCode: http.StatusOK,
					wantMessages: []jsonrpc.Message{
						req(1, "roots/list", &ListRootsParams{}),
						resp(2, &CallToolResult{Content: []Content{}}, nil),
					},
				},
			},
			wantSessions: 1,
		},
		{
			name: "background",
			// Enabling replay is necessary here because the standalone "GET" request
			// is fully asynchronous. Replay is needed to guarantee message delivery.
			//
			// TODO(rfindley): this should no longer be necessary.
			replay: true,
			tool: func(t *testing.T, _ context.Context, req *CallToolRequest) {
				// Perform operations on a background context, and ensure the client
				// receives it.
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				if err := req.Session.NotifyProgress(ctx, &ProgressNotificationParams{}); err != nil {
					t.Errorf("Notify failed: %v", err)
				}
				// TODO(rfindley): finish implementing logging.
				// if err := ss.LoggingMessage(ctx, &LoggingMessageParams{}); err != nil {
				// 	t.Errorf("Logging failed: %v", err)
				// }
				if _, err := req.Session.ListRoots(ctx, &ListRootsParams{}); err != nil {
					t.Errorf("ListRoots failed: %v", err)
				}
			},
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:    "POST",
					onRequest: 1,
					messages: []jsonrpc.Message{
						resp(1, &ListRootsResult{}, nil),
					},
					wantStatusCode: http.StatusAccepted,
				},
				{
					method:         "GET",
					async:          true,
					wantStatusCode: http.StatusOK,
					closeAfter:     2,
					wantMessages: []jsonrpc.Message{
						req(0, "notifications/progress", &ProgressNotificationParams{}),
						req(1, "roots/list", &ListRootsParams{}),
					},
				},
				{
					method: "POST",
					messages: []jsonrpc.Message{
						req(2, "tools/call", &CallToolParams{Name: "tool"}),
					},
					wantStatusCode: http.StatusOK,
					wantMessages: []jsonrpc.Message{
						resp(2, &CallToolResult{Content: []Content{}}, nil),
					},
				},
				{
					method:         "DELETE",
					wantStatusCode: http.StatusNoContent,
					// Delete request expects 204 No Content with empty body. So override
					// the default "accept: application/json, text/event-stream" header.
					headers: map[string][]string{"Accept": nil},
				},
			},
			wantSessions: 0, // session deleted
		},
		{
			name:   "no priming message on old protocol",
			replay: true,
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:                "POST",
					messages:              []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:        http.StatusOK,
					wantMessages:          []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
					wantBodyNotContaining: "prime",
				},
			},
			wantSessions: 1,
		},
		{
			name:   "no close message on old protocol",
			replay: true,
			tool: func(t *testing.T, _ context.Context, req *CallToolRequest) {
				req.Extra.CloseSSEStream(CloseSSEStreamArgs{RetryAfter: time.Millisecond})
			},
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:                "POST",
					messages:              []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:        http.StatusOK,
					wantMessages:          []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
					wantBodyNotContaining: "close",
				},
			},
			wantSessions: 1,
		},
		{
			name:   "priming message on 2025-11-25",
			replay: true,
			requests: []streamableRequest{
				initialize20251125,
				initialized20251125,
				{
					method:             "POST",
					headers:            http.Header{protocolVersionHeader: {protocolVersion20251125}},
					messages:           []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:     http.StatusOK,
					wantMessages:       []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
					wantBodyContaining: "prime",
				},
			},
			wantSessions: 1,
		},
		{
			name:   "close message on 2025-11-25",
			replay: true,
			tool: func(t *testing.T, _ context.Context, req *CallToolRequest) {
				req.Extra.CloseSSEStream(CloseSSEStreamArgs{RetryAfter: time.Millisecond})
			},
			requests: []streamableRequest{
				initialize20251125,
				initialized20251125,
				{
					method:                "POST",
					headers:               http.Header{protocolVersionHeader: {protocolVersion20251125}},
					messages:              []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:        http.StatusOK,
					wantMessages:          []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
					wantBodyContaining:    "close",
					wantBodyNotContaining: "result",
				},
			},
			wantSessions: 1,
		},
		{
			name:   "no close message",
			replay: true,
			tool: func(t *testing.T, _ context.Context, req *CallToolRequest) {
				req.Extra.CloseSSEStream(CloseSSEStreamArgs{})
			},
			requests: []streamableRequest{
				initialize,
				initialized,
				{
					method:                "POST",
					messages:              []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode:        http.StatusOK,
					wantMessages:          []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
					wantBodyNotContaining: "close",
				},
			},
			wantSessions: 1,
		},
		{
			name: "errors",
			requests: []streamableRequest{
				{
					method:         "PUT",
					wantStatusCode: http.StatusMethodNotAllowed,
				},
				{
					method:         "DELETE",
					wantStatusCode: http.StatusBadRequest,
				},
				{
					method:         "POST",
					messages:       []jsonrpc.Message{req(1, "notamethod", nil)},
					wantStatusCode: http.StatusBadRequest, // notamethod is an invalid method
				},
				{
					method:         "POST",
					messages:       []jsonrpc.Message{req(0, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusBadRequest, // tools/call must have an ID
				},
				{
					method:         "POST",
					messages:       []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "tool"})},
					wantStatusCode: http.StatusOK,
					wantMessages: []jsonrpc.Message{resp(2, nil, &jsonrpc.Error{
						Message: `method "tools/call" is invalid during session initialization`,
					})},
				},
			},
			wantSessions: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create a server containing a single tool, which runs the test tool
			// behavior, if any.
			server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
			server.AddTool(
				&Tool{Name: "tool", InputSchema: &jsonschema.Schema{Type: "object"}},
				func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
					if test.tool != nil {
						test.tool(t, ctx, req)
					}
					return &CallToolResult{}, nil
				})

			opts := &StreamableHTTPOptions{}
			if test.replay {
				opts.EventStore = NewMemoryEventStore(nil)
			}
			// Start the streamable handler.
			handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, opts)
			defer handler.closeAll()

			testStreamableHandler(t, handler, test.requests)
			if got := len(slices.Collect(server.Sessions())); got != test.wantSessions {
				t.Errorf("after test, got %d sessions, want %d", got, test.wantSessions)
			}
		})
	}
}

func testStreamableHandler(t *testing.T, handler http.Handler, requests []streamableRequest) {
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	// blocks records request blocks by jsonrpc. ID.
	//
	// When an OnRequest step is encountered, it waits on the corresponding
	// block. When a request with that ID is received, the block is closed.
	var mu sync.Mutex
	blocks := make(map[int64]chan struct{})
	for _, req := range requests {
		if req.onRequest > 0 {
			blocks[req.onRequest] = make(chan struct{})
		}
	}

	// signal when all synchronous requests have executed, so we can fail
	// async requests that are blocked.
	syncRequestsDone := make(chan struct{})

	// To avoid complicated accounting for session ID, just set the first
	// non-empty session ID from a response.
	var sessionID atomic.Value
	sessionID.Store("")

	// doStep executes a single step.
	doStep := func(t *testing.T, i int, request streamableRequest) {
		if request.onRequest > 0 {
			// Block the step until we've received the server->client request.
			mu.Lock()
			block := blocks[request.onRequest]
			mu.Unlock()
			select {
			case <-block:
			case <-syncRequestsDone:
				t.Errorf("after all sync requests are complete, request still blocked on %d", request.onRequest)
				return
			}
		}

		// Collect messages received during this request, unblock other steps
		// when requests are received.
		var got []jsonrpc.Message
		out := make(chan jsonrpc.Message)
		// Cancel the step if we encounter a request that isn't going to be
		// handled.
		//
		// Also, add a timeout (hopefully generous).
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		var wg sync.WaitGroup
		wg.Go(func() {

			for m := range out {
				if req, ok := m.(*jsonrpc.Request); ok && req.IsCall() {
					// Encountered a server->client request. We should have a
					// response queued. Otherwise, we may deadlock.
					mu.Lock()
					if block, ok := blocks[req.ID.Raw().(int64)]; ok {
						close(block)
					} else {
						t.Errorf("no queued response for %v", req.ID)
						cancel()
					}
					mu.Unlock()
				}
				got = append(got, m)
				if request.closeAfter > 0 && len(got) == request.closeAfter {
					cancel()
				}
			}
		})

		gotSessionID, gotStatusCode, gotBody, err := request.do(ctx, httpServer.URL, sessionID.Load().(string), out)

		// Don't fail on cancelled requests: error (if any) is handled
		// elsewhere.
		if err != nil && ctx.Err() == nil {
			t.Fatal(err)
		}

		if gotStatusCode != request.wantStatusCode {
			t.Errorf("request #%d: got status %d, want %d", i, gotStatusCode, request.wantStatusCode)
		}
		if got := gotSessionID != ""; got != request.wantSessionID {
			t.Errorf("request #%d: got session id: %t, want %t", i, got, request.wantSessionID)
		}
		wg.Wait()

		if request.wantBodyContaining != "" || request.wantBodyNotContaining != "" {
			body := string(gotBody)
			if request.wantBodyContaining != "" && !strings.Contains(body, request.wantBodyContaining) {
				t.Errorf("body does not contain %q:\n%s", request.wantBodyContaining, body)
			}
			if request.wantBodyNotContaining != "" && strings.Contains(body, request.wantBodyNotContaining) {
				t.Errorf("body contains %q:\n%s", request.wantBodyNotContaining, body)
			}
		} else {
			transform := cmpopts.AcyclicTransformer("jsonrpcid", func(id jsonrpc.ID) any { return id.Raw() })
			if diff := cmp.Diff(request.wantMessages, got, transform); diff != "" {
				t.Errorf("request #%d: received unexpected messages (-want +got):\n%s", i, diff)
			}
		}
		sessionID.CompareAndSwap("", gotSessionID)
	}

	var wg sync.WaitGroup
	for i, request := range requests {
		if request.async || request.onRequest > 0 {
			wg.Go(func() {
				doStep(t, i, request)
			})
		} else {
			doStep(t, i, request)
		}
	}

	// Fail any blocked responses if they weren't needed by a synchronous
	// request.
	close(syncRequestsDone)

	wg.Wait()
}

// A streamableRequest describes a single streamable HTTP request, consisting
// of a request payload and expected response.
type streamableRequest struct {
	// If onRequest is > 0, this step only executes after a request with the
	// given ID is received.
	//
	// All onRequest steps must occur before the step that creates the request.
	//
	// To avoid tests hanging when there's a bug, it's expected that this
	// request is received in the course of a *synchronous* request to the
	// server (otherwise, we wouldn't be able to terminate the test without
	// analyzing a dependency graph).
	onRequest int64
	// If set, async causes the step to run asynchronously to other steps.
	// Redundant with OnRequest: all OnRequest steps are asynchronous.
	async bool

	// Request attributes
	method   string            // HTTP request method (required)
	headers  http.Header       // additional headers to set, overlaid on top of the default headers
	messages []jsonrpc.Message // messages to send

	closeAfter            int               // if nonzero, close after receiving this many messages
	wantStatusCode        int               // expected status code
	wantBodyContaining    string            // if set, expect the response body to contain this text; overrides wantMessages
	wantBodyNotContaining string            // if set, a negative assertion on the body; overrides wantMessages
	wantMessages          []jsonrpc.Message // expected messages to receive; ignored if wantBodyContaining is set
	wantSessionID         bool              // whether or not a session ID is expected in the response
}

// streamingRequest makes a request to the given streamable server with the
// given url, sessionID, and method.
//
// If provided, the in messages are encoded in the request body. A single
// message is encoded as a JSON object. Multiple messages are batched as a JSON
// array.
//
// Any received messages are sent to the out channel, which is closed when the
// request completes.
//
// Returns the sessionID and http status code from the response. If an error is
// returned, sessionID and status code may still be set if the error occurs
// after the response headers have been received.
func (s streamableRequest) do(ctx context.Context, serverURL, sessionID string, out chan<- jsonrpc.Message) (string, int, []byte, error) {
	defer close(out)

	var body []byte
	if len(s.messages) == 1 {
		data, err := jsonrpc2.EncodeMessage(s.messages[0])
		if err != nil {
			return "", 0, nil, fmt.Errorf("encoding message: %w", err)
		}
		body = data
	} else {
		var rawMsgs []json.RawMessage
		for _, msg := range s.messages {
			data, err := jsonrpc2.EncodeMessage(msg)
			if err != nil {
				return "", 0, nil, fmt.Errorf("encoding message: %w", err)
			}
			rawMsgs = append(rawMsgs, data)
		}
		data, err := json.Marshal(rawMsgs)
		if err != nil {
			return "", 0, nil, fmt.Errorf("marshaling batch: %w", err)
		}
		body = data
	}

	req, err := http.NewRequestWithContext(ctx, s.method, serverURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, nil, fmt.Errorf("creating request: %w", err)
	}
	if sessionID != "" {
		req.Header.Set(sessionIDHeader, sessionID)
	}
	if s.method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json, text/event-stream")
	maps.Copy(req.Header, s.headers)

	if req.Header.Get("Content-Type") == "" {
		req.Header.Del("Content-Type")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	newSessionID := resp.Header.Get(sessionIDHeader)

	contentType := baseMediaType(resp.Header.Get("Content-Type"))
	var respBody []byte
	if contentType == "text/event-stream" {
		r := readerInto{resp.Body, new(bytes.Buffer)}
		for evt, err := range scanEvents(r) {
			if err != nil {
				return newSessionID, resp.StatusCode, nil, fmt.Errorf("reading events: %v", err)
			}
			if evt.Name == "" || evt.Name == "message" { // ordinary message
				// TODO(rfindley): do we need to check evt.name?
				// Does the MCP spec say anything about this?
				msg, err := jsonrpc2.DecodeMessage(evt.Data)
				if err != nil {
					return newSessionID, resp.StatusCode, nil, fmt.Errorf("decoding message: %w", err)
				}
				out <- msg
			}
		}
		respBody = r.w.Bytes()
	} else if contentType == "application/json" {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return newSessionID, resp.StatusCode, nil, fmt.Errorf("reading json body: %w", err)
		}
		respBody = data
		msg, err := jsonrpc2.DecodeMessage(data)
		if err != nil {
			return newSessionID, resp.StatusCode, nil, fmt.Errorf("decoding message: %w", err)
		}
		out <- msg
	} else {
		respBody, err = io.ReadAll(resp.Body)
		if err != nil {
			return newSessionID, resp.StatusCode, nil, fmt.Errorf("reading response: %v", err)
		}
	}

	return newSessionID, resp.StatusCode, respBody, nil
}

// readerInto is an io.Reader that writes any bytes read from r into w.
type readerInto struct {
	r io.Reader
	w *bytes.Buffer
}

// Read implements io.Reader.
func (r readerInto) Read(p []byte) (n int, err error) {
	n, err = r.r.Read(p)
	if err == nil || err == io.EOF {
		n2, err2 := r.w.Write(p[:n])
		if err2 != nil {
			return n, fmt.Errorf("failed to write: %v", err)
		}
		if n2 != n {
			return n, fmt.Errorf("short write: %d != %d", n2, n)
		}
	}
	return n, err
}

func mustMarshal(v any) json.RawMessage {
	if v == nil {
		return nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func TestEventID(t *testing.T) {
	tests := []struct {
		sid string
		idx int
	}{
		{"0", 0},
		{"0", 1},
		{"1", 0},
		{"1", 1},
		{"", 1},
		{"1234", 5678},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s_%d", test.sid, test.idx), func(t *testing.T) {
			eventID := formatEventID(test.sid, test.idx)
			gotSID, gotIdx, ok := parseEventID(eventID)
			if !ok {
				t.Fatalf("parseEventID(%q) failed, want ok", eventID)
			}
			if gotSID != test.sid || gotIdx != test.idx {
				t.Errorf("parseEventID(%q) = %s, %d, want %s, %d", eventID, gotSID, gotIdx, test.sid, test.idx)
			}
		})
	}

	invalid := []string{
		"",
		"_",
		"1_",
		"1_a",
		"1_-1",
	}

	for _, eventID := range invalid {
		t.Run(fmt.Sprintf("invalid_%q", eventID), func(t *testing.T) {
			if _, _, ok := parseEventID(eventID); ok {
				t.Errorf("parseEventID(%q) succeeded, want failure", eventID)
			}
		})
	}
}

func TestStreamableStateless(t *testing.T) {
	initReq := req(1, methodInitialize, &InitializeParams{})
	initResp := resp(1, &InitializeResult{
		Capabilities: &ServerCapabilities{
			Logging: &LoggingCapabilities{},
			Tools:   &ToolCapabilities{ListChanged: true},
		},
		ProtocolVersion: protocolVersion20251125,
		ServerInfo:      &Implementation{Name: "test", Version: "v1.0.0"},
	}, nil)
	// This version of sayHi expects
	// that request from our client).
	sayHi := func(ctx context.Context, req *CallToolRequest, args hiParams) (*CallToolResult, any, error) {
		if err := req.Session.Ping(ctx, nil); err == nil {
			// ping should fail, but not break the connection
			t.Errorf("ping succeeded unexpectedly")
		}
		return &CallToolResult{Content: []Content{&TextContent{Text: "hi " + args.Name}}}, nil, nil
	}

	requests := []streamableRequest{
		{
			method:         "POST",
			messages:       []jsonrpc.Message{initReq},
			wantStatusCode: http.StatusOK,
			wantMessages:   []jsonrpc.Message{initResp},
			wantSessionID:  false, // sessionless
		},
		{
			method:             "POST",
			wantStatusCode:     http.StatusOK,
			messages:           []jsonrpc.Message{req(1, "tools/list", struct{}{})},
			wantBodyContaining: "greet",
		},
		{
			method:         "GET",
			wantStatusCode: http.StatusMethodNotAllowed,
		},
		{
			method:         "POST",
			wantStatusCode: http.StatusOK,
			messages: []jsonrpc.Message{
				req(2, "tools/call", &CallToolParams{Name: "greet", Arguments: hiParams{Name: "World"}}),
			},
			wantMessages: []jsonrpc.Message{
				resp(2, &CallToolResult{
					Content: []Content{&TextContent{Text: "hi World"}},
				}, nil),
			},
		},
		{
			method:         "POST",
			wantStatusCode: http.StatusOK,
			messages: []jsonrpc.Message{
				req(2, "tools/call", &CallToolParams{Name: "greet", Arguments: hiParams{Name: "foo"}}),
			},
			wantMessages: []jsonrpc.Message{
				resp(2, &CallToolResult{
					Content: []Content{&TextContent{Text: "hi foo"}},
				}, nil),
			},
		},
	}

	testClientCompatibility := func(t *testing.T, handler http.Handler) {
		ctx := context.Background()
		httpServer := httptest.NewServer(mustNotPanic(t, handler))
		defer httpServer.Close()
		cs, err := NewClient(testImpl, nil).Connect(ctx, &StreamableClientTransport{Endpoint: httpServer.URL}, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { cs.Close() })
		res, err := cs.CallTool(ctx, &CallToolParams{Name: "greet", Arguments: hiParams{Name: "bar"}})
		if err != nil {
			t.Fatal(err)
		}
		if got, want := textContent(t, res), "hi bar"; got != want {
			t.Errorf("Result = %q, want %q", got, want)
		}
	}

	statelessHandler := NewStreamableHTTPHandler(func(*http.Request) *Server {
		server := NewServer(testImpl, nil)
		AddTool(server, &Tool{Name: "greet", Description: "say hi"}, sayHi)
		return server
	}, &StreamableHTTPOptions{
		Stateless: true,
	})

	testStreamableHandler(t, statelessHandler, requests)
	testClientCompatibility(t, statelessHandler)
}

func textContent(t *testing.T, res *CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("len(Content) = %d, want 1", len(res.Content))
	}
	text, ok := res.Content[0].(*TextContent)
	if !ok {
		t.Fatalf("Content[0] is %T, want *TextContent", res.Content[0])
	}
	return text.Text
}

func TestSessionHijackingPrevention(t *testing.T) {
	// This test verifies that sessions bound to a user ID cannot be accessed
	// by a different user (session hijacking prevention).
	ctx := context.Background()

	server := NewServer(testImpl, nil)
	streamHandler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, nil)

	// Use the bearer token directly as the user ID. This simulates how a real
	// verifier might extract a user ID from a JWT "sub" claim or introspection.
	verifier := func(_ context.Context, token string, _ *http.Request) (*auth.TokenInfo, error) {
		return &auth.TokenInfo{
			Scopes:     []string{"scope"},
			UserID:     token,
			Expiration: time.Date(5000, 1, 2, 3, 4, 5, 0, time.UTC),
		}, nil
	}
	handler := auth.RequireBearerToken(verifier, nil)(streamHandler)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	// Helper to send a JSON-RPC request as a given user.
	doRequest := func(msg jsonrpc.Message, sessionID, userID string) *http.Response {
		t.Helper()
		data, _ := jsonrpc2.EncodeMessage(msg)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer "+userID)
		if sessionID != "" {
			req.Header.Set(sessionIDHeader, sessionID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		return resp
	}

	// Create a session as user1.
	initReq := &jsonrpc.Request{Method: "initialize", ID: jsonrpc2.Int64ID(1)}
	initReq.Params, _ = json.Marshal(&InitializeParams{
		ProtocolVersion: protocolVersion20250618,
		ClientInfo:      &Implementation{Name: "test", Version: "1.0"},
	})
	resp := doRequest(initReq, "", "user1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("initialize failed with status %d: %s", resp.StatusCode, body)
	}
	sessionID := resp.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatal("no session ID in response")
	}

	pingReq := &jsonrpc.Request{Method: "ping", ID: jsonrpc2.Int64ID(2)}
	pingReq.Params, _ = json.Marshal(&PingParams{})

	// Try to access the session as user2 - should fail.
	resp2 := doRequest(pingReq, sessionID, "user2")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp2.Body)
		t.Errorf("expected status %d for user mismatch, got %d: %s", http.StatusForbidden, resp2.StatusCode, body)
	}

	// Access as original user1 should succeed.
	resp3 := doRequest(pingReq, sessionID, "user1")
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp3.Body)
		t.Errorf("expected status %d for matching user, got %d: %s", http.StatusOK, resp3.StatusCode, body)
	}
}

func TestStreamableGET(t *testing.T) {
	// This test checks the fix for problematic behavior described in #410:
	// Hanging GET headers should be written immediately, even if there are no
	// messages.
	server := NewServer(testImpl, nil)

	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	newReq := func(method string, msg jsonrpc.Message) *http.Request {
		var body io.Reader
		if msg != nil {
			data, err := jsonrpc2.EncodeMessage(msg)
			if err != nil {
				t.Fatal(err)
			}
			body = bytes.NewReader(data)
		}
		req, err := http.NewRequestWithContext(ctx, method, httpServer.URL, body)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Accept", "application/json, text/event-stream")
		if msg != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req
	}

	get1 := newReq(http.MethodGet, nil)
	resp, err := http.DefaultClient.Do(get1)
	if err != nil {
		t.Fatal(err)
	}
	// GET without session should return 400 Bad Request (not 405) because
	// GET is a valid method - it just requires a session ID.
	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("initial GET: got status %d, want %d", got, want)
	}
	defer resp.Body.Close()

	post1 := newReq(http.MethodPost, req(1, methodInitialize, &InitializeParams{}))
	resp, err = http.DefaultClient.Do(post1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		t.Errorf("initialize POST: got status %d, want %d; body:\n%s", got, want, string(body))
	}

	sessionID := resp.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatalf("initialized missing session ID")
	}

	get2 := newReq("GET", nil)
	get2.Header.Set(sessionIDHeader, sessionID)
	resp, err = http.DefaultClient.Do(get2)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusOK; got != want {
		t.Errorf("GET with session ID: got status %d, want %d", got, want)
	}

	t.Log("Sending final DELETE request to close session and release resources")
	del := newReq("DELETE", nil)
	del.Header.Set(sessionIDHeader, sessionID)
	resp, err = http.DefaultClient.Do(del)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if got, want := resp.StatusCode, http.StatusNoContent; got != want {
		t.Errorf("DELETE with session ID: got status %d, want %d", got, want)
	}
}

// TestStreamableMcpHeaderValidation tests the server-side Mcp-Method,
// Mcp-Name, and Mcp-Param header validation through the full HTTP handler,
// as specified in SEP-2243.
func TestStreamableMcpHeaderValidation(t *testing.T) {
	// Temporarily register the future version so the handler accepts it.

	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	server.AddTool(
		&Tool{Name: "my-tool", InputSchema: &jsonschema.Schema{Type: "object"}},
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return &CallToolResult{}, nil
		})
	server.AddTool(
		&Tool{
			Name: "execute_sql",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"region": map[string]any{
						"type":         "string",
						"x-mcp-header": "Region",
					},
					"query": map[string]any{
						"type": "string",
					},
				},
			},
		},
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return &CallToolResult{}, nil
		})

	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, &StreamableHTTPOptions{
		Stateless: true,
	})
	defer handler.closeAll()

	testMeta := Meta{
		MetaKeyProtocolVersion:    minVersionForStandardHeaders,
		MetaKeyClientInfo:         map[string]any{"name": "testClient", "version": "v1.0.0"},
		MetaKeyClientCapabilities: map[string]any{},
	}
	serverImpl := &Implementation{Name: "testServer", Version: "v1.0.0"}

	testStreamableHandler(t, handler, []streamableRequest{
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {minVersionForStandardHeaders},
				methodHeader:          {"tools/call"},
				nameHeader:            {"my-tool"},
			},
			messages:       []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Meta: testMeta, Name: "my-tool"})},
			wantStatusCode: http.StatusOK,
			wantMessages:   []jsonrpc.Message{resp(2, completeCallToolResultWithServerInfo(serverImpl), nil)},
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {minVersionForStandardHeaders},
				methodHeader:          {"prompts/get"},
				nameHeader:            {"my-tool"},
			},
			messages:           []jsonrpc.Message{req(3, "tools/call", &CallToolParams{Meta: testMeta, Name: "my-tool"})},
			wantStatusCode:     http.StatusBadRequest,
			wantBodyContaining: "Mcp-Method header value",
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {minVersionForStandardHeaders},
				methodHeader:          {"tools/call"},
				nameHeader:            {"wrong-tool"},
			},
			messages:           []jsonrpc.Message{req(4, "tools/call", &CallToolParams{Meta: testMeta, Name: "my-tool"})},
			wantStatusCode:     http.StatusBadRequest,
			wantBodyContaining: "Mcp-Name header value",
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {minVersionForStandardHeaders},
				methodHeader:          {"TOOLS/CALL"},
				nameHeader:            {"my-tool"},
			},
			messages:           []jsonrpc.Message{req(5, "tools/call", &CallToolParams{Meta: testMeta, Name: "my-tool"})},
			wantStatusCode:     http.StatusBadRequest,
			wantBodyContaining: "Mcp-Method header value",
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {minVersionForStandardHeaders},
				methodHeader:          {"tools/call"},
				nameHeader:            {"my-tool"},
			},
			messages:       []jsonrpc.Message{req(6, "tools/call", &CallToolParams{Meta: testMeta, Name: "my-tool"})},
			wantStatusCode: http.StatusOK,
			wantMessages:   []jsonrpc.Message{resp(6, completeCallToolResultWithServerInfo(serverImpl), nil)},
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader:        {minVersionForStandardHeaders},
				methodHeader:                 {"tools/call"},
				nameHeader:                   {"execute_sql"},
				paramHeaderPrefix + "Region": {"us-west1"},
			},
			messages: []jsonrpc.Message{req(7, "tools/call", &CallToolParams{
				Meta:      testMeta,
				Name:      "execute_sql",
				Arguments: map[string]any{"region": "us-west1", "query": "SELECT 1"},
			})},
			wantStatusCode: http.StatusOK,
			wantMessages:   []jsonrpc.Message{resp(7, completeCallToolResultWithServerInfo(serverImpl), nil)},
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader:        {minVersionForStandardHeaders},
				methodHeader:                 {"tools/call"},
				nameHeader:                   {"execute_sql"},
				paramHeaderPrefix + "Region": {"eu-central1"},
			},
			messages: []jsonrpc.Message{req(8, "tools/call", &CallToolParams{
				Meta:      testMeta,
				Name:      "execute_sql",
				Arguments: map[string]any{"region": "us-west1"},
			})},
			wantStatusCode:     http.StatusBadRequest,
			wantBodyContaining: "header mismatch",
		},
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {minVersionForStandardHeaders},
				methodHeader:          {"tools/call"},
				nameHeader:            {"execute_sql"},
			},
			messages: []jsonrpc.Message{req(9, "tools/call", &CallToolParams{
				Meta:      testMeta,
				Name:      "execute_sql",
				Arguments: map[string]any{"region": "us-west1"},
			})},
			wantStatusCode:     http.StatusBadRequest,
			wantBodyContaining: "missing",
		},
	})
}

// TestStreamableMcpHeaderValidationErrorFormat verifies that header
// validation errors return a JSON-RPC error with code -32020 and
// Content-Type application/json, per SEP-2243.
func TestStreamableMcpHeaderValidationErrorFormat(t *testing.T) {

	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	server.AddTool(
		&Tool{Name: "my-tool", InputSchema: &jsonschema.Schema{Type: "object"}},
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return &CallToolResult{}, nil
		})

	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, &StreamableHTTPOptions{
		Stateless: true,
	})
	defer handler.closeAll()

	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	// Use the MCP client with a custom RoundTripper to inject a bad header.
	var toolCallResp *http.Response
	var toolCallRespBody []byte

	customClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var originalMethodHeader string
			if req.Header.Get(methodHeader) == "tools/call" {
				originalMethodHeader = req.Header.Get(methodHeader)
				req.Header.Set(methodHeader, "wrong-method")
			}
			resp, err := http.DefaultTransport.RoundTrip(req)
			if err == nil && originalMethodHeader == "tools/call" {
				toolCallResp = resp
				toolCallRespBody, _ = io.ReadAll(resp.Body)
			}
			return resp, err
		}),
	}

	clientTransport := &StreamableClientTransport{
		Endpoint:   httpServer.URL,
		HTTPClient: customClient,
	}

	client := NewClient(&Implementation{Name: "testClient", Version: "v1.0.0"}, nil)
	ctx := context.Background()
	session, err := client.Connect(ctx, clientTransport, &ClientSessionOptions{protocolVersion: minVersionForStandardHeaders})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	_, err = session.CallTool(ctx, &CallToolParams{Name: "my-tool"})
	// We expect an error because the server should reject it.
	if err == nil {
		t.Error("CallTool succeeded unexpectedly")
	}

	if toolCallResp == nil {
		t.Fatal("no response captured")
	}

	// Verify HTTP status code.
	if toolCallResp.StatusCode != http.StatusBadRequest {
		t.Errorf("status code = %d, want %d", toolCallResp.StatusCode, http.StatusBadRequest)
	}

	// Verify Content-Type.
	if baseMediaType(toolCallResp.Header.Get("Content-Type")) != "application/json" {
		t.Errorf("Content-Type = %q, want %q", baseMediaType(toolCallResp.Header.Get("Content-Type")), "application/json")
	}

	// Verify JSON-RPC error body contains error code -32020.
	msg, err := jsonrpc2.DecodeMessage(toolCallRespBody)
	if err != nil {
		t.Fatalf("failed to decode message: %v", err)
	}
	resp, ok := msg.(*jsonrpc2.Response)
	if !ok {
		t.Fatalf("expected *jsonrpc2.Response, got %T", msg)
	}
	var wireErr *jsonrpc2.WireError
	if !errors.As(resp.Error, &wireErr) {
		t.Fatalf("expected *jsonrpc2.WireError, got %T", resp.Error)
	}
	if wireErr.Code != CodeHeaderMismatch {
		t.Errorf("wireErr.Code = %d, want %d", wireErr.Code, CodeHeaderMismatch)
	}
	if !strings.Contains(wireErr.Message, "Mcp-Method header value") {
		t.Errorf("wireErr.Message = %q, want it to contain %q", wireErr.Message, "Mcp-Method header value")
	}
}

// TestStreamableMcpHeaderVersionGating verifies that header validation
// is skipped for protocol versions older than minVersionForStandardHeaders.
func TestStreamableMcpHeaderVersionGating(t *testing.T) {
	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	server.AddTool(
		&Tool{Name: "my-tool", InputSchema: &jsonschema.Schema{Type: "object"}},
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return &CallToolResult{}, nil
		})

	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, nil)
	defer handler.closeAll()

	initReq := req(1, methodInitialize, &InitializeParams{ProtocolVersion: protocolVersion20251125})
	initResp := resp(1, &InitializeResult{
		Capabilities: &ServerCapabilities{
			Logging: &LoggingCapabilities{},
			Tools:   &ToolCapabilities{ListChanged: true},
		},
		ProtocolVersion: protocolVersion20251125,
		ServerInfo:      &Implementation{Name: "testServer", Version: "v1.0.0"},
	}, nil)

	testStreamableHandler(t, handler, []streamableRequest{
		{
			method:         "POST",
			messages:       []jsonrpc.Message{initReq},
			wantStatusCode: http.StatusOK,
			wantMessages:   []jsonrpc.Message{initResp},
			wantSessionID:  true,
		},
		{
			method:         "POST",
			headers:        http.Header{protocolVersionHeader: {protocolVersion20251125}},
			messages:       []jsonrpc.Message{req(0, notificationInitialized, &InitializedParams{})},
			wantStatusCode: http.StatusAccepted,
		},
		// Requests with deliberately wrong MCP headers should still succeed
		// because the protocol version is too old for validation.
		{
			method: "POST",
			headers: http.Header{
				protocolVersionHeader: {protocolVersion20251125},
				methodHeader:          {"wrong-method"},
				nameHeader:            {"wrong-name"},
			},
			messages:       []jsonrpc.Message{req(2, "tools/call", &CallToolParams{Name: "my-tool"})},
			wantStatusCode: http.StatusOK,
			wantMessages:   []jsonrpc.Message{resp(2, &CallToolResult{Content: []Content{}}, nil)},
		},
	})
}

// TestStreamableParamHeadersClientSetsHeaders verifies that the client sets
// Mcp-Param-* headers on tool calls when the tool has x-mcp-header annotations.
func TestStreamableParamHeadersClientSetsHeaders(t *testing.T) {

	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	server.AddTool(
		&Tool{
			Name: "execute_sql",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"region": map[string]any{
						"type":         "string",
						"x-mcp-header": "Region",
					},
					"query": map[string]any{
						"type": "string",
					},
				},
			},
		},
		func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
			return &CallToolResult{Content: []Content{&TextContent{Text: "ok"}}}, nil
		})

	// Install a receiving middleware on the server that opts the tools/list
	// response caching by setting a positive ttlMs.
	server.AddReceivingMiddleware(func(next MethodHandler) MethodHandler {
		return func(ctx context.Context, method string, req Request) (Result, error) {
			res, err := next(ctx, method, req)
			if err == nil && method == methodListTools {
				if r, ok := res.(*ListToolsResult); ok {
					r.TTLMs = 60_000
				}
			}
			return res, err
		}
	})

	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, &StreamableHTTPOptions{
		Stateless: true,
	})
	defer handler.closeAll()
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	var capturedHeaders http.Header
	customClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Header.Get(methodHeader) == "tools/call" {
				capturedHeaders = req.Header.Clone()
			}
			return http.DefaultTransport.RoundTrip(req)
		}),
	}

	clientTransport := &StreamableClientTransport{
		Endpoint:   httpServer.URL,
		HTTPClient: customClient,
	}

	client := NewClient(&Implementation{Name: "testClient", Version: "v1.0.0"}, nil)
	ctx := context.Background()
	session, err := client.Connect(ctx, clientTransport, &ClientSessionOptions{protocolVersion: minVersionForStandardHeaders})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// ListTools to populate the tool cache (needed for param headers).
	// Pass a non-nil params so the SEP-2575 per-request _meta triple is
	// injected; injectMeta is a no-op when params is nil.
	if _, err := session.ListTools(ctx, &ListToolsParams{}); err != nil {
		t.Fatal(err)
	}

	_, err = session.CallTool(ctx, &CallToolParams{
		Name:      "execute_sql",
		Arguments: map[string]any{"region": "us-west1", "query": "SELECT 1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if capturedHeaders == nil {
		t.Fatal("no tool call headers captured")
	}
	if got := capturedHeaders.Get(methodHeader); got != "tools/call" {
		t.Errorf("Mcp-Method = %q, want %q", got, "tools/call")
	}
	if got := capturedHeaders.Get(nameHeader); got != "execute_sql" {
		t.Errorf("Mcp-Name = %q, want %q", got, "execute_sql")
	}
	if got := capturedHeaders.Get(paramHeaderPrefix + "Region"); got != "us-west1" {
		t.Errorf("Mcp-Param-Region = %q, want %q", got, "us-west1")
	}
}

// TestStreamableFilterValidToolsIntegration verifies that AddTool rejects
// tools with invalid x-mcp-header annotations at registration time, and
// that valid tools are returned by ListTools.
func TestStreamableFilterValidToolsIntegration(t *testing.T) {

	server := NewServer(&Implementation{Name: "testServer", Version: "v1.0.0"}, nil)
	noop := func(ctx context.Context, req *CallToolRequest) (*CallToolResult, error) {
		return &CallToolResult{}, nil
	}

	// Valid tool with correct x-mcp-header annotation.
	server.AddTool(&Tool{
		Name: "valid-tool",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"region": map[string]any{
					"type":         "string",
					"x-mcp-header": "Region",
				},
			},
		},
	}, noop)

	// Invalid tool: x-mcp-header on an array type should panic.
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("AddTool with invalid x-mcp-header annotation did not panic")
			}
		}()
		server.AddTool(&Tool{
			Name: "invalid-tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"items": map[string]any{
						"type":         "array",
						"x-mcp-header": "Items",
					},
				},
			},
		}, noop)
	}()

	// Tool with no x-mcp-header annotations (always valid).
	server.AddTool(&Tool{
		Name:        "plain-tool",
		InputSchema: &jsonschema.Schema{Type: "object"},
	}, noop)

	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, &StreamableHTTPOptions{
		Stateless: true,
	})
	defer handler.closeAll()
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	client := NewClient(&Implementation{Name: "testClient", Version: "v1.0.0"}, nil)
	ctx := context.Background()
	session, err := client.Connect(ctx, &StreamableClientTransport{
		Endpoint: httpServer.URL,
	}, &ClientSessionOptions{protocolVersion: minVersionForStandardHeaders})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()

	// Pass non-nil params so the SEP-2575 per-request _meta triple is
	// injected; injectMeta is a no-op when params is nil.
	result, err := session.ListTools(ctx, &ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}

	toolNames := make([]string, len(result.Tools))
	for i, tool := range result.Tools {
		toolNames[i] = tool.Name
	}
	sort.Strings(toolNames)

	wantNames := []string{"plain-tool", "valid-tool"}
	if !slices.Equal(toolNames, wantNames) {
		t.Errorf("ListTools returned %v, want %v", toolNames, wantNames)
	}
}

// TestStreamable405AllowHeader verifies RFC 9110 §15.5.6 compliance:
// 405 Method Not Allowed responses MUST include an Allow header.
func TestStreamable405AllowHeader(t *testing.T) {
	server := NewServer(testImpl, nil)

	tests := []struct {
		name       string
		stateless  bool
		method     string
		wantStatus int
		wantAllow  string
	}{
		{
			name:       "unsupported method stateful",
			stateless:  false,
			method:     "PUT",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET, POST, DELETE",
		},
		{
			name:       "GET in stateless mode",
			stateless:  true,
			method:     "GET",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "POST",
		},
		{
			// In stateless mode, only POST is supported.
			// DELETE returns 405, consistent with Allow: POST.
			name:       "DELETE without session stateless",
			stateless:  true,
			method:     "DELETE",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "POST",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &StreamableHTTPOptions{Stateless: tt.stateless}
			handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, opts)
			httpServer := httptest.NewServer(mustNotPanic(t, handler))
			defer httpServer.Close()

			req, err := http.NewRequest(tt.method, httpServer.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Accept", "application/json, text/event-stream")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if got := resp.StatusCode; got != tt.wantStatus {
				t.Errorf("status code: got %d, want %d", got, tt.wantStatus)
			}

			allow := resp.Header.Get("Allow")
			if allow != tt.wantAllow {
				t.Errorf("Allow header: got %q, want %q", allow, tt.wantAllow)
			}
		})
	}
}

// TestStreamableGETWithoutSession verifies that GET without session ID in stateful mode
// returns 400 Bad Request (not 405), since GET is a supported method that requires a session.
func TestStreamableGETWithoutSession(t *testing.T) {
	server := NewServer(testImpl, nil)
	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	req, err := http.NewRequest("GET", httpServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	// GET without session should return 400 Bad Request, not 405 Method Not Allowed,
	// because GET is a valid method - it just requires a session ID.
	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(string(body)), "Bad Request: GET requires an Mcp-Session-Id header"; got != want {
		t.Errorf("body: got %q, want %q", got, want)
	}
}

func TestStreamableGETWithoutEventStreamAccept(t *testing.T) {
	server := NewServer(testImpl, nil)
	handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	req, err := http.NewRequest("GET", httpServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Errorf("status code: got %d, want %d", got, want)
	}
	if got, want := strings.TrimSpace(string(body)), "Accept must contain 'text/event-stream' for GET requests"; got != want {
		t.Errorf("body: got %q, want %q", got, want)
	}
}

func TestBaseMediaType(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "empty", want: ""},
		{name: "json", value: "application/json", want: "application/json"},
		{name: "json with params", value: "Application/JSON; charset=utf-8", want: "application/json"},
		{name: "event stream with params", value: "Text/Event-Stream; charset=utf-8", want: "text/event-stream"},
		{name: "invalid", value: "application/json; charset", want: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := baseMediaType(test.value); got != test.want {
				t.Errorf("baseMediaType(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestStreamableClientContextPropagation(t *testing.T) {
	type contextKey string
	const testKey = contextKey("test-key")
	const testValue = "test-value"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx2 := context.WithValue(ctx, testKey, testValue)

	server := httptest.NewServer(mustNotPanic(t, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.Method {
		case "POST":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set(sessionIDHeader, "test-session")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26","capabilities":{},"serverInfo":{"name":"test","version":"1.0"}}}`))
		case "GET":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
		case "DELETE":
			w.WriteHeader(http.StatusNoContent)
		}
	})))
	defer server.Close()

	transport := &StreamableClientTransport{Endpoint: server.URL}
	conn, err := transport.Connect(ctx2)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer conn.Close()

	streamableConn, ok := conn.(*streamableClientConn)
	if !ok {
		t.Fatalf("Expected *streamableClientConn, got %T", conn)
	}

	if got := streamableConn.ctx.Value(testKey); got != testValue {
		t.Errorf("Context value not propagated: got %v, want %v", got, testValue)
	}

	if streamableConn.ctx.Done() == nil {
		t.Error("Connection context is not cancellable")
	}

	cancel()
	select {
	case <-streamableConn.ctx.Done():
		t.Errorf("cancelling the connection context after successful connection broke the connection")
	default:
	}
}

func TestStreamableSessionTimeout(t *testing.T) {
	// TODO: this test relies on timing and may be flaky.
	// Fixing with testing/synctest is challenging because it uses real I/O (via
	// httptest.NewServer).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	server := NewServer(testImpl, nil)

	deleted := make(chan string, 1)
	handler := NewStreamableHTTPHandler(
		func(req *http.Request) *Server { return server },
		&StreamableHTTPOptions{
			SessionTimeout: 50 * time.Millisecond,
		},
	)
	handler.onTransportDeletion = func(sessionID string) {
		deleted <- sessionID
	}

	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	// Connect a client to create a session.
	// Pin to 2025-11-25: under 2026-07-28 the client probes server/discover
	// first against a stateful HTTP server (which rejects the new
	// protocol), opening an extra HTTP connection and therefore an extra
	// session. Pinning to 2025-11-25 skips the discover probe.
	client := NewClient(testImpl, nil)
	session, err := client.Connect(ctx, &StreamableClientTransport{Endpoint: httpServer.URL}, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatalf("client.Connect() failed: %v", err)
	}
	defer session.Close()

	sessionID := session.ID()
	if sessionID == "" {
		t.Fatal("client session has empty ID")
	}

	// Verify the session exists on the server.
	serverSessions := slices.Collect(server.Sessions())
	if len(serverSessions) != 1 {
		t.Fatalf("got %d sessions, want 1", len(serverSessions))
	}
	if got := serverSessions[0].ID(); got != sessionID {
		t.Fatalf("server session is %q, want %q", got, sessionID)
	}

	// Test that (possibly concurrent) requests keep the session alive.
	//
	// Spin up two goroutines, each making a request every 10ms. These requests
	// should keep the server from timing out.
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			for range 20 {
				if _, err := session.ListTools(ctx, nil); err != nil {
					t.Errorf("ListTools failed: %v", err)
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}

	wg.Wait()

	// Wait for the session to be cleaned up.
	select {
	case deletedID := <-deleted:
		if deletedID != sessionID {
			t.Errorf("deleted session ID = %q, want %q", deletedID, sessionID)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for session cleanup")
	}

	// Verify the session is gone from both handler and server.
	handler.mu.Lock()
	if len(handler.sessions) != 0 {
		t.Errorf("handler.sessions is not empty; length %d", len(handler.sessions))
	}
	if ss := slices.Collect(server.Sessions()); len(ss) != 0 {
		t.Errorf("server.Sessions() is not empty; length %d", len(ss))
	}
	handler.mu.Unlock()
}

// mustNotPanic is a helper to enforce that test handlers do not panic (see
// issue #556).
func mustNotPanic(t *testing.T, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if r := recover(); r != nil {
				buf := make([]byte, 1<<20)
				n := runtime.Stack(buf, false)
				fmt.Fprintf(os.Stderr, "handler panic: %v\n\n%s", r, buf[:n])
				t.Errorf("handler panicked: %v", r)
			}
		}()
		h.ServeHTTP(w, req)
	})
}

// TestPingEventFiltering verifies that the streamable client correctly filters
// out SSE "ping" events, which are used for keep-alive but should not be
// treated as JSON-RPC messages.
//
// This test addresses issue #636: the client should skip non-"message" events
// according to the SSE specification. It tests the fix in processStream where
// events with evt.Name != "" && evt.Name != "message" are skipped.
func TestPingEventFiltering(t *testing.T) {
	// This test verifies the low-level processStream filtering.
	// We create a mock response with ping and message events.

	sseData := `event: ping
data: ping

event: message
id: 1
data: {"jsonrpc":"2.0","id":1,"result":{}}

event: ping
data: keepalive

`

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader([]byte(sseData))),
	}

	// Create a minimal streamableClientConn for testing
	ctx := t.Context()

	incoming := make(chan jsonrpc.Message, 10)
	done := make(chan struct{})

	conn := &streamableClientConn{
		ctx:      ctx,
		done:     done,
		incoming: incoming,
	}

	// Create a test request
	testReq := &jsonrpc.Request{
		ID:     jsonrpc2.Int64ID(1),
		Method: "test",
	}

	// Process the stream
	go conn.processStream(ctx, "test", resp, testReq)

	// Collect messages with timeout
	var messages []jsonrpc.Message
	timeout := time.After(1 * time.Second)

collectLoop:
	for {
		select {
		case msg := <-incoming:
			messages = append(messages, msg)
			// We expect only 1 message (the response), not the ping events
			if len(messages) >= 1 {
				break collectLoop
			}
		case <-timeout:
			break collectLoop
		}
	}

	// Verify we only received the actual message, not the ping events
	if len(messages) != 1 {
		t.Errorf("got %d messages, want 1 (ping events should be filtered)", len(messages))
		for i, msg := range messages {
			t.Logf("message %d: %T", i, msg)
		}
	}

	// Verify the message is the response
	if len(messages) > 0 {
		resp, ok := messages[0].(*jsonrpc.Response)
		if !ok {
			t.Errorf("first message is %T, want *jsonrpc.Response", messages[0])
		} else if resp.ID.Raw() != int64(1) {
			t.Errorf("response ID is %v, want 1", resp.ID.Raw())
		}
	}
}

// TestProcessStreamPrimingEvent verifies that the streamable client correctly ignores
// SSE events with empty data buffers, which are used as priming events (e.g. SEP-1699).
func TestProcessStreamPrimingEvent(t *testing.T) {
	// We create a mock response with a priming event (empty data, with an ID),
	// followed by a normal event.
	sseData := `id: 123

id: 124
data: {"jsonrpc":"2.0","id":1,"result":{}}

`

	ctx := t.Context()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(sseData)),
	}

	incoming := make(chan jsonrpc.Message, 10)
	done := make(chan struct{})

	conn := &streamableClientConn{
		ctx:      ctx,
		done:     done,
		incoming: incoming,
		failed:   make(chan struct{}),
		logger:   ensureLogger(nil),
	}

	lastID, _, clientClosed := conn.processStream(ctx, "test", resp, nil)

	if clientClosed {
		t.Fatalf("processStream was unexpectedly closed by client")
	}

	if lastID != "124" {
		t.Errorf("lastEventID = %q, want %q", lastID, "124")
	}

	select {
	case msg := <-incoming:
		if res, ok := msg.(*jsonrpc.Response); !(ok && res.ID == jsonrpc2.Int64ID(1)) {
			t.Errorf("got unexpected message: %v", msg)
		}
	default:
		t.Errorf("expected a JSON-RPC message to be produced")
	}
}

// TestScanEventsPingFiltering is a unit test for the low-level event scanning
// with ping events to verify scanEvents properly parses all event types.
func TestScanEventsPingFiltering(t *testing.T) {
	// Create SSE stream with mixed events
	sseData := `event: ping
data: ping

event: message
data: {"jsonrpc":"2.0","method":"test","params":{}}

event: ping
data: keepalive

event: message
data: {"jsonrpc":"2.0","method":"test2","params":{}}

`

	reader := strings.NewReader(sseData)
	var events []Event

	// Scan all events
	for evt, err := range scanEvents(reader) {
		if err != nil {
			if err != io.EOF {
				t.Fatalf("scanEvents error: %v", err)
			}
			break
		}
		events = append(events, evt)
	}

	// Verify we got all 4 events
	if len(events) != 4 {
		t.Fatalf("got %d events, want 4", len(events))
	}

	// Verify event types
	expectedNames := []string{"ping", "message", "ping", "message"}
	for i, evt := range events {
		if evt.Name != expectedNames[i] {
			t.Errorf("event %d: got name %q, want %q", i, evt.Name, expectedNames[i])
		}
	}

	// Verify that we can decode the message events but would fail on ping events
	for i, evt := range events {
		switch evt.Name {
		case "message":
			_, err := jsonrpc.DecodeMessage(evt.Data)
			if err != nil {
				t.Errorf("event %d: failed to decode message event: %v", i, err)
			}
		case "ping":
			// Ping events have non-JSON data and should fail decoding
			_, err := jsonrpc.DecodeMessage(evt.Data)
			if err == nil {
				t.Errorf("event %d: ping event unexpectedly decoded as valid JSON-RPC", i)
			}
		}
	}
}

func Test_ExportErrSessionMissing(t *testing.T) {
	ctx := context.Background()

	// 1. Setup server
	impl := &Implementation{Name: "test", Version: "1.0.0"}
	server := NewServer(impl, nil)
	handler := NewStreamableHTTPHandler(func(r *http.Request) *Server { return server }, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	// 2. Setup client
	clientTransport := &StreamableClientTransport{
		Endpoint: ts.URL,
	}
	client := NewClient(impl, nil)
	// Pin to 2025-11-25: under 2026-07-28 the client probes server/discover
	// first against a stateful HTTP server (which rejects the new
	// protocol), opening an extra HTTP connection and therefore an extra
	// session. Pinning to 2025-11-25 skips the discover probe.
	session, err := client.Connect(ctx, clientTransport, &ClientSessionOptions{protocolVersion: protocolVersion20251125})
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer session.Close()

	// 3. Manually invalidate session on server
	handler.mu.Lock()
	if len(handler.sessions) != 1 {
		handler.mu.Unlock()
		t.Fatalf("expected 1 session, got %d", len(handler.sessions))
	}
	for id := range handler.sessions {
		delete(handler.sessions, id)
	}
	handler.mu.Unlock()

	// 4. Try to call a tool (or any request)
	_, err = session.ListTools(ctx, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// 5. Verify it's ErrSessionMissing
	if !errors.Is(err, ErrSessionMissing) {
		t.Errorf("expected error to wrap ErrSessionMissing, got: %v", err)
	}
}

// TestStreamableLocalhostProtection verifies that DNS rebinding protection
// is automatically enabled for localhost servers.
func TestStreamableLocalhostProtection(t *testing.T) {
	server := NewServer(testImpl, nil)

	tests := []struct {
		name              string
		listenAddr        string // Address to listen on
		hostHeader        string // Host header in request
		disableProtection bool   // DisableLocalhostProtection setting
		wantStatus        int
	}{
		// Auto-enabled for localhost listeners (127.0.0.1).
		{
			name:              "127.0.0.1 accepts 127.0.0.1",
			listenAddr:        "127.0.0.1:0",
			hostHeader:        "127.0.0.1:1234",
			disableProtection: false,
			wantStatus:        http.StatusOK,
		},
		{
			name:              "127.0.0.1 accepts localhost",
			listenAddr:        "127.0.0.1:0",
			hostHeader:        "localhost:1234",
			disableProtection: false,
			wantStatus:        http.StatusOK,
		},
		{
			name:              "127.0.0.1 rejects evil.com",
			listenAddr:        "127.0.0.1:0",
			hostHeader:        "evil.com",
			disableProtection: false,
			wantStatus:        http.StatusForbidden,
		},
		{
			name:              "127.0.0.1 rejects evil.com:80",
			listenAddr:        "127.0.0.1:0",
			hostHeader:        "evil.com:80",
			disableProtection: false,
			wantStatus:        http.StatusForbidden,
		},
		{
			name:              "127.0.0.1 rejects localhost.evil.com",
			listenAddr:        "127.0.0.1:0",
			hostHeader:        "localhost.evil.com",
			disableProtection: false,
			wantStatus:        http.StatusForbidden,
		},

		// When listening on 0.0.0.0, requests arriving via localhost are still protected
		// because LocalAddrContextKey returns the actual connection's local address.
		// This is actually more secure - DNS rebinding attacks target localhost regardless
		// of the listener configuration.
		{
			name:              "0.0.0.0 via localhost rejects evil.com",
			listenAddr:        "0.0.0.0:0",
			hostHeader:        "evil.com",
			disableProtection: false,
			wantStatus:        http.StatusForbidden,
		},

		// Explicit disable
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
			opts := &StreamableHTTPOptions{
				Stateless:                  true, // Simpler for testing
				DisableLocalhostProtection: tt.disableProtection,
			}
			handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, opts)

			listener, err := net.Listen("tcp", tt.listenAddr)
			if err != nil {
				t.Fatalf("Failed to listen on %s: %v", tt.listenAddr, err)
			}
			defer listener.Close()

			srv := &http.Server{Handler: handler}
			go srv.Serve(listener)
			defer srv.Close()

			reqReader := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
			req, err := http.NewRequest("POST", fmt.Sprintf("http://%s", listener.Addr().String()), reqReader)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = tt.hostHeader
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")

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

func TestStreamableOriginProtection(t *testing.T) {
	server := NewServer(testImpl, nil)

	tests := []struct {
		name           string
		protection     *http.CrossOriginProtection
		requestOrigin  string
		wantStatusCode int
	}{
		{
			name:           "no protection with Origin header",
			protection:     nil,
			requestOrigin:  "https://example.com",
			wantStatusCode: http.StatusOK,
		},
		{
			name: "custom protection with trusted origin and same Origin",
			protection: func() *http.CrossOriginProtection {
				p := http.NewCrossOriginProtection()
				if err := p.AddTrustedOrigin("https://example.com"); err != nil {
					t.Fatal(err)
				}
				return p
			}(),
			requestOrigin:  "https://example.com",
			wantStatusCode: http.StatusOK,
		},
		{
			name: "custom protection with trusted origin and different Origin",
			protection: func() *http.CrossOriginProtection {
				p := http.NewCrossOriginProtection()
				if err := p.AddTrustedOrigin("https://example.com"); err != nil {
					t.Fatal(err)
				}
				return p
			}(),
			requestOrigin:  "https://malicious.com",
			wantStatusCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &StreamableHTTPOptions{
				Stateless:             true, // avoid session ID requirement
				CrossOriginProtection: tt.protection,
			}
			handler := NewStreamableHTTPHandler(func(req *http.Request) *Server { return server }, opts)
			httpServer := httptest.NewServer(handler)
			defer httpServer.Close()

			reqReader := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
			req, err := http.NewRequest(http.MethodPost, httpServer.URL, reqReader)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Origin", tt.requestOrigin)
			req.Header.Set("Accept", "application/json, text/event-stream")

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			if got := resp.StatusCode; got != tt.wantStatusCode {
				t.Errorf("Status code: got %d, want %d", got, tt.wantStatusCode)
			}
		})
	}
}

// TestStandaloneSSEEmitsCommentForHTTP2Flush is a regression test for the
// HTTP/2 reverse-proxy header-buffering bug. The standalone SSE GET stream
// must emit at least one body byte (an SSE comment) immediately after the
// response headers. Without a DATA frame, HTTP/2 reverse proxies (Envoy,
// Caddy, net/http/httputil, etc.) buffer the HEADERS frame indefinitely
// because they have nothing to coalesce it with — calling Flush() alone is
// not sufficient.
//
// SSE explicitly defines lines starting with ":" as comments that clients
// ignore, so this is behavior-preserving for the SSE protocol while
// producing the DATA frame that HTTP/2 proxies need.
func TestStandaloneSSEEmitsCommentForHTTP2Flush(t *testing.T) {
	server := NewServer(testImpl, nil)
	handler := NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Initialize a session so we have a valid Mcp-Session-Id to use on the
	// standalone SSE GET.
	initBody := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
	initReq, err := http.NewRequestWithContext(ctx, http.MethodPost, httpServer.URL, initBody)
	if err != nil {
		t.Fatal(err)
	}
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initResp, err := http.DefaultClient.Do(initReq)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, initResp.Body)
	initResp.Body.Close()
	sessionID := initResp.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatalf("initialize response missing %s header", sessionIDHeader)
	}

	// Open the standalone SSE stream.
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, httpServer.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	getReq.Header.Set("Accept", "text/event-stream")
	getReq.Header.Set(sessionIDHeader, sessionID)

	resp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		t.Fatalf("GET standalone SSE: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d; want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q; want text/event-stream", got)
	}

	// Read the first chunk. With the fix, an SSE comment (": ok\n\n") is
	// written immediately. Without the fix, the body has no bytes to read
	// until the server pushes an event, which won't happen in this test.
	type readResult struct {
		n   int
		err error
		buf []byte
	}
	ch := make(chan readResult, 1)
	go func() {
		buf := make([]byte, 64)
		n, err := resp.Body.Read(buf)
		ch <- readResult{n: n, err: err, buf: buf}
	}()

	select {
	case r := <-ch:
		if r.err != nil && r.err != io.EOF {
			t.Fatalf("reading first SSE chunk: %v", r.err)
		}
		if r.n == 0 {
			t.Fatal("first SSE chunk was empty; expected an SSE comment to flush HTTP/2 proxy HEADERS frame")
		}
		got := string(r.buf[:r.n])
		// SSE spec: lines starting with ":" are comments, ignored by clients.
		// We don't pin the exact comment text; just require the first byte to
		// be the SSE comment marker so HTTP/2 proxies have a DATA frame.
		if !strings.HasPrefix(got, ":") {
			t.Errorf("first SSE chunk = %q; want it to start with %q (SSE comment marker, so HTTP/2 proxies receive a DATA frame and forward HEADERS)", got, ":")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first SSE bytes; the standalone SSE stream must emit a DATA frame immediately so HTTP/2 reverse proxies don't buffer the HEADERS frame")
	}
}

// newProtocolBody builds a raw JSON body for a tools/call request that
// carries the >= 2026-07-28 per-request _meta fields.
func newProtocolBody(t *testing.T, toolName string, args any) []byte {
	t.Helper()
	rawArgs, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"_meta": map[string]any{
				MetaKeyProtocolVersion:    protocolVersion20260728,
				MetaKeyClientInfo:         map[string]any{"name": "new-proto-client", "version": "9.9"},
				MetaKeyClientCapabilities: map[string]any{"sampling": map[string]any{}},
			},
			"name":      toolName,
			"arguments": json.RawMessage(rawArgs),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestEphemeralConnectOpts(t *testing.T) {
	mkReq := func(body []byte) *http.Request {
		r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		return r
	}

	h := &StreamableHTTPHandler{opts: StreamableHTTPOptions{}}

	oldProtocolBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": "x", "arguments": map[string]any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	initializeBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  methodInitialize,
		"params":  map[string]any{"protocolVersion": protocolVersion20250618},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                  string
		body                  []byte
		wantUsesNew           bool
		wantInitializeParams  bool
		wantInitializedParams bool
	}{
		{
			name:                  "new-protocol request: no synthetic state",
			body:                  newProtocolBody(t, "x", struct{}{}),
			wantUsesNew:           true,
			wantInitializeParams:  false,
			wantInitializedParams: false,
		},
		{
			name:                  "old-protocol request: synthetic state populated",
			body:                  oldProtocolBody,
			wantUsesNew:           false,
			wantInitializeParams:  true,
			wantInitializedParams: true,
		},
		{
			name:                  "initialize request: no synthetic InitializeParams",
			body:                  initializeBody,
			wantUsesNew:           false,
			wantInitializeParams:  false,
			wantInitializedParams: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := mkReq(tt.body)
			var pver string
			if tt.wantUsesNew {
				pver = protocolVersion20260728
			} else {
				pver = protocolVersion20250326
			}
			req.Header.Set(protocolVersionHeader, pver)
			req = req.WithContext(context.WithValue(req.Context(), protocolVersionContextKey{}, pver))
			info, err := h.ephemeralConnectOpts(req)
			if err != nil {
				t.Fatal(err)
			}
			if info.usesNewProtocol != tt.wantUsesNew {
				t.Errorf("usesNewProtocol = %v, want %v", info.usesNewProtocol, tt.wantUsesNew)
			}
			if got := info.opts.State.InitializeParams != nil; got != tt.wantInitializeParams {
				t.Errorf("InitializeParams non-nil = %v, want %v (value = %+v)",
					got, tt.wantInitializeParams, info.opts.State.InitializeParams)
			}
			if got := info.opts.State.InitializedParams != nil; got != tt.wantInitializedParams {
				t.Errorf("InitializedParams non-nil = %v, want %v (value = %+v)",
					got, tt.wantInitializedParams, info.opts.State.InitializedParams)
			}
		})
	}
}

// statelessHandlerCapture builds a stateless server with a single tool whose
// handler captures everything we want to assert about the per-request view of
// the session and the new-protocol accessors.
type statelessHandlerCapture struct {
	mu                    sync.Mutex
	sessionInitParams     *InitializeParams
	reqProtocolVersion    string
	reqClientInfo         *Implementation
	reqClientCapabilities *ClientCapabilities
}

func TestStreamableStateless_NewProtocolSession_NoFakeInit(t *testing.T) {
	// SEP-2575: the MCP-Protocol-Version header is mandatory for new-protocol
	// requests and must be a supported version. The 2026-07-28 version is
	// not yet in the global list, so register it for the duration of the test.

	capture := &statelessHandlerCapture{}
	mcpServer := NewServer(testImpl, nil)
	AddTool(mcpServer, &Tool{Name: "capture", Description: "captures request info"},
		func(ctx context.Context, req *CallToolRequest, args struct{}) (*CallToolResult, any, error) {
			capture.mu.Lock()
			defer capture.mu.Unlock()
			capture.sessionInitParams = req.Session.InitializeParams()
			capture.reqProtocolVersion = req.ProtocolVersion()
			capture.reqClientInfo = req.ClientInfo()
			capture.reqClientCapabilities = req.ClientCapabilities()
			return &CallToolResult{Content: []Content{&TextContent{Text: "ok"}}}, nil, nil
		})

	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return mcpServer },
		&StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := newProtocolBody(t, "capture", struct{}{})
	httpReq, err := http.NewRequest(http.MethodPost, httpServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")
	httpReq.Header.Set(protocolVersionHeader, protocolVersion20260728)
	// >= 2026-07-28 also requires the Mcp-Method and Mcp-Name standard
	// headers (see streamable_headers.go).
	httpReq.Header.Set(methodHeader, "tools/call")
	httpReq.Header.Set(nameHeader, "capture")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, respBody)
	}

	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.sessionInitParams == nil {
		t.Errorf("Session.InitializeParams() is nil, want populated initializeParams for new-protocol session")
	} else {
		if got, want := capture.sessionInitParams.ProtocolVersion, protocolVersion20260728; got != want {
			t.Errorf("Session.InitializeParams().ProtocolVersion = %q, want %q", got, want)
		}
		if got, want := capture.sessionInitParams.ClientInfo.Name, "new-proto-client"; got != want {
			t.Errorf("Session.InitializeParams().ClientInfo.Name = %q, want %q", got, want)
		}
	}
	if got, want := capture.reqProtocolVersion, protocolVersion20260728; got != want {
		t.Errorf("req.ProtocolVersion() = %q, want %q", got, want)
	}
	if capture.reqClientInfo == nil || capture.reqClientInfo.Name != "new-proto-client" {
		t.Errorf("req.ClientInfo() = %+v, want Name=new-proto-client", capture.reqClientInfo)
	}
	if capture.reqClientCapabilities == nil || capture.reqClientCapabilities.Sampling == nil {
		t.Errorf("req.ClientCapabilities() = %+v, want non-nil Sampling", capture.reqClientCapabilities)
	}
}

// TestStreamableStateful_RejectsNewProtocol verifies that a stateful HTTP
// server rejects requests carrying _meta.protocolVersion (i.e. >= 2026-07-28
// requests) with HTTP 400. The new protocol is
// supported on HTTP only when StreamableHTTPOptions.Stateless=true.
func TestStreamableStateful_RejectsNewProtocol(t *testing.T) {
	// Make 2026-07-28 a "known" version so that the request reaches servePOST
	// (otherwise the early header validation at ServeHTTP rejects it).

	server := NewServer(testImpl, nil)
	AddTool(server, &Tool{Name: "noop"},
		func(ctx context.Context, req *CallToolRequest, args struct{}) (*CallToolResult, any, error) {
			return &CallToolResult{Content: []Content{&TextContent{Text: "ok"}}}, nil, nil
		})
	handler := NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	// Initialize a legacy session first.
	initBody := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}`)
	initReq, err := http.NewRequest(http.MethodPost, httpServer.URL, initBody)
	if err != nil {
		t.Fatal(err)
	}
	initReq.Header.Set("Content-Type", "application/json")
	initReq.Header.Set("Accept", "application/json, text/event-stream")
	initResp, err := http.DefaultClient.Do(initReq)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, initResp.Body)
	initResp.Body.Close()
	sessionID := initResp.Header.Get(sessionIDHeader)
	if sessionID == "" {
		t.Fatalf("initialize response missing %s header", sessionIDHeader)
	}

	// Drive the existing session with a new-protocol request whose header and
	// body agree. The cross-check passes; the stateful-rejection check fires.
	body := newProtocolBody(t, "noop", struct{}{})
	req, err := http.NewRequest(http.MethodPost, httpServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(sessionIDHeader, sessionID)
	req.Header.Set(protocolVersionHeader, protocolVersion20260728)
	req.Header.Set(methodHeader, "tools/call")
	req.Header.Set(nameHeader, "noop")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", resp.StatusCode, respBody)
	}
	if !strings.Contains(string(respBody), "stateless") {
		t.Errorf("body = %q, want a message mentioning 'stateless'", respBody)
	}
}

// TestStreamableStateless_AcceptsNewProtocol is the positive control:
// confirms that a stateless server still accepts new-protocol requests
// (the rejection in TestStreamableStateful_RejectsNewProtocol must not
// fire on Stateless: true).
func TestStreamableStateless_AcceptsNewProtocol(t *testing.T) {

	server := NewServer(testImpl, nil)
	AddTool(server, &Tool{Name: "noop"},
		func(ctx context.Context, req *CallToolRequest, args struct{}) (*CallToolResult, any, error) {
			return &CallToolResult{Content: []Content{&TextContent{Text: "ok"}}}, nil, nil
		})
	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return server },
		&StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	body := newProtocolBody(t, "noop", struct{}{})
	req, err := http.NewRequest(http.MethodPost, httpServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(protocolVersionHeader, protocolVersion20260728)
	req.Header.Set(methodHeader, "tools/call")
	req.Header.Set(nameHeader, "noop")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, respBody)
	}
}

// TestStreamableClientUnsupportedVersionFallback exercises the full
// SEP-2575 fallback. The client requests protocolVersion20260728, which the
// server is configured to NOT advertise in supportedProtocolVersions for the
// duration of this test. The server therefore rejects the server/discover
// POST at the transport-level header validation with a plain HTTP 400 ("Bad
// Request: Unsupported protocol version ..."). The streamable client must
// recognize this body, keep the connection alive, and successfully complete
// the legacy initialize handshake.
func TestStreamableClientUnsupportedVersionFallback(t *testing.T) {
	// Remove protocolVersion20260728 from the server-side supported list so
	// the fallback path is exercised.
	orig := supportedProtocolVersions
	supportedProtocolVersions = slices.DeleteFunc(slices.Clone(orig), func(v string) bool {
		return v == protocolVersion20260728
	})
	t.Cleanup(func() { supportedProtocolVersions = orig })

	ctx := context.Background()

	server := NewServer(testImpl, nil)
	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return server },
		nil,
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	client := NewClient(testImpl, nil)
	transport := &StreamableClientTransport{Endpoint: httpServer.URL}

	session, err := client.Connect(ctx, transport, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	ir := session.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil; expected legacy initialize to populate it")
	}
	// The fallback initialize explicitly requests protocolVersion20251125
	// (see client.go), so the server negotiates that version.
	if ir.ProtocolVersion != protocolVersion20251125 {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q (legacy fallback)",
			ir.ProtocolVersion, protocolVersion20251125)
	}

	// Verify the session is fully usable after the fallback by issuing a
	// real call against the server.
	if err := session.Ping(ctx, nil); err != nil {
		t.Errorf("Ping after fallback initialize: %v", err)
	}
}

// TestStreamableStateful_AcceptsDiscover verifies that a stateful HTTP server
// accepts a server/discover probe carrying MCP-Protocol-Version: 2026-07-28
// (and the matching _meta.protocolVersion), instead of rejecting it with the
// "stateless required" 400. The SEP-2575 client flow has the client probing
// the server with the new protocol version to learn which versions are
// supported.
func TestStreamableStateful_AcceptsDiscover(t *testing.T) {

	server := NewServer(testImpl, nil)
	handler := NewStreamableHTTPHandler(func(*http.Request) *Server { return server }, nil)
	httpServer := httptest.NewServer(mustNotPanic(t, handler))
	defer httpServer.Close()

	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  methodDiscover,
		"params": map[string]any{
			"_meta": map[string]any{
				MetaKeyProtocolVersion:    protocolVersion20260728,
				MetaKeyClientInfo:         map[string]any{"name": "new-proto-client", "version": "9.9"},
				MetaKeyClientCapabilities: map[string]any{"sampling": map[string]any{}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, httpServer.URL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set(protocolVersionHeader, protocolVersion20260728)
	req.Header.Set(methodHeader, methodDiscover)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", resp.StatusCode, respBody)
	}

	// Parse the JSON-RPC response. The body may arrive as a bare JSON object
	// or as a single SSE event depending on the Accept negotiation; both
	// shapes are valid here.
	jsonPayload := respBody
	if i := bytes.Index(respBody, []byte("data: ")); i >= 0 {
		jsonPayload = respBody[i+len("data: "):]
		if j := bytes.IndexByte(jsonPayload, '\n'); j >= 0 {
			jsonPayload = jsonPayload[:j]
		}
	}
	var rpcResp struct {
		Result *DiscoverResult `json:"result"`
		Error  *jsonrpc.Error  `json:"error"`
	}
	if err := json.Unmarshal(jsonPayload, &rpcResp); err != nil {
		t.Fatalf("unmarshal response %q: %v", respBody, err)
	}
	if rpcResp.Error != nil {
		t.Fatalf("discover returned error: %+v (body = %s)", rpcResp.Error, respBody)
	}
	if rpcResp.Result == nil {
		t.Fatalf("discover returned no result; body = %s", respBody)
	}
	if slices.Contains(rpcResp.Result.SupportedVersions, protocolVersion20260728) {
		t.Errorf("DiscoverResult.SupportedVersions = %v, must not include %q on a stateful transport",
			rpcResp.Result.SupportedVersions, protocolVersion20260728)
	}
	if len(rpcResp.Result.SupportedVersions) == 0 {
		t.Errorf("DiscoverResult.SupportedVersions is empty; want at least one legacy version")
	}
}

// TestStreamableHTTP_E2E_DiscoverSuccess is a full end-to-end smoke test for
// SEP-2575 over the streamable HTTP transport.
func TestStreamableHTTP_E2E_DiscoverSuccess(t *testing.T) {
	ctx := context.Background()
	server := NewServer(&Implementation{Name: "e2e-server", Version: "v1"}, nil)
	// Register a simple tool so we can prove the session is usable end-to-end.
	AddTool(server, &Tool{Name: "echo", Description: "echoes its input"},
		func(_ context.Context, _ *CallToolRequest, args struct {
			Msg string `json:"msg"`
		}) (*CallToolResult, struct{}, error) {
			return &CallToolResult{
				Content: []Content{&TextContent{Text: args.Msg}},
			}, struct{}{}, nil
		},
	)

	handler := NewStreamableHTTPHandler(
		func(*http.Request) *Server { return server },
		&StreamableHTTPOptions{Stateless: true},
	)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	client := NewClient(&Implementation{Name: "e2e-client", Version: "v1"}, nil)
	transport := &StreamableClientTransport{Endpoint: httpServer.URL}
	cs, err := client.Connect(ctx, transport, &ClientSessionOptions{protocolVersion: protocolVersion20260728})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer cs.Close()

	ir := cs.InitializeResult()
	if ir == nil {
		t.Fatal("InitializeResult is nil after Connect; discover should have populated it")
	}
	if ir.ProtocolVersion != protocolVersion20260728 {
		t.Errorf("InitializeResult.ProtocolVersion = %q, want %q (negotiated via discover)",
			ir.ProtocolVersion, protocolVersion20260728)
	}
	if ir.ServerInfo == nil || ir.ServerInfo.Name != "e2e-server" {
		t.Errorf("InitializeResult.ServerInfo = %+v, want name=e2e-server", ir.ServerInfo)
	}

	// Prove the session is fully usable: list tools and call one.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
		t.Errorf("ListTools = %+v, want one tool named 'echo'", tools.Tools)
	}
	res, err := cs.CallTool(ctx, &CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"msg": "hello"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if len(res.Content) != 1 {
		t.Fatalf("CallTool result content = %+v, want 1 entry", res.Content)
	}
	tc, ok := res.Content[0].(*TextContent)
	if !ok || tc.Text != "hello" {
		t.Errorf("CallTool result[0] = %+v, want TextContent{Text:\"hello\"}", res.Content[0])
	}
}

// A POST whose call ID is already in flight on the same session must be rejected
// with a JSON-RPC error, must not overwrite the existing request-to-stream
// mapping, and must not publish the duplicate message to the session's incoming
// channel.
func TestStreamableServerRejectsDuplicateInFlightRequestID(t *testing.T) {
	id := jsonrpc2.Int64ID(1)
	conn := &streamableServerConn{
		logger:   ensureLogger(nil),
		incoming: make(chan jsonrpc.Message, 1),
		done:     make(chan struct{}),
		streams: map[string]*stream{
			"existing": {
				id:       "existing",
				logger:   ensureLogger(nil),
				requests: map[jsonrpc.ID]struct{}{id: {}},
			},
		},
		requestStreams: map[jsonrpc.ID]string{id: "existing"},
	}

	data, err := jsonrpc2.EncodeMessage(req(1, methodPing, &PingParams{}))
	if err != nil {
		t.Fatalf("EncodeMessage() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	httpReq := httptest.NewRequestWithContext(ctx, http.MethodPost, "/", bytes.NewReader(data))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	rec := httptest.NewRecorder()
	conn.servePOST(rec, httpReq)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	msg, err := jsonrpc2.DecodeMessage(rec.Body.Bytes())
	if err != nil {
		t.Fatalf("DecodeMessage() error = %v; body = %s", err, rec.Body.String())
	}
	resp, ok := msg.(*jsonrpc.Response)
	if !ok {
		t.Fatalf("response type = %T, want *jsonrpc.Response", msg)
	}
	if got := resp.ID.Raw(); got != int64(1) {
		t.Fatalf("response ID = %v, want 1", got)
	}
	var jerr *jsonrpc.Error
	if !errors.As(resp.Error, &jerr) {
		t.Fatalf("response error = %v, want *jsonrpc.Error", resp.Error)
	}
	if jerr.Code != jsonrpc.CodeInvalidRequest {
		t.Fatalf("error code = %d, want %d", jerr.Code, jsonrpc.CodeInvalidRequest)
	}
	if got := conn.requestStreams[id]; got != "existing" {
		t.Fatalf("requestStreams[%v] = %q, want %q", id, got, "existing")
	}
	select {
	case msg := <-conn.incoming:
		t.Fatalf("duplicate request was published to incoming: %v", msg)
	default:
	}
}

// TestStreamableMaxRequestBodyBytes verifies that the streamable HTTP handler
// enforces StreamableHTTPOptions.MaxRequestBodyBytes on incoming request
// bodies. The limit must apply uniformly regardless of transfer encoding:
// Content-Length, Transfer-Encoding: chunked, or HTTP/2 (which has no
// Content-Length).
func TestStreamableMaxRequestBodyBytes(t *testing.T) {
	// A minimally-valid JSON-RPC initialize request. The body-size check runs
	// before parsing, so exact contents don't matter for the reject cases,
	// but we use a real one for the "under limit" success case.
	validInit := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"v1"}}}`)

	// Build a payload of the requested size by padding a JSON object. The
	// server doesn't need to parse it; the MaxBytesReader trips first.
	oversizedBody := func(n int) []byte {
		b := make([]byte, n)
		b[0] = '{'
		for i := 1; i < n-1; i++ {
			b[i] = ' '
		}
		b[n-1] = '}'
		return b
	}

	tests := []struct {
		name           string
		limit          int64 // value assigned to MaxRequestBodyBytes
		body           []byte
		wantStatus     int
		wantStatusDesc string
	}{
		{
			name:           "default limit rejects oversized",
			limit:          0, // triggers DefaultMaxRequestBodyBytes (4 MiB)
			body:           oversizedBody(DefaultMaxRequestBodyBytes + 1),
			wantStatus:     http.StatusRequestEntityTooLarge,
			wantStatusDesc: "413 for body > 4MiB default",
		},
		{
			name:           "custom limit rejects oversized",
			limit:          1024,
			body:           oversizedBody(2048),
			wantStatus:     http.StatusRequestEntityTooLarge,
			wantStatusDesc: "413 for body > custom 1KiB limit",
		},
		{
			name:           "chunked encoding does not bypass limit",
			limit:          1024,
			body:           oversizedBody(4096),
			wantStatus:     http.StatusRequestEntityTooLarge,
			wantStatusDesc: "413 for chunked body > limit (regression: Content-Length-only checks are bypassable)",
		},
		{
			name:           "negative limit disables the check",
			limit:          -1,
			body:           oversizedBody(2 * DefaultMaxRequestBodyBytes),
			wantStatus:     http.StatusBadRequest, // body is not valid JSON-RPC; but the size check must not fire
			wantStatusDesc: "not 413 when limit is disabled",
		},
		{
			name:           "under limit succeeds",
			limit:          64 * 1024,
			body:           validInit,
			wantStatus:     http.StatusOK,
			wantStatusDesc: "200 for a valid initialize under the limit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := NewServer(testImpl, nil)
			handler := NewStreamableHTTPHandler(
				func(*http.Request) *Server { return server },
				&StreamableHTTPOptions{
					Stateless:           true,
					MaxRequestBodyBytes: tc.limit,
				},
			)
			ts := httptest.NewServer(handler)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodPost, ts.URL, bytes.NewReader(tc.body))
			if err != nil {
				t.Fatalf("NewRequest: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")

			resp, err := ts.Client().Do(req)
			if err != nil {
				t.Fatalf("Do: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)

			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("%s: status = %d, want %d (body=%q)",
					tc.wantStatusDesc, resp.StatusCode, tc.wantStatus, string(body))
			}
		})
	}
}
