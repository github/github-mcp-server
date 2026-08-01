// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// TODO: move server-side streamable HTTP logic from streamable.go to this file.

package mcp

/*
Streamable HTTP Server Design

This document describes the server-side implementation of the MCP streamable
HTTP transport, as defined by the MCP spec:
https://modelcontextprotocol.io/specification/2025-11-25/basic/transports#streamable-http

# Overview

The streamable HTTP transport enables MCP communication over HTTP, with
server-sent events (SSE) for server-to-client messages. The implementation
consists of several layered components:

	┌─────────────────────────────────────────────────────────────────┐
	│                   [StreamableHTTPHandler]                       │
	│   http.Handler that manages sessions and routes HTTP requests   │
	└─────────────────────────────────────────────────────────────────┘
	                              │
	                              ▼
	┌─────────────────────────────────────────────────────────────────┐
	│                  [StreamableServerTransport]                    │
	│  transport implementation, one per session; exposes ServeHTTP   │
	└─────────────────────────────────────────────────────────────────┘
	                              │
	                              ▼
	┌─────────────────────────────────────────────────────────────────┐
	│                   [streamableServerConn]                        │
	│        Connection implementation, handles message routing       │
	└─────────────────────────────────────────────────────────────────┘
	                              │
	                              ▼
	┌─────────────────────────────────────────────────────────────────┐
	│                         [stream]                                │
	│   Logical message channel within a session, may be resumed      │
	└─────────────────────────────────────────────────────────────────┘

# Sessions

As with other transports, a session represents a logical MCP connection between
a client and server. In the streamable transport, sessions are identified by a
unique session ID (Mcp-Session-Id header) and persist across multiple HTTP
requests.

[StreamableHTTPHandler] maintains a map of active sessions ([sessionInfo]),
each containing:
  - The [ServerSession] (MCP-level session state)
  - The [StreamableServerTransport] (for message I/O)
  - Optional timeout management for idle session cleanup

Sessions are created on the first POST request (typically containing the
initialize request) and destroyed either by:
  - Client sending a DELETE request
  - Session timeout due to inactivity
  - Server explicitly closing the session

# Streams

Within a session, there can be multiple concurrent "streams" - logical channels
for message delivery. This is distinct from HTTP streams; a single [stream] may
span multiple HTTP request/response cycles (via resumption).

There are two types of streams:

1. Optional standalone SSE stream (id = ""):
   - Created when client sends a GET request to the endpoint
   - Used for server-initiated messages (requests/notifications to client)
   - Persists for the lifetime of the session
   - Only one standalone stream per session

2. Request streams (id = random string):
   - Created for each POST request containing JSON-RPC calls
   - Used to route responses back to the originating HTTP request
   - Completed when all responses have been sent
   - Can be resumed via GET with Last-Event-ID if interrupted

# Message Routing

When the server writes a message, it must be routed to the correct [stream]:

  - Responses: Routed to the stream that originated the request
  - Requests/Notifications made during request handling: Routed to the same
    stream as the triggering request (via context)
  - Requests/Notifications made outside request handling: Routed to the
    standalone SSE stream

This routing is implemented using:
  - [streamableServerConn.requestStreams] maps request IDs to stream IDs
  - [idContextKey] is used to store the originating request ID in Context
  - [streamableServerConn.streams] maps stream IDs to [stream] objects

# Stream Resumption

If an HTTP connection is interrupted (network issues, etc.), clients can
resume a stream by sending a GET request with the Last-Event-ID header.
This requires an [EventStore] to be configured on the server.

  - [EventStore.Open] is called when a new stream is created
  - [EventStore.Append] is called for each message written to the stream
  - [EventStore.After] is called to replay messages after a given index
  - [EventStore.SessionClosed] is called when the session ends

Event IDs are formatted as "<streamID>_<index>" to identify both the
stream and position within that stream (see [formatEventID] and [parseEventID]).

# Stateless Mode

For simpler deployments, the handler supports "stateless" mode
([StreamableHTTPOptions.Stateless]) where:
  - No session ID validation is performed
  - Each request creates a temporary session that's closed after the request
  - Server-to-client requests are not supported (no way to receive response)

This mode is useful for simple tool servers that don't need bidirectional
communication.

# Response Formats

The server can respond to POST requests in two formats:

1. text/event-stream (default): Messages sent as SSE events, supports
   streaming multiple messages and server-initiated communication during
   request handling.

2. application/json ([StreamableHTTPOptions.JSONResponse]): Single JSON
   response, simpler but doesn't support streaming. Server-initiated messages
   during request handling go to the standalone SSE stream instead.

# HTTP Methods

  - POST: Send JSON-RPC messages (requests, responses, notifications)
  - GET: Open standalone SSE stream or resume an interrupted stream
  - DELETE: Terminate the session

# Key Implementation Details

The [stream] struct manages delivery of messages to HTTP responses.

Fields:
  - [stream.w] is the ResponseWriter for the current HTTP response (non-nil indicates claimed)
  - [stream.done] is closed to release the hanging HTTP request
  - [stream.requests] tracks pending request IDs (stream completes when empty)

Methods:
  - [stream.deliverLocked] delivers a message to the stream
  - [stream.close] sends a close event and releases the stream
  - [stream.release] releases the stream from the HTTP request, allowing resumption

[streamableServerConn] handles the [Connection] interface:
  - [streamableServerConn.Read] receives messages from the incoming channel (fed by POST handlers)
  - [streamableServerConn.Write] routes messages to appropriate streams
  - [streamableServerConn.Close] terminates the session and notifies the [EventStore]
*/
