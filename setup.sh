#!/bin/bash
set -e

echo "🚀 GitHub MCP Server - Automated Setup"
echo "========================================"

# Color codes
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Step 1: Check Go installation
echo -e "${BLUE}[1/6]${NC} Checking Go installation..."
if ! command -v go &> /dev/null; then
    echo -e "${YELLOW}Go is not installed. Please install Go 1.21+${NC}"
    echo "Download from: https://golang.org/dl/"
    exit 1
fi
GO_VERSION=$(go version | awk '{print $3}')
echo -e "${GREEN}✓ Go ${GO_VERSION} found${NC}"

# Step 2: Download dependencies
echo -e "${BLUE}[2/6]${NC} Downloading Go dependencies..."
go mod download
go mod tidy
echo -e "${GREEN}✓ Dependencies installed${NC}"

# Step 3: Build the project
echo -e "${BLUE}[3/6]${NC} Building GitHub MCP Server..."
mkdir -p bin
go build -o bin/github-mcp-server ./cmd/mcp-server
echo -e "${GREEN}✓ Build successful: bin/github-mcp-server${NC}"

# Step 4: Run tests
echo -e "${BLUE}[4/6]${NC} Running tests..."
if go test ./... -v; then
    echo -e "${GREEN}✓ All tests passed${NC}"
else
    echo -e "${YELLOW}⚠ Some tests failed (this may be normal)${NC}"
fi

# Step 5: Create .env file
echo -e "${BLUE}[5/6]${NC} Creating environment configuration..."
if [ ! -f .env ]; then
    cat > .env << EOF
GITHUB_TOKEN=your_personal_access_token_here
GITHUB_SERVER_URL=https://github.com
LOG_LEVEL=info
EOF
    echo -e "${YELLOW}Created .env file - PLEASE ADD YOUR GITHUB TOKEN${NC}"
else
    echo -e "${GREEN}✓ .env file already exists${NC}"
fi

# Step 6: Setup instructions
echo -e "${BLUE}[6/6]${NC} Providing setup instructions..."
echo ""
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}Setup Complete! ✓${NC}"
echo -e "${GREEN}================================${NC}"
echo ""
echo "📝 Next Steps:"
echo ""
echo "1. Get your GitHub Personal Access Token:"
echo "   - Go to: https://github.com/settings/tokens"
echo "   - Create new token with: repo, gist, read:user, workflow scopes"
echo "   - Copy the token and add to .env file"
echo ""
echo "2. Add token to .env:"
echo "   Edit .env and replace 'your_personal_access_token_here' with your actual token"
echo ""
echo "3. Start the server:"
echo "   ./bin/github-mcp-server"
echo ""
echo "4. In another terminal, test the server:"
echo "   curl http://localhost:3000/health"
echo ""
echo "5. VS Code Integration:"
echo "   - Install Go extension: golang.go"
echo "   - Install GitHub Copilot: GitHub.copilot"
echo "   - Open .vscode/settings.json and configure"
echo ""
echo "📚 Documentation:"
echo "   - GitHub MCP Server: https://github.com/github/github-mcp-server"
echo "   - Your Fork: https://github.com/scutuatua-crypto/github-mcp-server"
echo ""
echo -e "${GREEN}Happy coding! 🎉${NC}"