// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCopilotService_GetCloudAgentConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		responseBody string
		want         *CopilotCloudAgentConfiguration
		wantErr      bool
	}{
		{
			name: "with null mcp_configuration",
			responseBody: `{
				"mcp_configuration": null,
				"enabled_tools": {
					"codeql": true,
					"copilot_code_review": true,
					"secret_scanning": true,
					"dependency_vulnerability_checks": true
				},
				"require_actions_workflow_approval": true,
				"is_firewall_enabled": true,
				"is_firewall_recommended_allowlist_enabled": true,
				"custom_allowlist": []
			}`,
			want: &CopilotCloudAgentConfiguration{
				MCPConfiguration: nil,
				EnabledTools: &CopilotCloudAgentEnabledTools{
					Codeql:                        true,
					CopilotCodeReview:             true,
					SecretScanning:                true,
					DependencyVulnerabilityChecks: true,
				},
				RequireActionsWorkflowApproval:        true,
				IsFirewallEnabled:                     true,
				IsFirewallRecommendedAllowlistEnabled: true,
				CustomAllowlist:                       []string{},
			},
			wantErr: false,
		},
		{
			name: "with active mcp_configuration object",
			responseBody: `{
				"mcp_configuration": {
					"type": "resource",
					"uri": "stdio://server"
				},
				"enabled_tools": {
					"codeql": true,
					"copilot_code_review": true,
					"secret_scanning": true,
					"dependency_vulnerability_checks": true
				},
				"require_actions_workflow_approval": true,
				"is_firewall_enabled": true,
				"is_firewall_recommended_allowlist_enabled": true,
				"custom_allowlist": []
			}`,
			want: &CopilotCloudAgentConfiguration{
				MCPConfiguration: map[string]any{
					"type": "resource",
					"uri":  "stdio://server",
				},
				EnabledTools: &CopilotCloudAgentEnabledTools{
					Codeql:                        true,
					CopilotCodeReview:             true,
					SecretScanning:                true,
					DependencyVulnerabilityChecks: true,
				},
				RequireActionsWorkflowApproval:        true,
				IsFirewallEnabled:                     true,
				IsFirewallRecommendedAllowlistEnabled: true,
				CustomAllowlist:                       []string{},
			},
			wantErr: false,
		},
		{
			name: "with custom allowlist",
			responseBody: `{
				"mcp_configuration": null,
				"enabled_tools": {
					"codeql": false,
					"copilot_code_review": true,
					"secret_scanning": false,
					"dependency_vulnerability_checks": true
				},
				"require_actions_workflow_approval": false,
				"is_firewall_enabled": true,
				"is_firewall_recommended_allowlist_enabled": true,
				"custom_allowlist": ["example.com"]
			}`,
			want: &CopilotCloudAgentConfiguration{
				MCPConfiguration: nil,
				EnabledTools: &CopilotCloudAgentEnabledTools{
					Codeql:                        false,
					CopilotCodeReview:             true,
					SecretScanning:                false,
					DependencyVulnerabilityChecks: true,
				},
				RequireActionsWorkflowApproval:        false,
				IsFirewallEnabled:                     true,
				IsFirewallRecommendedAllowlistEnabled: true,
				CustomAllowlist:                       []string{"example.com"},
			},
			wantErr: false,
		},
		{
			name: "all tools disabled",
			responseBody: `{
				"mcp_configuration": null,
				"enabled_tools": {
					"codeql": false,
					"copilot_code_review": false,
					"secret_scanning": false,
					"dependency_vulnerability_checks": false
				},
				"require_actions_workflow_approval": false,
				"is_firewall_enabled": false,
				"is_firewall_recommended_allowlist_enabled": false,
				"custom_allowlist": []
			}`,
			want: &CopilotCloudAgentConfiguration{
				MCPConfiguration: nil,
				EnabledTools: &CopilotCloudAgentEnabledTools{
					Codeql:                        false,
					CopilotCodeReview:             false,
					SecretScanning:                false,
					DependencyVulnerabilityChecks: false,
				},
				RequireActionsWorkflowApproval:        false,
				IsFirewallEnabled:                     false,
				IsFirewallRecommendedAllowlistEnabled: false,
				CustomAllowlist:                       []string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			client, mux, _ := setup(t)

			mux.HandleFunc("/repos/o/r/copilot/cloud-agent/configuration", func(w http.ResponseWriter, r *http.Request) {
				testMethod(t, r, "GET")
				fmt.Fprint(w, tt.responseBody)
			})

			ctx := t.Context()
			config, _, err := client.Copilot.GetCloudAgentConfiguration(ctx, "o", "r")
			if (err != nil) != tt.wantErr {
				t.Errorf("GetCloudAgentConfiguration returned error: %v, wantErr: %v", err, tt.wantErr)
			}

			if !cmp.Equal(config, tt.want) {
				t.Errorf("GetCloudAgentConfiguration returned %+v, want %+v", config, tt.want)
			}
		})
	}
}

func TestCopilotService_GetCloudAgentConfiguration_BadOptions(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	const methodName = "GetCloudAgentConfiguration"
	testBadOptions(t, methodName, func() (err error) {
		_, _, err = client.Copilot.GetCloudAgentConfiguration(ctx, "\n", "\n")
		return err
	})
}

func TestCopilotService_GetCloudAgentConfiguration_NewRequestFailure(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	const methodName = "GetCloudAgentConfiguration"
	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		ctx := t.Context()
		got, resp, err := client.Copilot.GetCloudAgentConfiguration(ctx, "o", "r")
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestCopilotService_GetCloudAgentConfiguration_InvalidOwner(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Copilot.GetCloudAgentConfiguration(ctx, "%", "r")
	testURLParseError(t, err)
}

func TestCopilotService_GetCloudAgentConfiguration_InvalidRepo(t *testing.T) {
	t.Parallel()
	client, _, _ := setup(t)

	ctx := t.Context()
	_, _, err := client.Copilot.GetCloudAgentConfiguration(ctx, "o", "%")
	testURLParseError(t, err)
}

func TestCopilotService_GetCloudAgentConfiguration_MalformedJSON(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/repos/o/r/copilot/cloud-agent/configuration", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{invalid json}`)
	})

	ctx := t.Context()
	config, _, err := client.Copilot.GetCloudAgentConfiguration(ctx, "o", "r")
	if err == nil {
		t.Error("Expected error from malformed JSON")
	}
	if config != nil {
		t.Errorf("GetCloudAgentConfiguration should return nil on error, got %+v", config)
	}
}
