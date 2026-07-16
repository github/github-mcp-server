package ghmcp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/github/github-mcp-server/internal/oauth"
	"github.com/github/github-mcp-server/pkg/inventory"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionPrompter adapts an MCP server session to oauth.Prompter, presenting
// authorization prompts to the user via elicitation. Keeping the prompt on the
// MCP control channel (rather than a tool result) keeps the authorization URL
// and any session-bound state out of the model's context.
type sessionPrompter struct {
	session *mcp.ServerSession
}

// elicitationCaps returns the client's declared elicitation capabilities, or nil
// if the client did not advertise any.
func (p *sessionPrompter) elicitationCaps() *mcp.ElicitationCapabilities {
	params := p.session.InitializeParams()
	if params == nil || params.Capabilities == nil {
		return nil
	}
	return params.Capabilities.Elicitation
}

// CanPromptURL reports whether the client supports URL-mode elicitation.
func (p *sessionPrompter) CanPromptURL() bool {
	caps := p.elicitationCaps()
	return caps != nil && caps.URL != nil
}

// PromptURL presents the authorization URL via URL-mode elicitation and blocks
// until the user acknowledges, declines, or ctx is done.
func (p *sessionPrompter) PromptURL(ctx context.Context, prompt oauth.Prompt) error {
	res, err := p.session.Elicit(ctx, &mcp.ElicitParams{
		Mode:          "url",
		Message:       prompt.Message,
		URL:           prompt.URL,
		ElicitationID: rand.Text(),
	})
	if err != nil {
		// The client advertised URL elicitation but the request itself failed:
		// classify it as undeliverable (not a user decision) so the flow can fall
		// back to a channel that needs no client capability.
		return fmt.Errorf("%w: %w", oauth.ErrPromptUnavailable, err)
	}
	if res.Action != "accept" {
		return oauth.ErrPromptDeclined
	}
	return nil
}

// CanPromptForm reports whether the client supports form-mode elicitation. The
// SDK treats a client that advertises neither form nor URL capabilities as
// supporting forms, for backward compatibility, so we mirror that here.
func (p *sessionPrompter) CanPromptForm() bool {
	caps := p.elicitationCaps()
	if caps == nil {
		return false
	}
	return caps.Form != nil || caps.URL == nil
}

// PromptForm presents a textual acknowledgement (used to display a device code
// when URL elicitation is unavailable) and blocks until the user responds.
func (p *sessionPrompter) PromptForm(ctx context.Context, prompt oauth.Prompt) error {
	res, err := p.session.Elicit(ctx, &mcp.ElicitParams{
		Mode:    "form",
		Message: prompt.Message,
	})
	if err != nil {
		// As with PromptURL, a delivery failure is undeliverable rather than a
		// decline, so the flow can fall back instead of aborting.
		return fmt.Errorf("%w: %w", oauth.ErrPromptUnavailable, err)
	}
	if res.Action != "accept" {
		return oauth.ErrPromptDeclined
	}
	return nil
}

// oauthAuthenticator is the subset of *oauth.Manager that the middleware needs.
// Depending on the interface (rather than the concrete manager) lets the
// middleware be exercised with a deterministic fake, since driving the real
// manager to its branches would require standing up live GitHub flows.
type oauthAuthenticator interface {
	HasToken() bool
	Authenticate(ctx context.Context, prompter oauth.Prompter) (*oauth.Outcome, error)
	AwaitToken(ctx context.Context, flowID string) (*oauth.Outcome, error)
	Cancel(flowID string) bool
}

// oauthElicitIDPrefix identifies authorization responses in the multi-round-trip
// InputResponses map. The suffix is the manager's per-flow ID, which prevents a
// delayed response from an older prompt from affecting a newer flow.
const oauthElicitIDPrefix = "github_authorization:"

// protocolVersionNoServerElicitation is the first MCP protocol version that
// forbids server-initiated JSON-RPC requests (SEP-2322): from this version on
// the server may not send elicitation/create while serving a request and must
// instead return an InputRequests map from the tool call (multi round-trip
// requests). It mirrors the go-sdk's internal constant of the same value, which
// the SDK does not export.
const protocolVersionNoServerElicitation = "2026-07-28"

// serverMayInitiateElicitation reports whether the server is permitted to send
// elicitation requests to the client itself, which the spec allows only before
// protocol version 2026-07-28. A nil or un-negotiated session (only reached in
// unit tests; a real tools/call is always initialized) is treated as legacy.
func serverMayInitiateElicitation(ss *mcp.ServerSession) bool {
	if ss == nil {
		return true
	}
	params := ss.InitializeParams()
	return params == nil || params.ProtocolVersion < protocolVersionNoServerElicitation
}

// createOAuthToolMiddleware returns tool-handler middleware that authorizes the
// session lazily, on the first tool call. It runs inside the SDK's
// Server.callTool handler so results returned here still receive SDK
// finalization, including resultType: "input_required" for multi-round-trip
// responses.
//
// When a token is already available the call proceeds untouched. Otherwise the
// authorization flow runs, presenting its prompt over whichever channel the
// negotiated protocol allows: on protocol versions before 2026-07-28 the server
// elicits directly; from 2026-07-28 on, where server-initiated requests are
// forbidden (SEP-2322), it uses multi-round-trip elicitation returned from the
// tool call. Either way the last-resort channel returns the instruction as a
// tool result and asks the user to retry.
func createOAuthToolMiddleware(mgr oauthAuthenticator, logger *slog.Logger) inventory.ToolHandlerMiddleware {
	return func(next mcp.ToolHandler) mcp.ToolHandler {
		return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !serverMayInitiateElicitation(req.Session) {
				if flowID, response, ok := authorizationElicitResponse(req.Params.InputResponses); ok {
					return resumeMultiRoundTripAuthorization(ctx, mgr, next, req, flowID, response, logger)
				}
			}

			if mgr.HasToken() {
				return next(ctx, req)
			}
			if serverMayInitiateElicitation(req.Session) {
				return authorizeViaServerElicitation(ctx, mgr, next, req, logger)
			}
			return startMultiRoundTripAuthorization(ctx, mgr, next, req, logger)
		}
	}
}

// authorizeViaServerElicitation drives authorization on legacy protocol versions
// (before 2026-07-28), where the server may present the prompt itself via
// server-initiated elicitation. It blocks until the token arrives, then proceeds.
func authorizeViaServerElicitation(ctx context.Context, mgr oauthAuthenticator, next mcp.ToolHandler, req *mcp.CallToolRequest, logger *slog.Logger) (*mcp.CallToolResult, error) {
	outcome, err := mgr.Authenticate(ctx, &sessionPrompter{session: req.Session})
	if err != nil {
		return nil, fmt.Errorf("github authorization failed: %w", err)
	}
	if outcome != nil && outcome.UserAction != nil {
		logger.Info("surfacing github authorization instructions to user")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: outcome.UserAction.Message}},
		}, nil
	}
	return next(ctx, req)
}

// authorizationElicitResponse finds the authorization response and extracts the
// flow ID encoded in its input-request key.
func authorizationElicitResponse(responses mcp.InputResponseMap) (string, *mcp.ElicitResult, bool) {
	for id, response := range responses {
		flowID, ok := strings.CutPrefix(id, oauthElicitIDPrefix)
		if !ok || flowID == "" {
			continue
		}
		result, _ := response.(*mcp.ElicitResult)
		return flowID, result, true
	}
	return "", nil, false
}

// startMultiRoundTripAuthorization starts authorization on protocol version
// 2026-07-28 or later. Server-initiated requests are forbidden there (SEP-2322),
// so the prompt is returned as an elicitation input request for the client to
// fulfill and retry.
func startMultiRoundTripAuthorization(ctx context.Context, mgr oauthAuthenticator, next mcp.ToolHandler, req *mcp.CallToolRequest, logger *slog.Logger) (*mcp.CallToolResult, error) {
	outcome, err := mgr.Authenticate(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("github authorization failed: %w", err)
	}
	if outcome == nil || outcome.UserAction == nil {
		// Already authorized (e.g. the server opened a browser and the flow
		// completed); proceed.
		return next(ctx, req)
	}

	elicit := authorizationElicitParams(outcome.UserAction, &sessionPrompter{session: req.Session})
	if elicit == nil || outcome.FlowID == "" {
		// The client cannot present an elicitation (no capability, or no URL to
		// show), or the flow cannot be correlated; fall back to returning the
		// instructions as a tool result.
		logger.Info("surfacing github authorization instructions to user")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: outcome.UserAction.Message}},
		}, nil
	}
	logger.Info("requesting github authorization via elicitation")
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{oauthElicitIDPrefix + outcome.FlowID: elicit},
	}, nil
}

// resumeMultiRoundTripAuthorization handles the client's retry after it
// fulfilled the authorization elicitation.
func resumeMultiRoundTripAuthorization(ctx context.Context, mgr oauthAuthenticator, next mcp.ToolHandler, req *mcp.CallToolRequest, flowID string, response *mcp.ElicitResult, logger *slog.Logger) (*mcp.CallToolResult, error) {
	if response == nil || response.Action != "accept" {
		if !mgr.Cancel(flowID) {
			return expiredAuthorizationResult(), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "GitHub authorization was declined. Retry when you're ready to authorize."}},
		}, nil
	}

	outcome, err := mgr.AwaitToken(ctx, flowID)
	if errors.Is(err, oauth.ErrStaleAuthorizationFlow) {
		return expiredAuthorizationResult(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("github authorization failed: %w", err)
	}
	if outcome != nil && outcome.UserAction != nil {
		// The user acknowledged the prompt but has not finished authorizing;
		// surface the instructions so they can complete it and retry.
		logger.Info("surfacing github authorization instructions to user")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: outcome.UserAction.Message}},
		}, nil
	}
	return next(ctx, req)
}

func expiredAuthorizationResult() *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "This GitHub authorization prompt has expired. Retry the request to authorize again."}},
	}
}

// authorizationElicitParams builds the elicitation that presents the
// authorization instructions to the user. It mirrors sessionPrompter's channel
// selection: URL-mode when the client supports it, otherwise form-mode. It
// returns nil when the client advertised no elicitation capability or there is
// no authorization URL to show, so the caller falls back to a tool-result
// message.
func authorizationElicitParams(ua *oauth.UserAction, p *sessionPrompter) *mcp.ElicitParams {
	if ua.URL == "" {
		return nil
	}
	message := "Authorize the GitHub MCP Server to continue."
	if ua.UserCode != "" {
		message = fmt.Sprintf("Enter code %s to authorize the GitHub MCP Server.", ua.UserCode)
	}
	switch {
	case p.CanPromptURL():
		return &mcp.ElicitParams{Mode: "url", Message: message, URL: ua.URL, ElicitationID: rand.Text()}
	case p.CanPromptForm():
		return &mcp.ElicitParams{Mode: "form", Message: ua.Message}
	default:
		return nil
	}
}

// ensure sessionPrompter satisfies the Prompter contract.
var _ oauth.Prompter = (*sessionPrompter)(nil)
