// Copyright 2025 The Go MCP SDK Authors. All rights reserved.
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// The conformance client implements features required for MCP conformance testing.
// It mirrors the functionality of the TypeScript conformance client at
// https://github.com/modelcontextprotocol/typescript-sdk/blob/main/src/conformance/everything-client.ts
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// scenarioHandler is the function signature for all conformance test scenarios.
// It takes a context and the server URL to connect to.
type scenarioHandler func(ctx context.Context, serverURL string, configCtx map[string]any) error

var (
	// registry stores all registered scenario handlers.
	registry = make(map[string]scenarioHandler)
)

// registerScenario registers a new scenario handler with the given name.
// This function should be called during init() by scenario implementations.
func registerScenario(name string, handler scenarioHandler) {
	if _, exists := registry[name]; exists {
		log.Fatalf("Scenario %q is already registered", name)
	}
	registry[name] = handler
}

func init() {
	registerScenario("initialize", runBasicClient)
	registerScenario("tools_call", runToolsCallClient)
	registerScenario("request-metadata", runRequestMetadataClient)
	registerScenario("elicitation-sep1034-client-defaults", runElicitationDefaultsClient)
	registerScenario("sse-retry", runSSERetryClient)
	registerScenario("json-schema-ref-no-deref", runJSONSchemaRefNoDerefClient)
	registerScenario("sep-2322-client-request-state", runMrtrClient)
	registerScenario("http-standard-headers", runHTTPStandardHeadersClient)
	registerScenario("http-custom-headers", runHTTPCustomHeadersClient)
	registerScenario("http-invalid-tool-headers", runHTTPInvalidToolHeadersClient)

	authScenarios := []string{
		"auth/2025-03-26-oauth-metadata-backcompat",
		"auth/2025-03-26-oauth-endpoint-fallback",
		"auth/authorization-server-migration",
		"auth/basic-cimd",
		"auth/iss-normalized",
		"auth/iss-not-advertised",
		"auth/iss-supported",
		"auth/iss-supported-missing",
		"auth/iss-unexpected",
		"auth/iss-wrong-issuer",
		"auth/metadata-default",
		"auth/metadata-issuer-mismatch",
		"auth/metadata-var1",
		"auth/metadata-var2",
		"auth/metadata-var3",
		"auth/offline-access-not-supported",
		"auth/offline-access-scope",
		"auth/pre-registration",
		"auth/resource-mismatch",
		"auth/scope-from-scopes-supported",
		"auth/scope-from-www-authenticate",
		"auth/scope-omitted-when-undefined",
		"auth/scope-retry-limit",
		"auth/scope-step-up",
		"auth/token-endpoint-auth-basic",
		"auth/token-endpoint-auth-none",
		"auth/token-endpoint-auth-post",
	}
	for _, scenario := range authScenarios {
		registerScenario(scenario, runAuthClient)
	}
}

// ============================================================================
// Basic scenarios
// ============================================================================

func runBasicClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()

	_, err = session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}

	return nil
}

func runToolsCallClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}

	idx := slices.IndexFunc(tools.Tools, func(t *mcp.Tool) bool {
		return t.Name == "add_numbers"
	})
	if idx == -1 {
		return fmt.Errorf("tool %q not found", "add_numbers")
	}

	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "add_numbers",
		Arguments: map[string]any{"a": 5, "b": 3},
	})
	if err != nil {
		return fmt.Errorf("session.CallTool('add_numbers'): %v", err)
	}

	return nil
}

// ============================================================================
// Elicitation scenarios
// ============================================================================

func runElicitationDefaultsClient(ctx context.Context, serverURL string, _ map[string]any) error {
	elicitationHandler := func(ctx context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		return &mcp.ElicitResult{
			Action:  "accept",
			Content: map[string]any{},
		}, nil
	}

	session, err := connectToServer(ctx, serverURL, withClientOptions(&mcp.ClientOptions{
		ElicitationHandler: elicitationHandler,
	}))
	if err != nil {
		return err
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}

	var testToolName = "test_client_elicitation_defaults"
	idx := slices.IndexFunc(tools.Tools, func(t *mcp.Tool) bool {
		return t.Name == testToolName
	})
	if idx == -1 {
		return fmt.Errorf("tool %q not found", testToolName)
	}

	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      testToolName,
		Arguments: map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("session.CallTool(%q): %v", testToolName, err)
	}

	return nil
}

// ============================================================================
// SSE retry scenario
// ============================================================================

func runSSERetryClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()
	log.Printf("Connected to server %q", serverURL)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}

	var testToolName = "test_reconnection"
	idx := slices.IndexFunc(tools.Tools, func(t *mcp.Tool) bool {
		return t.Name == testToolName
	})
	if idx == -1 {
		return fmt.Errorf("tool %q not found", testToolName)
	}

	_, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      testToolName,
		Arguments: map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("session.CallTool(%q): %v", testToolName, err)
	}

	return nil
}

// ============================================================================
// Auth scenarios
// ============================================================================

func fetchAuthorizationCodeAndState(ctx context.Context, args *auth.AuthorizationArgs) (*auth.AuthorizationResult, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// In conformance tests the authorization server immediately redirects
	// to the callback URL with the authorization code and state.
	locURL, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		return nil, fmt.Errorf("parse location: %v", err)
	}

	return &auth.AuthorizationResult{
		Code:  locURL.Query().Get("code"),
		State: locURL.Query().Get("state"),
		Iss:   locURL.Query().Get("iss"),
	}, nil
}

func runAuthClient(ctx context.Context, serverURL string, configCtx map[string]any) error {
	authConfig := &auth.AuthorizationCodeHandlerConfig{
		RedirectURL:              "http://localhost:3000/callback",
		AuthorizationCodeFetcher: fetchAuthorizationCodeAndState,
		// Try client ID metadata document based registration.
		ClientIDMetadataDocumentConfig: &auth.ClientIDMetadataDocumentConfig{
			URL: "https://conformance-test.local/client-metadata.json",
		},
		// Try dynamic client registration.
		DynamicClientRegistrationConfig: &auth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				RedirectURIs: []string{"http://localhost:3000/callback"},
			},
		},
	}
	// Try pre-registered client information if provided in the context.
	if clientID, ok := configCtx["client_id"].(string); ok {
		if clientSecret, ok := configCtx["client_secret"].(string); ok {
			authConfig.PreregisteredClient = &oauthex.ClientCredentials{
				ClientID: clientID,
				ClientSecretAuth: &oauthex.ClientSecretAuth{
					ClientSecret: clientSecret,
				},
			}
		}
	}

	authHandler, err := auth.NewAuthorizationCodeHandler(authConfig)
	if err != nil {
		return fmt.Errorf("failed to create auth handler: %w", err)
	}

	session, err := connectToServer(ctx, serverURL, withOAuthHandler(authHandler))
	if err != nil {
		return err
	}
	defer session.Close()

	if _, err := session.ListTools(ctx, nil); err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}

	if _, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "test-tool",
		Arguments: map[string]any{},
	}); err != nil {
		return fmt.Errorf("session.CallTool('test-tool'): %v", err)
	}

	return nil
}

// ============================================================================
// request-metadata scenario (SEP-2575)
// ============================================================================

// runRequestMetadataClient exercises the SEP-2575 wire-level negotiation:
// every request must carry the MCP-Protocol-Version header and the per-request
// _meta envelope, and the client must retry with a supported version when its
// first choice is rejected with -32022. The Go SDK's Connect() drives
// server/discover unconditionally for 2026-07-28, which is exactly that
// mechanism.
func runRequestMetadataClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()
	return nil
}

// ============================================================================
// json-schema-ref-no-deref scenario (SEP-2106)
// ============================================================================

// runJSONSchemaRefNoDerefClient asserts that listTools does not dereference
// network $ref URLs in tool schemas. The Go SDK never fetches external refs;
// a plain connect → listTools is sufficient — if the SDK ever regressed and
// tried to GET the canary URL the conformance referee would record the
// failure.
func runJSONSchemaRefNoDerefClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()

	_, _ = session.ListTools(ctx, nil)
	return nil
}

// ============================================================================
// SEP-2322 multi-round-trip client scenario
// ============================================================================

// runMrtrClient drives the SEP-2322 client-request-state scenario. The
// SDK's built-in client-side MRTR middleware transparently fulfills embedded
// elicitation requests via ElicitationHandler and echoes requestState.
func runMrtrClient(ctx context.Context, serverURL string, _ map[string]any) error {
	elicitationHandler := func(context.Context, *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
		return &mcp.ElicitResult{
			Action:  "accept",
			Content: map[string]any{"confirmed": true},
		}, nil
	}
	session, err := connectToServer(ctx, serverURL,
		withClientOptions(&mcp.ClientOptions{
			Capabilities:       &mcp.ClientCapabilities{Elicitation: &mcp.ElicitationCapabilities{}},
			ElicitationHandler: elicitationHandler,
		}),
	)
	if err != nil {
		return err
	}
	defer session.Close()

	toolNames := []string{
		"test_mrtr_echo_state",
		"test_mrtr_no_state",
		"test_mrtr_unrelated",
		"test_mrtr_no_result_type",
	}
	for _, name := range toolNames {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      name,
			Arguments: map[string]any{},
		}); err != nil {
			log.Printf("CallTool(%q) rejected locally (expected for some cases): %v", name, err)
		}
	}
	return nil
}

// ============================================================================
// SEP-2243 HTTP header scenarios
// ============================================================================

// runHTTPStandardHeadersClient (SEP-2243 standard headers) drives tools,
// resources and prompts once each so the referee sees Mcp-Method / Mcp-Name
// headers on every verb the SDK supports.
func runHTTPStandardHeadersClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()

	if tools, err := session.ListTools(ctx, nil); err == nil && len(tools.Tools) > 0 {
		_, _ = session.CallTool(ctx, &mcp.CallToolParams{
			Name:      tools.Tools[0].Name,
			Arguments: map[string]any{},
		})
	}
	if res, err := session.ListResources(ctx, nil); err == nil && len(res.Resources) > 0 {
		_, _ = session.ReadResource(ctx, &mcp.ReadResourceParams{URI: res.Resources[0].URI})
	}
	if prompts, err := session.ListPrompts(ctx, nil); err == nil && len(prompts.Prompts) > 0 {
		_, _ = session.GetPrompt(ctx, &mcp.GetPromptParams{
			Name:      prompts.Prompts[0].Name,
			Arguments: map[string]string{},
		})
	}
	return nil
}

// runHTTPCustomHeadersClient (SEP-2243 custom headers) lists tools (so the
// SDK caches inputSchema and its x-mcp-header annotations), then makes the
// runner-supplied tool calls so the referee validates the Mcp-Param-*
// headers.
func runHTTPCustomHeadersClient(ctx context.Context, serverURL string, configCtx map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()

	if _, err := session.ListTools(ctx, nil); err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}
	for _, call := range readToolCallsContext(configCtx) {
		if _, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      call.Name,
			Arguments: call.Arguments,
		}); err != nil {
			log.Printf("CallTool(%q): %v", call.Name, err)
		}
	}
	return nil
}

// runHTTPInvalidToolHeadersClient (SEP-2243 invalid-tool filtering) lists
// tools — a correct SDK leaves only the valid ones — then calls each survivor
// so the referee records SUCCESS for every excluded tool never called.
func runHTTPInvalidToolHeadersClient(ctx context.Context, serverURL string, _ map[string]any) error {
	session, err := connectToServer(ctx, serverURL)
	if err != nil {
		return err
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("session.ListTools(): %v", err)
	}
	for _, tool := range tools.Tools {
		_, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      tool.Name,
			Arguments: map[string]any{"region": "us-west1"},
		})
		if err != nil {
			log.Printf("call %q rejected: %v", tool.Name, err)
		}
	}
	return nil
}

type toolCall struct {
	Name      string
	Arguments map[string]any
}

// readToolCallsContext parses the `toolCalls` array from MCP_CONFORMANCE_CONTEXT.
// Format: {"toolCalls": [{"name":"...", "arguments":{...}}, ...]}.
func readToolCallsContext(configCtx map[string]any) []toolCall {
	raw, ok := configCtx["toolCalls"].([]any)
	if !ok {
		return nil
	}
	out := make([]toolCall, 0, len(raw))
	for _, item := range raw {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := obj["name"].(string)
		args, _ := obj["arguments"].(map[string]any)
		if args == nil {
			args = map[string]any{}
		}
		out = append(out, toolCall{Name: name, Arguments: args})
	}
	return out
}

// ============================================================================
// Main entry point
// ============================================================================

func main() {
	if len(os.Args) != 2 {
		printUsageAndExit("Usage: %s <server-url>", os.Args[0])
	}

	serverURL := os.Args[1]
	scenarioName := os.Getenv("MCP_CONFORMANCE_SCENARIO")
	configCtx := getConformanceContext()

	if scenarioName == "" {
		printUsageAndExit("MCP_CONFORMANCE_SCENARIO not set")
	}

	handler, ok := registry[scenarioName]
	if !ok {
		printUsageAndExit("Unknown scenario: %q", scenarioName)
	}

	ctx := context.Background()
	if err := handler(ctx, serverURL, configCtx); err != nil {
		log.Fatalf("Scenario %q failed: %v", scenarioName, err)
	}
}

func getConformanceContext() map[string]any {
	ctxStr := os.Getenv("MCP_CONFORMANCE_CONTEXT")
	if ctxStr == "" {
		return nil
	}
	var ctx map[string]any
	_ = json.Unmarshal([]byte(ctxStr), &ctx)
	return ctx
}

func printUsageAndExit(format string, args ...any) {
	var scenarios []string
	for name := range registry {
		scenarios = append(scenarios, name)
	}
	sort.Strings(scenarios)

	msg := fmt.Sprintf(format, args...)
	log.Fatalf("%s\nAvailable scenarios:\n  - %s", msg, strings.Join(scenarios, "\n  - "))
}

type connectConfig struct {
	clientOptions *mcp.ClientOptions
	oauthHandler  auth.OAuthHandler
}

type connectOption func(*connectConfig)

func withClientOptions(opts *mcp.ClientOptions) connectOption {
	return func(c *connectConfig) {
		c.clientOptions = opts
	}
}

func withOAuthHandler(handler auth.OAuthHandler) connectOption {
	return func(c *connectConfig) {
		c.oauthHandler = handler
	}
}

// connectToServer connects to the MCP server and returns a client session.
// The caller is responsible for closing the session.
func connectToServer(ctx context.Context, serverURL string, opts ...connectOption) (*mcp.ClientSession, error) {
	config := &connectConfig{}
	for _, opt := range opts {
		opt(config)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}, config.clientOptions)

	transport := &mcp.StreamableClientTransport{
		Endpoint:     serverURL,
		OAuthHandler: config.oauthHandler,
	}

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("client.Connect(): %w", err)
	}

	return session, nil
}
