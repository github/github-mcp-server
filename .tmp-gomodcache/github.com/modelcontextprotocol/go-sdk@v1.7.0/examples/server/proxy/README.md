# Proxy Header Propagation Example

This example demonstrates how to propagate HTTP headers from an incoming MCP server request to an outgoing MCP client request.

This is particularly useful when building a gateway or proxy server that needs to forward authentication tokens, trace IDs, or other context-sensitive headers to downstream services.

## Architecture

This example runs three components in a single process:

1.  **Backend Server** (Port 8082):
    -   Exposes a tool `echo_headers` that returns the headers it received.

2.  **Proxy Server** (Port 8081):
    -   Exposes a tool `forward_headers`.
    -   Acts as a client to the Backend Server.
    -   Uses a custom `http.RoundTripper` (`HeaderForwardingTransport`) to inject headers from the context into outgoing requests.

3.  **Client**:
    -   Connects to the Proxy Server.
    -   Calls `forward_headers`.

## How it works

1.  The Client calls `forward_headers` on the Proxy Server.
2.  The Proxy Server receives the request. The request context contains the HTTP headers in `req.Extra.Header`.
3.  The Proxy's tool handler extracts these headers and places them into a new `context.Context` using a specific key (`headerContextKey`).
4.  The Proxy uses an `mcp.Client` configured with a custom `HTTPClient` that uses `HeaderForwardingTransport`.
5.  `HeaderForwardingTransport.RoundTrip` inspects the context of outgoing requests. If it finds headers under `headerContextKey`, it adds them to the HTTP request.
6.  The Backend Server receives the request with the propagated headers.

## Running the Example

Run the example with:

```bash
go run main.go
```

You should see output indicating:
1.  Backend and Proxy servers starting.
2.  Gateway receiving headers.
3.  Client receiving the result, which contains the echoed headers.

Example output:

```
2025/08/29 10:00:00 Starting Backend Server on :8082
2025/08/29 10:00:00 Starting Gateway Server on :8081
2025/08/29 10:00:00 Gateway received headers: map[...]
2025/08/29 10:00:00 Client received result: ...
```
