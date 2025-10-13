package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/github/github-mcp-server/pkg/helius"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "helius-mcp-server",
		Short: "Helius MCP Server",
		Long:  "MCP server for Helius Solana API integration",
	}

	stdioCmd = &cobra.Command{
		Use:   "stdio",
		Short: "Start Helius MCP server via stdio",
		RunE:  runStdioServer,
	}
)

func init() {
	rootCmd.AddCommand(stdioCmd)
}

func runStdioServer(cmd *cobra.Command, args []string) error {
	apiKey := os.Getenv("HELIUS_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("HELIUS_API_KEY environment variable required")
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"helius-mcp-server",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithLogging(),
	)

	// Create Helius MCP instance and register tools
	heliusMCP := helius.NewHeliusMCP(apiKey)
	heliusMCP.RegisterTools(mcpServer)

	// Create stdio server
	stdioServer := server.NewStdioServer(mcpServer)

	// Setup graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Start server
	fmt.Fprintf(os.Stderr, "Helius MCP Server running on stdio\n")
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}