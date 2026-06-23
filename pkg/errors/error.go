package errors

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"time"

	"github.com/github/github-mcp-server/pkg/utils"
	"github.com/google/go-github/v87/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type GitHubAPIError struct {
	Message  string           `json:"message"`
	Response *github.Response `json:"-"`
	Err      error            `json:"-"`
}

// NewGitHubAPIError creates a new GitHubAPIError with the provided message, response, and error.
func newGitHubAPIError(message string, resp *github.Response, err error) *GitHubAPIError {
	return &GitHubAPIError{
		Message:  message,
		Response: resp,
		Err:      err,
	}
}

func (e *GitHubAPIError) Error() string {
	return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
}

type GitHubGraphQLError struct {
	Message string `json:"message"`
	Err     error  `json:"-"`
}

func newGitHubGraphQLError(message string, err error) *GitHubGraphQLError {
	return &GitHubGraphQLError{
		Message: message,
		Err:     err,
	}
}

func (e *GitHubGraphQLError) Error() string {
	return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
}

type GitHubRawAPIError struct {
	Message  string         `json:"message"`
	Response *http.Response `json:"-"`
	Err      error          `json:"-"`
}

func newGitHubRawAPIError(message string, resp *http.Response, err error) *GitHubRawAPIError {
	return &GitHubRawAPIError{
		Message:  message,
		Response: resp,
		Err:      err,
	}
}

func (e *GitHubRawAPIError) Error() string {
	return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
}

type GitHubErrorKey struct{}
type GitHubCtxErrors struct {
	api     []*GitHubAPIError
	graphQL []*GitHubGraphQLError
	raw     []*GitHubRawAPIError
}

// ContextWithGitHubErrors updates or creates a context with a pointer to GitHub error information (to be used by middleware).
func ContextWithGitHubErrors(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		// If the context already has GitHubCtxErrors, we just empty the slices to start fresh
		val.api = []*GitHubAPIError{}
		val.graphQL = []*GitHubGraphQLError{}
		val.raw = []*GitHubRawAPIError{}
	} else {
		// If not, we create a new GitHubCtxErrors and set it in the context
		ctx = context.WithValue(ctx, GitHubErrorKey{}, &GitHubCtxErrors{})
	}

	return ctx
}

// GetGitHubAPIErrors retrieves the slice of GitHubAPIErrors from the context.
func GetGitHubAPIErrors(ctx context.Context) ([]*GitHubAPIError, error) {
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		return val.api, nil // return the slice of API errors from the context
	}
	return nil, fmt.Errorf("context does not contain GitHubCtxErrors")
}

// GetGitHubGraphQLErrors retrieves the slice of GitHubGraphQLErrors from the context.
func GetGitHubGraphQLErrors(ctx context.Context) ([]*GitHubGraphQLError, error) {
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		return val.graphQL, nil // return the slice of GraphQL errors from the context
	}
	return nil, fmt.Errorf("context does not contain GitHubCtxErrors")
}

// GetGitHubRawAPIErrors retrieves the slice of GitHubRawAPIErrors from the context.
func GetGitHubRawAPIErrors(ctx context.Context) ([]*GitHubRawAPIError, error) {
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		return val.raw, nil // return the slice of raw API errors from the context
	}
	return nil, fmt.Errorf("context does not contain GitHubCtxErrors")
}

func NewGitHubAPIErrorToCtx(ctx context.Context, message string, resp *github.Response, err error) (context.Context, error) {
	apiErr := newGitHubAPIError(message, resp, err)
	if ctx != nil {
		_, _ = addGitHubAPIErrorToContext(ctx, apiErr) // Explicitly ignore error for graceful handling
	}
	return ctx, nil
}

func NewGitHubGraphQLErrorToCtx(ctx context.Context, message string, err error) (context.Context, error) {
	graphQLErr := newGitHubGraphQLError(message, err)
	if ctx != nil {
		_, _ = addGitHubGraphQLErrorToContext(ctx, graphQLErr) // Explicitly ignore error for graceful handling
	}
	return ctx, nil
}

func addGitHubAPIErrorToContext(ctx context.Context, err *GitHubAPIError) (context.Context, error) {
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		val.api = append(val.api, err) // append the error to the existing slice in the context
		return ctx, nil
	}
	return nil, fmt.Errorf("context does not contain GitHubCtxErrors")
}

func addGitHubGraphQLErrorToContext(ctx context.Context, err *GitHubGraphQLError) (context.Context, error) {
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		val.graphQL = append(val.graphQL, err) // append the error to the existing slice in the context
		return ctx, nil
	}
	return nil, fmt.Errorf("context does not contain GitHubCtxErrors")
}

func addRawAPIErrorToContext(ctx context.Context, err *GitHubRawAPIError) (context.Context, error) {
	if val, ok := ctx.Value(GitHubErrorKey{}).(*GitHubCtxErrors); ok {
		val.raw = append(val.raw, err)
		return ctx, nil
	}

	return nil, fmt.Errorf("context does not contain GitHubCtxErrors")
}

// NewGitHubAPIErrorResponse returns an mcp.NewToolResultError and retains the error in the context for access via middleware
func NewGitHubAPIErrorResponse(ctx context.Context, message string, resp *github.Response, err error) *mcp.CallToolResult {
	apiErr := newGitHubAPIError(message, resp, err)
	if ctx != nil {
		_, _ = addGitHubAPIErrorToContext(ctx, apiErr) // Explicitly ignore error for graceful handling
	}

	var rateLimitErr *github.RateLimitError
	if stderrors.As(err, &rateLimitErr) {
		resetTime := rateLimitErr.Rate.Reset.Time
		if !resetTime.IsZero() {
			retryIn := time.Until(resetTime).Round(time.Second)
			if retryIn > 0 {
				return utils.NewToolResultError(fmt.Sprintf(
					"%s: GitHub API rate limit exceeded. Retry after %v.", message, retryIn))
			}
		}
		return utils.NewToolResultError(fmt.Sprintf(
			"%s: GitHub API rate limit exceeded. Wait before retrying.", message))
	}

	var abuseErr *github.AbuseRateLimitError
	if stderrors.As(err, &abuseErr) {
		if abuseErr.RetryAfter != nil {
			retryAfter := abuseErr.RetryAfter.Round(time.Second)
			if retryAfter > 0 {
				return utils.NewToolResultError(fmt.Sprintf(
					"%s: GitHub secondary rate limit exceeded. Retry after %v.",
					message, retryAfter))
			}
		}
		return utils.NewToolResultError(fmt.Sprintf(
			"%s: GitHub secondary rate limit exceeded. Wait before retrying.", message))
	}

	return utils.NewToolResultErrorFromErr(message, err)
}

// NewGitHubGraphQLErrorResponse returns an mcp.NewToolResultError and retains the error in the context for access via middleware
func NewGitHubGraphQLErrorResponse(ctx context.Context, message string, err error) *mcp.CallToolResult {
	graphQLErr := newGitHubGraphQLError(message, err)
	if ctx != nil {
		_, _ = addGitHubGraphQLErrorToContext(ctx, graphQLErr) // Explicitly ignore error for graceful handling
	}
	return utils.NewToolResultErrorFromErr(message, err)
}

// NewGitHubRawAPIErrorResponse returns an mcp.NewToolResultError and retains the error in the context for access via middleware
func NewGitHubRawAPIErrorResponse(ctx context.Context, message string, resp *http.Response, err error) *mcp.CallToolResult {
	rawErr := newGitHubRawAPIError(message, resp, err)
	if ctx != nil {
		_, _ = addRawAPIErrorToContext(ctx, rawErr) // Explicitly ignore error for graceful handling
	}
	return utils.NewToolResultErrorFromErr(message, err)
}

// NewGitHubAPIStatusErrorResponse handles cases where the API call succeeds (err == nil)
// but returns an unexpected HTTP status code. It creates a synthetic error from the
// status code and response body, then records it in context for observability tracking.
func NewGitHubAPIStatusErrorResponse(ctx context.Context, message string, resp *github.Response, body []byte) *mcp.CallToolResult {
	err := fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	return NewGitHubAPIErrorResponse(ctx, message, resp, err)
}

// StructuredResolutionError is a machine-readable error returned by name-resolution
// helpers (e.g. resolving a project field or single-select option by name). Agents
// can parse the JSON body to self-correct without re-prompting.
//
// Kind values:
//   - "field_not_found"      — no project field matches the supplied name
//   - "field_ambiguous"      — more than one project field shares the supplied name
//   - "option_not_found"     — no option on the resolved single-select field matches
//   - "option_ambiguous"     — duplicate option names on the resolved field
//   - "item_not_in_project"  — the issue/PR exists but is not an item on the project
//   - "wrong_field_type"     — the named field is not the data type the caller expected
type StructuredResolutionError struct {
	Kind       string `json:"error"`
	Name       string `json:"name,omitempty"`
	Field      string `json:"field,omitempty"`
	Candidates []any  `json:"candidates,omitempty"`
	Hint       string `json:"hint,omitempty"`
}

// Error implements the error interface; the message is the JSON body so that the
// downstream tool result also carries the structured payload as plain text.
func (e *StructuredResolutionError) Error() string {
	b, err := json.Marshal(e)
	if err != nil {
		return fmt.Sprintf(`{"error":%q,"name":%q}`, e.Kind, e.Name)
	}
	return string(b)
}

// NewStructuredResolutionError constructs a StructuredResolutionError.
func NewStructuredResolutionError(kind, name, hint string, candidates []any) *StructuredResolutionError {
	return &StructuredResolutionError{
		Kind:       kind,
		Name:       name,
		Hint:       hint,
		Candidates: candidates,
	}
}

// NewStructuredResolutionErrorResponse returns an mcp.CallToolResult whose text body
// is the JSON-serialised StructuredResolutionError, suitable for agent self-correction.
func NewStructuredResolutionErrorResponse(err *StructuredResolutionError) *mcp.CallToolResult {
	return utils.NewToolResultError(err.Error())
}
