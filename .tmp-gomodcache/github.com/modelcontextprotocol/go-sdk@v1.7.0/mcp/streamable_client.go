// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// TODO: move client-side streamable HTTP logic from streamable.go to this file.

package mcp

/*
Streamable HTTP Client Design

This document describes the client-side implementation of the MCP streamable
HTTP transport, as defined by the MCP spec:
https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http

# Overview

The client-side streamable transport allows an MCP client to communicate with a
server over HTTP, sending messages via POST and receiving responses via either
JSON or server-sent events (SSE). The implementation consists of two main
components:

	┌─────────────────────────────────────────────────────────────────┐
	│                 [StreamableClientTransport]                     │
	│   Transport configuration; creates connections via Connect()    │
	└─────────────────────────────────────────────────────────────────┘
	                              │
	                              ▼
	┌─────────────────────────────────────────────────────────────────┐
	│                   [streamableClientConn]                        │
	│   Connection implementation; handles HTTP request/response      │
	└─────────────────────────────────────────────────────────────────┘
	                              │
	                              ├──────────────────────────────────────┐
	                              ▼                                      ▼
	┌─────────────────────────────────────────┐  ┌────────────────────────────────────┐
	│        POST request handlers            │  │      Standalone SSE stream         │
	│   (one per outgoing message/call)       │  │   (server-initiated messages)      │
	└─────────────────────────────────────────┘  └────────────────────────────────────┘

# Sessions

The client optionally maintains a session with the server, identified by a
session ID (Mcp-Session-Id header). Sessionless servers (those that never
send Mcp-Session-Id) are fully supported; the client simply omits the
header from subsequent requests and skips the DELETE on close.

When a session is present:

  - Session ID is received from the server after initialization
  - Client includes the session ID in all subsequent requests
  - Session ends when the client calls Close() (sends DELETE) or server returns 404

[streamableClientConn] stores the session state:
  - [streamableClientConn.sessionID]: Server-assigned session identifier
  - [streamableClientConn.initializedResult]: Protocol version and server capabilities

# Connection Lifecycle

1. Connect: [StreamableClientTransport.Connect] creates a [streamableClientConn]
   with a detached context for the connection's lifetime. The context is detached
   to prevent the standalone SSE stream from being cancelled when the original
   Connect context times out.

2. Initialize: The MCP client sends initialize/initialized messages. Upon
   receiving [InitializeResult], the connection:
   - Stores the negotiated protocol version for the Mcp-Protocol-Version header
   - Captures the session ID from the Mcp-Session-Id response header
   - Starts the standalone SSE stream via [streamableClientConn.connectStandaloneSSE]

3. Operation: Messages are sent via POST, responses received via JSON or SSE.

4. Close: [streamableClientConn.Close] sends a DELETE request to terminate
   the session (unless the session is already gone), then cancels the connection
   context to clean up the standalone SSE stream.

# Sending Messages (Write)

[streamableClientConn.Write] sends all outgoing messages via HTTP POST:

	POST /endpoint
	Content-Type: application/json
	Accept: application/json, text/event-stream
	Mcp-Protocol-Version: <negotiated version>
	Mcp-Session-Id: <session ID, if established>

	<JSON-RPC message>

The server may respond with:
  - 202 Accepted: Message received, no response body (notifications/responses)
  - 200 OK with application/json: Single JSON-RPC response
  - 200 OK with text/event-stream: SSE stream of responses

# Receiving Messages (Read)

[streamableClientConn.Read] returns messages from the [streamableClientConn.incoming]
channel, which is populated by multiple concurrent goroutines:

1. POST response handlers ([streamableClientConn.handleJSON] and
   [streamableClientConn.handleSSE]): Process responses from POST requests

2. Standalone SSE stream: Receives server-initiated requests and notifications

The client handles both response formats:
  - JSON: [streamableClientConn.handleJSON] reads body, decodes message
  - SSE: [streamableClientConn.handleSSE] scans events, decodes each message

# Standalone SSE Stream

After initialization, [streamableClientConn.sessionUpdated] triggers
[streamableClientConn.connectStandaloneSSE] to open a GET request for
server-initiated messages:

	GET /endpoint
	Accept: text/event-stream
	Mcp-Session-Id: <session ID>

Stream behavior:
  - Optional: Server may return 405 Method Not Allowed (spec-compliant) or
    other 4xx errors (tolerated in non-strict mode for compatibility)
  - Persistent: Runs for the connection lifetime in a background goroutine
  - Resumable: Uses Last-Event-ID header on reconnection if server provides event IDs
  - Reconnects: Automatic reconnection with exponential backoff on interruption

# Stream Resumption

When an SSE stream (standalone or POST response) is interrupted, the client
attempts to reconnect using [streamableClientConn.connectSSE]:

Event ID tracking:
  - [streamableClientConn.processStream] tracks the last received event ID
  - On reconnection, the Last-Event-ID header is set to resume from that point
  - Server replays missed events if it has an [EventStore] configured

See [calculateReconnectDelay] for the reconnect delay details.

Server-initiated reconnection (SEP-1699)
  - SSE retry field: Sets the delay for the next reconnect attempt
  - If server doesn't provide event IDs, non-standalone streams don't reconnect

# Response Formats

The client must handle two response formats from POST requests:

1. application/json: Single JSON-RPC response
   - Body contains one JSON-RPC message
   - Handled by [streamableClientConn.handleJSON]
   - Simpler but doesn't support streaming or server-initiated messages

2. text/event-stream: SSE stream of messages
   - Body contains SSE events with JSON-RPC messages
   - Handled by [streamableClientConn.handleSSE]
   - Supports multiple messages and server-initiated communication
   - Stream completes when the response to the originating call is received

# HTTP Methods

  - POST: Send JSON-RPC messages (requests, responses, notifications)
    - Used by [streamableClientConn.Write]
    - Response may be JSON or SSE

  - GET: Open or resume SSE stream for server-initiated messages
    - Used by [streamableClientConn.connectSSE]
    - Always expects text/event-stream response (or 405)

  - DELETE: Terminate the session
    - Used by [streamableClientConn.Close]
    - Skipped if session is already known to be gone ([ErrSessionMissing])
      or if no session was established (sessionless server)

# Error Handling

Errors are categorized and handled differently:

1. Transient (recoverable via reconnection):
   - Network interruption during SSE streaming
   - Connection reset or timeout
   - Triggers reconnection in [streamableClientConn.handleSSE]

2. Terminal (breaks the connection):
   - 404 Not Found: Session terminated by server ([ErrSessionMissing])
   - Message decode errors: Protocol violation
   - Context cancellation: Client closed connection
   - Mismatched session IDs: Protocol error (only relevant for servers that use sessions)
	 - See issue #683: our terminal errors are too strict.

Terminal errors are stored via [streamableClientConn.fail] and returned by
subsequent [streamableClientConn.Read] calls. The [streamableClientConn.failed]
channel signals that the connection is broken.

Special case: [ErrSessionMissing] indicates the server has terminated the session,
so [streamableClientConn.Close] skips the DELETE request.

# Protocol Version Header

After initialization, all requests include:

	Mcp-Protocol-Version: <negotiated version>

This header (set by [streamableClientConn.setMCPHeaders]):
  - Allows the server to handle requests per the negotiated protocol
  - Is omitted before initialization completes
  - Uses the version from [streamableClientConn.initializedResult]

# Key Implementation Details

[StreamableClientTransport] configuration:
  - [StreamableClientTransport.Endpoint]: URL of the MCP server
  - [StreamableClientTransport.HTTPClient]: Custom HTTP client (optional)
  - [StreamableClientTransport.MaxRetries]: Reconnection attempts (default 5)

[streamableClientConn] handles the [Connection] interface:
  - [streamableClientConn.Read]: Returns messages from incoming channel
  - [streamableClientConn.Write]: Sends messages via POST, starts response handlers
  - [streamableClientConn.Close]: Sends DELETE, cancels context, closes done channel

State management:
  - [streamableClientConn.incoming]: Buffered channel for received messages
  - [streamableClientConn.sessionID]: Server-assigned session identifier (empty for sessionless servers)
  - [streamableClientConn.initializedResult]: Cached for protocol version header
  - [streamableClientConn.failed]: Channel closed on terminal error
  - [streamableClientConn.done]: Channel closed on graceful shutdown
  - [streamableClientConn.ctx]: Detached context for connection lifetime
  - [streamableClientConn.cancel]: Cancels ctx to terminate SSE streams

Context handling:
  - Connection context is detached from [StreamableClientTransport.Connect] context
    using [xcontext.Detach] to preserve context values (for auth middleware) while
    preventing premature cancellation of the standalone SSE stream
  - Individual POST requests use caller-provided contexts for cancellation
*/
