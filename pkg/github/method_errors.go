package github

import (
	"fmt"
	"strings"

	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func unknownMethodError(method string, supportedMethods ...string) *mcp.CallToolResult {
	return utils.NewToolResultError(fmt.Sprintf(
		"unknown method: %s. Supported methods are: %s",
		method,
		strings.Join(supportedMethods, ", "),
	))
}
