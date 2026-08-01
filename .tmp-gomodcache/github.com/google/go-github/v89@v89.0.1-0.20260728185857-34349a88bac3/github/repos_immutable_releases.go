// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// RepoImmutableReleasesStatus represents the immutable releases status for a repository.
type RepoImmutableReleasesStatus struct {
	Enabled         *bool `json:"enabled,omitempty"`
	EnforcedByOwner *bool `json:"enforced_by_owner,omitempty"`
}

// EnableImmutableReleases enables immutable releases for a repository.
//
// GitHub API docs: https://docs.github.com/rest/repos/repos?apiVersion=2022-11-28#enable-immutable-releases
//
//meta:operation PUT /repos/{owner}/{repo}/immutable-releases
func (s *RepositoriesService) EnableImmutableReleases(ctx context.Context, owner, repo string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/immutable-releases", owner, repo)

	req, err := s.client.NewRequest(ctx, "PUT", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// DisableImmutableReleases disables immutable releases for a repository.
//
// GitHub API docs: https://docs.github.com/rest/repos/repos?apiVersion=2022-11-28#disable-immutable-releases
//
//meta:operation DELETE /repos/{owner}/{repo}/immutable-releases
func (s *RepositoriesService) DisableImmutableReleases(ctx context.Context, owner, repo string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/immutable-releases", owner, repo)

	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req, nil)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// AreImmutableReleasesEnabled checks if immutable releases are enabled for
// the repository.
//
// GitHub API docs: https://docs.github.com/rest/repos/repos?apiVersion=2022-11-28#check-if-immutable-releases-are-enabled-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/immutable-releases
func (s *RepositoriesService) AreImmutableReleasesEnabled(ctx context.Context, owner, repo string) (*RepoImmutableReleasesStatus, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/immutable-releases", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var status *RepoImmutableReleasesStatus
	resp, err := s.client.Do(req, &status)
	if err != nil {
		return nil, resp, err
	}

	return status, resp, nil
}
