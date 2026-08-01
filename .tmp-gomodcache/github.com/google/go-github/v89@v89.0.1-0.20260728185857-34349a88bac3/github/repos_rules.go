// Copyright 2023 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
	"iter"
)

// ListRulesForBranch gets all the repository rules that apply to the specified branch.
//
// GitHub API docs: https://docs.github.com/rest/repos/rules?apiVersion=2022-11-28#get-rules-for-a-branch
//
//meta:operation GET /repos/{owner}/{repo}/rules/branches/{branch}
func (s *RepositoriesService) ListRulesForBranch(ctx context.Context, owner, repo, branch string, opts *ListOptions) (*BranchRules, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/rules/branches/%v", owner, repo, branch)

	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var rules *BranchRules
	resp, err := s.client.Do(req, &rules)
	if err != nil {
		return nil, resp, err
	}

	return rules, resp, nil
}

// ListRulesForBranchIter returns an iterator that paginates through all results of ListRulesForBranch.
//
// Note that since [BranchRules] contains a large number of slices, this iterator
// returns type `any` and it is therefore the responsibility of the caller to perform a
// type switch to determine what item is being returned for each iteration.
func (s *RepositoriesService) ListRulesForBranchIter(ctx context.Context, owner, repo, branch string, opts *ListOptions) iter.Seq2[any, error] {
	return func(yield func(any, error) bool) {
		// Create a copy of opts to avoid mutating the caller's struct
		if opts == nil {
			opts = &ListOptions{}
		} else {
			opts = Ptr(*opts)
		}

		for {
			results, resp, err := s.ListRulesForBranch(ctx, owner, repo, branch, opts)
			if err != nil {
				yield(nil, err)
				return
			}

			// Now iterate through ALL possible results from [BranchRules].
			for _, item := range results.Creation {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.Update {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.Deletion {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.RequiredLinearHistory {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.MergeQueue {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.RequiredDeployments {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.RequiredSignatures {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.PullRequest {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.RequiredStatusChecks {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.NonFastForward {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.CommitMessagePattern {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.CommitAuthorEmailPattern {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.CommitterEmailPattern {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.BranchNamePattern {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.TagNamePattern {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.Workflows {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.CodeScanning {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.CopilotCodeReview {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.FileExtensionRestriction {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.FilePathRestriction {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.MaxFilePathLength {
				if !yield(item, nil) {
					return
				}
			}
			for _, item := range results.MaxFileSize {
				if !yield(item, nil) {
					return
				}
			}

			if resp.NextPage == 0 {
				break
			}
			opts.Page = resp.NextPage
		}
	}
}

// RepositoryListRulesetsOptions specifies optional parameters to the
// RepositoriesService.GetAllRulesets method.
type RepositoryListRulesetsOptions struct {
	// IncludesParents indicates whether to include rulesets configured at the organization or enterprise level that apply to the repository.
	IncludesParents *bool `url:"includes_parents,omitempty"`
	ListOptions
}

// GetAllRulesets gets all the repository rulesets for the specified repository.
// By default, this endpoint will include rulesets configured at the organization or enterprise level that apply to the repository.
// To exclude those rulesets, set the `RepositoryListRulesetsOptions.IncludesParents` parameter to `false`.
//
// GitHub API docs: https://docs.github.com/rest/repos/rules?apiVersion=2022-11-28#get-all-repository-rulesets
//
//meta:operation GET /repos/{owner}/{repo}/rulesets
func (s *RepositoriesService) GetAllRulesets(ctx context.Context, owner, repo string, opts *RepositoryListRulesetsOptions) ([]*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/rulesets", owner, repo)

	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var ruleset []*RepositoryRuleset
	resp, err := s.client.Do(req, &ruleset)
	if err != nil {
		return nil, resp, err
	}

	return ruleset, resp, nil
}

// CreateRuleset creates a repository ruleset for the specified repository.
//
// GitHub API docs: https://docs.github.com/rest/repos/rules?apiVersion=2022-11-28#create-a-repository-ruleset
//
//meta:operation POST /repos/{owner}/{repo}/rulesets
func (s *RepositoriesService) CreateRuleset(ctx context.Context, owner, repo string, body RepositoryRuleset) (*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/rulesets", owner, repo)

	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var rs *RepositoryRuleset
	resp, err := s.client.Do(req, &rs)
	if err != nil {
		return nil, resp, err
	}

	return rs, resp, nil
}

// GetRuleset gets a repository ruleset for the specified repository.
// If includesParents is true, rulesets configured at the organization or enterprise level that apply to the repository will be returned.
//
// GitHub API docs: https://docs.github.com/rest/repos/rules?apiVersion=2022-11-28#get-a-repository-ruleset
//
//meta:operation GET /repos/{owner}/{repo}/rulesets/{ruleset_id}
func (s *RepositoriesService) GetRuleset(ctx context.Context, owner, repo string, rulesetID int64, includesParents bool) (*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/rulesets/%v?includes_parents=%v", owner, repo, rulesetID, includesParents)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var ruleset *RepositoryRuleset
	resp, err := s.client.Do(req, &ruleset)
	if err != nil {
		return nil, resp, err
	}

	return ruleset, resp, nil
}

// UpdateRuleset updates a repository ruleset for the specified repository.
//
// GitHub API docs: https://docs.github.com/rest/repos/rules?apiVersion=2022-11-28#update-a-repository-ruleset
//
//meta:operation PUT /repos/{owner}/{repo}/rulesets/{ruleset_id}
func (s *RepositoriesService) UpdateRuleset(ctx context.Context, owner, repo string, rulesetID int64, body RepositoryRuleset) (*RepositoryRuleset, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/rulesets/%v", owner, repo, rulesetID)

	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var rs *RepositoryRuleset
	resp, err := s.client.Do(req, &rs)
	if err != nil {
		return nil, resp, err
	}

	return rs, resp, nil
}

// DeleteRuleset deletes a repository ruleset for the specified repository.
//
// GitHub API docs: https://docs.github.com/rest/repos/rules?apiVersion=2022-11-28#delete-a-repository-ruleset
//
//meta:operation DELETE /repos/{owner}/{repo}/rulesets/{ruleset_id}
func (s *RepositoriesService) DeleteRuleset(ctx context.Context, owner, repo string, rulesetID int64) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/rulesets/%v", owner, repo, rulesetID)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}
