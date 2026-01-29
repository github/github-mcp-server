# MCP Host Configurations for OwlBan GitHub MCP Server

This directory contains configuration files for different MCP-compatible hosts to connect to the GitHub MCP Server.

## Available Configurations

### 1. VS Code (`vscode-settings.json`)
- **Location**: Add to VS Code User Settings (JSON)
- **Features**: Secure token input, automatic prompting
- **Setup**: Copy the content to your VS Code settings.json

### 2. Claude Desktop (`claude-config.json`)
- **Location**: Claude Desktop configuration file
- **Note**: Replace `YOUR_PAT_HERE` with your actual GitHub PAT
- **Setup**: Add to Claude's MCP configuration

### 3. Cursor (`cursor-config.json`)
- **Location**: Cursor settings or workspace configuration
- **Note**: Replace `YOUR_PAT_HERE` with your actual GitHub PAT
- **Setup**: Add to Cursor's MCP configuration

## Setup Instructions

1. **Get your GitHub PAT** using the guide in `../OWLBAN-PAT-GUIDE.md`
2. **Choose your MCP host** from the configurations above
3. **Configure the host** using the appropriate file
4. **Replace placeholder tokens** with your actual PAT
5. **Test the connection** by asking the AI to list your repositories

## Security Notes

- Never commit configuration files with real tokens
- Use environment variables when possible
- Rotate tokens regularly
- Use minimum required scopes

## Troubleshooting

- **Connection fails**: Check token validity and scopes
- **Permission errors**: Verify organization access and token permissions
- **Rate limiting**: Authenticated requests have higher limits (5,000/hour)

## Alternative: Use the Setup Script

For the easiest setup, run `../setup-owlban.bat` which will guide you through the process interactively.
