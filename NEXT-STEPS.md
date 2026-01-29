# Next Steps: Activate Your OwlBan GitHub MCP Server

## Immediate Actions (Do These First)

### 1. Get Your GitHub Personal Access Token
- Follow `OWLBAN-PAT-GUIDE.md` to create a PAT
- Required scopes: `repo`, `read:org`, `read:user`, `notifications`
- **Save this token securely** - you'll need it in step 2

### 2. Run the Setup Script
```bash
# Double-click or run in terminal:
setup-owlban.bat
```
- Enter your GitHub PAT when prompted
- The server will start in Docker automatically

### 3. Configure Your MCP Host Application

#### For VS Code (Recommended):
1. Open VS Code
2. Go to Settings (Ctrl+,)
3. Search for "mcp"
4. Add the configuration from `owlban-mcp-config.json`

#### For Claude Desktop:
1. Open Claude Desktop
2. Go to Settings → MCP
3. Add server configuration from `mcp-host-configs/claude-config.json`
4. Replace `YOUR_PAT_HERE` with your actual token

#### For Cursor:
1. Open Cursor
2. Go to Settings → MCP
3. Add server configuration from `mcp-host-configs/cursor-config.json`
4. Replace `YOUR_PAT_HERE` with your actual token

## Test the Setup

Once configured, test with these commands in your AI assistant:

- "Show me my GitHub repositories"
- "List my recent issues"
- "Check the status of my pull requests"
- "Get information about repository [repo-name]"

## Advanced Configuration

### Customize Toolsets (Optional)
Edit `owlban-toolsets.json` to enable/disable specific GitHub features:
- Basic: `context,repos,issues,pull_requests,users`
- Development: Add `actions,code_security`
- Full: Use `all` for complete access

### Environment Variables
For production use, set these environment variables:
```bash
export GITHUB_PERSONAL_ACCESS_TOKEN=your_token
export GITHUB_TOOLSETS="context,repos,issues,pull_requests,users"
```

## Troubleshooting

### Server Won't Start?
- Check Docker is running: `docker --version`
- Verify token is valid and has correct scopes
- Check network connectivity to GitHub

### Permission Errors?
- Ensure token has required scopes
- Verify you're a member of the target organizations
- Check token hasn't expired

### AI Assistant Can't Connect?
- Confirm MCP configuration is correct
- Restart your AI application
- Check the server logs in the terminal

## Support & Resources

- **Documentation**: `OWLBAN-SETUP-README.md`
- **Token Guide**: `OWLBAN-PAT-GUIDE.md`
- **Host Configs**: `mcp-host-configs/README.md`
- **Production Guide**: `PRODUCTION-SETUP-GUIDE.md`

## What's Working Now

✅ GitHub repository access and management
✅ Issue creation, updates, and tracking
✅ Pull request operations and reviews
✅ User and organization information
✅ GitHub Actions workflow monitoring
✅ Security scanning and alerts
✅ Code search and analysis

Your OwlBan team can now leverage AI-powered GitHub operations through natural language commands!

---

**Need Help?** Refer to the documentation files or check the server logs for error messages.
