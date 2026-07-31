package main

import (
	"fmt"
	"io"
	"os"

	"github.com/github/github-mcp-server/pkg/github"
)

func main() {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read JSON: %v\n", err)
		os.Exit(1)
	}

	output, err := github.JSONTextToMarkdown(string(input))
	if err != nil {
		fmt.Fprintf(os.Stderr, "render Markdown: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(output)
}
