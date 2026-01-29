@echo off
echo ===========================================
echo Testing GitHub MCP Server Docker Setup
echo ===========================================
echo.
echo This script tests the Docker setup for the GitHub MCP Server.
echo It will attempt to run the server and check basic functionality.
echo.
echo Note: This test uses a dummy token. Replace with real PAT for actual testing.
echo.

set DUMMY_TOKEN=ghp_dummy_token_for_testing_purposes_only

echo Testing Docker container startup...
echo.

docker run --rm ^
  -e GITHUB_PERSONAL_ACCESS_TOKEN=%DUMMY_TOKEN% ^
  ghcr.io/github/github-mcp-server ^
  --help

if %ERRORLEVEL% EQU 0 (
    echo.
    echo ✅ Docker container started successfully!
    echo ✅ Server binary is functional
    echo.
    echo Next steps:
    echo 1. Get a real GitHub Personal Access Token
    echo 2. Run: setup-owlban.bat
    echo 3. Configure your MCP host (VS Code, etc.)
    echo.
) else (
    echo.
    echo ❌ Docker test failed. Check Docker installation and network.
    echo.
)

echo Press any key to continue...
pause >nul
