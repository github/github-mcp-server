# 端到端 (e2e) 测试

E2E 测试的目的是提供一个（目前）简单的测试，使维护者对构建产物的黑盒行为有一定信心。它通过以下方式实现：
 * 构建 `github-mcp-server` docker image
 * 运行该 image
 * 通过 stdio 与 server 交互
 * 发出与实时 GitHub API 交互的请求

## 运行测试

必须运行支持通过 `docker` CLI 构建 image 和创建 container 的服务。

由于这些测试需要 token 来与 GitHub API 上的真实资源交互，因此受到 `e2e` build flag 的限制。

```
GITHUB_MCP_SERVER_E2E_TOKEN=<YOUR TOKEN> go test -v --tags e2e ./e2e
```

`GITHUB_MCP_SERVER_E2E_TOKEN` environment variable 在内部映射到 `GITHUB_PERSONAL_ACCESS_TOKEN`，但两者分开以避免意外复用凭据。

## 示例

以下 diff 调整 `get_me` 工具，使其返回 `foobar` 作为用户 login。

```diff
diff --git a/pkg/github/context_tools.go b/pkg/github/context_tools.go
index 1c91d70..ac4ef2b 100644
--- a/pkg/github/context_tools.go
+++ b/pkg/github/context_tools.go
@@ -39,6 +39,8 @@ func GetMe(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mc
                                return mcp.NewToolResultError(fmt.Sprintf("failed to get user: %s", string(body))), nil
                        }

+                       user.Login = sPtr("foobar")
+
                        r, err := json.Marshal(user)
                        if err != nil {
                                return nil, fmt.Errorf("failed to marshal user: %w", err)
@@ -47,3 +49,7 @@ func GetMe(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mc
                        return mcp.NewToolResultText(string(r)), nil
                }
 }
+
+func sPtr(s string) *string {
+       return &s
+}
```

运行测试：

```
➜ GITHUB_MCP_SERVER_E2E_TOKEN=$(gh auth token) go test -v --tags e2e ./e2e
=== RUN   TestE2E
    e2e_test.go:92: Building Docker image for e2e tests...
    e2e_test.go:36: Starting Stdio MCP client...
=== RUN   TestE2E/Initialize
=== RUN   TestE2E/CallTool_get_me
    e2e_test.go:85:
                Error Trace:    /Users/williammartin/workspace/github-mcp-server/e2e/e2e_test.go:85
                Error:          Not equal:
                                expected: "foobar"
                                actual  : "williammartin"

                                Diff:
                                --- Expected
                                +++ Actual
                                @@ -1 +1 @@
                                -foobar
                                +williammartin
                Test:           TestE2E/CallTool_get_me
                Messages:       expected login to match
--- FAIL: TestE2E (1.05s)
    --- PASS: TestE2E/Initialize (0.09s)
    --- FAIL: TestE2E/CallTool_get_me (0.46s)
FAIL
FAIL    github.com/github/github-mcp-server/e2e 1.433s
FAIL
```

## 调试测试

可以提供 `GITHUB_MCP_SERVER_E2E_DEBUG=true`，以使用 MCP server 的进程内版本运行 e2e 测试。由于它不集成 Docker，也不使用 cobra/viper configuration parsing，因此覆盖范围略有降低。但它允许在 MCP Server 内部设置断点，支持比完全黑盒测试更好的调试流程。

有人可能会认为，黑盒测试中对失败缺乏可见性也表明存在产品需求，但这解决了维护者当前面临的痛点。

## 限制

当前测试套件的范围有意保持得非常有限。这是因为 e2e 测试的维护成本往往会随时间显著增加。要了解 GitHub integration tests 的一些挑战，请参阅 [go-github integration tests README](https://github.com/google/go-github/blob/5b75aa86dba5cf4af2923afa0938774f37fa0a67/test/README.md)。我们会审慎地扩展此套件！

这些测试相当重复且冗长。这是有意为之，因为我们希望在引入抽象前先观察它们进一步发展。

目前，对失败的可见性并不理想。我们希望能够拆分 mcp-go client，使其无需 exec 即可接入表示 stdio 的 streams。这样就能轻松在 debugger 中设置断点。

### 全局状态变更测试

一些工具（例如将所有通知标为已读的工具）会改变测试人员的全局状态，并且不具备幂等性，因此它们对端到端测试价值不大，应改为依赖 unit testing 和手动验证。
