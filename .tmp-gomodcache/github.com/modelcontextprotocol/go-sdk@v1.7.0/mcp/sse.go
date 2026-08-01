// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package mcp

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/internal/jsonrpc2"
	"github.com/modelcontextprotocol/go-sdk/internal/util"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
)

// This file implements support for SSE (HTTP with server-sent events)
// transport server and client.
// https://modelcontextprotocol.io/specification/2024-11-05/basic/transports
//
// The transport is simple, at least relative to the new streamable transport
// introduced in the 2025-03-26 version of the spec. In short:
//
//  1. Sessions are initiated via a hanging GET request, which streams
//     server->client messages as SSE 'message' events.
//  2. The first event in the SSE stream must be an 'endpoint' event that
//     informs the client of the session endpoint.
//  3. The client POSTs client->server messages to the session endpoint.
//
// Therefore, the each new GET request hands off its responsewriter to an
// [SSEServerTransport] type that abstracts the transport as follows:
//  - Write writes a new event to the responseWriter, or fails if the GET has
//  exited.
//  - Read reads off a message queue that is pushed to via POST requests.
//  - Close causes the hanging GET to exit.

// SSEHandler is an http.Handler that serves SSE-based MCP sessions as defined by
// the [2024-11-05 version] of the MCP spec.
//
// [2024-11-05 version]: https://modelcontextprotocol.io/specification/2024-11-05/basic/transports
type SSEHandler struct {
	getServer    func(request *http.Request) *Server
	opts         SSEOptions
	onConnection func(*ServerSession) // for testing; must not block

	mu       sync.Mutex
	sessions map[string]*SSEServerTransport
}

// SSEOptions specifies options for an [SSEHandler].
type SSEOptions struct {
	// DisableLocalhostProtection disables automatic DNS rebinding protection.
	// By default, requests arriving via a localhost address (127.0.0.1, [::1])
	// that have a non-localhost Host header are rejected with 403 Forbidden.
	// This protects against DNS rebinding attacks regardless of whether the
	// server is listening on localhost specifically or on 0.0.0.0.
	//
	// Only disable this if you understand the security implications.
	// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices#local-mcp-server-compromise
	DisableLocalhostProtection bool
}

// NewSSEHandler returns a new [SSEHandler] that creates and manages MCP
// sessions created via incoming HTTP requests.
//
// Sessions are created when the client issues a GET request to the server,
// which must accept text/event-stream responses (server-sent events).
// For each such request, a new [SSEServerTransport] is created with a distinct
// messages endpoint, and connected to the server returned by getServer.
// The SSEHandler also handles requests to the message endpoints, by
// delegating them to the relevant server transport.
//
// The getServer function may return a distinct [Server] for each new
// request, or reuse an existing server. If it returns nil, the handler
// will return a 400 Bad Request.
func NewSSEHandler(getServer func(request *http.Request) *Server, opts *SSEOptions) *SSEHandler {
	s := &SSEHandler{
		getServer: getServer,
		sessions:  make(map[string]*SSEServerTransport),
	}

	if opts != nil {
		s.opts = *opts
	}

	return s
}

// A SSEServerTransport is a logical SSE session created through a hanging GET
// request.
//
// Use [SSEServerTransport.Connect] to initiate the flow of messages.
//
// When connected, it returns the following [Connection] implementation:
//   - Writes are SSE 'message' events to the GET response.
//   - Reads are received from POSTs to the session endpoint, via
//     [SSEServerTransport.ServeHTTP].
//   - Close terminates the hanging GET.
//
// The transport is itself an [http.Handler]. It is the caller's responsibility
// to ensure that the resulting transport serves HTTP requests on the given
// session endpoint.
//
// Each SSEServerTransport may be connected (via [Server.Connect]) at most
// once, since [SSEServerTransport.ServeHTTP] serves messages to the connected
// session.
//
// Most callers should instead use an [SSEHandler], which transparently handles
// the delegation to SSEServerTransports.
type SSEServerTransport struct {
	// Endpoint is the endpoint for this session, where the client can POST
	// messages.
	Endpoint string

	// Response is the hanging response body to the incoming GET request.
	Response http.ResponseWriter

	// incoming is the queue of incoming messages.
	// It is never closed, and by convention, incoming is non-nil if and only if
	// the transport is connected.
	incoming chan jsonrpc.Message

	// We must guard both pushes to the incoming queue and writes to the response
	// writer, because incoming POST requests are arbitrarily concurrent and we
	// need to ensure we don't write push to the queue, or write to the
	// ResponseWriter, after the session GET request exits.
	mu     sync.Mutex    // also guards writes to Response
	closed bool          // set when the stream is closed
	done   chan struct{} // closed when the connection is closed
}

// ServeHTTP handles POST requests to the transport endpoint.
func (t *SSEServerTransport) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if t.incoming == nil {
		http.Error(w, "session not connected", http.StatusInternalServerError)
		return
	}

	// Read and parse the message.
	data, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	// Optionally, we could just push the data onto a channel, and let the
	// message fail to parse when it is read. This failure seems a bit more
	// useful
	msg, err := jsonrpc2.DecodeMessage(data)
	if err != nil {
		http.Error(w, "failed to parse body", http.StatusBadRequest)
		return
	}
	if req, ok := msg.(*jsonrpc.Request); ok {
		if _, err := checkRequest(req, serverMethodInfos); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	select {
	case t.incoming <- msg:
		w.WriteHeader(http.StatusAccepted)
	case <-t.done:
		http.Error(w, "session closed", http.StatusBadRequest)
	}
}

// Connect sends the 'endpoint' event to the client.
// See [SSEServerTransport] for more details on the [Connection] implementation.
func (t *SSEServerTransport) Connect(context.Context) (Connection, error) {
	if t.incoming != nil {
		return nil, fmt.Errorf("already connected")
	}
	t.incoming = make(chan jsonrpc.Message, 100)
	t.done = make(chan struct{})
	_, err := writeEvent(t.Response, Event{
		Name: "endpoint",
		Data: []byte(t.Endpoint),
	})
	if err != nil {
		return nil, err
	}
	return &sseServerConn{t: t}, nil
}

func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// DNS rebinding protection: auto-enabled for localhost servers.
	// See: https://modelcontextprotocol.io/specification/2025-11-25/basic/security_best_practices#local-mcp-server-compromise
	if !h.opts.DisableLocalhostProtection && disablelocalhostprotection != "1" {
		if localAddr, ok := req.Context().Value(http.LocalAddrContextKey).(net.Addr); ok && localAddr != nil {
			if util.IsLoopback(localAddr.String()) && !util.IsLoopback(req.Host) {
				http.Error(w, fmt.Sprintf("Forbidden: invalid Host header %q", req.Host), http.StatusForbidden)
				return
			}
		}
	}

	// Validate 'Content-Type' header.
	if disablecontenttypecheck != "1" && req.Method == http.MethodPost {
		mediaType, _, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "Content-Type must be 'application/json'", http.StatusUnsupportedMediaType)
			return
		}
	}

	sessionID := req.URL.Query().Get("sessionid")

	// For POST requests, the message body is a message to send to a session.
	if req.Method == http.MethodPost {
		// Look up the session.
		if sessionID == "" {
			http.Error(w, "sessionid must be provided", http.StatusBadRequest)
			return
		}
		h.mu.Lock()
		session := h.sessions[sessionID]
		h.mu.Unlock()
		if session == nil {
			http.Error(w, "session not found", http.StatusNotFound)
			return
		}

		session.ServeHTTP(w, req)
		return
	}

	if req.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// GET requests create a new session, and serve messages over SSE.

	// TODO: it's not entirely documented whether we should check Accept here.
	// Let's again be lax and assume the client will accept SSE.

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sessionID = rand.Text()
	endpoint, err := req.URL.Parse("?sessionid=" + sessionID)
	if err != nil {
		http.Error(w, "internal error: failed to create endpoint", http.StatusInternalServerError)
		return
	}

	transport := &SSEServerTransport{Endpoint: endpoint.RequestURI(), Response: w}

	// The session is terminated when the request exits.
	h.mu.Lock()
	h.sessions[sessionID] = transport
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
	}()

	server := h.getServer(req)
	if server == nil {
		// The getServer argument to NewSSEHandler returned nil.
		http.Error(w, "no server available", http.StatusBadRequest)
		return
	}
	ss, err := server.Connect(req.Context(), transport, nil)
	if err != nil {
		http.Error(w, "connection failed", http.StatusInternalServerError)
		return
	}
	if h.onConnection != nil {
		h.onConnection(ss)
	}
	defer ss.Close() // close the transport when the GET exits

	select {
	case <-req.Context().Done():
	case <-transport.done:
	}
}

// sseServerConn implements the [Connection] interface for a single [SSEServerTransport].
// It hides the Connection interface from the SSEServerTransport API.
type sseServerConn struct {
	t *SSEServerTransport
}

// TODO(jba): get the session ID. (Not urgent because SSE transports have been removed from the spec.)
func (s *sseServerConn) SessionID() string { return "" }

// Read implements jsonrpc2.Reader.
func (s *sseServerConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-s.t.incoming:
		return msg, nil
	case <-s.t.done:
		return nil, io.EOF
	}
}

// Write implements jsonrpc2.Writer.
func (s *sseServerConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	data, err := jsonrpc2.EncodeMessage(msg)
	if err != nil {
		return err
	}

	s.t.mu.Lock()
	defer s.t.mu.Unlock()

	// Note that it is invalid to write to a ResponseWriter after ServeHTTP has
	// exited, and so we must lock around this write and check isDone, which is
	// set before the hanging GET exits.
	if s.t.closed {
		return io.EOF
	}

	_, err = writeEvent(s.t.Response, Event{Name: "message", Data: data})
	return err
}

// Close implements io.Closer, and closes the session.
//
// It must be safe to call Close more than once, as the close may
// asynchronously be initiated by either the server closing its connection, or
// by the hanging GET exiting.
func (s *sseServerConn) Close() error {
	s.t.mu.Lock()
	defer s.t.mu.Unlock()
	if !s.t.closed {
		s.t.closed = true
		close(s.t.done)
	}
	return nil
}

// An SSEClientTransport is a [Transport] that can communicate with an MCP
// endpoint serving the SSE transport defined by the 2024-11-05 version of the
// spec.
//
// https://modelcontextprotocol.io/specification/2024-11-05/basic/transports
type SSEClientTransport struct {
	// Endpoint is the SSE endpoint to connect to.
	Endpoint string

	// HTTPClient is the client to use for making HTTP requests. If nil,
	// http.DefaultClient is used.
	HTTPClient *http.Client
}

// Connect connects through the client endpoint.
func (c *SSEClientTransport) Connect(ctx context.Context) (Connection, error) {
	parsedURL, err := url.Parse(c.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", c.Endpoint, nil)
	if err != nil {
		return nil, err
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Check HTTP status code before attempting to parse SSE events.
	// This ensures proper error reporting for authentication failures (401),
	// authorization failures (403), and other HTTP errors.
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		resp.Body.Close()
		return nil, fmt.Errorf("failed to connect: %s", http.StatusText(resp.StatusCode))
	}

	msgEndpoint, err := func() (*url.URL, error) {
		var evt Event
		for evt, err = range scanEvents(resp.Body) {
			break
		}
		if err != nil {
			return nil, err
		}
		if evt.Name != "endpoint" {
			return nil, fmt.Errorf("first event is %q, want %q", evt.Name, "endpoint")
		}
		raw := string(evt.Data)
		return parsedURL.Parse(raw)
	}()
	if err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("missing endpoint: %v", err)
	}

	// From here on, the stream takes ownership of resp.Body.
	s := &sseClientConn{
		client:      httpClient,
		msgEndpoint: msgEndpoint,
		incoming:    make(chan []byte, 100),
		body:        resp.Body,
		done:        make(chan struct{}),
	}

	go func() {
		defer s.Close() // close the transport when the GET exits

		for evt, err := range scanEvents(resp.Body) {
			if err != nil {
				return
			}
			select {
			case s.incoming <- evt.Data:
			case <-s.done:
				return
			}
		}
	}()

	return s, nil
}

// An sseClientConn is a logical jsonrpc2 connection that implements the client
// half of the SSE protocol:
//   - Writes are POSTS to the session endpoint.
//   - Reads are SSE 'message' events, and pushes them onto a buffered channel.
//   - Close terminates the GET request.
type sseClientConn struct {
	client      *http.Client // HTTP client to use for requests
	msgEndpoint *url.URL     // session endpoint for POSTs
	incoming    chan []byte  // queue of incoming messages

	mu     sync.Mutex
	body   io.ReadCloser // body of the hanging GET
	closed bool          // set when the stream is closed
	done   chan struct{} // closed when the stream is closed
}

// TODO(jba): get the session ID. (Not urgent because SSE transports have been removed from the spec.)
func (c *sseClientConn) SessionID() string { return "" }

func (c *sseClientConn) isDone() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *sseClientConn) Read(ctx context.Context) (jsonrpc.Message, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()

	case <-c.done:
		return nil, io.EOF

	case data := <-c.incoming:
		// TODO(rfindley): do we really need to check this? We receive from c.done above.
		if c.isDone() {
			return nil, io.EOF
		}
		msg, err := jsonrpc2.DecodeMessage(data)
		if err != nil {
			return nil, err
		}
		return msg, nil
	}
}

func (c *sseClientConn) Write(ctx context.Context, msg jsonrpc.Message) error {
	data, err := jsonrpc2.EncodeMessage(msg)
	if err != nil {
		return err
	}
	if c.isDone() {
		return io.EOF
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.msgEndpoint.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to write: %s", resp.Status)
	}
	return nil
}

func (c *sseClientConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		_ = c.body.Close()
		close(c.done)
	}
	return nil
}
