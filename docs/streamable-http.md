# Streamable HTTP Server

The Streamable HTTP mode enables the GitHub MCP Server to run as an HTTP service, allowing clients to connect via standard HTTP protocols. This mode is ideal for deployment scenarios where stdio transport isn't suitable, such as reverse proxy setups, containerized environments, or distributed architectures.

## Features

- **Streamable HTTP Transport** — Full HTTP server with streaming support for real-time tool responses
- **OAuth Metadata Endpoints** — Standard `.well-known/oauth-protected-resource` discovery for OAuth clients
- **Scope Challenge Support** — Automatic scope validation with proper HTTP 403 responses and `WWW-Authenticate` headers
- **Scope Filtering** — Restrict available tools based on authenticated credentials and permissions
- **Custom Base Paths** — Support for reverse proxy deployments with customizable base URLs

## Running the Server

### MCP parameter headers

For protocol version `2026-07-28` and later, the SDK validates standard
`Mcp-Method`, `Mcp-Name`, and schema-annotated `Mcp-Param-*` headers at the
Streamable HTTP boundary. Tool schemas annotate `owner` and `repo` with
`x-mcp-header`; when an annotated argument is present and non-null, its header
must be present and match the body value. A header for an absent or null
argument is rejected. Header names are case-insensitive; string values and
JSON property names are case-sensitive.

Mismatches return HTTP `400` with JSON-RPC error code `-32020` before MCP
receiving middleware or tool handlers run. This applies to typed and untyped
tools without changing their advertised schemas. Full typed body validation
and defaults still run after tool authorization guards. Older protocol versions
do not require or validate these standard parameter headers.

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
    "read:org",
    "read:user",
    "user:email",
    "read:packages",
    "write:packages",
    "read:project",
    "project",
    "gist",
    "notifications"
  ],
  ...
}
```

This allows OAuth clients to discover authentication requirements and endpoint information automatically.
Scopes excluded from this default set, such as `delete_repo`, are requested only
through a per-tool OAuth authorization challenge when needed.

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

### With an OAuth Proxy (GHES / RFC 7591 DCR clients)

When deploying against GitHub Enterprise Server, GHES does not natively support the OAuth extensions required by MCP (RFC 8414 metadata discovery, RFC 7591 Dynamic Client Registration, PKCE). Similarly, on `github.com`, `https://github.com/login/oauth` does not support RFC 7591 Dynamic Client Registration (`/register`) and returns `application/x-www-form-urlencoded` token responses unless `Accept: application/json` is sent—which prevents MCP clients that rely on DCR or standard OIDC refresh tokens (such as Google Gemini Web/Mobile custom apps or OpenCode) from completing automated OAuth onboarding.

In either case, you can run a separate OAuth 2.1 proxy that implements RFC 8414, RFC 7591 DCR, and JSON token exchange in front of GitHub's OAuth endpoints, and use `--authorization-server` to advertise the proxy's URL in the protected resource metadata instead of the default derived from `--gh-host`:

```bash
# Self-hosted github.com behind an OAuth 2.1 + DCR proxy
github-mcp-server http \
  --base-url https://mcp.example.com \
  --authorization-server https://mcp.example.com/oauth-proxy

# GitHub Enterprise Server behind an OAuth proxy
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

> **Note:** GitHub's hosted remote MCP endpoint (`https://api.githubcopilot.com/mcp/`) advertises `https://github.com/login/oauth` as its authorization server. Clients that require RFC 7591 Dynamic Client Registration against the hosted endpoint must connect through an external OAuth 2.1 + DCR proxy rather than directly to `api.githubcopilot.com`.

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
