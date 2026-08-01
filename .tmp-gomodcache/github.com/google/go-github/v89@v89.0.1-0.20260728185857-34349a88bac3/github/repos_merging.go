// Copyright 2014 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// RepositoryMergeRequest represents a request to merge a branch in a
// repository.
type RepositoryMergeRequest struct {
	Base          string  `json:"base"`
	Head          string  `json:"head"`
	CommitMessage *string `json:"commit_message,omitempty"`
}

// RepoMergeUpstreamRequest represents a request to sync a branch of
// a forked repository to keep it up-to-date with the upstream repository.
type RepoMergeUpstreamRequest struct {
	Branch string `json:"branch"`
}

// RepoMergeUpstreamResult represents the result of syncing a branch of
// a forked repository with the upstream repository.
type RepoMergeUpstreamResult struct {
	Message    *string `json:"message,omitempty"`
	MergeType  *string `json:"merge_type,omitempty"`
	BaseBranch *string `json:"base_branch,omitempty"`
}

// Merge a branch in the specified repository.
//
// GitHub API docs: https://docs.github.com/rest/branches/branches?apiVersion=2022-11-28#merge-a-branch
//
//meta:operation POST /repos/{owner}/{repo}/merges
func (s *RepositoriesService) Merge(ctx context.Context, owner, repo string, body RepositoryMergeRequest) (*RepositoryCommit, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/merges", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var commit *RepositoryCommit
	resp, err := s.client.Do(req, &commit)
	if err != nil {
		return nil, resp, err
	}

	return commit, resp, nil
}

// MergeUpstream syncs a branch of a forked repository to keep it up-to-date
// with the upstream repository.
//
// GitHub API docs: https://docs.github.com/rest/branches/branches?apiVersion=2022-11-28#sync-a-fork-branch-with-the-upstream-repository
//
//meta:operation POST /repos/{owner}/{repo}/merge-upstream
func (s *RepositoriesService) MergeUpstream(ctx context.Context, owner, repo string, body RepoMergeUpstreamRequest) (*RepoMergeUpstreamResult, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/merge-upstream", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var result *RepoMergeUpstreamResult
	resp, err := s.client.Do(req, &result)
	if err != nil {
		return nil, resp, err
	}

	return result, resp, nil
}
