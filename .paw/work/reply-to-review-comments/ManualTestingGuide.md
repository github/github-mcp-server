# Manual Testing Guide: Reply to Review Comments

This guide walks through manually testing the `reply_to_review_comment` MCP tool by running the GitHub MCP Server in a Docker container and using VS Code with GitHub Copilot to reply to actual review comments.

## Prerequisites

- Docker installed and running
- VS Code with GitHub Copilot extension installed
- GitHub account with access to a test repository
- GitHub Personal Access Token (PAT) with `repo` scope
  - Create at: https://github.com/settings/tokens/new
  - Required scopes: `repo` (includes `repo:status`, `repo_deployment`, `public_repo`, `repo:invite`)
- Test repository where you can create PRs and review comments

## Part 1: Build the Docker Image

### Step 1: Build the Docker Image

From the repository root:

```bash
# Build the Docker image
docker build -t github-mcp-server:test .
```

**Expected Output:**
```
[+] Building 45.2s (18/18) FINISHED
 => [internal] load build definition from Dockerfile
 => => transferring dockerfile: 1.23kB
 => [internal] load metadata for gcr.io/distroless/static-debian12:latest
 => [internal] load metadata for docker.io/library/golang:1.25.3-alpine
 ...
 => exporting to image
 => => exporting layers
 => => writing image sha256:...
 => => naming to docker.io/library/github-mcp-server:test
```

### Step 2: Verify the Image

```bash
# List Docker images
docker images github-mcp-server:test
```

**Expected Output:**
```
REPOSITORY            TAG       IMAGE ID       CREATED          SIZE
github-mcp-server     test      abc123def456   30 seconds ago   ~60MB
```

## Part 2: Prepare Test Repository

### Step 1: Create a Test Branch and PR

```bash
# Clone your test repository (or use existing)
cd /path/to/your/test-repo

# Create a feature branch
git checkout -b test-reply-to-review-comments

# Make a small code change
echo "// Testing reply to review comments feature" >> README.md
git add README.md
git commit -m "Test: Add comment for review testing"

# Push to GitHub
git push origin test-reply-to-review-comments
```

### Step 2: Create Pull Request

1. Go to GitHub and create a PR from `test-reply-to-review-comments` to your default branch
2. Note the PR number (e.g., `#42`)

### Step 3: Add Review Comments

1. Go to the "Files changed" tab in your PR
2. Click the `+` icon next to a line in the diff
3. Add 2-3 review comments at different locations:
   - **Comment 1**: "Consider adding more context here"
   - **Comment 2**: "This could be more descriptive"
   - **Comment 3**: "Add error handling for this case"
4. Click "Start a review" for the first comment, then "Add review comment" for subsequent comments
5. Click "Finish your review" → "Comment" (don't approve or request changes yet)

**Record the following:**
- Repository owner: `<your-username-or-org>`
- Repository name: `<your-repo-name>`
- Pull request number: `<pr-number>`

## Part 3: Configure VS Code to Use the MCP Server

### Step 1: Create MCP Configuration Directory

```bash
# Create the VS Code MCP configuration directory
mkdir -p ~/.vscode/mcp
```

### Step 2: Create MCP Server Configuration

Create or edit `~/.vscode/mcp/servers.json` with the following content:

```json
{
  "mcpServers": {
    "github-test": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN=YOUR_GITHUB_PAT_HERE",
        "github-mcp-server:test",
        "stdio"
      ],
      "description": "GitHub MCP Server (test build for reply-to-review-comments feature)"
    }
  }
}
```

**Important**: Replace `YOUR_GITHUB_PAT_HERE` with your actual GitHub Personal Access Token.

**Alternative: Test with Specific Toolset**

To limit to only repository_management tools (faster initialization):

```json
{
  "mcpServers": {
    "github-test": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-e",
        "GITHUB_PERSONAL_ACCESS_TOKEN=YOUR_GITHUB_PAT_HERE",
        "-e",
        "GITHUB_TOOLSETS=repository_management",
        "github-mcp-server:test",
        "stdio"
      ],
      "description": "GitHub MCP Server (test build - repository_management toolset only)"
    }
  }
}
```

### Step 3: Restart VS Code

Close and reopen VS Code to load the new MCP server configuration.

### Step 4: Verify MCP Server Connection

1. Open the Command Palette (`Cmd+Shift+P` on macOS, `Ctrl+Shift+P` on Linux/Windows)
2. Type "GitHub Copilot: List Available MCP Servers"
3. Verify that `github-test` appears in the list
4. Check that the status shows "Connected" or "Running"

**Troubleshooting**: If the server doesn't appear or shows as disconnected:
- Check `~/.vscode/mcp/servers.json` for syntax errors
- Verify Docker is running: `docker ps`
- Check VS Code's Output panel (View → Output) and select "GitHub Copilot" to see connection logs

## Part 4: Verify MCP Tools in VS Code

### Step 1: Open GitHub Copilot Chat

1. Open VS Code
2. Click the GitHub Copilot icon in the Activity Bar (left sidebar)
3. Or use keyboard shortcut: `Cmd+Shift+I` (macOS) or `Ctrl+Shift+I` (Windows/Linux)

### Step 2: Verify Tool Access

In the Copilot Chat, type:

```
@github-test list available tools
```

or

```
What MCP tools are available from the github-test server?
```

**Expected Response:**
Copilot should show a list of available tools including `reply_to_review_comment` along with its parameters:
- owner (string): Repository owner
- repo (string): Repository name
- pull_number (number): Pull request number
- comment_id (number): Review comment ID
- body (string): Reply text

**Troubleshooting**: If Copilot doesn't recognize `@github-test`:
- Verify the MCP server is connected (Part 3, Step 4)
- Check that `servers.json` has correct syntax
- Restart VS Code completely (close all windows)

## Part 5: Retrieve Review Comment IDs Using VS Code

### Step 1: Open Your Test Repository in VS Code

```bash
# Open your test repository
code /path/to/your/test-repo
```

### Step 2: Use Copilot Chat to List Review Comments

In the Copilot Chat panel, type (replacing with your actual values):

```
@github-test please use pull_request_read with method "get_review_comments" to list all review comments on pull request #42 in repository your-username/your-repo
```

or more concisely:

```
@github-test get review comments for PR #42 in your-username/your-repo
```

**Expected Response:**
Copilot will execute the tool and show you the review comments with their IDs, similar to:

```
I found 3 review comments:

1. Comment ID: 1234567890
   Body: "Consider adding more context here"
   Path: README.md, Line 10
   
2. Comment ID: 1234567891
   Body: "This could be more descriptive"
   Path: README.md, Line 15
   
3. Comment ID: 1234567892
   Body: "Add error handling for this case"
   Path: main.go, Line 42
```

### Step 3: Note the Comment IDs

Copy the comment IDs from the response. You'll use these in the next part.

## Part 6: Reply to Review Comments Using VS Code

### Step 1: Reply to First Comment (Simple Text)

In the Copilot Chat panel, type:

```
@github-test use reply_to_review_comment to reply to comment ID 1234567890 on PR #42 in your-username/your-repo with this message: "Thanks for the feedback! I've updated the code to include more context in commit abc123."
```

**Expected Response:**
Copilot will execute the tool and show:

```
✓ Reply posted successfully!

Reply ID: 9876543210
URL: https://github.com/your-username/your-repo/pull/42#discussion_r9876543210
```

### Step 2: Verify in GitHub UI

1. Click the URL provided by Copilot (or open the PR in your browser)
2. Navigate to the "Files changed" tab if not already there
3. Scroll to the review comment you replied to
4. **Verify:**
   - Your reply appears indented underneath the original comment
   - The reply text matches what you sent
   - The reply is shown as part of the threaded conversation
   - You (or the PAT owner) appear as the author

### Step 3: Reply to Second Comment (Markdown Formatting)

Test Markdown formatting support. In Copilot Chat:

```
@github-test reply to comment 1234567891 on PR #42 in your-username/your-repo with:

Good point! Here's the updated code:

```markdown
# More Descriptive Heading

This section now includes detailed context.
```

Let me know if this addresses your concern.
```

**Expected Response:**
Copilot posts the reply and provides the reply ID and URL.

**Verify in GitHub UI:**
- Code block renders correctly with syntax highlighting
- Newlines and formatting are preserved
- The reply appears threaded under the original comment

### Step 4: Reply to Third Comment (User Mentions)

Test @mentions. In Copilot Chat:

```
@github-test reply to comment 1234567892 on PR #42 in your-username/your-repo with: "I've added error handling in the latest commit. @reviewer-username, please take another look when you have a chance."
```

**Verify in GitHub UI:**
- User mention is properly linked (clickable, blue)
- Mentioned user receives a notification (check email or GitHub notifications)
- Reply is threaded correctly

### Step 5: Test Natural Language Flow

Try a more conversational approach. In Copilot Chat:

```
@github-test I need to respond to the review comments on PR #42 in my repo your-username/your-repo. 

For comment 1234567890, say: "Fixed! I've renamed the variable to be more descriptive."

For comment 1234567891, say: "Agreed, I've refactored this section for better clarity."
```

**Expected Behavior:**
Copilot should:
1. Understand you want to reply to multiple comments
2. Execute `reply_to_review_comment` twice, once for each comment
3. Provide confirmation for each reply with ID and URL

## Part 7: Test Error Handling in VS Code

### Test 1: Invalid Comment ID (404 Error)

In Copilot Chat:

```
@github-test reply to comment 99999999999 on PR #42 in your-username/your-repo with: "This should fail"
```

**Expected Response:**
Copilot should report an error message:

```
❌ Error: failed to create reply to review comment: Not Found

This comment ID doesn't exist or may have been deleted.
```

### Test 2: Empty Body (422 Validation Error)

In Copilot Chat:

```
@github-test reply to comment 1234567890 on PR #42 in your-username/your-repo with an empty message
```

**Expected Response:**
Copilot should recognize this is invalid and either:
- Refuse to execute the tool (smart handling)
- Or execute and report: `Error: failed to create reply to review comment: Validation Failed`

### Test 3: Wrong Repository

In Copilot Chat:

```
@github-test reply to comment 1234567890 on PR #42 in nonexistent-user/fake-repo with: "This should fail"
```

**Expected Response:**
```
❌ Error: failed to create reply to review comment: Not Found

The repository or pull request may not exist, or you may not have access.
```

## Part 8: Test Advanced VS Code Workflows

### Workflow 1: Review and Respond to All Comments

In Copilot Chat, try a complex multi-step workflow:

```
@github-test help me respond to all review comments on PR #42 in your-username/your-repo. First, list all the review comments with their IDs, then I'll tell you how to respond to each one.
```

**Expected Behavior:**
1. Copilot lists all review comments with IDs
2. You can then say: "Reply to comment X with..." for each
3. Copilot tracks which comments you've responded to

### Workflow 2: Context-Aware Replies

Open a file that was reviewed (e.g., `README.md`), then:

```
@github-test I'm looking at the review comments on this file. Reply to comment 1234567890 on PR #42 saying that I've updated the section they mentioned.
```

**Expected Behavior:**
Copilot understands context from the open file and can reference it in the reply.

### Workflow 3: Batch Replies with Different Messages

```
@github-test I need to reply to multiple review comments on PR #42 in your-username/your-repo:

1. For comment 1234567890: "Fixed in commit abc123"
2. For comment 1234567891: "Good catch! I've refactored this section"
3. For comment 1234567892: "I've added error handling as requested"

Please post all three replies.
```

**Expected Behavior:**
Copilot executes three separate `reply_to_review_comment` calls and confirms each one.

## Part 9: Cleanup

### Step 1: Remove MCP Server Configuration (Optional)

If you want to remove the test MCP server from VS Code:

```bash
# Edit the configuration file
code ~/.vscode/mcp/servers.json

# Remove the "github-test" entry or delete the entire file
rm ~/.vscode/mcp/servers.json
```

Then restart VS Code.

### Step 2: Stop Any Running Docker Containers

The MCP server container is automatically stopped when VS Code closes, but you can verify:

```bash
# Check for any running containers
docker ps | grep github-mcp-server

# Stop any if found
docker stop <container-id>
```

### Step 3: Clean Up Test PR (Optional)

If you want to remove the test PR:

```bash
# Close the PR in GitHub UI or via API
# Delete the test branch
git push origin --delete test-reply-to-review-comments

# Or keep it for future testing
```

### Step 4: Remove Docker Image (Optional)

```bash
# Remove the test image
docker rmi github-mcp-server:test
```

## Success Criteria Checklist

After completing this manual test, verify:

- [ ] Docker image builds successfully without errors
- [ ] VS Code MCP server configuration is created correctly
- [ ] MCP server connects successfully in VS Code
- [ ] Tool is discoverable via Copilot Chat (`@github-test`)
- [ ] `pull_request_read` retrieves review comment IDs successfully via Copilot
- [ ] `reply_to_review_comment` creates replies that appear as threaded responses in GitHub UI
- [ ] Reply ID and URL are returned in Copilot's response
- [ ] Markdown formatting (code blocks) renders correctly in GitHub
- [ ] User mentions (@username) work and trigger notifications
- [ ] Invalid comment ID returns descriptive error message in Copilot Chat
- [ ] Natural language requests are understood by Copilot
- [ ] Batch replies work (multiple comments in one conversation)
- [ ] All replies appear in correct threads at correct code locations
- [ ] Original comment authors receive notifications
- [ ] VS Code integration feels smooth and natural

## Troubleshooting

### Issue: Docker build fails

**Solution:**
- Check Docker is running: `docker ps`
- Ensure you're in the repository root directory
- Check Go version in Dockerfile matches available Alpine images
- Try clearing Docker cache: `docker builder prune`

### Issue: MCP server not appearing in VS Code

**Solution:**
- Verify `~/.vscode/mcp/servers.json` exists and has correct syntax (use a JSON validator)
- Check that your GitHub PAT is correctly placed in the configuration
- Restart VS Code completely (close all windows)
- Check VS Code's Output panel: View → Output → "GitHub Copilot" to see connection logs
- Verify Docker is running: `docker ps`

### Issue: Copilot doesn't recognize @github-test

**Solution:**
- Make sure you're using `@` before `github-test` (e.g., `@github-test`)
- Verify the MCP server name in `servers.json` matches exactly: `"github-test"`
- Check that GitHub Copilot extension is installed and active
- Try reconnecting: Command Palette → "GitHub Copilot: Restart"

### Issue: Tool not found when trying to use it

**Solution:**
- The MCP server may not have started. Check Output panel for errors
- Verify the Docker container is running: `docker ps | grep github-mcp-server`
- Try: `@github-test list available tools` to see what's accessible
- Check token permissions (needs `repo` scope)

### Issue: 404 error on valid comment ID

**Possible causes:**
- Comment ID is from a general PR comment (issue comment), not a review comment
- Comment was deleted after ID was retrieved
- Wrong pull request number provided
- Repository name or owner is incorrect

**Solution:**
- Use `pull_request_read` with `method: "get_review_comments"` to ensure you're getting review comment IDs
- Verify PR number matches the PR containing the comment
- Double-check owner and repo parameters

### Issue: 403 Forbidden error

**Possible causes:**
- PAT lacks write permissions to repository
- Repository is archived
- Organization requires SSO authentication

**Solution:**
- Regenerate PAT with `repo` scope
- Test with a repository you own
- Enable SSO if required by organization

### Issue: Reply appears as separate comment

**This should not happen** - if it does:
- Verify Copilot is using `reply_to_review_comment`, not `add_issue_comment`
- Ask Copilot to clarify: "What tool did you just use?"
- Confirm comment_id is from a review comment (not an issue comment)
- Check GitHub UI to ensure comment exists and PR is open

### Issue: Copilot returns error but doesn't explain clearly

**Solution:**
- Ask Copilot to show the raw error: "What was the exact error message?"
- Check the Output panel: View → Output → "GitHub Copilot" for detailed logs
- Try the operation again with more explicit parameters
- Verify all values (owner, repo, PR number, comment ID) are correct

## Additional Testing Scenarios

### Scenario 1: Batch Replies via Natural Language

Test replying to multiple comments in a natural conversation. In Copilot Chat:

```
@github-test I need to respond to three review comments on PR #42 in your-username/your-repo:

- Reply "Addressed in latest commit. Thanks!" to comment 1234567890
- Reply "Good point, I've refactored this" to comment 1234567891  
- Reply "Fixed! Please review again" to comment 1234567892
```

**Verify:** 
- All three replies are posted successfully
- Each appears in its respective thread
- Copilot confirms all three operations

### Scenario 2: Long Reply with Complex Markdown

Test with a comprehensive reply containing multiple Markdown elements. In Copilot Chat:

```
@github-test reply to comment 1234567890 on PR #42 in your-username/your-repo with this detailed response:

Great feedback! Here's what I've changed:

## Updates

1. Added error handling
2. Improved variable naming
3. Added tests

### Code Example

```go
if err != nil {
    return fmt.Errorf("failed: %w", err)
}
```

### Testing

- [x] Unit tests pass
- [x] Integration tests pass
- [ ] Manual testing pending

cc @reviewer 👍
```

**Verify in GitHub:**
- Headers render correctly (## and ###)
- Numbered list displays properly
- Code block has Go syntax highlighting
- Checkboxes render (checked ✓ and unchecked ☐)
- User mention is blue and clickable
- Emoji displays correctly

### Scenario 3: Unicode and Special Characters

Test with non-ASCII characters. In Copilot Chat:

```
@github-test reply to comment 1234567890 on PR #42 with: "Merci beaucoup! 谢谢! 🎉 This feedback is très valuable. I've updated the code to handle edge cases properly."
```

**Verify:** All characters and emoji display correctly in GitHub UI.

### Scenario 4: Context-Aware Reply

Open the file that was reviewed in VS Code, then:

```
@github-test I'm looking at the code that was reviewed. Reply to comment 1234567890 on PR #42 saying that I've updated lines 15-20 to address their concern about error handling.
```

**Verify:** Copilot understands the context and posts an appropriate reply.

## Conclusion

If all tests pass, the `reply_to_review_comment` feature is working correctly with VS Code and GitHub Copilot and is ready for production use. The tool successfully:

1. Integrates with VS Code via MCP protocol
2. Works seamlessly with GitHub Copilot Chat for natural language interactions
3. Integrates with GitHub's API to post threaded replies
4. Supports Markdown formatting and GitHub features (@mentions, emoji, code blocks)
5. Returns proper response format (MinimalResponse with ID and URL)
6. Handles errors gracefully with descriptive messages in Copilot Chat
7. Works within Docker container with environment-based authentication
8. Maintains thread context at specific code locations
9. Supports both explicit and natural language commands
10. Enables batch operations through conversational workflows

The feature is ready for integration into AI-assisted code review workflows in VS Code, allowing developers to efficiently respond to review comments without leaving their editor.
