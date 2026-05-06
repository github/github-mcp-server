# Install GitHub MCP Server in Xcode

Xcode currently supports two built-in coding agents: **Codex** (powered by OpenAI) and **Claude Agent** (powered by Anthropic). Follow the standard installation guide for each agent, with one important difference: Xcode uses its own isolated configuration directories for each agent, separate from your global config.

> Configurations placed in these directories only affect agents when launched from Xcode. See [Apple's documentation](https://developer.apple.com/documentation/xcode/setting-up-coding-intelligence#Customize-the-Claude-Agent-and-Codex-environments) for more details.

## Configuration Directories

| Agent | Configuration Directory |
|-------|------------------------|
| Codex | `~/Library/Developer/Xcode/CodingAssistant/codex/` |
| Claude Agent | `~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/` |

Place your MCP server configuration in the relevant directory above rather than the default location used by the standalone CLI.

## Setup Guides

- **[Codex](install-codex.md)** — configure `config.toml` inside `~/Library/Developer/Xcode/CodingAssistant/codex/`
- **[Claude Agent](install-claude.md#xcode-claude-agent)** — configure `.claude.json` inside `~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig/`

## macOS Path Note

Xcode runs with a minimal `PATH` that typically excludes common binary locations. If you are using a local STDIO server (e.g. Docker or a pre-built binary), use the **full path** to the command in your config. Run `which docker` (or `which github-mcp-server`) in Terminal to find the correct path on your system. Common locations:

| Installation | Typical path |
|---|---|
| Docker (Intel Mac) | `/usr/local/bin/docker` |
| Docker (Apple Silicon) | `/usr/local/bin/docker` |
| Homebrew (Intel Mac) | `/usr/local/bin/` |
| Homebrew (Apple Silicon) | `/opt/homebrew/bin/` |

## Troubleshooting

| Issue | Possible Cause | Fix |
|-------|----------------|-----|
| Tools not loading | Config placed in wrong directory | Ensure config is in the Xcode-specific path above, not `~/.codex/` or `~/.claude.json` |
| Command not found (STDIO) | Xcode's PATH excludes binary location | Use the full path (e.g. `/usr/local/bin/docker` or `/opt/homebrew/bin/docker`); run `which docker` in Terminal to confirm |
| Docker not found | Docker not running | Start Docker Desktop and restart Xcode |
| Authentication failed | Invalid or expired PAT | Regenerate PAT and update config |

## References

- [Apple Developer Documentation — Setting up coding intelligence](https://developer.apple.com/documentation/xcode/setting-up-coding-intelligence#Customize-the-Claude-Agent-and-Codex-environments)
- [Codex MCP documentation](https://developers.openai.com/codex/mcp)
- Main project README: [Advanced configuration options](../../README.md)
