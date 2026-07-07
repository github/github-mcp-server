# End To End (e2e) Tests

The purpose of the E2E tests is to have a simple (currently) test that gives maintainers some confidence in the black box behavior of our artifacts. It does this by:
 * Building the `github-mcp-server` docker image
 * Running the image
 * Interacting with the server via stdio
 * Issuing requests that interact with the live GitHub API

## Running the Tests

A service must be running that supports image building and container creation via the `docker` CLI.

Since these tests require a token to interact with real resources on the GitHub API, it is gated behind the `e2e` build flag.

```
GITHUB_MCP_SERVER_E2E_TOKEN=<YOUR TOKEN> go test -v --tags e2e ./e2e
```

The `GITHUB_MCP_SERVER_E2E_TOKEN` environment variable is mapped to `GITHUB_PERSONAL_ACCESS_TOKEN` internally, but separated to avoid accidental reuse of credentials.

## Example

The following diff adjusts the `get_me` tool to return `foobar` as the user login.

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

Running the tests:

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

## Debugging the Tests

It is possible to provide `GITHUB_MCP_SERVER_E2E_DEBUG=true` to run the e2e tests with an in-process version of the MCP server. This has slightly reduced coverage as it doesn't integrate with Docker, or make use of the cobra/viper configuration parsing. However, it allows for placing breakpoints in the MCP Server internals, supporting much better debugging flows than the fully black-box tests.

One might argue that the lack of visibility into failures for the black box tests also indicates a product need, but this solves for the immediate pain point felt as a maintainer.

## Limitations

The current test suite is intentionally very limited in scope. This is because the maintenance costs on e2e tests tend to increase significantly over time. To read about some challenges with GitHub integration tests, see [go-github integration tests README](https://github.com/google/go-github/blob/5b75aa86dba5cf4af2923afa0938774f37fa0a67/test/README.md). We will expand this suite circumspectly!

The tests are quite repetitive and verbose. This is intentional as we want to see them develop more before committing to abstractions.

Currently, visibility into failures is not particularly good. We're hoping that we can pull apart the mcp-go client and have it hook into streams representing stdio without requiring an exec. This way we can get breakpoints in the debugger easily.

### Global State Mutation Tests

Some tools (such as those that mark all notifications as read) would change the global state for the tester, and are also not idempotent, so they offer little value for end to end tests and instead should rely on unit testing and manual verifications.
