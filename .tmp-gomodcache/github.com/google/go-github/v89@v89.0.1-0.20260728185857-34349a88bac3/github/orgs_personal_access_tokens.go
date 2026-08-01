// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// PersonalAccessToken represents the minimal representation of an organization programmatic access grant.
//
// GitHub API docs: https://docs.github.com/rest/orgs/personal-access-tokens?apiVersion=2022-11-28
type PersonalAccessToken struct {
	// "Unique identifier of the fine-grained personal access token.
	// The `pat_id` used to get details about an approved fine-grained personal access token.
	ID *int64 `json:"id"`

	// Owner is the GitHub user associated with the token.
	Owner *User `json:"owner"`

	// RepositorySelection is the type of repository selection requested.
	// Possible values are: "none", "all", "subset".
	RepositorySelection *string `json:"repository_selection"`

	// URL to the list of repositories the fine-grained personal access token can access.
	// Only follow when `repository_selection` is `subset`.
	RepositoriesURL *string `json:"repositories_url"`

	// Permissions are the permissions requested, categorized by type.
	Permissions *PersonalAccessTokenPermissions `json:"permissions"`

	// Date and time when the fine-grained personal access token was approved to access the organization.
	AccessGrantedAt *Timestamp `json:"access_granted_at"`

	// Whether the associated fine-grained personal access token has expired.
	TokenExpired *bool `json:"token_expired"`

	// Date and time when the associated fine-grained personal access token expires.
	TokenExpiresAt *Timestamp `json:"token_expires_at"`

	// TokenID
	TokenID *int64 `json:"token_id"`

	// TokenName
	TokenName *string `json:"token_name"`

	// Date and time when the associated fine-grained personal access token was last used for authentication.
	TokenLastUsedAt *Timestamp `json:"token_last_used_at"`
}

// ListFineGrainedPATOptions specifies optional parameters to ListFineGrainedPersonalAccessTokens.
type ListFineGrainedPATOptions struct {
	// The property by which to sort the results.
	// Default: created_at
	// Value: created_at
	Sort string `url:"sort,omitempty"`

	// The direction to sort the results by.
	// Default: desc
	// Value: asc, desc
	Direction string `url:"direction,omitempty"`

	// A list of owner usernames to use to filter the results.
	Owner []string `url:"-"`

	// The name of the repository to use to filter the results.
	Repository string `url:"repository,omitempty"`

	// The permission to use to filter the results.
	Permission string `url:"permission,omitempty"`

	// Only show fine-grained personal access tokens used before the given time.
	// This is a timestamp in ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ.
	LastUsedBefore string `url:"last_used_before,omitempty"`

	// Only show fine-grained personal access tokens used after the given time.
	// This is a timestamp in ISO 8601 format: YYYY-MM-DDTHH:MM:SSZ.
	LastUsedAfter string `url:"last_used_after,omitempty"`

	// TokenID filters results by the given fine-grained personal access token IDs.
	TokenID []int64 `url:"-"`

	ListOptions
}

// ListFineGrainedPersonalAccessTokens lists approved fine-grained personal access tokens owned by organization members that can access organization resources.
// Only GitHub Apps can call this API, using the `Personal access tokens` organization permissions (read).
//
// GitHub API docs: https://docs.github.com/rest/orgs/personal-access-tokens?apiVersion=2022-11-28#list-fine-grained-personal-access-tokens-with-access-to-organization-resources
//
//meta:operation GET /orgs/{org}/personal-access-tokens
func (s *OrganizationsService) ListFineGrainedPersonalAccessTokens(ctx context.Context, org string, opts *ListFineGrainedPATOptions) ([]*PersonalAccessToken, *Response, error) {
	u := fmt.Sprintf("orgs/%v/personal-access-tokens", org)
	// The `owner` parameter is a special case that uses the `owner[]=...` format and needs a custom function to format it correctly.
	u, err := addListFineGrainedPATOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, opts)
	if err != nil {
		return nil, nil, err
	}

	var pats []*PersonalAccessToken

	resp, err := s.client.Do(req, &pats)
	if err != nil {
		return nil, resp, err
	}

	return pats, resp, nil
}

// FineGrainedPersonalAccessTokenRequest represents the details of a request to access organization resources via a fine-grained personal access token.
type FineGrainedPersonalAccessTokenRequest struct {
	// Unique identifier of the request for access via fine-grained personal access token.
	ID int64 `json:"id"`

	// Reason is the reason for the request.
	Reason string `json:"reason"`

	// Owner is the GitHub user associated with the token.
	Owner User `json:"owner"`

	// RepositorySelection is the type of repository selection requested.
	// Possible values are: "none", "all", "subset".
	RepositorySelection string `json:"repository_selection"`

	// URL to the list of repositories the fine-grained personal access token can access.
	// Only follow when `repository_selection` is `subset`.
	RepositoriesURL string `json:"repositories_url"`

	// Permissions are the permissions requested, categorized by type.
	Permissions PersonalAccessTokenPermissions `json:"permissions"`

	// Date and time when the request was created.
	CreatedAt *Timestamp `json:"created_at"`

	// Whether the associated fine-grained personal access token has expired.
	TokenExpired bool `json:"token_expired"`

	// Date and time when the associated fine-grained personal access token expires.
	TokenExpiresAt *Timestamp `json:"token_expires_at"`

	// TokenID
	TokenID int64 `json:"token_id"`

	// TokenName
	TokenName string `json:"token_name"`

	// Date and time when the associated fine-grained personal access token was last used for authentication.
	TokenLastUsedAt *Timestamp `json:"token_last_used_at"`
}

// ListFineGrainedPersonalAccessTokenRequests lists requests to access organization resources via fine-grained personal access tokens.
// Only GitHub Apps can call this API, using the `Personal access tokens` organization permissions (read).
//
// GitHub API docs: https://docs.github.com/rest/orgs/personal-access-tokens?apiVersion=2022-11-28#list-requests-to-access-organization-resources-with-fine-grained-personal-access-tokens
//
//meta:operation GET /orgs/{org}/personal-access-token-requests
func (s *OrganizationsService) ListFineGrainedPersonalAccessTokenRequests(ctx context.Context, org string, opts *ListFineGrainedPATOptions) ([]*FineGrainedPersonalAccessTokenRequest, *Response, error) {
	u := fmt.Sprintf("orgs/%v/personal-access-token-requests", org)
	// The `owner` parameter is a special case that uses the `owner[]=...` format and needs a custom function to format it correctly.
	u, err := addListFineGrainedPATOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, opts)
	if err != nil {
		return nil, nil, err
	}

	var pats []*FineGrainedPersonalAccessTokenRequest
	resp, err := s.client.Do(req, &pats)
	if err != nil {
		return nil, resp, err
	}

	return pats, resp, nil
}

// ReviewPersonalAccessTokenRequestOptions specifies the parameters to the ReviewPersonalAccessTokenRequest method.
type ReviewPersonalAccessTokenRequestOptions struct {
	Action string  `json:"action"`
	Reason *string `json:"reason,omitempty"`
}

// ReviewPersonalAccessTokenRequest approves or denies a pending request to access organization resources via a fine-grained personal access token.
// Only GitHub Apps can call this API, using the `organization_personal_access_token_requests: write` permission.
// `action` can be one of `approve` or `deny`.
//
// GitHub API docs: https://docs.github.com/rest/orgs/personal-access-tokens?apiVersion=2022-11-28#review-a-request-to-access-organization-resources-with-a-fine-grained-personal-access-token
//
//meta:operation POST /orgs/{org}/personal-access-token-requests/{pat_request_id}
func (s *OrganizationsService) ReviewPersonalAccessTokenRequest(ctx context.Context, org string, requestID int64, opts ReviewPersonalAccessTokenRequestOptions) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/personal-access-token-requests/%v", org, requestID)

	req, err := s.client.NewRequest(ctx, "POST", u, &opts)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// addListFineGrainedPATOptions adds the owner and token_id parameters to the URL query string with the correct format if they are set.
//
// GitHub API expects the owner parameter to be a list of strings in the `owner[]=...` format.
// For multiple owner and token_id values, the owner and token_id parameters are repeated in the query string.
//
// Example:
// owner[]=user1&owner[]=user2&token_id[]=123&token_id[]=456
// This will filter the results to only include fine-grained personal access tokens owned by `user1` and `user2` and with token IDs `123` and `456`.
//
// This function ensures the owner and token_id parameters are formatted correctly in the URL query string.
func addListFineGrainedPATOptions(s string, opts *ListFineGrainedPATOptions) (string, error) {
	u, err := addOptions(s, opts)
	if err != nil {
		return s, err
	}

	if opts == nil {
		return "", errors.New("opts must be provided")
	}

	if len(opts.Owner) > 0 {
		ownerVals := make([]string, len(opts.Owner))
		for i, owner := range opts.Owner {
			ownerVals[i] = fmt.Sprintf("owner[]=%v", url.QueryEscape(owner))
		}
		ownerQuery := strings.Join(ownerVals, "&")

		if strings.Contains(u, "?") {
			u += "&" + ownerQuery
		} else {
			u += "?" + ownerQuery
		}
	}
	if len(opts.TokenID) > 0 {
		tokenIDVals := make([]string, len(opts.TokenID))
		for i, tokenID := range opts.TokenID {
			tokenIDVals[i] = fmt.Sprintf("token_id[]=%v", url.QueryEscape(strconv.FormatInt(tokenID, 10)))
		}
		tokenIDQuery := strings.Join(tokenIDVals, "&")

		if strings.Contains(u, "?") {
			u += "&" + tokenIDQuery
		} else {
			u += "?" + tokenIDQuery
		}
		return u, nil
	}

	return u, nil
}
