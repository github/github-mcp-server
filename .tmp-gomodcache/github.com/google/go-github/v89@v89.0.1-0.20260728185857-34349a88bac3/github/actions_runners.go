// Copyright 2020 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// RunnerApplicationDownload represents a binary for the self-hosted runner application that can be downloaded.
type RunnerApplicationDownload struct {
	OS                *string `json:"os,omitempty"`
	Architecture      *string `json:"architecture,omitempty"`
	DownloadURL       *string `json:"download_url,omitempty"`
	Filename          *string `json:"filename,omitempty"`
	TempDownloadToken *string `json:"temp_download_token,omitempty"`
	SHA256Checksum    *string `json:"sha256_checksum,omitempty"`
}

// ListRunnerApplicationDownloads lists self-hosted runner application binaries that can be downloaded and run.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#list-runner-applications-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/runners/downloads
func (s *ActionsService) ListRunnerApplicationDownloads(ctx context.Context, owner, repo string) ([]*RunnerApplicationDownload, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners/downloads", owner, repo)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var rads []*RunnerApplicationDownload
	resp, err := s.client.Do(req, &rads)
	if err != nil {
		return nil, resp, err
	}

	return rads, resp, nil
}

// CreateJITConfigRequest specifies body parameters to CreateOrgJITConfig, CreateRepoJITConfig, and CreateJITConfig.
type CreateJITConfigRequest struct {
	Name          string  `json:"name"`
	RunnerGroupID int64   `json:"runner_group_id"`
	WorkFolder    *string `json:"work_folder,omitempty"`

	// Labels represents the names of the custom labels to add to the runner.
	// Minimum items: 1. Maximum items: 100.
	Labels []string `json:"labels"`
}

// JITRunnerConfig represents encoded JIT configuration that can be used to bootstrap a self-hosted runner.
type JITRunnerConfig struct {
	Runner           *Runner `json:"runner,omitempty"`
	EncodedJITConfig *string `json:"encoded_jit_config,omitempty"`
}

// CreateOrgJITConfig creates a just-in-time configuration for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-configuration-for-a-just-in-time-runner-for-an-organization
//
//meta:operation POST /orgs/{org}/actions/runners/generate-jitconfig
func (s *ActionsService) CreateOrgJITConfig(ctx context.Context, org string, body CreateJITConfigRequest) (*JITRunnerConfig, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners/generate-jitconfig", org)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var jitConfig *JITRunnerConfig
	resp, err := s.client.Do(req, &jitConfig)
	if err != nil {
		return nil, resp, err
	}

	return jitConfig, resp, nil
}

// CreateRepoJITConfig creates a just-in-time configuration for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-configuration-for-a-just-in-time-runner-for-a-repository
//
//meta:operation POST /repos/{owner}/{repo}/actions/runners/generate-jitconfig
func (s *ActionsService) CreateRepoJITConfig(ctx context.Context, owner, repo string, body CreateJITConfigRequest) (*JITRunnerConfig, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners/generate-jitconfig", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var jitConfig *JITRunnerConfig
	resp, err := s.client.Do(req, &jitConfig)
	if err != nil {
		return nil, resp, err
	}

	return jitConfig, resp, nil
}

// RegistrationToken represents a token that can be used to add a self-hosted runner to a repository.
type RegistrationToken struct {
	Token     *string    `json:"token,omitempty"`
	ExpiresAt *Timestamp `json:"expires_at,omitempty"`
}

// CreateRegistrationToken creates a token that can be used to add a self-hosted runner.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-a-repository
//
//meta:operation POST /repos/{owner}/{repo}/actions/runners/registration-token
func (s *ActionsService) CreateRegistrationToken(ctx context.Context, owner, repo string) (*RegistrationToken, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners/registration-token", owner, repo)

	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var registrationToken *RegistrationToken
	resp, err := s.client.Do(req, &registrationToken)
	if err != nil {
		return nil, resp, err
	}

	return registrationToken, resp, nil
}

// Runner represents a self-hosted runner registered with a repository.
type Runner struct {
	ID            *int64          `json:"id,omitempty"`
	RunnerGroupID *int64          `json:"runner_group_id,omitempty"`
	Name          *string         `json:"name,omitempty"`
	OS            *string         `json:"os,omitempty"`
	Status        *string         `json:"status,omitempty"`
	Busy          *bool           `json:"busy,omitempty"`
	Labels        []*RunnerLabels `json:"labels,omitempty"`
	Ephemeral     *bool           `json:"ephemeral,omitempty"`
	Version       *string         `json:"version,omitempty"`
}

// RunnerLabels represents a collection of labels attached to each runner.
type RunnerLabels struct {
	ID   *int64  `json:"id,omitempty"`
	Name *string `json:"name,omitempty"`
	Type *string `json:"type,omitempty"`
}

// Runners represents a collection of self-hosted runners for a repository.
type Runners struct {
	TotalCount int       `json:"total_count"`
	Runners    []*Runner `json:"runners"`
}

// ListRunnersOptions specifies the optional parameters to the ListRunners and ListOrganizationRunners methods.
type ListRunnersOptions struct {
	Name *string `url:"name,omitempty"`
	ListOptions
}

// ListRunners lists all the self-hosted runners for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#list-self-hosted-runners-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/runners
func (s *ActionsService) ListRunners(ctx context.Context, owner, repo string, opts *ListRunnersOptions) (*Runners, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var runners *Runners
	resp, err := s.client.Do(req, &runners)
	if err != nil {
		return nil, resp, err
	}

	return runners, resp, nil
}

// GetRunner gets a specific self-hosted runner for a repository using its runner ID.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#get-a-self-hosted-runner-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/runners/{runner_id}
func (s *ActionsService) GetRunner(ctx context.Context, owner, repo string, runnerID int64) (*Runner, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners/%v", owner, repo, runnerID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var runner *Runner
	resp, err := s.client.Do(req, &runner)
	if err != nil {
		return nil, resp, err
	}

	return runner, resp, nil
}

// RemoveToken represents a token that can be used to remove a self-hosted runner from a repository.
type RemoveToken struct {
	Token     *string    `json:"token,omitempty"`
	ExpiresAt *Timestamp `json:"expires_at,omitempty"`
}

// CreateRemoveToken creates a token that can be used to remove a self-hosted runner from a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-remove-token-for-a-repository
//
//meta:operation POST /repos/{owner}/{repo}/actions/runners/remove-token
func (s *ActionsService) CreateRemoveToken(ctx context.Context, owner, repo string) (*RemoveToken, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners/remove-token", owner, repo)

	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var removeToken *RemoveToken
	resp, err := s.client.Do(req, &removeToken)
	if err != nil {
		return nil, resp, err
	}

	return removeToken, resp, nil
}

// RemoveRunner forces the removal of a self-hosted runner in a repository using the runner id.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#delete-a-self-hosted-runner-from-a-repository
//
//meta:operation DELETE /repos/{owner}/{repo}/actions/runners/{runner_id}
func (s *ActionsService) RemoveRunner(ctx context.Context, owner, repo string, runnerID int64) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/runners/%v", owner, repo, runnerID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListOrganizationRunnerApplicationDownloads lists self-hosted runner application binaries that can be downloaded and run.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#list-runner-applications-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/runners/downloads
func (s *ActionsService) ListOrganizationRunnerApplicationDownloads(ctx context.Context, org string) ([]*RunnerApplicationDownload, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners/downloads", org)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var rads []*RunnerApplicationDownload
	resp, err := s.client.Do(req, &rads)
	if err != nil {
		return nil, resp, err
	}

	return rads, resp, nil
}

// CreateOrganizationRegistrationToken creates a token that can be used to add a self-hosted runner to an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-organization
//
//meta:operation POST /orgs/{org}/actions/runners/registration-token
func (s *ActionsService) CreateOrganizationRegistrationToken(ctx context.Context, org string) (*RegistrationToken, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners/registration-token", org)

	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var registrationToken *RegistrationToken
	resp, err := s.client.Do(req, &registrationToken)
	if err != nil {
		return nil, resp, err
	}

	return registrationToken, resp, nil
}

// ListOrganizationRunners lists all the self-hosted runners for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#list-self-hosted-runners-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/runners
func (s *ActionsService) ListOrganizationRunners(ctx context.Context, org string, opts *ListRunnersOptions) (*Runners, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var runners *Runners
	resp, err := s.client.Do(req, &runners)
	if err != nil {
		return nil, resp, err
	}

	return runners, resp, nil
}

// GetOrganizationRunner gets a specific self-hosted runner for an organization using its runner ID.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#get-a-self-hosted-runner-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/runners/{runner_id}
func (s *ActionsService) GetOrganizationRunner(ctx context.Context, org string, runnerID int64) (*Runner, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners/%v", org, runnerID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var runner *Runner
	resp, err := s.client.Do(req, &runner)
	if err != nil {
		return nil, resp, err
	}

	return runner, resp, nil
}

// CreateOrganizationRemoveToken creates a token that can be used to remove a self-hosted runner from an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-remove-token-for-an-organization
//
//meta:operation POST /orgs/{org}/actions/runners/remove-token
func (s *ActionsService) CreateOrganizationRemoveToken(ctx context.Context, org string) (*RemoveToken, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners/remove-token", org)

	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var removeToken *RemoveToken
	resp, err := s.client.Do(req, &removeToken)
	if err != nil {
		return nil, resp, err
	}

	return removeToken, resp, nil
}

// RemoveOrganizationRunner forces the removal of a self-hosted runner from an organization using the runner id.
//
// GitHub API docs: https://docs.github.com/rest/actions/self-hosted-runners?apiVersion=2022-11-28#delete-a-self-hosted-runner-from-an-organization
//
//meta:operation DELETE /orgs/{org}/actions/runners/{runner_id}
func (s *ActionsService) RemoveOrganizationRunner(ctx context.Context, org string, runnerID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/runners/%v", org, runnerID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
