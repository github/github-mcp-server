# GitHub App 身份验证

本地 stdio 服务器无需浏览器、设备流或 elicitation，即可作为 GitHub App installation 进行身份验证。它使用 App 的私钥签署短期 JWT，将其交换为 installation access token，并在 token 过期前刷新。

此身份验证模式不适用于 `http` 命令。HTTP 客户端必须继续提供自己的 `Authorization` token。

> [!WARNING]
> 私钥可以为 installation 获授的每个 repository 和权限签发 token。请勿将其纳入源代码管理，限制服务器进程的访问权限，并且只在所需的 repository 上安装此 App。

## 配置

在 Personal Access Token、OAuth 登录和 GitHub App 身份验证中，只能配置其中一种。

| Flag | 环境变量 | 描述 |
|------|----------------------|-------------|
| `--app-id` | `GITHUB_APP_ID` | 用作 JWT issuer 的 App ID 或 client ID |
| `--app-installation-id` | `GITHUB_APP_INSTALLATION_ID` | 使用其 access token 的 installation |
| `--app-private-key-path` | `GITHUB_APP_PRIVATE_KEY_PATH` | 私钥 PEM 的路径 |
| _(无)_ | `GITHUB_APP_PRIVATE_KEY` | PEM 内容，可选择包含字面量 `\n` 转义符 |

建议使用挂载的私钥文件。由于命令行参数可能会被其他进程读取，因此没有用于内联 PEM 内容的 flag。

## 使用方法

```bash
github-mcp-server stdio \
  --app-id 123456 \
  --app-installation-id 7891011 \
  --app-private-key-path /secrets/github-app.pem
```

等效的环境变量配置为：

```bash
export GITHUB_APP_ID=123456
export GITHUB_APP_INSTALLATION_ID=7891011
export GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem
github-mcp-server stdio
```

对于 Docker，请以只读方式挂载密钥：

```bash
docker run -i --rm \
  -v /secrets/github-app.pem:/secrets/github-app.pem:ro \
  -e GITHUB_APP_ID=123456 \
  -e GITHUB_APP_INSTALLATION_ID=7891011 \
  -e GITHUB_APP_PRIVATE_KEY_PATH=/secrets/github-app.pem \
  ghcr.io/github/github-mcp-server
```

对于 GitHub Enterprise Server 或 `ghe.com`，还需设置 `--gh-host` 或 `GITHUB_HOST`。服务器将从该 host 推导 installation-token endpoint。

## 故障排除

- **需要私钥**：设置 `GITHUB_APP_PRIVATE_KEY_PATH` 或 `GITHUB_APP_PRIVATE_KEY`。
- **私钥无效**：提供 GitHub App 设置中生成的 RSA PEM。支持 PKCS#1 和 PKCS#8 密钥。
- **installation-token endpoint 返回 401**：验证 app ID 或 client ID、私钥、目标 host 和系统时钟。
- **installation-token endpoint 返回 404**：验证 installation ID，并确认 App 已安装在目标 host 上。
