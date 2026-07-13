package ghmcp

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"

	"github.com/github/github-mcp-server/internal/oauth"
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
	AwaitToken(ctx context.Context) (*oauth.Outcome, error)
	Cancel()
}

// oauthElicitID is the stable key for the authorization elicitation in the
// multi-round-trip flow. The client echoes it back in InputResponses when it
// retries the tool call, so the middleware can recognize the user's response.
const oauthElicitID = "github_authorization"

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

// createOAuthMiddleware returns receiving middleware that authorizes the session
// lazily, on the first tool call. Authorization is deferred until here (rather
// than at startup) because the prompts depend on an initialized session whose
// elicitation capabilities and protocol version are known.
//
// When a token is already available the call proceeds untouched. Otherwise the
// authorization flow runs, presenting its prompt over whichever channel the
// negotiated protocol allows: on protocol versions before 2026-07-28 the server
// elicits directly; from 2026-07-28 on, where server-initiated requests are
// forbidden (SEP-2322), it uses multi-round-trip elicitation returned from the
// tool call. Either way the last-resort channel returns the instruction as a
// tool result and asks the user to retry.
func createOAuthMiddleware(mgr oauthAuthenticator, logger *slog.Logger) func(next mcp.MethodHandler) mcp.MethodHandler {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, request mcp.Request) (mcp.Result, error) {
			if method != "tools/call" || mgr.HasToken() {
				return next(ctx, method, request)
			}

			callReq, ok := request.(*mcp.CallToolRequest)
			if !ok {
				return next(ctx, method, request)
			}

			if serverMayInitiateElicitation(callReq.Session) {
				return authorizeViaServerElicitation(ctx, mgr, next, method, request, callReq, logger)
			}
			return authorizeViaMultiRoundTrip(ctx, mgr, next, method, request, callReq, logger)
		}
	}
}

// authorizeViaServerElicitation drives authorization on legacy protocol versions
// (before 2026-07-28), where the server may present the prompt itself via
// server-initiated elicitation. It blocks until the token arrives, then proceeds.
func authorizeViaServerElicitation(ctx context.Context, mgr oauthAuthenticator, next mcp.MethodHandler, method string, request mcp.Request, callReq *mcp.CallToolRequest, logger *slog.Logger) (mcp.Result, error) {
	outcome, err := mgr.Authenticate(ctx, &sessionPrompter{session: callReq.Session})
	if err != nil {
		return nil, fmt.Errorf("github authorization failed: %w", err)
	}
	if outcome != nil && outcome.UserAction != nil {
		logger.Info("surfacing github authorization instructions to user")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: outcome.UserAction.Message}},
		}, nil
	}
	return next(ctx, method, request)
}

// authorizeViaMultiRoundTrip drives authorization on protocol version 2026-07-28
// and later, where server-initiated requests are forbidden (SEP-2322). The first
// tool call starts the flow and returns the authorization prompt as an
// elicitation input request; the client presents it and retries the call with
// the user's response, at which point we wait for the token and proceed.
func authorizeViaMultiRoundTrip(ctx context.Context, mgr oauthAuthenticator, next mcp.MethodHandler, method string, request mcp.Request, callReq *mcp.CallToolRequest, logger *slog.Logger) (mcp.Result, error) {
	// Retry: the client fulfilled the authorization elicitation and re-sent the
	// call with the user's response.
	if resp, ok := callReq.Params.InputResponses[oauthElicitID]; ok {
		res, _ := resp.(*mcp.ElicitResult)
		if res == nil || res.Action != "accept" {
			// The user declined or dismissed the prompt; tear the flow down so it
			// does not linger, and let them retry when they are ready.
			mgr.Cancel()
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: "GitHub authorization was declined. Retry when you're ready to authorize."}},
			}, nil
		}
		outcome, err := mgr.AwaitToken(ctx)
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
		return next(ctx, method, request)
	}

	// First attempt: start the flow. A nil prompter keeps the manager from
	// initiating any elicitation itself (forbidden on this protocol); it opens a
	// server-side browser when possible, otherwise returns the authorization
	// instructions for us to present via multi-round-trip elicitation.
	outcome, err := mgr.Authenticate(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("github authorization failed: %w", err)
	}
	if outcome == nil || outcome.UserAction == nil {
		// Already authorized (e.g. the server opened a browser and the flow
		// completed); proceed.
		return next(ctx, method, request)
	}

	elicit := authorizationElicitParams(outcome.UserAction, &sessionPrompter{session: callReq.Session})
	if elicit == nil {
		// The client cannot present an elicitation (no capability, or no URL to
		// show); fall back to returning the instructions as a tool result.
		logger.Info("surfacing github authorization instructions to user")
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: outcome.UserAction.Message}},
		}, nil
	}
	logger.Info("requesting github authorization via elicitation")
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{oauthElicitID: elicit},
		RequestState:  "github-authorization-pending",
	}, nil
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
		return &mcp.ElicitParams{Mode: "form", Message: message}
	default:
		return nil
	}
}

// ensure sessionPrompter satisfies the Prompter contract.
var _ oauth.Prompter = (*sessionPrompter)(nil)
