package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type TokenBalance struct {
	TokenAddress     string `json:"token_address"`
	Name             string `json:"name"`
	Symbol           string `json:"symbol"`
	Balance          string `json:"balance"`
	Decimals         int    `json:"decimals"`
	PossibleSpam     bool   `json:"possible_spam"`
	VerifiedContract bool   `json:"verified_contract"`
}

type MoralisResponse struct {
	Result []TokenBalance `json:"result"`
}

func main() {
	walletAddress := os.Getenv("WALLET_ADDRESS")
	if walletAddress == "" {
		walletAddress = "0xcB1C1FdE09f811B294172696404e88E658659905"
	}

	chain := os.Getenv("CHAIN")
	if chain == "" {
		chain = "eth"
	}

	apiKey := os.Getenv("MORALIS_API_KEY")
	if apiKey == "" {
		apiKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJub25jZSI6IjE2MjU2YzgxLWNlMDctNGNkMS1hNTYwLTU4ODI2MmZmZGIzYSIsIm9yZ0lkIjoiNDc0MzY4IiwidXNlcklkIjoiNDg4MDAzIiwidHlwZUlkIjoiNTM0OGY0YjItN2M2OC00ODgxLWJmZTMtMzU0MzM0NGE2YjhmIiwidHlwZSI6IlBST0pFQ1QiLCJpYXQiOjE3NTk3MzgzNDMsImV4cCI6NDkxNTQ5ODM0M30.QBahMKc7uaxlqFSWZkhJB3H560iNZxb1gpxkW7EQEck"
	}

	url := fmt.Sprintf("https://deep-index.moralis.io/api/v2.2/wallets/%s/tokens?chain=%s", walletAddress, chain)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Printf("Error creating request: %v\n", err)
		os.Exit(1)
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("X-API-Key", apiKey)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(1)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		os.Exit(1)
	}

	if res.StatusCode != 200 {
		fmt.Printf("Error: Status %d\n%s\n", res.StatusCode, string(body))
		os.Exit(1)
	}

	var response MoralisResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		fmt.Println(string(body))
		os.Exit(1)
	}

	fmt.Printf("🔍 Wallet: %s (Chain: %s)\n", walletAddress, chain)
	fmt.Printf("📊 Total Tokens: %d\n\n", len(response.Result))

	for i, token := range response.Result {
		spam := ""
		if token.PossibleSpam {
			spam = " ⚠️ SPAM"
		}
		verified := ""
		if token.VerifiedContract {
			verified = " ✅"
		}
		fmt.Printf("%d. %s (%s)%s%s\n", i+1, token.Name, token.Symbol, verified, spam)
		fmt.Printf("   Address: %s\n", token.TokenAddress)
		fmt.Printf("   Balance: %s (decimals: %d)\n\n", token.Balance, token.Decimals)
	}
}
