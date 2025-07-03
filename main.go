package main

import (
	"os"

	cmd "github.com/github/github-mcp-server/cmd/github-mcp-server"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
