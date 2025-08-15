# Install GitHub MCP Server in Gemini CLI

This guide covers installation of the GitHub MCP server for the Gemini CLI.

To set up the remote GitHub MCP (Model Context Protocol) on the Gemini CLI, you'll need to configure your Gemini settings to point to the GitHub MCP server and provide a Personal Access Token (PAT) for authentication.

Here's a step-by-step guide:

## 1. Prerequisites:
* Node.js and Gemini CLI: Ensure you have Node.js (version 20 or later recommended) and the Gemini CLI installed. If not, you can install the Gemini CLI using npm:
  ```
  npm install -g @google/gemini-cli
  ```
* GitHub Personal Access Token (PAT): You need a GitHub PAT with the necessary permissions (at least repo scope for accessing repositories).
  * Go to GitHub > Profile > Settings > Developer Settings > Personal Access Tokens.
  * Generate a new token, making sure to copy it immediately as you won't be able to see it again.

## 2. Locate (or Create) your Gemini CLI Configuration File:
The Gemini CLI stores its settings in a settings.json file. This file is typically located in your home directory within a .gemini folder.
* macOS/Linux:
  ```
  mkdir -p ~/.gemini
  code ~/.gemini/settings.json # or use your preferred editor like nano, vim, etc.
  ```
* Windows: The path will be similar, usually C:\\Users\\<YourUsername>\\.gemini\\settings.json.

## 3. Configure the settings.json file for the Remote GitHub MCP:
Open the settings.json file you located or created and add the following JSON object to configure the GitHub MCP server. If you already have other settings, ensure you add this within the main JSON object.
```json
{
  "mcpServers": {
    "github": {
      "httpUrl": "https://api.githubcopilot.com/mcp/",
      "headers": {
        "Authorization": "YOUR_GITHUB_PAT"
      },
      "timeout": 5000
    }
  }
}
```

**Important:**
* Replace "YOUR_GITHUB_PAT" with the actual Personal Access Token you generated in GitHub.
* The httpUrl for the GitHub MCP server is typically https://api.githubcopilot.com/mcp/.

## 4. Launch or Restart Gemini CLI:
If your Gemini CLI is already running, you might need to restart it for the new configuration to take effect. Just type gemini at your command prompt.
## 5. Verify MCP Server Installation:
Once Gemini CLI is running, you can verify that the GitHub MCP server is recognized by running the /mcp command at the Gemini CLI prompt:
```
/mcp
```
This should list the configured MCP servers, and you should see "github" among them.
