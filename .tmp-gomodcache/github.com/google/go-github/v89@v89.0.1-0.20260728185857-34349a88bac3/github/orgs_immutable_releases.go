// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ImmutableReleaseSettings represents the response from the immutable releases settings endpoint.
type ImmutableReleaseSettings struct {
	// EnforcedRepositories specifies how immutable releases are enforced in the organization. Possible values include "all", "none", or "selected".
	EnforcedRepositories *string `json:"enforced_repositories,omitempty"`
	// SelectedRepositoriesURL provides the API URL for managing the repositories
	// selected for immutable releases enforcement when EnforcedRepositories is set to "selected".
	SelectedRepositoriesURL *string `json:"selected_repositories_url,omitempty"`
}

// ImmutableReleasePolicy is for setting the immutable releases policy for repositories in an organization.
type ImmutableReleasePolicy struct {
	// EnforcedRepositories specifies how immutable releases are enforced in the organization. Possible values include "all", "none", or "selected".
	EnforcedRepositories *string `json:"enforced_repositories,omitempty"`
	// An array of repository ids for which immutable releases enforcement should be applied.
	// You can only provide a list of repository ids when the enforced_repositories is set to "selected"
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitempty"`
}

// setImmutableReleasesRepositoriesOptions represents the request body for setting repositories.
type setImmutableReleasesRepositoriesOptions struct {
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids"`
}

// GetImmutableReleasesSettings returns the immutable releases configuration that applies to repositories within the given organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs?apiVersion=2022-11-28#get-immutable-releases-settings-for-an-organization
//
//meta:operation GET /orgs/{org}/settings/immutable-releases
func (s *OrganizationsService) GetImmutableReleasesSettings(ctx context.Context, org string) (*ImmutableReleaseSettings, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/immutable-releases", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var settings *ImmutableReleaseSettings
	resp, err := s.client.Do(req, &settings)
	if err != nil {
		return nil, resp, err
	}

	return settings, resp, nil
}

// UpdateImmutableReleasesSettings sets immutable releases settings for an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs?apiVersion=2022-11-28#set-immutable-releases-settings-for-an-organization
//
//meta:operation PUT /orgs/{org}/settings/immutable-releases
func (s *OrganizationsService) UpdateImmutableReleasesSettings(ctx context.Context, org string, body ImmutableReleasePolicy) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/immutable-releases", org)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// ListImmutableReleaseRepositories lists selected repositories for immutable releases enforcement in an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs?apiVersion=2022-11-28#list-selected-repositories-for-immutable-releases-enforcement
//
//meta:operation GET /orgs/{org}/settings/immutable-releases/repositories
func (s *OrganizationsService) ListImmutableReleaseRepositories(ctx context.Context, org string, opts *ListOptions) (*ListRepositories, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/immutable-releases/repositories", org)

	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var repositories *ListRepositories
	resp, err := s.client.Do(req, &repositories)
	if err != nil {
		return nil, resp, err
	}

	return repositories, resp, nil
}

// SetImmutableReleaseRepositories sets selected repositories for immutable releases enforcement.
// It requires the organization's immutable releases policy for enforced_repositories to be set to "selected".
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs?apiVersion=2022-11-28#set-selected-repositories-for-immutable-releases-enforcement
//
//meta:operation PUT /orgs/{org}/settings/immutable-releases/repositories
func (s *OrganizationsService) SetImmutableReleaseRepositories(ctx context.Context, org string, repositoryIDs []int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/immutable-releases/repositories", org)

	body := &setImmutableReleasesRepositoriesOptions{
		SelectedRepositoryIDs: repositoryIDs,
	}

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// EnableRepositoryForImmutableRelease enables a selected repository for immutable releases in an organization.
// It requires enforced_repositories to be set to "selected".
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs?apiVersion=2022-11-28#enable-a-selected-repository-for-immutable-releases-in-an-organization
//
//meta:operation PUT /orgs/{org}/settings/immutable-releases/repositories/{repository_id}
func (s *OrganizationsService) EnableRepositoryForImmutableRelease(ctx context.Context, org string, repoID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/immutable-releases/repositories/%v", org, repoID)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// DisableRepositoryForImmutableRelease removes a repository from the organization's selected list for immutable releases enforcement.
// It requires enforced_repositories to be set to "selected".
//
// GitHub API docs: https://docs.github.com/rest/orgs/orgs?apiVersion=2022-11-28#disable-a-selected-repository-for-immutable-releases-in-an-organization
//
//meta:operation DELETE /orgs/{org}/settings/immutable-releases/repositories/{repository_id}
func (s *OrganizationsService) DisableRepositoryForImmutableRelease(ctx context.Context, org string, repoID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/immutable-releases/repositories/%v", org, repoID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
