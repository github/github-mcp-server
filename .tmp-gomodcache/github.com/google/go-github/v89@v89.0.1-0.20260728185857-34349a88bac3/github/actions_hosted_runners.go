// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
)

// HostedRunnerPublicIP represents the details of a public IP for GitHub-hosted runner.
type HostedRunnerPublicIP struct {
	Enabled bool   `json:"enabled"` // Whether public IP is enabled.
	Prefix  string `json:"prefix"`  // The prefix for the public IP. Example: 20.80.208.150
	Length  int    `json:"length"`  // The length of the IP prefix. Example: 28
}

// HostedRunnerMachineSpec represents the details of a particular machine specification for GitHub-hosted runner.
type HostedRunnerMachineSpec struct {
	ID        string `json:"id"`         // The ID used for the `size` parameter when creating a new runner. Example: 8-core
	CPUCores  int    `json:"cpu_cores"`  // The number of cores. Example: 8
	MemoryGB  int    `json:"memory_gb"`  // The available RAM for the machine spec. Example: 32
	StorageGB int    `json:"storage_gb"` // The available SSD storage for the machine spec. Example: 300
}

// HostedRunner represents a single GitHub-hosted runner with additional details.
type HostedRunner struct {
	ID                 *int64                   `json:"id,omitempty"`
	Name               *string                  `json:"name,omitempty"`
	RunnerGroupID      *int64                   `json:"runner_group_id,omitempty"`
	Platform           *string                  `json:"platform,omitempty"`
	ImageDetails       *HostedRunnerImageDetail `json:"image_details,omitempty"`
	MachineSizeDetails *HostedRunnerMachineSpec `json:"machine_size_details,omitempty"`
	Status             *string                  `json:"status,omitempty"`
	MaximumRunners     *int64                   `json:"maximum_runners,omitempty"`
	PublicIPEnabled    *bool                    `json:"public_ip_enabled,omitempty"`
	PublicIPs          []*HostedRunnerPublicIP  `json:"public_ips,omitempty"`
	LastActiveOn       *Timestamp               `json:"last_active_on,omitempty"`
}

// HostedRunnerImageDetail represents the image details of a GitHub-hosted runners.
type HostedRunnerImageDetail struct {
	ID          *string `json:"id"`           // The ID of the image. Use this ID for the `image` parameter when creating a new larger runner. Example: ubuntu-20.04
	SizeGB      *int64  `json:"size_gb"`      // Image size in GB. Example: 86
	DisplayName *string `json:"display_name"` // Display name for this image. Example: 20.04
	Source      *string `json:"source"`       // The image provider. Example: github, partner, custom
	Version     *string `json:"version"`      // The image version of the hosted runner pool. Example: latest
}

// HostedRunners represents a collection of GitHub-hosted runners for an organization.
type HostedRunners struct {
	TotalCount int             `json:"total_count"`
	Runners    []*HostedRunner `json:"runners"`
}

// ListHostedRunners lists all the GitHub-hosted runners for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#list-github-hosted-runners-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners
func (s *ActionsService) ListHostedRunners(ctx context.Context, org string, opts *ListOptions) (*HostedRunners, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners", org)
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

// HostedRunnerImage represents the image of GitHub-hosted runners.
type HostedRunnerImage struct {
	// The unique identifier of the runner image.
	ID string `json:"id"`
	// The source of the runner image. Can be one of: github, partner, custom.
	Source string `json:"source"`
	// The version of the runner image to deploy. This is relevant only for runners using custom images.
	Version *string `json:"version,omitempty"`
}

// CreateHostedRunnerRequest specifies body parameters to create Hosted Runner configuration.
type CreateHostedRunnerRequest struct {
	Name           string            `json:"name"`
	Image          HostedRunnerImage `json:"image"`
	Size           string            `json:"size"`
	RunnerGroupID  int64             `json:"runner_group_id"`
	MaximumRunners *int64            `json:"maximum_runners,omitempty"`
	EnableStaticIP *bool             `json:"enable_static_ip,omitempty"`
	ImageGen       *bool             `json:"image_gen,omitempty"`
}

// UpdateHostedRunnerRequest specifies body parameters to update Hosted Runner configuration.
type UpdateHostedRunnerRequest struct {
	Name           *string `json:"name,omitempty"`
	RunnerGroupID  *int64  `json:"runner_group_id,omitempty"`
	MaximumRunners *int64  `json:"maximum_runners,omitempty"`
	EnableStaticIP *bool   `json:"enable_static_ip,omitempty"`
	Size           *string `json:"size,omitempty"`
	ImageID        *string `json:"image_id,omitempty"`
	ImageVersion   *string `json:"image_version,omitempty"`
}

// validateCreateHostedRunnerRequest validates the provided CreateHostedRunnerRequest to ensure
// that all required fields are properly set and that no invalid fields are present for hosted runner create request.
//
// If any of these conditions are violated, an appropriate error message is returned.
// Otherwise, nil is returned, indicating the request is valid.
func validateCreateHostedRunnerRequest(request *CreateHostedRunnerRequest) error {
	if request.Name == "" {
		return errors.New("name is required for creating a hosted runner")
	}
	if request.Image == (HostedRunnerImage{}) {
		return errors.New("image is required for creating a hosted runner")
	}
	if request.Size == "" {
		return errors.New("size is required for creating a hosted runner")
	}
	if request.RunnerGroupID == 0 {
		return errors.New("runner group ID is required for creating a hosted runner")
	}
	return nil
}

// CreateHostedRunner creates a GitHub-hosted runner for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#create-a-github-hosted-runner-for-an-organization
//
//meta:operation POST /orgs/{org}/actions/hosted-runners
func (s *ActionsService) CreateHostedRunner(ctx context.Context, org string, body CreateHostedRunnerRequest) (*HostedRunner, *Response, error) {
	if err := validateCreateHostedRunnerRequest(&body); err != nil {
		return nil, nil, fmt.Errorf("validation failed: %w", err)
	}

	u := fmt.Sprintf("orgs/%v/actions/hosted-runners", org)
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

// HostedRunnerCustomImage represents a custom image definition for GitHub-hosted runners.
type HostedRunnerCustomImage struct {
	ID                int64  `json:"id"`
	Platform          string `json:"platform"`
	Name              string `json:"name"`
	Source            string `json:"source"`
	VersionsCount     int    `json:"versions_count"`
	TotalVersionsSize int    `json:"total_versions_size"`
	LatestVersion     string `json:"latest_version"`
	State             string `json:"state"`
}

// HostedRunnerCustomImages represents a collection of custom images for GitHub-hosted runners.
type HostedRunnerCustomImages struct {
	TotalCount int                        `json:"total_count"`
	Images     []*HostedRunnerCustomImage `json:"images"`
}

// HostedRunnerCustomImageVersion represents a version of a custom image for GitHub-hosted runners.
type HostedRunnerCustomImageVersion struct {
	Version      string    `json:"version"`
	SizeGB       int       `json:"size_gb"`
	State        string    `json:"state"`
	StateDetails string    `json:"state_details"`
	CreatedOn    Timestamp `json:"created_on"`
}

// HostedRunnerCustomImageVersions represents a collection of versions of a custom image.
type HostedRunnerCustomImageVersions struct {
	TotalCount    int                               `json:"total_count"`
	ImageVersions []*HostedRunnerCustomImageVersion `json:"image_versions"`
}

// HostedRunnerImageSpecs represents the details of a GitHub-hosted runner image.
type HostedRunnerImageSpecs struct {
	ID          string `json:"id"`
	Platform    string `json:"platform"`
	SizeGB      int    `json:"size_gb"`
	DisplayName string `json:"display_name"`
	Source      string `json:"source"`
}

// HostedRunnerImages represents the response containing the total count and details of runner images.
type HostedRunnerImages struct {
	TotalCount int                       `json:"total_count"`
	Images     []*HostedRunnerImageSpecs `json:"images"`
}

// GetHostedRunnerGitHubOwnedImages gets the list of GitHub-owned images available for GitHub-hosted runners for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-github-owned-images-for-github-hosted-runners-in-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/images/github-owned
func (s *ActionsService) GetHostedRunnerGitHubOwnedImages(ctx context.Context, org string) (*HostedRunnerImages, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/github-owned", org)
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

// GetHostedRunnerPartnerImages gets the list of partner images available for GitHub-hosted runners for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-partner-images-for-github-hosted-runners-in-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/images/partner
func (s *ActionsService) GetHostedRunnerPartnerImages(ctx context.Context, org string) (*HostedRunnerImages, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/partner", org)
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

// HostedRunnerPublicIPLimits represents the static public IP limits for GitHub-hosted runners.
type HostedRunnerPublicIPLimits struct {
	PublicIPs *PublicIPUsage `json:"public_ips"`
}

// PublicIPUsage provides details of static public IP limits for GitHub-hosted runners.
type PublicIPUsage struct {
	Maximum      int64 `json:"maximum"`       // The maximum number of static public IP addresses that can be used for Hosted Runners. Example: 50
	CurrentUsage int64 `json:"current_usage"` // The current number of static public IP addresses in use by Hosted Runners. Example: 17
}

// GetHostedRunnerLimits gets the GitHub-hosted runners Static public IP Limits for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-limits-on-github-hosted-runners-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/limits
func (s *ActionsService) GetHostedRunnerLimits(ctx context.Context, org string) (*HostedRunnerPublicIPLimits, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/limits", org)
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

// HostedRunnerMachineSpecs represents the response containing the total count and details of machine specs for GitHub-hosted runners.
type HostedRunnerMachineSpecs struct {
	TotalCount   int                        `json:"total_count"`
	MachineSpecs []*HostedRunnerMachineSpec `json:"machine_specs"`
}

// GetHostedRunnerMachineSpecs gets the list of machine specs available for GitHub-hosted runners for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-github-hosted-runners-machine-specs-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/machine-sizes
func (s *ActionsService) GetHostedRunnerMachineSpecs(ctx context.Context, org string) (*HostedRunnerMachineSpecs, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/machine-sizes", org)
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

// HostedRunnerPlatforms represents the response containing the total count and platforms for GitHub-hosted runners.
type HostedRunnerPlatforms struct {
	TotalCount int      `json:"total_count"`
	Platforms  []string `json:"platforms"`
}

// GetHostedRunnerPlatforms gets list of platforms available for GitHub-hosted runners for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-platforms-for-github-hosted-runners-in-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/platforms
func (s *ActionsService) GetHostedRunnerPlatforms(ctx context.Context, org string) (*HostedRunnerPlatforms, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/platforms", org)
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

// GetHostedRunner gets a GitHub-hosted runner in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-a-github-hosted-runner-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/{hosted_runner_id}
func (s *ActionsService) GetHostedRunner(ctx context.Context, org string, runnerID int64) (*HostedRunner, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/%v", org, runnerID)
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

// UpdateHostedRunner updates a GitHub-hosted runner for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#update-a-github-hosted-runner-for-an-organization
//
//meta:operation PATCH /orgs/{org}/actions/hosted-runners/{hosted_runner_id}
func (s *ActionsService) UpdateHostedRunner(ctx context.Context, org string, runnerID int64, body UpdateHostedRunnerRequest) (*HostedRunner, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/%v", org, runnerID)
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

// DeleteHostedRunner deletes GitHub-hosted runner from an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#delete-a-github-hosted-runner-for-an-organization
//
//meta:operation DELETE /orgs/{org}/actions/hosted-runners/{hosted_runner_id}
func (s *ActionsService) DeleteHostedRunner(ctx context.Context, org string, runnerID int64) (*HostedRunner, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/%v", org, runnerID)
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

// ListHostedRunnerCustomImages lists custom images for GitHub-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#list-custom-images-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/images/custom
func (s *ActionsService) ListHostedRunnerCustomImages(ctx context.Context, org string) (*HostedRunnerCustomImages, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/custom", org)
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

// GetHostedRunnerCustomImage gets a custom image definition for GitHub-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-a-custom-image-definition-for-github-actions-hosted-runners
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/images/custom/{image_definition_id}
func (s *ActionsService) GetHostedRunnerCustomImage(ctx context.Context, org string, imageDefinitionID int64) (*HostedRunnerCustomImage, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/custom/%v", org, imageDefinitionID)
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

// DeleteHostedRunnerCustomImage deletes a custom image from the organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#delete-a-custom-image-from-the-organization
//
//meta:operation DELETE /orgs/{org}/actions/hosted-runners/images/custom/{image_definition_id}
func (s *ActionsService) DeleteHostedRunnerCustomImage(ctx context.Context, org string, imageDefinitionID int64) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/custom/%v", org, imageDefinitionID)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListHostedRunnerCustomImageVersions lists image versions of a custom image for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#list-image-versions-of-a-custom-image-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/images/custom/{image_definition_id}/versions
func (s *ActionsService) ListHostedRunnerCustomImageVersions(ctx context.Context, org string, imageDefinitionID int64) (*HostedRunnerCustomImageVersions, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/custom/%v/versions", org, imageDefinitionID)
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

// GetHostedRunnerCustomImageVersion gets an image version of a custom image for GitHub-hosted runners in an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#get-an-image-version-of-a-custom-image-for-github-actions-hosted-runners
//
//meta:operation GET /orgs/{org}/actions/hosted-runners/images/custom/{image_definition_id}/versions/{version}
func (s *ActionsService) GetHostedRunnerCustomImageVersion(ctx context.Context, org string, imageDefinitionID int64, version string) (*HostedRunnerCustomImageVersion, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/custom/%v/versions/%v", org, imageDefinitionID, version)
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

// DeleteHostedRunnerCustomImageVersion deletes an image version of a custom image from the organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/hosted-runners?apiVersion=2022-11-28#delete-an-image-version-of-custom-image-from-the-organization
//
//meta:operation DELETE /orgs/{org}/actions/hosted-runners/images/custom/{image_definition_id}/versions/{version}
func (s *ActionsService) DeleteHostedRunnerCustomImageVersion(ctx context.Context, org string, imageDefinitionID int64, version string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/hosted-runners/images/custom/%v/versions/%v", org, imageDefinitionID, version)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
