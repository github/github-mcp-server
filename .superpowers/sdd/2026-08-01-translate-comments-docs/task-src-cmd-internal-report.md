# cmd/internal 源码注释中文化报告

## 状态

部分完成。已将本次实际修改文件中的英文自然语言注释翻译为中文，并保留 exported doc comment 的标识符前缀、协议名、命令、URL、环境变量和代码标识符。未创建 commit。

## 已修改文件

- `cmd/github-mcp-server/feature_flag_docs.go`
- `cmd/github-mcp-server/helpers.go`
- `cmd/github-mcp-server/list_scopes.go`
- `cmd/mcpcurl/main.go`
- `cmd/mcpcurl/main_test.go`
- `internal/buildinfo/buildinfo.go`
- `internal/githubapp/githubapp.go`
- `internal/githubapp/githubapp_test.go`
- `internal/oauth/callback.go`
- `internal/oauth/callback_test.go`
- `internal/oauth/oauth.go`
- `internal/profiler/profiler.go`
- `internal/toolsnaps/toolsnaps.go`
- `internal/toolsnaps/toolsnaps_test.go`
- `script/print-mcp-diff-configs/main.go`

## 验证

- `gofmt -w` 已在以上全部修改的 Go 文件执行。
- `git diff --check` 退出码为 0。
- `go test ./cmd/mcpcurl ./cmd/github-mcp-server ./internal/buildinfo ./internal/githubapp ./internal/oauth ./internal/profiler ./internal/toolsnaps ./script/print-mcp-diff-configs` 退出码为 0。

## 疑虑与排除项

- 范围内尚有未处理的英文注释，主要位于 `cmd/github-mcp-server/main.go`、`generate_docs.go`、`internal/ghmcp`、`internal/oauth/flow.go` 与 `internal/oauth/manager.go` 等；因此本子任务不应标记为完全完成。
- `internal/githubv4mock/query.go` 和 `objects_are_equal_values_test.go` 含上游复制内容及许可证，按 third-party 排除要求未修改；同目录其他文件也未在本轮处理。
- 工作区原有的删除、`docs/superpowers/` 未跟踪文件和 `cmd/mcpcurl/README.md` 变更均未触碰。
