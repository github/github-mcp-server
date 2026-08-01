// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ActionsEnabledOnEnterpriseRepos represents all the repositories in an enterprise for which Actions is enabled.
type ActionsEnabledOnEnterpriseRepos struct {
	TotalCount    int             `json:"total_count"`
	Organizations []*Organization `json:"organizations"`
}

// ActionsPermissionsEnterprise represents a policy for allowed actions in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28
type ActionsPermissionsEnterprise struct {
	EnabledOrganizations *string `json:"enabled_organizations,omitempty"`
	AllowedActions       *string `json:"allowed_actions,omitempty"`
	SelectedActionsURL   *string `json:"selected_actions_url,omitempty"`
}

func (a ActionsPermissionsEnterprise) String() string {
	return Stringify(a)
}

// DefaultWorkflowPermissionEnterprise represents the default permissions for GitHub Actions workflows for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28
type DefaultWorkflowPermissionEnterprise struct {
	DefaultWorkflowPermissions   *string `json:"default_workflow_permissions,omitempty"`
	CanApprovePullRequestReviews *bool   `json:"can_approve_pull_request_reviews,omitempty"`
}

// SelfHostRunnerPermissionsEnterprise represents the settings for whether organizations in the enterprise are allowed to manage self-hosted runners at the repository level.
type SelfHostRunnerPermissionsEnterprise struct {
	DisableSelfHostedRunnersForAllOrgs *bool `json:"disable_self_hosted_runners_for_all_orgs,omitempty"`
}

func (a SelfHostRunnerPermissionsEnterprise) String() string {
	return Stringify(a)
}

// GetActionsPermissionsInEnterprise gets the GitHub Actions permissions policy for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-github-actions-permissions-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions
func (s *ActionsService) GetActionsPermissionsInEnterprise(ctx context.Context, enterprise string) (*ActionsPermissionsEnterprise, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *ActionsPermissionsEnterprise
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateActionsPermissionsInEnterprise sets the permissions policy in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-github-actions-permissions-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions
func (s *ActionsService) UpdateActionsPermissionsInEnterprise(ctx context.Context, enterprise string, body ActionsPermissionsEnterprise) (*ActionsPermissionsEnterprise, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions", enterprise)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var p *ActionsPermissionsEnterprise
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, nil
}

// ListEnabledOrgsInEnterprise lists the selected organizations that are enabled for GitHub Actions in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#list-selected-organizations-enabled-for-github-actions-in-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/organizations
func (s *ActionsService) ListEnabledOrgsInEnterprise(ctx context.Context, owner string, opts *ListOptions) (*ActionsEnabledOnEnterpriseRepos, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/organizations", owner)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var orgs *ActionsEnabledOnEnterpriseRepos
	resp, err := s.client.Do(req, &orgs)
	if err != nil {
		return nil, resp, err
	}

	return orgs, resp, nil
}

// SetEnabledOrgsInEnterprise replaces the list of selected organizations that are enabled for GitHub Actions in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-selected-organizations-enabled-for-github-actions-in-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/organizations
func (s *ActionsService) SetEnabledOrgsInEnterprise(ctx context.Context, owner string, organizationIDs []int64) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/organizations", owner)

	req, err := s.client.NewRequest(ctx, "PUT", u, struct {
		IDs []int64 `json:"selected_organization_ids"`
	}{IDs: organizationIDs})
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// AddEnabledOrgInEnterprise adds an organization to the list of selected organizations that are enabled for GitHub Actions in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#enable-a-selected-organization-for-github-actions-in-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/organizations/{org_id}
func (s *ActionsService) AddEnabledOrgInEnterprise(ctx context.Context, owner string, organizationID int64) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/organizations/%v", owner, organizationID)

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

// RemoveEnabledOrgInEnterprise removes an organization from the list of selected organizations that are enabled for GitHub Actions in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#disable-a-selected-organization-for-github-actions-in-an-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/actions/permissions/organizations/{org_id}
func (s *ActionsService) RemoveEnabledOrgInEnterprise(ctx context.Context, owner string, organizationID int64) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/organizations/%v", owner, organizationID)

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

// GetActionsAllowedInEnterprise gets the actions that are allowed in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-allowed-actions-and-reusable-workflows-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/selected-actions
func (s *ActionsService) GetActionsAllowedInEnterprise(ctx context.Context, enterprise string) (*ActionsAllowed, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/selected-actions", enterprise)

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

// UpdateActionsAllowedInEnterprise sets the actions that are allowed in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-allowed-actions-and-reusable-workflows-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/selected-actions
func (s *ActionsService) UpdateActionsAllowedInEnterprise(ctx context.Context, enterprise string, body ActionsAllowed) (*ActionsAllowed, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/selected-actions", enterprise)
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

// GetDefaultWorkflowPermissionsInEnterprise gets the GitHub Actions default workflow permissions for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-default-workflow-permissions-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/workflow
func (s *ActionsService) GetDefaultWorkflowPermissionsInEnterprise(ctx context.Context, enterprise string) (*DefaultWorkflowPermissionEnterprise, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/workflow", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *DefaultWorkflowPermissionEnterprise
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateDefaultWorkflowPermissionsInEnterprise sets the GitHub Actions default workflow permissions for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-default-workflow-permissions-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/workflow
func (s *ActionsService) UpdateDefaultWorkflowPermissionsInEnterprise(ctx context.Context, enterprise string, body DefaultWorkflowPermissionEnterprise) (*DefaultWorkflowPermissionEnterprise, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/workflow", enterprise)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var p *DefaultWorkflowPermissionEnterprise
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, nil
}

// GetArtifactAndLogRetentionPeriodInEnterprise gets the artifact and log retention period for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-artifact-and-log-retention-settings-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/artifact-and-log-retention
func (s *ActionsService) GetArtifactAndLogRetentionPeriodInEnterprise(ctx context.Context, enterprise string) (*ArtifactPeriod, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/artifact-and-log-retention", enterprise)

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

// UpdateArtifactAndLogRetentionPeriodInEnterprise sets the artifact and log retention period for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-artifact-and-log-retention-settings-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/artifact-and-log-retention
func (s *ActionsService) UpdateArtifactAndLogRetentionPeriodInEnterprise(ctx context.Context, enterprise string, body ArtifactPeriodOpt) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/artifact-and-log-retention", enterprise)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetSelfHostedRunnerPermissionsInEnterprise gets the self-hosted runner permissions for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-self-hosted-runners-permissions-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/self-hosted-runners
func (s *ActionsService) GetSelfHostedRunnerPermissionsInEnterprise(ctx context.Context, enterprise string) (*SelfHostRunnerPermissionsEnterprise, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/self-hosted-runners", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *SelfHostRunnerPermissionsEnterprise
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateSelfHostedRunnerPermissionsInEnterprise sets the self-hosted runner permissions for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-self-hosted-runners-permissions-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/self-hosted-runners
func (s *ActionsService) UpdateSelfHostedRunnerPermissionsInEnterprise(ctx context.Context, enterprise string, body SelfHostRunnerPermissionsEnterprise) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/self-hosted-runners", enterprise)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetPrivateRepoForkPRWorkflowSettingsInEnterprise gets the settings for whether workflows from fork pull requests can run on private repositories in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-private-repo-fork-pr-workflow-settings-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/fork-pr-workflows-private-repos
func (s *ActionsService) GetPrivateRepoForkPRWorkflowSettingsInEnterprise(ctx context.Context, enterprise string) (*WorkflowsPermissions, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/fork-pr-workflows-private-repos", enterprise)

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

// UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise sets the settings for whether workflows from fork pull requests can run on private repositories in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-private-repo-fork-pr-workflow-settings-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/fork-pr-workflows-private-repos
func (s *ActionsService) UpdatePrivateRepoForkPRWorkflowSettingsInEnterprise(ctx context.Context, enterprise string, body *WorkflowsPermissionsOpt) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/fork-pr-workflows-private-repos", enterprise)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetEnterpriseForkPRContributorApprovalPermissions gets the fork PR contributor approval policy for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#get-fork-pr-contributor-approval-permissions-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/permissions/fork-pr-contributor-approval
func (s *ActionsService) GetEnterpriseForkPRContributorApprovalPermissions(ctx context.Context, enterprise string) (*ContributorApprovalPermissions, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/fork-pr-contributor-approval", enterprise)

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

// UpdateEnterpriseForkPRContributorApprovalPermissions sets the fork PR contributor approval policy for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/permissions?apiVersion=2022-11-28#set-fork-pr-contributor-approval-permissions-for-an-enterprise
//
//meta:operation PUT /enterprises/{enterprise}/actions/permissions/fork-pr-contributor-approval
func (s *ActionsService) UpdateEnterpriseForkPRContributorApprovalPermissions(ctx context.Context, enterprise string, body ContributorApprovalPermissions) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/permissions/fork-pr-contributor-approval", enterprise)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
