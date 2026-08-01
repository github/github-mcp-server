# 测试

本项目结合使用单元测试和端到端（e2e）测试来确保正确性和稳定性。

## 单元测试模式

- 单元测试与实现代码放在一起，文件名以 `_test.go` 结尾。
- 当前优先使用内部测试，即测试文件不使用 `_test` package 后缀。
- 测试使用 [testify](https://github.com/stretchr/testify) 进行断言和 require 语句。在继续测试没有意义时使用 `require`；例如，断言发生错误后几乎从不应继续执行测试。
- REST mock 使用仓库内的 `MockHTTPClientWithHandlers` helper；GraphQL mock 使用 `githubv4mock`。
- 每个工具的 schema 都会通过 `toolsnaps` utility 生成快照并检查变更（见下文）。
- 测试应保持明确且详细，以利于可维护性和清晰度。
- handler 单元测试应采用以下形式：
    1. 测试工具快照
    1. 对 schema 的关键断言（例如 `ReadOnly` annotation）
    1. 表驱动形式的行为测试

## 端到端（e2e）测试

- E2E 测试位于 [`e2e/`](../e2e/) 目录。有关运行和调试这些测试的完整说明，请参阅 [e2e/README.md](../e2e/README.md)。

## toolsnaps：工具 schema 快照

- `toolsnaps` utility 确保每个工具的 JSON schema 不会意外改变。
- 快照存储在 `__toolsnaps__/*.snap` 文件中，其中 `*` 表示工具名称。
- 运行测试时，会将当前工具 schema 与快照进行比较。如有差异，测试将失败并显示 diff。
- 如果有意变更工具的 schema，请使用环境变量运行测试以更新快照：`UPDATE_TOOLSNAPS=true go test ./...`
- 在 CI 中（当 `GITHUB_ACTIONS=true` 时），缺失的快照会导致测试失败，以确保快照始终已提交。

## 说明

- 某些会修改全局状态的工具（例如将所有通知标记为已读）主要使用单元测试而非 e2e 测试，以避免副作用。
- 有关 e2e 测试套件的限制和理念，请参阅 [e2e/README.md](../e2e/README.md)。
