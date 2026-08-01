// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ActionsPermissions represents a policy for repositories and allowed actions in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28
type ActionsPermissions struct {
	EnabledRepositories *string `json:"enabled_repositories,omitempty"`
	AllowedActions      *string `json:"allowed_actions,omitempty"`
	SelectedActionsURL  *string `json:"selected_actions_url,omitempty"`
	SHAPinningRequired  *bool   `json:"sha_pinning_required,omitempty"`
}

func (a ActionsPermissions) String() string {
	return Stringify(a)
}

// ActionsEnabledOnOrgRepos represents all the repositories in an organization for which Actions is enabled.
type ActionsEnabledOnOrgRepos struct {
	TotalCount   int           `json:"total_count"`
	Repositories []*Repository `json:"repositories"`
}

// ActionsAllowed represents selected actions that are allowed.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28
type ActionsAllowed struct {
	GithubOwnedAllowed *bool `json:"github_owned_allowed,omitempty"`
	VerifiedAllowed    *bool `json:"verified_allowed,omitempty"`
	// PatternsAllowed uses `omitzero` to allow an empty array to be sent in the request body in order to
	// configure the organization to allow no 3rd-party actions. It needs to still omit a `nil` value so
	// that it's possible to patch the other values without changing the allowed actions.
	PatternsAllowed []string `json:"patterns_allowed,omitzero"`
}

func (a ActionsAllowed) String() string {
	return Stringify(a)
}

// DefaultWorkflowPermissionOrganization represents the default permissions for GitHub Actions workflows for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28
type DefaultWorkflowPermissionOrganization struct {
	DefaultWorkflowPermissions   *string `json:"default_workflow_permissions,omitempty"`
	CanApprovePullRequestReviews *bool   `json:"can_approve_pull_request_reviews,omitempty"`
}

// SelfHostedRunnersSettingsOrganization represents the self-hosted runners permissions settings for repositories in an organization.
type SelfHostedRunnersSettingsOrganization struct {
	EnabledRepositories     *string `json:"enabled_repositories,omitempty"`
	SelectedRepositoriesURL *string `json:"selected_repositories_url,omitempty"`
}

func (s SelfHostedRunnersSettingsOrganization) String() string {
	return Stringify(s)
}

// SelfHostedRunnersSettingsOrganizationOpt specifies the self-hosted runners permissions settings for repositories in an organization.
type SelfHostedRunnersSettingsOrganizationOpt struct {
	EnabledRepositories *string `json:"enabled_repositories,omitempty"`
}

// GetActionsPermissions gets the GitHub Actions permissions policy for repositories and allowed actions in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-github-actions-permissions-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions
func (s *ActionsService) GetActionsPermissions(ctx context.Context, org string) (*ActionsPermissions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *ActionsPermissions
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateActionsPermissions sets the permissions policy for repositories and allowed actions in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-github-actions-permissions-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions
func (s *ActionsService) UpdateActionsPermissions(ctx context.Context, org string, body ActionsPermissions) (*ActionsPermissions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions", org)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var p *ActionsPermissions
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, nil
}

// ListEnabledReposInOrg lists the selected repositories that are enabled for GitHub Actions in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#list-selected-repositories-enabled-for-github-actions-in-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/repositories
func (s *ActionsService) ListEnabledReposInOrg(ctx context.Context, owner string, opts *ListOptions) (*ActionsEnabledOnOrgRepos, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/repositories", owner)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var repos *ActionsEnabledOnOrgRepos
	resp, err := s.client.Do(req, &repos)
	if err != nil {
		return nil, resp, err
	}

	return repos, resp, nil
}

// SetEnabledReposInOrg replaces the list of selected repositories that are enabled for GitHub Actions in an organization..
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-selected-repositories-enabled-for-github-actions-in-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/repositories
func (s *ActionsService) SetEnabledReposInOrg(ctx context.Context, owner string, repositoryIDs []int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/repositories", owner)

	req, err := s.client.NewRequest(ctx, "PUT", u, struct {
		IDs []int64 `json:"selected_repository_ids"`
	}{IDs: repositoryIDs})
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// AddEnabledReposInOrg adds a repository to the list of selected repositories that are enabled for GitHub Actions in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#enable-a-selected-repository-for-github-actions-in-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/repositories/{repository_id}
func (s *ActionsService) AddEnabledReposInOrg(ctx context.Context, owner string, repositoryID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/repositories/%v", owner, repositoryID)

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

// RemoveEnabledReposInOrg removes a single repository from the list of enabled repos for GitHub Actions in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#disable-a-selected-repository-for-github-actions-in-an-organization
//
//meta:operation DELETE /orgs/{org}/actions/permissions/repositories/{repository_id}
func (s *ActionsService) RemoveEnabledReposInOrg(ctx context.Context, owner string, repositoryID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/repositories/%v", owner, repositoryID)

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

// GetActionsAllowed gets the actions that are allowed in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-allowed-actions-and-reusable-workflows-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/selected-actions
func (s *ActionsService) GetActionsAllowed(ctx context.Context, org string) (*ActionsAllowed, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/selected-actions", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var actionsAllowed *ActionsAllowed
	resp, err := s.client.Do(req, &actionsAllowed)
	if err != nil {
		return nil, resp, err
	}

	return actionsAllowed, resp, nil
}

// UpdateActionsAllowed sets the actions that are allowed in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-allowed-actions-and-reusable-workflows-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/selected-actions
func (s *ActionsService) UpdateActionsAllowed(ctx context.Context, org string, body ActionsAllowed) (*ActionsAllowed, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/selected-actions", org)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var p *ActionsAllowed
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, nil
}

// GetDefaultWorkflowPermissionsInOrganization gets the GitHub Actions default workflow permissions for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-default-workflow-permissions-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/workflow
func (s *ActionsService) GetDefaultWorkflowPermissionsInOrganization(ctx context.Context, org string) (*DefaultWorkflowPermissionOrganization, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/workflow", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *DefaultWorkflowPermissionOrganization
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateDefaultWorkflowPermissionsInOrganization sets the GitHub Actions default workflow permissions for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-default-workflow-permissions-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/workflow
func (s *ActionsService) UpdateDefaultWorkflowPermissionsInOrganization(ctx context.Context, org string, body DefaultWorkflowPermissionOrganization) (*DefaultWorkflowPermissionOrganization, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/workflow", org)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var p *DefaultWorkflowPermissionOrganization
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, nil
}

// GetArtifactAndLogRetentionPeriodInOrganization gets the artifact and log retention period for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-artifact-and-log-retention-settings-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/artifact-and-log-retention
func (s *ActionsService) GetArtifactAndLogRetentionPeriodInOrganization(ctx context.Context, org string) (*ArtifactPeriod, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/artifact-and-log-retention", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var arp *ArtifactPeriod
	resp, err := s.client.Do(req, &arp)
	if err != nil {
		return nil, resp, err
	}

	return arp, resp, nil
}

// UpdateArtifactAndLogRetentionPeriodInOrganization sets the artifact and log retention period for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-artifact-and-log-retention-settings-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/artifact-and-log-retention
func (s *ActionsService) UpdateArtifactAndLogRetentionPeriodInOrganization(ctx context.Context, org string, body ArtifactPeriodOpt) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/artifact-and-log-retention", org)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetSelfHostedRunnersSettingsInOrganization gets the self-hosted runners permissions settings for repositories in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-self-hosted-runners-settings-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/self-hosted-runners
func (s *ActionsService) GetSelfHostedRunnersSettingsInOrganization(ctx context.Context, org string) (*SelfHostedRunnersSettingsOrganization, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/self-hosted-runners", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var settings *SelfHostedRunnersSettingsOrganization
	resp, err := s.client.Do(req, &settings)
	if err != nil {
		return nil, resp, err
	}

	return settings, resp, nil
}

// UpdateSelfHostedRunnersSettingsInOrganization sets the self-hosted runners permissions settings for repositories in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-self-hosted-runners-settings-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/self-hosted-runners
func (s *ActionsService) UpdateSelfHostedRunnersSettingsInOrganization(ctx context.Context, org string, body SelfHostedRunnersSettingsOrganizationOpt) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/self-hosted-runners", org)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// SelfHostedRunnersAllowedRepos represents the repositories that are allowed to use self-hosted runners in an organization.
type SelfHostedRunnersAllowedRepos struct {
	TotalCount   int           `json:"total_count"`
	Repositories []*Repository `json:"repositories"`
}

// ListRepositoriesSelfHostedRunnersAllowedInOrganization lists the repositories that are allowed to use self-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#list-repositories-allowed-to-use-self-hosted-runners-in-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/self-hosted-runners/repositories
func (s *ActionsService) ListRepositoriesSelfHostedRunnersAllowedInOrganization(ctx context.Context, org string, opts *ListOptions) (*SelfHostedRunnersAllowedRepos, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/self-hosted-runners/repositories", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var settings *SelfHostedRunnersAllowedRepos
	resp, err := s.client.Do(req, &settings)
	if err != nil {
		return nil, resp, err
	}

	return settings, resp, nil
}

// SetRepositoriesSelfHostedRunnersAllowedInOrganization allows the list of repositories to use self-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-repositories-allowed-to-use-self-hosted-runners-in-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/self-hosted-runners/repositories
func (s *ActionsService) SetRepositoriesSelfHostedRunnersAllowedInOrganization(ctx context.Context, org string, repositoryIDs []int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/self-hosted-runners/repositories", org)

	req, err := s.client.NewRequest(ctx, "PUT", u, struct {
		IDs []int64 `json:"selected_repository_ids"`
	}{IDs: repositoryIDs})
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// AddRepositorySelfHostedRunnersAllowedInOrganization adds a repository to the list of repositories that are allowed to use self-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#add-a-repository-to-the-list-of-repositories-allowed-to-use-self-hosted-runners-in-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/self-hosted-runners/repositories/{repository_id}
func (s *ActionsService) AddRepositorySelfHostedRunnersAllowedInOrganization(ctx context.Context, org string, repositoryID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/self-hosted-runners/repositories/%v", org, repositoryID)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// RemoveRepositorySelfHostedRunnersAllowedInOrganization removes a repository from the list of repositories that are allowed to use self-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#remove-a-repository-from-the-list-of-repositories-allowed-to-use-self-hosted-runners-in-an-organization
//
//meta:operation DELETE /orgs/{org}/actions/permissions/self-hosted-runners/repositories/{repository_id}
func (s *ActionsService) RemoveRepositorySelfHostedRunnersAllowedInOrganization(ctx context.Context, org string, repositoryID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/self-hosted-runners/repositories/%v", org, repositoryID)

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

// GetPrivateRepoForkPRWorkflowSettingsInOrganization gets the settings for whether workflows from fork pull requests can run on private repositories in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-private-repo-fork-pr-workflow-settings-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/fork-pr-workflows-private-repos
func (s *ActionsService) GetPrivateRepoForkPRWorkflowSettingsInOrganization(ctx context.Context, org string) (*WorkflowsPermissions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/fork-pr-workflows-private-repos", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *WorkflowsPermissions
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdatePrivateRepoForkPRWorkflowSettingsInOrganization sets the settings for whether workflows from fork pull requests can run on private repositories in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-private-repo-fork-pr-workflow-settings-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/fork-pr-workflows-private-repos
func (s *ActionsService) UpdatePrivateRepoForkPRWorkflowSettingsInOrganization(ctx context.Context, org string, body *WorkflowsPermissionsOpt) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/fork-pr-workflows-private-repos", org)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetOrganizationForkPRContributorApprovalPermissions gets the fork PR contributor approval policy for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-fork-pr-contributor-approval-permissions-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/permissions/fork-pr-contributor-approval
func (s *ActionsService) GetOrganizationForkPRContributorApprovalPermissions(ctx context.Context, org string) (*ContributorApprovalPermissions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/fork-pr-contributor-approval", org)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var policy *ContributorApprovalPermissions
	resp, err := s.client.Do(req, &policy)
	if err != nil {
		return nil, resp, err
	}

	return policy, resp, nil
}

// UpdateOrganizationForkPRContributorApprovalPermissions sets the fork PR contributor approval policy for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-fork-pr-contributor-approval-permissions-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/permissions/fork-pr-contributor-approval
func (s *ActionsService) UpdateOrganizationForkPRContributorApprovalPermissions(ctx context.Context, org string, body ContributorApprovalPermissions) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/permissions/fork-pr-contributor-approval", org)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
