// Package buildinfo 包含通过 ldflags 在构建时设置的变量。
// 这些变量让正式版本可以携带默认 OAuth 凭据，使用户无需配置自己的 OAuth app 即可登录。
// 这些值实际上是公开的（安全性依赖 PKCE 而非 client secret），但仍不写入源码，而是在构建时注入。
//
// 示例：
//
//	go build -ldflags="-X github.com/github/github-mcp-server/internal/buildinfo.OAuthClientID=xxx"
package buildinfo

// OAuthClientID 是默认 OAuth client ID，在构建时设置。本地/开发构建中为空。
var OAuthClientID string

// OAuthClientSecret 是默认 OAuth client secret，在构建时设置。对于公共 OAuth client，
// 依据 OAuth 2.1 它并不是真正的 secret（安全性由 PKCE 提供），但仍在构建时注入而非提交。
var OAuthClientSecret string
