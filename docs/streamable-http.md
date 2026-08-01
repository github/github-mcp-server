# Streamable HTTP Server

Streamable HTTP 模式使 GitHub MCP Server 可作为 HTTP 服务运行，允许客户端通过标准 HTTP protocol 连接。此模式适用于 stdio transport 不合适的部署场景，例如反向代理配置、容器化环境或分布式架构。

## 功能

- **Streamable HTTP Transport**：支持实时工具响应流的完整 HTTP server
- **OAuth Metadata Endpoint**：为 OAuth 客户端提供标准 `.well-known/oauth-protected-resource` discovery
- **Scope Challenge 支持**：自动验证 scope，并返回正确的 HTTP 403 响应和 `WWW-Authenticate` header
- **Scope Filtering**：根据已验证的 credentials 和权限限制可用工具
- **自定义 Base Path**：支持具有可自定义 base URL 的反向代理部署

## 运行 server

### 基本 HTTP Server

在默认 port（8082）上启动 server：

```bash
github-mcp-server http
```

server 将在 `http://localhost:8082` 上可用。

### 使用 Scope Challenge

启用 scope 验证以强制执行 GitHub 权限检查：

```bash
github-mcp-server http --scope-challenge
```

启用 `--scope-challenge` 后，scope 不足的请求会收到 `403 Forbidden` 响应，并带有指明所需 scope 的 `WWW-Authenticate` header。

### 使用 OAuth Metadata Discovery

如需在反向代理后或使用自定义 domain，需公开 OAuth metadata endpoint：

```bash
github-mcp-server http --scope-challenge --base-url https://myserver.com --base-path /mcp
```

OAuth protected resource metadata 的 `resource` attribute 将填充为 server protected resource endpoint 的完整 URL：

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

这使 OAuth 客户端能够自动发现身份验证要求和 endpoint 信息。

### 位于受信任的 Proxy 后（高级）

默认情况下，server 在构建 OAuth resource metadata URL 时会忽略 `X-Forwarded-Host` 和 `X-Forwarded-Proto` header，因此不受信任的客户端无法影响向 MCP 客户端公布的 URL。对于大多数部署，将 `--base-url` 设置为外部可见 URL 是正确方式。

如果 server 位于您完全控制的内部 forwarder 之后（例如需要为每个请求保留源 hostname 的 cluster 内 gateway），可以选择信任这些 header：

```bash
github-mcp-server http --trust-proxy-headers
```

等效的环境变量为 `GITHUB_TRUST_PROXY_HEADERS=1`。仅当信任上游 proxy 会设置或移除这些 header 时才启用此项；否则应使用 `--base-url`。设置 `--base-url` 后，它始终优先，`--trust-proxy-headers` 不会生效。

## 客户端配置

### 使用 OAuth 身份验证

如果您的 IDE 或客户端已配置 GitHub credentials（即 VS Code），只需引用 HTTP server：

```json
{
  "type": "http",
  "url": "http://localhost:8082"
}
```

server 将使用客户端现有的 GitHub 身份验证。

### 使用 Bearer Token 或自定义 Header

如需提供 PAT credentials 或自定义 server 行为偏好，可在客户端配置中包含额外 header：

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

有关客户端配置选项的更多信息，请参阅 [Remote Server](./remote-server.md) 文档。
