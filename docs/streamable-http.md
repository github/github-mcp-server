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
