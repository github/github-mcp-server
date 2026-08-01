// Copyright 2013 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// Label represents a GitHub label on an Issue.
type Label struct {
	ID          int64   `json:"id"`
	URL         string  `json:"url"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
	Default     bool    `json:"default"`
	NodeID      string  `json:"node_id"`
}

func (l Label) String() string {
	return Stringify(l)
}

// CreateIssueLabelRequest represents a request to create a label.
type CreateIssueLabelRequest struct {
	Name        string  `json:"name"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

// UpdateIssueLabelRequest represents a request to update a label.
type UpdateIssueLabelRequest struct {
	NewName     *string `json:"new_name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

// ListLabels lists all labels for a repository.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#list-labels-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/labels
func (s *IssuesService) ListLabels(ctx context.Context, owner, repo string, opts *ListOptions) ([]*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/labels", owner, repo)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var labels []*Label
	resp, err := s.client.Do(req, &labels)
	if err != nil {
		return nil, resp, err
	}

	return labels, resp, nil
}

// GetLabel gets a single label.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#get-a-label
//
//meta:operation GET /repos/{owner}/{repo}/labels/{name}
func (s *IssuesService) GetLabel(ctx context.Context, owner, repo, name string) (*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/labels/%v", owner, repo, name)
	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var label *Label
	resp, err := s.client.Do(req, &label)
	if err != nil {
		return nil, resp, err
	}

	return label, resp, nil
}

// CreateLabel creates a new label on the specified repository.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#create-a-label
//
//meta:operation POST /repos/{owner}/{repo}/labels
func (s *IssuesService) CreateLabel(ctx context.Context, owner, repo string, body CreateIssueLabelRequest) (*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/labels", owner, repo)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var l *Label
	resp, err := s.client.Do(req, &l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, nil
}

// UpdateLabel updates a label.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#update-a-label
//
//meta:operation PATCH /repos/{owner}/{repo}/labels/{name}
func (s *IssuesService) UpdateLabel(ctx context.Context, owner, repo, name string, body UpdateIssueLabelRequest) (*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/labels/%v", owner, repo, name)
	req, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var l *Label
	resp, err := s.client.Do(req, &l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, nil
}

// DeleteLabel deletes a label.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#delete-a-label
//
//meta:operation DELETE /repos/{owner}/{repo}/labels/{name}
func (s *IssuesService) DeleteLabel(ctx context.Context, owner, repo, name string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/labels/%v", owner, repo, name)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}
	return s.client.Do(req, nil)
}

// ListLabelsByIssue lists all labels for an issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#list-labels-for-an-issue
//
//meta:operation GET /repos/{owner}/{repo}/issues/{issue_number}/labels
func (s *IssuesService) ListLabelsByIssue(ctx context.Context, owner, repo string, number int, opts *ListOptions) ([]*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/labels", owner, repo, number)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var labels []*Label
	resp, err := s.client.Do(req, &labels)
	if err != nil {
		return nil, resp, err
	}

	return labels, resp, nil
}

// AddLabelsToIssue adds labels to an issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#add-labels-to-an-issue
//
//meta:operation POST /repos/{owner}/{repo}/issues/{issue_number}/labels
func (s *IssuesService) AddLabelsToIssue(ctx context.Context, owner, repo string, number int, body []string) ([]*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/labels", owner, repo, number)
	req, err := s.client.NewRequest(ctx, "POST", u, body)
	if err != nil {
		return nil, nil, err
	}

	var l []*Label
	resp, err := s.client.Do(req, &l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, nil
}

// RemoveLabelForIssue removes a label for an issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#remove-a-label-from-an-issue
//
//meta:operation DELETE /repos/{owner}/{repo}/issues/{issue_number}/labels/{name}
func (s *IssuesService) RemoveLabelForIssue(ctx context.Context, owner, repo string, number int, label string) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/labels/%v", owner, repo, number, label)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ReplaceLabelsForIssue replaces all labels for an issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#set-labels-for-an-issue
//
//meta:operation PUT /repos/{owner}/{repo}/issues/{issue_number}/labels
func (s *IssuesService) ReplaceLabelsForIssue(ctx context.Context, owner, repo string, number int, body []string) ([]*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/labels", owner, repo, number)
	req, err := s.client.NewRequest(ctx, "PUT", u, body)
	if err != nil {
		return nil, nil, err
	}

	var l []*Label
	resp, err := s.client.Do(req, &l)
	if err != nil {
		return nil, resp, err
	}

	return l, resp, nil
}

// RemoveLabelsForIssue removes all labels for an issue.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#remove-all-labels-from-an-issue
//
//meta:operation DELETE /repos/{owner}/{repo}/issues/{issue_number}/labels
func (s *IssuesService) RemoveLabelsForIssue(ctx context.Context, owner, repo string, number int) (*Response, error) {
	u := fmt.Sprintf("repos/%v/%v/issues/%v/labels", owner, repo, number)
	req, err := s.client.NewRequest(ctx, "DELETE", u, nil)
	if err != nil {
		return nil, err
	}

	return s.client.Do(req, nil)
}

// ListLabelsForMilestone lists labels for every issue in a milestone.
//
// GitHub API docs: https://docs.github.com/rest/issues/labels?apiVersion=2022-11-28#list-labels-for-issues-in-a-milestone
//
//meta:operation GET /repos/{owner}/{repo}/milestones/{milestone_number}/labels
func (s *IssuesService) ListLabelsForMilestone(ctx context.Context, owner, repo string, number int, opts *ListOptions) ([]*Label, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/milestones/%v/labels", owner, repo, number)
	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var labels []*Label
	resp, err := s.client.Do(req, &labels)
	if err != nil {
		return nil, resp, err
	}

	return labels, resp, nil
}
