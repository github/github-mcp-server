# GitHub Personal Access Token Setup Guide for OwlBan Group

## Step 1: Create a GitHub Personal Access Token (PAT)

1. Go to [GitHub Personal Access Tokens](https://github.com/settings/personal-access-tokens/new)
2. Click "Generate new token" → "Generate new token (classic)"
3. Give it a descriptive name, e.g., "OwlBan MCP Server"
4. Set expiration (recommend 90 days or never for organizational use)
5. Select the following scopes:

### Required Scopes for Basic Functionality:
- [x] `repo` - Full control of private repositories
- [x] `read:org` - Read org and team membership
- [x] `read:user` - Read ALL user profile data
- [x] `notifications` - Access notifications

### Additional Recommended Scopes:
- [x] `read:packages` - Download packages from GitHub Package Registry
- [x] `read:project` - Read project boards
- [x] `read:discussion` - Read team discussions

### Advanced Scopes (if needed):
- [ ] `write:repo_hook` - Write repository hooks
- [ ] `admin:repo_hook` - Full control of repository hooks
- [ ] `project` - Full control of projects
- [ ] `write:org` - Read and write org and team membership
- [ ] `admin:org` - Full control of orgs and teams

6. Click "Generate token"
7. **IMPORTANT**: Copy the token immediately - you won't see it again!

## Step 2: Secure Token Storage

### Option A: Environment Variable (Recommended)
```bash
# Add to your shell profile (.bashrc, .zshrc, etc.)
export OWLBAN_GITHUB_PAT=your_token_here

# Or create a .env file (add to .gitignore!)
echo "OWLBAN_GITHUB_PAT=your_token_here" > .env
```

### Option B: Password Manager
Store the token in a secure password manager like:
- 1Password
- LastPass
- Bitwarden
- KeePass

### Option C: VS Code Secrets (for VS Code users)
VS Code can store secrets securely using the MCP configuration.

## Step 3: Test Token Validity

Run this command to verify your token works:
```bash
curl -H "Authorization: token YOUR_TOKEN_HERE" https://api.github.com/user
```

You should see your GitHub user information in JSON format.

## Security Best Practices

1. **Never commit tokens to version control**
2. **Use the minimum required scopes**
3. **Rotate tokens regularly** (every 90 days)
4. **Use different tokens for different purposes**
5. **Monitor token usage** in GitHub settings
6. **Revoke tokens immediately** if compromised

## Token Permissions Reference

| Scope | Description | Use Case |
|-------|-------------|----------|
| `repo` | Full private repo access | Read/write code, issues, PRs |
| `read:org` | Read organization data | Access org repos, teams |
| `read:user` | Read user profile | Get user info, repos |
| `notifications` | Access notifications | Read GitHub notifications |
| `read:packages` | Download packages | Access GitHub Package Registry |
| `read:project` | Read projects | Access project boards |
| `read:discussion` | Read discussions | Access team discussions |

## Troubleshooting

### Token Not Working?
- Check token hasn't expired
- Verify correct scopes are selected
- Ensure token wasn't accidentally revoked
- Try regenerating the token

### Permission Denied?
- Organization may require SSO authorization
- Check if token has required scopes
- Verify you're a member of the organization

### Rate Limiting?
- Authenticated requests have higher limits (5,000 vs 60/hour)
- Check GitHub API status: https://www.githubstatus.com/
