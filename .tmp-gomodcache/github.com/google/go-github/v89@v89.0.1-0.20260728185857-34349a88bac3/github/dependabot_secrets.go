// Copyright 2022 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// GetRepoPublicKey gets a public key that should be used for Dependabot secret encryption.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#get-a-repository-public-key
//
//meta:operation GET /repos/{owner}/{repo}/dependabot/secrets/public-key
func (s *DependabotService) GetRepoPublicKey(ctx context.Context, owner, repo string) (*PublicKey, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/dependabot/secrets/public-key", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var pubKey *PublicKey
	resp, err := s.client.Do(req, &pubKey)
	if err != nil {
		return nil, resp, err
	}

	return pubKey, resp, nil
}

// GetOrgPublicKey gets a public key that should be used for Dependabot secret encryption.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#get-an-organization-public-key
//
//meta:operation GET /orgs/{org}/dependabot/secrets/public-key
func (s *DependabotService) GetOrgPublicKey(ctx context.Context, org string) (*PublicKey, *Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/public-key", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var pubKey *PublicKey
	resp, err := s.client.Do(req, &pubKey)
	if err != nil {
		return nil, resp, err
	}

	return pubKey, resp, nil
}

// ListRepoSecrets lists all Dependabot secrets available in a repository
// without revealing their encrypted values.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#list-repository-secrets
//
//meta:operation GET /repos/{owner}/{repo}/dependabot/secrets
func (s *DependabotService) ListRepoSecrets(ctx context.Context, owner, repo string, opts *ListOptions) (*Secrets, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/dependabot/secrets", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var secrets *Secrets
	resp, err := s.client.Do(req, &secrets)
	if err != nil {
		return nil, resp, err
	}

	return secrets, resp, nil
}

// ListOrgSecrets lists all Dependabot secrets available in an organization
// without revealing their encrypted values.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#list-organization-secrets
//
//meta:operation GET /orgs/{org}/dependabot/secrets
func (s *DependabotService) ListOrgSecrets(ctx context.Context, org string, opts *ListOptions) (*Secrets, *Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var secrets *Secrets
	resp, err := s.client.Do(req, &secrets)
	if err != nil {
		return nil, resp, err
	}

	return secrets, resp, nil
}

// GetRepoSecret gets a single repository Dependabot secret without revealing its encrypted value.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#get-a-repository-secret
//
//meta:operation GET /repos/{owner}/{repo}/dependabot/secrets/{secret_name}
func (s *DependabotService) GetRepoSecret(ctx context.Context, owner, repo, name string) (*Secret, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/dependabot/secrets/%v", owner, repo, name)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var secret *Secret
	resp, err := s.client.Do(req, &secret)
	if err != nil {
		return nil, resp, err
	}

	return secret, resp, nil
}

// GetOrgSecret gets a single organization Dependabot secret without revealing its encrypted value.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#get-an-organization-secret
//
//meta:operation GET /orgs/{org}/dependabot/secrets/{secret_name}
func (s *DependabotService) GetOrgSecret(ctx context.Context, org, name string) (*Secret, *Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v", org, name)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var secret *Secret
	resp, err := s.client.Do(req, &secret)
	if err != nil {
		return nil, resp, err
	}

	return secret, resp, nil
}

// CreateOrUpdateRepoSecret creates or updates a repository Dependabot secret with an encrypted value.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#create-or-update-a-repository-secret
//
//meta:operation PUT /repos/{owner}/{repo}/dependabot/secrets/{secret_name}
func (s *DependabotService) CreateOrUpdateRepoSecret(ctx context.Context, owner, repo, name string, body SecretRequest) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/dependabot/secrets/%v", owner, repo, name)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// CreateOrUpdateOrgSecret creates or updates an organization Dependabot secret with an encrypted value.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#create-or-update-an-organization-secret
//
//meta:operation PUT /orgs/{org}/dependabot/secrets/{secret_name}
func (s *DependabotService) CreateOrUpdateOrgSecret(ctx context.Context, org, name string, body SecretOrgRequest) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v", org, name)

	var bodyOverride any = body
	if len(body.SelectedRepositoryIDs) > 0 {
		repoIDs := make([]string, len(body.SelectedRepositoryIDs))
		for i, id := range body.SelectedRepositoryIDs {
			repoIDs[i] = fmt.Sprintf("%v", id)
		}

		bodyOverride = struct {
			SecretOrgRequest
			SelectedRepositoryIDs []string `json:"selected_repository_ids,omitzero"`
		}{
			SecretOrgRequest:      body,
			SelectedRepositoryIDs: repoIDs,
		}
	}

	req, err := s.client.NewRequest(ctx, "PUT", u, bodyOverride)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteRepoSecret deletes a Dependabot secret in a repository using the secret name.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#delete-a-repository-secret
//
//meta:operation DELETE /repos/{owner}/{repo}/dependabot/secrets/{secret_name}
func (s *DependabotService) DeleteRepoSecret(ctx context.Context, owner, repo, name string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/dependabot/secrets/%v", owner, repo, name)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteOrgSecret deletes a Dependabot secret in an organization using the secret name.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#delete-an-organization-secret
//
//meta:operation DELETE /orgs/{org}/dependabot/secrets/{secret_name}
func (s *DependabotService) DeleteOrgSecret(ctx context.Context, org, name string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v", org, name)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListSelectedReposForOrgSecret lists all repositories that have access to a Dependabot secret.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#list-selected-repositories-for-an-organization-secret
//
//meta:operation GET /orgs/{org}/dependabot/secrets/{secret_name}/repositories
func (s *DependabotService) ListSelectedReposForOrgSecret(ctx context.Context, org, name string, opts *ListOptions) (*SelectedReposList, *Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v/repositories", org, name)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var result *SelectedReposList
	resp, err := s.client.Do(req, &result)
	if err != nil {
		return nil, resp, err
	}

	return result, resp, nil
}

// SetSelectedReposForOrgSecret sets the repositories that have access to a Dependabot secret.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#set-selected-repositories-for-an-organization-secret
//
//meta:operation PUT /orgs/{org}/dependabot/secrets/{secret_name}/repositories
func (s *DependabotService) SetSelectedReposForOrgSecret(ctx context.Context, org, name string, ids []int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v/repositories", org, name)

	type repoIDs struct {
		SelectedIDs []int64 `json:"selected_repository_ids"`
	}

	req, err := s.client.NewRequest(ctx, "PUT", u, repoIDs{SelectedIDs: ids})
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// AddSelectedRepoToOrgSecret adds a repository to an organization Dependabot secret.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#add-selected-repository-to-an-organization-secret
//
//meta:operation PUT /orgs/{org}/dependabot/secrets/{secret_name}/repositories/{repository_id}
func (s *DependabotService) AddSelectedRepoToOrgSecret(ctx context.Context, org, name string, repoID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v/repositories/%v", org, name, repoID)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// RemoveSelectedRepoFromOrgSecret removes a repository from an organization Dependabot secret.
//
// GitHub API docs: https://docs.github.com/rest/dependabot/secrets?apiVersion=2022-11-28#remove-selected-repository-from-an-organization-secret
//
//meta:operation DELETE /orgs/{org}/dependabot/secrets/{secret_name}/repositories/{repository_id}
func (s *DependabotService) RemoveSelectedRepoFromOrgSecret(ctx context.Context, org, name string, repoID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/dependabot/secrets/%v/repositories/%v", org, name, repoID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
