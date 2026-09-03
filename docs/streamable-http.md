# Streamable HTTP Server

The Streamable HTTP mode enables the GitHub MCP Server to run as an HTTP service, allowing clients to connect via standard HTTP protocols. This mode is ideal for deployment scenarios where stdio transport isn't suitable, such as reverse proxy setups, containerized environments, or distributed architectures.

## Features

- **Streamable HTTP Transport** — Full HTTP server with streaming support for real-time tool responses
- **OAuth Metadata Endpoints** — Standard `.well-known/oauth-protected-resource` discovery for OAuth clients
- **Scope Challenge Support** — Automatic scope validation with proper HTTP 403 responses and `WWW-Authenticate` headers
- **Scope Filtering** — Restrict available tools based on authenticated credentials and permissions
- **Custom Base Paths** — Support for reverse proxy deployments with customizable base URLs

## Running the Server

### Basic HTTP Server

Start the server on the default port (8082):

```bash
github-mcp-server http
```

The server will be available at `http://localhost:8082`.

### With Scope Challenge

Enable scope validation to enforce GitHub permission checks:

```bash
github-mcp-server http --scope-challenge
```

When `--scope-challenge` is enabled, requests with insufficient scopes receive a `403 Forbidden` response with a `WWW-Authenticate` header indicating the required scopes.

### Repository deletion and request-state encryption

The `delete_repository` tool uses multi-round-trip elicitation and carries its
confirmed target through client-held request state. To expose this tool in HTTP
mode, configure a stable 32-byte encryption key encoded with standard Base64:

```bash
export GITHUB_MCP_SERVER_MRTR_STATE_KEY="$(openssl rand -base64 32 | tr -d '\n')"
github-mcp-server http
```

Use the same key on every replica that may handle a retry. Keep it secret and
stable during deployments; changing it invalidates confirmations already in
flight. If the variable is absent, `delete_repository` is not exposed by the
HTTP server. If it is present but malformed, the server refuses to start.

This self-hosted key is independent of keys used by the hosted remote server.
Integrators can provide their own request-state sealer through the exported
`github.RequestStateSealer` interface and expose it from their tool dependencies
through `github.RequestStateSealerProvider` without changing their key format.

### With OAuth Metadata Discovery

For use behind reverse proxies or with custom domains, expose OAuth metadata endpoints:

```bash
github-mcp-server http --scope-challenge --base-url https://myserver.com --base-path /mcp
```

The OAuth protected resource metadata's `resource` attribute will be populated with the full URL to the server's protected resource endpoint:

```json
{
  "resource_name": "GitHub MCP Server",
  "resource": "https://myserver.com/mcp",
  "authorization_servers": [
    "https://github.com/login/oauth"
  ],
  "scopes_supported": [
    "repo",
    ...
  ],
  ...
}
```

This allows OAuth clients to discover authentication requirements and endpoint information automatically.

The HTTP server is the OAuth protected resource, not the authorization server. It
therefore serves `/.well-known/oauth-protected-resource` but does not serve
`/.well-known/oauth-authorization-server` unless a separately deployed authorization
server is explicitly hosted on the same origin.

Clients discover authorization-server metadata from the issuer listed in
`authorization_servers`. For the default `https://github.com/login/oauth` issuer,
RFC 8414 path insertion produces
`https://github.com/.well-known/oauth-authorization-server/login/oauth`. Browser-based
clients require that authorization server and its discovery endpoints to support
their browser origin through CORS. If the selected authorization server does not,
configure `--authorization-server` to advertise a browser-compatible OAuth proxy.

### Behind a Trusted Proxy (advanced)

By default, the server ignores the `X-Forwarded-Host` and `X-Forwarded-Proto` headers when constructing OAuth resource metadata URLs, so an untrusted client cannot influence the URL advertised to MCP clients. For most deployments, setting `--base-url` to the externally visible URL is the right approach.

If the server sits behind an internal forwarder that you fully control (for example, an in-cluster gateway that needs to preserve the originating hostname per request), you can opt into honoring those headers:

```bash
github-mcp-server http --trust-proxy-headers
```

Equivalent environment variable: `GITHUB_TRUST_PROXY_HEADERS=1`. Only enable this when the upstream proxy is trusted to set or strip these headers; otherwise prefer `--base-url`. When `--base-url` is set, it always takes precedence and `--trust-proxy-headers` has no effect.

### With an OAuth Proxy (GHES / non-standard authorization server)

When deploying against GitHub Enterprise Server, GHES does not natively support the OAuth extensions required by MCP (RFC 8414 metadata discovery, RFC 7591 Dynamic Client Registration, PKCE). In this case you can run a separate OAuth proxy that implements the MCP OAuth spec and forwards authentication to GHES. Use `--authorization-server` to advertise the proxy's URL in the protected resource metadata instead of the one derived from `--gh-host`:

```bash
github-mcp-server http \
  --gh-host https://github.example.com \
  --base-url https://mcp.example.com \
  --authorization-server https://mcp.example.com/oauth-proxy
```

The `authorization_servers` field in the protected resource metadata will then point at your proxy:

```json
{
  "resource": "https://mcp.example.com",
  "authorization_servers": [
    "https://mcp.example.com/oauth-proxy"
  ],
  ...
}
```

Equivalent environment variable: `GITHUB_AUTHORIZATION_SERVER=https://mcp.example.com/oauth-proxy`.

When neither the flag nor environment variable is set, the server preserves the existing behavior and derives the authorization server from `--gh-host`. The override only changes the URL advertised in OAuth protected resource metadata; it does not change token validation or the GitHub API host.

## Client Configuration

### Using OAuth Authentication

If your IDE or client has GitHub credentials configured (i.e. VS Code), simply reference the HTTP server:

```json
{
  "type": "http",
  "url": "http://localhost:8082"
}
```

The server will use the client's existing GitHub authentication.

### Using Bearer Tokens or Custom Headers

To provide PAT credentials, or to customize server behavior preferences, you can include additional headers in the client configuration:

```json
{
  "type": "http",
  "url": "http://localhost:8082",
  "headers": {
    "Authorization": "Bearer ghp_yourtokenhere",
    "X-MCP-Toolsets": "default",
    "X-MCP-Readonly": "true"
  }
}
```

See [Remote Server](./remote-server.md) documentation for more details on client configuration options.
