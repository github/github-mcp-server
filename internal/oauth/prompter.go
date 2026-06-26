package oauth

import (
	"context"
	"errors"
)

// ErrPromptDeclined is returned by a Prompter when the user actively cancels or
// declines the authorization prompt. It is a deliberate "no", so the flow stops
// rather than falling back to another channel.
var ErrPromptDeclined = errors.New("authorization declined by user")

// ErrPromptUnavailable is returned by a Prompter when the prompt could not be
// delivered at all — for example the client advertised an elicitation capability
// but the request failed at the transport or protocol level. Unlike
// ErrPromptDeclined it reflects no user decision, so the flow falls back to a
// channel that needs no client capability instead of giving up.
var ErrPromptUnavailable = errors.New("authorization prompt could not be delivered")

// Prompt is the content shown to the user when asking them to authorize.
type Prompt struct {
	// Message is a human-readable instruction.
	Message string
	// URL is the authorization URL (PKCE) or device verification URI.
	URL string
	// UserCode is the device-flow code the user must enter, if any.
	UserCode string
}

// Prompter presents authorization prompts to the user out of band from the LLM
// context — for example via MCP elicitation. Keeping prompts out of the model's
// context prevents the authorization URL (and any session-bound state) from
// leaking into tool arguments or transcripts.
//
// A nil Prompter is valid and reports no capabilities, which drives the flow to
// its last-resort channel. Implementations wrap a transport-specific client
// (e.g. an MCP session); see the ghmcp adapter.
type Prompter interface {
	// CanPromptURL reports whether the client can display a URL securely via
	// URL-mode elicitation.
	CanPromptURL() bool

	// PromptURL securely presents an authorization URL to the user and blocks
	// until the user acknowledges, declines, or ctx is done. Returning nil means
	// the prompt was shown (not that authorization completed); the caller waits
	// for the OAuth flow itself to finish. It returns ErrPromptDeclined if the
	// user declines or cancels, or ErrPromptUnavailable if the prompt could not
	// be delivered.
	PromptURL(ctx context.Context, p Prompt) error

	// CanPromptForm reports whether the client supports form elicitation, used
	// to display a device code when URL elicitation is unavailable.
	CanPromptForm() bool

	// PromptForm presents a textual acknowledgement prompt and blocks until the
	// user responds. It returns ErrPromptDeclined if the user declines, or
	// ErrPromptUnavailable if the prompt could not be delivered.
	PromptForm(ctx context.Context, p Prompt) error
}

// canPromptURL reports URL support, tolerating a nil Prompter.
func canPromptURL(p Prompter) bool { return p != nil && p.CanPromptURL() }

// canPromptForm reports form support, tolerating a nil Prompter.
func canPromptForm(p Prompter) bool { return p != nil && p.CanPromptForm() }
