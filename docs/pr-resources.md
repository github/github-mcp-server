# Pull Request Resources

The GitHub MCP Server supports accessing files and content from Pull Requests (PRs) as MCP resources. This allows AI tools to read and analyze code changes in pull requests for various purposes like code review, documentation, and analysis.

## Resource URI Template

Pull Request resources use the following URI template:

```
repo://{owner}/{repo}/refs/pull/{prNumber}/head/contents{/path*}
```

## Parameters

- **`owner`** (required): The GitHub username or organization that owns the repository
- **`repo`** (required): The name of the repository
- **`prNumber`** (required): The pull request number (not the pull request ID)
- **`path`** (optional): The path to a specific file or directory within the pull request

## How It Works

When you request a PR resource, the server:

1. Fetches the pull request information from GitHub's API to get the latest commit SHA from the PR's head branch
2. Uses that commit SHA to retrieve the file content at that specific point in time
3. Returns the content as either text or binary (base64-encoded) depending on the file type

## Examples

### Basic Usage

Access the README file from PR #123:
```
repo://microsoft/vscode/refs/pull/123/head/contents/README.md
```

Access a specific file in a subdirectory:
```
repo://facebook/react/refs/pull/456/head/contents/packages/react/src/React.js
```

### Valid Path Values

The `path` parameter supports:

- **Single files**: `README.md`, `package.json`, `src/index.js`
- **Nested files**: `src/components/Button/Button.tsx`, `docs/api/authentication.md`  
- **Files with special characters**: `docs/how-to-use-@scoped-packages.md`
- **Files in any directory depth**: `very/deep/nested/directory/structure/file.txt`

### Supported File Types

The server automatically detects file types and returns appropriate content:

- **Text files** (`.md`, `.js`, `.py`, `.go`, etc.): Returned as text with proper MIME type
- **Binary files** (`.png`, `.jpg`, `.pdf`, etc.): Returned as base64-encoded blobs
- **Configuration files** (`.json`, `.yaml`, `.toml`, etc.): Returned as text with appropriate MIME type

## Use Cases

### Code Review
```
repo://owner/repo/refs/pull/789/head/contents/src/new-feature.js
```
Access the implementation of a new feature to provide code review feedback.

### Documentation Analysis
```
repo://owner/repo/refs/pull/234/head/contents/docs/api-changes.md
```
Review documentation changes in a pull request.

### Test Coverage
```
repo://owner/repo/refs/pull/567/head/contents/tests/new-feature.test.js
```
Examine test files to understand test coverage for new features.

### Configuration Changes
```
repo://owner/repo/refs/pull/890/head/contents/.github/workflows/ci.yml
```
Review changes to CI/CD workflows or other configuration files.

## Error Handling

The server will return appropriate errors for common scenarios:

- **Pull request not found**: If the PR number doesn't exist
- **File not found**: If the specified path doesn't exist in the PR
- **Invalid PR number**: If the PR number is not a valid integer
- **Access denied**: If the GitHub token doesn't have permission to access the PR or repository

## Limitations

- **Directories are not supported**: You can only access individual files, not directory listings
- **Private repositories**: Require appropriate GitHub token permissions
- **Large files**: Very large files may hit GitHub API limits
- **Binary files**: Returned as base64-encoded content which may be large

## Related Resources

- [Repository Resources](../README.md#tools) - For accessing files from the main branch
- [Branch Resources](../README.md#tools) - For accessing files from specific branches  
- [Tag Resources](../README.md#tools) - For accessing files from specific tags
- [Commit Resources](../README.md#tools) - For accessing files from specific commits

## Tips

1. **Use specific paths**: Always specify the full path to the file you want to access
2. **Check PR status**: Ensure the PR is still open and accessible before attempting to access resources
3. **Handle errors gracefully**: Always implement proper error handling for resource access
4. **Consider file size**: Be mindful of large files that may impact performance