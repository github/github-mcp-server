package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

const vscodeRedirectBase = "https://insiders.vscode.dev/redirect"

// VSCodeInstallPayload is the JSON payload passed to the vscode(:-insiders):mcp/install handler.
type VSCodeInstallPayload struct {
	Name   string         `json:"name"`
	Config map[string]any `json:"config"`
	Inputs []map[string]any `json:"inputs,omitempty"`
}

// VSCodeInstallRedirectURL builds a one-click VS Code MCP install redirect URL.
// When insiders is true, the link targets VS Code Insiders via the vscode-insiders: protocol
// instead of the broken quality=insiders query parameter on /redirect/mcp/install.
func VSCodeInstallRedirectURL(name string, config map[string]any, inputs []map[string]any, insiders bool) (string, error) {
	payload := VSCodeInstallPayload{
		Name:   name,
		Config: config,
		Inputs: inputs,
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal VS Code install payload: %w", err)
	}

	scheme := "vscode"
	if insiders {
		scheme = "vscode-insiders"
	}

	protocolURL := fmt.Sprintf("%s:mcp/install?%s", scheme, url.QueryEscape(string(payloadJSON)))
	// url.QueryEscape uses + for spaces; VS Code expects %20.
	protocolURL = strings.ReplaceAll(protocolURL, "+", "%20")

	return fmt.Sprintf("%s?url=%s", vscodeRedirectBase, url.QueryEscape(protocolURL)), nil
}

// VSCodeInstallRedirectURLHTTP builds a redirect URL for an HTTP MCP server config.
func VSCodeInstallRedirectURLHTTP(name, serverURL string, insiders bool) (string, error) {
	return VSCodeInstallRedirectURL(name, map[string]any{
		"type": "http",
		"url":  serverURL,
	}, nil, insiders)
}

// VSCodeHTTPToolsetInstallLink returns a markdown install link for a remote HTTP toolset.
func VSCodeHTTPToolsetInstallLink(name, serverURL string) (string, error) {
	installURL, err := VSCodeInstallRedirectURLHTTP(name, serverURL, false)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[Install](%s)", installURL), nil
}
