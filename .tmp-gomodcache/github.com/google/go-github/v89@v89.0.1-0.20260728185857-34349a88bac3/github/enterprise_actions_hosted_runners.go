// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ListHostedRunners lists all the GitHub-hosted runners for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#list-github-hosted-runners-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners
func (s *EnterpriseService) ListHostedRunners(ctx context.Context, enterprise string, opts *ListOptions) (*HostedRunners, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners", enterprise)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var runners *HostedRunners
	resp, err := s.client.Do(req, &runners)
	if err != nil {
		return nil, resp, err
	}

	return runners, resp, nil
}

// CreateHostedRunner creates a GitHub-hosted runner for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#create-a-github-hosted-runner-for-an-enterprise
//
//meta:operation POST /enterprises/{enterprise}/actions/hosted-runners
func (s *EnterpriseService) CreateHostedRunner(ctx context.Context, enterprise string, body CreateHostedRunnerRequest) (*HostedRunner, *Response, error) {
	if err := validateCreateHostedRunnerRequest(&body); err != nil {
		return nil, nil, fmt.Errorf("validation failed: %w", err)
	}

	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners", enterprise)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var hostedRunner *HostedRunner
	resp, err := s.client.Do(req, &hostedRunner)
	if err != nil {
		return nil, resp, err
	}

	return hostedRunner, resp, nil
}

// GetHostedRunnerGitHubOwnedImages gets the list of GitHub-owned images available for GitHub-hosted runners for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-github-owned-images-for-github-hosted-runners-in-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/images/github-owned
func (s *EnterpriseService) GetHostedRunnerGitHubOwnedImages(ctx context.Context, enterprise string) (*HostedRunnerImages, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/github-owned", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hostedRunnerImages *HostedRunnerImages
	resp, err := s.client.Do(req, &hostedRunnerImages)
	if err != nil {
		return nil, resp, err
	}

	return hostedRunnerImages, resp, nil
}

// GetHostedRunnerPartnerImages gets the list of partner images available for GitHub-hosted runners for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-partner-images-for-github-hosted-runners-in-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/images/partner
func (s *EnterpriseService) GetHostedRunnerPartnerImages(ctx context.Context, enterprise string) (*HostedRunnerImages, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/partner", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hostedRunnerImages *HostedRunnerImages
	resp, err := s.client.Do(req, &hostedRunnerImages)
	if err != nil {
		return nil, resp, err
	}

	return hostedRunnerImages, resp, nil
}

// GetHostedRunnerLimits gets the GitHub-hosted runners Static public IP Limits for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-limits-on-github-hosted-runners-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/limits
func (s *EnterpriseService) GetHostedRunnerLimits(ctx context.Context, enterprise string) (*HostedRunnerPublicIPLimits, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/limits", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var publicIPLimits *HostedRunnerPublicIPLimits
	resp, err := s.client.Do(req, &publicIPLimits)
	if err != nil {
		return nil, resp, err
	}

	return publicIPLimits, resp, nil
}

// GetHostedRunnerMachineSpecs gets the list of machine specs available for GitHub-hosted runners for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-github-hosted-runners-machine-specs-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/machine-sizes
func (s *EnterpriseService) GetHostedRunnerMachineSpecs(ctx context.Context, enterprise string) (*HostedRunnerMachineSpecs, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/machine-sizes", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var machineSpecs *HostedRunnerMachineSpecs
	resp, err := s.client.Do(req, &machineSpecs)
	if err != nil {
		return nil, resp, err
	}

	return machineSpecs, resp, nil
}

// GetHostedRunnerPlatforms gets list of platforms available for GitHub-hosted runners for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-platforms-for-github-hosted-runners-in-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/platforms
func (s *EnterpriseService) GetHostedRunnerPlatforms(ctx context.Context, enterprise string) (*HostedRunnerPlatforms, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/platforms", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var platforms *HostedRunnerPlatforms
	resp, err := s.client.Do(req, &platforms)
	if err != nil {
		return nil, resp, err
	}

	return platforms, resp, nil
}

// GetHostedRunner gets a GitHub-hosted runner in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-a-github-hosted-runner-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/{hosted_runner_id}
func (s *EnterpriseService) GetHostedRunner(ctx context.Context, enterprise string, runnerID int64) (*HostedRunner, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/%v", enterprise, runnerID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hostedRunner *HostedRunner
	resp, err := s.client.Do(req, &hostedRunner)
	if err != nil {
		return nil, resp, err
	}

	return hostedRunner, resp, nil
}

// UpdateHostedRunner updates a GitHub-hosted runner for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#update-a-github-hosted-runner-for-an-enterprise
//
//meta:operation PATCH /enterprises/{enterprise}/actions/hosted-runners/{hosted_runner_id}
func (s *EnterpriseService) UpdateHostedRunner(ctx context.Context, enterprise string, runnerID int64, body UpdateHostedRunnerRequest) (*HostedRunner, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/%v", enterprise, runnerID)
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var hostedRunner *HostedRunner
	resp, err := s.client.Do(req, &hostedRunner)
	if err != nil {
		return nil, resp, err
	}

	return hostedRunner, resp, nil
}

// DeleteHostedRunner deletes GitHub-hosted runner from an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#delete-a-github-hosted-runner-for-an-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/actions/hosted-runners/{hosted_runner_id}
func (s *EnterpriseService) DeleteHostedRunner(ctx context.Context, enterprise string, runnerID int64) (*HostedRunner, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/%v", enterprise, runnerID)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var hostedRunner *HostedRunner
	resp, err := s.client.Do(req, &hostedRunner)
	if err != nil {
		return nil, resp, err
	}

	return hostedRunner, resp, nil
}

// ListHostedRunnerCustomImages lists custom images for GitHub-hosted runners in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#list-custom-images-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/images/custom
func (s *EnterpriseService) ListHostedRunnerCustomImages(ctx context.Context, enterprise string) (*HostedRunnerCustomImages, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/custom", enterprise)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var images *HostedRunnerCustomImages
	resp, err := s.client.Do(req, &images)
	if err != nil {
		return nil, resp, err
	}

	return images, resp, nil
}

// GetHostedRunnerCustomImage gets a custom image definition for GitHub-hosted runners in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-an-enterprise-custom-image-definition-for-github-actions-hosted-runners
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/images/custom/{image_definition_id}
func (s *EnterpriseService) GetHostedRunnerCustomImage(ctx context.Context, enterprise string, imageDefinitionID int64) (*HostedRunnerCustomImage, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/custom/%v", enterprise, imageDefinitionID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var image *HostedRunnerCustomImage
	resp, err := s.client.Do(req, &image)
	if err != nil {
		return nil, resp, err
	}

	return image, resp, nil
}

// DeleteHostedRunnerCustomImage deletes a custom image from the enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#delete-a-custom-image-from-the-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/actions/hosted-runners/images/custom/{image_definition_id}
func (s *EnterpriseService) DeleteHostedRunnerCustomImage(ctx context.Context, enterprise string, imageDefinitionID int64) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/custom/%v", enterprise, imageDefinitionID)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListHostedRunnerCustomImageVersions lists image versions of a custom image for an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#list-image-versions-of-a-custom-image-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/images/custom/{image_definition_id}/versions
func (s *EnterpriseService) ListHostedRunnerCustomImageVersions(ctx context.Context, enterprise string, imageDefinitionID int64) (*HostedRunnerCustomImageVersions, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/custom/%v/versions", enterprise, imageDefinitionID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var versions *HostedRunnerCustomImageVersions
	resp, err := s.client.Do(req, &versions)
	if err != nil {
		return nil, resp, err
	}

	return versions, resp, nil
}

// GetHostedRunnerCustomImageVersion gets an image version of a custom image for GitHub-hosted runners in an enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#get-an-image-version-of-an-enterprise-custom-image-for-github-actions-hosted-runners
//
//meta:operation GET /enterprises/{enterprise}/actions/hosted-runners/images/custom/{image_definition_id}/versions/{version}
func (s *EnterpriseService) GetHostedRunnerCustomImageVersion(ctx context.Context, enterprise string, imageDefinitionID int64, version string) (*HostedRunnerCustomImageVersion, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/custom/%v/versions/%v", enterprise, imageDefinitionID, version)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var imageVersion *HostedRunnerCustomImageVersion
	resp, err := s.client.Do(req, &imageVersion)
	if err != nil {
		return nil, resp, err
	}

	return imageVersion, resp, nil
}

// DeleteHostedRunnerCustomImageVersion deletes an image version of a custom image from the enterprise.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/actions/hosted-runners?apiVersion=2022-11-28#delete-an-image-version-of-custom-image-from-the-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/actions/hosted-runners/images/custom/{image_definition_id}/versions/{version}
func (s *EnterpriseService) DeleteHostedRunnerCustomImageVersion(ctx context.Context, enterprise string, imageDefinitionID int64, version string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/hosted-runners/images/custom/%v/versions/%v", enterprise, imageDefinitionID, version)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
