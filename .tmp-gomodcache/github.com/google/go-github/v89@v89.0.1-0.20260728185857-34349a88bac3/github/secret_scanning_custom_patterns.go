// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// SecretScanningCustomPattern represents a custom pattern for secret scanning,
// as returned by the GitHub API.
//
// GitHub API docs: https://docs.github.com/en/rest/secret-scanning/secret-scanning
type SecretScanningCustomPattern struct {
	// ID is the unique identifier of the custom pattern.
	ID int64 `json:"id"`

	// Name is the name of the custom pattern.
	Name string `json:"name"`

	// Pattern is the regular expression that defines the custom pattern.
	Pattern string `json:"pattern"`

	// Slug is the URL-friendly identifier derived from the pattern name.
	Slug string `json:"slug"`

	// State is the publish state of the pattern. Possible values are:
	// "published" or "unpublished".
	State string `json:"state"`

	// PushProtectionEnabled reports whether push protection is enabled for
	// this custom pattern.
	PushProtectionEnabled bool `json:"push_protection_enabled"`

	// StartDelimiter is the start delimiter regex for the custom pattern.
	StartDelimiter *string `json:"start_delimiter,omitempty"`

	// EndDelimiter is the end delimiter regex for the custom pattern.
	EndDelimiter *string `json:"end_delimiter,omitempty"`

	// MustMatch is the list of regexes that the secret must match.
	MustMatch []string `json:"must_match,omitempty"`

	// MustNotMatch is the list of regexes that the secret must not match.
	MustNotMatch []string `json:"must_not_match,omitempty"`

	// CustomPatternVersion is used to confirm you're updating the current
	// version of the pattern and to avoid unintentionally overriding
	// someone else's update.
	CustomPatternVersion *string `json:"custom_pattern_version,omitempty"`

	// CreatedAt is the time the custom pattern was created.
	CreatedAt *Timestamp `json:"created_at,omitempty"`

	// UpdatedAt is the time the custom pattern was last updated.
	UpdatedAt *Timestamp `json:"updated_at,omitempty"`
}

// SecretScanningCustomPatternRequest represents a single custom pattern to
// be created for secret scanning.
type SecretScanningCustomPatternRequest struct {
	// Name is the name of the custom pattern.
	Name string `json:"name"`

	// Pattern is the regular expression that defines the custom pattern.
	Pattern string `json:"pattern"`

	// StartDelimiter is the start delimiter regex for the custom pattern.
	// Defaults to a sensible boundary regex when not specified.
	StartDelimiter *string `json:"start_delimiter,omitempty"`

	// EndDelimiter is the end delimiter regex for the custom pattern.
	// Defaults to a sensible boundary regex when not specified.
	EndDelimiter *string `json:"end_delimiter,omitempty"`

	// MustMatch is the list of regexes that the secret must match.
	MustMatch []string `json:"must_match,omitempty"`

	// MustNotMatch is the list of regexes that the secret must not match.
	MustNotMatch []string `json:"must_not_match,omitempty"`
}

// SecretScanningCreateCustomPatternsRequest represents the bulk request body
// used to create one or more custom patterns.
type SecretScanningCreateCustomPatternsRequest struct {
	// Patterns is the list of custom patterns to create.
	Patterns []*SecretScanningCustomPatternRequest `json:"patterns"`
}

// SecretScanningCreateCustomPatternsResponse represents the bulk response
// returned after creating one or more custom patterns.
type SecretScanningCreateCustomPatternsResponse struct {
	// CreatedPatterns is the list of successfully created custom patterns.
	CreatedPatterns []*SecretScanningCustomPattern `json:"created_patterns"`
}

// SecretScanningCustomPatternToDelete identifies a single custom pattern to
// remove in a bulk delete operation.
type SecretScanningCustomPatternToDelete struct {
	// PatternID is the ID of the custom pattern to delete.
	PatternID int64 `json:"pattern_id"`

	// CustomPatternVersion is used to confirm you're deleting the current
	// version of the pattern.
	CustomPatternVersion *string `json:"custom_pattern_version,omitempty"`
}

// SecretScanningDeleteCustomPatternsRequest represents the bulk request body
// used to delete one or more custom patterns.
type SecretScanningDeleteCustomPatternsRequest struct {
	// Patterns is the list of custom patterns to delete.
	Patterns []*SecretScanningCustomPatternToDelete `json:"patterns"`

	// PostDeleteAction controls what happens to alerts associated with the
	// deleted patterns. Possible values are: "delete_alerts" (default) or
	// "resolve_alerts".
	PostDeleteAction *string `json:"post_delete_action,omitempty"`
}

// SecretScanningUpdateCustomPatternRequest represents the fields that can be
// updated on an existing custom pattern. At least one of Pattern,
// StartDelimiter, EndDelimiter, MustMatch, or MustNotMatch must be set.
type SecretScanningUpdateCustomPatternRequest struct {
	// Pattern is the updated regular expression of the custom pattern.
	Pattern *string `json:"pattern,omitempty"`

	// StartDelimiter is the updated start delimiter regex for the custom
	// pattern.
	StartDelimiter *string `json:"start_delimiter,omitempty"`

	// EndDelimiter is the updated end delimiter regex for the custom
	// pattern.
	EndDelimiter *string `json:"end_delimiter,omitempty"`

	// MustMatch is the updated list of regexes that the secret must match.
	MustMatch []string `json:"must_match,omitempty"`

	// MustNotMatch is the updated list of regexes that the secret must not
	// match.
	MustNotMatch []string `json:"must_not_match,omitempty"`

	// CustomPatternVersion is required and is used to confirm you're
	// updating the current version of the pattern.
	CustomPatternVersion *string `json:"custom_pattern_version"`
}

// SecretScanningCustomPatternListOptions specifies the optional parameters
// to the SecretScanningService.ListCustomPatternsForRepo method.
type SecretScanningCustomPatternListOptions struct {
	// State filters custom patterns by state. Possible values are:
	// "published" or "unpublished". When absent, returns patterns in all
	// states.
	State string `url:"state,omitempty"`

	// PushProtection filters custom patterns by whether push protection is
	// enabled. Possible values are: "enabled" or "disabled". When absent,
	// returns patterns regardless of push protection status.
	PushProtection string `url:"push_protection,omitempty"`

	// Sort is the property to sort the results by. Possible values are:
	// "created", "updated", or "name". Default: "created".
	Sort string `url:"sort,omitempty"`

	// Direction is the direction to sort the results by. Possible values
	// are: "asc" or "desc". Default: "desc".
	Direction string `url:"direction,omitempty"`

	// ListOptions controls pagination via Page and PerPage.
	ListOptions
}

// ListCustomPatternsForRepo lists the secret scanning custom patterns
// defined at the repository level.
//
// GitHub API docs: https://docs.github.com/rest/secret-scanning/custom-patterns?apiVersion=2022-11-28#list-repository-custom-patterns
//
//meta:operation GET /repos/{owner}/{repo}/secret-scanning/custom-patterns
func (s *SecretScanningService) ListCustomPatternsForRepo(ctx context.Context, owner, repo string, opts *SecretScanningCustomPatternListOptions) ([]*SecretScanningCustomPattern, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/secret-scanning/custom-patterns", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var patterns []*SecretScanningCustomPattern
	resp, err := s.client.Do(req, &patterns)
	if err != nil {
		return nil, resp, err
	}

	return patterns, resp, nil
}

// CreateCustomPatternsForRepo creates one or more secret scanning custom
// patterns at the repository level.
//
// GitHub API docs: https://docs.github.com/rest/secret-scanning/custom-patterns?apiVersion=2022-11-28#bulk-create-repository-custom-patterns
//
//meta:operation POST /repos/{owner}/{repo}/secret-scanning/custom-patterns
func (s *SecretScanningService) CreateCustomPatternsForRepo(ctx context.Context, owner, repo string, body SecretScanningCreateCustomPatternsRequest) (*SecretScanningCreateCustomPatternsResponse, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/secret-scanning/custom-patterns", owner, repo)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var result *SecretScanningCreateCustomPatternsResponse
	resp, err := s.client.Do(req, &result)
	if err != nil {
		return nil, resp, err
	}

	return result, resp, nil
}

// UpdateCustomPatternForRepo updates a single secret scanning custom pattern
// at the repository level.
//
// GitHub API docs: https://docs.github.com/rest/secret-scanning/custom-patterns?apiVersion=2022-11-28#update-a-repository-custom-pattern
//
//meta:operation PATCH /repos/{owner}/{repo}/secret-scanning/custom-patterns/{pattern_id}
func (s *SecretScanningService) UpdateCustomPatternForRepo(ctx context.Context, owner, repo string, patternID int64, body SecretScanningUpdateCustomPatternRequest) (*SecretScanningCustomPattern, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/secret-scanning/custom-patterns/%v", owner, repo, patternID)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var pattern *SecretScanningCustomPattern
	resp, err := s.client.Do(req, &pattern)
	if err != nil {
		return nil, resp, err
	}

	return pattern, resp, nil
}

// DeleteCustomPatternsForRepo deletes one or more secret scanning custom
// patterns at the repository level.
//
// GitHub API docs: https://docs.github.com/rest/secret-scanning/custom-patterns?apiVersion=2022-11-28#bulk-delete-repository-custom-patterns
//
//meta:operation DELETE /repos/{owner}/{repo}/secret-scanning/custom-patterns
func (s *SecretScanningService) DeleteCustomPatternsForRepo(ctx context.Context, owner, repo string, body SecretScanningDeleteCustomPatternsRequest) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/secret-scanning/custom-patterns", owner, repo)

	req, err := s.client.NewRequest(ctx, "DELETE", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
