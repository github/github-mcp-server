// Copyright 2025 The go-github AUTHORS. All rights reserved.
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

func TestEnterpriseService_CreateRepositoryRuleset_OrgNameRepoName(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 84,
			"name": "ruleset",
			"target": "branch",
			"source_type": "Enterprise",
			"source": "e",
			"enforcement": "active",
			"bypass_actors": [
				{
					"actor_id": 234,
					"actor_type": "Team"
				}
			],
			"conditions": {
				"organization_name": {
					"include": [
						"important_organization",
						"another_important_organization"
					],
					"exclude": [
						"unimportant_organization"
					]
				},
			  "repository_name": {
					"include": [
						"important_repository",
						"another_important_repository"
					],
					"exclude": [
						"unimportant_repository"
					],
					"protected": true
				},
			  "ref_name": {
					"include": [
						"refs/heads/main",
						"refs/heads/master"
					],
					"exclude": [
						"refs/heads/dev*"
					]
				}
			},
			"rules": [
			  {
				"type": "creation"
			  },
			  {
				"type": "update",
				"parameters": {
				  "update_allows_fetch_and_merge": true
				}
			  },
			  {
				"type": "deletion"
			  },
			  {
				"type": "required_linear_history"
			  },
			  {
				"type": "required_deployments",
				"parameters": {
				  "required_deployment_environments": ["test"]
				}
			  },
			  {
				"type": "required_signatures"
			  },
			  {
				"type": "pull_request",
				"parameters": {
					"allowed_merge_methods": ["rebase","squash"],
				  "dismiss_stale_reviews_on_push": true,
				  "require_code_owner_review": true,
				  "require_last_push_approval": true,
				  "required_approving_review_count": 1,
				  "required_review_thread_resolution": true
				}
			  },
			  {
				"type": "required_status_checks",
				"parameters": {
					"do_not_enforce_on_create": true,
				  "required_status_checks": [
					{
					  "context": "test",
					  "integration_id": 1
					}
				  ],
				  "strict_required_status_checks_policy": true
				}
			  },
			  {
				"type": "non_fast_forward"
			  },
			  {
				"type": "commit_message_pattern",
				"parameters": {
				  "name": "avoid test commits",
				  "negate": true,
				  "operator": "starts_with",
				  "pattern": "[test]"
				}
			  },
			  {
				"type": "commit_author_email_pattern",
				"parameters": {
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
				"type": "committer_email_pattern",
				"parameters": {
				  "name": "avoid commit emails",
				  "negate": true,
				  "operator": "ends_with",
				  "pattern": "abc"
				}
			  },
			  {
				"type": "branch_name_pattern",
				"parameters": {
				  "name": "avoid branch names",
				  "negate": true,
				  "operator": "regex",
				  "pattern": "github$"
				}
			  },
			  {
				"type": "tag_name_pattern",
				"parameters": {
				  "name": "avoid tag names",
				  "negate": true,
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
			    "type": "code_scanning",
			    "parameters": {
				  "code_scanning_tools": [
				    {
					  "tool": "CodeQL",
					  "security_alerts_threshold": "high_or_higher",
					  "alerts_threshold": "errors"
				    }
				  ]
			    }
			  }
			]
		  }`)
	})

	ctx := t.Context()
	ruleset, _, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationName: &RepositoryRulesetOrganizationNamesConditionParameters{
				Include: []string{"important_organization", "another_important_organization"},
				Exclude: []string{"unimportant_organization"},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	})
	if err != nil {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(84)),
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		SourceType:  Ptr(RulesetSourceTypeEnterprise),
		Source:      "e",
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationName: &RepositoryRulesetOrganizationNamesConditionParameters{
				Include: []string{"important_organization", "another_important_organization"},
				Exclude: []string{"unimportant_organization"},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	}
	if !cmp.Equal(ruleset, want) {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned %+v, want %+v", ruleset, want)
	}

	const methodName = "CreateRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateRepositoryRuleset_OmitZero_Nil(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := RepositoryRuleset{
		Name:         "test ruleset",
		BypassActors: nil,
	}

	mux.HandleFunc("/enterprises/e/rulesets/84", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id": 84, "name": "test ruleset"}`)
	})

	ctx := t.Context()
	_, _, err := client.Enterprise.UpdateRepositoryRuleset(ctx, "e", 84, input)
	if err != nil {
		t.Errorf("Enterprise.UpdateRepositoryRuleset returned error: %v", err)
	}
}

func TestEnterpriseService_UpdateRepositoryRuleset_OmitZero_EmptySlice(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	input := RepositoryRuleset{
		Name:         "test ruleset",
		BypassActors: []*BypassActor{},
	}

	mux.HandleFunc("/enterprises/e/rulesets/84", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		testJSONBody(t, r, input)
		fmt.Fprint(w, `{"id": 84, "name": "test ruleset", "bypass_actors": []}`)
	})

	ctx := t.Context()
	_, _, err := client.Enterprise.UpdateRepositoryRuleset(ctx, "e", 84, input)
	if err != nil {
		t.Errorf("Enterprise.UpdateRepositoryRuleset returned error: %v", err)
	}
}

func TestEnterpriseService_CreateRepositoryRuleset_OrgNameRepoProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 84,
			"name": "ruleset",
			"target": "branch",
			"source_type": "Enterprise",
			"source": "e",
			"enforcement": "active",
			"bypass_actors": [
				{
					"actor_id": 234,
					"actor_type": "Team"
				}
			],
			"conditions": {
				"organization_name": {
					"include": [
						"important_organization",
						"another_important_organization"
					],
					"exclude": [
						"unimportant_organization"
					]
				},
			  "repository_property": {
					"include": [
						{
							"name": "testIncludeProp",
							"source": "custom",
							"property_values": [
								"true"
							]
						}
					],
					"exclude": [
						{
							"name": "testExcludeProp",
							"property_values": [
								"false"
							]
						}
					]
				},
			  "ref_name": {
					"include": [
						"refs/heads/main",
						"refs/heads/master"
					],
					"exclude": [
						"refs/heads/dev*"
					]
				}
			},
			"rules": [
			  {
				"type": "creation"
			  },
			  {
				"type": "update",
				"parameters": {
				  "update_allows_fetch_and_merge": true
				}
			  },
			  {
				"type": "deletion"
			  },
			  {
				"type": "required_linear_history"
			  },
			  {
				"type": "required_deployments",
				"parameters": {
				  "required_deployment_environments": ["test"]
				}
			  },
			  {
				"type": "required_signatures"
			  },
			  {
				"type": "pull_request",
				"parameters": {
					"allowed_merge_methods": ["rebase","squash"],
				  "dismiss_stale_reviews_on_push": true,
				  "require_code_owner_review": true,
				  "require_last_push_approval": true,
				  "required_approving_review_count": 1,
				  "required_review_thread_resolution": true
				}
			  },
			  {
				"type": "required_status_checks",
				"parameters": {
					"do_not_enforce_on_create": true,
				  "required_status_checks": [
					{
					  "context": "test",
					  "integration_id": 1
					}
				  ],
				  "strict_required_status_checks_policy": true
				}
			  },
			  {
				"type": "non_fast_forward"
			  },
			  {
				"type": "commit_message_pattern",
				"parameters": {
				  "name": "avoid test commits",
				  "negate": true,
				  "operator": "starts_with",
				  "pattern": "[test]"
				}
			  },
			  {
				"type": "commit_author_email_pattern",
				"parameters": {
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
				"type": "committer_email_pattern",
				"parameters": {
				  "name": "avoid commit emails",
				  "negate": true,
				  "operator": "ends_with",
				  "pattern": "abc"
				}
			  },
			  {
				"type": "branch_name_pattern",
				"parameters": {
				  "name": "avoid branch names",
				  "negate": true,
				  "operator": "regex",
				  "pattern": "github$"
				}
			  },
			  {
				"type": "tag_name_pattern",
				"parameters": {
				  "name": "avoid tag names",
				  "negate": true,
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
			    "type": "code_scanning",
			    "parameters": {
				  "code_scanning_tools": [
				    {
					  "tool": "CodeQL",
					  "security_alerts_threshold": "high_or_higher",
					  "alerts_threshold": "errors"
				    }
				  ]
			    }
			  }
			]
		  }`)
	})

	ctx := t.Context()
	ruleset, _, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationName: &RepositoryRulesetOrganizationNamesConditionParameters{
				Include: []string{"important_organization", "another_important_organization"},
				Exclude: []string{"unimportant_organization"},
			},
			RepositoryProperty: &RepositoryRulesetRepositoryPropertyConditionParameters{
				Include: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testIncludeProp",
						Source:         Ptr("custom"),
						PropertyValues: []string{"true"},
					},
				},
				Exclude: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testExcludeProp",
						PropertyValues: []string{"false"},
					},
				},
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	})
	if err != nil {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(84)),
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		SourceType:  Ptr(RulesetSourceTypeEnterprise),
		Source:      "e",
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationName: &RepositoryRulesetOrganizationNamesConditionParameters{
				Include: []string{"important_organization", "another_important_organization"},
				Exclude: []string{"unimportant_organization"},
			},
			RepositoryProperty: &RepositoryRulesetRepositoryPropertyConditionParameters{
				Include: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testIncludeProp",
						Source:         Ptr("custom"),
						PropertyValues: []string{"true"},
					},
				},
				Exclude: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testExcludeProp",
						PropertyValues: []string{"false"},
					},
				},
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	}
	if !cmp.Equal(ruleset, want) {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned %+v, want %+v", ruleset, want)
	}

	const methodName = "CreateRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateRepositoryRuleset_OrgIdRepoName(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 84,
			"name": "ruleset",
			"target": "branch",
			"source_type": "Enterprise",
			"source": "e",
			"enforcement": "active",
			"bypass_actors": [
				{
					"actor_id": 234,
					"actor_type": "Team"
				}
			],
			"conditions": {
				"organization_id": {
					"organization_ids": [1001, 1002]
				},
			  "repository_name": {
					"include": [
						"important_repository",
						"another_important_repository"
					],
					"exclude": [
						"unimportant_repository"
					],
					"protected": true
				},
			  "ref_name": {
					"include": [
						"refs/heads/main",
						"refs/heads/master"
					],
					"exclude": [
						"refs/heads/dev*"
					]
				}
			},
			"rules": [
			  {
				"type": "creation"
			  },
			  {
				"type": "update",
				"parameters": {
				  "update_allows_fetch_and_merge": true
				}
			  },
			  {
				"type": "deletion"
			  },
			  {
				"type": "required_linear_history"
			  },
			  {
				"type": "required_deployments",
				"parameters": {
				  "required_deployment_environments": ["test"]
				}
			  },
			  {
				"type": "required_signatures"
			  },
			  {
				"type": "pull_request",
				"parameters": {
					"allowed_merge_methods": ["rebase","squash"],
				  "dismiss_stale_reviews_on_push": true,
				  "require_code_owner_review": true,
				  "require_last_push_approval": true,
				  "required_approving_review_count": 1,
				  "required_review_thread_resolution": true
				}
			  },
			  {
				"type": "required_status_checks",
				"parameters": {
					"do_not_enforce_on_create": true,
				  "required_status_checks": [
					{
					  "context": "test",
					  "integration_id": 1
					}
				  ],
				  "strict_required_status_checks_policy": true
				}
			  },
			  {
				"type": "non_fast_forward"
			  },
			  {
				"type": "commit_message_pattern",
				"parameters": {
				  "name": "avoid test commits",
				  "negate": true,
				  "operator": "starts_with",
				  "pattern": "[test]"
				}
			  },
			  {
				"type": "commit_author_email_pattern",
				"parameters": {
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
				"type": "committer_email_pattern",
				"parameters": {
				  "name": "avoid commit emails",
				  "negate": true,
				  "operator": "ends_with",
				  "pattern": "abc"
				}
			  },
			  {
				"type": "branch_name_pattern",
				"parameters": {
				  "name": "avoid branch names",
				  "negate": true,
				  "operator": "regex",
				  "pattern": "github$"
				}
			  },
			  {
				"type": "tag_name_pattern",
				"parameters": {
				  "name": "avoid tag names",
				  "negate": true,
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
			    "type": "code_scanning",
			    "parameters": {
				  "code_scanning_tools": [
				    {
					  "tool": "CodeQL",
					  "security_alerts_threshold": "high_or_higher",
					  "alerts_threshold": "errors"
				    }
				  ]
			    }
			  }
			]
		  }`)
	})

	ctx := t.Context()
	ruleset, _, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationID: &RepositoryRulesetOrganizationIDsConditionParameters{
				OrganizationIDs: []int64{1001, 1002},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	})
	if err != nil {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(84)),
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		SourceType:  Ptr(RulesetSourceTypeEnterprise),
		Source:      "e",
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationID: &RepositoryRulesetOrganizationIDsConditionParameters{
				OrganizationIDs: []int64{1001, 1002},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	}
	if !cmp.Equal(ruleset, want) {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned %+v, want %+v", ruleset, want)
	}

	const methodName = "CreateRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_CreateRepositoryRuleset_OrgIdRepoProperty(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "POST")
		fmt.Fprint(w, `{
			"id": 84,
			"name": "ruleset",
			"target": "branch",
			"source_type": "Enterprise",
			"source": "e",
			"enforcement": "active",
			"bypass_actors": [
				{
					"actor_id": 234,
					"actor_type": "Team"
				}
			],
			"conditions": {
				"organization_id": {
					"organization_ids": [1001, 1002]
				},
			  "repository_property": {
					"include": [
						{
							"name": "testIncludeProp",
							"source": "custom",
							"property_values": [
								"true"
							]
						}
					],
					"exclude": [
						{
							"name": "testExcludeProp",
							"property_values": [
								"false"
							]
						}
					]
				},
			  "ref_name": {
					"include": [
						"refs/heads/main",
						"refs/heads/master"
					],
					"exclude": [
						"refs/heads/dev*"
					]
				}
			},
			"rules": [
			  {
				"type": "creation"
			  },
			  {
				"type": "update",
				"parameters": {
				  "update_allows_fetch_and_merge": true
				}
			  },
			  {
				"type": "deletion"
			  },
			  {
				"type": "required_linear_history"
			  },
			  {
				"type": "required_deployments",
				"parameters": {
				  "required_deployment_environments": ["test"]
				}
			  },
			  {
				"type": "required_signatures"
			  },
			  {
				"type": "pull_request",
				"parameters": {
					"allowed_merge_methods": ["rebase","squash"],
				  "dismiss_stale_reviews_on_push": true,
				  "require_code_owner_review": true,
				  "require_last_push_approval": true,
				  "required_approving_review_count": 1,
				  "required_review_thread_resolution": true
				}
			  },
			  {
				"type": "required_status_checks",
				"parameters": {
					"do_not_enforce_on_create": true,
				  "required_status_checks": [
					{
					  "context": "test",
					  "integration_id": 1
					}
				  ],
				  "strict_required_status_checks_policy": true
				}
			  },
			  {
				"type": "non_fast_forward"
			  },
			  {
				"type": "commit_message_pattern",
				"parameters": {
				  "name": "avoid test commits",
				  "negate": true,
				  "operator": "starts_with",
				  "pattern": "[test]"
				}
			  },
			  {
				"type": "commit_author_email_pattern",
				"parameters": {
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
				"type": "committer_email_pattern",
				"parameters": {
				  "name": "avoid commit emails",
				  "negate": true,
				  "operator": "ends_with",
				  "pattern": "abc"
				}
			  },
			  {
				"type": "branch_name_pattern",
				"parameters": {
				  "name": "avoid branch names",
				  "negate": true,
				  "operator": "regex",
				  "pattern": "github$"
				}
			  },
			  {
				"type": "tag_name_pattern",
				"parameters": {
				  "name": "avoid tag names",
				  "negate": true,
				  "operator": "contains",
				  "pattern": "github"
				}
			  },
			  {
			    "type": "code_scanning",
			    "parameters": {
				  "code_scanning_tools": [
				    {
					  "tool": "CodeQL",
					  "security_alerts_threshold": "high_or_higher",
					  "alerts_threshold": "errors"
				    }
				  ]
			    }
			  }
			]
		  }`)
	})

	ctx := t.Context()
	ruleset, _, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationID: &RepositoryRulesetOrganizationIDsConditionParameters{
				OrganizationIDs: []int64{1001, 1002},
			},
			RepositoryProperty: &RepositoryRulesetRepositoryPropertyConditionParameters{
				Include: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testIncludeProp",
						Source:         Ptr("custom"),
						PropertyValues: []string{"true"},
					},
				},
				Exclude: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testExcludeProp",
						PropertyValues: []string{"false"},
					},
				},
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	})
	if err != nil {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(84)),
		Name:        "ruleset",
		Target:      Ptr(RulesetTargetBranch),
		SourceType:  Ptr(RulesetSourceTypeEnterprise),
		Source:      "e",
		Enforcement: "active",
		BypassActors: []*BypassActor{
			{
				ActorID:   Ptr(int64(234)),
				ActorType: Ptr(BypassActorTypeTeam),
			},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationID: &RepositoryRulesetOrganizationIDsConditionParameters{
				OrganizationIDs: []int64{1001, 1002},
			},
			RepositoryProperty: &RepositoryRulesetRepositoryPropertyConditionParameters{
				Include: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testIncludeProp",
						Source:         Ptr("custom"),
						PropertyValues: []string{"true"},
					},
				},
				Exclude: []*RepositoryRulesetRepositoryPropertyTargetParameters{
					{
						Name:           "testExcludeProp",
						PropertyValues: []string{"false"},
					},
				},
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{
			Creation: &EmptyRuleParameters{},
			Update: &UpdateRuleParameters{
				UpdateAllowsFetchAndMerge: true,
			},
			Deletion:              &EmptyRuleParameters{},
			RequiredLinearHistory: &EmptyRuleParameters{},
			RequiredDeployments: &RequiredDeploymentsRuleParameters{
				RequiredDeploymentEnvironments: []string{"test"},
			},
			RequiredSignatures: &EmptyRuleParameters{},
			PullRequest: &PullRequestRuleParameters{
				AllowedMergeMethods:            []PullRequestMergeMethod{PullRequestMergeMethodRebase, PullRequestMergeMethodSquash},
				DismissStaleReviewsOnPush:      true,
				RequireCodeOwnerReview:         true,
				RequireLastPushApproval:        true,
				RequiredApprovingReviewCount:   1,
				RequiredReviewThreadResolution: true,
			},
			RequiredStatusChecks: &RequiredStatusChecksRuleParameters{
				DoNotEnforceOnCreate: Ptr(true),
				RequiredStatusChecks: []*RuleStatusCheck{
					{
						Context:       "test",
						IntegrationID: Ptr(int64(1)),
					},
				},
				StrictRequiredStatusChecksPolicy: true,
			},
			NonFastForward: &EmptyRuleParameters{},
			CommitMessagePattern: &PatternRuleParameters{
				Name:     Ptr("avoid test commits"),
				Negate:   Ptr(true),
				Operator: "starts_with",
				Pattern:  "[test]",
			},
			CommitAuthorEmailPattern: &PatternRuleParameters{
				Operator: "contains",
				Pattern:  "github",
			},
			CommitterEmailPattern: &PatternRuleParameters{
				Name:     Ptr("avoid commit emails"),
				Negate:   Ptr(true),
				Operator: "ends_with",
				Pattern:  "abc",
			},
			BranchNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid branch names"),
				Negate:   Ptr(true),
				Operator: "regex",
				Pattern:  "github$",
			},
			TagNamePattern: &PatternRuleParameters{
				Name:     Ptr("avoid tag names"),
				Negate:   Ptr(true),
				Operator: "contains",
				Pattern:  "github",
			},
			CodeScanning: &CodeScanningRuleParameters{
				CodeScanningTools: []*RuleCodeScanningTool{
					{
						AlertsThreshold:         CodeScanningAlertsThresholdErrors,
						SecurityAlertsThreshold: CodeScanningSecurityAlertsThresholdHighOrHigher,
						Tool:                    "CodeQL",
					},
				},
			},
		},
	}
	if !cmp.Equal(ruleset, want) {
		t.Errorf("Enterprise.CreateRepositoryRuleset returned %+v, want %+v", ruleset, want)
	}

	const methodName = "CreateRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.CreateRepositoryRuleset(ctx, "e", RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_GetRepositoryRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets/84", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "GET")
		fmt.Fprint(w, `{
			"id": 84,
			"name": "test ruleset",
			"target": "branch",
			"source_type": "Enterprise",
			"source": "e",
			"enforcement": "active",
			"bypass_mode": "none",
			"node_id": "nid",
			"_links": {
			  "self": {
					"href": "https://api.github.com/enterprises/e/rulesets/84"
				}
			},
			"conditions": {
				"organization_name": {
					"include": [
						"important_organization",
						"another_important_organization"
					],
					"exclude": [
						"unimportant_organization"
					]
				},
				"repository_name": {
					"include": [
						"important_repository",
						"another_important_repository"
					],
					"exclude": [
						"unimportant_repository"
				  ],
				  "protected": true
				},
				"ref_name": {
					"include": [
						"refs/heads/main",
						"refs/heads/master"
				  ],
				  "exclude": [
						"refs/heads/dev*"
				  ]
				}
			},
			  "rules": [
					{
						"type": "creation"
					}
			  ]
		}`)
	})

	ctx := t.Context()
	rulesets, _, err := client.Enterprise.GetRepositoryRuleset(ctx, "e", 84)
	if err != nil {
		t.Errorf("Enterprise.GetRepositoryRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(84)),
		Name:        "test ruleset",
		Target:      Ptr(RulesetTargetBranch),
		SourceType:  Ptr(RulesetSourceTypeEnterprise),
		Source:      "e",
		Enforcement: "active",
		NodeID:      Ptr("nid"),
		Links: &RepositoryRulesetLinks{
			Self: &RepositoryRulesetLink{HRef: Ptr("https://api.github.com/enterprises/e/rulesets/84")},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationName: &RepositoryRulesetOrganizationNamesConditionParameters{
				Include: []string{"important_organization", "another_important_organization"},
				Exclude: []string{"unimportant_organization"},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{Creation: &EmptyRuleParameters{}},
	}
	if !cmp.Equal(rulesets, want) {
		t.Errorf("Enterprise.GetRepositoryRuleset returned %+v, want %+v", rulesets, want)
	}

	const methodName = "GetRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.GetRepositoryRuleset(ctx, "e", 84)
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_UpdateRepositoryRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets/84", func(w http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "PUT")
		fmt.Fprint(w, `{
			"id": 84,
			"name": "test ruleset",
			"target": "branch",
			"source_type": "Enterprise",
			"source": "e",
			"enforcement": "active",
			"bypass_mode": "none",
			"node_id": "nid",
			"_links": {
			  "self": {
				"href": "https://api.github.com/enterprises/e/rulesets/84"
			  }
			},
			"conditions": {
				"organization_name": {
					"include": [
						"important_organization",
						"another_important_organization"
					],
					"exclude": [
						"unimportant_organization"
					]
				},
				"repository_name": {
					"include": [
						"important_repository",
						"another_important_repository"
				  ],
					"exclude": [
						"unimportant_repository"
					],
				  "protected": true
				},
				"ref_name": {
					"include": [
						"refs/heads/main",
						"refs/heads/master"
					],
					"exclude": [
						"refs/heads/dev*"
					]
				}
			},
			"rules": [
				{
				  "type": "creation"
				}
			]
		}`)
	})

	ctx := t.Context()
	rulesets, _, err := client.Enterprise.UpdateRepositoryRuleset(ctx, "e", 84, RepositoryRuleset{
		Name:        "test ruleset",
		Target:      Ptr(RulesetTargetBranch),
		Enforcement: "active",
		Conditions: &RepositoryRulesetConditions{
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
		},
		Rules: &RepositoryRulesetRules{Creation: &EmptyRuleParameters{}},
	})
	if err != nil {
		t.Errorf("Enterprise.UpdateRepositoryRuleset returned error: %v", err)
	}

	want := &RepositoryRuleset{
		ID:          Ptr(int64(84)),
		Name:        "test ruleset",
		Target:      Ptr(RulesetTargetBranch),
		SourceType:  Ptr(RulesetSourceTypeEnterprise),
		Source:      "e",
		Enforcement: "active",
		NodeID:      Ptr("nid"),
		Links: &RepositoryRulesetLinks{
			Self: &RepositoryRulesetLink{HRef: Ptr("https://api.github.com/enterprises/e/rulesets/84")},
		},
		Conditions: &RepositoryRulesetConditions{
			OrganizationName: &RepositoryRulesetOrganizationNamesConditionParameters{
				Include: []string{"important_organization", "another_important_organization"},
				Exclude: []string{"unimportant_organization"},
			},
			RepositoryName: &RepositoryRulesetRepositoryNamesConditionParameters{
				Include:   []string{"important_repository", "another_important_repository"},
				Exclude:   []string{"unimportant_repository"},
				Protected: Ptr(true),
			},
			RefName: &RepositoryRulesetRefConditionParameters{
				Include: []string{"refs/heads/main", "refs/heads/master"},
				Exclude: []string{"refs/heads/dev*"},
			},
		},
		Rules: &RepositoryRulesetRules{Creation: &EmptyRuleParameters{}},
	}
	if !cmp.Equal(rulesets, want) {
		t.Errorf("Enterprise.UpdateRepositoryRuleset returned %+v, want %+v", rulesets, want)
	}

	const methodName = "UpdateRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		got, resp, err := client.Enterprise.UpdateRepositoryRuleset(ctx, "e", 84, RepositoryRuleset{})
		if got != nil {
			t.Errorf("testNewRequestAndDoFailure %v = %#v, want nil", methodName, got)
		}
		return resp, err
	})
}

func TestEnterpriseService_DeleteRepositoryRuleset(t *testing.T) {
	t.Parallel()
	client, mux, _ := setup(t)

	mux.HandleFunc("/enterprises/e/rulesets/84", func(_ http.ResponseWriter, r *http.Request) {
		testMethod(t, r, "DELETE")
	})

	ctx := t.Context()
	_, err := client.Enterprise.DeleteRepositoryRuleset(ctx, "e", 84)
	if err != nil {
		t.Errorf("Enterprise.DeleteRepositoryRuleset returned error: %v", err)
	}

	const methodName = "DeleteRepositoryRuleset"

	testNewRequestAndDoFailure(t, methodName, client, func() (*Response, error) {
		return client.Enterprise.DeleteRepositoryRuleset(ctx, "e", 84)
	})
}
