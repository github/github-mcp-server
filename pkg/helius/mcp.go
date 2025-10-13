package helius

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// HeliusMCP integrates Helius API with MCP protocol
type HeliusMCP struct {
	apiKey string
	client *http.Client
}

// NewHeliusMCP creates a new Helius MCP instance
func NewHeliusMCP(apiKey string) *HeliusMCP {
	return &HeliusMCP{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// RegisterTools registers Helius tools with MCP server
func (h *HeliusMCP) RegisterTools(s *server.MCPServer) {
	s.AddTool(mcp.NewTool("helius_get_account",
		mcp.WithDescription("Get Solana account info via Helius"),
		mcp.WithString("address", mcp.Description("Account address")),
	), h.handleGetAccount)

	s.AddTool(mcp.NewTool("helius_get_transaction",
		mcp.WithDescription("Get transaction details via Helius"),
		mcp.WithString("signature", mcp.Description("Transaction signature")),
	), h.handleGetTransaction)

	s.AddTool(mcp.NewTool("helius_get_nft_metadata",
		mcp.WithDescription("Get NFT metadata via Helius"),
		mcp.WithString("mint", mcp.Description("NFT mint address")),
	), h.handleGetNFTMetadata)
}

func (h *HeliusMCP) handleGetAccount(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	address, ok := request.GetArguments()["address"].(string)
	if !ok {
		return mcp.NewToolResultError("INVALID_PARAMS", "address required"), nil
	}

	url := fmt.Sprintf("https://mainnet.helius-rpc.com/?api-key=%s", h.apiKey)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getAccountInfo",
		"params":  []any{address, map[string]string{"encoding": "base64"}},
	}

	data, _ := json.Marshal(payload)
	resp, err := h.client.Post(url, "application/json", nil)
	if err != nil {
		return mcp.NewToolResultError("REQUEST_FAILED", err.Error()), nil
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	
	resultJSON, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (h *HeliusMCP) handleGetTransaction(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	signature, ok := request.GetArguments()["signature"].(string)
	if !ok {
		return mcp.NewToolResultError("INVALID_PARAMS", "signature required"), nil
	}

	url := fmt.Sprintf("https://mainnet.helius-rpc.com/?api-key=%s", h.apiKey)
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "getTransaction",
		"params":  []any{signature, "json"},
	}

	data, _ := json.Marshal(payload)
	resp, err := h.client.Post(url, "application/json", nil)
	if err != nil {
		return mcp.NewToolResultError("REQUEST_FAILED", err.Error()), nil
	}
	defer resp.Body.Close()

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	
	resultJSON, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(resultJSON)), nil
}

func (h *HeliusMCP) handleGetNFTMetadata(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	mint, ok := request.GetArguments()["mint"].(string)
	if !ok {
		return mcp.NewToolResultError("INVALID_PARAMS", "mint required"), nil
	}

	url := fmt.Sprintf("https://api.helius.xyz/v0/token-metadata?api-key=%s", h.apiKey)
	payload := map[string]any{"mintAccounts": []string{mint}}

	data, _ := json.Marshal(payload)
	resp, err := h.client.Post(url, "application/json", nil)
	if err != nil {
		return mcp.NewToolResultError("REQUEST_FAILED", err.Error()), nil
	}
	defer resp.Body.Close()

	var result []map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	
	resultJSON, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(resultJSON)), nil
}