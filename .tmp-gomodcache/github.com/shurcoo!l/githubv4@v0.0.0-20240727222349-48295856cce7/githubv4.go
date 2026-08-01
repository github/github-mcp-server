package githubv4

import (
	"context"
	"net/http"

	"github.com/shurcooL/graphql"
)

// Client is a GitHub GraphQL API v4 client.
type Client struct {
	client *graphql.Client
}

// NewClient creates a new GitHub GraphQL API v4 client with the provided http.Client.
// If httpClient is nil, then http.DefaultClient is used.
//
// Note that GitHub GraphQL API v4 requires authentication, so
// the provided http.Client is expected to take care of that.
func NewClient(httpClient *http.Client) *Client {
	return &Client{
		client: graphql.NewClient("https://api.github.com/graphql", httpClient),
	}
}

// NewEnterpriseClient creates a new GitHub GraphQL API v4 client for the GitHub Enterprise
// instance with the specified GraphQL endpoint URL, using the provided http.Client.
// If httpClient is nil, then http.DefaultClient is used.
//
// Note that GitHub GraphQL API v4 requires authentication, so
// the provided http.Client is expected to take care of that.
func NewEnterpriseClient(url string, httpClient *http.Client) *Client {
	return &Client{
		client: graphql.NewClient(url, httpClient),
	}
}

// Query executes a single GraphQL query request,
// with a query derived from q, populating the response into it.
// q should be a pointer to struct that corresponds to the GitHub GraphQL schema.
func (c *Client) Query(ctx context.Context, q interface{}, variables map[string]interface{}) error {
	return c.client.Query(ctx, q, variables)
}

// Mutate executes a single GraphQL mutation request,
// with a mutation derived from m, populating the response into it.
// m should be a pointer to struct that corresponds to the GitHub GraphQL schema.
// Provided input will be set as a variable named "input".
func (c *Client) Mutate(ctx context.Context, m interface{}, input Input, variables map[string]interface{}) error {
	if variables == nil {
		variables = map[string]interface{}{"input": input}
	} else {
		variables["input"] = input
	}
	return c.client.Mutate(ctx, m, variables)
}
