// Copyright 2020 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ListRunnerApplicationDownloads lists self-hosted runner application binaries that can be downloaded and run.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/self-hosted-runners?apiVersion=2022-11-28#list-runner-applications-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/runners/downloads
func (s *EnterpriseService) ListRunnerApplicationDownloads(ctx context.Context, enterprise string) ([]*RunnerApplicationDownload, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/runners/downloads", enterprise)
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

// CreateJITConfig creates a just-in-time configuration for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-configuration-for-a-just-in-time-runner-for-an-enterprise
//
//meta:operation POST /enterprises/{enterprise}/actions/runners/generate-jitconfig
func (s *EnterpriseService) CreateJITConfig(ctx context.Context, enterprise string, body CreateJITConfigRequest) (*JITRunnerConfig, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/runners/generate-jitconfig", enterprise)

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

// CreateRegistrationToken creates a token that can be used to add a self-hosted runner.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/self-hosted-runners?apiVersion=2022-11-28#create-a-registration-token-for-an-enterprise
//
//meta:operation POST /enterprises/{enterprise}/actions/runners/registration-token
func (s *EnterpriseService) CreateRegistrationToken(ctx context.Context, enterprise string) (*RegistrationToken, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/runners/registration-token", enterprise)

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

// ListRunners lists all the self-hosted runners for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/self-hosted-runners?apiVersion=2022-11-28#list-self-hosted-runners-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/runners
func (s *EnterpriseService) ListRunners(ctx context.Context, enterprise string, opts *ListRunnersOptions) (*Runners, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/runners", enterprise)
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

// GetRunner gets a specific self-hosted runner configured in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/self-hosted-runners?apiVersion=2022-11-28#get-a-self-hosted-runner-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/runners/{runner_id}
func (s *EnterpriseService) GetRunner(ctx context.Context, enterprise string, runnerID int64) (*Runner, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/runners/%v", enterprise, runnerID)
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

// RemoveRunner forces the removal of a self-hosted runner from an enterprise using the runner id.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/self-hosted-runners?apiVersion=2022-11-28#delete-a-self-hosted-runner-from-an-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/actions/runners/{runner_id}
func (s *EnterpriseService) RemoveRunner(ctx context.Context, enterprise string, runnerID int64) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/runners/%v", enterprise, runnerID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
