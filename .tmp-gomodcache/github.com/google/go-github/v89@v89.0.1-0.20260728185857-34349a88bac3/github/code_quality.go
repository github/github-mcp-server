// Copyright 2026 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// CodeQualityFindingRule represents the rule associated with a code quality finding.
type CodeQualityFindingRule struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Help        *string `json:"help,omitempty"`
	Severity    string  `json:"severity"`
	Category    string  `json:"category"`
}

// CodeQualityFindingLocation represents the location of a code quality finding.
type CodeQualityFindingLocation struct {
	Path        string `json:"path"`
	StartLine   *int   `json:"start_line,omitempty"`
	EndLine     *int   `json:"end_line,omitempty"`
	StartColumn *int   `json:"start_column,omitempty"`
	EndColumn   *int   `json:"end_column,omitempty"`
}

// CodeQualityFindingMessage represents the message of a code quality finding.
type CodeQualityFindingMessage struct {
	Text     string `json:"text"`
	Markdown string `json:"markdown"`
}

// CodeQualityFinding represents a single code quality finding.
type CodeQualityFinding struct {
	Number    int                        `json:"number"`
	State     string                     `json:"state"`
	URL       string                     `json:"url"`
	Rule      CodeQualityFindingRule     `json:"rule"`
	Location  CodeQualityFindingLocation `json:"location"`
	Message   CodeQualityFindingMessage  `json:"message"`
	CreatedAt *Timestamp                 `json:"created_at,omitempty"`
}

// ListCodeQualityFindingsOptions specifies the optional parameters to
// CodeQualityService.ListFindings.
type ListCodeQualityFindingsOptions struct {
	State     string `url:"state,omitempty"`
	Direction string `url:"direction,omitempty"`

	ListCursorOptions
}

// CodeQualityService handles communication with the code quality related
// methods of the GitHub API.
//
// GitHub API docs: https://docs.github.com/rest/code-quality/code-quality?apiVersion=2022-11-28
type CodeQualityService service

// CodeQualitySetupConfiguration represents a code quality setup configuration for a repository.
type CodeQualitySetupConfiguration struct {
	State       *string    `json:"state,omitempty"`
	Languages   []string   `json:"languages,omitempty"`
	RunnerType  *string    `json:"runner_type,omitempty"`
	RunnerLabel *string    `json:"runner_label,omitempty"`
	UpdatedAt   *Timestamp `json:"updated_at,omitempty"`
	Schedule    *string    `json:"schedule,omitempty"`
}

// CodeQualityUpdateSetupRequest specifies parameters to the
// CodeQualityService.UpdateSetup method.
type CodeQualityUpdateSetupRequest struct {
	State       *string  `json:"state,omitempty"`
	RunnerType  *string  `json:"runner_type,omitempty"`
	RunnerLabel *string  `json:"runner_label,omitempty"`
	Languages   []string `json:"languages,omitempty"`
}

// CodeQualityUpdateSetupResponse represents a response from updating a code quality setup configuration.
type CodeQualityUpdateSetupResponse struct {
	RunID  *int64  `json:"run_id,omitempty"`
	RunURL *string `json:"run_url,omitempty"`
}

// GetSetup gets a code quality setup configuration for a repository.
//
// GitHub API docs: https://docs.github.com/rest/code-quality/code-quality?apiVersion=2022-11-28#get-a-code-quality-setup-configuration
//
//meta:operation GET /repos/{owner}/{repo}/code-quality/setup
func (s *CodeQualityService) GetSetup(ctx context.Context, owner, repo string) (*CodeQualitySetupConfiguration, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/code-quality/setup", owner, repo)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var cfg *CodeQualitySetupConfiguration
	resp, err := s.client.Do(req, &cfg)
	if err != nil {
		return nil, resp, err
	}

	return cfg, resp, nil
}

// UpdateSetup updates a code quality setup configuration for a repository.
//
// This method might return an AcceptedError and a status code of 202. This is because this is the status that GitHub
// returns to signify that it has now scheduled the update in a background task.
//
// GitHub API docs: https://docs.github.com/rest/code-quality/code-quality?apiVersion=2022-11-28#update-a-code-quality-setup-configuration
//
//meta:operation PATCH /repos/{owner}/{repo}/code-quality/setup
func (s *CodeQualityService) UpdateSetup(ctx context.Context, owner, repo string, body CodeQualityUpdateSetupRequest) (*CodeQualityUpdateSetupResponse, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/code-quality/setup", owner, repo)

	request, err := s.client.NewRequest(ctx, "PATCH", u, body)
	if err != nil {
		return nil, nil, err
	}

	var result *CodeQualityUpdateSetupResponse
	resp, err := s.client.Do(request, &result)
	if err != nil {
		return nil, resp, err
	}

	return result, resp, nil
}

// ListFindings lists code quality findings for a repository.
//
// GitHub API docs: https://docs.github.com/rest/code-quality/code-quality?apiVersion=2022-11-28#list-code-quality-findings-for-a-repository
//
//meta:operation GET /repos/{owner}/{repo}/code-quality/findings
func (s *CodeQualityService) ListFindings(ctx context.Context, owner, repo string, opts *ListCodeQualityFindingsOptions) ([]*CodeQualityFinding, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/code-quality/findings", owner, repo)

	u, err := addOptions(u, opts)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var findings []*CodeQualityFinding
	resp, err := s.client.Do(req, &findings)
	if err != nil {
		return nil, resp, err
	}

	return findings, resp, nil
}

// GetFinding gets a single code quality finding for a repository.
//
// GitHub API docs: https://docs.github.com/rest/code-quality/code-quality?apiVersion=2022-11-28#get-a-code-quality-finding
//
//meta:operation GET /repos/{owner}/{repo}/code-quality/findings/{finding_number}
func (s *CodeQualityService) GetFinding(ctx context.Context, owner, repo string, findingNumber int) (*CodeQualityFinding, *Response, error) {
	u := fmt.Sprintf("repos/%v/%v/code-quality/findings/%v", owner, repo, findingNumber)

	req, err := s.client.NewRequest(ctx, "GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	var finding *CodeQualityFinding
	resp, err := s.client.Do(req, &finding)
	if err != nil {
		return nil, resp, err
	}

	return finding, resp, nil
}
