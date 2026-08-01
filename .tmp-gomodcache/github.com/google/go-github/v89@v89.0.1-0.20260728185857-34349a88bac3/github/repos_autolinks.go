// Copyright 2021 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// CreateAutolinkRequest specifies parameters for RepositoriesService.CreateAutolink method.
type CreateAutolinkRequest struct {
	KeyPrefix      string `json:"key_prefix"`
	URLTemplate    string `json:"url_template"`
	IsAlphanumeric *bool  `json:"is_alphanumeric,omitempty"`
}

// Autolink represents autolinks to external resources like Jira issues and Zendesk tickets.
type Autolink struct {
	ID             *int64  `json:"id,omitempty"`
	KeyPrefix      *string `json:"key_prefix,omitempty"`
	URLTemplate    *string `json:"url_template,omitempty"`
	IsAlphanumeric *bool   `json:"is_alphanumeric,omitempty"`
}

// ListAutolinks returns a list of autolinks configured for the given repository.
// Information about autolinks are only available to repository administrators.
//
// GitHub API docs: https://docs.github.com/rest/repos/autolinks?apiVersion=2022-11-28#get-all-autolinks-of-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/autolinks
func (s *RepositoriesService) ListAutolinks(ctx context.Context, owner, repo string) ([]*Autolink, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/autolinks", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var autolinks []*Autolink
	resp, err := s.client.Do(req, &autolinks)
	if err != nil {
		return nil, resp, err
	}

	return autolinks, resp, nil
}

// CreateAutolink creates an autolink reference for a repository.
// Users with admin access to the repository can create an autolink.
//
// GitHub API docs: https://docs.github.com/rest/repos/autolinks?apiVersion=2022-11-28#create-an-autolink-reference-for-a-repository
//
//meta:operation POST /repos/{owner}/{repo}/autolinks
func (s *RepositoriesService) CreateAutolink(ctx context.Context, owner, repo string, body CreateAutolinkRequest) (*Autolink, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/autolinks", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var al *Autolink
	resp, err := s.client.Do(req, &al)
	if err != nil {
		return nil, resp, err
	}
	return al, resp, nil
}

// GetAutolink returns a single autolink reference by ID that was configured for the given repository.
// Information about autolinks are only available to repository administrators.
//
// GitHub API docs: https://docs.github.com/rest/repos/autolinks?apiVersion=2022-11-28#get-an-autolink-reference-of-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/autolinks/{autolink_id}
func (s *RepositoriesService) GetAutolink(ctx context.Context, owner, repo string, id int64) (*Autolink, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/autolinks/%v", owner, repo, id)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var autolink *Autolink
	resp, err := s.client.Do(req, &autolink)
	if err != nil {
		return nil, resp, err
	}

	return autolink, resp, nil
}

// DeleteAutolink deletes a single autolink reference by ID that was configured for the given repository.
// Information about autolinks are only available to repository administrators.
//
// GitHub API docs: https://docs.github.com/rest/repos/autolinks?apiVersion=2022-11-28#delete-an-autolink-reference-from-a-repository
//
//meta:operation DELETE /repos/{owner}/{repo}/autolinks/{autolink_id}
func (s *RepositoriesService) DeleteAutolink(ctx context.Context, owner, repo string, id int64) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/autolinks/%v", owner, repo, id)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req, nil)
}
