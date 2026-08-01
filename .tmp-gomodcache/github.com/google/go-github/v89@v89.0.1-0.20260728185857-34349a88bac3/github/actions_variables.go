// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
)

// ActionsCreateOrgVariableRequest represents a request to create an
// organization variable.
type ActionsCreateOrgVariableRequest struct {
	Name                  string  `json:"name"`
	Value                 string  `json:"value"`
	Visibility            string  `json:"visibility"`
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitzero"`
}

// ActionsUpdateOrgVariableRequest represents a request to update an
// organization variable.
type ActionsUpdateOrgVariableRequest struct {
	Name                  *string `json:"name,omitempty"`
	Value                 *string `json:"value,omitempty"`
	Visibility            *string `json:"visibility,omitempty"`
	SelectedRepositoryIDs []int64 `json:"selected_repository_ids,omitzero"`
}

// ActionsCreateVariableRequest represents a request to create a variable
// for a repository or repository environment variable.
type ActionsCreateVariableRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ActionsUpdateVariableRequest represents a request to update a variable
// for a repository or repository environment variable.
type ActionsUpdateVariableRequest struct {
	Name  *string `json:"name,omitempty"`
	Value *string `json:"value,omitempty"`
}

// ActionsVariable represents a repository action variable.
type ActionsVariable struct {
	Name      string     `json:"name"`
	Value     string     `json:"value"`
	CreatedAt *Timestamp `json:"created_at,omitempty"`
	UpdatedAt *Timestamp `json:"updated_at,omitempty"`

	// Used by ListOrgVariables and GetOrgVariables
	Visibility              *string `json:"visibility,omitempty"`
	SelectedRepositoriesURL *string `json:"selected_repositories_url,omitempty"`
}

// ActionsVariables represents one item from the ListVariables response.
type ActionsVariables struct {
	TotalCount int                `json:"total_count"`
	Variables  []*ActionsVariable `json:"variables"`
}

// ListRepoVariables lists all variables available in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#list-repository-variables
//
//meta:operation GET /repos/{owner}/{repo}/actions/variables
func (s *ActionsService) ListRepoVariables(ctx context.Context, owner, repo string, opts *ListOptions) (*ActionsVariables, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/variables", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variables *ActionsVariables
	resp, err := s.client.Do(req, &variables)
	if err != nil {
		return nil, resp, err
	}

	return variables, resp, nil
}

// ListRepoOrgVariables lists all organization variables available in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#list-repository-organization-variables
//
//meta:operation GET /repos/{owner}/{repo}/actions/organization-variables
func (s *ActionsService) ListRepoOrgVariables(ctx context.Context, owner, repo string, opts *ListOptions) (*ActionsVariables, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/organization-variables", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variables *ActionsVariables
	resp, err := s.client.Do(req, &variables)
	if err != nil {
		return nil, resp, err
	}

	return variables, resp, nil
}

// ListOrgVariables lists all variables available in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#list-organization-variables
//
//meta:operation GET /orgs/{org}/actions/variables
func (s *ActionsService) ListOrgVariables(ctx context.Context, org string, opts *ListOptions) (*ActionsVariables, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variables *ActionsVariables
	resp, err := s.client.Do(req, &variables)
	if err != nil {
		return nil, resp, err
	}

	return variables, resp, nil
}

// ListEnvVariables lists all variables available in an environment.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#list-environment-variables
//
//meta:operation GET /repos/{owner}/{repo}/environments/{environment_name}/variables
func (s *ActionsService) ListEnvVariables(ctx context.Context, owner, repo, env string, opts *ListOptions) (*ActionsVariables, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/environments/%v/variables", owner, repo, env)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variables *ActionsVariables
	resp, err := s.client.Do(req, &variables)
	if err != nil {
		return nil, resp, err
	}

	return variables, resp, nil
}

// GetRepoVariable gets a single repository variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#get-a-repository-variable
//
//meta:operation GET /repos/{owner}/{repo}/actions/variables/{name}
func (s *ActionsService) GetRepoVariable(ctx context.Context, owner, repo, name string) (*ActionsVariable, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/variables/%v", owner, repo, name)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variable *ActionsVariable
	resp, err := s.client.Do(req, &variable)
	if err != nil {
		return nil, resp, err
	}

	return variable, resp, nil
}

// GetOrgVariable gets a single organization variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#get-an-organization-variable
//
//meta:operation GET /orgs/{org}/actions/variables/{name}
func (s *ActionsService) GetOrgVariable(ctx context.Context, org, name string) (*ActionsVariable, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables/%v", org, name)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variable *ActionsVariable
	resp, err := s.client.Do(req, &variable)
	if err != nil {
		return nil, resp, err
	}

	return variable, resp, nil
}

// GetEnvVariable gets a single environment variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#get-an-environment-variable
//
//meta:operation GET /repos/{owner}/{repo}/environments/{environment_name}/variables/{name}
func (s *ActionsService) GetEnvVariable(ctx context.Context, owner, repo, env, variableName string) (*ActionsVariable, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/environments/%v/variables/%v", owner, repo, env, variableName)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var variable *ActionsVariable
	resp, err := s.client.Do(req, &variable)
	if err != nil {
		return nil, resp, err
	}

	return variable, resp, nil
}

// CreateRepoVariable creates a repository variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#create-a-repository-variable
//
//meta:operation POST /repos/{owner}/{repo}/actions/variables
func (s *ActionsService) CreateRepoVariable(ctx context.Context, owner, repo string, body ActionsCreateVariableRequest) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/variables", owner, repo)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// CreateOrgVariable creates an organization variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#create-an-organization-variable
//
//meta:operation POST /orgs/{org}/actions/variables
func (s *ActionsService) CreateOrgVariable(ctx context.Context, org string, body ActionsCreateOrgVariableRequest) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables", org)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// CreateEnvVariable creates an environment variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#create-an-environment-variable
//
//meta:operation POST /repos/{owner}/{repo}/environments/{environment_name}/variables
func (s *ActionsService) CreateEnvVariable(ctx context.Context, owner, repo, env string, body ActionsCreateVariableRequest) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/environments/%v/variables", owner, repo, env)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req, nil)
}

// UpdateRepoVariable updates a repository variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#update-a-repository-variable
//
//meta:operation PATCH /repos/{owner}/{repo}/actions/variables/{name}
func (s *ActionsService) UpdateRepoVariable(ctx context.Context, owner, repo, name string, body ActionsUpdateVariableRequest) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/variables/%v", owner, repo, name)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// UpdateOrgVariable updates an organization variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#update-an-organization-variable
//
//meta:operation PATCH /orgs/{org}/actions/variables/{name}
func (s *ActionsService) UpdateOrgVariable(ctx context.Context, org, name string, body ActionsUpdateOrgVariableRequest) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables/%v", org, name)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// UpdateEnvVariable updates an environment variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#update-an-environment-variable
//
//meta:operation PATCH /repos/{owner}/{repo}/environments/{environment_name}/variables/{name}
func (s *ActionsService) UpdateEnvVariable(ctx context.Context, owner, repo, env, name string, body ActionsUpdateVariableRequest) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/environments/%v/variables/%v", owner, repo, env, name)

	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteRepoVariable deletes a variable in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#delete-a-repository-variable
//
//meta:operation DELETE /repos/{owner}/{repo}/actions/variables/{name}
func (s *ActionsService) DeleteRepoVariable(ctx context.Context, owner, repo, name string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/variables/%v", owner, repo, name)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteOrgVariable deletes a variable in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#delete-an-organization-variable
//
//meta:operation DELETE /orgs/{org}/actions/variables/{name}
func (s *ActionsService) DeleteOrgVariable(ctx context.Context, org, name string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables/%v", org, name)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// DeleteEnvVariable deletes a variable in an environment.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#delete-an-environment-variable
//
//meta:operation DELETE /repos/{owner}/{repo}/environments/{environment_name}/variables/{name}
func (s *ActionsService) DeleteEnvVariable(ctx context.Context, owner, repo, env, variableName string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/environments/%v/variables/%v", owner, repo, env, variableName)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListSelectedReposForOrgVariable lists all repositories that have access to a variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#list-selected-repositories-for-an-organization-variable
//
//meta:operation GET /orgs/{org}/actions/variables/{name}/repositories
func (s *ActionsService) ListSelectedReposForOrgVariable(ctx context.Context, org, name string, opts *ListOptions) (*SelectedReposList, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables/%v/repositories", org, name)

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

// SetSelectedReposForOrgVariable sets the repositories that have access to a variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#set-selected-repositories-for-an-organization-variable
//
//meta:operation PUT /orgs/{org}/actions/variables/{name}/repositories
func (s *ActionsService) SetSelectedReposForOrgVariable(ctx context.Context, org, name string, ids []int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/variables/%v/repositories", org, name)

	type repoIDs struct {
		SelectedIDs []int64 `json:"selected_repository_ids"`
	}

	req, err := s.client.NewRequest(ctx, "PUT", u, repoIDs{SelectedIDs: ids})
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// AddSelectedRepoToOrgVariable adds a repository to an organization variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#add-selected-repository-to-an-organization-variable
//
//meta:operation PUT /orgs/{org}/actions/variables/{name}/repositories/{repository_id}
func (s *ActionsService) AddSelectedRepoToOrgVariable(ctx context.Context, org, name string, repo *Repository) (*Response, error) {
	if repo == nil {
		return nil, errors.New("repository must be provided")
	}
	if repo.ID == nil {
		return nil, errors.New("id must be provided")
	}

	u := fmt.Sprintf("orgs/%v/actions/variables/%v/repositories/%v", org, name, *repo.ID)
	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// RemoveSelectedRepoFromOrgVariable removes a repository from an organization variable.
//
// GitHub API docs: https://docs.github.com/rest/actions/variables?apiVersion=2022-11-28#remove-selected-repository-from-an-organization-variable
//
//meta:operation DELETE /orgs/{org}/actions/variables/{name}/repositories/{repository_id}
func (s *ActionsService) RemoveSelectedRepoFromOrgVariable(ctx context.Context, org, name string, repo *Repository) (*Response, error) {
	if repo == nil {
		return nil, errors.New("repository must be provided")
	}
	if repo.ID == nil {
		return nil, errors.New("id must be provided")
	}

	u := fmt.Sprintf("orgs/%v/actions/variables/%v/repositories/%v", org, name, *repo.ID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
