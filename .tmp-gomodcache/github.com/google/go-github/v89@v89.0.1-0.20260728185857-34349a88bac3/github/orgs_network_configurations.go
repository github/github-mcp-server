// Copyright 2025 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

// ComputeService represents a hosted compute service the network configuration supports.
type ComputeService string

const (
	ComputeServiceNone       ComputeService = "none"
	ComputeServiceActions    ComputeService = "actions"
	ComputeServiceCodespaces ComputeService = "codespaces"
)

// NetworkConfigurations represents a hosted compute network configuration. This type is identical
// for enterprise and organization endpoints.
type NetworkConfigurations struct {
	TotalCount            *int64                  `json:"total_count,omitempty"`
	NetworkConfigurations []*NetworkConfiguration `json:"network_configurations,omitempty"`
}

// NetworkConfiguration represents a hosted compute network configurations. This type is identical
// for enterprise and organization endpoints.
type NetworkConfiguration struct {
	ID                 *string         `json:"id,omitempty"`
	Name               *string         `json:"name,omitempty"`
	ComputeService     *ComputeService `json:"compute_service,omitempty"`
	NetworkSettingsIDs []string        `json:"network_settings_ids,omitempty"`
	CreatedOn          *Timestamp      `json:"created_on"`
}

// NetworkSettingsResource represents a hosted compute network settings resource. This type is identical
// for enterprise and organization endpoints.
type NetworkSettingsResource struct {
	ID                     *string `json:"id,omitempty"`
	NetworkConfigurationID *string `json:"network_configuration_id,omitempty"`
	Name                   *string `json:"name,omitempty"`
	SubnetID               *string `json:"subnet_id,omitempty"`
	Region                 *string `json:"region,omitempty"`
}

func validateComputeService(compute *ComputeService) error {
	if compute == nil {
		return nil
	}
	if *compute != ComputeServiceNone && *compute != ComputeServiceActions {
		return errors.New("compute service can only be one of: none, actions")
	}
	return nil
}

var validNetworkNameRE = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func validateNetworkName(name string) error {
	if len(name) < 1 || len(name) > 100 {
		return errors.New("must be between 1 and 100 characters")
	}
	if !validNetworkNameRE.MatchString(name) {
		return errors.New("may only contain upper and lowercase letters a-z, numbers 0-9, '.', '-', and '_'")
	}
	return nil
}

func validateNetworkSettingsID(settingsID []string) error {
	if len(settingsID) != 1 {
		return errors.New("exactly one network settings id must be specified")
	}
	return nil
}

func validateNetworkConfigurationRequest(req NetworkConfigurationRequest) error {
	networkName := req.GetName()
	if err := validateNetworkName(networkName); err != nil {
		return err
	}

	computeService := req.GetComputeService()
	if err := validateComputeService(computeService); err != nil {
		return err
	}

	networkIDs := req.NetworkSettingsIDs
	return validateNetworkSettingsID(networkIDs)
}

// NetworkConfigurationRequest represents a request to create or update a network configuration for an organization.
type NetworkConfigurationRequest struct {
	Name               *string         `json:"name,omitempty"`
	ComputeService     *ComputeService `json:"compute_service,omitempty"`
	NetworkSettingsIDs []string        `json:"network_settings_ids,omitempty"`
}

// ListNetworkConfigurations lists all hosted compute network configurations configured in an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/network-configurations?apiVersion=2022-11-28#list-hosted-compute-network-configurations-for-an-organization
//
//meta:operation GET /orgs/{org}/settings/network-configurations
func (s *OrganizationsService) ListNetworkConfigurations(ctx context.Context, org string, opts *ListOptions) (*NetworkConfigurations, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/network-configurations", org)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var configurations *NetworkConfigurations
	resp, err := s.client.Do(req, &configurations)
	if err != nil {
		return nil, resp, err
	}

	return configurations, resp, nil
}

// CreateNetworkConfiguration creates a hosted compute network configuration for an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/network-configurations?apiVersion=2022-11-28#create-a-hosted-compute-network-configuration-for-an-organization
//
//meta:operation POST /orgs/{org}/settings/network-configurations
func (s *OrganizationsService) CreateNetworkConfiguration(ctx context.Context, org string, body NetworkConfigurationRequest) (*NetworkConfiguration, *Response, error) {
	if err := validateNetworkConfigurationRequest(body); err != nil {
		return nil, nil, fmt.Errorf("validation failed: %w", err)
	}

	u := fmt.Sprintf("orgs/%v/settings/network-configurations", org)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var configuration *NetworkConfiguration
	resp, err := s.client.Do(req, &configuration)
	if err != nil {
		return nil, resp, err
	}

	return configuration, resp, nil
}

// GetNetworkConfiguration gets a hosted compute network configuration configured in an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/network-configurations?apiVersion=2022-11-28#get-a-hosted-compute-network-configuration-for-an-organization
//
//meta:operation GET /orgs/{org}/settings/network-configurations/{network_configuration_id}
func (s *OrganizationsService) GetNetworkConfiguration(ctx context.Context, org, networkID string) (*NetworkConfiguration, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/network-configurations/%v", org, networkID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var configuration *NetworkConfiguration
	resp, err := s.client.Do(req, &configuration)
	if err != nil {
		return nil, resp, err
	}

	return configuration, resp, nil
}

// UpdateNetworkConfiguration updates a hosted compute network configuration for an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/network-configurations?apiVersion=2022-11-28#update-a-hosted-compute-network-configuration-for-an-organization
//
//meta:operation PATCH /orgs/{org}/settings/network-configurations/{network_configuration_id}
func (s *OrganizationsService) UpdateNetworkConfiguration(ctx context.Context, org, networkID string, body NetworkConfigurationRequest) (*NetworkConfiguration, *Response, error) {
	if err := validateNetworkConfigurationRequest(body); err != nil {
		return nil, nil, fmt.Errorf("validation failed: %w", err)
	}

	u := fmt.Sprintf("orgs/%v/settings/network-configurations/%v", org, networkID)
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var configuration *NetworkConfiguration
	resp, err := s.client.Do(req, &configuration)
	if err != nil {
		return nil, resp, err
	}

	return configuration, resp, nil
}

// DeleteNetworkConfigurations deletes a hosted compute network configuration from an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/network-configurations?apiVersion=2022-11-28#delete-a-hosted-compute-network-configuration-from-an-organization
//
//meta:operation DELETE /orgs/{org}/settings/network-configurations/{network_configuration_id}
func (s *OrganizationsService) DeleteNetworkConfigurations(ctx context.Context, org, networkID string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/network-configurations/%v", org, networkID)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	var configuration *NetworkConfiguration
	resp, err := s.client.Do(req, &configuration)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// GetNetworkConfigurationResource gets a hosted compute network settings resource configured for an organization.
//
// GitHub API docs: https://docs.github.com/rest/orgs/network-configurations?apiVersion=2022-11-28#get-a-hosted-compute-network-settings-resource-for-an-organization
//
//meta:operation GET /orgs/{org}/settings/network-settings/{network_settings_id}
func (s *OrganizationsService) GetNetworkConfigurationResource(ctx context.Context, org, networkID string) (*NetworkSettingsResource, *Response, error) {
	u := fmt.Sprintf("orgs/%v/settings/network-settings/%v", org, networkID)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var resource *NetworkSettingsResource
	resp, err := s.client.Do(req, &resource)
	if err != nil {
		return nil, resp, err
	}

	return resource, resp, nil
}
