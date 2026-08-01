// Copyright 2022 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// EnterpriseSecurityAnalysisSettings represents security analysis settings for an enterprise.
type EnterpriseSecurityAnalysisSettings struct {
	AdvancedSecurityEnabledForNewRepositories             *bool   `json:"advanced_security_enabled_for_new_repositories,omitempty"`
	SecretScanningEnabledForNewRepositories               *bool   `json:"secret_scanning_enabled_for_new_repositories,omitempty"`
	SecretScanningPushProtectionEnabledForNewRepositories *bool   `json:"secret_scanning_push_protection_enabled_for_new_repositories,omitempty"`
	SecretScanningPushProtectionCustomLink                *string `json:"secret_scanning_push_protection_custom_link,omitempty"`
	SecretScanningValidityChecksEnabled                   *bool   `json:"secret_scanning_validity_checks_enabled,omitempty"`
}

// GetCodeSecurityAndAnalysis gets code security and analysis features for an enterprise.
//
// Deprecated: This endpoint has been deprecated by GitHub.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/code-security-and-analysis?apiVersion=2022-11-28#get-code-security-and-analysis-features-for-an-enterprise
//
//meta:operation GET /enterprises/{enterprise}/code_security_and_analysis
func (s *EnterpriseService) GetCodeSecurityAndAnalysis(ctx context.Context, enterprise string) (*EnterpriseSecurityAnalysisSettings, *Response, error) {
	u := fmt.Sprintf("enterprises/%v/code_security_and_analysis", enterprise)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var settings *EnterpriseSecurityAnalysisSettings
	resp, err := s.client.Do(req, &settings)
	if err != nil {
		return nil, resp, err
	}

	return settings, resp, nil
}

// UpdateCodeSecurityAndAnalysis updates code security and analysis features for new repositories in an enterprise.
//
// Deprecated: This endpoint has been deprecated by GitHub.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/code-security-and-analysis?apiVersion=2022-11-28#update-code-security-and-analysis-features-for-an-enterprise
//
//meta:operation PATCH /enterprises/{enterprise}/code_security_and_analysis
func (s *EnterpriseService) UpdateCodeSecurityAndAnalysis(ctx context.Context, enterprise string, body *EnterpriseSecurityAnalysisSettings) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/code_security_and_analysis", enterprise)
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// EnableDisableSecurityFeature enables or disables a security feature for all repositories in an enterprise.
//
// Valid values for securityProduct: "advanced_security", "secret_scanning", "secret_scanning_push_protection".
// Valid values for enablement:  "enable_all", "disable_all".
//
// Deprecated: This endpoint has been deprecated by GitHub.
//
// GitHub API docs: https://docs.github.com/enterprise-cloud@latest/rest/enterprise-admin/code-security-and-analysis?apiVersion=2022-11-28#enable-or-disable-a-security-feature
//
//meta:operation POST /enterprises/{enterprise}/{security_product}/{enablement}
func (s *EnterpriseService) EnableDisableSecurityFeature(ctx context.Context, enterprise, securityProduct, enablement string) (*Response, error) {
	u := fmt.Sprintf("enterprises/%v/%v/%v", enterprise, securityProduct, enablement)
	req, err := s.client.NewRequest(ctx, "POST", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}
