@echo off
echo ===========================================
echo GitHub MCP Server Setup for OwlBan Group
echo ===========================================
echo.
echo This script will help you set up the GitHub MCP Server for the OwlBan group.
echo.
echo Prerequisites:
echo 1. Docker must be installed and running
echo 2. You need a GitHub Personal Access Token (PAT)
echo.
echo If you don't have a PAT, create one at: https://github.com/settings/personal-access-tokens/new
echo Required scopes: repo, read:org, read:user, notifications (adjust based on your needs)
echo.
pause

echo.
echo Please enter your GitHub Personal Access Token:
set /p GITHUB_TOKEN=

if "%GITHUB_TOKEN%"=="" (
    echo Error: No token provided. Exiting.
    pause
    exit /b 1
)

echo.
echo Starting GitHub MCP Server in Docker...
echo.
echo The server will be available at stdio mode.
echo Configure your MCP host (VS Code, etc.) to use this server.
echo.
echo Press Ctrl+C to stop the server.
echo.

docker run -i --rm ^
  -e GITHUB_PERSONAL_ACCESS_TOKEN=%GITHUB_TOKEN% ^
  ghcr.io/github/github-mcp-server

echo.
echo Setup complete. Configure your MCP host with the server details.
pause
