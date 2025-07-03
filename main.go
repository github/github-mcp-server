package main

import (
	"os"

	cmd "github.com/SchulteDev/github_github-mcp-server/cmd/github-mcp-server"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
