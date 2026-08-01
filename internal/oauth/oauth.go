// Package oauth 实现面向用户的 OAuth 2.1 登录流程，stdio server 通过它在没有预先配置的
// Personal Access Token 时获取 GitHub token。
//
// 它同时支持 GitHub OAuth Apps 和 GitHub Apps（user-to-server）。实际区别仅在于 GitHub App user token
// 会过期且带有 refresh token；此 package 始终返回可刷新的 [golang.org/x/oauth2.TokenSource]，调用方无需针对 app 类型做特殊处理。
//
// 本 package 仅依赖 golang.org/x/oauth2 和标准库。MCP 相关事项（session、elicitation）被抽象在 [Prompter]
// interface 后，因此可在没有真实 client 时测试流程。
package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
)

// Config 描述 OAuth client 及其交互的 GitHub endpoint。
type Config struct {
	ClientID     string
	ClientSecret string
	// 授权期间请求的 scope。GitHub Apps 忽略它们（访问权限由已安装权限控制）；OAuth Apps 会遵守它们。
	Scopes []string
	// Endpoint 保存 authorization、token 和 device endpoint。使用 [GitHubEndpoint] 构建。
	Endpoint oauth2.Endpoint
	// CallbackPort 是 PKCE callback server 的固定本地端口。零值会请求随机端口，这是 native binary 的安全默认值，
	// 但无法通过 Docker port mapping 访问（参见 Manager）。
	CallbackPort int
}

// NewGitHubConfig 为给定 GitHub host 构建 Config。空 host 指向 github.com；其他 host 可以是 GHES
// 或 ghe.com hostname，带或不带 scheme 均可。
func NewGitHubConfig(clientID, clientSecret string, scopes []string, host string, callbackPort int) Config {
	return Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       scopes,
		Endpoint:     GitHubEndpoint(host),
		CallbackPort: callbackPort,
	}
}

// GitHubEndpoint 返回 GitHub host 的 OAuth authorization、token 和 device endpoint。空 host 指向 github.com。
func GitHubEndpoint(host string) oauth2.Endpoint {
	base := NormalizeHost(host)
	return oauth2.Endpoint{
		AuthURL:       base + "/login/oauth/authorize",
		TokenURL:      base + "/login/oauth/access_token",
		DeviceAuthURL: base + "/login/device/code",
	}
}

// NormalizeHost 将用户提供的 host 转换为不带尾部斜杠的 scheme+host base URL。会移除 API subdomain，
// 因为 OAuth endpoint 位于 web host 而非 API host（api.github.com -> github.com）。空 host 会得到
// github.com 默认值，因此调用方也可用它识别默认 host（NormalizeHost(host) == "https://github.com"）。
func NormalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return "https://github.com"
	}

	scheme := "https"
	switch {
	case strings.HasPrefix(host, "https://"):
		host = strings.TrimPrefix(host, "https://")
	case strings.HasPrefix(host, "http://"):
		scheme = "http"
		host = strings.TrimPrefix(host, "http://")
	}

	// 丢弃 path、query 和 fragment；只需要 scheme://host。
	if i := strings.IndexAny(host, "/?#"); i >= 0 {
		host = host[:i]
	}

	host = strings.TrimPrefix(host, "api.")

	return fmt.Sprintf("%s://%s", scheme, host)
}

// randomState 返回用作 OAuth state 参数（CSRF protection）和 elicitation ID 的加密随机 URL-safe 字符串。
func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating random state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
