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
		return err
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
		return err
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
}

// createOAuthMiddleware returns receiving middleware that authorizes the session
// lazily, on the first tool call. Authorization is deferred until here (rather
// than at startup) because the prompts depend on an initialized session whose
// elicitation capabilities are known.
//
// When a token is already available the call proceeds untouched. Otherwise the
// flow runs: secure channels (browser, URL elicitation) block until the token
// arrives and then the call proceeds; the last-resort channel returns the
// instruction to the user as a tool result and asks them to retry.
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
	}
}

// ensure sessionPrompter satisfies the Prompter contract.
var _ oauth.Prompter = (*sessionPrompter)(nil)
