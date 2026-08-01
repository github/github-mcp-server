// Copyright 2022 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ActionsPermissionsRepository represents a policy for repositories and allowed actions in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28
type ActionsPermissionsRepository struct {
	Enabled            *bool   `json:"enabled,omitempty"`
	AllowedActions     *string `json:"allowed_actions,omitempty"`
	SelectedActionsURL *string `json:"selected_actions_url,omitempty"`
	SHAPinningRequired *bool   `json:"sha_pinning_required,omitempty"`
}

func (a ActionsPermissionsRepository) String() string {
	return Stringify(a)
}

// DefaultWorkflowPermissionRepository represents the default permissions for GitHub Actions workflows for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28
type DefaultWorkflowPermissionRepository struct {
	DefaultWorkflowPermissions   *string `json:"default_workflow_permissions,omitempty"`
	CanApprovePullRequestReviews *bool   `json:"can_approve_pull_request_reviews,omitempty"`
}

// GetActionsPermissions gets the GitHub Actions permissions policy for repositories and allowed actions in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-github-actions-permissions-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/permissions
func (s *RepositoriesService) GetActionsPermissions(ctx context.Context, owner, repo string) (*ActionsPermissionsRepository, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *ActionsPermissionsRepository
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateActionsPermissions sets the permissions policy for repositories and allowed actions in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-github-actions-permissions-for-a-repository
//
//meta:operation PUT /repos/{owner}/{repo}/actions/permissions
func (s *RepositoriesService) UpdateActionsPermissions(ctx context.Context, owner, repo string, body ActionsPermissionsRepository) (*ActionsPermissionsRepository, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions", owner, repo)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var permissions *ActionsPermissionsRepository
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// GetDefaultWorkflowPermissions gets the GitHub Actions default workflow permissions in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-default-workflow-permissions-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/permissions/workflow
func (s *RepositoriesService) GetDefaultWorkflowPermissions(ctx context.Context, owner, repo string) (*DefaultWorkflowPermissionRepository, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/workflow", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var permissions *DefaultWorkflowPermissionRepository
	resp, err := s.client.Do(req, &permissions)
	if err != nil {
		return nil, resp, err
	}

	return permissions, resp, nil
}

// UpdateDefaultWorkflowPermissions sets the GitHub Actions default workflow permissions in a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-default-workflow-permissions-for-a-repository
//
//meta:operation PUT /repos/{owner}/{repo}/actions/permissions/workflow
func (s *RepositoriesService) UpdateDefaultWorkflowPermissions(ctx context.Context, owner, repo string, body DefaultWorkflowPermissionRepository) (*DefaultWorkflowPermissionRepository, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/workflow", owner, repo)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var p *DefaultWorkflowPermissionRepository
	resp, err := s.client.Do(req, &p)
	if err != nil {
		return nil, resp, err
	}

	return p, resp, nil
}

// GetArtifactAndLogRetentionPeriod gets the artifact and log retention period for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-artifact-and-log-retention-settings-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/permissions/artifact-and-log-retention
func (s *RepositoriesService) GetArtifactAndLogRetentionPeriod(ctx context.Context, owner, repo string) (*ArtifactPeriod, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/artifact-and-log-retention", owner, repo)

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

// UpdateArtifactAndLogRetentionPeriod sets the artifact and log retention period for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-artifact-and-log-retention-settings-for-a-repository
//
//meta:operation PUT /repos/{owner}/{repo}/actions/permissions/artifact-and-log-retention
func (s *RepositoriesService) UpdateArtifactAndLogRetentionPeriod(ctx context.Context, owner, repo string, body ArtifactPeriodOpt) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/artifact-and-log-retention", owner, repo)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetPrivateRepoForkPRWorkflowSettings gets the settings for whether workflows from fork pull requests can run on a private repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-private-repo-fork-pr-workflow-settings-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/permissions/fork-pr-workflows-private-repos
func (s *RepositoriesService) GetPrivateRepoForkPRWorkflowSettings(ctx context.Context, owner, repo string) (*WorkflowsPermissions, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/fork-pr-workflows-private-repos", owner, repo)

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

// UpdatePrivateRepoForkPRWorkflowSettings sets the settings for whether workflows from fork pull requests can run on a private repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-private-repo-fork-pr-workflow-settings-for-a-repository
//
//meta:operation PUT /repos/{owner}/{repo}/actions/permissions/fork-pr-workflows-private-repos
func (s *RepositoriesService) UpdatePrivateRepoForkPRWorkflowSettings(ctx context.Context, owner, repo string, body *WorkflowsPermissionsOpt) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/fork-pr-workflows-private-repos", owner, repo)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// GetForkPRContributorApprovalPermissions gets the fork PR contributor approval policy for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#get-fork-pr-contributor-approval-permissions-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/permissions/fork-pr-contributor-approval
func (s *ActionsService) GetForkPRContributorApprovalPermissions(ctx context.Context, owner, repo string) (*ContributorApprovalPermissions, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/fork-pr-contributor-approval", owner, repo)

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

// UpdateForkPRContributorApprovalPermissions sets the fork PR contributor approval policy for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/permissions?apiVersion=2022-11-28#set-fork-pr-contributor-approval-permissions-for-a-repository
//
//meta:operation PUT /repos/{owner}/{repo}/actions/permissions/fork-pr-contributor-approval
func (s *ActionsService) UpdateForkPRContributorApprovalPermissions(ctx context.Context, owner, repo string, body ContributorApprovalPermissions) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/permissions/fork-pr-contributor-approval", owner, repo)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
