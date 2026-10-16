# Remote GitHub MCP server in FLUJO

Connect [FLUJO](https://flujo.com.co/) to the hosted GitHub MCP server and verify a repository read using its built-in tool tester. This guide covers a manual Bearer header and the read-only repository toolset.

## Prerequisites

- A running FLUJO instance.
- A GitHub access token authorized to read the repository you want to test. The recorded validation used the token from an existing authorized GitHub CLI session, available with `gh auth token`. Paste the token only into the secret header field below.

See the [remote server documentation](../remote-server.md) for authentication and toolset options.

## Connect and test

1. In FLUJO, open **Connected Apps > Connect App > I have connection details > At a remote URL**.
2. Enter `https://api.githubcopilot.com/mcp/x/repos/readonly` and select **Connect**.
3. Select **Continue to setup**. The initial test without a header will report that authentication is required.
4. Under **Custom HTTP headers**, select **Add header**. Set **Header** to `Authorization`, enable **Secret**, then enter `Bearer YOUR_ACCESS_TOKEN` as the value. Replace the placeholder with your token.
5. Leave the OAuth client fields blank for this manual-header route. Select **3) Test run**.
6. After the MCP handshake passes and tools are discovered, select **Update server**.
7. Open the saved server's **Tools** tab and choose `get_file_contents`.
8. Enter the repository's `owner`, `repo`, and file `path`, then select **Test tool**. The result should contain the requested file.

The endpoint restricts the exposed tools to repository reads. Token permissions still determine which repositories can be read; use only the permissions needed for your task.

## Validation scope

FLUJO 3.45.2 completed the authenticated handshake, discovered 13 tools, and retrieved a public text fixture through this route. The test supplied a GitHub CLI OAuth access token as a secret Bearer header; it did not test FLUJO's interactive OAuth flow or PAT-specific behavior.

Local-server setup and other toolsets are not covered here. Interactive OAuth requires a registered GitHub App or OAuth app, as explained in the [host support notes](README.md#support-by-host-application).
