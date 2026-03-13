package errors

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/go-github/v82/github"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GitHubAPIError represents an error from the GitHub REST API.
// Use errors.As to extract this from a CallToolResult.GetError().
type GitHubAPIError struct {
	Message  string           `json:"message"`
	Response *github.Response `json:"-"`
	Err      error            `json:"-"`
}

// NewGitHubAPIError creates a new GitHubAPIError with the provided message, response, and error.
func NewGitHubAPIError(message string, resp *github.Response, err error) *GitHubAPIError {
	return &GitHubAPIError{
		Message:  message,
		Response: resp,
		Err:      err,
	}
}

func (e *GitHubAPIError) Error() string {
	return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
}

func (e *GitHubAPIError) Unwrap() error {
	return e.Err
}

// GitHubGraphQLError represents an error from the GitHub GraphQL API.
// Use errors.As to extract this from a CallToolResult.GetError().
type GitHubGraphQLError struct {
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// NewGitHubGraphQLError creates a new GitHubGraphQLError with the provided message and error.
func NewGitHubGraphQLError(message string, err error) *GitHubGraphQLError {
	return &GitHubGraphQLError{
		Message: message,
		Err:     err,
	}
}

func (e *GitHubGraphQLError) Error() string {
	return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
}

func (e *GitHubGraphQLError) Unwrap() error {
	return e.Err
}

// GitHubRawAPIError represents an error from a raw HTTP GitHub API call.
// Use errors.As to extract this from a CallToolResult.GetError().
type GitHubRawAPIError struct {
	Message  string         `json:"message"`
	Response *http.Response `json:"-"`
	Err      error          `json:"-"`
}

// NewGitHubRawAPIError creates a new GitHubRawAPIError with the provided message, response, and error.
func NewGitHubRawAPIError(message string, resp *http.Response, err error) *GitHubRawAPIError {
	return &GitHubRawAPIError{
		Message:  message,
		Response: resp,
		Err:      err,
	}
}

func (e *GitHubRawAPIError) Error() string {
	return fmt.Errorf("%s: %w", e.Message, e.Err).Error()
}

func (e *GitHubRawAPIError) Unwrap() error {
	return e.Err
}

// NewGitHubAPIErrorResponse returns a CallToolResult with the error set via SetError,
// embedding a typed GitHubAPIError accessible via result.GetError() and errors.As.
func NewGitHubAPIErrorResponse(_ context.Context, message string, resp *github.Response, err error) *mcp.CallToolResult {
	apiErr := NewGitHubAPIError(message, resp, err)
	var result mcp.CallToolResult
	result.SetError(apiErr)
	return &result
}

// NewGitHubGraphQLErrorResponse returns a CallToolResult with the error set via SetError,
// embedding a typed GitHubGraphQLError accessible via result.GetError() and errors.As.
func NewGitHubGraphQLErrorResponse(_ context.Context, message string, err error) *mcp.CallToolResult {
	graphQLErr := NewGitHubGraphQLError(message, err)
	var result mcp.CallToolResult
	result.SetError(graphQLErr)
	return &result
}

// NewGitHubRawAPIErrorResponse returns a CallToolResult with the error set via SetError,
// embedding a typed GitHubRawAPIError accessible via result.GetError() and errors.As.
func NewGitHubRawAPIErrorResponse(_ context.Context, message string, resp *http.Response, err error) *mcp.CallToolResult {
	rawErr := NewGitHubRawAPIError(message, resp, err)
	var result mcp.CallToolResult
	result.SetError(rawErr)
	return &result
}

// NewGitHubAPIStatusErrorResponse handles cases where the API call succeeds (err == nil)
// but returns an unexpected HTTP status code. It creates a synthetic error from the
// status code and response body, then sets it on the result via SetError.
func NewGitHubAPIStatusErrorResponse(ctx context.Context, message string, resp *github.Response, body []byte) *mcp.CallToolResult {
	err := fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	return NewGitHubAPIErrorResponse(ctx, message, resp, err)
}
