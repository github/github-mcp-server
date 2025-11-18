package errors

import (
	"context"
	"fmt"

	"github.com/google/go-github/v79/github"
	"github.com/mark3labs/mcp-go/mcp"
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

type GitHubErrorKey struct{}
type GitHubCtxErrors struct {
	api     []*GitHubAPIError
	graphQL []*GitHubGraphQLError
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

func NewGitHubAPIErrorToCtx(ctx context.Context, message string, resp *github.Response, err error) (context.Context, error) {
	apiErr := newGitHubAPIError(message, resp, err)
	if ctx != nil {
		_, _ = addGitHubAPIErrorToContext(ctx, apiErr) // Explicitly ignore error for graceful handling
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

// NewGitHubAPIErrorResponse returns an mcp.NewToolResultError and retains the error in the context for access via middleware
func NewGitHubAPIErrorResponse(ctx context.Context, message string, resp *github.Response, err error) *mcp.CallToolResult {
	apiErr := newGitHubAPIError(message, resp, err)
	if ctx != nil {
		_, _ = addGitHubAPIErrorToContext(ctx, apiErr) // Explicitly ignore error for graceful handling
	}
	return mcp.NewToolResultErrorFromErr(message, err)
}

// NewGitHubGraphQLErrorResponse returns an mcp.NewToolResultError and retains the error in the context for access via middleware
func NewGitHubGraphQLErrorResponse(ctx context.Context, message string, err error) *mcp.CallToolResult {
	graphQLErr := newGitHubGraphQLError(message, err)
	if ctx != nil {
		_, _ = addGitHubGraphQLErrorToContext(ctx, graphQLErr) // Explicitly ignore error for graceful handling
	}
	return mcp.NewToolResultErrorFromErr(message, err)
}
