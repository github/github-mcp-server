// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ListAllRepositoryRulesets gets all the repository rulesets for the specified organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/rules?apiVersion=2022-11-28#get-all-organization-repository-rulesets
//
//meta:operation GET /orgs/{org}/rulesets
func (s *OrganizationsService) ListAllRepositoryRulesets(ctx context.Context, org string, opts *ListOptions) ([]*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("orgs/%v/rulesets", org)

	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var rulesets []*RepositoryRuleset
	resp, err := s.client.Do(req, &rulesets)
	if err != nil {
		return nil, resp, err
	}

	return rulesets, resp, nil
}

// CreateRepositoryRuleset creates a repository ruleset for the specified organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/rules?apiVersion=2022-11-28#create-an-organization-repository-ruleset
//
//meta:operation POST /orgs/{org}/rulesets
func (s *OrganizationsService) CreateRepositoryRuleset(ctx context.Context, org string, body RepositoryRuleset) (*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("orgs/%v/rulesets", org)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var rs *RepositoryRuleset
	resp, err := s.client.Do(req, &rs)
	if err != nil {
		return nil, resp, err
	}

	return rs, resp, nil
}

// GetRepositoryRuleset gets a repository ruleset for the specified organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/rules?apiVersion=2022-11-28#get-an-organization-repository-ruleset
//
//meta:operation GET /orgs/{org}/rulesets/{ruleset_id}
func (s *OrganizationsService) GetRepositoryRuleset(ctx context.Context, org string, rulesetID int64) (*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("orgs/%v/rulesets/%v", org, rulesetID)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var ruleset *RepositoryRuleset
	resp, err := s.client.Do(req, &ruleset)
	if err != nil {
		return nil, resp, err
	}

	return ruleset, resp, nil
}

// UpdateRepositoryRuleset updates a repository ruleset for the specified organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/rules?apiVersion=2022-11-28#update-an-organization-repository-ruleset
//
//meta:operation PUT /orgs/{org}/rulesets/{ruleset_id}
func (s *OrganizationsService) UpdateRepositoryRuleset(ctx context.Context, org string, rulesetID int64, body RepositoryRuleset) (*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("orgs/%v/rulesets/%v", org, rulesetID)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var rs *RepositoryRuleset
	resp, err := s.client.Do(req, &rs)
	if err != nil {
		return nil, resp, err
	}

	return rs, resp, nil
}

// DeleteRepositoryRuleset deletes a repository ruleset from the specified organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/rules?apiVersion=2022-11-28#delete-an-organization-repository-ruleset
//
//meta:operation DELETE /orgs/{org}/rulesets/{ruleset_id}
func (s *OrganizationsService) DeleteRepositoryRuleset(ctx context.Context, org string, rulesetID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/rulesets/%v", org, rulesetID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
