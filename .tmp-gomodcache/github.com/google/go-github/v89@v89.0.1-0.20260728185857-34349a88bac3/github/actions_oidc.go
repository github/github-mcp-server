// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// OIDCSubjectClaimCustomTemplate represents an OIDC subject claim customization template.
type OIDCSubjectClaimCustomTemplate struct {
	UseDefault          *bool    `json:"use_default,omitempty"`
	IncludeClaimKeys    []string `json:"include_claim_keys,omitempty"`
	UseImmutableSubject *bool    `json:"use_immutable_subject,omitempty"`
	SubClaimPrefix      *string  `json:"sub_claim_prefix,omitempty"`
}

// GetOrgOIDCSubjectClaimCustomTemplate gets the subject claim customization template for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#get-the-customization-template-for-an-oidc-subject-claim-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/oidc/customization/sub
func (s *ActionsService) GetOrgOIDCSubjectClaimCustomTemplate(ctx context.Context, org string) (*OIDCSubjectClaimCustomTemplate, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/oidc/customization/sub", org)
	return s.getOIDCSubjectClaimCustomTemplate(ctx, u)
}

// GetRepoOIDCSubjectClaimCustomTemplate gets the subject claim customization template for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#get-the-customization-template-for-an-oidc-subject-claim-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/actions/oidc/customization/sub
func (s *ActionsService) GetRepoOIDCSubjectClaimCustomTemplate(ctx context.Context, owner, repo string) (*OIDCSubjectClaimCustomTemplate, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/oidc/customization/sub", owner, repo)
	return s.getOIDCSubjectClaimCustomTemplate(ctx, u)
}

func (s *ActionsService) getOIDCSubjectClaimCustomTemplate(ctx context.Context, url string) (*OIDCSubjectClaimCustomTemplate, *Response, error) {
	req, err := s.client.NewRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var tmpl *OIDCSubjectClaimCustomTemplate
	resp, err := s.client.Do(req, &tmpl)
	if err != nil {
		return nil, resp, err
	}

	return tmpl, resp, nil
}

// SetOrgOIDCSubjectClaimCustomTemplate sets the subject claim customization for an organization.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#set-the-customization-template-for-an-oidc-subject-claim-for-an-organization
//
//meta:operation PUT /orgs/{org}/actions/oidc/customization/sub
func (s *ActionsService) SetOrgOIDCSubjectClaimCustomTemplate(ctx context.Context, org string, body OIDCSubjectClaimCustomTemplate) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/oidc/customization/sub", org)
	return s.setOIDCSubjectClaimCustomTemplate(ctx, u, body)
}

// SetRepoOIDCSubjectClaimCustomTemplate sets the subject claim customization for a repository.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#set-the-customization-template-for-an-oidc-subject-claim-for-a-repository
//
//meta:operation PUT /repos/{owner}/{repo}/actions/oidc/customization/sub
func (s *ActionsService) SetRepoOIDCSubjectClaimCustomTemplate(ctx context.Context, owner, repo string, body OIDCSubjectClaimCustomTemplate) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/actions/oidc/customization/sub", owner, repo)
	return s.setOIDCSubjectClaimCustomTemplate(ctx, u, body)
}

func (s *ActionsService) setOIDCSubjectClaimCustomTemplate(ctx context.Context, url string, body OIDCSubjectClaimCustomTemplate) (*Response, error) {
	req, err := s.client.NewRequest(ctx, "PUT", url, body)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// InclusionSource represents whether the custom oidc property claim was defined at the organization or enterprise level.
type InclusionSource string

// InclusionSource constants represent whether the inclusion was defined at the organization or enterprise level.
const (
	InclusionSourceEnterprise   InclusionSource = "enterprise"
	InclusionSourceOrganization InclusionSource = "organization"
)

// OIDCCustomPropertyClaim represents an OIDC custom property claim for GitHub Actions.
type OIDCCustomPropertyClaim struct {
	CustomPropertyName string `json:"custom_property_name"`
}

// OIDCCustomPropertyClaimResponse represents the OIDC custom property claim along with the inclusion source (enterprise or organization) for GitHub Actions.
type OIDCCustomPropertyClaimResponse struct {
	OIDCCustomPropertyClaim
	InclusionSource InclusionSource `json:"inclusion_source"`
}

// ListEnterpriseOIDCCustomPropertyClaims lists the custom property claims in oidc for enterprise actions.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#list-oidc-custom-property-inclusions-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/actions/oidc/customization/properties/repo
func (s *ActionsService) ListEnterpriseOIDCCustomPropertyClaims(ctx context.Context, enterprise string) ([]*OIDCCustomPropertyClaimResponse, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/oidc/customization/properties/repo", enterprise)
	return s.listOIDCCustomPropertyClaims(ctx, u)
}

// ListOrgOIDCCustomPropertyClaims lists the custom property claims in oidc for organization actions.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#list-oidc-custom-property-inclusions-for-an-organization
//
//meta:operation GET /orgs/{org}/actions/oidc/customization/properties/repo
func (s *ActionsService) ListOrgOIDCCustomPropertyClaims(ctx context.Context, org string) ([]*OIDCCustomPropertyClaimResponse, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/oidc/customization/properties/repo", org)
	return s.listOIDCCustomPropertyClaims(ctx, u)
}

func (s *ActionsService) listOIDCCustomPropertyClaims(ctx context.Context, url string) ([]*OIDCCustomPropertyClaimResponse, *Response, error) {
	req, err := s.client.NewRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, nil, err
	}

	var props []*OIDCCustomPropertyClaimResponse
	resp, err := s.client.Do(req, &props)
	if err != nil {
		return nil, resp, err
	}

	return props, resp, nil
}

// SetEnterpriseOIDCCustomPropertyClaim sets a new custom property claim in oidc for enterprise actions.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#create-an-oidc-custom-property-inclusion-for-an-enterprise
//
//meta:operation POST /enterprises/{enterprise}/actions/oidc/customization/properties/repo
func (s *ActionsService) SetEnterpriseOIDCCustomPropertyClaim(ctx context.Context, enterprise string, body OIDCCustomPropertyClaim) (*OIDCCustomPropertyClaim, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/oidc/customization/properties/repo", enterprise)
	return s.setOIDCCustomPropertyClaim(ctx, u, body)
}

// SetOrgOIDCCustomPropertyClaim sets a new custom property claim in oidc for organization actions.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#create-an-oidc-custom-property-inclusion-for-an-organization
//
//meta:operation POST /orgs/{org}/actions/oidc/customization/properties/repo
func (s *ActionsService) SetOrgOIDCCustomPropertyClaim(ctx context.Context, org string, body OIDCCustomPropertyClaim) (*OIDCCustomPropertyClaim, *Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/oidc/customization/properties/repo", org)
	return s.setOIDCCustomPropertyClaim(ctx, u, body)
}

func (s *ActionsService) setOIDCCustomPropertyClaim(ctx context.Context, url string, body OIDCCustomPropertyClaim) (*OIDCCustomPropertyClaim, *Response, error) {
	req, err := s.client.NewRequest(ctx, "POST", url, body)
	if err != nil {
		return nil, nil, err
	}

	var customProperty *OIDCCustomPropertyClaim
	resp, err := s.client.Do(req, &customProperty)
	if err != nil {
		return nil, resp, err
	}
	return customProperty, resp, err
}

// DeleteEnterpriseOIDCCustomPropertyClaim deletes a custom property claim in oidc for enterprise actions.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#delete-an-oidc-custom-property-inclusion-for-an-enterprise
//
//meta:operation DELETE /enterprises/{enterprise}/actions/oidc/customization/properties/repo/{custom_property_name}
func (s *ActionsService) DeleteEnterpriseOIDCCustomPropertyClaim(ctx context.Context, enterprise, customProperty string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/actions/oidc/customization/properties/repo/%v", enterprise, customProperty)
	return s.deleteOIDCCustomPropertyClaim(ctx, u)
}

// DeleteOrgOIDCCustomPropertyClaim deletes a custom property claim in oidc for organization actions.
//
// GitHub API docs: https://docs.github.com/rest/actions/oidc?apiVersion=2022-11-28#delete-an-oidc-custom-property-inclusion-for-an-organization
//
//meta:operation DELETE /orgs/{org}/actions/oidc/customization/properties/repo/{custom_property_name}
func (s *ActionsService) DeleteOrgOIDCCustomPropertyClaim(ctx context.Context, enterprise, customProperty string) (*Response, error) {
	u := fmt.Sprintf("orgs/%v/actions/oidc/customization/properties/repo/%v", enterprise, customProperty)
	return s.deleteOIDCCustomPropertyClaim(ctx, u)
}

func (s *ActionsService) deleteOIDCCustomPropertyClaim(ctx context.Context, url string) (*Response, error) {
	req, err := s.client.NewRequest(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
