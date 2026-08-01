// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// CopilotCloudAgentConfiguration represents the Copilot cloud agent configuration for a repository.
type CopilotCloudAgentConfiguration struct {
	MCPConfiguration                      any                            `json:"mcp_configuration"`
	EnabledTools                          *CopilotCloudAgentEnabledTools `json:"enabled_tools"`
	RequireActionsWorkflowApproval        bool                           `json:"require_actions_workflow_approval"`
	IsFirewallEnabled                     bool                           `json:"is_firewall_enabled"`
	IsFirewallRecommendedAllowlistEnabled bool                           `json:"is_firewall_recommended_allowlist_enabled"`
	CustomAllowlist                       []string                       `json:"custom_allowlist"`
}

// CopilotCloudAgentEnabledTools represents the enabled review tools for Copilot cloud agent.
type CopilotCloudAgentEnabledTools struct {
	Codeql                        bool `json:"codeql"`
	CopilotCodeReview             bool `json:"copilot_code_review"`
	SecretScanning                bool `json:"secret_scanning"`
	DependencyVulnerabilityChecks bool `json:"dependency_vulnerability_checks"`
}

// GetCloudAgentConfiguration gets the Copilot cloud agent configuration for a repository.
//
// GitHub API docs: https://docs.github.com/rest/copilot/copilot-cloud-agent-management?apiVersion=2022-11-28#get-copilot-cloud-agent-configuration-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/copilot/cloud-agent/configuration
func (s *CopilotService) GetCloudAgentConfiguration(ctx context.Context, owner, repo string) (*CopilotCloudAgentConfiguration, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/copilot/cloud-agent/configuration", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var config *CopilotCloudAgentConfiguration
	resp, err := s.client.Do(req, &config)
	if err != nil {
		return nil, resp, err
	}

	return config, resp, nil
}
